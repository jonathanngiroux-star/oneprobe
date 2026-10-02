// Package alert evaluates threshold rules locally against the query
// engine and notifies via log and webhook. Self-hosted alerting works
// with no external control plane: the engine, state, and notifiers all
// live in the single binary.
package alert

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/query"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// EvalStore is the store contract the engine needs: rules, persisted
// alert state, and series for the query engine.
type EvalStore interface {
	ListAlertRules() ([]store.AlertRule, error)
	GetAlertState(ruleID int64) (store.AlertState, error)
	SetAlertState(ruleID int64, state string, lastValue float64, at time.Time) error
	QuerySeries(name string, fromMs, toMs int64) ([]store.Series, error)
}

// Engine evaluates rules at each tick.
type Engine struct {
	store   EvalStore
	logger  *log.Logger
	httpcli *http.Client
	// selectorLookback: how far back a bare-selector rule looks for a
	// sample (5m, matching the serve query endpoint).
	selectorLookback time.Duration
}

// NewEngine builds an engine. A nil logger disables log notifications;
// a nil http client disables webhooks.
func NewEngine(st EvalStore, logger *log.Logger, httpcli *http.Client) *Engine {
	return &Engine{store: st, logger: logger, httpcli: httpcli, selectorLookback: 5 * time.Minute}
}

// Evaluate runs every enabled rule once at `now`.
func (e *Engine) Evaluate(now time.Time) {
	rules, err := e.store.ListAlertRules()
	if err != nil {
		if e.logger != nil {
			e.logger.Printf("alert: list rules: %v", err)
		}
		return
	}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		e.evaluateRule(r, now)
	}
}

// evaluateRule computes the rule's worst series against its threshold,
// advances the state machine, and notifies on transitions.
func (e *Engine) evaluateRule(r store.AlertRule, now time.Time) {
	expr, err := query.Parse(r.Expr)
	if err != nil {
		if e.logger != nil {
			e.logger.Printf("alert: rule %q expr %q: %v", r.Name, r.Expr, err)
		}
		return
	}
	res, err := query.Eval(expr, e.store, now, e.selectorLookback)
	if err != nil {
		if e.logger != nil {
			e.logger.Printf("alert: rule %q eval: %v", r.Name, err)
		}
		return
	}

	// A single value drives the rule: the series that most violates the
	// threshold. No series → condition not met.
	worst, violated := worstPoint(res.Series, r.Op, r.Value)
	if !violated {
		e.transition(r, "resolved", worst, now)
		return
	}

	state, err := e.store.GetAlertState(r.ID)
	if err != nil {
		state = store.AlertState{RuleID: r.ID, State: "resolved"}
	}
	switch {
	case state.State == "firing":
		// Still violating: refresh value/eval time.
		_ = e.store.SetAlertState(r.ID, "firing", worst, now)
	case r.ForSecs > 0 && state.State != "pending":
		_ = e.store.SetAlertState(r.ID, "pending", worst, now)
		if e.logger != nil {
			e.logger.Printf("alert: %q pending (%s %s %v at %s)", r.Name, r.Expr, r.Op, r.Value, now.Format(time.RFC3339))
		}
	case r.ForSecs > 0 && state.State == "pending":
		if now.Sub(state.ChangedAt) >= time.Duration(r.ForSecs)*time.Second {
			e.transition(r, "firing", worst, now)
		} else {
			_ = e.store.SetAlertState(r.ID, "pending", worst, now)
		}
	default:
		// for=0: violating means firing immediately.
		e.transition(r, "firing", worst, now)
	}
}

// transition moves the rule to state and notifies on the change.
func (e *Engine) transition(r store.AlertRule, state string, value float64, now time.Time) {
	prev, err := e.store.GetAlertState(r.ID)
	if err != nil {
		prev = store.AlertState{RuleID: r.ID, State: "resolved"}
	}
	_ = e.store.SetAlertState(r.ID, state, value, now)
	if prev.State == state {
		return
	}
	switch state {
	case "firing":
		if e.logger != nil {
			e.logger.Printf("alert: %q FIRING (%s %s %v, value=%v at %s)",
				r.Name, r.Expr, r.Op, r.Value, value, now.Format(time.RFC3339))
		}
		e.postWebhook(r, "firing", value, now)
	case "resolved":
		if e.logger != nil {
			e.logger.Printf("alert: %q resolved (value=%v at %s)",
				r.Name, value, now.Format(time.RFC3339))
		}
		e.postWebhook(r, "resolved", value, now)
	}
}

// worstPoint finds the series value that most violates the threshold.
func worstPoint(series []store.Series, op string, threshold float64) (float64, bool) {
	worst := 0.0
	found := false
	for _, sr := range series {
		for _, p := range sr.Points {
			if violates(p.Value, op, threshold) {
				if !found {
					worst, found = p.Value, true
					continue
				}
				// "Most violating": furthest below a < threshold,
				// furthest above a > one.
				switch op {
				case "<", "<=":
					if p.Value < worst {
						worst = p.Value
					}
				case ">", ">=":
					if p.Value > worst {
						worst = p.Value
					}
				default:
					worst = p.Value
				}
			}
		}
	}
	return worst, found
}

func violates(v float64, op string, threshold float64) bool {
	switch op {
	case ">":
		return v > threshold
	case ">=":
		return v >= threshold
	case "<":
		return v < threshold
	case "<=":
		return v <= threshold
	case "==":
		return v == threshold
	case "!=":
		return v != threshold
	}
	return false
}

// postWebhook POSTs the alert to the rule's webhook URL, if any. A
// failed POST is logged, never fatal, and never blocks evaluation.
func (e *Engine) postWebhook(r store.AlertRule, state string, value float64, now time.Time) {
	if r.WebhookURL == "" || e.httpcli == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"rule":      r.Name,
		"expr":      r.Expr,
		"op":        r.Op,
		"threshold": r.Value,
		"state":     state,
		"value":     value,
		"at":        now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		if e.logger != nil {
			e.logger.Printf("alert: webhook %q encode: %v", r.Name, err)
		}
		return
	}
	resp, err := e.httpcli.Post(r.WebhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		if e.logger != nil {
			e.logger.Printf("alert: webhook %q post: %v", r.Name, err)
		}
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		if e.logger != nil {
			e.logger.Printf("alert: webhook %q status %d", r.Name, resp.StatusCode)
		}
	}
}
