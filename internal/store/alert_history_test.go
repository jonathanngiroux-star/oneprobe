package store

import (
	"testing"
	"time"
)

// TestAlertEventRecordingAndHistory pins the alert history store
// surface: every transition is append-only, newest first.
func TestAlertEventRecordingAndHistory(t *testing.T) {
	s := openTestStore(t)
	r, err := s.AddAlertRule(AlertRule{Name: "hist", Expr: "up < 1", Op: "<", Value: 1, Enabled: true})
	if err != nil {
		t.Fatalf("AddAlertRule: %v", err)
	}

	t0 := time.Unix(1759305600, 0).UTC()
	// firing event
	if err := s.RecordAlertEvent(r.ID, "firing", 0.0, t0); err != nil {
		t.Fatalf("RecordAlertEvent: %v", err)
	}
	// resolved event
	if err := s.RecordAlertEvent(r.ID, "resolved", 1.0, t0.Add(5*time.Minute)); err != nil {
		t.Fatalf("RecordAlertEvent: %v", err)
	}

	events, err := s.AlertHistory(r.ID, 10)
	if err != nil {
		t.Fatalf("AlertHistory: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	// Newest first.
	if events[0].State != "resolved" || events[1].State != "firing" {
		t.Errorf("order wrong: %+v", events)
	}
	if events[1].Value != 0.0 || !events[1].At.Equal(t0) {
		t.Errorf("firing event = %+v", events[1])
	}
	if !events[0].At.Equal(t0.Add(5 * time.Minute)) {
		t.Errorf("resolved event time = %v", events[0].At)
	}

	// Empty history is [], never null.
	other, _ := s.AddAlertRule(AlertRule{Name: "quiet", Expr: "up > 0", Op: ">", Value: 0, Enabled: true})
	events, err = s.AlertHistory(other.ID, 10)
	if err != nil {
		t.Fatalf("AlertHistory: %v", err)
	}
	if events == nil || len(events) != 0 {
		t.Errorf("empty history = %v, want non-nil empty", events)
	}
}
