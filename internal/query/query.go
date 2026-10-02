// Package query implements Oneprobe's documented PromQL subset:
// selectors with = and =~ matchers, rate()/increase() over ranges,
// sum/avg/max/min aggregations with optional `by` grouping, and
// numeric comparisons. Anything outside the subset is a named parse
// error — never a silent mis-evaluation. testdata/promql/golden.json
// pins the supported set; docs/query.md is the contract.
package query

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Node is one parsed expression.
type Node interface{ String() string }

// Matcher is one label matcher: `=` exact or `=~` regex.
type Matcher struct {
	Name  string `json:"name"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

// Selector is a metric name plus optional label matchers. Range is
// nonzero only inside a rate()/increase() call.
type Selector struct {
	Name     string        `json:"name"`
	Matchers []Matcher     `json:"matchers"` // sorted by name
	Range    time.Duration `json:"range_ns"`
}

// Call is rate()/increase() over a range selector.
type Call struct {
	Func string   `json:"func"` // rate | increase
	Sel  Selector `json:"sel"`
}

// Aggregation is sum/avg/max/min over an inner expression, optionally
// grouped `by` label names.
type Aggregation struct {
	Op    string   `json:"op"` // sum | avg | max | min
	By    []string `json:"by,omitempty"`
	Inner Node     `json:"inner"`
}

// Comparison is an expression compared to a numeric constant; it
// filters vectors to those that satisfy the comparison.
type Comparison struct {
	Left  Node    `json:"left"`
	Op    string  `json:"op"` // > >= < <= == !=
	Value float64 `json:"value"`
}

func (s Selector) String() string {
	var b strings.Builder
	b.WriteString(s.Name)
	if len(s.Matchers) > 0 {
		b.WriteByte('{')
		for i, m := range s.Matchers {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(m.Name)
			b.WriteString(m.Op)
			b.WriteString(strconv.Quote(m.Value))
		}
		b.WriteByte('}')
	}
	return b.String()
}

func (c Call) String() string {
	return c.Func + "(" + c.Sel.String() + "[" + FormatDuration(c.Sel.Range) + "]" + ")"
}

func (a Aggregation) String() string {
	if len(a.By) == 0 {
		return a.Op + "(" + a.Inner.String() + ")"
	}
	return a.Op + " by (" + strings.Join(a.By, ",") + ") (" + a.Inner.String() + ")"
}

func (c Comparison) String() string {
	return c.Left.String() + " " + c.Op + " " + strconv.FormatFloat(c.Value, 'f', -1, 64)
}

// FormatDuration renders a duration in PromQL units (w d h m s ms),
// largest exact unit first: 5m, 90s, 1500ms.
func FormatDuration(d time.Duration) string {
	units := []struct {
		name string
		div  time.Duration
	}{
		{"w", 7 * 24 * time.Hour},
		{"d", 24 * time.Hour},
		{"h", time.Hour},
		{"m", time.Minute},
		{"s", time.Second},
		{"ms", time.Millisecond},
	}
	for _, u := range units {
		if d >= u.div && d%u.div == 0 {
			return strconv.FormatInt(int64(d/u.div), 10) + u.name
		}
	}
	return strconv.FormatInt(int64(d/time.Millisecond), 10) + "ms"
}

// Supported functions — the whole documented set. The parser refuses
// anything else by name.
var (
	rangeFuncs = map[string]bool{"rate": true, "increase": true}
	aggFuncs   = map[string]bool{"sum": true, "avg": true, "max": true, "min": true}
)

// Parse parses one expression of the documented subset.
func Parse(expr string) (Node, error) {
	toks, err := lex(expr)
	if err != nil {
		return nil, err
	}
	if len(toks) == 0 {
		return nil, fmt.Errorf("empty expression")
	}
	p := &parser{toks: toks}
	n, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.toks) {
		return nil, fmt.Errorf("unexpected token %q after expression", p.toks[p.pos].text)
	}
	return n, nil
}

// ---- lexer ----

type tokKind int

const (
	tIdent tokKind = iota
	tString
	tNumber
	tDuration
	tOp
)

type token struct {
	kind tokKind
	text string // raw text (idents, ops, strings unquoted)
	num  float64
	dur  time.Duration
}

func lex(s string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '"':
			j := i + 1
			for j < len(s) && s[j] != '"' {
				j++
			}
			if j >= len(s) {
				return nil, fmt.Errorf("unterminated string at %d", i)
			}
			toks = append(toks, token{kind: tString, text: s[i+1 : j]})
			i = j + 1
		case isDigit(c):
			j := i
			for j < len(s) && isDigit(s[j]) {
				j++
			}
			dot := false
			if j < len(s) && s[j] == '.' && j+1 < len(s) && isDigit(s[j+1]) {
				dot = true
				j++
				for j < len(s) && isDigit(s[j]) {
					j++
				}
			}
			// Unit letters after an integer make a duration.
			if !dot && j < len(s) && isUnitStart(s[j]) {
				k := j
				for k < len(s) && isLetter(s[k]) {
					k++
				}
				text := s[i:k]
				d, err := parseDurationUnits(s[j:k], text)
				if err != nil {
					return nil, err
				}
				toks = append(toks, token{kind: tDuration, text: text, dur: d})
				i = k
				continue
			}
			if !dot && j < len(s) && isLetter(s[j]) {
				// Digits followed by non-unit letters: bad duration.
				k := j
				for k < len(s) && isLetter(s[k]) {
					k++
				}
				return nil, fmt.Errorf("bad duration %q (supported units: ms s m h d w)", s[i:k])
			}
			v, err := strconv.ParseFloat(s[i:j], 64)
			if err != nil {
				return nil, fmt.Errorf("bad number %q", s[i:j])
			}
			toks = append(toks, token{kind: tNumber, text: s[i:j], num: v})
			i = j
		case isLetter(c) || c == '_':
			j := i
			for j < len(s) && (isLetter(s[j]) || isDigit(s[j]) || s[j] == '_' || s[j] == ':') {
				j++
			}
			toks = append(toks, token{kind: tIdent, text: s[i:j]})
			i = j
		case c == '=':
			if i+1 < len(s) && s[i+1] == '~' {
				toks = append(toks, token{kind: tOp, text: "=~"})
				i += 2
			} else if i+1 < len(s) && s[i+1] == '=' {
				toks = append(toks, token{kind: tOp, text: "=="})
				i += 2
			} else {
				toks = append(toks, token{kind: tOp, text: "="})
				i++
			}
		case c == '!':
			if i+1 < len(s) && s[i+1] == '=' {
				toks = append(toks, token{kind: tOp, text: "!="})
				i += 2
			} else {
				toks = append(toks, token{kind: tOp, text: "!"})
				i++
			}
		case c == '>':
			if i+1 < len(s) && s[i+1] == '=' {
				toks = append(toks, token{kind: tOp, text: ">="})
				i += 2
			} else {
				toks = append(toks, token{kind: tOp, text: ">"})
				i++
			}
		case c == '<':
			if i+1 < len(s) && s[i+1] == '=' {
				toks = append(toks, token{kind: tOp, text: "<="})
				i += 2
			} else {
				toks = append(toks, token{kind: tOp, text: "<"})
				i++
			}
		case c == '~':
			toks = append(toks, token{kind: tOp, text: "~"})
			i++
		case c == '(' || c == ')' || c == '{' || c == '}' || c == '[' || c == ']' || c == ',':
			toks = append(toks, token{kind: tOp, text: string(c)})
			i++
		default:
			return nil, fmt.Errorf("unexpected character %q at %d", string(c), i)
		}
	}
	return toks, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
func isUnitStart(c byte) bool { return isLetter(c) }

// parseDurationUnits converts unit letters to a duration.
func parseDurationUnits(units, full string) (time.Duration, error) {
	table := map[string]time.Duration{
		"ms": time.Millisecond, "s": time.Second, "m": time.Minute,
		"h": time.Hour, "d": 24 * time.Hour, "w": 7 * 24 * time.Hour,
	}
	if len(units) == 0 {
		return 0, fmt.Errorf("bad duration %q", full)
	}
	if d, ok := table[units]; ok {
		// Number part was scanned separately; recover it from full.
		numText := full[:len(full)-len(units)]
		n, err := strconv.ParseInt(numText, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("bad duration %q", full)
		}
		return time.Duration(n) * d, nil
	}
	return 0, fmt.Errorf("bad duration %q (supported units: ms s m h d w)", full)
}

// ---- parser ----

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peek() *token {
	if p.pos < len(p.toks) {
		return &p.toks[p.pos]
	}
	return nil
}

func (p *parser) next() *token {
	t := p.peek()
	if t != nil {
		p.pos++
	}
	return t
}

func (p *parser) expectOp(text string) error {
	t := p.next()
	if t == nil {
		return fmt.Errorf("expected %q, got end of input", text)
	}
	if t.kind != tOp || t.text != text {
		return fmt.Errorf("expected %q, got %q", text, t.text)
	}
	return nil
}

func (p *parser) parseExpr() (Node, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t != nil && t.kind == tOp {
		switch t.text {
		case ">", ">=", "<", "<=", "==", "!=":
			op := p.next().text
			num := p.next()
			if num == nil || num.kind != tNumber {
				got := "end of input"
				if num != nil {
					got = num.text
				}
				return nil, fmt.Errorf("expected number after %s, got %q", op, got)
			}
			return Comparison{Left: left, Op: op, Value: num.num}, nil
		case "[":
			return nil, fmt.Errorf("range selector is only valid inside rate()/increase()")
		case "!":
			return nil, fmt.Errorf("unexpected operator %q (comparisons: > >= < <= == !=)", p.next().text)
		}
	}
	return left, nil
}

func (p *parser) parsePrimary() (Node, error) {
	t := p.peek()
	if t == nil || t.kind != tIdent {
		got := "end of input"
		if t != nil {
			got = t.text
		}
		return nil, fmt.Errorf("expected metric name, got %q", got)
	}
	name := p.next().text
	next := p.peek()

	switch {
	case aggFuncs[name] && next != nil && next.kind == tIdent && next.text == "by":
		p.next() // consume "by"
		by, err := p.parseByList()
		if err != nil {
			return nil, err
		}
		if err := p.expectOp("("); err != nil {
			return nil, fmt.Errorf("expected ( after by-list: %v", err)
		}
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expectOp(")"); err != nil {
			return nil, err
		}
		return Aggregation{Op: name, By: by, Inner: inner}, nil

	case aggFuncs[name] && next != nil && next.kind == tOp && next.text == "(":
		p.next() // consume "("
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expectOp(")"); err != nil {
			return nil, err
		}
		return Aggregation{Op: name, Inner: inner}, nil

	case rangeFuncs[name]:
		if next == nil || next.kind != tOp || next.text != "(" {
			return nil, fmt.Errorf("%s() requires a range selector like metric[5m]", name)
		}
		p.next() // consume "("
		sel, err := p.parseSelector()
		if err != nil {
			return nil, err
		}
		bt := p.peek()
		if bt == nil || bt.kind != tOp || bt.text != "[" {
			got := "end of input"
			if bt != nil {
				got = bt.text
			}
			return nil, fmt.Errorf("%s() requires a range selector like metric[5m], got %q", name, got)
		}
		p.next() // consume "["
		dt := p.next()
		if dt == nil || dt.kind != tDuration {
			got := "end of input"
			if dt != nil {
				got = dt.text
			}
			return nil, fmt.Errorf("bad duration: expected a duration like 5m inside [ ], got %q", got)
		}
		if err := p.expectOp("]"); err != nil {
			return nil, err
		}
		if err := p.expectOp(")"); err != nil {
			return nil, err
		}
		sel.Range = dt.dur
		return Call{Func: name, Sel: sel}, nil

	case next != nil && next.kind == tOp && next.text == "(":
		return nil, fmt.Errorf("unsupported function %q (supported: rate, increase)", name)
	}

	return p.parseSelectorFromName(name)
}

func (p *parser) parseByList() ([]string, error) {
	if err := p.expectOp("("); err != nil {
		return nil, err
	}
	var labels []string
	for {
		t := p.next()
		if t == nil {
			return nil, fmt.Errorf("unexpected end of expression inside by-list")
		}
		if t.kind == tOp && t.text == ")" {
			return labels, nil
		}
		if t.kind != tIdent {
			return nil, fmt.Errorf("expected label name in by-list, got %q", t.text)
		}
		labels = append(labels, t.text)
		nt := p.peek()
		if nt != nil && nt.kind == tOp && nt.text == "," {
			p.next()
		}
	}
}

// parseSelector parses name + optional matchers (no range).
func (p *parser) parseSelector() (Selector, error) {
	t := p.peek()
	if t == nil || t.kind != tIdent {
		return Selector{}, fmt.Errorf("expected metric name, got end of input")
	}
	name := p.next().text
	return p.parseSelectorFromName(name)
}

func (p *parser) parseSelectorFromName(name string) (Selector, error) {
	sel := Selector{Name: name}
	t := p.peek()
	if t == nil || t.kind != tOp || t.text != "{" {
		return sel, nil
	}
	p.next() // consume "{"
	for {
		nt := p.peek()
		if nt == nil {
			return Selector{}, fmt.Errorf("unexpected end of expression inside label matchers")
		}
		if nt.kind == tOp && nt.text == "}" {
			p.next()
			break
		}
		lt := p.next()
		if lt.kind != tIdent {
			return Selector{}, fmt.Errorf("expected label name, got %q", lt.text)
		}
		ot := p.next()
		if ot == nil {
			return Selector{}, fmt.Errorf("unexpected end of expression inside label matchers")
		}
		if ot.kind != tOp || (ot.text != "=" && ot.text != "=~") {
			return Selector{}, fmt.Errorf("expected = or =~ after label name %q, got %q", lt.text, ot.text)
		}
		vt := p.next()
		if vt == nil {
			return Selector{}, fmt.Errorf("unexpected end of expression inside label matchers")
		}
		if vt.kind != tString {
			return Selector{}, fmt.Errorf("expected string value for label %q, got %q", lt.text, vt.text)
		}
		sel.Matchers = append(sel.Matchers, Matcher{Name: lt.text, Op: ot.text, Value: vt.text})
		ct := p.peek()
		if ct != nil && ct.kind == tOp && ct.text == "," {
			p.next()
		}
	}
	sort.Slice(sel.Matchers, func(i, j int) bool { return sel.Matchers[i].Name < sel.Matchers[j].Name })
	return sel, nil
}
