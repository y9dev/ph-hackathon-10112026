// Fixes from the usability / UI / accessibility audit: dialogs, focus return, analytics scopes,
// suggested groups, routing admin, long-title overflow, mobile submit form.
import { expect, test } from '@playwright/test';
import { createSimilarGroup, loginAs, logout, noHorizontalScroll, openFromInbox, ROLES, submitRequest, uniqueTitle } from './helpers';

test.describe('Request detail: dialogs and focus', () => {
  test('B1 focus returns to the opener on Esc / Cancel; stage move explains itself; close dialogs differ', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    const c = await submitRequest(page, uniqueTitle('Сломан турникет'));
    await logout(page);
    await loginAs(page, ROLES.admin);
    await openFromInbox(page, c);

    // Stage move: confirm text, optional message, Cancel restores focus to the button.
    const take = page.getByRole('button', { name: 'Взять в проработку', exact: true });
    await take.click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toHaveAttribute('aria-modal', 'true');
    await expect(dialog.getByText('Статус изменится на «В проработке». Автор получит уведомление.')).toBeVisible();
    await expect(dialog.getByLabel('Сообщение автору')).toBeVisible();
    await expect(dialog.getByText('(необязательно)')).toBeVisible();
    await dialog.getByRole('button', { name: 'Отмена' }).click();
    await expect(dialog).toBeHidden();
    await expect(take).toBeFocused();

    // Esc returns focus too.
    await take.click();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog')).toBeHidden();
    await expect(take).toBeFocused();

    // Theme dialog.
    const themeBtn = page.getByRole('button', { name: 'Добавить в тему' });
    await themeBtn.click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(themeBtn).toBeFocused();

    // Assign dialog (admin can reassign).
    const assign = page.getByRole('button', { name: 'Переназначить' });
    await assign.click();
    await page.getByRole('dialog').getByRole('button', { name: 'Отмена' }).click();
    await expect(assign).toBeFocused();

    // Close dialogs: distinct title / helper copy, required marker before any error.
    const noImpl = page.getByRole('button', { name: 'Закрыть без реализации' });
    await noImpl.click();
    let d = page.getByRole('dialog');
    await expect(d.getByRole('heading', { name: 'Закрыть без реализации' })).toBeVisible();
    await expect(d.getByText('Объясните, почему реализация не нужна или что было сделано иначе')).toBeVisible();
    await expect(d.getByText('(обязательно)')).toBeVisible();
    await expect(d.getByLabel('Решение для автора')).toHaveAttribute('aria-required', 'true');
    await page.keyboard.press('Escape');
    await expect(noImpl).toBeFocused();

    const decline = page.getByRole('button', { name: 'Отклонить', exact: true });
    await decline.click();
    d = page.getByRole('dialog');
    await expect(d.getByRole('heading', { name: 'Отклонить обращение' })).toBeVisible();
    await expect(d.getByText('Объясните причину отказа')).toBeVisible();
    await d.getByRole('button', { name: 'Отмена' }).click();
    await expect(decline).toBeFocused();

    // Success path: the opener disappears (status changed), focus lands on the staff panel, not on <body>.
    await take.click();
    await page.getByRole('dialog').getByRole('button', { name: 'Взять в проработку', exact: true }).click();
    await expect(page.getByRole('dialog')).toBeHidden();
    await expect(page.locator('[data-dialog-fallback]')).toBeFocused();
    // Without a message nothing extra is posted; the stage item appears in the history.
    await expect(page.getByText('Статус: В проработке. Автор уведомлён').first()).toBeVisible();
  });

  test('A1 a long theme title does not overflow request detail at 375 and 1280', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    const c = await submitRequest(page, uniqueTitle('Длинная тема'));
    await logout(page);
    await loginAs(page, ROLES.admin);
    await openFromInbox(page, c);

    const longTheme = `Тема${'Оченьдлинноеназваниебезпробелов'.repeat(4)}`.slice(0, 140);
    await page.getByRole('button', { name: 'Добавить в тему' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('Название новой темы').fill(longTheme);
    await dialog.getByRole('button', { name: 'Добавить', exact: true }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByRole('button', { name: new RegExp(`^Тема: ${longTheme.slice(0, 20)}`) })).toBeVisible();

    for (const [w, h] of [
      [1280, 800],
      [375, 812],
    ] as const) {
      await page.setViewportSize({ width: w, height: h });
      await noHorizontalScroll(page, `/requests/:id with a long theme @${w}`);
      // The staff card button itself stays inside its card.
      const btn = page.getByRole('button', { name: new RegExp(`^Тема: ${longTheme.slice(0, 20)}`) });
      const card = page.locator('[data-dialog-fallback]');
      const [bb, cb] = [await btn.boundingBox(), await card.boundingBox()];
      expect(bb!.x + bb!.width).toBeLessThanOrEqual(cb!.x + cb!.width + 1);
    }
  });
});

