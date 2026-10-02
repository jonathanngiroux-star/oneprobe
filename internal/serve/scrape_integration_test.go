package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestServeScrapesRegisteredTarget pins the serve+wiring contract: a
// target added through the API is scraped by the embedded loop, its
// samples are queryable through the API, and recent scrapes are visible.
func TestServeScrapesRegisteredTarget(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	metrics := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		fmt.Fprint(w, "up 1\nfake_metric_total 7\n")
	}))
	defer metrics.Close()

	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	h.SetScrapeTickInterval(20 * time.Millisecond)

	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	// Register the target through the API.
	body := fmt.Sprintf(`{"name":"fake","url":%q,"interval":%d}`, metrics.URL, 100)
	resp, err := http.Post(srv.URL+"/api/targets", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/targets: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("POST /api/targets status %d, want 201: %s", resp.StatusCode, raw)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(raw), &created); err != nil || created.ID == 0 {
		t.Fatalf("created target body = %q (err %v)", raw, err)
	}

	// Wait for the loop to scrape at least twice.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := hits
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	final := hits
	mu.Unlock()
	if final < 2 {
		t.Fatalf("target scraped %d times in 3s, want >=2", final)
	}

	// Samples are queryable.
	q := srv.URL + "/api/query?metric=fake_metric_total"
	resp, err = http.Get(q)
	if err != nil {
		t.Fatalf("GET /api/query: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	var series struct {
		Metric string `json:"metric"`
		Points []struct {
			Ts    int64   `json:"ts"`
			Value float64 `json:"value"`
		} `json:"points"`
	}
	if err := json.Unmarshal([]byte(raw), &series); err != nil {
		t.Fatalf("query response not JSON: %v (%s)", err, raw)
	}
	if series.Metric != "fake_metric_total" || len(series.Points) == 0 {
		t.Errorf("query = %s, want fake_metric_total with points", raw)
	}

	// Recent scrapes visible for the target.
	resp, err = http.Get(srv.URL + "/api/targets/1/scrapes")
	if err != nil {
		t.Fatalf("GET scrapes: %v", err)
	}
	raw = readAll(t, resp)
	resp.Body.Close()
	var scrapes []struct {
		Ok          bool `json:"ok"`
		SampleCount int  `json:"sample_count"`
	}
	if err := json.Unmarshal([]byte(raw), &scrapes); err != nil || len(scrapes) == 0 {
		t.Fatalf("scrapes = %s (err %v), want non-empty", raw, err)
	}
	if !scrapes[0].Ok || scrapes[0].SampleCount != 2 {
		t.Errorf("latest scrape = %+v, want ok with 2 samples", scrapes[0])
	}
}
