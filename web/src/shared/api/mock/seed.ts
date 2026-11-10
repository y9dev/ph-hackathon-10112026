// Synthetic Russian demo data for the in-browser mock. Times are relative to "now"
// so SLA states (ok / soon / overdue / paused) look realistic whenever it runs.
import type { CommentKind, RequestType, Status } from '../types';
import type { MComment, MEvent, MockDb, MNotification, MRequest, MRule, MUser } from './model';

export const DEMO_PASSWORD = 'demo12345';

const H = 3_600_000;
const iso = (t: number) => new Date(t).toISOString();

export const users: MUser[] = [
  { id: 1, name: 'Тимур Ахметов', email: 'timur@modus.demo', password: DEMO_PASSWORD, role: 'employee', department_id: 2, active: true, title: 'Сотрудник' },
  { id: 2, name: 'Алексей Морозов', email: 'it.head@modus.demo', password: DEMO_PASSWORD, role: 'manager', department_id: 1, active: true, title: 'Руководитель IT' },
  { id: 3, name: 'Ирина Ковалёва', email: 'service.head@modus.demo', password: DEMO_PASSWORD, role: 'manager', department_id: 2, active: true, title: 'Руководитель сервиса' },
  { id: 4, name: 'Виктор Ланге', email: 'admin@modus.demo', password: DEMO_PASSWORD, role: 'admin', department_id: 6, active: true, title: 'Администратор' },
  { id: 5, name: 'Дмитрий Орлов', email: 'supply.head@modus.demo', password: DEMO_PASSWORD, role: 'manager', department_id: 3, active: true, title: 'Руководитель закупок' },
  { id: 6, name: 'Наталья Белова', email: 'sales.head@modus.demo', password: DEMO_PASSWORD, role: 'manager', department_id: 4, active: true, title: 'Руководитель продаж' },
  { id: 7, name: 'Сергей Ким', email: 'design.head@modus.demo', password: DEMO_PASSWORD, role: 'manager', department_id: 5, active: true, title: 'Главный инженер проекта' },
  { id: 8, name: 'Ольга Савченко', email: 'hr@modus.demo', password: DEMO_PASSWORD, role: 'manager', department_id: 6, active: true, title: 'Руководитель администрации' },
  { id: 9, name: 'Павел Широков', email: 'p.shirokov@modus.demo', password: DEMO_PASSWORD, role: 'manager', department_id: 1, active: true, title: 'Системный инженер' },
  { id: 10, name: 'Мария Лебедева', email: 'm.lebedeva@modus.demo', password: DEMO_PASSWORD, role: 'employee', department_id: 4, active: true, title: 'Менеджер по продажам' },
  { id: 11, name: 'Ержан Нургалиев', email: 'e.nurgaliev@modus.demo', password: DEMO_PASSWORD, role: 'employee', department_id: 5, active: true, title: 'Инженер-проектировщик' },
  { id: 12, name: 'Анна Петрова', email: 'a.petrova@modus.demo', password: DEMO_PASSWORD, role: 'employee', department_id: 3, active: true, title: 'Специалист по закупкам' },
  { id: 13, name: 'Руслан Жакупов', email: 'r.zhakupov@modus.demo', password: DEMO_PASSWORD, role: 'employee', department_id: 2, active: true, title: 'Инженер ПНР' },
  { id: 14, name: 'Елена Гришина', email: 'e.grishina@modus.demo', password: DEMO_PASSWORD, role: 'employee', department_id: 6, active: false, title: 'Офис-менеджер' },
];

export const departments = [
  { id: 1, name: 'IT', head_user_id: 2 },
  { id: 2, name: 'Сервис и пусконаладка', head_user_id: 3 },
  { id: 3, name: 'Закупки и логистика', head_user_id: 5 },
  { id: 4, name: 'Продажи', head_user_id: 6 },
  { id: 5, name: 'Проектирование', head_user_id: 7 },
  { id: 6, name: 'Администрация', head_user_id: 8 },
];

