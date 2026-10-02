package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// TestQueryEndpointEvaluatesExpressions pins the /api/query?expr=
// surface: a stored series is queryable through the PromQL subset.
func TestQueryEndpointEvaluatesExpressions(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()

	// A target for the samples to belong to.
	if _, err := h.store.AddTarget(store.Target{
		Name: "seed", URL: "http://127.0.0.1:9999/metrics",
		Interval: scrape.Millis(15 * time.Second), Enabled: true,
	}); err != nil {
		t.Fatalf("AddTarget: %v", err)
	}

	now := time.Now().UTC().UnixMilli()
	base := now - 60000
	// host="a" series rise by 2 over the window; host="b" only has
	// code=200, matched out by the selector.
	type seedRow struct {
		name   string
		labels string
		first  float64
		last   float64
	}
	rows := []seedRow{
		{"http_requests_total", `{code="200",host="a"}`, 10, 12},
		{"http_requests_total", `{code="500",host="a"}`, 1, 3},
		{"http_requests_total", `{code="200",host="b"}`, 5, 7},
		{"up", "", 1, 1},
	}
	var batch []store.Sample
	for _, r := range rows {
		batch = append(batch,
			store.Sample{TargetID: 1, Name: r.name, Labels: r.labels, Ts: base, Value: r.first},
			store.Sample{TargetID: 1, Name: r.name, Labels: r.labels, Ts: now, Value: r.last},
		)
	}
	if err := h.store.InsertSamples(batch); err != nil {
		t.Fatalf("seed InsertSamples: %v", err)
	}

	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	// sum by (code) over host="a": instant evaluation takes the last
	// point of each series → code=200 → 12, code=500 → 3.
	q := srv.URL + "/api/query?expr=" + url.QueryEscape(`sum by (code) (http_requests_total{host="a"})`)
	resp, err := http.Get(q)
	if err != nil {
		t.Fatalf("GET /api/query: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}
	var out struct {
		Series []struct {
			Labels string `json:"labels"`
			Points []struct {
				Value float64 `json:"value"`
			} `json:"points"`
		} `json:"series"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	if len(out.Series) != 2 {
		t.Fatalf("got %d series, want 2 (code=200, code=500): %s", len(out.Series), raw)
	}
	for _, sr := range out.Series {
		switch sr.Labels {
		case `{code="200"}`:
			if len(sr.Points) != 1 || sr.Points[0].Value != 12 {
				t.Errorf("code=200 sum = %+v, want 12", sr.Points)
			}
		case `{code="500"}`:
			if len(sr.Points) != 1 || sr.Points[0].Value != 3 {
				t.Errorf("code=500 sum = %+v, want 3", sr.Points)
			}
		default:
			t.Errorf("unexpected group %q", sr.Labels)
		}
	}
}

// TestQueryEndpointRejectsBadExprNamingIt: an undocumented function is
// a 400 naming the function, never a silent empty result.
func TestQueryEndpointRejectsBadExprNamingIt(t *testing.T) {
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer h.Close()
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/query?expr=" + url.QueryEscape(`histogram_quantile(0.9, x)`))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	raw := readAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400: %s", resp.StatusCode, raw)
	}
	if want := `unsupported function "histogram_quantile"`; !containsStr(raw, want) {
		t.Errorf("error must name the function, got: %s", raw)
	}
}

func containsStr(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
