// Demo data of the in-browser mock = the backend seed (backend/internal/seed, CR1): exactly the 20
// requests A-001…A-020 of docs/DATASET.md (MV-0001 = A-001 …), the 18 dataset topic labels as
// categories, the dataset's conditional roles as users (display name = role title, no personal
// names). Only test dates are added, relative to "now", so SLA states look realistic.
import type { Priority, RequestType, Status } from '../types';
import type { MCategory, MComment, MDept, MEvent, MockDb, MNotification, MRequest, MRule, MUser } from './model';

export const DEMO_PASSWORD = 'Demo1234!';
export const MOCK_DB_VERSION = 4;
const DOMAIN = '@modus.demo';

/** One-click logins (order = UI order), as in the backend seed. */
export const DEMO_LOGINS = ['purchasing.employee', 'sales.manager', 'purchasing.head', 'it.specialist', 'design.head', 'admin'];

/** DEMO ONLY tokens of the seeded anonymous requests (published in backend/README.md). */
export const DEMO_TOKENS = {
  A005: 'DEM0-A005-7K3Q-9XWB-2HRT-M4ZP-C8NV',
  A017: 'DEM0-A017-5JPV-W2QX-8KTN-R6MD-H3ZB',
  A018: 'DEM0-A018-Q9RT-4MXK-7CWP-N2JH-B5VZ',
};

const TOKEN_ALPHABET = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';
export const TOKEN_SYMBOLS = 28;

/** Same rules as backend CanonicalToken: case/separator insensitive, O→0, I/L→1. */
export function canonicalToken(s: string): string | null {
  if (s.length > 100) return null;
  let out = '';
  for (let ch of s.toUpperCase()) {
    if ('- \t\n\r'.includes(ch)) continue;
    if (ch === 'O') ch = '0';
    if (ch === 'I' || ch === 'L') ch = '1';
    if (!TOKEN_ALPHABET.includes(ch)) return null;
    out += ch;
  }
  return out.length === TOKEN_SYMBOLS ? out : null;
}

/** Random token grouped by 4 (crypto.getRandomValues; 256 is a multiple of 32, so unbiased). */
export function newToken(): string {
  const raw = new Uint8Array(TOKEN_SYMBOLS);
  crypto.getRandomValues(raw);
  let s = '';
  raw.forEach((v, i) => {
    if (i > 0 && i % 4 === 0) s += '-';
    s += TOKEN_ALPHABET[v & 31];
  });
  return s;
}

const H = 3_600_000;
const iso = (t: number) => new Date(t).toISOString();

const D = { purchasing: 1, sales: 2, design: 3, accounting: 4, projects: 5, it: 6, adminHR: 7, directions: 8 } as const;

const deptSeed: [string, string][] = [
  ['Закупки', 'purchasing.head'],
  ['Продажи', 'sales.head'],
  ['Проектирование', 'design.head'],
  ['Бухгалтерия', 'accounting.head'],
  ['Управление проектами', 'projects.head'],
  ['IT', ''], // the dataset has no «Руководитель IT»
  ['Административный отдел и HR', 'admin.head'],
  ['Руководство направлений', 'direction.head'],
];

