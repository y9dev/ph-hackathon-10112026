// Evidence screenshots of every main screen at 1280 and 375 px.
// Run: SCREENS=1 npx playwright test e2e/screens.spec.ts --project=desktop
import { expect, test, type Page } from '@playwright/test';
import { isMock, loginAs, logout, pickCategory, ROLES, submitRequest } from './helpers';

const OUT = '../docs/evidence/screens';
const WIDTHS = [
  { w: 1280, h: 800, tag: '1280' },
  { w: 375, h: 812, tag: '375' },
];
const DEMO_TOKEN_A005 = 'DEM0-A005-7K3Q-9XWB-2HRT-M4ZP-C8NV';

test.skip(!process.env.SCREENS, 'set SCREENS=1 to capture evidence screenshots');
test.describe.configure({ mode: 'serial' });

async function shoot(page: Page, name: string, fullPage = true) {
  for (const { w, h, tag } of WIDTHS) {
    await page.setViewportSize({ width: w, height: h });
    await page.waitForTimeout(400);
    // Fail loudly on horizontal overflow (no horizontal scroll at 375–1280).
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow, `${name} @${tag} horizontal overflow`).toBeLessThanOrEqual(0);
    await page.screenshot({ path: `${OUT}/${name}-${tag}.png`, fullPage });
  }
  await page.setViewportSize({ width: 1280, height: 800 });
}

test('employee screens', async ({ page }) => {
  await page.goto('/login');
  await expect(page.getByRole('heading', { name: 'Демо-аккаунты' })).toBeVisible();
  await shoot(page, '01-login');

  await loginAs(page, ROLES.employee);
  await page.goto('/my');
  await expect(page.getByRole('heading', { level: 1, name: 'Мои обращения' })).toBeVisible();
  await expect(page.getByTestId('device-anon')).toBeVisible();
  await shoot(page, '02-my-requests');

  await page.goto('/new');
  await page.getByRole('combobox', { name: 'Категория' }).click();
  await page.getByRole('combobox', { name: 'Категория' }).fill('док');
  await shoot(page, '17-category-picker', false);
  await page.keyboard.press('Enter');
  await page.getByLabel('Коротко о сути').fill('В заявках не хватает параметров оборудования');
  await page.getByLabel('Что произошло').fill('В заявках на оборудование нет назначения изделия и параметров.');
  await expect(page.getByText(/Первый ответ — до/).filter({ visible: true }).first()).toBeVisible();
  await shoot(page, '03-submit');

  await submitRequest(page, 'Нет единого шаблона заявки на оборудование');
  await shoot(page, '04-success');

  await page.goto('/requests/1');
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await shoot(page, '05-detail-author');

  await page.goto('/requests/4');
  await expect(page.getByRole('list', { name: 'Этапы обращения' })).toContainText('Завершено');
  await shoot(page, '14-detail-author-lifecycle');

  await page.goto('/notifications');
  await expect(page.getByRole('heading', { level: 1, name: 'Уведомления' })).toBeVisible();
  await shoot(page, '09-notifications');

  // CR2: anonymous submit → one-time token screen.
  await page.goto('/new');
  await pickCategory(page);
  await page.getByLabel('Коротко о сути').fill('Анонимно: статус пакета документов');
  await page.getByLabel('Что произошло').fill('Хотелось бы видеть, на каком этапе находится пакет документов.');
  await page.getByRole('switch', { name: 'Отправить анонимно' }).click();
  await page.getByRole('button', { name: 'Отправить', exact: true }).click();
  await expect(page.getByTestId('anonymous-token')).toBeVisible();
  await shoot(page, '18-anonymous-token');
  await page.getByLabel('Я сохранил токен').check();
  await page.getByRole('link', { name: 'Готово' }).click();

  // CR4: Telegram page (bot enabled in the mock / with a fake Bot API on the real backend).
  await page.goto('/telegram');
  if (await page.getByTestId('telegram-code').isVisible({ timeout: 5000 }).catch(() => false)) await shoot(page, '20-telegram');
  await logout(page);

  // CR2: public lookup by token (A-005 asks the author a question).
  await page.goto('/anonymous');
  await shoot(page, '19-anonymous-lookup');
  await page.getByLabel('Токен доступа').fill(DEMO_TOKEN_A005);
  await page.getByRole('button', { name: 'Открыть обращение' }).click();
  await expect(page.getByText('Нужен ваш ответ').first()).toBeVisible();
  await shoot(page, '21-anonymous-request');
});

