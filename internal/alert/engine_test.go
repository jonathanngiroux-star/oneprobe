package alert

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/query"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// fakeEvalStore feeds the engine's store dependency: rules + states in
// memory, series from a query fake.
type fakeEvalStore struct {
	mu     sync.Mutex
	rules  []store.AlertRule
	states map[int64]store.AlertState
	series map[string][]store.Series
}

func newFakeEvalStore() *fakeEvalStore {
	return &fakeEvalStore{states: map[int64]store.AlertState{}, series: map[string][]store.Series{}}
}

func (f *fakeEvalStore) ListAlertRules() ([]store.AlertRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.AlertRule, len(f.rules))
	copy(out, f.rules)
	return out, nil
}

func (f *fakeEvalStore) GetAlertState(id int64) (store.AlertState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if st, ok := f.states[id]; ok {
		return st, nil
	}
	return store.AlertState{RuleID: id, State: "resolved"}, nil
}

func (f *fakeEvalStore) SetAlertState(id int64, state string, value float64, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	prev, ok := f.states[id]
	st := store.AlertState{RuleID: id, State: state, LastValue: value, LastEvalAt: at}
	if ok && prev.State == state {
		st.ChangedAt = prev.ChangedAt
	} else {
		st.ChangedAt = at
	}
	f.states[id] = st
	return nil
}

// QuerySeries feeds the query engine's selector evaluation.
func (f *fakeEvalStore) QuerySeries(name string, fromMs, toMs int64) ([]store.Series, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Series, len(f.series[name]))
	copy(out, f.series[name])
	return out, nil
}

// TestEngineFiresWhenThresholdCrossed: a rule `up < 1` against a stored
// 0 goes resolved → firing on the first tick (for=0).
func TestEngineFiresWhenThresholdCrossed(t *testing.T) {
	st := newFakeEvalStore()
	st.rules = []store.AlertRule{{
		ID: 1, Name: "host-down", Expr: "up", Op: "<", Value: 1, Enabled: true,
	}}
	st.series["up"] = []store.Series{{
		Name: "up", TargetID: 1, Labels: `{host="a"}`,
		Points: []store.Point{{Name: "up", Ts: 1759305600000, Value: 0}},
	}}

	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)
	eng := NewEngine(st, logger, nil)
	now := time.Unix(1759305600, 0).UTC()
	eng.Evaluate(now)

	s, _ := st.GetAlertState(1)
	if s.State != "firing" {
		t.Errorf("state = %q, want firing (up=0 < 1)\nlog: %s", s.State, logBuf.String())
	}
	if !containsStr(logBuf.String(), "host-down") || !containsStr(logBuf.String(), "FIRING") {
		t.Errorf("log notifier must name the rule and state, got: %s", logBuf.String())
	}
}

// TestEnginePendingForDuration: for=30s requires the condition to hold
// 30s before firing; crossing once is pending, holding is firing.
func TestEnginePendingForDuration(t *testing.T) {
	st := newFakeEvalStore()
	st.rules = []store.AlertRule{{
		ID: 2, Name: "slow-burn", Expr: "up", Op: "<", Value: 1, ForSecs: 30, Enabled: true,
	}}
	st.series["up"] = []store.Series{{
		Name: "up", TargetID: 1, Labels: "",
		Points: []store.Point{{Name: "up", Ts: 1759305600000, Value: 0}},
	}}

	var logBuf bytes.Buffer
	eng := NewEngine(st, log.New(&logBuf, "", 0), nil)
	t0 := time.Unix(1759305600, 0).UTC()

	eng.Evaluate(t0)
	s, _ := st.GetAlertState(2)
	if s.State != "pending" {
		t.Fatalf("after first eval: state = %q, want pending (for=30s not held)", s.State)
	}

	// +10s: still pending.
	eng.Evaluate(t0.Add(10 * time.Second))
	s, _ = st.GetAlertState(2)
	if s.State != "pending" {
		t.Fatalf("at +10s: state = %q, want pending", s.State)
	}

	// +31s: fires.
	eng.Evaluate(t0.Add(31 * time.Second))
	s, _ = st.GetAlertState(2)
	if s.State != "firing" {
		t.Fatalf("at +31s: state = %q, want firing", s.State)
	}
}

// TestEngineResolvesWhenConditionClears.
func TestEngineResolvesWhenConditionClears(t *testing.T) {
	st := newFakeEvalStore()
	st.rules = []store.AlertRule{{
		ID: 3, Name: "flappy", Expr: "up", Op: "<", Value: 1, Enabled: true,
	}}
	var logBuf bytes.Buffer
	eng := NewEngine(st, log.New(&logBuf, "", 0), nil)
	t0 := time.Unix(1759305600, 0).UTC()

	st.series["up"] = []store.Series{{
		Name: "up", TargetID: 1, Labels: "",
		Points: []store.Point{{Name: "up", Ts: t0.UnixMilli(), Value: 0}},
	}}
	eng.Evaluate(t0)
	s, _ := st.GetAlertState(3)
	if s.State != "firing" {
		t.Fatalf("setup: state = %q, want firing", s.State)
	}

	// Condition clears.
	st.series["up"] = []store.Series{{
		Name: "up", TargetID: 1, Labels: "",
		Points: []store.Point{{Name: "up", Ts: t0.Add(time.Minute).UnixMilli(), Value: 1}},
	}}
	eng.Evaluate(t0.Add(time.Minute))
	s, _ = st.GetAlertState(3)
	if s.State != "resolved" {
		t.Errorf("after recovery: state = %q, want resolved\nlog: %s", s.State, logBuf.String())
	}
	if !containsStr(logBuf.String(), "resolved") {
		t.Errorf("recovery must be logged, got: %s", logBuf.String())
	}
}

