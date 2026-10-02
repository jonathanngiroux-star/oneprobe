package store

import (
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
)

func addWebTarget(t *testing.T, s *Store) Target {
	t.Helper()
	tg, err := s.AddTarget(Target{
		Name: "web-1", URL: "http://127.0.0.1:9100/metrics",
		Interval: scrape.Millis(15 * time.Second), Enabled: true,
	})
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	return tg
}

func TestInsertSamplesAndQueryRangeByName(t *testing.T) {
	s := openTestStore(t)
	tg := addWebTarget(t, s)

	base := int64(1759305600000) // ms
	samples := []Sample{
		{TargetID: tg.ID, Name: "http_requests_total", Labels: `{code="200"}`, Ts: base, Value: 100},
		{TargetID: tg.ID, Name: "http_requests_total", Labels: `{code="200"}`, Ts: base + 15000, Value: 105},
		{TargetID: tg.ID, Name: "http_requests_total", Labels: `{code="500"}`, Ts: base, Value: 1},
		{TargetID: tg.ID, Name: "cpu_idle", Labels: "", Ts: base, Value: 0.99},
	}
	if err := s.InsertSamples(samples); err != nil {
		t.Fatalf("InsertSamples: %v", err)
	}

	// QueryRange by bare name returns every series under that name.
	got, err := s.QueryRange("http_requests_total", base, base+30000)
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d points, want 3 (two series)", len(got))
	}
	// Ordering: same-ts points sort by labels; the later ts last.
	if got[0].Name != "http_requests_total" || got[0].Labels != `{code="200"}` {
		t.Errorf("point 0 = %+v", got[0])
	}
	if got[1].Labels != `{code="500"}` || got[1].Ts != base {
		t.Errorf("point 1 = %+v, want code=500 at base ts", got[1])
	}
	if got[2].Labels != `{code="200"}` || got[2].Ts != base+15000 {
		t.Errorf("point 2 = %+v, want code=200 at base+15s", got[2])
	}

	// Range boundaries are inclusive.
	got, err = s.QueryRange("http_requests_total", base+15000, base+15000)
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("inclusive range: got %d points, want 1", len(got))
	}
}

func TestInsertSamplesUpsertsOnConflict(t *testing.T) {
	s := openTestStore(t)
	tg := addWebTarget(t, s)
	base := int64(1759305600000)
	first := []Sample{{TargetID: tg.ID, Name: "up", Labels: "", Ts: base, Value: 1}}
	if err := s.InsertSamples(first); err != nil {
		t.Fatalf("InsertSamples: %v", err)
	}
	// Same (target, name, labels, ts) — must replace, not duplicate or fail.
	first[0].Value = 0
	if err := s.InsertSamples(first); err != nil {
		t.Fatalf("InsertSamples re-insert: %v", err)
	}
	got, _ := s.QueryRange("up", base, base)
	if len(got) != 1 || got[0].Value != 0 {
		t.Errorf("upsert failed: %+v", got)
	}
}

func TestInsertSamplesRejectsUnknownTarget(t *testing.T) {
	s := openTestStore(t)
	err := s.InsertSamples([]Sample{{TargetID: 9999, Name: "up", Ts: 1, Value: 1}})
	if err == nil {
		t.Fatal("must reject samples for a target that does not exist")
	}
}

func TestInsertSamplesRejectsBatchWithEmptyName(t *testing.T) {
	s := openTestStore(t)
	tg := addWebTarget(t, s)
	err := s.InsertSamples([]Sample{
		{TargetID: tg.ID, Name: "up", Ts: 1, Value: 1},
		{TargetID: tg.ID, Name: "", Ts: 1, Value: 1},
	})
	if err == nil {
		t.Fatal("must reject an empty metric name")
	}
}

func TestRecordScrapeMetaAndListScrapes(t *testing.T) {
	s := openTestStore(t)
	tg := addWebTarget(t, s)
	at := time.Unix(1759305605, 0).UTC()
	if err := s.RecordScrape(tg.ID, at, 150, 12, nil); err != nil {
		t.Fatalf("RecordScrape: %v", err)
	}
	failing := "connection refused"
	if err := s.RecordScrape(tg.ID, at.Add(15*time.Second), 0, 0, &failing); err != nil {
		t.Fatalf("RecordScrape error case: %v", err)
	}
	scrapes, err := s.ListScrapes(tg.ID, 2)
	if err != nil {
		t.Fatalf("ListScrapes: %v", err)
	}
	if len(scrapes) != 2 {
		t.Fatalf("got %d scrapes, want 2", len(scrapes))
	}
	// Newest first.
	if scrapes[0].Error != "connection refused" || scrapes[0].Ok {
		t.Errorf("newest scrape = %+v, want the failing one", scrapes[0])
	}
	if !scrapes[1].Ok || scrapes[1].DurationMs != 150 || scrapes[1].SampleCount != 12 {
		t.Errorf("older scrape = %+v", scrapes[1])
	}
	if !scrapes[1].At.Equal(at) {
		t.Errorf("scrape time = %v, want %v", scrapes[1].At, at)
	}
}

