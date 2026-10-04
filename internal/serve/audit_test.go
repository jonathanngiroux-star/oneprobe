package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// TestServeNegativePaths walks every API refusal: each must return the
// right status AND name the rejected input.
// TestPostTargetReportsCreateVsUpdate: POSTing a URL that already
// exists is an UPDATE — the API must say 200 (not 201) and the body
// must carry created:false so the GUI can report the rename honestly.
func TestPostTargetReportsCreateVsUpdate(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t))
	defer srv.Close()

	post := func(name, url string) (int, map[string]any) {
		resp, err := http.Post(srv.URL+"/api/targets", "application/json",
			strings.NewReader(fmt.Sprintf(`{"name":%q,"url":%q,"interval":1000}`, name, url)))
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body
	}

	code, body := post("web-1", "http://127.0.0.1:9100/metrics")
	if code != http.StatusCreated {
		t.Errorf("first POST -> %d, want 201", code)
	}
	if body["created"] != true {
		t.Errorf("first POST body created=%v, want true", body["created"])
	}

	code, body = post("web-1-rename", "http://127.0.0.1:9100/metrics")
	if code != http.StatusOK {
		t.Errorf("upsert POST -> %d, want 200", code)
	}
	if body["created"] != false {
		t.Errorf("upsert POST body created=%v, want false", body["created"])
	}
}

func TestServeNegativePaths(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t))
	defer srv.Close()

	type probe struct {
		name   string
		do     func() (*http.Response, string)
		want   int
		wantIn string
	}
	post := func(path, body string) probe {
		return probe{path, func() (*http.Response, string) {
			resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
			if err != nil {
				t.Fatalf("POST %s: %v", path, err)
			}
			return resp, readAll(t, resp)
		}, 0, ""}
	}
	_ = post

	// /api/targets POST: invalid JSON body.
	resp, err := http.Post(srv.URL+"/api/targets", "application/json", strings.NewReader("{not json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 400 || !containsStr(raw, "invalid JSON") {
		t.Errorf("bad JSON body: status %d body %q", resp.StatusCode, raw)
	}

	// /api/targets POST: invalid URL named in the refusal.
	resp, err = http.Post(srv.URL+"/api/targets", "application/json",
		strings.NewReader(`{"name":"x","url":"not-a-url","interval":1000}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 400 || !containsStr(raw, "not-a-url") {
		t.Errorf("bad URL: status %d body %q (must name the URL)", resp.StatusCode, raw)
	}

	// /api/targets POST: zero interval named.
	resp, err = http.Post(srv.URL+"/api/targets", "application/json",
		strings.NewReader(`{"name":"x","url":"http://127.0.0.1:9999/metrics","interval":0}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 400 || !containsStr(raw, "interval") {
		t.Errorf("zero interval: status %d body %q", resp.StatusCode, raw)
	}

	// /api/targets/9999/scrapes: unknown target (empty list, 200 — an
	// unknown id is not an error, it simply has no scrapes).
	resp, err = http.Get(srv.URL + "/api/targets/9999/scrapes")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("unknown target scrapes: status %d body %q", resp.StatusCode, raw)
	}

	// /api/targets/abc: non-numeric id named.
	resp, err = http.Get(srv.URL + "/api/targets/abc/scrapes")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 400 || !containsStr(raw, "abc") {
		t.Errorf("bad id: status %d body %q (must name it)", resp.StatusCode, raw)
	}

	// /api/query with both expr and metric.
	resp, err = http.Get(srv.URL + "/api/query?expr=up&metric=up")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 400 || !containsStr(raw, "not both") {
		t.Errorf("both params: status %d body %q", resp.StatusCode, raw)
	}

	// /api/query with neither.
	resp, err = http.Get(srv.URL + "/api/query")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 400 || !containsStr(raw, "metric parameter is required") {
		t.Errorf("no params: status %d body %q", resp.StatusCode, raw)
	}

	// /api/alerts POST: empty name named.
	resp, err = http.Post(srv.URL+"/api/alerts", "application/json",
		strings.NewReader(`{"name":"","expr":"up","op":"<","value":1}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 400 || !containsStr(raw, "name") {
		t.Errorf("empty name: status %d body %q", resp.StatusCode, raw)
	}

	// /api/alerts/42 DELETE: unknown id.
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/alerts/42", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 404 || !containsStr(raw, "42") {
		t.Errorf("unknown alert: status %d body %q", resp.StatusCode, raw)
	}

	// Method not allowed surfaces.
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/query"},
		{http.MethodPost, "/api/scrapes"},
		{http.MethodDelete, "/api/targets"},
		{http.MethodPost, "/api/alerts/1"},
		{http.MethodPost, "/healthz"},
		{http.MethodPost, "/donate"},
		{http.MethodPost, "/"},
	} {
		req, _ := http.NewRequest(tc.method, srv.URL+tc.path, strings.NewReader("{}"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		raw := readAll(t, resp)
		resp.Body.Close()
		if resp.StatusCode != 405 {
			t.Errorf("%s %s: status %d, want 405 (%s)", tc.method, tc.path, resp.StatusCode, raw)
		}
	}
}

// TestRecentScrapesNotScrapedBranch pins the /api/scrapes shape for a
// registered-but-never-scraped target.
func TestRecentScrapesNotScrapedBranch(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	h.SetAlertEvalInterval(time.Hour)

	if _, err := h.store.AddTarget(store.Target{
		Name: "fresh", URL: "http://127.0.0.1:19999/metrics",
		Interval: scrape.Millis(time.Minute), Enabled: false, // never scraped
	}); err != nil {
		t.Fatalf("AddTarget: %v", err)
	}

	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/scrapes")
	if err != nil {
		t.Fatalf("GET /api/scrapes: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	var rows []struct {
		TargetID int64 `json:"target_id"`
		Scrapes  []struct {
			Ok bool `json:"ok"`
		} `json:"scrapes"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if len(rows[0].Scrapes) != 0 {
		t.Errorf("never-scraped target must have empty scrapes, got %s", raw)
	}
}
