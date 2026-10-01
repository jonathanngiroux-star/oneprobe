package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	s, err := Open(db)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenCreatesSchemaAndTablesExist(t *testing.T) {
	s := openTestStore(t)
	for _, table := range []string{"targets", "samples", "alert_rules", "alert_state", "meta"} {
		var name string
		err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing after Open: %v", table, err)
		}
	}
	var v int
	if err := s.db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&v); err != nil {
		t.Fatalf("schema_version row missing: %v", err)
	}
	if v != SchemaVersion {
		t.Errorf("schema_version = %d, want %d", v, SchemaVersion)
	}
}

func TestAddTargetAndListTargetsRoundTrips(t *testing.T) {
	s := openTestStore(t)
	now := time.Unix(1759305600, 0).UTC()
	added, err := s.AddTarget(Target{
		Name:     "web-1",
		URL:      "http://127.0.0.1:9100/metrics",
		Interval: 15 * time.Second,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	if added.ID == 0 {
		t.Errorf("AddTarget did not assign an ID")
	}
	// Upsert on the same URL updates, not duplicates.
	second, err := s.AddTarget(Target{Name: "web-1-rename", URL: "http://127.0.0.1:9100/metrics", Interval: 30 * time.Second, Enabled: true})
	if err != nil {
		t.Fatalf("AddTarget upsert: %v", err)
	}
	if second.ID != added.ID {
		t.Errorf("same URL should upsert to same ID: %d != %d", second.ID, added.ID)
	}

	targets, err := s.ListTargets()
	if err != nil {
		t.Fatalf("ListTargets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("len(ListTargets()) = %d, want 1", len(targets))
	}
	got := targets[0]
	if got.Name != "web-1-rename" || got.Interval != 30*time.Second || !got.Enabled {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if !got.CreatedAt.Equal(now) && got.CreatedAt.IsZero() {
		t.Errorf("CreatedAt not stored: %+v", got)
	}
}

func TestTargetValidationRejectsBadInput(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.AddTarget(Target{Name: "x", URL: "not-a-url", Interval: time.Second}); err == nil {
		t.Error("AddTarget must reject a URL without a scheme/host")
	}
	if _, err := s.AddTarget(Target{Name: "x", URL: "http://127.0.0.1:9100/metrics", Interval: 0}); err == nil {
		t.Error("AddTarget must reject a zero interval")
	}
}

func TestDeleteTargetRemovesRow(t *testing.T) {
	s := openTestStore(t)
	tg, err := s.AddTarget(Target{Name: "gone", URL: "http://127.0.0.1:9999/metrics", Interval: time.Minute, Enabled: true})
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	if err := s.DeleteTarget(tg.ID); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	targets, _ := s.ListTargets()
	if len(targets) != 0 {
		t.Errorf("target still present after delete: %+v", targets)
	}
}
