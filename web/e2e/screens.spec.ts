// Evidence screenshots of every main screen at 1280 and 375 px.
// Run: SCREENS=1 npx playwright test e2e/screens.spec.ts --project=desktop
import { expect, test, type Page } from '@playwright/test';
import { loginAs, logout, ROLES, submitRequest } from './helpers';

const OUT = '../docs/evidence/screens';
const WIDTHS = [
  { w: 1280, h: 800, tag: '1280' },
  { w: 375, h: 812, tag: '375' },
];

test.skip(!process.env.SCREENS, 'set SCREENS=1 to capture evidence screenshots');
test.describe.configure({ mode: 'serial' });

async function shoot(page: Page, name: string) {
  for (const { w, h, tag } of WIDTHS) {
    await page.setViewportSize({ width: w, height: h });
    await page.waitForTimeout(400);
    // Fail loudly on horizontal overflow (no horizontal scroll at 375–1280).
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow, `${name} @${tag} horizontal overflow`).toBeLessThanOrEqual(0);
    await page.screenshot({ path: `${OUT}/${name}-${tag}.png`, fullPage: true });
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
  await expect(page.getByText('Нужен ваш ответ').first()).toBeVisible();
  await shoot(page, '02-my-requests');

  await page.goto('/new');
  await page.getByRole('radiogroup', { name: 'Категория' }).getByRole('radio').first().click();
  await page.getByLabel('Коротко о сути').fill('Не подключается VPN на объекте');
  await page.getByLabel('Подробное описание').fill('VPN на ноутбуке не подключается через мобильный интернет.');
  await expect(page.getByText(/Первый ответ — до/).filter({ visible: true }).first()).toBeVisible();
  await shoot(page, '03-submit');

  await submitRequest(page, 'Ноутбук не видит принтер на объекте');
  await shoot(page, '04-success');

  await page.goto('/requests/10');
  await expect(page.getByText('Нужен ваш ответ').first()).toBeVisible();
  await shoot(page, '05-detail-author');

  await page.goto('/requests/12');
  await expect(page.getByRole('list', { name: 'Этапы обращения' })).toContainText('Завершено');
  await shoot(page, '14-detail-author-lifecycle');

  await page.goto('/notifications');
  await expect(page.getByRole('heading', { level: 1, name: 'Уведомления' })).toBeVisible();
  await shoot(page, '09-notifications');
  await logout(page);
});

test('staff screens', async ({ page }) => {
  await loginAs(page, ROLES.admin);
  await page.goto('/inbox');
  await expect(page.getByRole('tab', { name: /Требуют реакции/ })).toBeVisible();
  await expect(page.getByText(/Просрочено на/).first()).toBeVisible();
  await shoot(page, '06-inbox');

  await page.goto('/requests/8');
  await expect(page.getByRole('heading', { name: 'Работа с обращением' })).toBeVisible();
  await shoot(page, '07-detail-staff');

  await page.goto('/requests/4');
  await expect(page.getByTestId('latest-report')).toBeVisible();
  await shoot(page, '15-detail-staff-implementation-report');

  await page.goto('/analytics');
  await expect(page.getByRole('heading', { name: 'Повторяющиеся проблемы' })).toBeVisible();
  await expect(page.getByText('Всего обращений')).toBeVisible();
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
  await page.getByRole('dialog').getByRole('button', { name: 'Отклонить' }).click();
  await expect(page.getByText('Без решения закрыть нельзя').first()).toBeVisible();
  await page.screenshot({ path: `${OUT}/13-close-dialog-validation-1280.png` });
});
