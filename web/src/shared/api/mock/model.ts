// Internal (server-side) model used by the in-browser mock. Mirrors the backend
// tables, NOT the API DTOs. CR2: anonymous requests store NO author (author_id null);
// author-side rows are marked by_author and the token is kept only as its canonical hash stand-in.
import type { CommentKind, Priority, RequestType, Role, RoutingSource, Status, TimelineKind } from '../types';

export interface MUser {
  id: number;
  name: string;
  email: string;
  password: string;
  role: Role;
  department_id: number | null;
  active: boolean;
  title: string;
}
export interface MDept {
  id: number;
  name: string;
  head_user_id: number | null;
}
export interface MCategory {
  id: number;
  name: string;
  description: string;
  icon: string;
  sort: number;
  active: boolean;
}
export interface MRule {
  id: number;
  category_id: number;
  request_type: RequestType | null;
  department_id: number;
  assignee_user_id: number | null;
  first_response_hours: number;
  resolution_hours: number;
  active: boolean;
}
export interface MAttachment {
  id: number;
  filename: string;
  mime: string;
  size: number;
}
export interface MRequest {
  id: number;
  number: string;
  type: RequestType;
  category_id: number;
  title: string;
  body: string;
  proposed_solution: string | null;
  /** null for anonymous requests (CR2: no author link anywhere). */
  author_id: number | null;
  is_anonymous: boolean;
  /** Canonical anonymous token (the backend stores its SHA-256; the mock keeps the canonical form). */
  anon_token: string | null;
  priority: Priority;
  department_id: number | null;
  assignee_id: number | null;
  status: Status;
  routing_source: RoutingSource;
  first_response_due: string;
  resolution_due: string;
  first_responded_at: string | null;
  paused_at: string | null;
  closed_at: string | null;
  result_text: string | null;
  duplicate_of: number | null;
  theme_id: number | null;
  created_at: string;
  updated_at: string;
  attachments: MAttachment[];
}
export interface MComment {
  id: number;
  request_id: number;
  /** null for comments written by an anonymous author (by_author = true). */
  author_id: number | null;
  by_author?: boolean;
  kind: CommentKind;
  body: string;
  created_at: string;
}
export interface MEvent {
  id: number;
  request_id: number;
  actor_id: number | null;
  by_author?: boolean;
  kind: Exclude<TimelineKind, 'comment' | 'question' | 'answer' | 'internal' | 'report'>;
  from_status?: Status;
  to_status?: Status;
  text?: string | null;
  created_at: string;
}
export interface MNotification {
  id: number;
  user_id: number;
  kind: string;
  request_id: number;
  title: string;
  body: string;
  read: boolean;
  created_at: string;
}
export interface MTheme {
  id: number;
  title: string;
  created_at: string;
}

export interface MockDb {
  version: number;
  seq: number;
  users: MUser[];
  departments: MDept[];
  categories: MCategory[];
  rules: MRule[];
  requests: MRequest[];
  comments: MComment[];
  events: MEvent[];
  notifications: MNotification[];
  themes: MTheme[];
  /** CR4: Telegram links per user (mock of telegram_links). */
  telegramLinks: Record<number, { linked_at: string; username: string }>;
  telegramEnabled: boolean;
  sessionUserId: number | null;
  failedLogins: number;
  numberCounter: number;
}
