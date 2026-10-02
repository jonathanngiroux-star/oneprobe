package query

import (
	"math"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// TestEvalAllComparisonOps covers every documented comparison operator
// on both satisfying and violating series — the audit found compare()
// and violates() at 25-37% coverage, meaning ==, !=, >=, <= were
// effectively untested.
func TestEvalAllComparisonOps(t *testing.T) {
	base := time.UnixMilli(1759305600000)
	cases := []struct {
		expr  string
		wantN int
	}{
		{"up > 0", 2},
		{"up >= 1", 2},
		{"up < 1", 1},
		{"up <= 0", 1},
		{"up == 1", 2},
		{"up != 1", 1},
	}
	for _, tc := range cases {
		src := fs(
			store.Series{Name: "up", TargetID: 1, Labels: `{h="a"}`, Points: pts(1)},
			store.Series{Name: "up", TargetID: 2, Labels: `{h="b"}`, Points: pts(1)},
			store.Series{Name: "up", TargetID: 3, Labels: `{h="c"}`, Points: pts(0)},
		)
		res, err := Eval(Parse0(t, tc.expr), src, base, time.Hour)
		if err != nil {
			t.Errorf("Eval(%q): %v", tc.expr, err)
			continue
		}
		if len(res.Series) != tc.wantN {
			t.Errorf("Eval(%q) kept %d series, want %d", tc.expr, len(res.Series), tc.wantN)
		}
	}
}

// TestEvalRateCounterResetClampsToZero: a counter reset (value drops)
// must not produce a negative rate — Prometheus semantics clamp the
// step to 0. A negative rate would also feed a bogus "worst violating"
// into alert evaluation.
func TestEvalRateCounterResetClampsToZero(t *testing.T) {
	// Counter: 100 → 120 → 5 (process restarted) → 15.
	src := fs(store.Series{Name: "requests_total", TargetID: 1, Labels: "",
		Points: pts(100, 120, 5, 15)})
	// Window covers the reset.
	base := time.UnixMilli(1759305600000 + 3*15000)
	res, err := Eval(Parse0(t, "rate(requests_total[5m])"), src, base, 5*time.Minute)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 1 {
		t.Fatalf("got %d series, want 1", len(res.Series))
	}
	got := res.Series[0].Points[0].Value
	if got < 0 {
		t.Errorf("rate over a counter reset = %v, want >= 0 (clamp resets to 0)", got)
	}
}

// TestEvalComparisonKeepsNaNOut: NaN must never satisfy > < >= <=
// comparisons (IEEE: all comparisons false). A NaN sample must not fire
// an alert through the comparison path.
func TestEvalComparisonKeepsNaNOut(t *testing.T) {
	base := time.UnixMilli(1759305600000)
	src := fs(store.Series{Name: "temp", TargetID: 1, Labels: "",
		Points: []store.Point{{Name: "temp", Ts: base.UnixMilli(), Value: nan()}}})
	for _, expr := range []string{"temp > 0", "temp < 0", "temp >= 0", "temp <= 0"} {
		res, err := Eval(Parse0(t, expr), src, base, time.Hour)
		if err != nil {
			t.Errorf("Eval(%q): %v", expr, err)
			continue
		}
		if len(res.Series) != 0 {
			t.Errorf("Eval(%q) kept NaN series (%v) — NaN must not satisfy comparisons", expr, res.Series)
		}
	}
}

// TestEvalSelectorLookbackFallsBackAcrossSeries: the instant selector
// takes the LAST point per series in the window — including when that
// point is older than a newer point in another series (no global
// latest, per-series latest).
func TestEvalSelectorPerSeriesLastPoint(t *testing.T) {
	base := time.UnixMilli(1759305600000)
	src := fs(
		store.Series{Name: "up", TargetID: 1, Labels: `{h="a"}`, Points: pts(1, 2)}, // last=2
		store.Series{Name: "up", TargetID: 2, Labels: `{h="b"}`, Points: pts(5, 9)}, // last=9
	)
	res, err := Eval(Parse0(t, "up"), src, base.Add(2*15*time.Second), time.Hour)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 2 {
		t.Fatalf("got %d series, want 2", len(res.Series))
	}
	for _, sr := range res.Series {
		if len(sr.Points) != 1 {
			t.Errorf("series %s: %d points, want 1 (instant)", sr.Labels, len(sr.Points))
		}
	}
	if res.Series[0].Points[0].Value != 2 || res.Series[1].Points[0].Value != 9 {
		t.Errorf("per-series last points wrong: %v, %v", res.Series[0].Points[0].Value, res.Series[1].Points[0].Value)
	}
}

func nan() float64 {
	return math.NaN()
}
