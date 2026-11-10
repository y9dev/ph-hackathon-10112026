import { expect, test } from '@playwright/test';
import { loginAs, logout, openFromInbox, ROLES, submitRequest, uniqueTitle } from './helpers';

test.describe('Journeys J1–J4', () => {
  test('J1 submit: employee sends a request and gets a number, owner and deadline @mobile', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await page.goto('/new');

    // Validation is inline and keeps input.
    await page.getByRole('button', { name: 'Отправить', exact: true }).click();
    await expect(page.getByText('Выберите категорию', { exact: true })).toBeVisible();
    await expect(page.getByText('Минимум 5 символов')).toBeVisible();

    // Routing preview appears before submit.
    await page.getByRole('radiogroup', { name: 'Категория' }).getByRole('radio').first().click();
    await expect(page.getByText(/Первый ответ — до/).filter({ visible: true }).first()).toBeVisible();

    const title = uniqueTitle('Не печатает принтер');
    await page.getByLabel('Коротко о сути').fill(title);
    await page.getByLabel('Подробное описание').fill('Общий принтер на втором этаже печатает с полосами уже неделю.');
    await page.getByRole('switch', { name: 'Отправить анонимно' }).click();
    await page.getByRole('button', { name: 'Отправить', exact: true }).click();

    await expect(page.getByRole('heading', { name: 'Спасибо! Обращение принято' })).toBeVisible();
    await expect(page.getByTestId('request-number')).toHaveText(/MV-\d+/);
    await expect(page.getByText('Первый ответ до')).toBeVisible();
    await expect(page.getByText('Отправлено анонимно — ваше имя скрыто.')).toBeVisible();

    await page.getByRole('link', { name: 'Открыть обращение' }).click();
    await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
    await expect(page.getByRole('main').getByText('Новое', { exact: true }).first()).toBeVisible();
  });

  test('J2 track: employee finds the request in "Мои обращения" with its timeline', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    const c = await submitRequest(page, uniqueTitle('Медленный VPN на объекте'));
    await page.getByRole('navigation', { name: 'Основная навигация' }).first().getByRole('link', { name: 'Мои обращения' }).click();
    await expect(page.getByRole('heading', { level: 1, name: 'Мои обращения' })).toBeVisible();
    await page.getByRole('link', { name: new RegExp(c.title) }).click();
    await expect(page.getByRole('heading', { level: 1, name: c.title })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'История' })).toBeVisible();
    await expect(page.getByText('Обращение создано')).toBeVisible();
    await expect(page.getByText(c.number).first()).toBeVisible();
  });

  test('J3 review: staff takes the request, cannot close without result, closes with result; author sees it', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    const c = await submitRequest(page, uniqueTitle('Нет зимней резины'));
    await logout(page);

    await loginAs(page, ROLES.admin);
    await openFromInbox(page, c);
    await page.getByRole('button', { name: 'Взять в работу' }).click();
    await expect(page.getByText('Статус: На рассмотрении. Автор уведомлён').first()).toBeVisible();

    await page.getByRole('button', { name: 'Принять' }).click();
    await expect(page.getByText('Статус: Принято. Автор уведомлён').first()).toBeVisible();

    await page.getByRole('button', { name: 'Отметить реализованным' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('button', { name: 'Отметить реализованным' }).click();
    await expect(dialog.getByText('Без решения закрыть нельзя — опишите результат')).toBeVisible();
    await dialog.getByLabel('Решение для автора').fill('Шины заменены на зимние, машина готова к выездам.');
    await dialog.getByRole('button', { name: 'Отметить реализованным' }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByRole('heading', { name: 'Решение: Реализовано' })).toBeVisible();
    await logout(page);

    await loginAs(page, ROLES.employee);
    await page.goto('/my?filter=closed');
    await page.getByRole('link', { name: new RegExp(c.title) }).click();
    await expect(page.getByText('Шины заменены на зимние, машина готова к выездам.').first()).toBeVisible();
  });

  test('J4 clarify: staff asks a question, author answers, request returns to review', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    const c = await submitRequest(page, uniqueTitle('Нет чек-листа выезда'));
    await logout(page);

    await loginAs(page, ROLES.admin);
    await openFromInbox(page, c);
    await page.getByRole('button', { name: 'Запросить уточнение' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('Вопрос автору').fill('На каких объектах это происходило чаще всего?');
    await dialog.getByRole('button', { name: 'Отправить' }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByText('Нужно уточнение').first()).toBeVisible();
    await logout(page);

    await loginAs(page, ROLES.employee);
    await page.goto('/my');
    await expect(page.getByRole('button', { name: /Нужен ответ/ })).toBeVisible();
    await page.getByRole('link', { name: new RegExp(c.title) }).click();
    await expect(page.getByText('Нужен ваш ответ').first()).toBeVisible();
    await page.getByRole('button', { name: 'Ответить' }).click();
    await page.getByLabel('Текст сообщения').fill('Чаще всего в Экибастузе: забываем удлинители и бланки протоколов.');
    await page.getByRole('button', { name: 'Отправить' }).click();
    await expect(page.getByText('Ответ отправлен').first()).toBeVisible();
    await expect(page.getByText('На рассмотрении').first()).toBeVisible();
  });

  test('guards: employee cannot open staff pages; unknown request shows not found', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await page.goto('/inbox');
    await expect(page.getByRole('heading', { name: 'Нет доступа' })).toBeVisible();
    await page.goto('/requests/999999');
    await expect(page.getByRole('heading', { name: 'Обращение не найдено' })).toBeVisible();
  });
});
