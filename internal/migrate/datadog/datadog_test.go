package datadog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T) Dashboard {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "datadog", "sample-dashboard.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var d Dashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return d
}

func TestFixtureHasExpectedShape(t *testing.T) {
	d := loadFixture(t)
	if d.ID != "abc-123" || d.Title != "Web Overview" {
		t.Errorf("fixture identity changed: %+v", d)
	}
	if len(d.Widgets) != 4 {
		t.Fatalf("fixture has %d widgets, want 4 (fixture drift breaks the CI gate)", len(d.Widgets))
	}
}

func TestDiffClassifiesEveryWidgetIntoMappedPartialUnmapped(t *testing.T) {
	d := loadFixture(t)
	report := Diff(d)

	if report.Source != "datadog" {
		t.Errorf("Source = %q, want \"datadog\"", report.Source)
	}
	if !report.DryRun {
		t.Error("Diff must always produce a dry-run report")
	}
	if report.Input.ID != d.ID || report.Input.Title != d.Title {
		t.Errorf("Input does not echo the source dashboard: %+v", report.Input)
	}

	// Every widget lands in exactly one bucket.
	if len(report.Mapped)+len(report.Partial)+len(report.Unmapped) != len(d.Widgets) {
		t.Errorf("buckets do not partition the widgets: mapped=%d partial=%d unmapped=%d widgets=%d",
			len(report.Mapped), len(report.Partial), len(report.Unmapped), len(d.Widgets))
	}

	// Widget 1: plain timeseries with a simple metric query → mapped.
	if len(report.Mapped) != 1 {
		t.Errorf("mapped = %d, want 1", len(report.Mapped))
	} else if report.Mapped[0].WidgetID != 1 || report.Mapped[0].Title != "Request rate" {
		t.Errorf("mapped entry = %+v, want widget 1 \"Request rate\"", report.Mapped[0])
	}

	// Widgets 2 (anomalies()) and 3 (toplist) → partial, with reasons.
	if len(report.Partial) != 2 {
		t.Errorf("partial = %d, want 2", len(report.Partial))
	}
	for _, p := range report.Partial {
		if p.Reason == "" {
			t.Errorf("partial entry (widget %d) must carry a reason", p.WidgetID)
		}
	}

	// Widget 4: synthetics check, no translatable query → unmapped.
	if len(report.Unmapped) != 1 {
		t.Errorf("unmapped = %d, want 1", len(report.Unmapped))
	} else if report.Unmapped[0].WidgetID != 4 {
		t.Errorf("unmapped entry = %+v, want widget 4", report.Unmapped[0])
	}
}

func TestDiffRisksAreNonEmptyAndHonest(t *testing.T) {
	report := Diff(loadFixture(t))
	if len(report.Risks) == 0 {
		t.Fatal("a dashboard with anomalies() and synthetics must produce at least one risk")
	}
	sawAnomalies, sawSynthetics := false, false
	for _, r := range report.Risks {
		if r.Key == "" || r.Note == "" {
			t.Errorf("risk must carry key and note: %+v", r)
		}
		if r.Key == "query.anomalies" {
			sawAnomalies = true
		}
		if r.Key == "widget.synthetics" {
			sawSynthetics = true
		}
	}
	if !sawAnomalies {
		t.Error("risks must call out anomalies() as an unsupported Datadog function")
	}
	if !sawSynthetics {
		t.Error("risks must call out synthetics widgets as out of scope")
	}
}

func TestDiffJSONIsDeterministic(t *testing.T) {
	first, err := json.Marshal(Diff(loadFixture(t)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	second, err := json.Marshal(Diff(loadFixture(t)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(first) != string(second) {
		t.Error("two Diffs on the same input must serialize identically (CI golden gate)")
	}
}
