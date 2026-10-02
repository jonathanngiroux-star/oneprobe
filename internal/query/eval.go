package query

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// SeriesSource is the store contract the evaluator needs.
type SeriesSource interface {
	QuerySeries(name string, fromMs, toMs int64) ([]store.Series, error)
}

// Result is an evaluated vector: one or more series of computed points.
type Result struct {
	Series []store.Series `json:"series"`
}

// Eval evaluates a parsed expression against the store at instant
// `now`: range windows look back from now, the outermost evaluation
// produces the final vector.
func Eval(n Node, src SeriesSource, now time.Time, lookback time.Duration) (Result, error) {
	switch e := n.(type) {
	case Selector:
		return evalSelector(e, src, now, lookback)
	case Call:
		return evalCall(e, src, now)
	case Aggregation:
		return evalAggregation(e, src, now, lookback)
	case Comparison:
		return evalComparison(e, src, now, lookback)
	default:
		return Result{}, fmt.Errorf("unsupported expression node %T", n)
	}
}

func evalSelector(sel Selector, src SeriesSource, now time.Time, lookback time.Duration) (Result, error) {
	all, err := src.QuerySeries(sel.Name, now.Add(-lookback).UnixMilli(), now.UnixMilli())
	if err != nil {
		return Result{}, fmt.Errorf("query %s: %w", sel.Name, err)
	}
	matched, err := filterByMatchers(all, sel.Matchers)
	if err != nil {
		return Result{}, err
	}
	// Keep only the last point of each series: a selector at instant
	// `now` yields one sample per series.
	for i := range matched {
		if len(matched[i].Points) > 0 {
			matched[i].Points = matched[i].Points[len(matched[i].Points)-1:]
		}
	}
	return Result{Series: matched}, nil
}

func evalCall(c Call, src SeriesSource, now time.Time) (Result, error) {
	from := now.Add(-c.Sel.Range)
	all, err := src.QuerySeries(c.Sel.Name, from.UnixMilli(), now.UnixMilli())
	if err != nil {
		return Result{}, fmt.Errorf("query %s: %w", c.Sel.Name, err)
	}
	matched, err := filterByMatchers(all, c.Sel.Matchers)
	if err != nil {
		return Result{}, err
	}
	var out []store.Series
	for _, sr := range matched {
		if len(sr.Points) < 2 {
			// One or zero points in the window: rate undefined, treat
			// as 0 rather than dropping the series (an honest "no
			// signal" for a just-started counter).
			if len(sr.Points) == 1 {
				out = append(out, store.Series{
					Name: c.Sel.Name, TargetID: sr.TargetID, Labels: sr.Labels,
					Points: []store.Point{{Name: c.Sel.Name, Ts: sr.Points[0].Ts, Value: 0}},
				})
			}
			continue
		}
		first, last := sr.Points[0], sr.Points[len(sr.Points)-1]
		spanSecs := float64(last.Ts-first.Ts) / 1000.0
		var v float64
		if spanSecs > 0 {
			switch c.Func {
			case "rate":
				// Counter reset (value dropped, e.g. process restart):
				// Prometheus semantics clamp the step to 0 rather than
				// produce a negative rate.
				delta := last.Value - first.Value
				if delta < 0 {
					delta = 0
				}
				v = delta / spanSecs
			case "increase":
				delta := last.Value - first.Value
				if delta < 0 {
					delta = 0
				}
				v = delta
			}
		}
		out = append(out, store.Series{
			Name: c.Sel.Name, TargetID: sr.TargetID, Labels: sr.Labels,
			Points: []store.Point{{Name: c.Sel.Name, Ts: last.Ts, Value: v}},
		})
	}
	return Result{Series: out}, nil
}