test.describe('Analytics', () => {
  test('U4/U5 scopes, date range, confidence badge and ticking requests of a suggested group', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await createSimilarGroup(page);
    await logout(page);
    await loginAs(page, ROLES.admin);
    await page.goto('/analytics');
    await expect(page.getByRole('heading', { level: 1, name: 'Аналитика' })).toBeVisible();
    await expect(page.getByTestId('analytics-range')).toContainText(/Период: \d{2}\.\d{2}\.\d{4} – \d{2}\.\d{2}\.\d{4}/);
    await page.getByRole('radio', { name: 'Всё время' }).click();
    await expect(page.getByTestId('analytics-range')).toContainText('всё время');

    await expect(page.getByText('Показатели за выбранный период')).toBeVisible();
    await expect(page.getByText('За выбранный период').first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Просрочено сейчас' })).toBeVisible();
    await expect(page.getByText('На сейчас, независимо от периода')).toBeVisible();

    const groups = page.getByRole('heading', { name: 'Повторяющиеся проблемы' });
    await expect(groups).toBeVisible();
    const first = page.locator('li').filter({ has: page.getByRole('button', { name: 'Создать тему' }) }).first();
    await expect(first.getByText(/Уверенность: (высокая|средняя|низкая)/)).toBeVisible();
    await expect(first.getByText('Проверьте состав группы перед созданием темы')).toBeVisible();
    const boxes = first.getByRole('checkbox');
    const n = await boxes.count();
    expect(n).toBeGreaterThanOrEqual(2);
    await expect(boxes.first()).toBeChecked();
    await expect(first.getByText(`Выбрано ${n} из ${n}`)).toBeVisible();
    // Unticking down to one request disables creation.
    for (let i = 1; i < n; i++) await boxes.nth(i).uncheck();
    await expect(first.getByText('Для темы нужно выбрать минимум 2 обращения')).toBeVisible();
    await expect(first.getByRole('button', { name: 'Создать тему' })).toBeDisabled();
  });

  test('A3/A4 mobile: bar labels wrap, "Создать тему" sits under the title, 44px rows', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await createSimilarGroup(page);
    await logout(page);
    await loginAs(page, ROLES.admin);
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/analytics');
    await expect(page.getByRole('heading', { name: 'Повторяющиеся проблемы' })).toBeVisible();
    await noHorizontalScroll(page, '/analytics @375');
    await expect(page.locator('a[href*="category="]').first()).toBeVisible();
    const heights = await page.locator('a[href*="category="]').evaluateAll((els) => els.map((e) => e.getBoundingClientRect().height));
    expect(heights.length).toBeGreaterThan(0);
    for (const h of heights) expect(h).toBeGreaterThanOrEqual(43.5);
    const item = page.locator('li').filter({ has: page.getByRole('button', { name: 'Создать тему' }) }).first();
    const title = await item.locator('p.font-semibold').boundingBox();
    const btn = await item.getByRole('button', { name: 'Создать тему' }).boundingBox();
    expect(btn!.y).toBeGreaterThan(title!.y + title!.height - 1);
    await expect(page.getByText('Таблица значений')).toBeVisible();
  });
});