const userSeed: [string, string, MUser['role'], number][] = [
  ['admin', 'Администратор системы', 'admin', D.it],
  ['purchasing.employee', 'Сотрудник закупок', 'employee', D.purchasing],
  ['design.employee', 'Сотрудник проектирования', 'employee', D.design],
  ['accounting.employee', 'Сотрудник бухгалтерии', 'employee', D.accounting],
  ['sales.manager', 'Менеджер по продажам', 'employee', D.sales],
  ['project.coordinator', 'Координатор проекта', 'employee', D.projects],
  ['project.lead', 'Руководитель проекта', 'employee', D.projects],
  ['office.employee', 'Сотрудник офиса', 'employee', D.adminHR],
  ['purchasing.head', 'Руководитель закупок', 'manager', D.purchasing],
  ['direction.head', 'Руководитель направления', 'manager', D.directions],
  ['catalog.owner', 'Ответственный за номенклатуру', 'manager', D.purchasing],
  ['training.owner', 'Ответственный за обучение', 'manager', D.adminHR],
  ['accounting.head', 'Руководитель бухгалтерии', 'manager', D.accounting],
  ['process.owner', 'Ответственный за процессы', 'manager', D.directions],
  ['it.specialist', 'IT-специалист', 'manager', D.it],
  ['unit.head', 'Руководитель подразделения', 'manager', D.directions],
  ['contracts.owner', 'Ответственный за договоры', 'manager', D.sales],
  ['sales.head', 'Руководитель продаж', 'manager', D.sales],
  ['projects.head', 'Руководитель проектов', 'manager', D.projects],
  ['design.head', 'Руководитель проектирования', 'manager', D.design],
  ['admin.head', 'Административный руководитель', 'manager', D.adminHR],
  ['hr.comms', 'HR / внутренние коммуникации', 'manager', D.adminHR],
  ['documents.admin', 'Администратор документов', 'manager', D.adminHR],
];

export const users: MUser[] = userSeed.map(([login, name, role, dept], i) => ({
  id: i + 1,
  name,
  email: login + DOMAIN,
  password: DEMO_PASSWORD,
  role,
  department_id: dept,
  active: true,
  title: name,
}));
const uid = (login: string) => users.find((u) => u.email === login + DOMAIN)!.id;

export const departments: MDept[] = deptSeed.map(([name, head], i) => ({ id: i + 1, name, head_user_id: head ? uid(head) : null }));

const catSeed: [string, string, string][] = [
  ['Межотдельное взаимодействие', 'arrow-left-right', 'Заявки и передача работы между подразделениями'],
  ['Информационная прозрачность', 'eye', 'Доступ к статусам и сведениям, нужным для работы'],
  ['Каталог и данные', 'database', 'Номенклатура, карточки позиций и справочные данные'],
  ['Обучение', 'graduation-cap', 'Занятия, семинары и обучение сотрудников'],
  ['Документооборот', 'file-text', 'Движение и согласование документов между подразделениями'],
  ['Регламенты', 'book-open', 'Инструкции, памятки и порядок выполнения операций'],
  ['Интеграции и IT', 'plug', 'Рабочие системы, синхронизация данных и автоматизация'],
  ['Знания и наставничество', 'users', 'Передача опыта, наставничество и памятки для новичков'],
  ['Договорная работа', 'file-signature', 'Подготовка, проверка и согласование договоров'],
  ['Задачи и коммуникации', 'list-checks', 'Ведение рабочих задач и истории обсуждений'],
  ['Оперативность', 'timer', 'Сроки реакции и уведомления о задержках'],
  ['Коммуникации', 'messages-square', 'Каналы общения и фиксация итоговых решений'],
  ['Проектная деятельность', 'folder-kanban', 'Проекты, спецификации и история изменений'],
  ['Планирование ресурсов', 'calendar-range', 'Загрузка команд и согласование сроков'],
  ['Постановка задач', 'target', 'Цель, срок, приоритет и ответственный в поручениях'],
  ['Корпоративная среда', 'building-2', 'Рабочие помещения, атмосфера и внутренние мероприятия'],
  ['Хранение документов', 'archive', 'Общие папки, именование и архивирование версий'],
  ['Прозрачность результатов', 'trophy', 'Обзор итогов и реализованных улучшений'],
];
export const categories: MCategory[] = catSeed.map(([name, icon, description], i) => ({ id: i + 1, name, icon, description, sort: (i + 1) * 10, active: true }));

const C = {
  interdept: 1, infoTransparency: 2, catalog: 3, training: 4, docflow: 5, regulations: 6, integrations: 7, knowledge: 8, contracts: 9,
  tasksComms: 10, responsiveness: 11, comms: 12, projects: 13, resources: 14, taskSetting: 15, workplace: 16, docStorage: 17, results: 18,
} as const;

