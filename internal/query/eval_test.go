package query

import (
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// fakeStore implements SeriesSource over hand-built series.
type fakeStore struct {
	series map[string][]store.Series // metric name → series
}

func (f *fakeStore) QuerySeries(name string, fromMs, toMs int64) ([]store.Series, error) {
	return f.series[name], nil
}

func fs(series ...store.Series) *fakeStore {
	s := &fakeStore{series: map[string][]store.Series{}}
	for _, sr := range series {
		s.series[sr.Name] = append(s.series[sr.Name], sr)
	}
	return s
}

func pts(vals ...float64) []store.Point {
	var out []store.Point
	t := int64(1759305600000)
	for _, v := range vals {
		out = append(out, store.Point{Ts: t, Value: v})
		t += 15000 // 15s steps
	}
	return out
}

func TestEvalBareSelector(t *testing.T) {
	src := fs(store.Series{Name: "up", TargetID: 1, Labels: "", Points: pts(1)})
	res, err := Eval(Parse0(t, "up"), src, time.UnixMilli(1759305600000), time.Hour)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 1 || res.Series[0].Labels != "" || res.Series[0].Points[0].Value != 1 {
		t.Errorf("res = %+v", res.Series)
	}
}

func TestEvalLabelMatcherFilters(t *testing.T) {
	src := fs(
		store.Series{Name: "http_requests_total", TargetID: 1, Labels: `{code="200"}`, Points: pts(1)},
		store.Series{Name: "http_requests_total", TargetID: 1, Labels: `{code="500"}`, Points: pts(2)},
	)
	res, err := Eval(Parse0(t, `http_requests_total{code="500"}`), src, time.UnixMilli(1759305600000), time.Hour)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 1 || res.Series[0].Labels != `{code="500"}` {
		t.Errorf("matcher did not filter: %+v", res.Series)
	}
}

func TestEvalRegexMatcher(t *testing.T) {
	src := fs(
		store.Series{Name: "up", TargetID: 1, Labels: `{job="web-1"}`, Points: pts(1)},
		store.Series{Name: "up", TargetID: 2, Labels: `{job="db"}`, Points: pts(1)},
	)
	res, err := Eval(Parse0(t, `up{job=~"web.*"}`), src, time.UnixMilli(1759305600000), time.Hour)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 1 || res.Series[0].Labels != `{job="web-1"}` {
		t.Errorf("regex matcher wrong: %+v", res.Series)
	}
}

func TestEvalRate(t *testing.T) {
	// Counter 10→16 over 60s → rate = 6/60s = 0.1/s.
	src := fs(store.Series{Name: "http_requests_total", TargetID: 1, Labels: "",
		Points: pts(10, 12, 14, 16)}) // 4 points, 15s apart → 45s span
	res, err := Eval(Parse0(t, `rate(http_requests_total[1m])`), src, time.UnixMilli(1759305600000+45000), time.Minute)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 1 || len(res.Series[0].Points) != 1 {
		t.Fatalf("res = %+v", res.Series)
	}
	// rate = (last - first) / (t_last - t_first) in seconds.
	want := (16.0 - 10.0) / 45.0
	got := res.Series[0].Points[0].Value
	if got < want-1e-9 || got > want+1e-9 {
		t.Errorf("rate = %v, want %v", got, want)
	}
}

func TestEvalIncrease(t *testing.T) {
	src := fs(store.Series{Name: "http_requests_total", TargetID: 1, Labels: "",
		Points: pts(10, 12, 14, 16)})
	res, err := Eval(Parse0(t, `increase(http_requests_total[1m])`), src, time.UnixMilli(1759305600000+45000), time.Minute)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	got := res.Series[0].Points[0].Value
	if got < 6.0-1e-9 || got > 6.0+1e-9 {
		t.Errorf("increase = %v, want 6", got)
	}
}

func TestEvalRateSinglePointIsZero(t *testing.T) {
	src := fs(store.Series{Name: "x", TargetID: 1, Labels: "", Points: pts(5)})
	res, err := Eval(Parse0(t, `rate(x[5m])`), src, time.UnixMilli(1759305600000), 5*time.Minute)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 1 || res.Series[0].Points[0].Value != 0 {
		t.Errorf("single-point rate should be 0, got %+v", res.Series)
	}
}

func TestEvalSumAggregatesAcrossSeries(t *testing.T) {
	src := fs(
		store.Series{Name: "up", TargetID: 1, Labels: `{host="a"}`, Points: pts(1)},
		store.Series{Name: "up", TargetID: 2, Labels: `{host="b"}`, Points: pts(1)},
		store.Series{Name: "up", TargetID: 3, Labels: `{host="c"}`, Points: pts(0)},
	)
	res, err := Eval(Parse0(t, `sum(up)`), src, time.UnixMilli(1759305600000), time.Hour)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 1 || len(res.Series[0].Points) != 1 || res.Series[0].Points[0].Value != 2 {
		t.Errorf("sum(up) = %+v, want one series with value 2", res.Series)
	}
	if res.Series[0].Labels != "" {
		t.Errorf("sum() drops labels, got %q", res.Series[0].Labels)
	}
}

func TestEvalSumByKeepsGroupLabels(t *testing.T) {
	src := fs(
		store.Series{Name: "http_requests_total", TargetID: 1, Labels: `{code="200",host="a"}`, Points: pts(10)},
		store.Series{Name: "http_requests_total", TargetID: 2, Labels: `{code="500",host="a"}`, Points: pts(3)},
		store.Series{Name: "http_requests_total", TargetID: 3, Labels: `{code="200",host="b"}`, Points: pts(7)},
	)
	res, err := Eval(Parse0(t, `sum by (code) (http_requests_total)`), src, time.UnixMilli(1759305600000), time.Hour)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(res.Series), res.Series)
	}
	want := map[string]float64{`{code="200"}`: 17, `{code="500"}`: 3}
	for _, sr := range res.Series {
		if want[sr.Labels] != sr.Points[0].Value {
			t.Errorf("group %s = %v, want %v", sr.Labels, sr.Points[0].Value, want[sr.Labels])
		}
	}
}

