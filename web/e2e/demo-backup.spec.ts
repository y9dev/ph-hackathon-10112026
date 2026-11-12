/* eslint-disable @typescript-eslint/no-explicit-any -- window.__demo is the injected overlay API */
import { expect, test, type Locator } from '@playwright/test';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { loginAs, logout, openFromInbox, ROLES, uniqueTitle } from './helpers';
import { Director, DemoRecorder } from './demo-kit/recorder';

// Backup demo video for the 3-minute pitch (pitch/SCRIPT.md, slide 4: 70 seconds, five steps).
// Not part of the regular suite: it drives a seeded real backend with a slow animated cursor, spotlight
// captions and short step cards, and writes JPEG frames plus a sound timeline. Afterwards
// `node scripts/make-demo-video.mjs` renders pitch/demo-backup.mp4 (see DEMO_SCRIPT.md).
test.use({ viewport: { width: 1280, height: 720 }, deviceScaleFactor: 1.5 });

const FRAMES = process.env.DEMO_FRAMES_DIR ?? join(tmpdir(), 'modus-demo-frames');

/** The list row (or the link itself) that contains a request link. */
const rowOf = (link: Locator) => link.locator('xpath=ancestor::*[self::li or self::tr or self::article][1]').or(link).first();

test('backup demo video: five steps of one request', async ({ page }) => {
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
  await page.evaluate(() => (window as any).__demo.card('Moonshard', 'Modus Voice', 'Идея сотрудника — до решения'));
  await d.pause(900);
  await rec.start();
  await page.evaluate(() => {
    const r = document.getElementById('__demo')!;
    r.style.opacity = '0.99';
    requestAnimationFrame(() => (r.style.opacity = '1'));
  });
  rec.mark('card');
  await d.pause(1700);

  // ---- 1. submit ----------------------------------------------------------------------------------
  await d.sceneCard('Шаг 1 из 5', 'Подача', 'Алексей из закупок сообщает о проблеме', 700);
  await offCamera(async () => {
    await loginAs(page, ROLES.employee);
  });
  await d.cardOff();

  const title = uniqueTitle('Нет параметров в заявке');
  await d.click(page.getByRole('link', { name: 'Новое обращение' }).first());
  await expect(page.getByRole('heading', { level: 1, name: 'Новое обращение' })).toBeVisible();
  await d.type(page.getByRole('combobox', { name: 'Категория' }), 'Межотдел');
  await d.click(page.getByRole('listbox', { name: 'Список категорий' }).getByRole('option').first());
  await d.explain(page.locator('aside[aria-label="Что произойдёт дальше"]'), 'Сразу видно, кому уйдёт и к какому сроку', { kicker: 'Владелец и срок' });
  await d.type(page.getByLabel('Коротко о сути'), title, 10);
  await d.type(page.getByLabel('Что произошло'), 'Закупка каждый раз уточняет параметры по телефону.', 10);
  await d.click(page.getByRole('button', { name: 'Отправить', exact: true }));
  await expect(page.getByRole('heading', { name: 'Спасибо! Обращение принято' })).toBeVisible();
  rec.mark('ok');
  const number = (await page.getByTestId('request-number').textContent())!.trim();
  await d.explain(page.getByTestId('request-number'), 'Номер, владелец, срок. Не потеряется', { kicker: 'Принято', pad: 22 });

  // ---- 2. manager asks ------------------------------------------------------------------------------
  await d.sceneCard('Шаг 2 из 5', 'Вопрос', 'Руководитель просит уточнить', 700);
  await offCamera(async () => {
    await logout(page);
    await loginAs(page, ROLES.purchasingHead);
    await page.goto('/inbox?tab=all');
  });
  await d.cardOff();

  const link = page.getByRole('link', { name: title }).first();
  await expect(link).toBeVisible();
  await d.explain(rowOf(link), 'Во «Входящих», срок уже идёт', { kicker: 'Руководитель', pad: 6 });
  await d.click(link);
  await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
  await d.click(page.getByRole('button', { name: 'Запросить уточнение' }));
  await d.type(page.getByLabel('Текст сообщения'), 'Какое напряжение, сколько точек?', 10);
  await d.click(page.getByRole('button', { name: 'Отправить' }));
  await expect(page.getByText('Вопрос отправлен, автор уведомлён').first()).toBeVisible();
  rec.mark('ok');
  await d.caption('Срок на паузе, автор получил уведомление', 'top', 'Уточнение');
  await d.pause(1100);

  // ---- 3. author answers -----------------------------------------------------------------------------
  await d.sceneCard('Шаг 3 из 5', 'Ответ', 'Автор отвечает в системе', 700);
  await offCamera(async () => {
    await logout(page);
    await loginAs(page, ROLES.employee);
  });
  await d.cardOff();

  const mine = page.getByRole('link', { name: title }).first();
  await expect(mine).toBeVisible();
  await d.click(mine);
  await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
  await d.explain(page.getByText('Нужен ваш ответ').first(), 'Автор видит вопрос', { kicker: 'Мои обращения', pad: 14 });
  await d.click(page.getByRole('button', { name: 'Ответить' }));
  await d.type(page.getByLabel('Текст сообщения'), '220 В, шесть точек.', 10);
  await d.click(page.getByRole('button', { name: 'Отправить' }));
  await expect(page.getByText('Ответ отправлен').first()).toBeVisible();
  rec.mark('ok');
  await d.caption('Обращение снова в работе', 'top', 'Ответ');
  await d.pause(900);

  // ---- 4. stages and result ---------------------------------------------------------------------------
  await d.sceneCard('Шаг 4 из 5', 'Итог', 'Стадии, отчёт и обязательный результат', 700);
  await offCamera(async () => {
    await logout(page);
    await loginAs(page, ROLES.purchasingHead);
    await openFromInbox(page, { number, title });
  });
  await d.cardOff();

  const dialog = page.getByRole('dialog');
  await d.click(page.getByRole('button', { name: 'Перейти к принятию решения', exact: true }));
  await expect(dialog.getByText('Автор получит уведомление')).toBeVisible();
  await d.click(dialog.getByRole('button', { name: 'Перейти к принятию решения', exact: true }));
  await expect(dialog).toBeHidden();
  await d.click(page.getByRole('button', { name: 'Начать реализацию', exact: true }));
  await d.click(dialog.getByRole('button', { name: 'Начать реализацию', exact: true }));
  await expect(dialog).toBeHidden();
  await d.type(page.getByLabel('Текст сообщения'), 'Параметры согласованы.', 10);
  await d.click(page.getByRole('button', { name: 'Отправить' }));
  await expect(page.getByText('Отчёт отправлен, автор уведомлён').first()).toBeVisible();
  rec.mark('ok');
  await d.explain(page.getByRole('button', { name: 'Завершить с результатом' }), 'Без результата не закрыть', { kicker: 'Главное правило', pad: 12 });
  await d.click(page.getByRole('button', { name: 'Завершить с результатом' }));
  await d.type(dialog.getByLabel('Решение для автора'), 'Параметры в заявках теперь обязательны.', 10);
  await d.click(dialog.getByRole('button', { name: 'Завершить с результатом' }));
  await expect(dialog).toBeHidden();
  const done = page.getByRole('heading', { name: 'Решение: Реализовано' });
  await expect(done).toBeVisible();
  rec.mark('ok');
  await d.explain(done, 'Результат и история сохранены', { kicker: 'Готово', pad: 14 });

  // ---- 5. numbers -------------------------------------------------------------------------------------
  await d.sceneCard('Шаг 5 из 5', 'Цифры', 'Что видит руководство', 700);
  await offCamera(async () => {
    await page.evaluate(() => (window as any).__demo.caption(''));
    await logout(page);
    await loginAs(page, ROLES.admin);
    await page.goto('/analytics');
    await expect(page.getByTestId('by-type')).toBeVisible();
  });
  await d.cardOff();
  await d.explain(page.locator('section[aria-labelledby="kpi-title"]'), 'Сколько рассмотрено вовремя и до результата', { kicker: 'Аналитика', pad: 8 });

  // ---- end card ----------------------------------------------------------------------------------------
  await d.sceneCard('Moonshard', 'Слова — в решения', 'Modus Voice', 1700);
  const result = await rec.stop(1);
  // eslint-disable-next-line no-console
  console.log(`demo frames: ${result.frames}, events: ${result.events}, duration: ${result.duration.toFixed(1)} s -> ${FRAMES}`);
});
