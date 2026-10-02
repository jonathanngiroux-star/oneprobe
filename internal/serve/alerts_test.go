package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// TestAlertRuleCRUDThroughAPI pins the /api/alerts surface: create,
// list, delete.
func TestAlertRuleCRUDThroughAPI(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	h.SetAlertEvalInterval(time.Hour) // engine off for CRUD tests

	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	// Create.
	body := `{"name":"host-down","expr":"up","op":"<","value":1,"for_secs":0,"enabled":true}`
	resp, err := http.Post(srv.URL+"/api/alerts", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/alerts: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("POST status %d: %s", resp.StatusCode, raw)
	}
	var created store.AlertRule
	if err := json.Unmarshal([]byte(raw), &created); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	if created.ID == 0 || created.Name != "host-down" {
		t.Fatalf("created = %+v", created)
	}

	// List.
	resp, err = http.Get(srv.URL + "/api/alerts")
	if err != nil {
		t.Fatalf("GET /api/alerts: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	var rules []store.AlertRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rules) != 1 || rules[0].Name != "host-down" || rules[0].Expr != "up" {
		t.Fatalf("list = %s", raw)
	}

	// Delete.
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/alerts/1", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("DELETE status = %d, want 204", resp.StatusCode)
	}

	// Gone.
	resp, err = http.Get(srv.URL + "/api/alerts")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	var after []store.AlertRule
	json.Unmarshal([]byte(raw), &after)
	if len(after) != 0 {
		t.Fatalf("rule survived delete: %s", raw)
	}
}

// TestAlertValidationNamesRejectedField.
func TestAlertValidationNamesRejectedField(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	h.SetAlertEvalInterval(time.Hour)
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	body := `{"name":"bad-op","expr":"up","op":"~","value":1}`
	resp, err := http.Post(srv.URL+"/api/alerts", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400: %s", resp.StatusCode, raw)
	}
	if !containsStr(raw, `op "~"`) {
		t.Errorf("refusal must name the op, got: %s", raw)
	}
}

// TestAlertEngineFiresEndToEnd pins the full local loop: a seeded
// violating sample + a rule → the engine (evaluating every tick) fires
// it → /api/alerts shows firing state.
func TestAlertEngineFiresEndToEnd(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	h.SetAlertEvalInterval(50 * time.Millisecond)

	if _, err := h.store.AddTarget(store.Target{
		Name: "seed", URL: "http://127.0.0.1:9999/metrics",
		Interval: scrape.Millis(15 * time.Second), Enabled: false,
	}); err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	now := time.Now().UTC().UnixMilli()
	if err := h.store.InsertSamples([]store.Sample{
		{TargetID: 1, Name: "up", Ts: now, Value: 0},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	// Create the rule through the API.
	body := `{"name":"host-down","expr":"up","op":"<","value":1,"enabled":true}`
	resp, err := http.Post(srv.URL+"/api/alerts", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/alerts: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("create status %d: %s", resp.StatusCode, raw)
	}

	// The engine evaluates every 50ms; wait for firing.
	deadline := time.Now().Add(3 * time.Second)
	var state string
	for time.Now().Before(deadline) {
		resp, err := http.Get(srv.URL + "/api/alerts")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		raw = readAll(t, resp)
		resp.Body.Close()
		var list []map[string]any
		json.Unmarshal([]byte(raw), &list)
		if len(list) == 1 {
			if st, ok := list[0]["state"].(string); ok {
				state = st
				if state == "firing" {
					break
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if state != "firing" {
		t.Fatalf("alert state = %q after 3s, want firing (up=0 < 1)", state)
	}
}
