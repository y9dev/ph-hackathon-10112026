package requests

import "modusvoice/internal/platform/authz"

// PersonView is the contract PersonView produced ONLY by the mappers in this file (R2).
type PersonView struct {
	Name      string  `json:"name"`
	Anonymous bool    `json:"anonymous"`
	IsMe      bool    `json:"is_me"`
	Role      *string `json:"role"`
}

// AuthorRef is the internal author identity of a request; it never leaves the service.
type AuthorRef struct {
	ID          int64
	Name        string
	IsAnonymous bool
}

// AuthorView maps the request author for viewer. For anonymous requests every non-author
// (including admins) gets the fixed anonymous label; the author sees their own name.
func AuthorView(author AuthorRef, viewer authz.Actor) PersonView {
	isMe := viewer.UserID == author.ID
	if author.IsAnonymous && !isMe {
		return PersonView{Name: AnonymousLabel, Anonymous: true}
	}
	return PersonView{Name: author.Name, Anonymous: author.IsAnonymous, IsMe: isMe}
}

// ActorView maps the actor of a timeline entry or notification. Actions by the author are
// masked exactly like the author; staff carry their role label; nil actor = system.
func ActorView(actorID *int64, actorName string, actorRole authz.Role, author AuthorRef, viewer authz.Actor) PersonView {
	if actorID == nil {
		return PersonView{Name: SystemLabel}
	}
	if *actorID == author.ID {
		return AuthorView(author, viewer)
	}
	var role *string
	if actorRole == authz.RoleManager || actorRole == authz.RoleAdmin {
		l := authz.RoleLabel(actorRole)
		role = &l
	}
	return PersonView{Name: actorName, IsMe: *actorID == viewer.UserID, Role: role}
}

// recipientViewer builds a minimal viewer for notification text rendering.
func recipientViewer(userID int64) authz.Actor { return authz.Actor{UserID: userID} }