// «Хранение документов» intentionally has NO rule (A-019 goes through the unrouted admin queue).
const ruleSeed: [number, RequestType | null, number, string, number, number][] = [
  [C.interdept, null, D.purchasing, 'purchasing.head', 48, 240],
  [C.infoTransparency, null, D.directions, 'direction.head', 48, 240],
  [C.catalog, null, D.purchasing, 'catalog.owner', 24, 168],
  [C.training, null, D.adminHR, 'training.owner', 72, 336],
  [C.docflow, null, D.accounting, 'accounting.head', 24, 168],
  [C.regulations, null, D.directions, 'process.owner', 48, 240],
  [C.integrations, null, D.it, 'it.specialist', 24, 120],
  [C.knowledge, null, D.directions, 'unit.head', 72, 336],
  [C.contracts, null, D.sales, 'contracts.owner', 24, 120],
  [C.tasksComms, null, D.sales, 'sales.head', 48, 240],
  [C.responsiveness, null, D.directions, 'direction.head', 24, 120],
  [C.comms, null, D.it, 'it.specialist', 48, 240],
  [C.projects, null, D.projects, 'projects.head', 24, 168],
  [C.resources, null, D.design, 'design.head', 48, 240],
  [C.taskSetting, null, D.directions, 'direction.head', 48, 240],
  [C.workplace, null, D.adminHR, 'admin.head', 72, 336],
  [C.workplace, 'feedback', D.adminHR, 'hr.comms', 72, 336],
  [C.results, null, D.directions, 'unit.head', 72, 336],
];
export const rules: MRule[] = ruleSeed.map(([category_id, request_type, department_id, assignee, fr, res], i) => ({
  id: i + 1,
  category_id,
  request_type,
  department_id,
  assignee_user_id: uid(assignee),
  first_response_hours: fr,
  resolution_hours: res,
  active: true,
}));

function ruleFor(cat: number, type: RequestType): MRule | undefined {
  return rules.find((r) => r.category_id === cat && r.request_type === type) ?? rules.find((r) => r.category_id === cat && r.request_type === null);
}

interface ReqSeed {
  typ: RequestType;
  prio: Priority;
  status: Status;
  cat: number;
  author: string;
  responsible: string;
  token?: string;
  title: string;
  body: string;
  note?: string;
  question?: string;
  result?: string;
  /** hours since creation, hours to first response (0 = none), hours to closing */
  ageH: number;
  frH?: number;
  closeH?: number;
}

const P = 'problem' as const;
const I = 'idea' as const;
const F = 'feedback' as const;

