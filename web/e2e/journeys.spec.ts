import { expect, test } from '@playwright/test';
import { loginAs, logout, moveStage, openFromInbox, pickCategory, ROLES, submitRequest, uniqueTitle } from './helpers';

test.describe('Journeys J1–J4', () => {
  test('J1 submit: employee sends a request and gets a number, owner and deadline @mobile', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await page.goto('/new');

    // Validation is inline and keeps input.
    await page.getByRole('button', { name: 'Отправить', exact: true }).click();
    await expect(page.getByText('Выберите категорию', { exact: true })).toBeVisible();
    await expect(page.getByText('Минимум 5 символов')).toBeVisible();

    // Routing preview appears before submit.
    await pickCategory(page);
    await expect(page.getByText(/Первый ответ — до/).filter({ visible: true }).first()).toBeVisible();

    const title = uniqueTitle('В заявке нет параметров');
    await page.getByLabel('Коротко о сути').fill(title);
    await page.getByLabel('Что произошло').fill('В заявке на оборудование нет назначения и параметров, приходится уточнять.');
    // Priority is optional, "Средний" by default.
    await expect(page.getByRole('radio', { name: 'Средний' })).toHaveAttribute('aria-checked', 'true');
    await page.getByRole('radio', { name: 'Высокий' }).click();
    await page.getByRole('button', { name: 'Отправить', exact: true }).click();

    await expect(page.getByRole('heading', { name: 'Спасибо! Обращение принято' })).toBeVisible();
    await expect(page.getByTestId('request-number')).toHaveText(/MV-\d+/);
    await expect(page.getByText('Первый ответ до')).toBeVisible();

    await page.getByRole('link', { name: 'Открыть обращение' }).click();
    await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
    await expect(page.getByRole('main').getByText('Новое', { exact: true }).first()).toBeVisible();
    await expect(page.getByTestId('priority-chip').first()).toHaveText('Приоритет: Высокий');
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

  test('J3 lifecycle: take into work, pass to decision, start implementation, report, complete; author sees stepper, report and result', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    const c = await submitRequest(page, uniqueTitle('Нет зимней резины'));
    await logout(page);

    await loginAs(page, ROLES.admin);
    await openFromInbox(page, c);
    const stepper = page.getByRole('list', { name: 'Этапы обращения' });
    await expect(stepper.getByRole('listitem').first()).toHaveAttribute('aria-current', 'step');

    await moveStage(page, 'Взять в проработку', 'Принято в работу, вернусь с вопросами.');
    await expect(page.getByText('Статус: В проработке. Автор уведомлён').first()).toBeVisible();
    // The optional message is posted as a normal comment after the status change.
    await expect(page.getByText('Принято в работу, вернусь с вопросами.')).toBeVisible();

    await moveStage(page, 'Перейти к принятию решения');
    await expect(page.getByText('Статус: Принятие решения. Автор уведомлён').first()).toBeVisible();
    await expect(stepper.getByRole('listitem').nth(1)).toHaveAttribute('aria-current', 'step');
    await expect(page.getByRole('button', { name: 'Вернуть на доработку' })).toBeVisible();

    await moveStage(page, 'Начать реализацию');
    await expect(page.getByText('Статус: Реализация. Автор уведомлён').first()).toBeVisible();
    await expect(stepper.getByRole('listitem').nth(2)).toHaveAttribute('aria-current', 'step');

    // In implementation the "Отчёт" tab is preselected.
    await expect(page.getByRole('radio', { name: /Отчёт/ })).toHaveAttribute('aria-checked', 'true');
    await page.getByLabel('Текст сообщения').fill('Шины заказаны, установка в пятницу.');
    await page.getByRole('button', { name: 'Отправить' }).click();
    await expect(page.getByText('Отчёт отправлен, автор уведомлён').first()).toBeVisible();
    await expect(page.getByTestId('latest-report')).toContainText('Шины заказаны, установка в пятницу.');

    await page.getByRole('button', { name: 'Завершить с результатом' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByRole('heading', { name: 'Завершить обращение' })).toBeVisible();
    await expect(dialog.getByText('Опишите, что именно сделано')).toBeVisible();
    await expect(dialog.getByText('(обязательно)')).toBeVisible();
    await dialog.getByRole('button', { name: 'Завершить с результатом' }).click();
    await expect(dialog.getByText('Без решения закрыть нельзя — опишите результат')).toHaveCount(1);
    await expect(dialog.getByLabel('Решение для автора')).toBeFocused();
    await dialog.getByLabel('Решение для автора').fill('Шины заменены на зимние, машина готова к выездам.');
    await dialog.getByRole('button', { name: 'Завершить с результатом' }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByRole('heading', { name: 'Решение: Реализовано' })).toBeVisible();
    await logout(page);

    await loginAs(page, ROLES.employee);
    await page.goto('/my?filter=closed');
    await page.getByRole('link', { name: new RegExp(c.title) }).click();
    await expect(page.getByRole('heading', { level: 1, name: c.title })).toBeVisible();
    await expect(page.getByRole('list', { name: 'Этапы обращения' })).toContainText('Завершено');
    await expect(page.getByTestId('latest-report')).toContainText('Шины заказаны, установка в пятницу.');
    await expect(page.getByText('Шины заменены на зимние, машина готова к выездам.').first()).toBeVisible();
  });

  test('J5 close: staff closes a request with a result; author sees "Закрыто"', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    const c = await submitRequest(page, uniqueTitle('Вопрос про отпуск'));
    await logout(page);

    await loginAs(page, ROLES.admin);
    await openFromInbox(page, c);
    await page.getByRole('button', { name: 'Закрыть без реализации' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByRole('heading', { name: 'Закрыть без реализации' })).toBeVisible();
    await expect(dialog.getByText('Объясните, почему реализация не нужна или что было сделано иначе')).toBeVisible();
    await dialog.getByRole('button', { name: 'Закрыть без реализации' }).click();
    await expect(dialog.getByText('Без решения закрыть нельзя — опишите результат')).toBeVisible();
    await dialog.getByLabel('Решение для автора').fill('Ответили лично, дополнительных действий не требуется.');
    await dialog.getByRole('button', { name: 'Закрыть без реализации' }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByRole('heading', { name: 'Решение: Закрыто' })).toBeVisible();
    await logout(page);

    await loginAs(page, ROLES.employee);
    await page.goto('/my?filter=closed');
    await page.getByRole('link', { name: new RegExp(c.title) }).click();
    await expect(page.getByRole('heading', { level: 1, name: c.title })).toBeVisible();
    await expect(page.getByRole('list', { name: 'Этапы обращения' })).toContainText('Закрыто');
  });

  test('J4 clarify: staff asks a question, author answers, request returns to review', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    const c = await submitRequest(page, uniqueTitle('Нет чек-листа выезда'));
    await logout(page);

    await loginAs(page, ROLES.admin);
    await openFromInbox(page, c);
    // "Запросить уточнение" selects the "Вопрос" tab of the composer and focuses its field.
    await page.getByRole('button', { name: 'Запросить уточнение' }).click();
    await expect(page.getByRole('radio', { name: /Вопрос/ })).toHaveAttribute('aria-checked', 'true');
    await expect(page.getByLabel('Текст сообщения')).toBeFocused();
    await page.getByLabel('Текст сообщения').fill('На каких объектах это происходило чаще всего?');
    await page.getByRole('button', { name: 'Отправить' }).click();
    await expect(page.getByText('Вопрос отправлен, автор уведомлён').first()).toBeVisible();
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
    await expect(page.getByText('В проработке').first()).toBeVisible();
  });

  test('guards: employee cannot open staff pages; unknown request shows not found', async ({ page }) => {
    await loginAs(page, ROLES.employee);
    await page.goto('/inbox');
    await expect(page.getByRole('heading', { name: 'Нет доступа' })).toBeVisible();
    await page.goto('/requests/999999');
    await expect(page.getByRole('heading', { name: 'Обращение не найдено' })).toBeVisible();
  });
});
