package store

import (
	"fmt"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
)

// Point is one stored sample point for range queries.
type Point struct {
	Name   string  `json:"name"`
	Labels string  `json:"labels"`
	Ts     int64   `json:"ts"` // unix milliseconds
	Value  float64 `json:"value"`
}

// ScrapeMeta is one recorded scrape outcome (recent-scrapes UI).
type ScrapeMeta struct {
	TargetID    int64     `json:"target_id"`
	At          time.Time `json:"at"`
	DurationMs  int64     `json:"duration_ms"`
	SampleCount int       `json:"sample_count"`
	Ok          bool      `json:"ok"`
	Error       string    `json:"error,omitempty"`
}

// InsertSamples stores a scrape batch atomically: either every sample
// lands or none do. (target_id, name, labels, ts) conflicts replace the
// previous value — a re-scrape of the same timestamp is an upsert.
func (s *Store) InsertSamples(samples []Sample) error {
	if len(samples) == 0 {
		return nil
	}
	// Validate the whole batch before opening a transaction so a bad
	// row cannot half-land.
	seen := make(map[int64]bool, len(samples))
	for _, smp := range samples {
		if smp.Name == "" {
			return fmt.Errorf("sample with empty name (target %d, ts %d)", smp.TargetID, smp.Ts)
		}
		seen[smp.TargetID] = true
	}
	for id := range seen {
		var exists bool
		if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM targets WHERE id = ?)`, id).Scan(&exists); err != nil {
			return fmt.Errorf("check target %d: %w", id, err)
		}
		if !exists {
			return fmt.Errorf("unknown target id %d in sample batch", id)
		}
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO samples (target_id, name, labels, ts, value)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(target_id, name, labels, ts) DO UPDATE SET value = excluded.value`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()
	for _, smp := range samples {
		if _, err := stmt.Exec(smp.TargetID, smp.Name, smp.Labels, smp.Ts, smp.Value); err != nil {
			return fmt.Errorf("insert sample %s ts %d: %w", smp.Name, smp.Ts, err)
		}
	}
	return tx.Commit()
}

// QueryRange returns points for one bare metric name within [from, to]
// (unix milliseconds, inclusive) across all targets and series,
// ordered by timestamp then labels.
func (s *Store) QueryRange(name string, fromMs, toMs int64) ([]Point, error) {
	rows, err := s.db.Query(
		`SELECT name, labels, ts, value FROM samples
		 WHERE name = ? AND ts >= ? AND ts <= ?
		 ORDER BY ts, labels`, name, fromMs, toMs)
	if err != nil {
		return nil, fmt.Errorf("query range: %w", err)
	}
	defer rows.Close()
	var points []Point
	for rows.Next() {
		var p Point
		if err := rows.Scan(&p.Name, &p.Labels, &p.Ts, &p.Value); err != nil {
			return nil, fmt.Errorf("scan point: %w", err)
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// DeleteSamplesBefore removes samples older than the cutoff (unix ms)
// and returns the number of rows deleted. Retention sweeps call this.
func (s *Store) DeleteSamplesBefore(cutoffMs int64) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`DELETE FROM samples WHERE ts < ?`, cutoffMs)
	if err != nil {
		return 0, fmt.Errorf("delete samples: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete samples: rows affected: %w", err)
	}
	return n, nil
}

// InsertScrapeSamples adapts scrape's parsed samples into store rows:
// map labels to the canonical string form and stamp missing timestamps
// with the scrape time.
func (s *Store) InsertScrapeSamples(targetID int64, samples []scrape.Sample) error {
	if len(samples) == 0 {
		return nil
	}
	rows := make([]Sample, 0, len(samples))
	for _, smp := range samples {
		ts := smp.TsMs
		if ts == 0 {
			ts = time.Now().UTC().UnixMilli()
		}
		rows = append(rows, Sample{
			TargetID: targetID,
			Name:     smp.Name,
			Labels:   scrape.CanonicalLabels(smp.Labels),
			Ts:       ts,
			Value:    smp.Value,
		})
	}
	return s.InsertSamples(rows)
}

// RecordScrape stores one scrape outcome. A nil scrapeErr records a
// successful scrape; a non-nil one records the failure text.
func (s *Store) RecordScrape(targetID int64, at time.Time, durationMs int64, sampleCount int, scrapeErr *string) error {
	errText := ""
	if scrapeErr != nil {
		errText = *scrapeErr
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO scrapes (target_id, at_ms, duration_ms, sample_count, error)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(target_id, at_ms) DO UPDATE SET
			duration_ms = excluded.duration_ms,
			sample_count = excluded.sample_count,
			error = excluded.error`,
		targetID, at.UnixMilli(), durationMs, sampleCount, errText)
	if err != nil {
		return fmt.Errorf("record scrape: %w", err)
	}
	return nil
}

// ListScrapes returns the most recent scrape outcomes for a target,
// newest first, at most limit rows.
func (s *Store) ListScrapes(targetID int64, limit int) ([]ScrapeMeta, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(
		`SELECT target_id, at_ms, duration_ms, sample_count, error
		 FROM scrapes WHERE target_id = ?
		 ORDER BY at_ms DESC LIMIT ?`, targetID, limit)
	if err != nil {
		return nil, fmt.Errorf("list scrapes: %w", err)
	}
	defer rows.Close()
	var out []ScrapeMeta
	for rows.Next() {
		var m ScrapeMeta
		var atMs int64
		if err := rows.Scan(&m.TargetID, &atMs, &m.DurationMs, &m.SampleCount, &m.Error); err != nil {
			return nil, fmt.Errorf("scan scrape: %w", err)
		}
		m.At = time.UnixMilli(atMs).UTC()
		m.Ok = m.Error == ""
		out = append(out, m)
	}
	return out, rows.Err()
}

// SeriesNames returns every distinct bare metric name in samples,
// sorted.
func (s *Store) SeriesNames() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT name FROM samples ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("series names: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("scan name: %w", err)
		}
		names = append(names, n)
	}
	return names, rows.Err()
}
