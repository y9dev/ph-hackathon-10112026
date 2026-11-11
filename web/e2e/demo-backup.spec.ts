/* eslint-disable @typescript-eslint/no-explicit-any -- window.__demo is the injected overlay API */
import { expect, test, type Locator } from '@playwright/test';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { loginAs, logout, openFromInbox, ROLES, uniqueTitle } from './helpers';
import { Director, DemoRecorder } from './demo-kit/recorder';

// Backup demo video (DEMO_SCRIPT.md). Not part of the regular suite: it drives a seeded real backend with a
// slow animated cursor, spotlight captions and scene cards, and writes JPEG frames plus a sound timeline.
// Afterwards `node scripts/make-demo-video.mjs` renders pitch/demo-backup.mp4 (see DEMO_SCRIPT.md).
// The video shows the main flow with a regular (non-anonymous) request; anonymity is shown separately.
test.use({ viewport: { width: 1280, height: 720 }, deviceScaleFactor: 1.5 });

const FRAMES = process.env.DEMO_FRAMES_DIR ?? join(tmpdir(), 'modus-demo-frames');

/** The list row (or the link itself) that contains a request link. */
const rowOf = (link: Locator) => link.locator('xpath=ancestor::*[self::li or self::tr or self::article][1]').or(link).first();