test('staff screens', async ({ page }) => {
  await loginAs(page, ROLES.admin);
  await page.goto('/inbox');
  await expect(page.getByRole('tab', { name: /Требуют реакции/ })).toBeVisible();
  await expect(page.getByText(/Просрочено на/).first()).toBeVisible();
  await shoot(page, '06-inbox');

  await page.goto('/requests/7');
  await expect(page.getByRole('heading', { name: 'Работа с обращением' })).toBeVisible();
  await shoot(page, '07-detail-staff');

  await page.goto('/requests/3');
  await expect(page.getByTestId('latest-report')).toBeVisible();
  await shoot(page, '15-detail-staff-implementation-report');

  await page.goto('/analytics');
  await expect(page.getByTestId('kpi-on-time')).toBeVisible();
  await expect(page.getByTestId('by-type')).toBeVisible();
  await shoot(page, '08-analytics');

  for (const [tab, name] of [
    ['', '10-admin-routing'],
    ['?tab=categories', '11-admin-categories'],
    ['?tab=users', '12-admin-users'],
  ] as const) {
    await page.goto(`/admin${tab}`);
    await expect(page.getByRole('table')).toBeVisible();
    await shoot(page, name);
  }

  await page.goto('/requests/6');
  await page.getByRole('button', { name: 'Отклонить' }).click();
  await expect(page.getByRole('dialog').getByRole('heading', { name: 'Отклонить обращение' })).toBeVisible();
  await page.getByRole('dialog').getByRole('button', { name: 'Отклонить' }).click();
  await expect(page.getByText('Без решения закрыть нельзя').first()).toBeVisible();
  await page.screenshot({ path: `${OUT}/13-close-dialog-validation-1280.png` });
  await page.keyboard.press('Escape');

  // CR3: redesigned theme dialog.
  await page.goto('/requests/2');
  await page.getByRole('button', { name: /Добавить в тему|^Тема:/ }).click();
  await expect(page.getByRole('dialog').getByRole('radio', { name: 'Новая тема' })).toBeVisible();
  await shoot(page, '24-theme-dialog', false);
});

test('observer screens (CR6)', async ({ page }) => {
  await loginAs(page, ROLES.observer);
  await page.goto('/inbox?view=all&tab=all');
  await expect(page.getByRole('radio', { name: 'Все обращения' })).toHaveAttribute('aria-checked', 'true');
  await expect(page.getByRole('row').filter({ hasText: 'MV-0001' })).toBeVisible();
  await shoot(page, '22-inbox-all-observer');
  await page.goto('/requests/1');
  await expect(page.getByTestId('observer-banner')).toBeVisible();
  await shoot(page, '23-detail-observer');
});

test('long theme title does not overflow request detail (1280 and 375)', async ({ page }, info) => {
  test.skip(!isMock(info), 'mutates the shared real database');
  await loginAs(page, ROLES.admin);
  await page.goto('/requests/7');
  await expect(page.getByRole('heading', { name: 'Работа с обращением' })).toBeVisible();
  const longTheme = `Тема${'Оченьдлинноеназваниебезпробелов'.repeat(4)}`.slice(0, 140);
  await page.getByRole('button', { name: /Добавить в тему|^Тема:/ }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('radio', { name: 'Новая тема' }).click();
  await dialog.getByLabel('Название новой темы').fill(longTheme);
  await dialog.getByRole('button', { name: 'Добавить', exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole('button', { name: /^Тема: Тема/ })).toBeVisible();
  await shoot(page, '16-detail-staff-long-theme');
});
