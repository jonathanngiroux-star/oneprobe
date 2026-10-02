package scrape

import "testing"

// TestParseHashInsideLabelValue: " # " inside a QUOTED label value is
// legal exposition text; the exemplar stripper must not fire inside
// quotes. A line like metric{note="a # b"} 1 must parse with the value
// preserved — currently the stripper truncates mid-label and the parse
// fails with "unterminated label set".
func TestParseHashInsideLabelValue(t *testing.T) {
	samples, err := Parse(`metric{note="a # b"} 1
`)
	if err != nil {
		t.Fatalf("Parse rejected a legal line: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1", len(samples))
	}
	if got := samples[0].Labels["note"]; got != "a # b" {
		t.Errorf("note = %q, want %q (exemplar stripper fired inside quotes)", got, "a # b")
	}
	if samples[0].Value != 1 {
		t.Errorf("value = %v, want 1", samples[0].Value)
	}
}

// TestParseExemplarStillStrippedOutsideQuotes pins the feature the
// stripper exists for: a real OpenMetrics exemplar trailer after the
// value must not become data.
func TestParseExemplarStillStrippedOutsideQuotes(t *testing.T) {
	samples, err := Parse(`requests_bucket{le="0.1"} 1 # {trace_id="abc"} 1.0 1759305600000
metric{note="x # y"} 5 # {trace_id="z"} 2
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("got %d samples, want 2", len(samples))
	}
	if samples[0].Value != 1 || samples[0].Labels["le"] != "0.1" {
		t.Errorf("sample 0 = %+v", samples[0])
	}
	if samples[1].Value != 5 || samples[1].Labels["note"] != "x # y" {
		t.Errorf("sample 1 = %+v (quoted # must survive, trailing exemplar must not)", samples[1])
	}
}

// TestParseTrailingEscapedBackslashBeforeComma: a label value ending
// in an escaped backslash ("x\\" = literal x\) followed by another
// label is legal. The quote scanner must not treat the closing quote
// after a DOUBLE backslash as escaped — currently the comma split
// misfires and the value silently becomes `x",b="y`.
func TestParseTrailingEscapedBackslashBeforeComma(t *testing.T) {
	// Raw exposition: a="x\\",b="y"
	samples, err := Parse(`metric{a="x\\",b="y"} 1
`)
	if err != nil {
		t.Fatalf("Parse rejected a legal line: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1", len(samples))
	}
	if got := samples[0].Labels["a"]; got != `x\` {
		t.Errorf("a = %q, want %q (quote scanner double-escape bug)", got, `x\`)
	}
	if got := samples[0].Labels["b"]; got != "y" {
		t.Errorf("b = %q, want %q", got, "y")
	}
	if samples[0].Value != 1 {
		t.Errorf("value = %v, want 1", samples[0].Value)
	}
}

// TestParseCommaInsideLabelValue: commas inside quoted values are
// value bytes, not pair separators.
func TestParseCommaInsideLabelValue(t *testing.T) {
	samples, err := Parse(`metric{note="a,b",code="200"} 7
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := samples[0].Labels["note"]; got != "a,b" {
		t.Errorf("note = %q, want %q", got, "a,b")
	}
	if got := samples[0].Labels["code"]; got != "200" {
		t.Errorf("code = %q, want %q", got, "200")
	}
}

// TestParseCloseBraceInsideLabelValue: a quoted label value may contain
// the byte '}' — the label-set terminator must be found outside quotes,
// not at the first '}' in the line.
func TestParseCloseBraceInsideLabelValue(t *testing.T) {
	samples, err := Parse(`metric{note="}"} 1
`)
	if err != nil {
		t.Fatalf("Parse rejected a legal line: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1", len(samples))
	}
	if got := samples[0].Labels["note"]; got != "}" {
		t.Errorf("note = %q, want %q", got, "}")
	}
	if samples[0].Value != 1 {
		t.Errorf("value = %v, want 1", samples[0].Value)
	}
}

// TestCanonicalLabelsRoundTripsThroughParseLabels: canonical output
// must re-parse to the identical label set (the store's v1→v2
// migration and the query engine both rely on this equivalence).
func TestCanonicalLabelsRoundTripsThroughParseLabels(t *testing.T) {
	cases := []map[string]string{
		{"note": "a # b"},
		{"note": "a,b"},
		{"a": `x\`},
		{"text": `a"b\c`},
		{"code": "200", "host": "web-1"},
	}
	for _, labels := range cases {
		canon := CanonicalLabels(labels)
		parsed, err := ParseLabels(canon[1 : len(canon)-1])
		if err != nil {
			t.Errorf("ParseLabels(%q): %v", canon, err)
			continue
		}
		if len(parsed) != len(labels) {
			t.Errorf("round-trip %v → %q → %v: length %d, want %d", labels, canon, parsed, len(parsed), len(labels))
			continue
		}
		for k, v := range labels {
			if parsed[k] != v {
				t.Errorf("round-trip %v → %q → %v: [%s] = %q, want %q", labels, canon, parsed, k, parsed[k], v)
			}
		}
	}
}