export const categories = [
  { id: 1, name: 'IT и связь', description: 'Компьютеры, программы, доступы, VPN, телефония', icon: 'monitor', sort: 10, active: true },
  { id: 2, name: 'Работа на объектах', description: 'Пусконаладка, сервис, инструмент, регламенты выезда', icon: 'wrench', sort: 20, active: true },
  { id: 3, name: 'Закупки и поставки', description: 'Сроки поставок, склад, документы поставщиков', icon: 'truck', sort: 30, active: true },
  { id: 4, name: 'Продажи и клиенты', description: 'Коммерческие предложения, CRM, работа с заказчиками', icon: 'users', sort: 40, active: true },
  { id: 5, name: 'Проектирование', description: 'Чертежи, программы для проектирования, согласования', icon: 'pen-tool', sort: 50, active: true },
  { id: 6, name: 'Охрана труда', description: 'Безопасность на объектах и в офисе, спецодежда, СИЗ', icon: 'hard-hat', sort: 60, active: true },
  { id: 7, name: 'Офис и быт', description: 'Рабочие места, кухня, парковка, уборка', icon: 'building-2', sort: 70, active: true },
  { id: 8, name: 'Процессы и управление', description: 'Согласования, отчётность, взаимодействие отделов', icon: 'lightbulb', sort: 80, active: true },
  { id: 9, name: 'Обучение', description: 'Курсы, наставничество, аттестация', icon: 'file-text', sort: 90, active: false },
];

export const rules: MRule[] = [
  { id: 1, category_id: 1, request_type: null, department_id: 1, assignee_user_id: 9, first_response_hours: 8, resolution_hours: 72, active: true },
  { id: 2, category_id: 2, request_type: null, department_id: 2, assignee_user_id: null, first_response_hours: 24, resolution_hours: 120, active: true },
  { id: 3, category_id: 3, request_type: null, department_id: 3, assignee_user_id: null, first_response_hours: 24, resolution_hours: 168, active: true },
  { id: 4, category_id: 4, request_type: null, department_id: 4, assignee_user_id: null, first_response_hours: 24, resolution_hours: 168, active: true },
  { id: 5, category_id: 5, request_type: null, department_id: 5, assignee_user_id: null, first_response_hours: 24, resolution_hours: 168, active: true },
  { id: 6, category_id: 6, request_type: 'problem', department_id: 2, assignee_user_id: 3, first_response_hours: 4, resolution_hours: 48, active: true },
  { id: 7, category_id: 6, request_type: 'idea', department_id: 6, assignee_user_id: null, first_response_hours: 24, resolution_hours: 168, active: true },
  { id: 8, category_id: 7, request_type: null, department_id: 6, assignee_user_id: null, first_response_hours: 24, resolution_hours: 120, active: true },
  { id: 9, category_id: 2, request_type: 'idea', department_id: 2, assignee_user_id: 3, first_response_hours: 48, resolution_hours: 240, active: false },
];

interface SeedComment {
  by: number;
  kind: CommentKind;
  body: string;
  /** Hours after creation. */
  at: number;
}
interface SeedRequest {
  cat: number;
  type: RequestType;
  title: string;
  body: string;
  sol?: string;
  author: number;
  anon?: boolean;
  status: Status;
  /** Hours ago. */
  created: number;
  /** Hours after creation of the first staff reaction. */
  respond?: number;
  /** Hours after creation the request was closed. */
  close?: number;
  result?: string;
  dupOf?: number;
  comments?: SeedComment[];
  files?: { filename: string; mime: string; size: number }[];
  unrouted?: boolean;
  assignee?: number;
}