func TestEvalAvgMaxMin(t *testing.T) {
	src := fs(
		store.Series{Name: "latency", TargetID: 1, Labels: `{h="a"}`, Points: pts(10)},
		store.Series{Name: "latency", TargetID: 2, Labels: `{h="b"}`, Points: pts(20)},
		store.Series{Name: "latency", TargetID: 3, Labels: `{h="c"}`, Points: pts(30)},
	)
	base := time.UnixMilli(1759305600000)
	for _, tc := range []struct {
		expr string
		want float64
	}{
		{"avg(latency)", 20}, {"max(latency)", 30}, {"min(latency)", 10},
	} {
		res, err := Eval(Parse0(t, tc.expr), src, base, time.Hour)
		if err != nil {
			t.Fatalf("Eval(%s): %v", tc.expr, err)
		}
		if res.Series[0].Points[0].Value != tc.want {
			t.Errorf("%s = %v, want %v", tc.expr, res.Series[0].Points[0].Value, tc.want)
		}
	}
}

func TestEvalComparisonFilters(t *testing.T) {
	src := fs(
		store.Series{Name: "up", TargetID: 1, Labels: `{h="a"}`, Points: pts(1)},
		store.Series{Name: "up", TargetID: 2, Labels: `{h="b"}`, Points: pts(0)},
	)
	base := time.UnixMilli(1759305600000)
	res, err := Eval(Parse0(t, `up == 0`), src, base, time.Hour)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 1 || res.Series[0].Labels != `{h="b"}` {
		t.Errorf("up == 0 should keep only h=b: %+v", res.Series)
	}
	res, err = Eval(Parse0(t, `up > 0`), src, base, time.Hour)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 1 || res.Series[0].Labels != `{h="a"}` {
		t.Errorf("up > 0 should keep only h=a: %+v", res.Series)
	}
}

func TestEvalSumRateComposition(t *testing.T) {
	// Two hosts, each rising by 2 over the window → sum(rate)=2/45 per
	// host, total 4/45.
	src := fs(
		store.Series{Name: "http_requests_total", TargetID: 1, Labels: `{h="a"}`, Points: pts(10, 12)},
		store.Series{Name: "http_requests_total", TargetID: 2, Labels: `{h="b"}`, Points: pts(5, 7)},
	)
	res, err := Eval(Parse0(t, `sum(rate(http_requests_total[1m]))`), src, time.UnixMilli(1759305600000+15000), time.Minute)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	want := (2.0 / 15.0) * 2
	got := res.Series[0].Points[0].Value
	if got < want-1e-9 || got > want+1e-9 {
		t.Errorf("sum(rate(...)) = %v, want %v", got, want)
	}
}

func TestEvalEmptyMetricIsEmptyResult(t *testing.T) {
	src := fs()
	res, err := Eval(Parse0(t, "up"), src, time.UnixMilli(1759305600000), time.Hour)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(res.Series) != 0 {
		t.Errorf("want empty result, got %+v", res.Series)
	}
}

// Parse0 fails the test on a parse error — evaluator tests only test
// evaluation.
func Parse0(t *testing.T, expr string) Node {
	t.Helper()
	n, err := Parse(expr)
	if err != nil {
		t.Fatalf("Parse(%q): %v", expr, err)
	}
	return n
}
