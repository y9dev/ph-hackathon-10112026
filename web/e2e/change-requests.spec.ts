// CR1–CR6 journeys: 18-category picker, anonymous token flow, observer restrictions, analytics tiles,
// theme dialog regression (CR3) and the Telegram page. Runs against the mock and the real backend.
import { expect, test, type Page } from '@playwright/test';
import { createSimilarGroup, isMock, loginAs, logout, noHorizontalScroll, pickCategory, ROLES, uniqueTitle } from './helpers';

test.describe('CR1 category picker, types and priority', () => {
  test('18 categories are searchable and keyboard accessible; first text field stays visible at 375 @mobile', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/new');
    const box = page.getByRole('combobox', { name: 'Категория' });
    await expect(box).toHaveAttribute('aria-expanded', 'false');
    const title = await page.getByLabel('Коротко о сути').boundingBox();
    expect(title!.y).toBeLessThan(812 - 64 - 72);
    await noHorizontalScroll(page, '/new @375');

    await box.click();
    const list = page.getByRole('listbox', { name: 'Список категорий' });
    await expect(list.getByRole('option')).toHaveCount(18);
    await box.fill('док');
    await expect(list.getByRole('option', { name: /Документооборот/ })).toBeVisible();
    await expect(list.getByRole('option', { name: /Хранение документов/ })).toBeVisible();
    await box.fill('несуществующее');
    await expect(list.getByText('Ничего не нашли')).toBeVisible();
    // Keyboard: type, arrow, Enter.
    await box.fill('');
    await box.pressSequentially('обуч');
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('Enter');
    await expect(box).toHaveValue('Обучение');
    await expect(page.getByTestId('category-description')).toHaveText('Занятия, семинары и обучение сотрудников');
    await page.keyboard.press('ArrowDown');
    await expect(list).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(list).toBeHidden();
  });

  test('positive feedback adapts the copy; priority defaults to Средний', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await page.goto('/new');
    await expect(page.getByLabel('Что произошло')).toBeVisible();
    await page.getByRole('radio', { name: 'Предложение' }).click();
    await expect(page.getByLabel('Что предлагаете')).toBeVisible();
    await page.getByRole('radio', { name: 'Положительный отзыв' }).click();
    await expect(page.getByLabel('За что благодарите')).toBeVisible();
    await expect(page.getByLabel(/Как, по-вашему, это решить|Как это можно сделать/)).toHaveCount(0);
    await expect(page.getByRole('radiogroup', { name: 'Приоритет' }).getByRole('radio', { name: 'Средний' })).toHaveAttribute('aria-checked', 'true');
  });
});

