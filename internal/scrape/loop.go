package scrape

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// Target is one Prometheus-format scrape destination. It lives in
// scrape (not store) because store imports scrape for label
// canonicalization; store.Target is a structurally identical alias.
// JSON interval is milliseconds — a bare time.Duration would serialize
// nanoseconds and confuse every API consumer.
type Target struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Interval  Millis `json:"interval"`
	Enabled   bool   `json:"enabled"`
	CreatedAt Time   `json:"created_at"`
}

// Millis is a time.Duration that (un)marshals as integer milliseconds.
type Millis time.Duration

func (m Millis) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(time.Duration(m).Milliseconds(), 10)), nil
}

func (m *Millis) UnmarshalJSON(b []byte) error {
	var ms int64
	if err := json.Unmarshal(b, &ms); err != nil {
		return fmt.Errorf("interval must be integer milliseconds: %w", err)
	}
	*m = Millis(time.Duration(ms) * time.Millisecond)
	return nil
}

// Duration returns the interval as a time.Duration.
func (m Millis) Duration() time.Duration { return time.Duration(m) }

// Time is a time.Time that marshals as RFC 3339 UTC.
type Time time.Time

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).UTC().Format(time.RFC3339))
}

func (t *Time) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("timestamp must be an RFC 3339 string: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("timestamp must be RFC 3339: %w", err)
	}
	*t = Time(parsed)
	return nil
}

// LoopStore is the store contract the scrape loop needs: list targets,
// insert samples, record scrape outcomes.
type LoopStore interface {
	ListTargets() ([]Target, error)
	InsertScrapeSamples(targetID int64, samples []Sample) error
	RecordScrape(targetID int64, at time.Time, durationMs int64, sampleCount int, scrapeErr *string) error
}

// Loop scrapes enabled targets on their intervals. Tick is explicit so
// tests drive time; Run wires Tick to a real ticker.
type Loop struct {
	store   LoopStore
	fetcher *Fetcher

	mu      sync.Mutex
	lastDue map[int64]time.Time // target ID → last scrape completion
}

// NewLoop builds a loop over the store.
func NewLoop(store LoopStore, fetcher *Fetcher) *Loop {
	return &Loop{store: store, fetcher: fetcher, lastDue: map[int64]time.Time{}}
}

// Tick scrapes every enabled target whose interval has elapsed since
// its last successful-or-failed scrape. Targets never scraped are due
// immediately.
func (l *Loop) Tick(now time.Time) {
	targets, err := l.store.ListTargets()
	if err != nil {
		return // store errors: nothing sensible to do; next Tick retries
	}
	var wg sync.WaitGroup
	for _, tg := range targets {
		if !tg.Enabled {
			continue
		}
		l.mu.Lock()
		last, seen := l.lastDue[tg.ID]
		if !seen || now.Sub(last) >= tg.Interval.Duration() {
			l.lastDue[tg.ID] = now
			wg.Add(1)
			go func(tg Target) {
				defer wg.Done()
				l.scrapeOne(tg, now)
			}(tg)
		}
		l.mu.Unlock()
	}
	wg.Wait()
}

// scrapeOne fetches one target and records the outcome either way.
func (l *Loop) scrapeOne(tg Target, now time.Time) {
	samples, durMs, err := l.fetcher.Fetch(tg.URL)
	if err != nil {
		errText := err.Error()
		_ = l.store.RecordScrape(tg.ID, now, durMs, 0, &errText)
		return
	}
	if serr := l.store.InsertScrapeSamples(tg.ID, samples); serr != nil {
		// Store failure is a scrape failure for bookkeeping purposes.
		errText := serr.Error()
		_ = l.store.RecordScrape(tg.ID, now, durMs, 0, &errText)
		return
	}
	_ = l.store.RecordScrape(tg.ID, now, durMs, len(samples), nil)
}

// Run blocks, ticking at the given cadence until stop is closed. Each
// Tick re-reads targets, so target changes take effect without a
// restart.
func (l *Loop) Run(every time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		l.Tick(time.Now().UTC())
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
	}
}
