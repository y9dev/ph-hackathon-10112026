package seed

// CR1: every value below comes from docs/DATASET.md (extract of spec/Датасет.docx). Request
// texts are kept verbatim; titles are short headings derived from the first clause. Authors and
// responsibles are the dataset's conditional roles (display name = role title, no personal names).
// Only test dates (created_at, first response, closing times) are added, as the dataset permits.

// DemoPassword is the password of every seeded account.
const DemoPassword = "Demo1234!"

// DemoEmails are offered as one-click logins when DEMO_MODE=true (order = UI order): two authors,
// two responsible managers, a manager of another department (observer demo, CR6), the admin.
var DemoEmails = []string{
	"purchasing.employee" + Domain, "sales.manager" + Domain,
	"purchasing.head" + Domain, "it.specialist" + Domain,
	"design.head" + Domain, "admin" + Domain,
}

// Demo anonymous tokens (DEMO ONLY, published in backend/README.md): the seeded anonymous requests
// A-005, A-017 and A-018 can be opened with them at /anonymous/lookup. Real tokens are random.
const (
	DemoTokenA005 = "DEM0-A005-7K3Q-9XWB-2HRT-M4ZP-C8NV"
	DemoTokenA017 = "DEM0-A017-5JPV-W2QX-8KTN-R6MD-H3ZB"
	DemoTokenA018 = "DEM0-A018-Q9RT-4MXK-7CWP-N2JH-B5VZ"
)

// department indexes
const (
	dPurchasing = iota
	dSales
	dDesign
	dAccounting
	dProjects
	dIT
	dAdminHR
	dDirections
)

type deptSeed struct{ name, head string } // head = user login ("" = no head)

var departments = []deptSeed{
	dPurchasing: {"Закупки", "purchasing.head"},
	dSales:      {"Продажи", "sales.head"},
	dDesign:     {"Проектирование", "design.head"},
	dAccounting: {"Бухгалтерия", "accounting.head"},
	dProjects:   {"Управление проектами", "projects.head"},
	dIT:         {"IT", ""}, // the dataset has no «Руководитель IT»: routed to the IT-специалист directly
	dAdminHR:    {"Административный отдел и HR", "admin.head"},
	dDirections: {"Руководство направлений", "direction.head"},
}

type userSeed struct {
	login, name, role string // name = the dataset's conditional role
	dept              int
}

var users = []userSeed{
	{"admin", "Администратор системы", "admin", dIT},
	// authors (dataset «Автор»)
	{"purchasing.employee", "Сотрудник закупок", "employee", dPurchasing},
	{"design.employee", "Сотрудник проектирования", "employee", dDesign},
	{"accounting.employee", "Сотрудник бухгалтерии", "employee", dAccounting},
	{"sales.manager", "Менеджер по продажам", "employee", dSales},
	{"project.coordinator", "Координатор проекта", "employee", dProjects},
	{"project.lead", "Руководитель проекта", "employee", dProjects},
	{"office.employee", "Сотрудник офиса", "employee", dAdminHR},
	// responsibles (dataset «Условный ответственный»)
	{"purchasing.head", "Руководитель закупок", "manager", dPurchasing},
	{"direction.head", "Руководитель направления", "manager", dDirections},
	{"catalog.owner", "Ответственный за номенклатуру", "manager", dPurchasing},
	{"training.owner", "Ответственный за обучение", "manager", dAdminHR},
	{"accounting.head", "Руководитель бухгалтерии", "manager", dAccounting},
	{"process.owner", "Ответственный за процессы", "manager", dDirections},
	{"it.specialist", "IT-специалист", "manager", dIT},
	{"unit.head", "Руководитель подразделения", "manager", dDirections},
	{"contracts.owner", "Ответственный за договоры", "manager", dSales},
	{"sales.head", "Руководитель продаж", "manager", dSales},
	{"projects.head", "Руководитель проектов", "manager", dProjects},
	{"design.head", "Руководитель проектирования", "manager", dDesign},
	{"admin.head", "Административный руководитель", "manager", dAdminHR},
	{"hr.comms", "HR / внутренние коммуникации", "manager", dAdminHR},
	{"documents.admin", "Администратор документов", "manager", dAdminHR},
}

