package alert

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

func setSeries(st *fakeEvalStore, v float64) {
	st.series["up"] = []store.Series{{
		Name: "up", TargetID: 1, Labels: `{host="a"}`,
		Points: []store.Point{{Name: "up", Ts: 1759305600000, Value: v}},
	}}
}

// TestForTimerRestartsAfterFlap: a `for: 30s` rule that goes pending,
// clears, then violates again must restart its hold from the SECOND
// violation — not fire 30s after the first. A stale ChangedAt would
// fire the alert for a condition that only re-appeared moments ago.
func TestForTimerRestartsAfterFlap(t *testing.T) {
	st := newFakeEvalStore()
	st.rules = []store.AlertRule{{
		ID: 8, Name: "flap", Expr: "up", Op: "<", Value: 1, ForSecs: 30, Enabled: true,
	}}
	var logBuf bytes.Buffer
	eng := NewEngine(st, log.New(&logBuf, "", 0), nil)
	t0 := time.Unix(1759305600, 0).UTC()

	setSeries(st, 0)
	eng.Evaluate(t0)
	s, _ := st.GetAlertState(8)
	if s.State != "pending" {
		t.Fatalf("t0: state = %q, want pending", s.State)
	}

	// Condition clears at +10s: pending must resolve, not stick.
	setSeries(st, 1)
	eng.Evaluate(t0.Add(10 * time.Second))
	s, _ = st.GetAlertState(8)
	if s.State != "resolved" {
		t.Fatalf("+10s (clear): state = %q, want resolved (pending must not stick)", s.State)
	}

	// Violate again at +12s: pending, timer restarts from HERE.
	setSeries(st, 0)
	eng.Evaluate(t0.Add(12 * time.Second))
	s, _ = st.GetAlertState(8)
	if s.State != "pending" {
		t.Fatalf("+12s (re-violate): state = %q, want pending", s.State)
	}
	changedAt := s.ChangedAt
	if !changedAt.Equal(t0.Add(12 * time.Second)) {
		t.Errorf("pending ChangedAt = %v, want +12s (timer must restart)", changedAt)
	}

	// 29s into the new hold: still pending, not firing.
	eng.Evaluate(t0.Add(41 * time.Second))
	s, _ = st.GetAlertState(8)
	if s.State != "pending" {
		t.Errorf("+41s: state = %q, want pending (29s < for=30s since restart)", s.State)
	}

	// 30s since the re-violation: fires.
	eng.Evaluate(t0.Add(42 * time.Second))
	s, _ = st.GetAlertState(8)
	if s.State != "firing" {
		t.Errorf("+42s: state = %q, want firing (30s held since restart)", s.State)
	}
}

// TestEngineEqualityOps: == and != are legal rule ops and evaluate
// literally.
func TestEngineEqualityOps(t *testing.T) {
	st := newFakeEvalStore()
	st.rules = []store.AlertRule{{
		ID: 9, Name: "eq", Expr: "up", Op: "==", Value: 1, Enabled: true,
	}}
	var logBuf bytes.Buffer
	eng := NewEngine(st, log.New(&logBuf, "", 0), nil)
	now := time.Unix(1759305600, 0).UTC()

	setSeries(st, 1) // up == 1 → fires
	eng.Evaluate(now)
	s, _ := st.GetAlertState(9)
	if s.State != "firing" {
		t.Errorf("up==1: state = %q, want firing", s.State)
	}

	setSeries(st, 2) // no longer == 1 → resolves
	eng.Evaluate(now.Add(time.Second))
	s, _ = st.GetAlertState(9)
	if s.State != "resolved" {
		t.Errorf("up==2: state = %q, want resolved", s.State)
	}

	// != op
	st2 := newFakeEvalStore()
	st2.rules = []store.AlertRule{{
		ID: 10, Name: "ne", Expr: "up", Op: "!=", Value: 1, Enabled: true,
	}}
	eng2 := NewEngine(st2, log.New(&bytes.Buffer{}, "", 0), nil)
	setSeries(st2, 5) // up != 1 → fires
	eng2.Evaluate(now)
	s2, _ := st2.GetAlertState(10)
	if s2.State != "firing" {
		t.Errorf("up!=1 with up=5: state = %q, want firing", s2.State)
	}
}

// TestEngineInvalidExprIsLoggedNotFatal: a rule with an unparseable
// expression must not crash evaluation of the OTHER rules, and the
// message must name the broken rule.
func TestEngineInvalidExprIsLoggedNotFatal(t *testing.T) {
	st := newFakeEvalStore()
	st.rules = []store.AlertRule{
		{ID: 11, Name: "broken", Expr: "up >< 3", Op: "<", Value: 1, Enabled: true},
		{ID: 12, Name: "healthy", Expr: "up", Op: "<", Value: 1, Enabled: true},
	}
	var logBuf bytes.Buffer
	eng := NewEngine(st, log.New(&logBuf, "", 0), nil)

	setSeries(st, 0)
	eng.Evaluate(time.Unix(1759305600, 0).UTC()) // must not panic

	if !containsStr(logBuf.String(), `"broken"`) {
		t.Errorf("log must name the broken rule, got: %s", logBuf.String())
	}
	// The healthy rule still fired.
	s, _ := st.GetAlertState(12)
	if s.State != "firing" {
		t.Errorf("healthy rule state = %q, want firing (broken rule must not block it)", s.State)
	}
	// The broken rule's state is untouched.
	s, _ = st.GetAlertState(11)
	if s.State != "resolved" {
		t.Errorf("broken rule state = %q, want resolved (untouched)", s.State)
	}
}

// TestWebhookPayloadShape pins the exact JSON the webhook delivers —
// it is the integration contract for whatever receives it.
func TestWebhookPayloadShape(t *testing.T) {
	type payload struct {
		Rule      string  `json:"rule"`
		Expr      string  `json:"expr"`
		Op        string  `json:"op"`
		Threshold float64 `json:"threshold"`
		State     string  `json:"state"`
		Value     float64 `json:"value"`
		At        string  `json:"at"`
	}
	var got payload
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(200)
		close(done)
	}))
	defer srv.Close()

	st := newFakeEvalStore()
	st.rules = []store.AlertRule{{
		ID: 13, Name: "shaped", Expr: "up", Op: "<", Value: 1,
		WebhookURL: srv.URL, Enabled: true,
	}}
	setSeries(st, 0)
	eng := NewEngine(st, log.New(&bytes.Buffer{}, "", 0), &http.Client{Timeout: 2 * time.Second})
	eng.Evaluate(time.Unix(1759305600, 0).UTC())
	<-done

	if got.Rule != "shaped" || got.State != "firing" || got.Value != 0 || got.Threshold != 1 || got.Op != "<" {
		t.Errorf("payload = %+v", got)
	}
	if got.At == "" {
		t.Error("payload.at missing")
	}
}