func evalAggregation(a Aggregation, src SeriesSource, now time.Time, lookback time.Duration) (Result, error) {
	inner, err := Eval(a.Inner, src, now, lookback)
	if err != nil {
		return Result{}, err
	}
	// Group series by the `by` labels (or one global group when absent).
	groups := map[string][]store.Series{}
	var order []string
	for _, sr := range inner.Series {
		key := groupKey(sr, a.By)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], sr)
	}
	sort.Strings(order)

	var out []store.Series
	for _, key := range order {
		group := groups[key]
		if len(group) == 0 {
			continue
		}
		// Every series in a group must have the same point count for a
		// temporal aggregation; the store guarantees aligned scrapes
		// per target but not across targets, so aggregate by point
		// index across the intersection length.
		n := math.MaxInt32
		for _, sr := range group {
			if len(sr.Points) < n {
				n = len(sr.Points)
			}
		}
		var pts []store.Point
		for i := 0; i < n; i++ {
			var acc float64
			switch a.Op {
			case "sum", "avg":
				acc = 0
			case "max":
				acc = math.Inf(-1)
			case "min":
				acc = math.Inf(1)
			}
			for _, sr := range group {
				v := sr.Points[i].Value
				switch a.Op {
				case "sum", "avg":
					acc += v
				case "max":
					if v > acc {
						acc = v
					}
				case "min":
					if v < acc {
						acc = v
					}
				}
			}
			if a.Op == "avg" {
				acc /= float64(len(group))
			}
			pts = append(pts, store.Point{Name: a.Op + "(" + group[0].Name + ")", Ts: group[0].Points[i].Ts, Value: acc})
		}
		out = append(out, store.Series{
			Name:     a.Op,
			TargetID: group[0].TargetID,
			Labels:   key,
			Points:   pts,
		})
	}
	return Result{Series: out}, nil
}

func evalComparison(c Comparison, src SeriesSource, now time.Time, lookback time.Duration) (Result, error) {
	left, err := Eval(c.Left, src, now, lookback)
	if err != nil {
		return Result{}, err
	}
	var out []store.Series
	for _, sr := range left.Series {
		var pts []store.Point
		for _, p := range sr.Points {
			if compare(p.Value, c.Op, c.Value) {
				pts = append(pts, p)
			}
		}
		if len(pts) > 0 {
			out = append(out, store.Series{Name: sr.Name, TargetID: sr.TargetID, Labels: sr.Labels, Points: pts})
		}
	}
	return Result{Series: out}, nil
}

func compare(v float64, op string, want float64) bool {
	switch op {
	case ">":
		return v > want
	case ">=":
		return v >= want
	case "<":
		return v < want
	case "<=":
		return v <= want
	case "==":
		return v == want
	case "!=":
		return v != want
	}
	return false
}

// filterByMatchers keeps series whose labels satisfy every matcher.
// Matchers apply to the canonical label string form `{k="v",...}`.
func filterByMatchers(all []store.Series, matchers []Matcher) ([]store.Series, error) {
	if len(matchers) == 0 {
		return all, nil
	}
	var out []store.Series
	for _, sr := range all {
		labels, err := parseLabelString(sr.Labels)
		if err != nil {
			return nil, fmt.Errorf("stored labels %q: %w", sr.Labels, err)
		}
		ok := true
		for _, m := range matchers {
			got, present := labels[m.Name]
			switch m.Op {
			case "=":
				if !present || got != m.Value {
					ok = false
				}
			case "=~":
				if !present {
					ok = false
					break
				}
				re, err := regexp.Compile("^(?:" + m.Value + ")$")
				if err != nil {
					return nil, fmt.Errorf("bad regex %q in matcher: %w", m.Value, err)
				}
				if !re.MatchString(got) {
					ok = false
				}
			}
			if !ok {
				break
			}
		}
		if ok {
			out = append(out, sr)
		}
	}
	return out, nil
}

// parseLabelString parses the canonical `{k="v",...}` form into a map.
// An empty string is no labels.
func parseLabelString(s string) (map[string]string, error) {
	if s == "" {
		return map[string]string{}, nil
	}
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, fmt.Errorf("not canonical label form")
	}
	inner := s[1 : len(s)-1]
	out := map[string]string{}
	if inner == "" {
		return out, nil
	}
	for _, pair := range strings.Split(inner, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("malformed pair %q", pair)
		}
		k := kv[0]
		v := strings.Trim(kv[1], `"`)
		out[k] = v
	}
	return out, nil
}

// groupKey builds the aggregation group's label string from the `by`
// labels ("" for a global group).
func groupKey(sr store.Series, by []string) string {
	if len(by) == 0 {
		return ""
	}
	labels, err := parseLabelString(sr.Labels)
	if err != nil {
		return ""
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, name := range by {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(name)
		b.WriteString(`="`)
		b.WriteString(labels[name])
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}