const requestSeeds: ReqSeed[] = [
  { typ: P, prio: 'medium', status: 'new', cat: C.interdept, author: 'purchasing.employee', responsible: 'purchasing.head',
    title: 'В заявках на оборудование не хватает назначения и параметров',
    body: 'В заявках на приобретение оборудования иногда не хватает назначения изделия и требуемых параметров. Предлагаю добавить обязательные поля, чтобы сократить число уточнений.',
    note: 'Запросить пример корректно заполненной заявки; оценить изменение шаблона.', ageH: 74 },
  { typ: I, prio: 'medium', status: 'in_review', cat: C.infoTransparency, author: 'purchasing.employee', responsible: 'direction.head',
    title: 'Краткий статус проекта для общения с поставщиками',
    body: 'При общении с поставщиками полезно видеть общую стадию проекта без доступа к коммерческим деталям. Можно ли показывать краткий статус и ориентировочные контрольные даты?',
    note: 'Определить перечень доступных статусов и источник их обновления.', ageH: 8 * 24 + 5, frH: 20 },
  { typ: P, prio: 'high', status: 'in_progress', cat: C.catalog, author: 'design.employee', responsible: 'catalog.owner',
    title: 'Разные обозначения одного изделия в рабочих материалах',
    body: 'В рабочих материалах иногда встречаются разные обозначения одного изделия. Это увеличивает время проверки спецификации. Нужен понятный способ находить актуальную карточку позиции.',
    note: 'Подготовить правило проверки и корректировки номенклатуры.', ageH: 12 * 24 + 3, frH: 6 },
  { typ: I, prio: 'low', status: 'implemented', cat: C.training, author: 'purchasing.employee', responsible: 'training.owner',
    title: 'Короткие занятия по новым товарным линейкам',
    body: 'Было бы полезно проводить короткие занятия по новым товарным линейкам с разбором типовых запросов. Предлагаю составить календарь таких встреч.',
    result: 'Запланирован пробный обучающий семинар.', ageH: 58 * 24 + 2, frH: 30, closeH: 17 * 24 },
  { typ: P, prio: 'medium', status: 'needs_info', cat: C.docflow, author: 'accounting.employee', responsible: 'accounting.head', token: DEMO_TOKENS.A005,
    title: 'Статус пакета документов между подразделениями',
    body: 'При обработке документов между подразделениями возникают дополнительные вопросы о том, на каком этапе находится пакет. Хотелось бы видеть ответственного и текущий статус.',
    question: 'Уточните, пожалуйста, виды документов и ключевые этапы согласования.', ageH: 6 * 24 + 4, frH: 9 },
  { typ: I, prio: 'medium', status: 'in_review', cat: C.regulations, author: 'accounting.employee', responsible: 'process.owner',
    title: 'Пошаговая памятка для операций нескольких подразделений',
    body: 'Предлагаю собрать короткую пошаговую памятку для операций, в которых последовательно участвуют несколько подразделений, и определить порядок актуализации памятки.',
    note: 'Назначить владельца процесса и согласовать черновик инструкции.', ageH: 5 * 24 + 6, frH: 26 },
  { typ: P, prio: 'high', status: 'new', cat: C.integrations, author: 'accounting.employee', responsible: 'it.specialist',
    title: 'Повторный ввод одних и тех же сведений в разных системах',
    body: 'Иногда повторный ввод одних и тех же сведений в разных рабочих системах занимает много времени. Нужна оценка, какие операции можно синхронизировать безопасно.',
    note: 'Собрать примеры операций и оценить техническую реализуемость.', ageH: 2 * 24 + 3 },
  { typ: I, prio: 'low', status: 'implemented', cat: C.knowledge, author: 'accounting.employee', responsible: 'unit.head',
    title: 'Памятка при передаче сложной задачи новому сотруднику',
    body: 'При передаче сложной задачи новому сотруднику полезен короткий разбор типовых ошибок и контактов, к которым можно обратиться за помощью. Предлагаю сделать такую памятку.',
    result: 'Создана учебная инструкция.', ageH: 50 * 24 + 7, frH: 52, closeH: 15 * 24 },
  { typ: P, prio: 'high', status: 'in_progress', cat: C.contracts, author: 'sales.manager', responsible: 'contracts.owner',
    title: 'Кто готовит и проверяет приложение к договору',
    body: 'Не всегда понятно, кто готовит очередную версию приложения к договору и кто должен проверить её перед отправкой. Это приводит к повторным согласованиям.',
    note: 'Разграничить этапы подготовки, проверки и утверждения.', ageH: 4 * 24 + 2, frH: 4 },
  { typ: I, prio: 'medium', status: 'in_review', cat: C.tasksComms, author: 'sales.manager', responsible: 'sales.head',
    title: 'Не закрывать задачу до подтверждения результата заказчиком',
    body: 'Предлагаю не закрывать рабочую задачу до подтверждения заказчиком результата внутри компании, чтобы связанные вопросы оставались в общей истории обсуждения.',
    note: 'Проверить, подходит ли правило для разных типов задач.', ageH: 7 * 24 + 1, frH: 55 },
  { typ: P, prio: 'high', status: 'new', cat: C.responsiveness, author: 'sales.manager', responsible: 'direction.head',
    title: 'Сроки ответа смежных подразделений при срочном предложении',
    body: 'При срочной подготовке клиентского предложения сложно прогнозировать сроки ответа от смежных подразделений. Хотелось бы видеть ожидаемый срок и уведомление при задержке.',
    note: 'Определить допустимые сроки реакции для разных категорий.', ageH: 6 },
  { typ: I, prio: 'low', status: 'in_review', cat: C.comms, author: 'sales.manager', responsible: 'it.specialist',
    title: 'Единое место для итоговых решений по задаче',
    body: 'Рабочие обсуждения распределены по нескольким каналам, из-за чего теряется контекст. Предлагаю выбрать единое место для итоговых решений по задаче.',
    note: 'Сравнить варианты и предложить правила фиксации итогов.', ageH: 3 * 24 + 5, frH: 11 },
  { typ: P, prio: 'medium', status: 'needs_info', cat: C.projects, author: 'project.coordinator', responsible: 'projects.head',
    title: 'История изменений спецификации при смене оборудования',
    body: 'Когда изменяется состав оборудования в проекте, не всегда ясно, какие строки спецификации были пересмотрены. Нужна более заметная история изменений.',
    question: 'Уточните, пожалуйста, формат исходных файлов и способы сравнения версий.', ageH: 10 * 24 + 2, frH: 14 },
  { typ: I, prio: 'medium', status: 'in_review', cat: C.resources, author: 'project.coordinator', responsible: 'design.head',
    title: 'Загруженность проектной группы на ближайшие недели',
    body: 'Предлагаю показывать загруженность проектной группы на ближайшие недели в укрупнённом виде. Так будет проще согласовывать сроки запуска новых задач.',
    note: 'Согласовать показатели загрузки и права просмотра.', ageH: 9 * 24 + 4, frH: 61 },
  { typ: P, prio: 'medium', status: 'in_progress', cat: C.taskSetting, author: 'project.lead', responsible: 'direction.head',
    title: 'Поручения без ожидаемого результата и даты завершения',
    body: 'Некоторые поручения поступают без ожидаемого результата и даты завершения. Предлагаю договориться о минимальном наборе данных при постановке задачи.',
    note: 'Подготовить шаблон с целью, сроком, приоритетом и ответственным.', ageH: 9 * 24 + 1, frH: 18 },
  { typ: I, prio: 'low', status: 'new', cat: C.training, author: 'project.lead', responsible: 'training.owner',
    title: 'Практические демонстрации оборудования для новых сотрудников',
    body: 'Для новых сотрудников полезны практические демонстрации оборудования, а не только презентации. Предлагаю пилотное занятие с учебными образцами.',
    note: 'Проверить наличие образцов и составить план пилота.', ageH: 26 },
  { typ: I, prio: 'low', status: 'declined', cat: C.workplace, author: 'office.employee', responsible: 'admin.head', token: DEMO_TOKENS.A017,
    title: 'Зона для коротких перерывов рядом с рабочими помещениями',
    body: 'Предлагаю организовать отдельную небольшую зону для коротких перерывов рядом с рабочими помещениями.',
    result: 'Сейчас нет подходящего помещения; вернуться к вопросу при перепланировке.', ageH: 41 * 24 + 3, frH: 40, closeH: 3 * 24 + 6 },
  { typ: F, prio: 'low', status: 'closed', cat: C.workplace, author: 'office.employee', responsible: 'hr.comms', token: DEMO_TOKENS.A018,
    title: 'Польза внутренних учебных встреч',
    body: 'Хочу отметить пользу внутренних учебных встреч: после них стало легче обращаться к коллегам за консультацией. Было бы хорошо сохранить этот формат.',
    note: 'Зафиксировать положительный отзыв и передать организаторам обучения.',
    result: 'Положительный отзыв зафиксирован и передан организаторам обучения.', ageH: 33 * 24 + 5, frH: 22, closeH: 2 * 24 + 4 },
  { typ: P, prio: 'medium', status: 'in_review', cat: C.docStorage, author: 'project.coordinator', responsible: 'documents.admin',
    title: 'Актуальная редакция инструкции в общих папках',
    body: 'В общих папках трудно быстро определить актуальную редакцию инструкции. Можно ли закрепить единые правила именования и архивирования старых версий?',
    note: 'Согласовать порядок хранения и владельцев папок.', ageH: 2 * 24 + 1, frH: 3 },
  { typ: I, prio: 'low', status: 'new', cat: C.results, author: 'purchasing.employee', responsible: 'unit.head',
    title: 'Краткий обзор завершённых внутренних улучшений',
    body: 'Предлагаю публиковать краткий обзор завершённых внутренних улучшений, чтобы сотрудники видели результат направленных предложений.',
    note: 'Рассмотреть формат квартальной подборки без конфиденциальных сведений.', ageH: 4 * 24 + 2 },
];

