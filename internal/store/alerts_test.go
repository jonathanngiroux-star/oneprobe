package store

import (
	"testing"
	"time"
)

func TestAlertRuleCRUDRoundTrips(t *testing.T) {
	s := openTestStore(t)
	r, err := s.AddAlertRule(AlertRule{
		Name: "high-cpu", Expr: "avg by (host) (cpu_idle) < 10", Op: "<", Value: 10,
		ForSecs: 300, WebhookURL: "http://127.0.0.1:9999/hook", Enabled: true,
	})
	if err != nil {
		t.Fatalf("AddAlertRule: %v", err)
	}
	if r.ID == 0 || r.CreatedAt.IsZero() {
		t.Fatalf("AddAlertRule did not stamp id/created: %+v", r)
	}

	rules, err := s.ListAlertRules()
	if err != nil {
		t.Fatalf("ListAlertRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}
	got := rules[0]
	if got.Name != "high-cpu" || got.Expr != "avg by (host) (cpu_idle) < 10" || got.Op != "<" ||
		got.Value != 10 || got.ForSecs != 300 || got.WebhookURL != "http://127.0.0.1:9999/hook" || !got.Enabled {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestAlertRuleUpsertOnName(t *testing.T) {
	s := openTestStore(t)
	first, err := s.AddAlertRule(AlertRule{Name: "dup", Expr: "up < 1", Op: "<", Value: 1, Enabled: true})
	if err != nil {
		t.Fatalf("AddAlertRule: %v", err)
	}
	second, err := s.AddAlertRule(AlertRule{Name: "dup", Expr: "up < 0.5", Op: "<", Value: 0.5, Enabled: true})
	if err != nil {
		t.Fatalf("AddAlertRule upsert: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("same-name rule should upsert to the same ID: %d != %d", second.ID, first.ID)
	}
	rules, _ := s.ListAlertRules()
	if len(rules) != 1 || rules[0].Expr != "up < 0.5" {
		t.Errorf("upsert did not update: %+v", rules)
	}
}

func TestAlertRuleValidationRejectsBadInput(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.AddAlertRule(AlertRule{Name: "", Expr: "up < 1", Op: "<", Value: 1}); err == nil {
		t.Error("must reject an empty name")
	} else if !containsStr(err.Error(), "name") {
		t.Errorf("refusal must name the field, got: %v", err)
	}
	if _, err := s.AddAlertRule(AlertRule{Name: "x", Expr: "", Op: "<", Value: 1}); err == nil {
		t.Error("must reject an empty expr")
	} else if !containsStr(err.Error(), "expr") {
		t.Errorf("refusal must name the field, got: %v", err)
	}
	if _, err := s.AddAlertRule(AlertRule{Name: "x", Expr: "up < 1", Op: "~", Value: 1}); err == nil {
		t.Error("must reject an unknown comparison op")
	} else if !containsStr(err.Error(), `op "~"`) {
		t.Errorf("refusal must name the op, got: %v", err)
	}
}

func TestAlertStatePersistsAndTransitions(t *testing.T) {
	s := openTestStore(t)
	r, err := s.AddAlertRule(AlertRule{Name: "st", Expr: "up < 1", Op: "<", Value: 1, Enabled: true})
	if err != nil {
		t.Fatalf("AddAlertRule: %v", err)
	}

	// Default state is resolved with zero values.
	st, err := s.GetAlertState(r.ID)
	if err != nil {
		t.Fatalf("GetAlertState: %v", err)
	}
	if st.State != "resolved" {
		t.Errorf("initial state = %q, want resolved", st.State)
	}

	// Transition to firing.
	at := time.Unix(1759305600, 0).UTC()
	if err := s.SetAlertState(r.ID, "firing", 0.25, at); err != nil {
		t.Fatalf("SetAlertState: %v", err)
	}
	st, err = s.GetAlertState(r.ID)
	if err != nil {
		t.Fatalf("GetAlertState: %v", err)
	}
	if st.State != "firing" || st.LastValue != 0.25 || !st.LastEvalAt.Equal(at) || !st.ChangedAt.Equal(at) {
		t.Errorf("firing state = %+v", st)
	}

	// Same-state update refreshes eval time but not changed_at.
	later := at.Add(15 * time.Second)
	if err := s.SetAlertState(r.ID, "firing", 0.1, later); err != nil {
		t.Fatalf("SetAlertState: %v", err)
	}
	st, _ = s.GetAlertState(r.ID)
	if !st.LastEvalAt.Equal(later) || !st.ChangedAt.Equal(at) {
		t.Errorf("same-state update moved changed_at: %+v", st)
	}

	// Transition back to resolved.
	if err := s.SetAlertState(r.ID, "resolved", 1, later.Add(time.Second)); err != nil {
		t.Fatalf("SetAlertState: %v", err)
	}
	st, _ = s.GetAlertState(r.ID)
	if st.State != "resolved" || !st.ChangedAt.Equal(later.Add(time.Second)) {
		t.Errorf("resolved state = %+v", st)
	}
}

func TestDeleteAlertRuleCascadesState(t *testing.T) {
	s := openTestStore(t)
	r, err := s.AddAlertRule(AlertRule{Name: "gone", Expr: "up < 1", Op: "<", Value: 1, Enabled: true})
	if err != nil {
		t.Fatalf("AddAlertRule: %v", err)
	}
	if err := s.SetAlertState(r.ID, "firing", 0, time.Unix(1759305600, 0).UTC()); err != nil {
		t.Fatalf("SetAlertState: %v", err)
	}
	if err := s.DeleteAlertRule(r.ID); err != nil {
		t.Fatalf("DeleteAlertRule: %v", err)
	}
	// The state ROW must be gone (cascade). GetAlertState maps a missing
	// row to "resolved", so assert at the SQL level.
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM alert_state WHERE rule_id = ?`, r.ID).Scan(&n); err != nil {
		t.Fatalf("count state rows: %v", err)
	}
	if n != 0 {
		t.Errorf("state row survived rule delete: %d rows", n)
	}
}

func containsStr(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
