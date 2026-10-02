package query

import "testing"

// TestParseValidExpressions pins the documented PromQL subset's parse
// tree via each node's canonical String() form.
func TestParseValidExpressions(t *testing.T) {
	cases := []struct{ expr, want string }{
		// Selectors.
		{`up`, `up`},
		{`http_requests_total`, `http_requests_total`},
		{`up{job="web"}`, `up{job="web"}`},
		{`up{job="web",instance="i-1"}`, `up{instance="i-1",job="web"}`}, // matchers sorted
		{`up{job=~"web.*"}`, `up{job=~"web.*"}`},
		// Range functions.
		{`rate(http_requests_total[5m])`, `rate(http_requests_total[5m])`},
		{`increase(errors_total[90s])`, `increase(errors_total[90s])`},
		{`rate(up{job="web"}[30s])`, `rate(up{job="web"}[30s])`},
		// Aggregations.
		{`sum(up)`, `sum(up)`},
		{`avg(up)`, `avg(up)`},
		{`max(up)`, `max(up)`},
		{`min(up)`, `min(up)`},
		{`sum by (host) (up)`, `sum by (host) (up)`},
		{`avg by (a,b) (up)`, `avg by (a,b) (up)`},
		{`sum(rate(http_requests_total[5m]))`, `sum(rate(http_requests_total[5m]))`},
		{`sum by (code) (rate(http_requests_total[1m]))`, `sum by (code) (rate(http_requests_total[1m]))`},
		// Comparisons.
		{`up > 1`, `up > 1`},
		{`up >= 1`, `up >= 1`},
		{`up < 0.5`, `up < 0.5`},
		{`up <= 0`, `up <= 0`},
		{`sum(up) == 3`, `sum(up) == 3`},
		{`rate(errors_total[5m]) != 0`, `rate(errors_total[5m]) != 0`},
		// Whitespace tolerance.
		{`  sum(  up  )  `, `sum(up)`},
		{`up { job = "web" }`, `up{job="web"}`},
	}
	for _, tc := range cases {
		n, err := Parse(tc.expr)
		if err != nil {
			t.Errorf("Parse(%q): %v", tc.expr, err)
			continue
		}
		if got := n.String(); got != tc.want {
			t.Errorf("Parse(%q).String() = %q, want %q", tc.expr, got, tc.want)
		}
	}
}

// TestParseErrorsNameTheRejectedInput: every refusal names the bad token
// and, for functions, the supported set — an undocumented function is
// never silently accepted.
func TestParseErrorsNameTheRejectedInput(t *testing.T) {
	cases := []struct{ expr, wantInErr string }{
		{``, "empty expression"},
		{`histogram_quantile(0.9, x)`, `unsupported function "histogram_quantile" (supported: rate, increase)`},
		{`absent(up)`, `unsupported function "absent"`},
		{`up{`, `unexpected end`},
		{`up{job=}`, `expected string value`},
		{`up{job~"x"}`, "expected ="},
		{`rate(http_requests_total[5x])`, "bad duration"},
		{`rate(http_requests_total[])`, "bad duration"},
		{`rate(up)`, "range selector"},
		{`up[5m]`, "range selector"}, // range only valid inside rate/increase
		{`up > `, "expected number"},
		{`up ! 3`, "unexpected"},
		{`count(up)`, `unsupported function "count"`},
		{`sum by (up)`, "expected ("},
	}
	for _, tc := range cases {
		_, err := Parse(tc.expr)
		if err == nil {
			t.Errorf("Parse(%q) = nil error, want one containing %q", tc.expr, tc.wantInErr)
			continue
		}
		if !contains(err.Error(), tc.wantInErr) {
			t.Errorf("Parse(%q) error = %q, want substring %q", tc.expr, err.Error(), tc.wantInErr)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