/** Creates an anonymous request as the demo employee and returns its number and token (success screen). */
async function createAnonymous(page: Page, title: string): Promise<{ number: string; token: string }> {
  await loginAs(page, ROLES.employee);
  await page.goto('/new');
  await pickCategory(page);
  await page.getByLabel('Коротко о сути').fill(title);
  await page.getByLabel('Что произошло').fill('Анонимное описание проблемы, достаточно подробное для отправки.');
  await page.getByRole('switch', { name: 'Отправить анонимно' }).click();
  await expect(page.getByText('Ваше имя не сохраняется — ни в базе данных, ни в журналах.')).toBeVisible();
  await page.getByRole('button', { name: 'Отправить', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Обращение отправлено анонимно' })).toBeVisible();
  const token = (await page.getByTestId('anonymous-token').textContent())!.trim();
  const number = (await page.getByTestId('request-number').textContent())!.trim();
  return { number, token };
}

test.describe('CR2 anonymity by token', () => {
  test('create → token screen → lookup in a clean browser → staff asks → author answers by token', async ({ page, browser, baseURL }) => {
    const title = uniqueTitle('Анонимно про заявки');
    const { number, token } = await createAnonymous(page, title);
    expect(token).toMatch(/^[0-9A-Z]{4}(-[0-9A-Z]{4})+$/);
    await expect(page.getByText('Это единственный способ вернуться к обращению.', { exact: false })).toBeVisible();
    await expect(page.getByText('восстановить токен невозможно', { exact: false })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Скачать .txt' })).toBeVisible();
    // Close/Done stay locked until "Я сохранил токен".
    await expect(page.getByRole('button', { name: 'Готово' })).toBeDisabled();
    await page.getByLabel('Я сохранил токен').check();
    await page.getByRole('link', { name: 'Готово' }).click();
    await expect(page).toHaveURL(/\/my/);
    // Device list: status via lookup; the named list does not contain it.
    const device = page.getByTestId('device-anon');
    await expect(device.getByText(title)).toBeVisible();
    await expect(device.getByText('Новое')).toBeVisible();
    await logout(page);

    // Owner of the first category asks a question.
    await loginAs(page, ROLES.purchasingHead);
    await page.goto(`/inbox?tab=all&q=${number}`);
    await page.getByRole('link', { name: title }).first().click();
    await expect(page.getByRole('main').getByText('Автор (анонимно)').first()).toBeVisible();
    await page.getByRole('button', { name: 'Запросить уточнение' }).click();
    await page.getByLabel('Текст сообщения').fill('Какие поля в заявке чаще всего пустые?');
    await page.getByRole('button', { name: 'Отправить' }).click();
    await expect(page.getByText('Вопрос отправлен, автор уведомлён').first()).toBeVisible();

    // Clean browser, no login: look up with a messy copy of the token. The mock database lives in the tab's
    // sessionStorage, so in mock runs the same tab is used after logging out (still without a session).
    const ctx = isMock(test.info()) ? null : await browser.newContext({ baseURL, locale: 'ru-RU' });
    if (!ctx) await logout(page);
    const anon = ctx ? await ctx.newPage() : page;
    await anon.goto('/login');
    await anon.getByRole('link', { name: /Проверить анонимное обращение по токену/ }).click();
    await expect(anon).toHaveURL(/\/anonymous$/);
    await anon.getByLabel('Токен доступа').fill(`  ${token.toLowerCase().replace(/-/g, ' ')} `);
    await anon.getByRole('button', { name: 'Открыть обращение' }).click();
    await expect(anon.getByRole('heading', { level: 1, name: title })).toBeVisible();
    expect(anon.url()).not.toContain(token);
    expect(anon.url()).not.toContain(token.replace(/-/g, ''));
    await expect(anon.getByText('Нужен ваш ответ').first()).toBeVisible();
    await expect(anon.getByText('Какие поля в заявке чаще всего пустые?')).toBeVisible();
    await anon.getByRole('button', { name: 'Ответить' }).click();
    await anon.getByLabel(/Текст сообщения|Ответ на уточнение/).fill('Назначение изделия и рабочее давление.');
    await anon.getByRole('button', { name: 'Отправить' }).click();
    await expect(anon.getByText('Ответ отправлен').first()).toBeVisible();
    await expect(anon.getByText('В проработке').first()).toBeVisible();
    // Reload keeps the request open (token in sessionStorage of this tab only).
    await anon.reload();
    await expect(anon.getByRole('heading', { level: 1, name: title })).toBeVisible();
    // A wrong token is refused with a plain message.
    await anon.getByRole('button', { name: 'Выйти из обращения' }).click();
    await anon.getByLabel('Токен доступа').fill('AAAA-BBBB-CCCC-DDDD-EEEE-FFFF-GGGG');
    await anon.getByRole('button', { name: 'Открыть обращение' }).click();
    await expect(anon.getByText('Обращение с таким токеном не найдено', { exact: false })).toBeVisible();
    await ctx?.close();
  });
});

test.describe('CR6 manager of another department (observer)', () => {
  test('sees all requests, owner department, banner; composer limited to comment and note', async ({ page }) => {
    await loginAs(page, ROLES.observer);
    await page.goto('/inbox');
    await page.getByRole('radio', { name: 'Все обращения' }).click();
    await expect(page).toHaveURL(/view=all/);
    await expect(page.getByRole('button', { name: 'Только мои отделы' })).toHaveAttribute('aria-pressed', 'false');
    await page.getByRole('tab', { name: /^Все/ }).click();
    const row = page.getByRole('row').filter({ hasText: 'MV-0001' });
    await expect(row).toBeVisible();
    await expect(row.getByText('Закупки', { exact: true }).filter({ visible: true }).first()).toBeVisible();
    // mine_scope narrows to owned requests (MV-0014 «Планирование ресурсов» belongs to Проектирование).
    await page.getByRole('button', { name: 'Только мои отделы' }).click();
    await expect(page.getByRole('row').filter({ hasText: 'MV-0014' })).toBeVisible();
    await expect(page.getByRole('row').filter({ hasText: 'MV-0001' })).toHaveCount(0);

    await page.goto('/requests/1');
    await expect(page.getByTestId('observer-banner')).toContainText('Менять статус и назначать нельзя');
    await expect(page.getByRole('heading', { name: 'Работа с обращением' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Добавить в тему' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Переназначить' })).toHaveCount(0);
    const kinds = page.getByRole('radiogroup', { name: 'Тип сообщения' }).getByRole('radio');
    await expect(kinds).toHaveText(['Комментарий', 'Заметка']);
    await page.getByRole('radio', { name: 'Заметка' }).click();
    await page.getByLabel('Текст сообщения').fill('У проектирования есть похожий шаблон, можем поделиться.');
    await page.getByRole('button', { name: 'Отправить' }).click();
    await expect(page.getByText('Заметка сохранена').first()).toBeVisible();
    await expect(page.getByText('У проектирования есть похожий шаблон', { exact: false })).toBeVisible();
  });
});

test.describe('CR5 analytics tiles', () => {
  test('«Ответили вовремя» and «Время решения» replace «Первый ответ»; by type block', async ({ page }) => {
    await loginAs(page, ROLES.admin);
    await page.goto('/analytics?period=all');
    const onTime = page.getByTestId('kpi-on-time');
    await expect(onTime.getByRole('heading', { name: 'Ответили вовремя' })).toBeVisible();
    await expect(onTime).toContainText(/\d+%/);
    await expect(onTime).toContainText('Доля обращений, на которые ответили до срока первого ответа');
    await expect(onTime).toContainText(/Обычно отвечаем за .+/);
    await expect(onTime).toContainText(/по \d+ обращени/);
    const res = page.getByTestId('kpi-resolution');
    await expect(res.getByRole('heading', { name: 'Время решения' })).toBeVisible();
    await expect(res).toContainText('Типичное время от подачи до закрытия');
    await expect(res).toContainText(/(час|день|дня|дней)/);
    await expect(page.getByRole('heading', { name: 'Первый ответ', exact: true })).toHaveCount(0);
    await expect(page.getByText(/90%|перцентил|медиан/i)).toHaveCount(0);
    const byType = page.getByTestId('by-type');
    await expect(byType.getByRole('heading', { name: 'По типам обращений' })).toBeVisible();
    for (const t of ['Проблема', 'Предложение', 'Положительный отзыв']) await expect(byType.getByText(t)).toBeVisible();
    await expect(byType.getByText(/закрыто \d+ из \d+/).first()).toBeVisible();
  });

  test('managers get company-wide analytics with the department filter', async ({ page }) => {
    await loginAs(page, ROLES.observer);
    await page.goto('/analytics?period=all');
    await expect(page.getByTestId('kpi-on-time')).toBeVisible();
    await expect(page.getByLabel('Отдел')).toBeVisible();
  });
});

test.describe('CR3 theme dialog regression', () => {
  for (const [w, h, scheme] of [
    [1280, 800, 'light'],
    [375, 812, 'dark'],
  ] as const) {
    test(`two clear options, validation, reopen state, Esc and focus @${w} ${scheme}`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme });
      await page.setViewportSize({ width: w, height: h });
      await loginAs(page, ROLES.employee);
      const title = uniqueTitle('Тема для заявок');
      await page.goto('/new');
      await pickCategory(page);
      await page.getByLabel('Коротко о сути').fill(title);
      await page.getByLabel('Что произошло').fill('Описание для проверки диалога добавления в тему.');
      await page.getByRole('button', { name: 'Отправить', exact: true }).click();
      await expect(page.getByTestId('request-number')).toBeVisible();
      await page.getByRole('link', { name: 'Открыть обращение' }).click();
      const url = page.url();
      await logout(page);
      await loginAs(page, ROLES.purchasingHead);
      await page.goto(url);

      const opener = page.getByRole('button', { name: 'Добавить в тему' });
      await opener.click();
      const dialog = page.getByRole('dialog');
      await expect(dialog.getByRole('radio', { name: 'Новая тема' })).toHaveAttribute('aria-checked', 'true');
      await expect(dialog.getByRole('radio', { name: 'Добавить в существующую' })).toBeVisible();
      // One field per mode, each with its own label (no "Существующая тема" select whose first option is "new").
      await expect(dialog.getByLabel('Название новой темы')).toBeFocused();
      await expect(dialog.getByText('Существующая тема')).toHaveCount(0);
      await expect(dialog.getByRole('combobox')).toHaveCount(0);
      const box = await dialog.boundingBox();
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(w);
      await noHorizontalScroll(page, `theme dialog @${w}`);

      // Validation: empty and 1 character are rejected inline (server rule 2..140).
      await dialog.getByRole('button', { name: 'Добавить', exact: true }).click();
      await expect(dialog.getByText('Название темы: от 2 до 140 символов')).toBeVisible();
      await dialog.getByLabel('Название новой темы').fill('А');
      await dialog.getByRole('button', { name: 'Добавить', exact: true }).click();
      await expect(dialog.getByText('Название темы: от 2 до 140 символов')).toBeVisible();
      await expect(dialog.getByLabel('Название новой темы')).toHaveValue('А');

      // Esc closes the dialog (not only a dropdown) and returns focus to the opener.
      await page.keyboard.press('Escape');
      await expect(dialog).toBeHidden();
      await expect(opener).toBeFocused();

      // Reopen: state is fresh.
      await opener.click();
      await expect(dialog.getByLabel('Название новой темы')).toHaveValue('');
      await expect(dialog.getByText('Название темы: от 2 до 140 символов')).toHaveCount(0);
      const theme = `Шаблоны заявок ${title.slice(-5)}`;
      await dialog.getByLabel('Название новой темы').fill(theme);
      await dialog.getByRole('button', { name: 'Добавить', exact: true }).click();
      await expect(dialog).toBeHidden();
      // The card reflects the theme right away.
      await expect(page.getByRole('button', { name: `Тема: ${theme}` })).toBeVisible();
      await expect(page.getByText(theme).first()).toBeVisible();

      // Reopen: "existing" mode with the current theme selected.
      await page.getByRole('button', { name: `Тема: ${theme}` }).click();
      await expect(dialog.getByRole('radio', { name: 'Добавить в существующую' })).toHaveAttribute('aria-checked', 'true');
      await expect(dialog.getByRole('radio', { name: new RegExp(theme) })).toBeChecked();
      await expect(dialog.getByText('Обращение уже в этой теме.')).toBeVisible();
      await dialog.getByLabel('Найти тему').fill('нет такой темы');
      await expect(dialog.getByText('Ничего не нашли', { exact: false })).toBeVisible();
      await dialog.getByRole('button', { name: 'Отмена' }).click();
      await expect(dialog).toBeHidden();
    });
  }
});

