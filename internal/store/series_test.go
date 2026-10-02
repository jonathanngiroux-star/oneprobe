package store

import (
	"testing"
)

func TestQuerySeriesGroupsByLabels(t *testing.T) {
	s := openTestStore(t)
	tg := addWebTarget(t, s)
	base := int64(1759305600000)
	batch := []Sample{
		{TargetID: tg.ID, Name: "http_requests_total", Labels: `{code="200"}`, Ts: base, Value: 1},
		{TargetID: tg.ID, Name: "http_requests_total", Labels: `{code="200"}`, Ts: base + 1000, Value: 2},
		{TargetID: tg.ID, Name: "http_requests_total", Labels: `{code="500"}`, Ts: base, Value: 5},
		{TargetID: tg.ID, Name: "cpu_idle", Labels: "", Ts: base, Value: 90},
	}
	if err := s.InsertSamples(batch); err != nil {
		t.Fatalf("InsertSamples: %v", err)
	}

	series, err := s.QuerySeries("http_requests_total", base, base+1000)
	if err != nil {
		t.Fatalf("QuerySeries: %v", err)
	}
	if len(series) != 2 {
		t.Fatalf("got %d series, want 2", len(series))
	}
	// Ordered by labels: {code="200"} before {code="500"}.
	if series[0].Labels != `{code="200"}` || len(series[0].Points) != 2 {
		t.Errorf("series 0 = %+v", series[0])
	}
	if series[0].Points[0].Ts != base || series[0].Points[1].Value != 2 {
		t.Errorf("series 0 points = %+v", series[0].Points)
	}
	if series[1].Labels != `{code="500"}` || len(series[1].Points) != 1 || series[1].Points[0].Value != 5 {
		t.Errorf("series 1 = %+v", series[1])
	}
}

func TestQuerySeriesEmptyWindowReturnsNothing(t *testing.T) {
	s := openTestStore(t)
	tg := addWebTarget(t, s)
	if err := s.InsertSamples([]Sample{{TargetID: tg.ID, Name: "up", Ts: 1759305600000, Value: 1}}); err != nil {
		t.Fatalf("InsertSamples: %v", err)
	}
	series, err := s.QuerySeries("up", 0, 1000)
	if err != nil {
		t.Fatalf("QuerySeries: %v", err)
	}
	if len(series) != 0 {
		t.Errorf("got %d series, want 0 for empty window", len(series))
	}
}