// TestEngineEmptyResultResolves: no series at all (target gone, no data)
// is NOT firing — an absent metric is not a 0.
func TestEngineEmptyResultResolves(t *testing.T) {
	st := newFakeEvalStore()
	st.rules = []store.AlertRule{{
		ID: 4, Name: "missing-metric", Expr: "no_such_metric", Op: "<", Value: 100, Enabled: true,
	}}
	var logBuf bytes.Buffer
	eng := NewEngine(st, log.New(&logBuf, "", 0), nil)
	eng.Evaluate(time.Unix(1759305600, 0).UTC())
	s, _ := st.GetAlertState(4)
	if s.State != "resolved" {
		t.Errorf("absent metric must resolve, got %q", s.State)
	}
}

// TestEngineSkipsDisabledRules.
func TestEngineSkipsDisabledRules(t *testing.T) {
	st := newFakeEvalStore()
	st.rules = []store.AlertRule{{
		ID: 5, Name: "off", Expr: "up", Op: "<", Value: 1, Enabled: false,
	}}
	st.series["up"] = []store.Series{{
		Name: "up", TargetID: 1, Labels: "",
		Points: []store.Point{{Name: "up", Ts: 1759305600000, Value: 0}},
	}}
	var logBuf bytes.Buffer
	eng := NewEngine(st, log.New(&logBuf, "", 0), nil)
	eng.Evaluate(time.Unix(1759305600, 0).UTC())
	if logBuf.Len() != 0 {
		t.Errorf("disabled rule must not be evaluated, log: %s", logBuf.String())
	}
	s, _ := st.GetAlertState(5)
	if s.State != "resolved" {
		t.Errorf("disabled rule state must stay resolved, got %q", s.State)
	}
}

// TestWebhookNotifierPosts: a firing rule with a webhook URL POSTs the
// alert payload to it.
func TestWebhookNotifierPosts(t *testing.T) {
	var mu sync.Mutex
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		gotBody = b
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	st := newFakeEvalStore()
	st.rules = []store.AlertRule{{
		ID: 6, Name: "hooked", Expr: "up", Op: "<", Value: 1,
		WebhookURL: srv.URL, Enabled: true,
	}}
	st.series["up"] = []store.Series{{
		Name: "up", TargetID: 1, Labels: `{host="a"}`,
		Points: []store.Point{{Name: "up", Ts: 1759305600000, Value: 0}},
	}}

	var logBuf bytes.Buffer
	eng := NewEngine(st, log.New(&logBuf, "", 0), &http.Client{Timeout: 2 * time.Second})
	eng.Evaluate(time.Unix(1759305600, 0).UTC())

	mu.Lock()
	defer mu.Unlock()
	if gotBody == nil {
		t.Fatal("webhook was not called")
	}
	if gotBody["rule"] != "hooked" || gotBody["state"] != "firing" {
		t.Errorf("webhook payload = %v, want rule=hooked state=firing", gotBody)
	}
	if gotBody["value"] != float64(0) {
		t.Errorf("webhook value = %v, want 0", gotBody["value"])
	}
	if _, ok := gotBody["at"]; !ok {
		t.Error("webhook payload must carry an `at` timestamp")
	}
}

// TestWebhookFailureDoesNotPanicOrBlockEvaluation: a dead webhook is
// logged, not fatal.
func TestWebhookFailureDoesNotPanicOrBlockEvaluation(t *testing.T) {
	st := newFakeEvalStore()
	st.rules = []store.AlertRule{{
		ID: 7, Name: "dead-hook", Expr: "up", Op: "<", Value: 1,
		WebhookURL: "http://127.0.0.1:1/nope", Enabled: true,
	}}
	st.series["up"] = []store.Series{{
		Name: "up", TargetID: 1, Labels: "",
		Points: []store.Point{{Name: "up", Ts: 1759305600000, Value: 0}},
	}}
	var logBuf bytes.Buffer
	eng := NewEngine(st, log.New(&logBuf, "", 0), &http.Client{Timeout: 200 * time.Millisecond})
	eng.Evaluate(time.Unix(1759305600, 0).UTC()) // must not panic or hang
	s, _ := st.GetAlertState(7)
	if s.State != "firing" {
		t.Errorf("state must still fire despite dead webhook, got %q", s.State)
	}
	if !containsStr(logBuf.String(), "webhook") {
		t.Errorf("dead webhook must be logged, got: %s", logBuf.String())
	}
}

// Compile-time: the engine consumes the query package.
var _ = query.Parse

func containsStr(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
