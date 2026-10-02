package scrape

import (
	"math"
	"strings"
	"testing"
)

func TestParseSimpleSample(t *testing.T) {
	samples, err := Parse("http_requests_total 1027\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1", len(samples))
	}
	s := samples[0]
	if s.Name != "http_requests_total" || s.Value != 1027 || len(s.Labels) != 0 {
		t.Errorf("sample = %+v", s)
	}
	if s.Identity() != "http_requests_total" {
		t.Errorf("Identity = %q, want bare name", s.Identity())
	}
}

func TestParseLabelsAndCanonicalIdentity(t *testing.T) {
	// Labels declared out of key order; identity must sort keys.
	samples, err := Parse(`http_requests_total{method="get",code="200"} 5
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := samples[0].Identity(); got != `http_requests_total{code="200",method="get"}` {
		t.Errorf("Identity = %q, want sorted keys", got)
	}
	if samples[0].Labels["code"] != "200" || samples[0].Labels["method"] != "get" {
		t.Errorf("labels = %v", samples[0].Labels)
	}
}

func TestParseSpecialValues(t *testing.T) {
	body := "a 1.5e-07\nb NaN\nc +Inf\nd -Inf\ne -3.25\n"
	samples, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []struct {
		name string
		val  float64
	}{
		{"a", 1.5e-07},
		{"b", math.NaN()},
		{"c", math.Inf(1)},
		{"d", math.Inf(-1)},
		{"e", -3.25},
	}
	if len(samples) != len(want) {
		t.Fatalf("got %d samples, want %d", len(samples), len(want))
	}
	for i, w := range want {
		if samples[i].Name != w.name {
			t.Errorf("sample %d name = %q, want %q", i, samples[i].Name, w.name)
		}
		if math.IsNaN(w.val) {
			if !math.IsNaN(samples[i].Value) {
				t.Errorf("sample %s = %v, want NaN", w.name, samples[i].Value)
			}
		} else if samples[i].Value != w.val {
			t.Errorf("sample %s = %v, want %v", w.name, samples[i].Value, w.val)
		}
	}
}

func TestParseOptionalTimestampMillis(t *testing.T) {
	samples, err := Parse("up 1 1759305600000\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if samples[0].TsMs != 1759305600000 {
		t.Errorf("TsMs = %d, want 1759305600000", samples[0].TsMs)
	}
}

func TestParseSkipsCommentsHelpTypeAndBlanks(t *testing.T) {
	body := `# HELP http_requests_total Total requests.
# TYPE http_requests_total counter

# a plain comment
http_requests_total 3
`
	samples, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(samples) != 1 || samples[0].Name != "http_requests_total" {
		t.Errorf("got %+v", samples)
	}
}

func TestParseSkipsExemplars(t *testing.T) {
	// OpenMetrics/text exemplar trailers after ' #' must not become data.
	body := "requests_bucket{le=\"0.1\"} 1 # {trace_id=\"abc\"} 1.0 1759305600000\n"
	samples, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(samples) != 1 || samples[0].Value != 1 {
		t.Errorf("got %+v", samples)
	}
}

func TestParseLabelEscapes(t *testing.T) {
	samples, err := Parse(`msg{text="a\"b\\c"} 1
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := samples[0].Labels["text"]; got != `a"b\c` {
		t.Errorf("unescaped label = %q, want %q", got, `a"b\c`)
	}
}

func TestParseMalformedLineNamesLineNumber(t *testing.T) {
	body := "up 1\nno_value_here\nanother 2\n"
	_, err := Parse(body)
	if err == nil {
		t.Fatal("Parse must fail on a line without a value")
	}
	var pe *ParseError
	if !errorsAs(err, &pe) {
		t.Fatalf("error must be *ParseError, got %T: %v", err, err)
	}
	if pe.Line != 2 {
		t.Errorf("ParseError.Line = %d, want 2", pe.Line)
	}
	if !strings.Contains(pe.Error(), "no_value_here") {
		t.Errorf("error must name the offending text, got: %v", pe)
	}
}

func TestParseBadValueNamesLine(t *testing.T) {
	_, err := Parse("up not_a_number\n")
	var pe *ParseError
	if !errorsAs(err, &pe) {
		t.Fatalf("error must be *ParseError, got %T: %v", err, err)
	}
	if pe.Line != 1 || !strings.Contains(pe.Error(), "not_a_number") {
		t.Errorf("error = %v, want line 1 naming the bad value", pe)
	}
}

func TestParseHistogramLinesAreSamples(t *testing.T) {
	body := `request_duration_seconds_bucket{le="0.1"} 100
request_duration_seconds_bucket{le="0.2"} 150
request_duration_seconds_sum 12.5
request_duration_seconds_count 150
`
	samples, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(samples) != 4 {
		t.Fatalf("got %d samples, want 4", len(samples))
	}
	if samples[0].Labels["le"] != "0.1" {
		t.Errorf("le label = %q", samples[0].Labels["le"])
	}
}

func errorsAs(err error, target *(*ParseError)) bool {
	for err != nil {
		if e, ok := err.(*ParseError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
