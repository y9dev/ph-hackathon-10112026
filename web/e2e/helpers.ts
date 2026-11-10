import { expect, type Page } from '@playwright/test';

/** Demo account buttons are labelled "Войти как <роль> — <должность>, <имя>". First match per role is used. */
export const ROLES = {
  employee: /^Войти как Сотрудник —/,
  admin: /^Войти как Администратор —/,
};

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

/** J1 core: fills the submit form with the minimum and returns the tracking number. */
export async function submitRequest(page: Page, title: string): Promise<Created> {
  await page.goto('/new');
  await expect(page.getByRole('heading', { level: 1, name: 'Новое обращение' })).toBeVisible();
  await page.getByRole('radiogroup', { name: 'Категория' }).getByRole('radio').first().click();
  await page.getByLabel('Коротко о сути').fill(title);
  await page.getByLabel('Подробное описание').fill('Проблема повторяется каждый день на объекте, работа стоит около часа.');
  await page.getByRole('button', { name: 'Отправить', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Спасибо! Обращение принято' })).toBeVisible();
  const number = (await page.getByTestId('request-number').textContent())!.trim();
  expect(number).toMatch(/^MV-\d+$/);
  return { number, title };
}

/** Opens a request from the staff inbox using search by number. */
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
