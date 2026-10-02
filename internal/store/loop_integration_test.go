package store

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
)

// Compile-time seam check: the real store satisfies the loop contract.
var _ scrape.LoopStore = (*Store)(nil)

// TestStoreSatisfiesLoopEndToEnd scrapes a real httptest target through
// the real loop into real SQLite and reads the data back — no mocks.
func TestStoreSatisfiesLoopEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `# HELP up Up
# TYPE up gauge
up 1
http_requests_total{code="200",method="get"} 42
http_requests_total{code="500"} 1
`)
	}))
	defer srv.Close()

	s := openTestStore(t)
	if _, err := s.AddTarget(Target{
		Name: "web-1", URL: srv.URL, Interval: scrape.Millis(30 * time.Second), Enabled: true,
	}); err != nil {
		t.Fatalf("AddTarget: %v", err)
	}

	loop := scrape.NewLoop(s, scrape.NewFetcher(2*time.Second))
	now := time.Unix(1759305600, 0).UTC()
	loop.Tick(now)
	// Second tick inside the interval: must not re-scrape.
	loop.Tick(now.Add(10 * time.Second))

	// Samples landed with canonical labels. No exposition timestamps →
	// rows are stamped with real Now by the adapter, so query a wide real
	// window around Now, not around the fake tick time.
	hi := time.Now().UTC().UnixMilli() + 1000
	points, err := s.QueryRange("http_requests_total", hi-60*60*1000, hi)
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("got %d points, want 2", len(points))
	}
	saw500, saw200 := false, false
	for _, p := range points {
		if p.Labels == `{code="500"}` {
			saw500 = true
		}
		if p.Labels == `{code="200",method="get"}` {
			saw200 = true
		}
	}
	if !saw500 || !saw200 {
		t.Errorf("labels not canonical: %+v", points)
	}
	bare, err := s.QueryRange("up", hi-60*60*1000, hi)
	if err != nil || len(bare) != 1 || bare[0].Value != 1 {
		t.Errorf("up sample = %+v err=%v", bare, err)
	}

	// Scrape meta: exactly one success recorded (second tick skipped).
	scrapes, err := s.ListScrapes(1, 5)
	if err != nil {
		t.Fatalf("ListScrapes: %v", err)
	}
	if len(scrapes) != 1 || !scrapes[0].Ok || scrapes[0].SampleCount != 3 {
		t.Errorf("scrapes = %+v, want one ok scrape with 3 samples", scrapes)
	}
}
