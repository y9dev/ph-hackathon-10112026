# UX specification — Modus Voice

UI language: Russian (strings in `web/src/shared/i18n/ru.ts`). Tone: calm, respectful, plain words ("Обращение", "Предложение", "Проблема"), no jargon. Direction: trustworthy corporate tool for engineers — quiet slate neutrals, one teal brand colour, status colours used only for status. Desktop-first for managers, mobile-first for the submit flow (engineers in the field).

## 1. Information architecture
```
Employee:  Мои обращения (home) · Новое обращение · Уведомления
Manager:   Входящие (home) · Мои обращения · Новое обращение · Аналитика · Уведомления
Admin:     Входящие (all) · Аналитика · Администрирование (Маршрутизация | Категории | Пользователи) · Мои обращения · Уведомления
Shared:    Карточка обращения /requests/:id · Вход /login
```
Desktop: left sidebar (collapsible) + top bar with search, bell, user menu. Mobile (<768 px): bottom tab bar (3–4 items) + floating "+" for new request; tables become cards.

Flow map:
`Login → Home(role) → [Новое обращение → Успех(номер, владелец, срок) → Карточка] · [Список → Карточка → действия] · [Аналитика → клик по метрике/группе → Входящие с фильтром]`

## 2. Journeys (6-field)
| J | Actor | Trigger | Steps | Success | Failure/edge | Feedback |
|---|---|---|---|---|---|---|
| J1 Submit | Employee | Sees a problem/idea | + Новое → type (Проблема/Предложение) → category cards → title → description → (optional proposed solution, files, anonymity) → Отправить | Success page: number MV-0042, "Передано: отдел IT, ответ до 12.11", link "Следить" | Validation inline; routing unmatched → still accepted, says "передано администратору"; upload errors per file; draft preserved in localStorage | Routing preview shown *before* submit; toast + success screen |
| J2 Track | Employee | Wants status | Мои обращения → card → timeline | Sees status, owner dept, deadline, decision | Empty state with CTA; anonymous marker | Notification bell + unread dot; status chip in list |
| J3 Review | Manager | New/overdue request | Входящие (tab "Требуют реакции") → open → choose action: Взять в работу / Запросить уточнение / Принять / Отклонить / Дубликат → result form | Status changes, timeline entry, author notified | Closing without result blocked with inline message; reassign to other dept | Toast "Автор уведомлён"; SLA chip updates |
| J4 Clarify | Manager→Author | Missing info | Manager "Запросить уточнение" (status needs_info) → author sees banner "Нужен ваш ответ" → answers | Back to "На рассмотрении", SLA resumes | Author never answers: reminder visible in manager list ("ждём автора N дн.") | Notifications both ways |
| J5 Analyse | Manager/Admin | Weekly review | Аналитика → period/department filter → KPIs → groups → drill into list | Insight on recurring issues | Empty period state | Tooltips explain each metric |
| J6 Configure | Admin | Org changes | Администрирование → Маршрутизация table → edit row (dialog) | Next request follows rule | Duplicate rule rejected with message | Toast |