// category indexes (the 18 distinct dataset topic labels; ARCHITECTURE §13 says "17 unique" but
// lists 18 — all 18 labels occur in the dataset, so all are created)
const (
	cInterdept = iota
	cInfoTransparency
	cCatalog
	cTraining
	cDocflow
	cRegulations
	cIntegrations
	cKnowledge
	cContracts
	cTasksComms
	cResponsiveness
	cComms
	cProjects
	cResources
	cTaskSetting
	cWorkplace
	cDocStorage
	cResultsTransparency
)

type catSeed struct{ name, icon, desc string } // icon = lucide icon name

var categories = []catSeed{
	cInterdept:           {"Межотдельное взаимодействие", "arrow-left-right", "Заявки и передача работы между подразделениями"},
	cInfoTransparency:    {"Информационная прозрачность", "eye", "Доступ к статусам и сведениям, нужным для работы"},
	cCatalog:             {"Каталог и данные", "database", "Номенклатура, карточки позиций и справочные данные"},
	cTraining:            {"Обучение", "graduation-cap", "Занятия, семинары и обучение сотрудников"},
	cDocflow:             {"Документооборот", "file-text", "Движение и согласование документов между подразделениями"},
	cRegulations:         {"Регламенты", "book-open", "Инструкции, памятки и порядок выполнения операций"},
	cIntegrations:        {"Интеграции и IT", "plug", "Рабочие системы, синхронизация данных и автоматизация"},
	cKnowledge:           {"Знания и наставничество", "users", "Передача опыта, наставничество и памятки для новичков"},
	cContracts:           {"Договорная работа", "file-signature", "Подготовка, проверка и согласование договоров"},
	cTasksComms:          {"Задачи и коммуникации", "list-checks", "Ведение рабочих задач и истории обсуждений"},
	cResponsiveness:      {"Оперативность", "timer", "Сроки реакции и уведомления о задержках"},
	cComms:               {"Коммуникации", "messages-square", "Каналы общения и фиксация итоговых решений"},
	cProjects:            {"Проектная деятельность", "folder-kanban", "Проекты, спецификации и история изменений"},
	cResources:           {"Планирование ресурсов", "calendar-range", "Загрузка команд и согласование сроков"},
	cTaskSetting:         {"Постановка задач", "target", "Цель, срок, приоритет и ответственный в поручениях"},
	cWorkplace:           {"Корпоративная среда", "building-2", "Рабочие помещения, атмосфера и внутренние мероприятия"},
	cDocStorage:          {"Хранение документов", "archive", "Общие папки, именование и архивирование версий"},
	cResultsTransparency: {"Прозрачность результатов", "trophy", "Обзор итогов и реализованных улучшений"},
}

// ruleSeed routes a category (optionally one type) to a department and the dataset's
// conditional responsible. «Хранение документов» intentionally has NO rule: A-019 lands in the
// unrouted admin queue and the admin assigns it manually (dataset scenario "альтернатива при
// отсутствии назначенного сотрудника").
type ruleSeed struct {
	cat      int
	typ      string // "" = any type
	dept     int
	assignee string // login
	fr, res  int    // hours
}

