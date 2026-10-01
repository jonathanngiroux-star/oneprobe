// Package store is Oneprobe's embedded SQLite layer: targets, samples
// (bounded retention), alert rules, and alert state — all in the single
// binary, no external database.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

	_ "modernc.org/sqlite"
)

// SchemaVersion is the current SQLite schema version, recorded in meta.
const SchemaVersion = 1

// Target is one Prometheus-format scrape destination.
type Target struct {
	ID        int64         `json:"id"`
	Name      string        `json:"name"`
	URL       string        `json:"url"`
	Interval  time.Duration `json:"interval"` // scrape interval
	Enabled   bool          `json:"enabled"`
	CreatedAt time.Time     `json:"created_at"`
}

// Sample is one scraped time-series point for a target. Metric carries
// the series identity (name plus labels, e.g. `http_requests_total{code="200"}`).
type Sample struct {
	TargetID int64   `json:"target_id"`
	Metric   string  `json:"metric"`
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

const schema = `
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
CREATE TABLE IF NOT EXISTS samples (
	target_id INTEGER NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
	metric    TEXT    NOT NULL,
	ts        INTEGER NOT NULL,
	value     REAL    NOT NULL,
	PRIMARY KEY (target_id, metric, ts)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples (ts);
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

// Store wraps the embedded SQLite database.
type Store struct {
	db *sql.DB
}

// Open creates or opens the SQLite file and applies the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	// Single-writer pragmas: WAL for concurrent reads during scrapes,
	// busy_timeout for plain write contention. (Multi-statement write
	// transactions are serialized in-process once the scrape loop lands.)
	if _, err := db.Exec(`PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("pragmas: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if _, err := db.Exec(
		`INSERT INTO meta (key, value) VALUES ('schema_version', ?)
		 ON CONFLICT(key) DO NOTHING`, SchemaVersion); err != nil {
		db.Close()
		return nil, fmt.Errorf("record schema_version: %w", err)
	}
	return &Store{db: db}, nil
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
	if t.Interval <= 0 {
		return Target{}, fmt.Errorf("target interval must be positive, got %s", t.Interval)
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
		t.Name, t.URL, t.Interval.Milliseconds(), enabled, time.Now().UTC().Unix(),
	).Scan(&out.ID, &createdAtUnix)
	if err != nil {
		return Target{}, fmt.Errorf("add target: %w", err)
	}
	out.Name, out.URL, out.Interval, out.Enabled = t.Name, t.URL, t.Interval, t.Enabled
	out.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
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
	var targets []Target
	for rows.Next() {
		var t Target
		var intervalMs, enabled, createdAt int64
		if err := rows.Scan(&t.ID, &t.Name, &t.URL, &intervalMs, &enabled, &createdAt); err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}
		t.Interval = time.Duration(intervalMs) * time.Millisecond
		t.Enabled = enabled == 1
		t.CreatedAt = time.Unix(createdAt, 0).UTC()
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
