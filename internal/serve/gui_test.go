package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMigrateEndpointRunsDatadogDryRun pins the GUI's backend: POST a
// dashboard export JSON, get the mapped/partial/unmapped/risks report.
func TestMigrateEndpointRunsDatadogDryRun(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	h.SetAlertEvalInterval(time.Hour)
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	dashboard := `{
		"id": "gui-1", "title": "GUI Test",
		"widgets": [
			{"id": 1, "type": "timeseries", "title": "CPU",
			 "requests": [{"q": "avg:system.cpu.user{host:web-1}"}]},
			{"id": 2, "type": "check_status", "title": "Synth",
			 "requests": [{"q": "\"synthetics\" in (\"check_state\")"}]}
		]
	}`
	resp, err := http.Post(srv.URL+"/api/migrate/datadog", "application/json",
		strings.NewReader(dashboard))
	if err != nil {
		t.Fatalf("POST /api/migrate/datadog: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}
	var report struct {
		Source   string `json:"source"`
		DryRun   bool   `json:"dry_run"`
		Mapped   []any  `json:"mapped"`
		Partial  []any  `json:"partial"`
		Unmapped []any  `json:"unmapped"`
		Risks    []any  `json:"risks"`
	}
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	if report.Source != "datadog" || !report.DryRun {
		t.Errorf("source/dry_run = %q/%v", report.Source, report.DryRun)
	}
	if len(report.Mapped) != 1 || len(report.Unmapped) != 1 {
		t.Errorf("mapped=%d unmapped=%d, want 1/1: %s", len(report.Mapped), len(report.Unmapped), raw)
	}
}

// TestMigrateEndpointRejectsGarbage: invalid JSON is a 400 naming the
// problem, not a 500.
func TestMigrateEndpointRejectsGarbage(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/migrate/datadog", "application/json",
		strings.NewReader("{not json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400: %s", resp.StatusCode, raw)
	}
	if !containsStr(raw, "dashboard") {
		t.Errorf("error must mention the dashboard export, got: %s", raw)
	}
}

// TestMigrateEndpointRejectsWrongSource: --from= is datadog-only in
// v0.1; the refusal names it.
func TestMigrateEndpointRejectsWrongSource(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/migrate/grafana", "application/json",
		strings.NewReader(`{"widgets":[]}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404 (only datadog exists): %s", resp.StatusCode, raw)
	}
}

// TestDonatePageIsHTMLWithCopyButtons: /donate must serve the HTML
// donate page (the GUI), not the bare text — with both addresses,
// copy buttons, and no payment form.
func TestDonatePageIsHTMLWithCopyButtons(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/donate")
	if err != nil {
		t.Fatalf("GET /donate: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html (GUI donate page)", ct)
	}
	for _, want := range []string{
		"0x85ee7E71f762d772599cbF1EC20E651B30657521",
		"bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg",
		"copy", // copy buttons
	} {
		if !containsStr(raw, want) {
			t.Errorf("donate page missing %q", want)
		}
	}
	if containsStr(raw, "stripe") || containsStr(raw, "Stripe") {
		t.Error("donate page must contain no payment form")
	}
}

// TestAlertHistoryEndpoint: the alert engine's state transitions must
// be queryable as history for the GUI.
func TestAlertHistoryEndpoint(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/alerts/1/history")
	if err != nil {
		t.Fatalf("GET history: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}
	if !containsStr(raw, "[]") && !containsStr(raw, "events") {
		t.Errorf("history response shape wrong: %s", raw)
	}
}