var rules = []ruleSeed{
	{cInterdept, "", dPurchasing, "purchasing.head", 48, 240},
	{cInfoTransparency, "", dDirections, "direction.head", 48, 240},
	{cCatalog, "", dPurchasing, "catalog.owner", 24, 168},
	{cTraining, "", dAdminHR, "training.owner", 72, 336},
	{cDocflow, "", dAccounting, "accounting.head", 24, 168},
	{cRegulations, "", dDirections, "process.owner", 48, 240},
	{cIntegrations, "", dIT, "it.specialist", 24, 120},
	{cKnowledge, "", dDirections, "unit.head", 72, 336},
	{cContracts, "", dSales, "contracts.owner", 24, 120},
	{cTasksComms, "", dSales, "sales.head", 48, 240},
	{cResponsiveness, "", dDirections, "direction.head", 24, 120},
	{cComms, "", dIT, "it.specialist", 48, 240},
	{cProjects, "", dProjects, "projects.head", 24, 168},
	{cResources, "", dDesign, "design.head", 48, 240},
	{cTaskSetting, "", dDirections, "direction.head", 48, 240},
	{cWorkplace, "", dAdminHR, "admin.head", 72, 336},
	{cWorkplace, "feedback", dAdminHR, "hr.comms", 72, 336}, // A-018: positive feedback goes to HR
	{cResultsTransparency, "", dDirections, "unit.head", 72, 336},
}

const (
	P = "problem"
	I = "idea"
	F = "feedback"
)

// reqSeed is one dataset request. Times are TEST DATES relative to "now": ageH = hours since
// creation, frH = hours from creation to the first staff response (0 = none yet), closeH = hours
// from creation to closing. note = «Пример дальнейшего действия» (internal note by the responsible),
// question = staff clarification for «Ожидает уточнения», result = result_text of closed requests.
type reqSeed struct {
	code, typ, prio, status string
	cat                     int
	author                  string // login
	responsible             string // login (for the unrouted A-019: assigned manually by the admin)
	anon                    bool
	token                   string
	title, body             string
	note, question, result  string
	ageH, frH, closeH       float64
}