test('backup demo video: five user scenarios with captions and cursor', async ({ page }) => {
  test.setTimeout(900_000);
  const rec = new DemoRecorder(page, FRAMES);
  const d = new Director(page, rec);
  await d.install();
  const offCamera = async (fn: () => Promise<void>) => {
    rec.pause();
    await fn();
    rec.resume();
  };

  // ---- intro card ------------------------------------------------------------------------------
  await page.goto('/login');
  await page.evaluate(() => (window as any).__demo.card('Modus Voice · демо', 'Путь обращения от сотрудника до решения', 'Работающий MVP на синтетических данных. Пять коротких сценариев.'));
  await d.pause(900);
  await rec.start();
  await page.evaluate(() => {
    const r = document.getElementById('__demo')!;
    r.style.opacity = '0.99';
    requestAnimationFrame(() => (r.style.opacity = '1'));
  });
  rec.mark('card');
  await d.pause(2000);

  // ---- 1. employee submits a request ---------------------------------------------------------------
  await d.sceneCard('Сценарий 1 из 5 · Сотрудник', 'Алексей сообщает о проблеме', 'Сотрудник закупок отправляет обращение за минуту', 1200);
  await offCamera(async () => {
    await loginAs(page, ROLES.employee);
  });
  await d.cardOff();

  const title = uniqueTitle('Нет параметров в заявке на оборудование');
  await d.caption('Сотрудник открывает форму нового обращения', 'bottom', 'Шаг 1');
  await d.click(page.getByRole('link', { name: 'Новое обращение' }).first());
  await expect(page.getByRole('heading', { level: 1, name: 'Новое обращение' })).toBeVisible();
  await d.pause(400);

  await d.caption('Выбирает категорию: от неё зависит, кому уйдёт обращение', 'bottom', 'Шаг 1');
  await d.type(page.getByRole('combobox', { name: 'Категория' }), 'Межотдел');
  await d.click(page.getByRole('listbox', { name: 'Список категорий' }).getByRole('option').first());
  await d.explain(page.locator('aside[aria-label="Что произойдёт дальше"]'), 'Справа сразу видно, кому уйдёт обращение и за какой срок ответят', { kicker: 'Маршрутизация' });

  await d.caption('Коротко описывает суть', 'bottom', 'Шаг 1');
  await d.type(page.getByLabel('Коротко о сути'), title);
  await d.type(page.getByLabel('Что произошло'), 'В заявках на оборудование не указаны параметры, закупка каждый раз уточняет их по телефону.', 12);
  await d.click(page.getByRole('button', { name: 'Отправить', exact: true }));
  await expect(page.getByRole('heading', { name: 'Спасибо! Обращение принято' })).toBeVisible();
  rec.mark('ok');
  const number = (await page.getByTestId('request-number').textContent())!.trim();
  await d.explain(page.getByTestId('request-number'), 'Обращение получило номер, владельца и срок первого ответа. Оно не потеряется', { kicker: 'Обращение принято', pad: 22, extraMs: 300 });

  // ---- 2. manager receives it and asks a question ---------------------------------------------
  await d.sceneCard('Сценарий 2 из 5 · Руководитель', 'Ермек получает обращение', 'Руководитель закупок видит его во «Входящих» и задаёт вопрос', 1200);
  await offCamera(async () => {
    await logout(page);
    await loginAs(page, ROLES.purchasingHead);
    await page.goto('/inbox?tab=all');
  });
  await d.cardOff();

  const link = page.getByRole('link', { name: title }).first();
  await expect(link).toBeVisible();
  await d.explain(rowOf(link), 'Обращение пришло по правилу маршрутизации и стоит в очереди со сроком ответа', { kicker: 'Входящие', pad: 6 });
  await d.click(link);
  await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
  await d.pause(400);
  await d.click(page.getByRole('button', { name: 'Запросить уточнение' }));
  await d.caption('Руководитель уточняет детали у автора', 'top', 'Шаг 2');
  await d.type(page.getByLabel('Текст сообщения'), 'Укажите нормы напряжения и количество точек подключения.');
  await d.click(page.getByRole('button', { name: 'Отправить' }));
  await expect(page.getByText('Вопрос отправлен, автор уведомлён').first()).toBeVisible();
  rec.mark('ok');
  await d.caption('Срок ответа поставлен на паузу, автор получил уведомление', 'top', 'Шаг 2');
  await d.pause(1500);

  // ---- 3. the author answers -------------------------------------------------------------------------
  await d.sceneCard('Сценарий 3 из 5 · Автор', 'Алексей отвечает на вопрос', 'Автор видит запрос уточнения в «Моих обращениях»', 1200);
  await offCamera(async () => {
    await logout(page);
    await loginAs(page, ROLES.employee);
  });
  await d.cardOff();

  const mine = page.getByRole('link', { name: title }).first();
  await expect(mine).toBeVisible();
  await d.explain(rowOf(mine), 'В «Моих обращениях» видно статус: система ждёт ответа автора', { kicker: 'Мои обращения', pad: 6 });
  await d.click(mine);
  await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
  await d.pause(400);
  await d.explain(page.getByText('Нужен ваш ответ').first(), 'Автор видит вопрос руководителя и историю обращения', { kicker: 'Уточнение', pad: 14 });
  await d.click(page.getByRole('button', { name: 'Ответить' }));
  await d.type(page.getByLabel('Текст сообщения'), 'Напряжение 220 В, точек шесть, работа в шкафу на объекте.');
  await d.click(page.getByRole('button', { name: 'Отправить' }));
  await expect(page.getByText('Ответ отправлен').first()).toBeVisible();
  rec.mark('ok');
  await d.caption('Обращение вернулось в работу, руководитель получил уведомление', 'top', 'Шаг 3');
  await d.pause(1500);

  // ---- 4. manager moves through the stages and closes with a result -----------------------------
  await d.sceneCard('Сценарий 4 из 5 · Руководитель', 'Ведёт обращение до результата', 'Стадии, отчёт о ходе и обязательный итог', 1200);
  await offCamera(async () => {
    await logout(page);
    await loginAs(page, ROLES.purchasingHead);
    await openFromInbox(page, { number, title });
  });
  await d.cardOff();

  const dialog = page.getByRole('dialog');
  await d.explain(page.locator('ol:has([aria-current="step"])').first(), 'Обращение идёт по стадиям заказчика: разработка, принятие решения, реализация', { kicker: 'Стадии', pad: 12 });
  await d.click(page.getByRole('button', { name: 'Перейти к принятию решения', exact: true }));
  await expect(dialog.getByText('Автор получит уведомление')).toBeVisible();
  await d.click(dialog.getByRole('button', { name: 'Перейти к принятию решения', exact: true }));
  await expect(dialog).toBeHidden();
  await d.click(page.getByRole('button', { name: 'Начать реализацию', exact: true }));
  await d.click(dialog.getByRole('button', { name: 'Начать реализацию', exact: true }));
  await expect(dialog).toBeHidden();
  await d.caption('Стадия «Реализация»: можно писать отчёты о ходе', 'top', 'Шаг 4');
  await d.type(page.getByLabel('Текст сообщения'), 'Параметры согласованы, заявка обновлена.');
  await d.click(page.getByRole('button', { name: 'Отправить' }));
  await expect(page.getByText('Отчёт отправлен, автор уведомлён').first()).toBeVisible();
  rec.mark('ok');
  await d.pause(600);
  await d.explain(page.getByRole('button', { name: 'Завершить с результатом' }), 'Закрыть обращение без результата нельзя: итог для автора обязателен', { kicker: 'Итог', pad: 12 });
  await d.click(page.getByRole('button', { name: 'Завершить с результатом' }));
  await d.type(dialog.getByLabel('Решение для автора'), 'В заявки добавлены обязательные параметры, закупка работает по новой форме.');
  await d.click(dialog.getByRole('button', { name: 'Завершить с результатом' }));
  await expect(dialog).toBeHidden();
  const done = page.getByRole('heading', { name: 'Решение: Реализовано' });
  await expect(done).toBeVisible();
  rec.mark('ok');
  await d.explain(done, 'Обращение закрыто с результатом. Решение и вся история остаются в системе', { kicker: 'Готово', pad: 14 });

  // ---- 5. analytics ---------------------------------------------------------------------------------------
  await d.sceneCard('Сценарий 5 из 5 · Руководство', 'Что видит администратор', 'Аналитика по всем обращениям', 1200);
  await offCamera(async () => {
    await page.evaluate(() => (window as any).__demo.caption(''));
    await logout(page);
    await loginAs(page, ROLES.admin);
    await page.goto('/analytics');
    await expect(page.getByTestId('by-type')).toBeVisible();
  });
  await d.cardOff();
  await d.explain(page.locator('section[aria-labelledby="kpi-title"]'), 'Сколько обращений рассмотрено, как быстро отвечают и сколько доходит до результата', { kicker: 'Показатели', pad: 8, extraMs: 300 });
  await d.explain(page.getByTestId('by-type'), 'Какие типы обращений приходят чаще: проблемы, предложения или отзывы', { kicker: 'Структура', pad: 8 });

  // ---- end card ----------------------------------------------------------------------------------------
  await d.sceneCard('Modus Voice', 'Обращение с владельцем, сроком и итогом', 'Ограничения: данные синтетические, интеграции с рабочими системами пока в проработке, повторяющиеся проблемы ищутся по сходству текста.', 2800);
  const result = await rec.stop(1.2);
  // eslint-disable-next-line no-console
  console.log(`demo frames: ${result.frames}, events: ${result.events}, duration: ${result.duration.toFixed(1)} s -> ${FRAMES}`);
});
