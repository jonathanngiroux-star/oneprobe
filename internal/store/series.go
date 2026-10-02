package store

import (
	"fmt"
)

// Series is one label-grouped series of points from QuerySeries.
type Series struct {
	Name     string  `json:"name"`
	TargetID int64   `json:"target_id"`
	Labels   string  `json:"labels"`
	Points   []Point `json:"points"`
}

// QuerySeries returns every series for a bare metric name within
// [from, to] (unix milliseconds, inclusive), grouped by (target, label
// set) — the same label set from two targets is two series — ordered
// by labels then target, points ordered by ts. The query engine's
// selector evaluation starts here.
func (s *Store) QuerySeries(name string, fromMs, toMs int64) ([]Series, error) {
	rows, err := s.db.Query(
		`SELECT target_id, labels, ts, value FROM samples
		 WHERE name = ? AND ts >= ? AND ts <= ?
		 ORDER BY labels, target_id, ts`, name, fromMs, toMs)
	if err != nil {
		return nil, fmt.Errorf("query series: %w", err)
	}
	defer rows.Close()

	out := []Series{}
	for rows.Next() {
		var targetID int64
		var labels string
		var p Point
		if err := rows.Scan(&targetID, &labels, &p.Ts, &p.Value); err != nil {
			return nil, fmt.Errorf("scan series point: %w", err)
		}
		p.Name = name
		if len(out) == 0 || out[len(out)-1].TargetID != targetID || out[len(out)-1].Labels != labels {
			out = append(out, Series{Name: name, TargetID: targetID, Labels: labels})
		}
		out[len(out)-1].Points = append(out[len(out)-1].Points, p)
	}
	return out, rows.Err()
}