const STATUS_LABEL: Record<Status, string> = {
  new: 'Новое', in_review: 'В проработке', needs_info: 'Нужно уточнение', decision: 'Принятие решения', in_progress: 'Реализация',
  implemented: 'Реализовано', declined: 'Отклонено', duplicate: 'Дубликат', closed: 'Закрыто',
};
const TERMINAL: Status[] = ['implemented', 'declined', 'duplicate', 'closed'];

export function buildSeed(now: number = Date.now()): MockDb {
  let seq = 100;
  const next = () => ++seq;
  const requests: MRequest[] = [];
  const comments: MComment[] = [];
  const events: MEvent[] = [];
  const notifications: MNotification[] = [];
  const adminId = uid('admin');

  const notify = (user_id: number, kind: string, request_id: number, title: string, body: string, at: number) =>
    notifications.push({ id: next(), user_id, kind, request_id, title, body, read: now - at > 72 * H, created_at: iso(at) });

  requestSeeds.forEach((t, idx) => {
    const id = idx + 1;
    const number = `MV-${String(id).padStart(4, '0')}`;
    const created = now - t.ageH * H;
    const at = (h: number) => Math.min(created + h * H, now - 10 * 60_000);
    const author = t.token ? null : uid(t.author);
    const rule = ruleFor(t.cat, t.typ);
    let frd = created + (rule?.first_response_hours ?? 24) * H;
    let rd = created + (rule?.resolution_hours ?? 120) * H;
    const r: MRequest = {
      id, number, type: t.typ, category_id: t.cat, title: t.title, body: t.body, proposed_solution: null,
      author_id: author, is_anonymous: Boolean(t.token), anon_token: t.token ? canonicalToken(t.token) : null, priority: t.prio,
      department_id: rule?.department_id ?? null, assignee_id: rule?.assignee_user_id ?? null, status: 'new',
      routing_source: rule ? 'rule' : 'unrouted', first_response_due: '', resolution_due: '', first_responded_at: null, paused_at: null,
      closed_at: null, result_text: null, duplicate_of: null, theme_id: null, created_at: iso(created), updated_at: iso(created), attachments: [],
    };
    let staff = rule?.assignee_user_id ?? adminId;
    let updated = created;
    let paused: number | null = null;
    const deptName = rule ? departments.find((d) => d.id === rule.department_id)!.name : '';
    events.push({ id: next(), request_id: id, actor_id: author, by_author: true, kind: 'created', to_status: 'new', created_at: iso(created) });
    events.push({
      id: next(), request_id: id, actor_id: null, kind: 'routed', created_at: iso(created),
      text: rule
        ? `Передано: ${deptName}, ответственный: ${users.find((u) => u.id === rule.assignee_user_id)!.name}. Срок первого ответа — ${rule.first_response_hours} ч`
        : 'Подходящего правила маршрутизации нет — обращение передано администратору',
    });
    notify(staff, 'created', id, rule ? `Новое обращение ${number}` : `Нераспределённое обращение ${number}`, t.title, created);
    const firstResponse = (a: number) => {
      if (!r.first_responded_at) r.first_responded_at = iso(a);
    };
    const comment = (kind: MComment['kind'], body: string, a: number) => {
      comments.push({ id: next(), request_id: id, author_id: staff, kind, body, created_at: iso(a) });
      updated = Math.max(updated, a);
    };
    const setStatus = (to: Status, a: number, text: string | null = null) => {
      const from = r.status;
      if (from === 'needs_info' && paused !== null) {
        frd += a - paused;
        rd += a - paused;
        paused = null;
      }
      if (to === 'needs_info') paused = a;
      r.status = to;
      if (TERMINAL.includes(to)) r.closed_at = iso(a);
      updated = Math.max(updated, a);
      events.push({ id: next(), request_id: id, actor_id: staff, kind: 'status_changed', from_status: from, to_status: to, text, created_at: iso(a) });
      if (author) notify(author, 'status_changed', id, `${number}: статус «${STATUS_LABEL[to]}»`, text ?? t.title, a);
    };

    let frH = t.frH ?? 0;
    if (!rule) {
      // A-019: unrouted -> admin assigns the dataset's responsible manually.
      const a = at(frH);
      r.department_id = D.adminHR;
      r.assignee_id = uid(t.responsible);
      r.routing_source = 'manual';
      events.push({ id: next(), request_id: id, actor_id: adminId, kind: 'assigned', text: `Отдел: Административный отдел и HR; ответственный: ${users.find((u) => u.id === r.assignee_id)!.name}`, created_at: iso(a) });
      notify(r.assignee_id, 'assigned', id, `Вам передано обращение ${number}`, t.title, a);
      firstResponse(a);
      updated = a;
      staff = r.assignee_id;
      frH += 1;
    }
    if (t.note && !t.question) comment('internal', t.note, at(frH === 0 ? Math.min(2, t.ageH / 2) : frH + 0.5));
    if (t.status !== 'new') {
      const first = at(frH);
      if (t.status === 'needs_info') {
        comment('question', t.question!, first);
        firstResponse(first);
        setStatus('needs_info', first);
        if (author) notify(author, 'question', id, `${number}: нужен ваш ответ`, t.question!, first);
      } else {
        firstResponse(first);
        setStatus('in_review', first);
        if (t.status === 'in_progress' || t.status === 'implemented') {
          const end = t.status === 'implemented' ? t.closeH! : t.ageH;
          const span = end - frH;
          setStatus('decision', at(frH + span * 0.3));
          setStatus('in_progress', at(frH + span * 0.5));
          if (t.status === 'implemented') {
            r.result_text = t.result!;
            setStatus('implemented', at(t.closeH!), t.result!);
          }
        } else if (t.status === 'declined' || t.status === 'closed') {
          r.result_text = t.result!;
          setStatus(t.status, at(t.closeH!), t.result!);
        }
      }
    }
    r.first_response_due = iso(frd);
    r.resolution_due = iso(rd);
    r.paused_at = paused !== null ? iso(paused) : null;
    r.updated_at = iso(updated);
    requests.push(r);
  });

  return {
    version: MOCK_DB_VERSION,
    seq: seq + 1000,
    users: users.map((u) => ({ ...u })),
    departments: departments.map((d) => ({ ...d })),
    categories: categories.map((c) => ({ ...c })),
    rules: rules.map((r) => ({ ...r })),
    requests,
    comments,
    events,
    notifications,
    themes: [],
    telegramLinks: {},
    telegramEnabled: true,
    sessionUserId: null,
    failedLogins: 0,
    numberCounter: requests.length,
  };
}
