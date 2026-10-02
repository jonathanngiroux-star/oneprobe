package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// validOps are the comparison operators an alert rule may use. The rule
// mirrors the query engine's Comparison set exactly.
var validOps = map[string]bool{">": true, ">=": true, "<": true, "<=": true, "==": true, "!=": true}

// AddAlertRule validates and upserts a rule. Adding a rule with an
// existing name updates that row (same ID).
func (s *Store) AddAlertRule(r AlertRule) (AlertRule, error) {
	if r.Name == "" {
		return AlertRule{}, errors.New("alert rule name is required")
	}
	if r.Expr == "" {
		return AlertRule{}, errors.New("alert rule expr is required")
	}
	if !validOps[r.Op] {
		return AlertRule{}, fmt.Errorf("alert rule op %q is not one of > >= < <= == !=", r.Op)
	}
	if r.ForSecs < 0 {
		return AlertRule{}, fmt.Errorf("alert rule for_secs must be >= 0, got %d", r.ForSecs)
	}
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	var out AlertRule
	var createdAt int64
	err := s.db.QueryRow(
		`INSERT INTO alert_rules (name, expr, op, value, for_secs, webhook_url, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
			expr = excluded.expr,
			op = excluded.op,
			value = excluded.value,
			for_secs = excluded.for_secs,
			webhook_url = excluded.webhook_url,
			enabled = excluded.enabled
		 RETURNING id, created_at`,
		r.Name, r.Expr, r.Op, r.Value, r.ForSecs, r.WebhookURL, enabled, time.Now().UTC().Unix(),
	).Scan(&out.ID, &createdAt)
	if err != nil {
		return AlertRule{}, fmt.Errorf("add alert rule: %w", err)
	}
	out.Name, out.Expr, out.Op, out.Value = r.Name, r.Expr, r.Op, r.Value
	out.ForSecs, out.WebhookURL, out.Enabled = r.ForSecs, r.WebhookURL, r.Enabled
	out.CreatedAt = time.Unix(createdAt, 0).UTC()
	return out, nil
}

// ListAlertRules returns all rules, oldest first.
func (s *Store) ListAlertRules() ([]AlertRule, error) {
	rows, err := s.db.Query(
		`SELECT id, name, expr, op, value, for_secs, webhook_url, enabled, created_at
		 FROM alert_rules ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()
	rules := []AlertRule{}
	for rows.Next() {
		var r AlertRule
		var enabled, createdAt int64
		if err := rows.Scan(&r.ID, &r.Name, &r.Expr, &r.Op, &r.Value, &r.ForSecs, &r.WebhookURL, &enabled, &createdAt); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		r.Enabled = enabled == 1
		r.CreatedAt = time.Unix(createdAt, 0).UTC()
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// DeleteAlertRule removes a rule; its alert_state row cascades.
func (s *Store) DeleteAlertRule(id int64) error {
	res, err := s.db.Exec(`DELETE FROM alert_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("delete alert rule: no rule with id %d", id)
	}
	return nil
}

// GetAlertState returns the persisted state of one rule. A rule with no
// state row reports "resolved" with zero timestamps — never an error.
func (s *Store) GetAlertState(ruleID int64) (AlertState, error) {
	var st AlertState
	var lastEval, changed int64
	err := s.db.QueryRow(
		`SELECT state, last_value, last_eval_at, changed_at
		 FROM alert_state WHERE rule_id = ?`, ruleID,
	).Scan(&st.State, &st.LastValue, &lastEval, &changed)
	if errors.Is(err, sql.ErrNoRows) {
		return AlertState{RuleID: ruleID, State: "resolved"}, nil
	}
	if err != nil {
		return AlertState{}, fmt.Errorf("get alert state: %w", err)
	}
	st.RuleID = ruleID
	st.LastEvalAt = time.Unix(lastEval, 0).UTC()
	st.ChangedAt = time.Unix(changed, 0).UTC()
	return st, nil
}

// SetAlertState persists an evaluation result. A state *transition*
// (firing→resolved etc.) moves changed_at; a same-state update only
// refreshes last_value/last_eval_at.
func (s *Store) SetAlertState(ruleID int64, state string, lastValue float64, at time.Time) error {
	if state != "pending" && state != "firing" && state != "resolved" {
		return fmt.Errorf("alert state must be pending|firing|resolved, got %q", state)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO alert_state (rule_id, state, last_value, last_eval_at, changed_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(rule_id) DO UPDATE SET
			state = excluded.state,
			last_value = excluded.last_value,
			last_eval_at = excluded.last_eval_at,
			changed_at = CASE WHEN alert_state.state = excluded.state
				THEN alert_state.changed_at ELSE excluded.changed_at END`,
		ruleID, state, lastValue, at.Unix(), at.Unix())
	if err != nil {
		return fmt.Errorf("set alert state: %w", err)
	}
	return nil
}
