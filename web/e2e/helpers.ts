import { expect, type Page, type TestInfo } from '@playwright/test';

/**
 * Demo accounts (CR1: users are the dataset's conditional roles). Buttons are labelled
 * "Войти как <роль-должность> (<роль в системе>)"; the same six accounts exist in the mock and the backend seed.
 */
export const ROLES = {
  /** Сотрудник закупок — author of A-001, A-002, A-004, A-020 */
  employee: /^Войти как Сотрудник закупок/,
  /** Менеджер по продажам — another author */
  sales: /^Войти как Менеджер по продажам/,
  /** Руководитель закупок — owner of «Межотдельное взаимодействие» (first category) and «Каталог и данные» */
  purchasingHead: /^Войти как Руководитель закупок/,
  /** IT-специалист — owner of «Интеграции и IT» and «Коммуникации» */
  it: /^Войти как IT-специалист/,
  /** Руководитель проектирования — manager of another department (CR6 observer demo) */
  observer: /^Войти как Руководитель проектирования/,
  admin: /^Войти как Администратор системы/,
};

/** Mock runs use the Vite mock server (5174); real-backend runs point E2E_BASE_URL elsewhere. */
export function isMock(info: TestInfo): boolean {
  return String(info.project.use.baseURL ?? '').includes(':5174');
}

export async function loginAs(page: Page, role: RegExp) {
  await page.goto('/login');
  await page.getByRole('button', { name: role }).first().click();
  await expect(page.getByRole('navigation', { name: 'Основная навигация' }).first()).toBeVisible();
}

export async function logout(page: Page) {
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await page.getByRole('menuitem', { name: 'Выйти' }).click();
  await expect(page).toHaveURL(/\/login/);
}

export interface Created {
  number: string;
  title: string;
}

/** Picks a category in the searchable picker (first match of `query`, or the first category). */
export async function pickCategory(page: Page, query?: string) {
  const box = page.getByRole('combobox', { name: 'Категория' });
  await box.click();
  if (query) await box.fill(query);
  await page.getByRole('listbox', { name: 'Список категорий' }).getByRole('option').first().click();
}

/** J1 core: fills the submit form with the minimum and returns the tracking number. */
export async function submitRequest(page: Page, title: string, body = 'Проблема повторяется каждый день, работа стоит около часа.'): Promise<Created> {
  await page.goto('/new');
  await expect(page.getByRole('heading', { level: 1, name: 'Новое обращение' })).toBeVisible();
  await pickCategory(page);
  await page.getByLabel('Коротко о сути').fill(title);
  await page.getByLabel('Что произошло').fill(body);
  await page.getByRole('button', { name: 'Отправить', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Спасибо! Обращение принято' })).toBeVisible();
  const number = (await page.getByTestId('request-number').textContent())!.trim();
  expect(number).toMatch(/^MV-\d+$/);
  return { number, title };
}

/** Opens a request from the staff inbox using search by number (admin: all requests; managers: own inbox). */
export async function openFromInbox(page: Page, c: Created) {
  await page.goto('/inbox?tab=all');
  await page.getByLabel('Поиск', { exact: true }).fill(c.number);
  await expect(page).toHaveURL(new RegExp(`q=${c.number}`));
  await page.getByRole('link', { name: c.title }).first().click();
  await expect(page.getByRole('heading', { level: 1, name: c.title })).toBeVisible();
}

/**
 * Stage moves open a confirm dialog with an optional one-line "Сообщение автору".
 * Clicks the staff action, optionally types a message and confirms with the same-named dialog button.
 */
export async function moveStage(page: Page, label: string, message?: string) {
  await page.getByRole('button', { name: label, exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('heading', { name: label })).toBeVisible();
  await expect(dialog.getByText('Автор получит уведомление')).toBeVisible();
  if (message) await dialog.getByLabel('Сообщение автору').fill(message);
  await dialog.getByRole('button', { name: label, exact: true }).click();
  await expect(dialog).toBeHidden();
}

export function uniqueTitle(prefix: string) {
  return `${prefix} ${Date.now().toString(36).slice(-5)}`;
}

export async function noHorizontalScroll(page: Page, label: string) {
  // Polls briefly: after a viewport change the sidebar padding transition (200 ms) settles first.
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), {
      message: `${label} horizontal overflow`,
      timeout: 2000,
    })
    .toBeLessThanOrEqual(0);
}

/**
 * The dataset has no near-duplicate requests, so suggested groups are empty after seeding.
 * Three near-identical requests in one category make a group (backend MinGroupSize = 3; the mock mirrors it).
 */
export async function createSimilarGroup(page: Page) {
  const tag = Date.now().toString(36).slice(-4);
  const body = `Сканер штрихкодов на складе ${tag} зависает при приёмке поставки, приходится перезагружать терминал вручную.`;
  await submitRequest(page, `Зависает сканер штрихкодов ${tag}`, body);
  await submitRequest(page, `Сканер штрихкодов зависает ${tag}`, body);
  await submitRequest(page, `Снова зависает сканер штрихкодов ${tag}`, body);
}
