// Types mirror docs/api/openapi.yaml (FROZEN CONTRACT). Keep field names in snake_case.

export type Role = 'employee' | 'manager' | 'admin';
export type RequestType = 'problem' | 'idea';
export type Status = 'new' | 'in_review' | 'needs_info' | 'decision' | 'in_progress' | 'implemented' | 'declined' | 'duplicate' | 'closed';
export type CommentKind = 'comment' | 'question' | 'answer' | 'internal' | 'report';
export type InboxTab = 'needs_action' | 'in_progress' | 'waiting_author' | 'closed' | 'all';
export type ListScope = 'mine' | 'inbox' | 'all';
export type ListSort = 'deadline' | 'newest' | 'oldest' | 'updated';
export type SlaStateKind = 'ok' | 'soon' | 'overdue' | 'paused' | 'done';
export type RoutingSource = 'rule' | 'manual' | 'unrouted';
export type TimelineKind =
  | 'created'
  | 'routed'
  | 'assigned'
  | 'status_changed'
  | 'comment'
  | 'question'
  | 'answer'
  | 'internal'
  | 'report'
  | 'theme_linked'
  | 'overdue';

export interface ApiErrorBody {
  error: { code: string; message: string; fields?: Record<string, string> | null };
}

export interface User {
  id: number;
  name: string;
  email: string;
  role: Role;
  department_id: number | null;
  department_name: string | null;
  led_department_ids: number[];
}

export interface DemoUser {
  email: string;
  name: string;
  role: string;
  title: string;
}

export interface DemoUsers {
  password_hint: string;
  users: DemoUser[];
}

export interface Department {
  id: number;
  name: string;
  head_user_id: number | null;
  head_name: string | null;
}

export interface Category {
  id: number;
  name: string;
  description: string;
  icon: string;
  sort: number;
  active: boolean;
}

export type CategoryInput = Partial<Omit<Category, 'id'>>;

export interface MetaLimits {
  max_files: number;
  max_file_mb: number;
  max_total_mb: number;
  allowed_extensions: string[];
  title_max: number;
  body_max: number;
}

export interface Meta {
  categories: Category[];
  departments: Department[];
  statuses: { id: Status; label: string; terminal: boolean }[];
  limits: MetaLimits;
  assignable_users: { id: number; name: string; department_id: number }[];
}

export interface RoutingPreview {
  routed: boolean;
  department: Department | null;
  assignee_name: string | null;
  first_response_hours: number;
  resolution_hours: number;
  first_response_due: string;
  similar_count: number;
}

export interface CreateRequest {
  type: RequestType;
  category_id: number;
  title: string;
  body: string;
  proposed_solution?: string;
  is_anonymous?: boolean;
}

export interface PersonView {
  name: string;
  anonymous: boolean;
  is_me: boolean;
  role?: string | null;
}

export interface SlaState {
  first_response_due: string;
  resolution_due: string;
  first_responded_at: string | null;
  state: SlaStateKind;
  overdue_kind: 'first_response' | 'resolution' | null;
  overdue_hours: number;
}

export interface IdName {
  id: number;
  name: string;
}

export interface RequestSummary {
  id: number;
  number: string;
  type: RequestType;
  category: IdName;
  title: string;
  status: Status;
  author: PersonView;
  department: IdName | null;
  assignee: IdName | null;
  routing_source: RoutingSource;
  sla: SlaState;
  comments_count: number;
  awaiting_author: boolean;
  created_at: string;
  updated_at: string;
  closed_at: string | null;
}

export interface TabCounts {
  needs_action: number;
  in_progress: number;
  waiting_author: number;
  closed: number;
  all: number;
  overdue: number;
}

export interface RequestList {
  items: RequestSummary[];
  total: number;
  page: number;
  page_size: number;
  counts?: TabCounts | null;
}

export interface Attachment {
  id: number;
  filename: string;
  mime: string;
  size: number;
}

export interface TimelineItem {
  id: string;
  at: string;
  actor: PersonView | null;
  kind: TimelineKind;
  from_status?: Status | null;
  to_status?: Status | null;
  text: string | null;
}

export interface SimilarItem {
  id: number;
  number: string;
  title: string;
  status: Status;
  score: number;
}

export interface RequestDetail extends RequestSummary {
  body: string;
  proposed_solution: string | null;
  result_text: string | null;
  duplicate_of: { id: number; number: string } | null;
  theme: { id: number; title: string } | null;
  attachments: Attachment[];
  timeline: TimelineItem[];
  allowed_statuses: Status[];
  allowed_comment_kinds: CommentKind[];
  can_assign: boolean;
  similar: { count: number; items?: SimilarItem[] | null };
}

export interface Notification {
  id: number;
  kind: string;
  request_id: number;
  request_number: string;
  title: string;
  body: string;
  read: boolean;
  created_at: string;
}

export interface NotificationList {
  unread_count: number;
  items: Notification[];
}

export interface Theme {
  id: number;
  title: string;
  count: number;
  open_count: number;
  categories: string[];
}

export interface SuggestedGroup {
  key: string;
  title: string;
  count: number;
  score: number;
  categories: string[];
  requests: { id: number; number: string; title: string; status: Status }[];
}

export interface AnalyticsSummary {
  kpis: {
    total: number;
    reviewed_share: number;
    median_first_response_hours: number | null;
    p90_first_response_hours: number | null;
    outcome_share: number;
    implemented_count: number;
    overdue_count: number;
    auto_routed_share: number;
  };
  by_status: { status: Status; count: number }[];
  by_category: { category: string; count: number; implemented: number; declined: number; open: number }[];
  by_department: { department: string; count: number; open: number; overdue: number }[];
  timeseries: { date: string; created: number; closed: number }[];
  overdue: RequestSummary[];
}

export interface RoutingRule {
  id: number;
  category_id: number;
  category_name: string;
  request_type: RequestType | null;
  department_id: number;
  department_name: string;
  assignee_user_id: number | null;
  assignee_name: string | null;
  first_response_hours: number;
  resolution_hours: number;
  active: boolean;
}

export interface RoutingRuleInput {
  category_id: number;
  request_type: RequestType | null;
  department_id: number;
  assignee_user_id: number | null;
  first_response_hours: number;
  resolution_hours: number;
  active: boolean;
}

export interface AdminUser {
  id: number;
  name: string;
  email: string;
  role: Role;
  department_id: number | null;
  active: boolean;
}

export interface AdminUserInput {
  name?: string;
  email?: string;
  role?: Role;
  department_id?: number | null;
  active?: boolean;
  password?: string;
}

export interface ListParams {
  scope: ListScope;
  tab?: InboxTab;
  status?: Status;
  category_id?: number;
  department_id?: number;
  assignee?: 'me' | 'none';
  overdue?: boolean;
  q?: string;
  sort?: ListSort;
  page?: number;
  page_size?: number;
}

export interface ItemsOf<T> {
  items: T[];
}