test.describe('CR3 suggested group → «Создать тему»', () => {
  test('creates the theme for the ticked requests and the request card shows it', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await createSimilarGroup(page);
    await logout(page);
    await loginAs(page, ROLES.admin);
    await page.goto('/analytics');
    const item = page.locator('li').filter({ has: page.getByRole('button', { name: 'Создать тему' }) }).first();
    await expect(item).toBeVisible();
    const title = (await item.locator('p.font-semibold').textContent())!.trim();
    const firstReq = item.getByRole('link').first();
    const href = (await firstReq.getAttribute('href'))!;
    await item.getByRole('button', { name: 'Создать тему' }).click();
    await expect(page.getByText(`Тема «${title}» создана`).first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Подтверждённые темы' })).toBeVisible();
    await expect(page.getByText(title, { exact: true }).first()).toBeVisible();
    await page.goto(href);
    await expect(page.getByRole('button', { name: `Тема: ${title}` })).toBeVisible();
  });
});

test.describe('CR4 Telegram', () => {
  test('bell icon → /telegram: QR, code, countdown, linked state (mock)', async ({ page }, info) => {
    test.skip(!isMock(info), 'mock-only: linking needs the bot');
    await loginAs(page, ROLES.employee);
    // Not in the navigation or the user menu.
    await expect(page.getByRole('navigation', { name: 'Основная навигация' }).first().getByText(/Telegram/)).toHaveCount(0);
    await page.getByRole('button', { name: /Открыть уведомления/ }).click();
    const tg = page.getByRole('link', { name: 'Получать уведомления в Telegram' });
    await expect(tg).toBeVisible();
    const b = await tg.boundingBox();
    expect(b!.width).toBeGreaterThanOrEqual(43.5);
    expect(b!.height).toBeGreaterThanOrEqual(43.5);
    await tg.click();
    await expect(page).toHaveURL(/\/telegram$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Уведомления в Telegram' })).toBeVisible();
    await expect(page.getByRole('img', { name: /QR-код ссылки на бота/ })).toBeVisible();
    await expect(page.getByTestId('telegram-code')).toHaveText(/^[2-9A-Z]{6}$/);
    await expect(page.getByTestId('telegram-seconds')).toContainText(/Код сменится через \d+/);
    const first = await page.getByTestId('telegram-code').textContent();
    // The code rotates (20 s period) without a layout shift of the code box.
    const before = await page.getByTestId('telegram-code-box').boundingBox();
    await expect(page.getByTestId('telegram-code')).not.toHaveText(first!, { timeout: 25_000 });
    const after = await page.getByTestId('telegram-code-box').boundingBox();
    expect(after!.height).toBe(before!.height);
    // The user sends the code to the bot → status polling switches to the linked state.
    await page.evaluate(() => (window as unknown as { __mvMockTelegramLink: (u: string) => void }).__mvMockTelegramLink('demo_user'));
    await expect(page.getByTestId('telegram-linked')).toBeVisible({ timeout: 8000 });
    await expect(page.getByText('Аккаунт: @demo_user')).toBeVisible();
    await expect(page.getByText('/answer MV-0001 текст')).toBeVisible();
    // Notifications page header entry.
    await page.goto('/notifications');
    await expect(page.getByRole('link', { name: 'Получать уведомления в Telegram' })).toBeVisible();
  });

  test('hidden everywhere when the bot is disabled (features.telegram=false)', async ({ page }, info) => {
    test.skip(!isMock(info), 'mock switch ?mockTelegram=off');
    await page.goto('/login?mockTelegram=off');
    await page.getByRole('button', { name: ROLES.employee }).first().click();
    await expect(page.getByRole('navigation', { name: 'Основная навигация' }).first()).toBeVisible();
    await page.getByRole('button', { name: /Открыть уведомления/ }).click();
    await expect(page.getByRole('link', { name: 'Все уведомления' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Получать уведомления в Telegram' })).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.goto('/notifications');
    await expect(page.getByRole('heading', { level: 1, name: 'Уведомления' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Получать уведомления в Telegram' })).toHaveCount(0);
    await page.goto('/telegram');
    await expect(page.getByTestId('telegram-disabled')).toBeVisible();
  });

  test('real backend with a fake Bot API: code sent to the bot links the account', async ({ page, request }, info) => {
    const fake = process.env.TG_FAKE;
    test.skip(isMock(info) || !fake, 'set TG_FAKE=http://127.0.0.1:<port> of the fake Telegram API');
    await loginAs(page, ROLES.sales);
    await page.goto('/telegram');
    const code = (await page.getByTestId('telegram-code').textContent())!.trim();
    await request.post(`${fake}/inject`, { data: { text: `/start ${code}` } });
    await expect(page.getByTestId('telegram-linked')).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText('Аккаунт: @demo_user')).toBeVisible();
  });
});
