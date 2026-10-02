// Package store is Oneprobe's embedded SQLite layer: targets, samples
// (bounded retention), alert rules, and alert state — all in the single
// binary, no external database.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
	_ "modernc.org/sqlite"
)

// SchemaVersion is the current SQLite schema version, recorded in meta.
// v1: fused metric identity in samples.metric.
// v2: samples split into name + labels columns; scrapes table added.
const SchemaVersion = 2

// Target is one Prometheus-format scrape destination. It is an alias
// for scrape.Target: the type lives in scrape because store imports
// scrape (label canonicalization), so scrape cannot import store.
type Target = scrape.Target

// Sample is one scraped time-series point for a target. Name is the bare
// metric name; Labels is the canonical label string `{k="v",...}` with
// sorted keys ("" when the series has no labels).
type Sample struct {
	TargetID int64   `json:"target_id"`
	Name     string  `json:"name"`
	Labels   string  `json:"labels"`
	Ts       int64   `json:"ts"` // unix milliseconds
	Value    float64 `json:"value"`
}

// AlertRule is a locally-evaluated threshold rule over the query subset.
// Self-hosted alerting must work with no external control plane.
type AlertRule struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Expr       string    `json:"expr"` // PromQL-subset selector, e.g. `up{job="web"}`
	Op         string    `json:"op"`   // one of: >, >=, <, <=, ==, !=
	Value      float64   `json:"value"`
	ForSecs    int64     `json:"for_secs"` // must hold this long before firing
	WebhookURL string    `json:"webhook_url,omitempty"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

// AlertState is the evaluation state of one rule.
type AlertState struct {
	RuleID     int64     `json:"rule_id"`
	State      string    `json:"state"` // pending | firing | resolved
	LastValue  float64   `json:"last_value"`
	LastEvalAt time.Time `json:"last_eval_at"`
	ChangedAt  time.Time `json:"changed_at"`
}

// baseSchema is version-independent DDL: every table shape that has
// existed since v1, created with IF NOT EXISTS so opening a v1 file
// does not fail or redefine anything. Version-specific objects (column
// sets added after v1, indexes on those columns) live in currentSchema
// and run only after migrate() has brought the file to SchemaVersion.
// The samples table is deliberately ABSENT here: its v2 shape (name +
// labels columns) only applies to fresh files; a v1 file keeps its old
// samples table until migrate() rebuilds it.
const baseSchema = `
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS targets (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT    NOT NULL,
	url         TEXT    NOT NULL UNIQUE,
	interval_ms INTEGER NOT NULL,
	enabled     INTEGER NOT NULL DEFAULT 1,
	created_at  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS scrapes (
	target_id    INTEGER NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
	at_ms        INTEGER NOT NULL,
	duration_ms  INTEGER NOT NULL,
	sample_count INTEGER NOT NULL,
	error        TEXT    NOT NULL DEFAULT '',
	PRIMARY KEY (target_id, at_ms)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS alert_rules (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT    NOT NULL UNIQUE,
	expr        TEXT    NOT NULL,
	op          TEXT    NOT NULL,
	value       REAL    NOT NULL,
	for_secs    INTEGER NOT NULL DEFAULT 0,
	webhook_url TEXT    NOT NULL DEFAULT '',
	enabled     INTEGER NOT NULL DEFAULT 1,
	created_at  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS alert_state (
	rule_id     INTEGER PRIMARY KEY REFERENCES alert_rules(id) ON DELETE CASCADE,
	state       TEXT    NOT NULL DEFAULT 'resolved',
	last_value  REAL    NOT NULL DEFAULT 0,
	last_eval_at INTEGER NOT NULL,
	changed_at  INTEGER NOT NULL
);
`

// currentSchema is DDL valid only at SchemaVersion: the v2 samples
// table (fresh files) and indexes over v2 columns.
const currentSchema = `
CREATE TABLE IF NOT EXISTS samples (
	target_id INTEGER NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
	name      TEXT    NOT NULL,
	labels    TEXT    NOT NULL DEFAULT '',
	ts        INTEGER NOT NULL,
	value     REAL    NOT NULL,
	PRIMARY KEY (target_id, name, labels, ts)
);
CREATE INDEX IF NOT EXISTS idx_samples_name_ts ON samples (name, ts);
`

// Store wraps the embedded SQLite database.
type Store struct {
	db *sql.DB
	// writeMu serializes multi-statement write transactions in-process.
	// SQLite's busy_timeout does not retry a read→write upgrade under
	// concurrency (SQLITE_BUSY_SNAPSHOT), so a single-writer mutex is
	// the supported pattern for the embedded case.
	writeMu sync.Mutex
}

// Open creates or opens the SQLite file, applies the schema, and
// migrates older schema versions in place.
func Open(path string) (*Store, error) {
	// Pragmas ride the DSN so they apply to EVERY pooled connection —
	// a bare Exec after Open only affects one pooled connection, which
	// is exactly how "ON DELETE CASCADE silently does nothing" ships.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := db.Exec(`SELECT 1`); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}
	// Version-independent base DDL first: on a v1 file the existing
	// samples table is left alone by IF NOT EXISTS, and version-specific
	// indexes/columns are applied only after migrate() below.
	if _, err := db.Exec(baseSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply base schema: %w", err)
	}
	if _, err := db.Exec(
		`INSERT INTO meta (key, value) VALUES ('schema_version', ?)
		 ON CONFLICT(key) DO NOTHING`, SchemaVersion); err != nil {
		db.Close()
		return nil, fmt.Errorf("record schema_version: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	// Now the schema is at SchemaVersion: apply current-version DDL
	// (indexes etc.) that assumes the current column set.
	if _, err := db.Exec(currentSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply current schema: %w", err)
	}
	return s, nil
}

// SchemaIsCurrent reports whether the on-disk schema version matches
// the code's SchemaVersion. Always true after a successful Open.
func (s *Store) SchemaIsCurrent() bool {
	var v int
	if err := s.db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&v); err != nil {
		return false
	}
	return v == SchemaVersion
}

// migrate upgrades older schema versions in place. Each step runs
// inside one transaction; an empty/fresh database is already current
// (the CREATE TABLE IF NOT EXISTS above defines v2 for new files).
func (s *Store) migrate() error {
	var v int
	if err := s.db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&v); err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}
	if v == SchemaVersion {
		return nil
	}
	if v > SchemaVersion {
		return fmt.Errorf("database schema %d is newer than this binary (%d) — upgrade oneprobe first", v, SchemaVersion)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	switch v {
	case 1:
		if err := s.migrateV1toV2(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown schema version %d", v)
	}

	if _, err := s.db.Exec(`UPDATE meta SET value = ? WHERE key='schema_version'`, SchemaVersion); err != nil {
		return fmt.Errorf("bump schema_version: %w", err)
	}
	return nil
}

// migrateV1toV2 rebuilds the v1 samples table (fused `metric` identity,
// PK (target_id, metric, ts)) into the v2 shape (name + labels columns).
// Identities are split via scrape's parser into bare name + canonical
// labels; malformed label sets keep their full identity in name with
// empty labels (visible, never dropped). Runs in one transaction.
func (s *Store) migrateV1toV2() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin v1→v2: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE samples_v2 (
		target_id INTEGER NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
		name      TEXT    NOT NULL,
		labels    TEXT    NOT NULL DEFAULT '',
		ts        INTEGER NOT NULL,
		value     REAL    NOT NULL,
		PRIMARY KEY (target_id, name, labels, ts)
	)`); err != nil {
		return fmt.Errorf("create samples_v2: %w", err)
	}

	rows, err := tx.Query(`SELECT target_id, metric, ts, value FROM samples`)
	if err != nil {
		return fmt.Errorf("read v1 samples: %w", err)
	}
	type row struct {
		targetID int64
		metric   string
		ts       int64
		value    float64
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.targetID, &r.metric, &r.ts, &r.value); err != nil {
			rows.Close()
			return fmt.Errorf("scan v1 sample: %w", err)
		}
		all = append(all, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read v1 samples: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT INTO samples_v2 (target_id, name, labels, ts, value)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare v1→v2 insert: %w", err)
	}
	defer stmt.Close()
	for _, r := range all {
		name, labels := splitIdentity(r.metric)
		if _, err := stmt.Exec(r.targetID, name, labels, r.ts, r.value); err != nil {
			return fmt.Errorf("insert migrated sample %q ts %d: %w", r.metric, r.ts, err)
		}
	}

	if _, err := tx.Exec(`DROP TABLE samples`); err != nil {
		return fmt.Errorf("drop v1 samples: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE samples_v2 RENAME TO samples`); err != nil {
		return fmt.Errorf("rename samples_v2: %w", err)
	}
	return tx.Commit()
}

// splitIdentity splits a v1 fused identity into bare name and canonical
// label string. Malformed label sets stay whole in the name.
func splitIdentity(fused string) (name, labels string) {
	i := 0
	for i < len(fused) && fused[i] != '{' {
		i++
	}
	if i == len(fused) {
		return fused, ""
	}
	rest := fused[i:]
	if len(rest) < 2 || rest[len(rest)-1] != '}' {
		return fused, ""
	}
	inner := rest[1 : len(rest)-1]
	if pairs, err := scrape.ParseLabels(inner); err == nil {
		return fused[:i], scrape.CanonicalLabels(pairs)
	}
	return fused, ""
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// AddTarget validates and upserts a target. Adding a URL that already
// exists updates that row (same ID) rather than duplicating it.
func (s *Store) AddTarget(t Target) (Target, error) {
	if t.Name == "" {
		return Target{}, errors.New("target name is required")
	}
	u, err := url.Parse(t.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Target{}, fmt.Errorf("target url must be a valid http(s) URL, got %q", t.URL)
	}
	if t.Interval.Duration() <= 0 {
		return Target{}, fmt.Errorf("target interval must be positive, got %s", t.Interval.Duration())
	}
	enabled := 0
	if t.Enabled {
		enabled = 1
	}
	var out Target
	var createdAtUnix int64
	err = s.db.QueryRow(
		`INSERT INTO targets (name, url, interval_ms, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(url) DO UPDATE SET
			name = excluded.name,
			interval_ms = excluded.interval_ms,
			enabled = excluded.enabled
		 RETURNING id, created_at`,
		t.Name, t.URL, t.Interval.Duration().Milliseconds(), enabled, time.Now().UTC().Unix(),
	).Scan(&out.ID, &createdAtUnix)
	if err != nil {
		return Target{}, fmt.Errorf("add target: %w", err)
	}
	out.Name, out.URL, out.Interval, out.Enabled = t.Name, t.URL, t.Interval, t.Enabled
	out.CreatedAt = scrape.Time(time.Unix(createdAtUnix, 0).UTC())
	return out, nil
}

// ListTargets returns all targets, oldest first.
func (s *Store) ListTargets() ([]Target, error) {
	rows, err := s.db.Query(
		`SELECT id, name, url, interval_ms, enabled, created_at
		 FROM targets ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list targets: %w", err)
	}
	defer rows.Close()
	targets := []Target{}
	for rows.Next() {
		var t Target
		var intervalMs, enabled, createdAt int64
		if err := rows.Scan(&t.ID, &t.Name, &t.URL, &intervalMs, &enabled, &createdAt); err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}
		t.Interval = scrape.Millis(time.Duration(intervalMs) * time.Millisecond)
		t.Enabled = enabled == 1
		t.CreatedAt = scrape.Time(time.Unix(createdAt, 0).UTC())
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// DeleteTarget removes a target. Samples are removed by ON DELETE CASCADE.
func (s *Store) DeleteTarget(id int64) error {
	res, err := s.db.Exec(`DELETE FROM targets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete target: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("delete target: no target with id %d", id)
	}
	return nil
}
