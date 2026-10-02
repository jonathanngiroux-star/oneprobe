package store

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
)

// TestOpenRefusesNewerSchema: a database stamped with a FUTURE schema
// version must refuse loudly (upgrade the binary first), not half-open.
func TestOpenRefusesNewerSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "future.db")
	raw := openRaw(t, dbPath)
	execScript(t, raw, `
CREATE TABLE meta (key TEXT PRIMARY KEY, value INTEGER NOT NULL);
INSERT INTO meta VALUES ('schema_version', 99);
`)
	raw.Close()
	_, err := Open(dbPath)
	if err == nil {
		t.Fatal("Open on a newer-schema file must fail")
	}
	if !containsStr(err.Error(), "newer") {
		t.Errorf("refusal must say the file is newer, got: %v", err)
	}
}

// TestDeleteTargetCascadesSamplesAndScrapes: removing a target removes
// its samples and scrape records (the FK cascade now applies to every
// pooled connection).
func TestDeleteTargetCascadesSamplesAndScrapes(t *testing.T) {
	s := openTestStore(t)
	tg, err := s.AddTarget(Target{
		Name: "gone", URL: "http://127.0.0.1:9999/metrics",
		Interval: scrapeMillis(time.Minute), Enabled: true,
	})
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	base := int64(1759305600000)
	if err := s.InsertSamples([]Sample{
		{TargetID: tg.ID, Name: "up", Ts: base, Value: 1},
	}); err != nil {
		t.Fatalf("InsertSamples: %v", err)
	}
	if err := s.RecordScrape(tg.ID, time.UnixMilli(base), 5, 1, nil); err != nil {
		t.Fatalf("RecordScrape: %v", err)
	}
	if err := s.DeleteTarget(tg.ID); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	for _, q := range []struct {
		name string
		sql  string
	}{
		{"samples", `SELECT COUNT(*) FROM samples WHERE target_id = ?`},
		{"scrapes", `SELECT COUNT(*) FROM scrapes WHERE target_id = ?`},
	} {
		var n int
		if err := s.db.QueryRow(q.sql, tg.ID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", q.name, err)
		}
		if n != 0 {
			t.Errorf("%s survived target delete: %d rows (FK cascade broken)", q.name, n)
		}
	}
}

// TestConcurrentWritersDoNotCorrupt: N goroutines writing sample
// batches + scrape records through one Store must all succeed under
// SQLITE's single-writer constraints (the writeMu serialization is
// the mechanism under test). Fails via SQLITE_BUSY_SNAPSHOT without it.
func TestConcurrentWritersDoNotCorrupt(t *testing.T) {
	s := openTestStore(t)
	const writers = 8
	const batches = 20
	tg, err := s.AddTarget(Target{
		Name: "busy", URL: "http://127.0.0.1:9998/metrics",
		Interval: scrapeMillis(time.Minute), Enabled: true,
	})
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, writers*batches)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for b := 0; b < batches; b++ {
				ts := int64(1759305600000 + (w*10000+b)*1000)
				batch := []Sample{
					{TargetID: tg.ID, Name: "w", Labels: `{w="` + itoa(w) + `"}`, Ts: ts, Value: float64(b)},
				}
				if err := s.InsertSamples(batch); err != nil {
					errs <- err
					return
				}
				if err := s.RecordScrape(tg.ID, time.UnixMilli(ts), 1, 1, nil); err != nil {
					errs <- err
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent write failed: %v", err)
	}
	// Count landed rows: writers × batches.
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM samples WHERE name='w'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != writers*batches {
		t.Errorf("landed %d sample rows, want %d (lost writes)", n, writers*batches)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func scrapeMillis(d time.Duration) scrape.Millis {
	return scrape.Millis(d)
}
