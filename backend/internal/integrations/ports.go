// Package integrations defines the PORTS external systems plug into (INTEGRATIONS.md).
// Business modules never import an adapter; adapters are wired in app/modules.go.
// Built: LogNotifier (logs domain events as outgoing notifications).
// Documented stubs: bitrix24, onec, email (see sub-packages).
package integrations

import (
	"context"
	"time"
)

// Message is an outgoing notification for an external channel. It carries ids, numbers and
// statuses only; anonymous authorship is never exported.
type Message struct {
	Topic     string
	RequestID int64
	Number    string
	Text      string
	At        time.Time
}

// Notifier delivers messages to an external channel (Bitrix24 im.notify, e-mail, webhook).
type Notifier interface {
	Notify(ctx context.Context, m Message) error
}

// DirectoryUser is a user record from an external directory (Bitrix24 user.get, AD).
type DirectoryUser struct {
	ExternalID   string
	Email        string
	Name         string
	DepartmentID string
	Active       bool
}

// DirectoryDepartment is a department from an external directory.
type DirectoryDepartment struct {
	ExternalID     string
	Name           string
	HeadExternalID string
}

// DirectoryProvider syncs users and org structure (maps to users.external_id/source).
type DirectoryProvider interface {
	Users(ctx context.Context) ([]DirectoryUser, error)
	Departments(ctx context.Context) ([]DirectoryDepartment, error)
}

// Task is an accepted improvement exported as tracked work.
type Task struct {
	RequestID   int64
	Number      string
	Title       string
	Link        string
	Deadline    time.Time
	AssigneeRef string
}

// TaskSink creates tasks in an external tracker (Bitrix24 tasks.task.add) on request.accepted.
type TaskSink interface {
	CreateTask(ctx context.Context, t Task) (externalID string, err error)
}

// ExternalIdentity is the result of an SSO login.
type ExternalIdentity struct {
	Source     string
	ExternalID string
	Email      string
	Name       string
}

// IdentityProvider authenticates through an external IdP (OAuth2/OIDC); local login stays as fallback.
type IdentityProvider interface {
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (ExternalIdentity, error)
}
