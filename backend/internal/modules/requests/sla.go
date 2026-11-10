package requests

import (
	"math"
	"time"
)

// SLA states.
const (
	SLAOk      = "ok"
	SLASoon    = "soon"
	SLAOverdue = "overdue"
	SLAPaused  = "paused"
	SLADone    = "done"

	OverdueFirstResponse = "first_response"
	OverdueResolution    = "resolution"

	// soonFraction: a deadline is "soon" when less than this share of its window remains.
	soonFraction = 0.25
)

// SLAInput is what the SLA computation needs from a request.
type SLAInput struct {
	Status           Status
	CreatedAt        time.Time
	FirstResponseDue time.Time
	ResolutionDue    time.Time
	FirstRespondedAt *time.Time
}

// SLAState is the contract SlaState.
type SLAState struct {
	FirstResponseDue time.Time  `json:"first_response_due"`
	ResolutionDue    time.Time  `json:"resolution_due"`
	FirstRespondedAt *time.Time `json:"first_responded_at"`
	State            string     `json:"state"`
	OverdueKind      *string    `json:"overdue_kind"`
	OverdueHours     int        `json:"overdue_hours"`
}

// OverdueKindAt returns which deadline is overdue at now ("" if none). The clock is paused
// in needs_info and stopped in terminal statuses (R3).
func OverdueKindAt(in SLAInput, now time.Time) string {
	if in.Status.Terminal() || in.Status == StatusNeedsInfo {
		return ""
	}
	if in.FirstRespondedAt == nil && now.After(in.FirstResponseDue) {
		return OverdueFirstResponse
	}
	if now.After(in.ResolutionDue) {
		return OverdueResolution
	}
	return ""
}

// ComputeSLA derives the SLA state at now.
func ComputeSLA(in SLAInput, now time.Time) SLAState {
	st := SLAState{FirstResponseDue: in.FirstResponseDue, ResolutionDue: in.ResolutionDue, FirstRespondedAt: in.FirstRespondedAt}
	switch {
	case in.Status.Terminal():
		st.State = SLADone
		return st
	case in.Status == StatusNeedsInfo:
		st.State = SLAPaused
		return st
	}
	if kind := OverdueKindAt(in, now); kind != "" {
		due := in.ResolutionDue
		if kind == OverdueFirstResponse {
			due = in.FirstResponseDue
		}
		k := kind
		st.State = SLAOverdue
		st.OverdueKind = &k
		st.OverdueHours = int(math.Ceil(now.Sub(due).Hours()))
		if st.OverdueHours < 1 {
			st.OverdueHours = 1
		}
		return st
	}
	due := in.ResolutionDue
	if in.FirstRespondedAt == nil {
		due = in.FirstResponseDue
	}
	window := due.Sub(in.CreatedAt)
	if window > 0 && float64(due.Sub(now)) < soonFraction*float64(window) {
		st.State = SLASoon
	} else {
		st.State = SLAOk
	}
	return st
}

// applyTransition changes the status and maintains the SLA clock: entering needs_info pauses it,
// leaving needs_info shifts both deadlines by the paused duration; terminal statuses stamp closed_at.
// The caller has already checked CanTransition.
func applyTransition(r *Row, to Status, now time.Time) {
	if r.Status == StatusNeedsInfo && to != StatusNeedsInfo && r.PausedAt != nil {
		r.FirstResponseDue = ShiftForPause(r.FirstResponseDue, *r.PausedAt, now)
		r.ResolutionDue = ShiftForPause(r.ResolutionDue, *r.PausedAt, now)
		r.PausedAt = nil
	}
	if to == StatusNeedsInfo && r.Status != StatusNeedsInfo {
		p := now
		r.PausedAt = &p
	}
	r.Status = to
	if to.Terminal() {
		c := now
		r.ClosedAt = &c
	}
}

// ShiftForPause moves a deadline forward by the time spent paused (needs_info).
func ShiftForPause(due, pausedAt, now time.Time) time.Time {
	if now.Before(pausedAt) {
		return due
	}
	return due.Add(now.Sub(pausedAt))
}