test.describe('Admin routing', () => {
  test('U6 one unit, explicit headers, sorted rows and a banner for categories without a rule', async ({ page }) => {
    await loginAs(page, ROLES.admin);
    await page.goto('/admin');
    const table = page.getByRole('table');
    await expect(table).toBeVisible();
    await expect(table.getByRole('columnheader', { name: 'Первый ответ, ч' })).toBeVisible();
    await expect(table.getByRole('columnheader', { name: 'Решение, ч' })).toBeVisible();
    await expect(table.getByText(/дн\./)).toHaveCount(0);
    const names = await table.locator('tbody tr td:first-child').allTextContents();
    expect(names).toEqual([...names].sort((a, b) => a.localeCompare(b, 'ru')));
    await expect(page.getByTestId('no-rule-banner')).toContainText(/^Без правила: .+ — обращения попадут в «Не распределено»$/);
    await page.getByRole('button', { name: 'Добавить правило' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByLabel('Первый ответ, ч')).toBeVisible();
    await expect(dialog.getByLabel('Решение, ч')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('button', { name: 'Добавить правило' })).toBeFocused();
  });
});

test.describe('Mobile submit form', () => {
  test('U11 compact category picker keeps the first text field on screen; routing summary under the choice; invalid submit focuses the first field', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/new');
    const box = page.getByRole('combobox', { name: 'Категория' });
    await expect(box).toBeVisible();
    await noHorizontalScroll(page, '/new @375');
    // 18 categories in one compact field: the first text field is visible without scrolling.
    // Its top edge is above the sticky submit bar (bottom nav 64px + bar ~72px).
    const title = await page.getByLabel('Коротко о сути').boundingBox();
    expect(title!.y).toBeLessThan(812 - 64 - 72);

    await box.click();
    await page.getByRole('option').first().click();
    await expect(page.getByTestId('routing-compact')).toBeVisible();
    await expect(page.getByTestId('routing-compact')).toContainText(/Отдел «.+» · ответ до |Примет администратор/);

    // Invalid submit: validation stays inline and focus goes to the first invalid field (the title).
    await page.getByRole('button', { name: 'Отправить', exact: true }).click();
    await expect(page.getByText('Минимум 5 символов')).toBeVisible();
    await expect(page.getByLabel('Коротко о сути')).toBeFocused();
  });

  test('A6/A7 the anonymity row is a full-height hit area and the header logo link is 44x44', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/new');
    const logo = await page.getByRole('link', { name: 'Modus Voice' }).first().boundingBox();
    expect(logo!.width).toBeGreaterThanOrEqual(43.5);
    expect(logo!.height).toBeGreaterThanOrEqual(43.5);
    const sw = page.getByRole('switch', { name: 'Отправить анонимно' });
    await expect(sw).toHaveAttribute('aria-checked', 'false');
    // Clicking the description text (not the switch itself) toggles it: the label stretches over the whole row.
    // force: the stretched label deliberately sits above the text, which Playwright reports as "intercepts pointer events".
    const desc = page.getByText('Ваше имя не сохраняется — ни в базе данных', { exact: false });
    await desc.evaluate((el) => el.scrollIntoView({ block: 'center' }));
    await desc.click({ force: true });
    await expect(sw).toHaveAttribute('aria-checked', 'true');
  });
});

test.describe('Landmarks and ARIA', () => {
  test('B3-B6 heading order, labelled regions, file input label, live toasts', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await page.goto('/new');
    await expect(page.getByLabel('Файлы', { exact: true })).toHaveCount(1);
    // Toast viewport is a labelled region; toasts are polite live regions.
    await expect(page.getByRole('region', { name: /Уведомления/ })).toHaveCount(1);
    await page.goto('/my');
    const levels = await page.evaluate(() => [...document.querySelectorAll('main h1, main h2, main h3, main h4')].map((h) => Number(h.tagName[1])));
    let prev = 0;
    for (const l of levels) {
      expect(l - prev, `heading levels ${levels.join(',')}`).toBeLessThanOrEqual(1);
      prev = l;
    }
  });
});