## 3. Screens and states (every screen: loading · empty · error · success)
1. **Вход** — email/password; in demo mode a "Демо-аккаунты" panel with one-click role buttons (Сотрудник, Руководитель IT, Руководитель сервиса, Администратор). States: invalid creds, throttled.
2. **Мои обращения** — list cards (number, title, status chip, category, updated, SLA/“нужен ваш ответ” banner). Filter chips by status. Empty: illustration-free text + "Создать обращение".
3. **Новое обращение** — single scrollable form, two columns on desktop (form | live "Что произойдёт дальше" panel: routing preview, deadline, privacy note, "N коллег уже сообщали о похожем" when applicable). Segmented control for type, category as selectable cards with description, character counters, anonymity toggle with plain-language explanation and limits, drag/drop + button for files (≤5, 10 MB). Primary button sticky on mobile.
4. **Успех** — big number, owner, deadline, "Что дальше" 3 steps, buttons "Открыть обращение", "Создать ещё".
5. **Входящие** — tabs with counts: Требуют реакции · В работе · Ждём автора · Закрытые · Все; filters (категория, отдел, статус, просрочено, мои); search; sort (срок ↑ default); desktop table with sticky header, mobile cards. Overdue rows have amber/red marker + text "Просрочено на 6 ч" (never colour-only).
6. **Карточка обращения** — header (number, type, status chip, SLA chip, category, department, owner, dates). Main: description, proposed solution, attachments, **timeline** (events + comments, internal notes visually distinct with lock icon), composer (comment / question / answer depending on role & status). Right panel (staff): action buttons, assign, "Похожие обращения" with score, "Добавить в тему", result block. Author view: result card highlighted when closed.
7. **Аналитика** — period + department filters; KPI row (4 TZ metrics + overdue + auto-routed share); status distribution; by category bars; trend line (created vs closed); "Повторяющиеся проблемы" list (suggested groups with score, confirm → theme); overdue list.
8. **Администрирование** — tabs: Маршрутизация (table: category/type → department → owner → SLA hours; add/edit dialog), Категории (name, description, active, order), Пользователи (name, email, role, department, active).
9. **Уведомления** — popover (latest 8) + full page; mark all read; each item deep-links.

## 4. Component set (design system v0)
Button (primary/secondary/ghost/danger, sizes, loading), Input, Textarea (counter), Select, SegmentedControl, CategoryCard, Switch, FileDrop, Badge/StatusChip, SlaChip, Card, Table/ResponsiveList, Tabs, Dialog, Popover, Toast, Skeleton, EmptyState, Timeline, KpiTile, BarList, LineChart, Avatar (initials), FormField (label/help/error).
All interactive components: default, hover, focus-visible (2 px ring), active, disabled, loading, error where applicable. Minimum target 40 px (44 on mobile).

## 5. Design tokens (CSS variables in `web/src/styles/tokens.css`, light + dark)
```
--bg:#F6F8F9;           --surface:#FFFFFF;      --surface-2:#EEF2F4;   --border:#D9E0E4;
--text:#16232B;         --text-muted:#52636E;   (contrast on --bg ≥ 7:1 / 5.2:1)
--brand:#0B7A75;        --brand-hover:#096762;  --brand-soft:#DDF3F1;  --on-brand:#FFFFFF
--info:#2457C5; --success:#1B7F43; --warning:#B45309; --danger:#B42318; (each with -soft bg)
status: new #2457C5 · in_review #6D3FC4 · needs_info #B45309 · accepted #0B7A75 · implemented #1B7F43 · declined #52636E · duplicate #52636E
dark: --bg:#0E1619 --surface:#151F24 --surface-2:#1C292F --border:#2B3B43 --text:#E6EEF1 --text-muted:#9FB1BA --brand:#34C5BC --brand-soft:#123B3A
font: Inter (self-hosted, cyrillic subset), system fallback; sizes 12/14/16/20/24/32; line-height 1.5 body, 1.25 headings
space: 4-pt scale (4,8,12,16,24,32,48); radius: 8 (controls) · 12 (cards) · 999 (chips)
shadow: sm 0 1px 2px rgb(16 35 43/.08); md 0 4px 16px rgb(16 35 43/.10)
motion: 120–200 ms ease-out; respects prefers-reduced-motion
```
Status is always icon + text + colour. SLA chips: ok (grey "до 12.11 18:00"), soon (<25% left, warning), overdue (danger, "Просрочено на 6 ч"), paused (info, "Ждём автора").

## 6. Copy rules for anonymity (shown in the form)
"Ваше имя не увидят ни руководитель, ни администратор — только вы. Но содержание обращения, вложения и время могут подсказать, кто его написал. Отвечать на уточнения вы сможете в системе." Checkbox label: "Отправить анонимно".

## 7. Accessibility & responsiveness acceptance
Semantic landmarks, labelled fields, visible focus, errors tied via `aria-describedby`, dialogs trap focus, toasts `aria-live=polite`, keyboard path for all journeys, contrast ≥ 4.5:1 (checked), no colour-only meaning, works 360–1440 px.
