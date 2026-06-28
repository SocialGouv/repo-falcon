package memory

import (
	"testing"
	"time"
)

func TestReflectClassifies(t *testing.T) {
	now := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)
	day := func(d int) time.Time { return now.AddDate(0, 0, -d) }
	recs := []Record{
		// Pref: cited useful by two distinct recent results.
		{Question: "q1", Nodes: []string{"Pref"}, Outcome: "useful", Time: day(1)},
		{Question: "q2", Nodes: []string{"Pref"}, Outcome: "useful", Time: day(2)},
		// Tent: useful once only.
		{Question: "q3", Nodes: []string{"Tent"}, Outcome: "useful", Time: day(1)},
		// Cont: both useful and dead_end.
		{Question: "q4", Nodes: []string{"Cont"}, Outcome: "useful", Time: day(40)},
		{Question: "q5", Nodes: []string{"Cont"}, Outcome: "dead_end", Time: day(1)},
		// Dead end + correction records.
		{Question: "dead question", Outcome: "dead_end", Time: day(1)},
		{Question: "wrong q", Outcome: "corrected", Correction: "the truth", Time: day(1)},
	}
	l := Reflect(recs, now, 30, 2)

	if len(l.Preferred) != 1 || l.Preferred[0].Node != "Pref" {
		t.Errorf("expected Pref preferred, got %+v", l.Preferred)
	}
	if len(l.Tentative) != 1 || l.Tentative[0].Node != "Tent" {
		t.Errorf("expected Tent tentative, got %+v", l.Tentative)
	}
	if len(l.Contested) != 1 || l.Contested[0].Node != "Cont" {
		t.Errorf("expected Cont contested, got %+v", l.Contested)
	}
	// Two dead_end records (q5 on Cont + the standalone "dead question") and one
	// correction.
	if len(l.DeadEnds) != 2 || len(l.Corrections) != 1 {
		t.Errorf("dead ends=%d corrections=%d", len(l.DeadEnds), len(l.Corrections))
	}
}

func TestReflectDeterministicRender(t *testing.T) {
	now := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)
	recs := []Record{
		{Question: "a", Nodes: []string{"X"}, Outcome: "useful", Time: now.AddDate(0, 0, -1)},
		{Question: "b", Nodes: []string{"X"}, Outcome: "useful", Time: now.AddDate(0, 0, -2)},
	}
	if RenderLessons(Reflect(recs, now, 30, 2)) != RenderLessons(Reflect(recs, now, 30, 2)) {
		t.Error("render must be byte-stable for the same input and now")
	}
}
