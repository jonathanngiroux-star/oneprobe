package scrape

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestFetchParsesAndReturnsSamples(t *testing.T) {
	body := `# HELP up Up
# TYPE up gauge
up 1
http_requests_total{code="200"} 42
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	f := NewFetcher(time.Second)
	samples, durMs, err := f.Fetch(srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("got %d samples, want 2", len(samples))
	}
	if samples[0].Name != "up" || samples[0].Value != 1 {
		t.Errorf("sample 0 = %+v", samples[0])
	}
	if samples[1].Labels["code"] != "200" || samples[1].Value != 42 {
		t.Errorf("sample 1 = %+v", samples[1])
	}
	if durMs < 0 {
		t.Errorf("duration %d < 0", durMs)
	}
}

func TestFetchIgnoresContentTypeHeader(t *testing.T) {
	// Oneprobe parses the body regardless of Content-Type — exporters
	// are inconsistent in the wild and the parser is the contract.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		fmt.Fprint(w, "up 1\n")
	}))
	defer srv.Close()
	f := NewFetcher(time.Second)
	samples, _, err := f.Fetch(srv.URL)
	if err != nil || len(samples) != 1 {
		t.Errorf("Fetch: samples=%v err=%v", samples, err)
	}
}

func TestFetchNon200Fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()
	f := NewFetcher(time.Second)
	_, _, err := f.Fetch(srv.URL)
	if err == nil {
		t.Fatal("non-200 must fail")
	}
	if !contains(err.Error(), "500") {
		t.Errorf("error must name the status, got: %v", err)
	}
}

func TestFetchMalformedBodyFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "garbage_line_without_value\n")
	}))
	defer srv.Close()
	f := NewFetcher(time.Second)
	_, _, err := f.Fetch(srv.URL)
	if err == nil {
		t.Fatal("malformed exposition must fail the scrape")
	}
	if !contains(err.Error(), "line 1") {
		t.Errorf("error must name the line, got: %v", err)
	}
}

func TestFetchTimeoutFailsFast(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		fmt.Fprint(w, "up 1\n")
	}))
	defer srv.Close()
	f := NewFetcher(50 * time.Millisecond)
	start := time.Now()
	_, _, err := f.Fetch(srv.URL)
	if err == nil {
		t.Fatal("timeout must fail")
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Errorf("fetch took %s, want fast timeout fail", elapsed)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestLoopScrapesOnInterval pins the loop contract: enabled targets are
// scraped when their interval is due, disabled targets never.
func TestLoopScrapesOnInterval(t *testing.T) {
	var mu sync.Mutex
	scrapes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		scrapes++
		mu.Unlock()
		fmt.Fprint(w, "up 1\n")
	}))
	defer srv.Close()

	st := newFakeStore()
	loop := NewLoop(st, NewFetcher(time.Second))
	st.targets = []Target{{
		ID: 1, Name: "web-1", URL: srv.URL,
		Interval: Millis(30 * time.Second), Enabled: true,
	}, {
		ID: 2, Name: "off", URL: "http://127.0.0.1:1/metrics",
		Interval: Millis(30 * time.Second), Enabled: false,
	}}

	// Tick the loop at t0, t0+30, t0+60: target 1 is due each time.
	for i := 0; i < 3; i++ {
		at := time.Unix(1759305600+int64(i*30), 0).UTC()
		loop.Tick(at)
	}
	mu.Lock()
	defer mu.Unlock()
	if scrapes != 3 {
		t.Errorf("target scraped %d times, want 3 (once per interval tick)", scrapes)
	}
	for _, rec := range st.scrapeRecorded {
		if rec.targetID == 2 {
			t.Error("disabled target was scraped")
		}
	}
}

// TestLoopSkipsTargetNotYetDue pins due-ness: a target whose interval
// has not elapsed is not scraped.
func TestLoopSkipsTargetNotYetDue(t *testing.T) {
	var mu sync.Mutex
	scrapes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		scrapes++
		mu.Unlock()
		fmt.Fprint(w, "up 1\n")
	}))
	defer srv.Close()

	st := newFakeStore()
	loop := NewLoop(st, NewFetcher(time.Second))
	st.targets = []Target{{
		ID: 1, Name: "web-1", URL: srv.URL,
		Interval: Millis(60 * time.Second), Enabled: true,
	}}

	// First tick scrapes; ticks at +10s and +20s must not.
	loop.Tick(time.Unix(1759305600, 0).UTC())
	loop.Tick(time.Unix(1759305610, 0).UTC())
	loop.Tick(time.Unix(1759305620, 0).UTC())
	mu.Lock()
	defer mu.Unlock()
	if scrapes != 1 {
		t.Errorf("target scraped %d times, want 1 (not due until +60s)", scrapes)
	}
}

// TestLoopRecordsFailure pins the failure path: a dead target records a
// failed scrape (with error text), not a silent skip.
func TestLoopRecordsFailure(t *testing.T) {
	st := newFakeStore()
	loop := NewLoop(st, NewFetcher(100*time.Millisecond))
	st.targets = []Target{{
		ID: 7, Name: "dead", URL: "http://127.0.0.1:1/metrics",
		Interval: Millis(15 * time.Second), Enabled: true,
	}}
	loop.Tick(time.Unix(1759305600, 0).UTC())
	if len(st.scrapeRecorded) != 1 {
		t.Fatalf("recorded %d scrapes, want 1", len(st.scrapeRecorded))
	}
	rec := st.scrapeRecorded[0]
	if rec.targetID != 7 {
		t.Errorf("recorded target %d, want 7", rec.targetID)
	}
	if rec.err == nil || *rec.err == "" {
		t.Error("failed scrape must record the error text")
	}
	if len(st.inserted) != 0 {
		t.Errorf("failed scrape inserted %d samples, want 0", len(st.inserted))
	}
}

// fakeStore implements LoopStore without SQLite.
type fakeStore struct {
	mu             sync.Mutex
	targets        []Target
	inserted       []Sample
	scrapeRecorded []scrapeRecord
}

type scrapeRecord struct {
	targetID    int64
	at          time.Time
	durationMs  int64
	sampleCount int
	err         *string
}

func (f *fakeStore) ListTargets() ([]Target, error) { return f.targets, nil }

func (f *fakeStore) InsertScrapeSamples(targetID int64, samples []Sample) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inserted = append(f.inserted, samples...)
	return nil
}

func (f *fakeStore) RecordScrape(targetID int64, at time.Time, durationMs int64, sampleCount int, scrapeErr *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scrapeRecorded = append(f.scrapeRecorded, scrapeRecord{targetID, at, durationMs, sampleCount, scrapeErr})
	return nil
}

func newFakeStore() *fakeStore { return &fakeStore{} }
