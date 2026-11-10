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
// For anonymous requests ID is 0 and Name is empty: no author link exists in the DB (CR2).
// While serving the token holder (anonymous author endpoints) ID is set to anonSelfID in memory.
type AuthorRef struct {
	ID          int64
	Name        string
	IsAnonymous bool
}

// anonSelfID is an in-memory sentinel: the viewer holding an anonymous request's token is mapped
// as its author (viewer.UserID == AuthorRef.ID == anonSelfID). It never reaches the database.
const anonSelfID int64 = -1

// anonAuthorViewer is the Actor used to render a request for its anonymous token holder.
func anonAuthorViewer() authz.Actor { return authz.Actor{UserID: anonSelfID, Role: authz.RoleEmployee} }

// isAuthorOf reports whether viewer is the author of r (never true for a zero id).
func isAuthorOf(viewer authz.Actor, author AuthorRef) bool {
	return author.ID != 0 && viewer.UserID == author.ID
}

// AuthorView maps the request author for viewer. Anonymous requests always show the fixed
// anonymous label (also to the token holder, who gets is_me=true); named authors show their name.
func AuthorView(author AuthorRef, viewer authz.Actor) PersonView {
	isMe := isAuthorOf(viewer, author)
	if author.IsAnonymous {
		return PersonView{Name: AnonymousLabel, Anonymous: true, IsMe: isMe}
	}
	return PersonView{Name: author.Name, IsMe: isMe}
}

// ActorView maps the actor of a timeline entry or notification. Author-side rows (byAuthor, or the
// actor id equals the named author) are masked exactly like the author; staff carry their role
// label; a nil actor that is not the author is the system.
func ActorView(actorID *int64, byAuthor bool, actorName string, actorRole authz.Role, author AuthorRef, viewer authz.Actor) PersonView {
	if byAuthor || (actorID != nil && author.ID != 0 && *actorID == author.ID) {
		return AuthorView(author, viewer)
	}
	if actorID == nil {
		return PersonView{Name: SystemLabel}
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
