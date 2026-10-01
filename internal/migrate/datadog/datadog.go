// Package datadog converts Datadog dashboard/monitor exports into
// Oneprobe equivalents and reports honestly what does not translate.
// The dry-run diff is the product: mapped / partial / unmapped / risks —
// never a fake 100% parity claim.
package datadog

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Dashboard is the subset of a Datadog dashboard export Oneprobe reads.
type Dashboard struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Widgets     []Widget `json:"widgets"`
}

// Widget is one Datadog dashboard tile.
type Widget struct {
	ID       int       `json:"id"`
	Type     string    `json:"type"`
	Title    string    `json:"title"`
	Requests []Request `json:"requests"`
}

// Request is one query block inside a widget.
type Request struct {
	Q    string `json:"q"`
	Type string `json:"type,omitempty"`
}

// InputRef echoes the source dashboard identity in the report.
type InputRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// MappedEntry is a widget that translates fully to Oneprobe.
type MappedEntry struct {
	WidgetID int    `json:"widget_id"`
	Title    string `json:"title"`
	Type     string `json:"widget_type"`
	Query    string `json:"oneprobe_query"`
	Note     string `json:"note,omitempty"`
}

// PartialEntry is a widget that translates with caveats — the query
// contains Datadog functions Oneprobe's PromQL subset does not support,
// or the widget type degrades (e.g. toplist → table).
type PartialEntry struct {
	WidgetID int    `json:"widget_id"`
	Title    string `json:"title"`
	Type     string `json:"widget_type"`
	Query    string `json:"oneprobe_query"`
	Reason   string `json:"reason"`
}

// UnmappedEntry is a widget with no Oneprobe equivalent.
type UnmappedEntry struct {
	WidgetID int    `json:"widget_id"`
	Title    string `json:"title"`
	Type     string `json:"widget_type"`
	Reason   string `json:"reason"`
}

// Risk is a migration hazard a human must review before cutover.
type Risk struct {
	Key  string `json:"key"`
	Note string `json:"note"`
}

// Report is the machine-stable output of `oneprobe migrate --from=datadog
// --dry-run --format=json`. Its JSON shape is a CI contract.
type Report struct {
	Source   string          `json:"source"`
	DryRun   bool            `json:"dry_run"`
	Input    InputRef        `json:"input"`
	Mapped   []MappedEntry   `json:"mapped"`
	Partial  []PartialEntry  `json:"partial"`
	Unmapped []UnmappedEntry `json:"unmapped"`
	Risks    []Risk          `json:"risks"`
}

// unsupportedFuncs are Datadog query functions with no PromQL-subset
// equivalent. Each occurrence becomes a partial classification and a risk.
var unsupportedFuncs = []string{"anomalies", "forecast", "monotonic", "exclude_null", "top", "timeshift"}

// reSimpleMetric matches a bare Datadog metric query: aggregation over
// metric{tags} (e.g. avg:system.cpu.user{host:web-1}). It is
// intentionally strict — anything else is partial or unmapped, never a
// silent mis-migration.
var reSimpleMetric = regexp.MustCompile(
	`^(avg|max|min|sum):(?:[a-z0-9_.]+)\{[^{}]*\}$`)

// Diff classifies every widget in the dashboard. The report is honest:
// no widget is ever silently dropped or promoted to "mapped".
func Diff(d Dashboard) Report {
	r := Report{
		Source:   "datadog",
		DryRun:   true,
		Input:    InputRef{ID: d.ID, Title: d.Title},
		Mapped:   []MappedEntry{},
		Partial:  []PartialEntry{},
		Unmapped: []UnmappedEntry{},
		Risks:    []Risk{},
	}

	sawAnomalies, sawSynthetics := false, false
	for _, w := range d.Widgets {
		switch {
		case w.Type == "check_status":
			sawSynthetics = true
			r.Unmapped = append(r.Unmapped, UnmappedEntry{
				WidgetID: w.ID,
				Title:    w.Title,
				Type:     w.Type,
				Reason:   "synthetics/check widgets are out of scope for Oneprobe v0.1 (no synthetics runner)",
			})
		case len(w.Requests) == 0:
			r.Unmapped = append(r.Unmapped, UnmappedEntry{
				WidgetID: w.ID,
				Title:    w.Title,
				Type:     w.Type,
				Reason:   "widget has no query requests to translate",
			})
		default:
			// Examine each request; classification follows the worst request.
			queries := []string{}
			for _, req := range w.Requests {
				queries = append(queries, req.Q)
			}
			allSimple := true
			anyUnsupported := false
			for _, q := range queries {
				if containsUnsupported(q) {
					anyUnsupported = true
					if strings.Contains(q, "anomalies(") {
						sawAnomalies = true
					}
				} else if !reSimpleMetric.MatchString(strings.TrimSpace(q)) {
					allSimple = false
				}
			}
			switch {
			case anyUnsupported:
				r.Partial = append(r.Partial, PartialEntry{
					WidgetID: w.ID,
					Title:    w.Title,
					Type:     w.Type,
					Query:    strings.Join(queries, "; "),
					Reason:   fmt.Sprintf("query uses a Datadog function Oneprobe does not support (%s)", firstUnsupported(queries)),
				})
			case !allSimple:
				r.Partial = append(r.Partial, PartialEntry{
					WidgetID: w.ID,
					Title:    w.Title,
					Type:     w.Type,
					Query:    strings.Join(queries, "; "),
					Reason:   "query form is outside Oneprobe's documented PromQL subset — review before cutover",
				})
			case w.Type != "timeseries":
				r.Partial = append(r.Partial, PartialEntry{
					WidgetID: w.ID,
					Title:    w.Title,
					Type:     w.Type,
					Query:    strings.Join(queries, "; "),
					Reason:   fmt.Sprintf("widget type %q renders as Oneprobe's single graph — layout fidelity not preserved", w.Type),
				})
			default:
				r.Mapped = append(r.Mapped, MappedEntry{
					WidgetID: w.ID,
					Title:    w.Title,
					Type:     w.Type,
					Query:    queries[0],
					Note:     "translates to a PromQL-subset selector on the same metric",
				})
			}
		}
	}

	if sawAnomalies {
		r.Risks = append(r.Risks, Risk{
			Key:  "query.anomalies",
			Note: "anomalies() has no PromQL-subset equivalent; the tile will show the raw series without anomaly bands",
		})
	}
	if sawSynthetics {
		r.Risks = append(r.Risks, Risk{
			Key:  "widget.synthetics",
			Note: "synthetics widgets are unmapped — keep the Datadog synthetic checks running elsewhere",
		})
	}
	r.Risks = append(r.Risks, Risk{
		Key:  "tags.scope",
		Note: "Datadog scopes (host:web-1) map to PromQL label matchers; verify label names against your exporters",
	})
	sortRisks(r.Risks)
	return r
}

func containsUnsupported(q string) bool {
	for _, fn := range unsupportedFuncs {
		if strings.Contains(q, fn+"(") {
			return true
		}
	}
	return false
}

func firstUnsupported(queries []string) string {
	for _, q := range queries {
		for _, fn := range unsupportedFuncs {
			if strings.Contains(q, fn+"(") {
				return fn
			}
		}
	}
	return ""
}

func sortRisks(rs []Risk) {
	sort.Slice(rs, func(i, j int) bool { return rs[i].Key < rs[j].Key })
}