func TestSeriesNamesAreBareAndSorted(t *testing.T) {
	s := openTestStore(t)
	tg := addWebTarget(t, s)
	base := int64(1759305600000)
	batch := []Sample{
		{TargetID: tg.ID, Name: "up", Ts: base, Value: 1},
		{TargetID: tg.ID, Name: "http_requests_total", Labels: `{code="200"}`, Ts: base, Value: 5},
		{TargetID: tg.ID, Name: "http_requests_total", Labels: `{code="500"}`, Ts: base + 15000, Value: 6},
	}
	if err := s.InsertSamples(batch); err != nil {
		t.Fatalf("InsertSamples: %v", err)
	}
	names, err := s.SeriesNames()
	if err != nil {
		t.Fatalf("SeriesNames: %v", err)
	}
	// Distinct bare names, not identities.
	if len(names) != 2 || names[0] != "http_requests_total" || names[1] != "up" {
		t.Errorf("SeriesNames = %v, want [http_requests_total up]", names)
	}
}

func TestDeleteSamplesBefore(t *testing.T) {
	s := openTestStore(t)
	tg := addWebTarget(t, s)
	base := int64(1759305600000)
	batch := []Sample{
		{TargetID: tg.ID, Name: "up", Ts: base, Value: 1},
		{TargetID: tg.ID, Name: "up", Ts: base + 86400000, Value: 1}, // +1 day
	}
	if err := s.InsertSamples(batch); err != nil {
		t.Fatalf("InsertSamples: %v", err)
	}
	deleted, err := s.DeleteSamplesBefore(base + 43200000) // +12h cutoff
	if err != nil {
		t.Fatalf("DeleteSamplesBefore: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted %d rows, want 1", deleted)
	}
	got, _ := s.QueryRange("up", 0, base+86400000)
	if len(got) != 1 || got[0].Ts != base+86400000 {
		t.Errorf("retention delete wrong rows: %+v", got)
	}
}

// TestMigrateV1ToV2 proves a v1 database (fused metric identity column)
// is migrated in place: identities split into name + labels, no data lost.
func TestMigrateV1ToV2(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/oneprobe.db"

	// Hand-build a v1 database: old schema, old-style fused identity rows.
	v1 := openRaw(t, dbPath)
	execScript(t, v1, `
CREATE TABLE meta (key TEXT PRIMARY KEY, value INTEGER NOT NULL);
CREATE TABLE targets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL, url TEXT NOT NULL UNIQUE,
	interval_ms INTEGER NOT NULL, enabled INTEGER NOT NULL DEFAULT 1, created_at INTEGER NOT NULL
);
CREATE TABLE samples (
	target_id INTEGER NOT NULL, metric TEXT NOT NULL, ts INTEGER NOT NULL, value REAL NOT NULL,
	PRIMARY KEY (target_id, metric, ts)
) WITHOUT ROWID;
INSERT INTO meta VALUES ('schema_version', 1);
INSERT INTO targets (name, url, interval_ms, enabled, created_at)
	VALUES ('web-1', 'http://127.0.0.1:9100/metrics', 15000, 1, 1759305600);
INSERT INTO samples VALUES
	(1, 'http_requests_total', 1759305600000, 100),
	(1, 'http_requests_total{code="200",method="get"}', 1759305600000, 90),
	(1, 'cpu_idle', 1759305600000, 0.99);
`)
	v1.Close()

	// Reopen with current code: must migrate to v2 transparently.
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open v1 file: %v", err)
	}
	defer s.Close()

	if s.SchemaIsCurrent() != true {
		t.Fatalf("schema not current after Open")
	}
	got, err := s.QueryRange("http_requests_total", 1759305600000, 1759305600000)
	if err != nil {
		t.Fatalf("QueryRange after migration: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d points, want 2 (bare + labeled series)", len(got))
	}
	if got[0].Name != "http_requests_total" || got[0].Labels != "" {
		t.Errorf("bare series not split correctly: %+v", got[0])
	}
	if got[1].Labels != `{code="200",method="get"}` {
		t.Errorf("labels not split correctly: %+v", got[1])
	}
	names, _ := s.SeriesNames()
	if len(names) != 2 { // http_requests_total + cpu_idle
		t.Errorf("SeriesNames after migration = %v", names)
	}
}
