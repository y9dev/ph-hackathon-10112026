package insights

import (
	"testing"
	"time"
)

func TestOnTimeShareAndMedians(t *testing.T) {
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	at := func(h float64) *time.Time { v := t0.Add(time.Duration(h * float64(time.Hour))); return &v }
	fact := func(frH, dueH float64, status string, closeH float64) Fact {
		f := Fact{Status: status, CreatedAt: t0, FirstResponseDue: *at(dueH)}
		if frH > 0 {
			f.FirstRespondedAt = at(frH)
		}
		if closeH > 0 {
			f.ClosedAt = at(closeH)
		}
		return f
	}
	tests := []struct {
		name      string
		facts     []Fact
		share     *float64
		sample    int
		medFirst  *float64
		medResolv *float64
	}{
		{"empty", nil, nil, 0, nil, nil},
		{"unanswered only", []Fact{fact(0, 24, "new", 0)}, nil, 0, nil, nil},
		{"exactly at deadline counts as on time", []Fact{fact(24, 24, "in_review", 0)}, f64(1), 1, f64(24), nil},
		{"one late of two", []Fact{fact(2, 24, "closed", 40), fact(30, 24, "in_review", 0), fact(0, 24, "new", 0)}, f64(0.5), 2, f64(16), f64(40)},
		{"pause-shifted deadline respected", []Fact{fact(30, 34, "in_review", 0)}, f64(1), 1, f64(30), nil},
		{"three thirds", []Fact{fact(1, 24, "implemented", 10), fact(2, 24, "declined", 20), fact(50, 24, "closed", 60)}, f64(0.667), 3, f64(2), f64(20)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := OnTimeShare(tc.facts); !eqPtr(got, tc.share) {
				t.Errorf("share %v want %v", deref(got), deref(tc.share))
			}
			first, res := FirstResponseAndResolution(tc.facts)
			if len(first) != tc.sample {
				t.Errorf("sample %d want %d", len(first), tc.sample)
			}
			if got := median(first); !eqPtr(got, tc.medFirst) {
				t.Errorf("median first %v want %v", deref(got), deref(tc.medFirst))
			}
			if got := median(res); !eqPtr(got, tc.medResolv) {
				t.Errorf("median resolution %v want %v", deref(got), deref(tc.medResolv))
			}
		})
	}
}

func TestByType(t *testing.T) {
	tests := []struct {
		name  string
		facts []Fact
		want  []TypeCount
	}{
		{"empty keeps all types", nil, []TypeCount{{"problem", 0, 0}, {"idea", 0, 0}, {"feedback", 0, 0}}},
		{"counts and closed", []Fact{{Type: "problem", Status: "new"}, {Type: "problem", Status: "declined"}, {Type: "feedback", Status: "closed"}, {Type: "idea", Status: "in_progress"}},
			[]TypeCount{{"problem", 2, 1}, {"idea", 1, 0}, {"feedback", 1, 1}}},
		{"unknown type ignored", []Fact{{Type: "bug", Status: "new"}}, []TypeCount{{"problem", 0, 0}, {"idea", 0, 0}, {"feedback", 0, 0}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ByType(tc.facts)
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v", got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("got %+v want %+v", got, tc.want)
				}
			}
		})
	}
}

func f64(v float64) *float64 { return &v }

func eqPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
