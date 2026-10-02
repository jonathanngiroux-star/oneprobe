package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
)

// TestEmptyListsMarshalAsArrays: every list-returning store method must
// serialize an EMPTY result as [] — a nil slice marshals as JSON null,
// which crashes client-side `.length`/`len()` handling. Found live in
// the audit: GET /api/targets/1/scrapes returned the four bytes `null`.
func TestEmptyListsMarshalAsArrays(t *testing.T) {
	s := openTestStore(t)
	tg, err := s.AddTarget(Target{
		Name: "empty", URL: "http://127.0.0.1:9997/metrics",
		Interval: scrape.Millis(time.Minute), Enabled: true,
	})
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}

	// List methods.
	targets, err := s.ListTargets()
	if err != nil {
		t.Fatalf("ListTargets: %v", err)
	}
	if targets == nil {
		t.Error("ListTargets: empty result is nil (marshals as null, want [])")
	}
	scrapes, err := s.ListScrapes(tg.ID, 10)
	if err != nil {
		t.Fatalf("ListScrapes: %v", err)
	}
	if scrapes == nil {
		t.Error("ListScrapes: empty result is nil (marshals as null, want [])")
	}
	rules, err := s.ListAlertRules()
	if err != nil {
		t.Fatalf("ListAlertRules: %v", err)
	}
	if rules == nil {
		t.Error("ListAlertRules: empty result is nil (marshals as null, want [])")
	}

	// Query methods.
	points, err := s.QueryRange("nothing", 0, 1)
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if points == nil {
		t.Error("QueryRange: empty result is nil (marshals as null, want [])")
	}
	series, err := s.QuerySeries("nothing", 0, 1)
	if err != nil {
		t.Fatalf("QuerySeries: %v", err)
	}
	if series == nil {
		t.Error("QuerySeries: empty result is nil (marshals as null, want [])")
	}
	names, err := s.SeriesNames()
	if err != nil {
		t.Fatalf("SeriesNames: %v", err)
	}
	if names == nil {
		t.Error("SeriesNames: empty result is nil (marshals as null, want [])")
	}

	// End-to-end: the actual bytes must be [], never null.
	b, err := json.Marshal(scrapes)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != "[]" {
		t.Errorf("empty scrapes marshal = %s, want []", b)
	}
}