const seedRequests: SeedRequest[] = [
  // 1 — VPN cluster
  { cat: 1, type: 'problem', title: 'Не подключается VPN на объекте в Экибастузе', body: 'Третий день подряд VPN на ноутбуке не подключается через мобильный интернет на объекте. Пишет «превышено время ожидания». Из-за этого не могу выгрузить протоколы испытаний в общую папку.', author: 1, status: 'in_review', created: 30, respond: 3, comments: [{ by: 9, kind: 'comment', body: 'Проверяем настройки шлюза для мобильных операторов. Пока можно передавать протоколы через корпоративный облачный диск.', at: 3 }] },
  // 2 — VPN cluster
  { cat: 1, type: 'problem', title: 'VPN отключается каждые 10 минут на площадке', body: 'На площадке заказчика VPN держится около десяти минут и отключается. Приходится переподключаться, файлы проекта не синхронизируются. Мобильный интернет при этом работает нормально.', author: 13, status: 'new', created: 11 },
  // 3 — VPN cluster
  { cat: 1, type: 'problem', title: 'Нет доступа к серверу через VPN с объекта', body: 'С объекта не открывается файловый сервер через VPN. В офисе всё работает. Похоже, проблема в подключении через мобильный интернет на объекте.', author: 11, status: 'new', created: 2 },
  // 4 — 1C slowness cluster
  { cat: 1, type: 'problem', title: '1С очень медленно открывает документы по утрам', body: 'С 9 до 11 утра 1С открывает документы реализации по минуте и больше. Менеджеры не успевают выставлять счета клиентам вовремя.', author: 10, status: 'accepted', created: 120, respond: 5, comments: [{ by: 2, kind: 'comment', body: 'Подтверждаем: утром идёт регламентное обновление базы. Перенесём его на ночь.', at: 5 }] },
  // 5 — 1C slowness cluster
  { cat: 8, type: 'problem', title: 'Медленно работает 1С при выставлении счетов', body: 'Выставление счёта в 1С занимает несколько минут, особенно утром. Клиенты ждут на телефоне, это неудобно и выглядит непрофессионально.', author: 12, status: 'duplicate', created: 100, respond: 20, close: 20, dupOf: 4, result: 'Та же причина, что и в MV-0004: утреннее регламентное обновление базы. Решаем там.' },
  // 6 — overdue first response (IT, 8 h)
  { cat: 1, type: 'problem', title: 'Принтер на втором этаже печатает с полосами', body: 'Общий принтер у переговорной на втором этаже печатает с серыми полосами уже неделю. Договоры для клиентов приходится перепечатывать в другом кабинете.', author: 10, status: 'new', created: 14 },
  // 7 — supply cluster, overdue new
  { cat: 3, type: 'problem', title: 'Срывы сроков поставки датчиков давления', body: 'Поставщик второй месяц подряд задерживает датчики давления на 2–3 недели. Из-за этого переносим пусконаладку у заказчиков и платим штрафы.', sol: 'Найти второго поставщика и держать минимальный запас датчиков на складе.', author: 13, status: 'new', created: 34 },
  // 8 — supply cluster
  { cat: 3, type: 'problem', title: 'Задержки поставки комплектующих для щитов', body: 'Комплектующие для сборки щитов автоматики приходят с опозданием на две-три недели. Сроки поставки поставщик не подтверждает письменно.', author: 1, anon: true, status: 'in_review', created: 75, respond: 10, comments: [{ by: 5, kind: 'internal', body: 'Это уже третий сигнал по этому поставщику. Готовлю претензию и запрос альтернативных КП.', at: 12 }] },
  // 9 — supply cluster, overdue resolution
  { cat: 3, type: 'idea', title: 'Резервный склад комплектующих для срочных объектов', body: 'Предлагаю держать небольшой резерв ходовых комплектующих и датчиков для срочных объектов, чтобы не зависеть от сроков поставки.', sol: 'Список из 20 позиций уже подготовлен, могу прислать.', author: 13, status: 'accepted', created: 200, respond: 30, comments: [{ by: 5, kind: 'comment', body: 'Хорошая идея. Согласуем бюджет резерва с финансовым отделом.', at: 30 }] },
  // 10 — J4: anonymous request of the demo employee waiting for author
  { cat: 2, type: 'problem', title: 'Нет единого чек-листа перед выездом на пусконаладку', body: 'Перед выездом каждый собирает инструмент и документы по-своему. Регулярно что-то забываем, и приходится возвращаться или ждать курьера. Нужен общий чек-лист.', author: 1, anon: true, status: 'needs_info', created: 52, respond: 6, comments: [{ by: 3, kind: 'question', body: 'Спасибо за сигнал. Подскажите, на каких объектах это происходило чаще всего и чего обычно не хватает?', at: 6 }] },
  // 11
  { cat: 2, type: 'idea', title: 'Фотоотчёт о пусконаладке прямо с телефона', body: 'Сейчас фотографии с объекта пересылаем в мессенджере, потом вручную раскладываем по папкам. Предлагаю загружать их сразу в карточку объекта с телефона.', author: 13, status: 'needs_info', created: 140, respond: 20, comments: [{ by: 3, kind: 'question', body: 'Какие фото нужны обязательно, а какие по желанию? Подготовьте, пожалуйста, пример.', at: 20 }] },
  // 12 — implemented, demo employee
  { cat: 6, type: 'problem', title: 'Не хватает касок и жилетов нужных размеров', body: 'На складе остались только каски и жилеты размера XL. Новым инженерам выдать нечего, а на объекты без СИЗ не пускают.', author: 1, status: 'implemented', created: 400, respond: 2, close: 96, result: 'Закупили 30 комплектов СИЗ размеров S–XL. Установлен минимальный остаток по каждому размеру, склад следит за ним ежемесячно.', comments: [{ by: 3, kind: 'comment', body: 'Заказали партию, ожидаем поставку на этой неделе.', at: 30 }] },
  // 13 — PPE cluster
  { cat: 6, type: 'problem', title: 'Спецодежда выдаётся только больших размеров', body: 'Спецодежду и жилеты выдают только больших размеров. Для сотрудниц и новых инженеров нет подходящих размеров касок и жилетов.', author: 13, status: 'implemented', created: 380, respond: 3, close: 98, result: 'Решено вместе с MV-0012: закуплены все размеры, введён минимальный остаток.' },
  // 14 — declined
  { cat: 7, type: 'idea', title: 'Поставить второй кофейный аппарат на кухне', body: 'В обед на кухне очередь к кофейному аппарату. Предлагаю поставить второй аппарат на третьем этаже.', author: 10, status: 'declined', created: 500, respond: 20, close: 60, result: 'Пока отклоняем: в следующем квартале переезжаем в новый офис, там запланированы две кухни. Вернёмся к вопросу после переезда.' },
  // 15 — implemented
  { cat: 4, type: 'idea', title: 'Шаблон коммерческого предложения с расчётом сроков', body: 'Каждый менеджер делает КП в своём формате. Предлагаю единый шаблон с автоматическим расчётом сроков поставки и монтажа.', sol: 'Шаблон в Excel с формулами, потом перенести в CRM.', author: 10, status: 'implemented', created: 700, respond: 10, close: 300, result: 'Единый шаблон КП добавлен в CRM. Сроки поставки подтягиваются из справочника закупок.' },
  // 16 — in review, design
  { cat: 5, type: 'problem', title: 'Лицензий на программу проектирования не хватает', body: 'В отделе 9 проектировщиков и только 6 лицензий. Приходится ждать, пока коллега освободит лицензию, проекты задерживаются.', author: 11, status: 'in_review', created: 90, respond: 18, comments: [{ by: 7, kind: 'comment', body: 'Собираю статистику использования лицензий за месяц, чтобы обосновать докупку.', at: 18 }] },
  // 17 — design idea, accepted soon resolution
  { cat: 5, type: 'idea', title: 'Библиотека типовых узлов для проектов автоматики', body: 'Предлагаю собрать общую библиотеку типовых узлов и шкафов, чтобы не чертить их заново в каждом проекте.', author: 11, status: 'accepted', created: 160, respond: 8, comments: [{ by: 7, kind: 'comment', body: 'Принимаем. Назначим ответственного за библиотеку и сроки наполнения.', at: 8 }] },
  // 18 — unrouted (category 8 has no rule)
  { cat: 8, type: 'idea', title: 'Согласование командировок в один шаг', body: 'Сейчас командировку согласуют три человека по почте, на это уходит до трёх дней. Предлагаю сделать одно согласование у руководителя отдела.', author: 13, status: 'new', created: 20, unrouted: true },
  // 19 — new, soon due (sales 24 h)
  { cat: 4, type: 'problem', title: 'Клиенты не получают уведомления о готовности заказа', body: 'Несколько клиентов пожаловались, что не получили письмо о готовности заказа к отгрузке. Узнают только когда сами звонят.', author: 10, status: 'new', created: 20 },
  // 20 — in_review, office
  { cat: 7, type: 'problem', title: 'Не работает кондиционер в переговорной', body: 'В большой переговорной не работает кондиционер. Во второй половине дня там невозможно проводить встречи с клиентами.', author: 12, status: 'in_review', created: 40, respond: 4 },
  // 21 — implemented, IT
  { cat: 1, type: 'idea', title: 'Единый пароль для корпоративных сервисов', body: 'Для почты, CRM и 1С сейчас разные пароли. Предлагаю единый вход, чтобы не путаться и не записывать пароли на бумажках.', author: 12, status: 'implemented', created: 900, respond: 6, close: 500, result: 'Подключили единый вход для почты и CRM. 1С переведём во втором квартале.' },
  // 22 — declined, sales
  { cat: 4, type: 'idea', title: 'Отдельный чат для каждого клиента', body: 'Предлагаю заводить отдельный чат в мессенджере для каждого крупного клиента.', author: 10, status: 'declined', created: 300, respond: 12, close: 40, result: 'Отклонено: переписка с клиентами должна храниться в CRM. Вместо этого включили интеграцию CRM с почтой.' },
  // 23 — new, service, anonymous by other employee
  { cat: 2, type: 'problem', title: 'Служебный автомобиль без зимней резины', body: 'Служебный автомобиль для выездов до сих пор на летней резине. На трассе уже гололёд, ездить на объекты опасно.', author: 13, anon: true, status: 'new', created: 6, files: [{ filename: 'шины.jpg', mime: 'image/jpeg', size: 842_000 }] },
  // 24 — in_review, assigned to Pavel (IT staff), overdue resolution
  { cat: 1, type: 'problem', title: 'Не работает сканер документов в бухгалтерии', body: 'Сканер документов в бухгалтерии не определяется компьютером после обновления. Первичку приходится сканировать на телефон.', author: 12, status: 'in_review', created: 110, respond: 2 },
  // 25 — implemented, service, demo employee
  { cat: 2, type: 'idea', title: 'Набор запасных предохранителей в каждой машине', body: 'Предлагаю укомплектовать каждую сервисную машину набором ходовых предохранителей и клемм. Часто из-за мелочи приходится ехать на склад.', author: 1, status: 'implemented', created: 1000, respond: 10, close: 200, result: 'Наборы собраны и выданы во все четыре сервисные машины. Пополнение — при ежемесячном ТО.', files: [{ filename: 'список_комплекта.xlsx', mime: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', size: 18_400 }] },
  // 26 — new, demo employee, files
  { cat: 1, type: 'problem', title: 'Ноутбук перегревается при работе с программой ПЛК', body: 'Служебный ноутбук сильно греется и выключается, когда запускаю среду программирования контроллеров на объекте.', author: 1, status: 'new', created: 1, files: [{ filename: 'ошибка.png', mime: 'image/png', size: 214_000 }] },
];

/** Build the full mock database from the seed. */
export function buildSeed(now: number = Date.now()): MockDb {
  let seq = 100;
  const next = () => ++seq;
  const requests: MRequest[] = [];
  const comments: MComment[] = [];
  const events: MEvent[] = [];
  const notifications: MNotification[] = [];
  let attId = 1;

  seedRequests.forEach((s, idx) => {
    const id = idx + 1;
    const created = now - s.created * H;
    const rule =
      rules.find((r) => r.active && r.category_id === s.cat && r.request_type === s.type) ??
      rules.find((r) => r.active && r.category_id === s.cat && r.request_type === null);
    const routed = Boolean(rule) && !s.unrouted;
    const dept = routed ? rule!.department_id : null;
    const head = dept ? departments.find((d) => d.id === dept)!.head_user_id : null;
    const assignee = s.assignee ?? (routed ? (rule!.assignee_user_id ?? head) : null);
    const staff = assignee ?? head ?? 4;
    const fr = routed ? rule!.first_response_hours : 24;
    const rs = routed ? rule!.resolution_hours : 120;
    const respondedAt = s.respond !== undefined ? created + s.respond * H : null;
    const closedAt = s.close !== undefined ? created + s.close * H : null;
    let updated = created;

    const req: MRequest = {
      id,
      number: `MV-${String(id).padStart(4, '0')}`,
      type: s.type,
      category_id: s.cat,
      title: s.title,
      body: s.body,
      proposed_solution: s.sol ?? null,
      author_id: s.author,
      is_anonymous: Boolean(s.anon),
      department_id: dept,
      assignee_id: assignee,
      status: s.status,
      routing_source: routed ? 'rule' : 'unrouted',
      first_response_due: iso(created + fr * H),
      resolution_due: iso(created + rs * H),
      first_responded_at: respondedAt ? iso(respondedAt) : null,
      paused_at: null,
      closed_at: closedAt ? iso(closedAt) : null,
      result_text: s.result ?? null,
      duplicate_of: s.dupOf ?? null,
      theme_id: null,
      created_at: iso(created),
      updated_at: iso(created),
      attachments: (s.files ?? []).map((f) => ({ id: attId++, ...f })),
    };

    events.push({ id: next(), request_id: id, actor_id: s.author, kind: 'created', created_at: iso(created) });
    events.push({
      id: next(),
      request_id: id,
      actor_id: null,
      kind: 'routed',
      text: routed
        ? `Передано в отдел «${departments.find((d) => d.id === dept)!.name}» по правилу маршрутизации`
        : 'Правило не найдено — передано администратору',
      created_at: iso(created + 1000),
    });

    const change = (from: Status, to: Status, at: number, text?: string | null) => {
      events.push({ id: next(), request_id: id, actor_id: staff, kind: 'status_changed', from_status: from, to_status: to, text: text ?? null, created_at: iso(at) });
      updated = Math.max(updated, at);
    };

    const r = respondedAt ?? created;
    const c = closedAt ?? now;
    switch (s.status) {
      case 'in_review':
        change('new', 'in_review', r);
        break;
      case 'needs_info':
        change('new', 'in_review', r - 60_000);
        break;
      case 'accepted':
        change('new', 'in_review', r);
        change('in_review', 'accepted', r + (now - r) * 0.3);
        break;
      case 'implemented':
        change('new', 'in_review', r);
        change('in_review', 'accepted', r + (c - r) * 0.3);
        change('accepted', 'implemented', c, s.result);
        break;
      case 'declined':
        change('new', 'in_review', r);
        change('in_review', 'declined', c, s.result);
        break;
      case 'duplicate':
        change('new', 'duplicate', c, s.result);
        break;
      default:
        break;
    }

    for (const cm of s.comments ?? []) {
      const at = created + cm.at * H;
      comments.push({ id: next(), request_id: id, author_id: cm.by, kind: cm.kind, body: cm.body, created_at: iso(at) });
      if (cm.kind === 'question') {
        events.push({ id: next(), request_id: id, actor_id: cm.by, kind: 'status_changed', from_status: 'in_review', to_status: 'needs_info', text: null, created_at: iso(at + 1000) });
        req.paused_at = iso(at);
      }
      updated = Math.max(updated, at);
    }
    req.updated_at = iso(updated);
    requests.push(req);
  });

  // Confirmed theme: PPE sizes (requests 12 and 13).
  requests[11].theme_id = 1;
  requests[12].theme_id = 1;
  events.push({ id: next(), request_id: 12, actor_id: 3, kind: 'theme_linked', text: 'Обеспечение СИЗ на объектах', created_at: iso(now - 300 * H) });

  // Notifications for the demo accounts.
  const notify = (user_id: number, request_id: number, kind: string, title: string, body: string, hoursAgo: number, read = false) =>
    notifications.push({ id: next(), user_id, kind, request_id, title, body, read, created_at: iso(now - hoursAgo * H) });
  notify(1, 10, 'question', 'Нужен ваш ответ по MV-0010', 'Ирина Ковалёва задала уточняющий вопрос.', 46);
  notify(1, 1, 'status_changed', 'MV-0001: На рассмотрении', 'Обращение взято в работу отделом IT.', 27);
  notify(1, 1, 'comment', 'Новый комментарий в MV-0001', 'Павел Широков: Проверяем настройки шлюза…', 27);
  notify(1, 12, 'status_changed', 'MV-0012: Реализовано', 'Закупили 30 комплектов СИЗ размеров S–XL.', 304, true);
  notify(1, 25, 'status_changed', 'MV-0025: Реализовано', 'Наборы собраны и выданы во все сервисные машины.', 800, true);
  notify(2, 6, 'overdue', 'Просрочен первый ответ: MV-0006', 'Принтер на втором этаже печатает с полосами.', 6);
  notify(2, 3, 'created', 'Новое обращение MV-0003', 'Нет доступа к серверу через VPN с объекта.', 2);
  notify(2, 2, 'created', 'Новое обращение MV-0002', 'VPN отключается каждые 10 минут на площадке.', 11, true);
  notify(3, 23, 'created', 'Новое обращение MV-0023', 'Служебный автомобиль без зимней резины.', 6);
  notify(3, 11, 'reminder', 'Ждём автора: MV-0011', 'Автор пока не ответил на уточнение.', 24, true);
  notify(4, 18, 'unrouted', 'Не распределено: MV-0018', 'Для категории «Процессы и управление» нет правила.', 20);
  notify(4, 7, 'overdue', 'Просрочен первый ответ: MV-0007', 'Срывы сроков поставки датчиков давления.', 10);

  return {
    version: 3,
    seq: seq + 1000,
    users: users.map((u) => ({ ...u })),
    departments: departments.map((d) => ({ ...d })),
    categories: categories.map((c) => ({ ...c })),
    rules: rules.map((r) => ({ ...r })),
    requests,
    comments,
    events,
    notifications,
    themes: [{ id: 1, title: 'Обеспечение СИЗ на объектах', created_at: iso(now - 300 * H) }],
    sessionUserId: null,
    failedLogins: 0,
    numberCounter: requests.length,
  };
}
