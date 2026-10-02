// Package scrape parses the Prometheus text exposition format and
// scrapes targets over HTTP. The parser is the wire boundary: it must
// reject malformed lines loudly (naming the line and text) rather than
// silently dropping data.
package scrape

import (
	"bufio"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Sample is one parsed exposition line.
type Sample struct {
	Name   string
	Labels map[string]string
	Value  float64
	TsMs   int64 // 0 = scrape time (exposition timestamp omitted)
}

// Identity is the canonical series key: name plus labels with sorted
// keys. Two scrapes of the same series produce identical identities.
func (s Sample) Identity() string {
	if len(s.Labels) == 0 {
		return s.Name
	}
	keys := make([]string, 0, len(s.Labels))
	for k := range s.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(s.Name)
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(EscapeLabelValue(s.Labels[k]))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

// CanonicalLabels renders a label map as the canonical string form
// `{k="v",...}` with sorted keys ("" for an empty map).
func CanonicalLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	return labelsBody(labels)
}

// labelsBody renders `{k="v",...}` sorted (no bare-name form).
func labelsBody(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(EscapeLabelValue(labels[k]))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

// ParseLabels parses a label body (the inside of `{...}`) into a map.
// Exported for the store's v1→v2 identity migration.
func ParseLabels(body string) (map[string]string, error) {
	return parseLabels(body)
}

func escapeLabelValue(v string) string {
	return EscapeLabelValue(v)
}

// EscapeLabelValue escapes a raw label value for the exposition form.
func EscapeLabelValue(v string) string {
	var b strings.Builder
	for _, r := range v {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParseError names the offending line number and text.
type ParseError struct {
	Line int
	Text string
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d %q: %v", e.Line, e.Text, e.Err)
}

// Unwrap exposes the underlying cause for errors.Is/As.
func (e *ParseError) Unwrap() error { return e.Err }

// Parse reads a Prometheus text exposition body and returns its
// samples. Comments (HELP/TYPE/plain), blank lines, and OpenMetrics
// exemplar trailers (" # ...") are skipped. Malformed lines fail the
// whole parse with a *ParseError naming line number and text.
func Parse(body string) ([]Sample, error) {
	var samples []Sample
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		s, err := parseLine(line, lineNo)
		if err != nil {
			return nil, err
		}
		samples = append(samples, s)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return samples, nil
}

// exemplarStart finds the offset where an OpenMetrics exemplar trailer
// (` # `) begins, ignoring occurrences inside quoted label values. -1
// when there is none.
func exemplarStart(line string) int {
	for i := 0; i+2 < len(line); i++ {
		if line[i] != ' ' || line[i+1] != '#' || line[i+2] != ' ' {
			continue
		}
		if !insideQuotes(line, i) {
			return i
		}
	}
	return -1
}

// insideQuotes reports whether offset i sits inside a double-quoted
// label value. A quote is a CLOSING quote only when preceded by an
// even number of backslashes: in `x\\"` the second backslash is itself
// escaped, so the `"` after it really closes.
func insideQuotes(s string, upto int) bool {
	inQuote := false
	for i := 0; i < upto; i++ {
		switch s[i] {
		case '"':
			if escapedHere(s, i) {
				continue // escaped quote: value byte, no toggle
			}
			inQuote = !inQuote
		}
	}
	return inQuote
}

// escapedHere reports whether the byte at i is preceded by an odd number
// of backslashes (i.e. is itself escaped).
func escapedHere(s string, i int) bool {
	n := 0
	for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
}

func parseLine(line string, lineNo int) (Sample, error) {
	// Strip an exemplar/trailer: everything from " # " onward — but
	// only OUTSIDE quotes, where ` # ` is legal label-value bytes.
	if i := exemplarStart(line); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}

	name, labels, rest, err := splitMetricIdentifier(line)
	if err != nil {
		return Sample{}, &ParseError{Line: lineNo, Text: line, Err: err}
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return Sample{}, &ParseError{Line: lineNo, Text: line, Err: fmt.Errorf("missing value")}
	}
	value, err := parseValue(fields[0])
	if err != nil {
		return Sample{}, &ParseError{Line: lineNo, Text: line, Err: err}
	}
	var ts int64
	if len(fields) >= 2 {
		ts, err = strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return Sample{}, &ParseError{Line: lineNo, Text: line, Err: fmt.Errorf("bad timestamp %q: %w", fields[1], err)}
		}
	}
	return Sample{Name: name, Labels: labels, Value: value, TsMs: ts}, nil
}

// splitMetricIdentifier splits "name{labels} rest" into its parts.
func splitMetricIdentifier(line string) (name string, labels map[string]string, rest string, err error) {
	i := 0
	for i < len(line) && (isNameByte(line[i])) {
		i++
	}
	name = line[:i]
	if name == "" {
		return "", nil, "", fmt.Errorf("missing metric name")
	}
	labels = map[string]string{}
	if i < len(line) && line[i] == '{' {
		// Find the label-set terminator: the first '}' OUTSIDE quotes
		// ('}' is legal value bytes inside a quoted label).
		end := -1
		for j := i; j < len(line); j++ {
			if line[j] == '}' && !insideQuotes(line, j) {
				end = j - i
				break
			}
		}
		if end < 0 {
			return "", nil, "", fmt.Errorf("unterminated label set")
		}
		labelBody := line[i+1 : i+end]
		rest = strings.TrimSpace(line[i+end+1:])
		if labelBody != "" {
			labels, err = parseLabels(labelBody)
			if err != nil {
				return "", nil, "", err
			}
		}
		return name, labels, rest, nil
	}
	rest = strings.TrimSpace(line[i:])
	return name, labels, rest, nil
}

func isNameByte(c byte) bool {
	return c == '_' || c == ':' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func parseLabels(body string) (map[string]string, error) {
	labels := map[string]string{}
	// Split on commas not inside quoted values.
	parts := splitTopLevel(body)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		eq := strings.IndexByte(p, '=')
		if eq < 0 {
			return nil, fmt.Errorf("malformed label %q (want k=\"v\")", p)
		}
		key := strings.TrimSpace(p[:eq])
		valRaw := strings.TrimSpace(p[eq+1:])
		if len(valRaw) < 2 || valRaw[0] != '"' || valRaw[len(valRaw)-1] != '"' {
			return nil, fmt.Errorf("malformed label value %q", p)
		}
		val, err := unescapeLabelValue(valRaw[1 : len(valRaw)-1])
		if err != nil {
			return nil, err
		}
		if key == "" {
			return nil, fmt.Errorf("empty label name in %q", p)
		}
		labels[key] = val
	}
	return labels, nil
}

// splitTopLevel splits on commas outside double quotes. A quote is a
// CLOSING quote only when not escaped: `x\\"` ends the value (the
// second backslash is itself escaped), `x\"` does not.
func splitTopLevel(s string) []string {
	var parts []string
	last := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			if escapedHere(s, i) {
				continue // escaped quote: value byte, no toggle
			}
			inQuote = !inQuote
		case ',':
			if !inQuote {
				parts = append(parts, s[last:i])
				last = i + 1
			}
		}
	}
	parts = append(parts, s[last:])
	return parts
}

func unescapeLabelValue(v string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		if v[i] != '\\' {
			b.WriteByte(v[i])
			continue
		}
		if i+1 >= len(v) {
			return "", fmt.Errorf("dangling escape in label value %q", v)
		}
		switch v[i+1] {
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case 'n':
			b.WriteByte('\n')
		default:
			return "", fmt.Errorf("unsupported escape \\%c in label value", v[i+1])
		}
		i++
	}
	return b.String(), nil
}

// parseValue accepts the exposition special values: NaN, +Inf, -Inf,
// and any decimal/exponent float.
func parseValue(s string) (float64, error) {
	switch s {
	case "NaN":
		return math.NaN(), nil
	case "+Inf", "Inf":
		return math.Inf(1), nil
	case "-Inf":
		return math.Inf(-1), nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("bad value %q: %w", s, err)
	}
	return v, nil
}