// Dataset status -> ours: Новое=new, На рассмотрении=in_review, В работе=in_progress,
// Ожидает уточнения=needs_info, Реализовано=implemented, Отклонено=declined, Закрыто=closed.
var requestSeeds = []reqSeed{
	{code: "A-001", typ: P, prio: "medium", status: "new", cat: cInterdept, author: "purchasing.employee", responsible: "purchasing.head",
		title: "В заявках на оборудование не хватает назначения и параметров",
		body:  "В заявках на приобретение оборудования иногда не хватает назначения изделия и требуемых параметров. Предлагаю добавить обязательные поля, чтобы сократить число уточнений.",
		note:  "Запросить пример корректно заполненной заявки; оценить изменение шаблона.",
		ageH:  74}, // 48 h first-response SLA missed: overdue
	{code: "A-002", typ: I, prio: "medium", status: "in_review", cat: cInfoTransparency, author: "purchasing.employee", responsible: "direction.head",
		title: "Краткий статус проекта для общения с поставщиками",
		body:  "При общении с поставщиками полезно видеть общую стадию проекта без доступа к коммерческим деталям. Можно ли показывать краткий статус и ориентировочные контрольные даты?",
		note:  "Определить перечень доступных статусов и источник их обновления.",
		ageH:  8*24 + 5, frH: 20},
	{code: "A-003", typ: P, prio: "high", status: "in_progress", cat: cCatalog, author: "design.employee", responsible: "catalog.owner",
		title: "Разные обозначения одного изделия в рабочих материалах",
		body:  "В рабочих материалах иногда встречаются разные обозначения одного изделия. Это увеличивает время проверки спецификации. Нужен понятный способ находить актуальную карточку позиции.",
		note:  "Подготовить правило проверки и корректировки номенклатуры.",
		ageH:  12*24 + 3, frH: 6}, // 168 h resolution SLA missed: overdue
	{code: "A-004", typ: I, prio: "low", status: "implemented", cat: cTraining, author: "purchasing.employee", responsible: "training.owner",
		title:  "Короткие занятия по новым товарным линейкам",
		body:   "Было бы полезно проводить короткие занятия по новым товарным линейкам с разбором типовых запросов. Предлагаю составить календарь таких встреч.",
		result: "Запланирован пробный обучающий семинар.",
		ageH:   58*24 + 2, frH: 30, closeH: 17 * 24},
	{code: "A-005", typ: P, prio: "medium", status: "needs_info", cat: cDocflow, author: "accounting.employee", responsible: "accounting.head",
		anon: true, token: DemoTokenA005,
		title:    "Статус пакета документов между подразделениями",
		body:     "При обработке документов между подразделениями возникают дополнительные вопросы о том, на каком этапе находится пакет. Хотелось бы видеть ответственного и текущий статус.",
		question: "Уточните, пожалуйста, виды документов и ключевые этапы согласования.",
		ageH:     6*24 + 4, frH: 9},
	{code: "A-006", typ: I, prio: "medium", status: "in_review", cat: cRegulations, author: "accounting.employee", responsible: "process.owner",
		title: "Пошаговая памятка для операций нескольких подразделений",
		body:  "Предлагаю собрать короткую пошаговую памятку для операций, в которых последовательно участвуют несколько подразделений, и определить порядок актуализации памятки.",
		note:  "Назначить владельца процесса и согласовать черновик инструкции.",
		ageH:  5*24 + 6, frH: 26},
	{code: "A-007", typ: P, prio: "high", status: "new", cat: cIntegrations, author: "accounting.employee", responsible: "it.specialist",
		title: "Повторный ввод одних и тех же сведений в разных системах",
		body:  "Иногда повторный ввод одних и тех же сведений в разных рабочих системах занимает много времени. Нужна оценка, какие операции можно синхронизировать безопасно.",
		note:  "Собрать примеры операций и оценить техническую реализуемость.",
		ageH:  2*24 + 3}, // 24 h SLA missed: overdue
	{code: "A-008", typ: I, prio: "low", status: "implemented", cat: cKnowledge, author: "accounting.employee", responsible: "unit.head",
		title:  "Памятка при передаче сложной задачи новому сотруднику",
		body:   "При передаче сложной задачи новому сотруднику полезен короткий разбор типовых ошибок и контактов, к которым можно обратиться за помощью. Предлагаю сделать такую памятку.",
		result: "Создана учебная инструкция.",
		ageH:   50*24 + 7, frH: 52, closeH: 15 * 24},
	{code: "A-009", typ: P, prio: "high", status: "in_progress", cat: cContracts, author: "sales.manager", responsible: "contracts.owner",
		title: "Кто готовит и проверяет приложение к договору",
		body:  "Не всегда понятно, кто готовит очередную версию приложения к договору и кто должен проверить её перед отправкой. Это приводит к повторным согласованиям.",
		note:  "Разграничить этапы подготовки, проверки и утверждения.",
		ageH:  4*24 + 2, frH: 4},
	{code: "A-010", typ: I, prio: "medium", status: "in_review", cat: cTasksComms, author: "sales.manager", responsible: "sales.head",
		title: "Не закрывать задачу до подтверждения результата заказчиком",
		body:  "Предлагаю не закрывать рабочую задачу до подтверждения заказчиком результата внутри компании, чтобы связанные вопросы оставались в общей истории обсуждения.",
		note:  "Проверить, подходит ли правило для разных типов задач.",
		ageH:  7*24 + 1, frH: 55}, // answered after the 48 h deadline
	{code: "A-011", typ: P, prio: "high", status: "new", cat: cResponsiveness, author: "sales.manager", responsible: "direction.head",
		title: "Сроки ответа смежных подразделений при срочном предложении",
		body:  "При срочной подготовке клиентского предложения сложно прогнозировать сроки ответа от смежных подразделений. Хотелось бы видеть ожидаемый срок и уведомление при задержке.",
		note:  "Определить допустимые сроки реакции для разных категорий.",
		ageH:  6},
	{code: "A-012", typ: I, prio: "low", status: "in_review", cat: cComms, author: "sales.manager", responsible: "it.specialist",
		title: "Единое место для итоговых решений по задаче",
		body:  "Рабочие обсуждения распределены по нескольким каналам, из-за чего теряется контекст. Предлагаю выбрать единое место для итоговых решений по задаче.",
		note:  "Сравнить варианты и предложить правила фиксации итогов.",
		ageH:  3*24 + 5, frH: 11},
	{code: "A-013", typ: P, prio: "medium", status: "needs_info", cat: cProjects, author: "project.coordinator", responsible: "projects.head",
		title:    "История изменений спецификации при смене оборудования",
		body:     "Когда изменяется состав оборудования в проекте, не всегда ясно, какие строки спецификации были пересмотрены. Нужна более заметная история изменений.",
		question: "Уточните, пожалуйста, формат исходных файлов и способы сравнения версий.",
		ageH:     10*24 + 2, frH: 14},
	{code: "A-014", typ: I, prio: "medium", status: "in_review", cat: cResources, author: "project.coordinator", responsible: "design.head",
		title: "Загруженность проектной группы на ближайшие недели",
		body:  "Предлагаю показывать загруженность проектной группы на ближайшие недели в укрупнённом виде. Так будет проще согласовывать сроки запуска новых задач.",
		note:  "Согласовать показатели загрузки и права просмотра.",
		ageH:  9*24 + 4, frH: 61}, // answered after the 48 h deadline
	{code: "A-015", typ: P, prio: "medium", status: "in_progress", cat: cTaskSetting, author: "project.lead", responsible: "direction.head",
		title: "Поручения без ожидаемого результата и даты завершения",
		body:  "Некоторые поручения поступают без ожидаемого результата и даты завершения. Предлагаю договориться о минимальном наборе данных при постановке задачи.",
		note:  "Подготовить шаблон с целью, сроком, приоритетом и ответственным.",
		ageH:  9*24 + 1, frH: 18},
	{code: "A-016", typ: I, prio: "low", status: "new", cat: cTraining, author: "project.lead", responsible: "training.owner",
		title: "Практические демонстрации оборудования для новых сотрудников",
		body:  "Для новых сотрудников полезны практические демонстрации оборудования, а не только презентации. Предлагаю пилотное занятие с учебными образцами.",
		note:  "Проверить наличие образцов и составить план пилота.",
		ageH:  26},
	{code: "A-017", typ: I, prio: "low", status: "declined", cat: cWorkplace, author: "office.employee", responsible: "admin.head",
		anon: true, token: DemoTokenA017,
		title:  "Зона для коротких перерывов рядом с рабочими помещениями",
		body:   "Предлагаю организовать отдельную небольшую зону для коротких перерывов рядом с рабочими помещениями.",
		result: "Сейчас нет подходящего помещения; вернуться к вопросу при перепланировке.",
		ageH:   41*24 + 3, frH: 40, closeH: 3*24 + 6},
	{code: "A-018", typ: F, prio: "low", status: "closed", cat: cWorkplace, author: "office.employee", responsible: "hr.comms",
		anon: true, token: DemoTokenA018,
		title:  "Польза внутренних учебных встреч",
		body:   "Хочу отметить пользу внутренних учебных встреч: после них стало легче обращаться к коллегам за консультацией. Было бы хорошо сохранить этот формат.",
		note:   "Зафиксировать положительный отзыв и передать организаторам обучения.",
		result: "Положительный отзыв зафиксирован и передан организаторам обучения.",
		ageH:   33*24 + 5, frH: 22, closeH: 2*24 + 4},
	{code: "A-019", typ: P, prio: "medium", status: "in_review", cat: cDocStorage, author: "project.coordinator", responsible: "documents.admin",
		title: "Актуальная редакция инструкции в общих папках",
		body:  "В общих папках трудно быстро определить актуальную редакцию инструкции. Можно ли закрепить единые правила именования и архивирования старых версий?",
		note:  "Согласовать порядок хранения и владельцев папок.",
		ageH:  2*24 + 1, frH: 3},
	{code: "A-020", typ: I, prio: "low", status: "new", cat: cResultsTransparency, author: "purchasing.employee", responsible: "unit.head",
		title: "Краткий обзор завершённых внутренних улучшений",
		body:  "Предлагаю публиковать краткий обзор завершённых внутренних улучшений, чтобы сотрудники видели результат направленных предложений.",
		note:  "Рассмотреть формат квартальной подборки без конфиденциальных сведений.",
		ageH:  4*24 + 2}, // 72 h SLA missed: overdue
}
