// Package tui is Oneprobe's terminal interface: a Bubble Tea model
// over the SAME store the GUI and CLI use — no second data model.
// The Model is a plain struct with an update/render split so tests
// drive it in-process; the tea.Model adapter is a thin wrapper.
//
// Keys (documented on every screen):
//
//	tab / shift+tab  next / previous screen
//	enter            submit (query expr, migrate path)
//	q / ctrl+c       quit
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/donate"
	"github.com/jonathanngiroux-star/oneprobe/internal/migrate/datadog"
	"github.com/jonathanngiroux-star/oneprobe/internal/query"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// Screen is one TUI view.
type Screen int

const (
	ScreenTargets Screen = iota
	ScreenScrapes
	ScreenQuery
	ScreenAlerts
	ScreenMigrate
	ScreenDonate
	screenCount
)

var screenNames = map[Screen]string{
	ScreenTargets: "targets",
	ScreenScrapes: "scrapes",
	ScreenQuery:   "query",
	ScreenAlerts:  "alerts",
	ScreenMigrate: "migrate",
	ScreenDonate:  "donate",
}

// Model is the whole TUI state. Tests call update/View directly; the
// tea.Model adapter forwards to them.
type Model struct {
	store *store.Store

	screen   Screen
	quitting bool

	tickEvery time.Duration

	// inputs
	input string // current one-line input (query expr / migrate path)

	// results
	targets    []store.Target
	scrapeRows []scrapeRow
	alerts     []alertRow
	queryOut   string // rendered query result or error
	migrateOut string // rendered migrate summary or error

	// last scrape error per target (targets screen column)
	lastErr map[int64]string
}

type scrapeRow struct {
	name     string
	at       string
	duration int64
	samples  int
	ok       bool
	err      string
}

type alertRow struct {
	rule  store.AlertRule
	state string
	value float64
	since string
}

// Option configures New.
type Option func(*Model)

// WithTickInterval sets the data-refresh cadence (tests accelerate it).
func WithTickInterval(d time.Duration) Option {
	return func(m *Model) { m.tickEvery = d }
}

// New builds the TUI model over the shared store.
func New(s *store.Store, opts ...Option) Model {
	m := Model{
		store:     s,
		tickEvery: 2 * time.Second,
		lastErr:   map[int64]string{},
	}
	for _, o := range opts {
		o(&m)
	}
	m.refresh()
	return m
}

// refresh reloads everything from the store (same store as the GUI).
func (m *Model) refresh() {
	if m.store == nil {
		return
	}
	targets, err := m.store.ListTargets()
	if err == nil {
		m.targets = targets
	}
	rows := []scrapeRow{}
	alerts := []alertRow{}
	for _, tg := range m.targets {
		scrapes, err := m.store.ListScrapes(tg.ID, 1)
		if err == nil && len(scrapes) > 0 {
			s := scrapes[0]
			row := scrapeRow{
				name:     tg.Name,
				at:       s.At.Format(time.RFC3339),
				duration: s.DurationMs,
				samples:  s.SampleCount,
				ok:       s.Ok,
				err:      s.Error,
			}
			rows = append(rows, row)
			if s.Ok {
				m.lastErr[tg.ID] = ""
			} else {
				m.lastErr[tg.ID] = s.Error
			}
		} else {
			rows = append(rows, scrapeRow{name: tg.Name})
		}
	}
	m.scrapeRows = rows

	rules, err := m.store.ListAlertRules()
	if err == nil {
		for _, r := range rules {
			st, err := m.store.GetAlertState(r.ID)
			if err != nil {
				continue
			}
			since := "-"
			if !st.ChangedAt.IsZero() {
				since = st.ChangedAt.Format(time.RFC3339)
			}
			alerts = append(alerts, alertRow{rule: r, state: st.State, value: st.LastValue, since: since})
		}
		m.alerts = alerts
	}
}

// refreshMsg asks the model to reload store data.
type refreshMsg struct{}

// keyMsg is a normalized key/rune event (tests construct these; the
// adapter builds them from tea.KeyMsg).
type keyMsg string

// update handles one event, returning the next model. Pure enough for
// in-process tests: no terminal, no goroutines.
func (m Model) update(msg interface{}) Model {
	switch msg := msg.(type) {
	case refreshMsg:
		m.refresh()
		return m

	case keyMsg:
		switch string(msg) {
		case "ctrl+c":
			m.quitting = true
			return m
		case "q":
			// q quits only outside a text-input screen — typing
			// "histogram_quantile" must not exit the TUI. Input
			// screens quit via ctrl+c.
			if m.screen != ScreenQuery && m.screen != ScreenMigrate {
				m.quitting = true
			} else {
				m.input += "q"
			}
			return m
		case "tab":
			m.screen = (m.screen + 1) % screenCount
			m.input = ""
			return m
		case "shift+tab":
			m.screen = (m.screen + screenCount - 1) % screenCount
			m.input = ""
			return m
		case "enter":
			switch m.screen {
			case ScreenQuery:
				m.runQuery()
			case ScreenMigrate:
				m.runMigrate()
			}
			m.input = ""
			return m
		case "backspace":
			if len(m.input) > 0 {
				r := []rune(m.input)
				m.input = string(r[:len(r)-1])
			}
			return m
		default:
			// Printable input into the query/migrate prompt. A keyMsg may
			// carry multiple runes (terminal coalescing, paste) — append
			// the whole string.
			if m.screen == ScreenQuery || m.screen == ScreenMigrate {
				if len(msg) > 0 {
					m.input += string(msg)
				}
			}
			return m
		}
	}
	return m
}

// runQuery evaluates the input through the SAME engine as the GUI.
func (m *Model) runQuery() {
	expr := strings.TrimSpace(m.input)
	if expr == "" {
		return
	}
	node, err := query.Parse(expr)
	if err != nil {
		m.queryOut = "error: " + err.Error()
		return
	}
	res, err := query.Eval(node, m.store, time.Now().UTC(), 5*time.Minute)
	if err != nil {
		m.queryOut = "error: " + err.Error()
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s -> %d series\n", expr, len(res.Series))
	for i, sr := range res.Series {
		if i >= 10 {
			fmt.Fprintf(&b, "  ... (%d more)\n", len(res.Series)-10)
			break
		}
		last := 0.0
		if len(sr.Points) > 0 {
			last = sr.Points[len(sr.Points)-1].Value
		}
		fmt.Fprintf(&b, "  %s%s = %v (%d points)\n", sr.Name, sr.Labels, last, len(sr.Points))
	}
	m.queryOut = b.String()
}

// runMigrate dry-runs the entered dashboard-export path — the SAME
// classifier as the CLI and the GUI endpoint.
func (m *Model) runMigrate() {
	path := strings.TrimSpace(m.input)
	if path == "" {
		return
	}
	raw, err := readDashboardFile(path)
	if err != nil {
		m.migrateOut = "error: " + err.Error()
		return
	}
	dash, err := decodeDashboard(raw, path)
	if err != nil {
		m.migrateOut = "error: " + err.Error()
		return
	}
	report := datadog.Diff(dash)
	var b strings.Builder
	fmt.Fprintf(&b, "%s: mapped %d, partial %d, unmapped %d, risks %d\n",
		report.Input.Title, len(report.Mapped), len(report.Partial), len(report.Unmapped), len(report.Risks))
	for _, w := range report.Mapped {
		fmt.Fprintf(&b, "  mapped   %s\n", w.Title)
	}
	for _, w := range report.Partial {
		fmt.Fprintf(&b, "  partial  %s (%s)\n", w.Title, w.Reason)
	}
	for _, w := range report.Unmapped {
		fmt.Fprintf(&b, "  unmapped %s (%s)\n", w.Title, w.Reason)
	}
	for _, r := range report.Risks {
		fmt.Fprintf(&b, "  risk     %s: %s\n", r.Key, r.Note)
	}
	m.migrateOut = b.String()
}

// View renders the current screen. Pure.
func (m Model) View() string {
	if m.quitting {
		return "bye\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Oneprobe — %s\n\n", screenNames[m.screen])

	switch m.screen {
	case ScreenTargets:
		b.WriteString("targets:\n")
		if len(m.targets) == 0 {
			b.WriteString("  (none — add one via the GUI or API)\n")
		}
		for _, tg := range m.targets {
			err := m.lastErr[tg.ID]
			errCol := "-"
			if err != "" {
				errCol = err
			}
			fmt.Fprintf(&b, "  %-16s %-40s %6s  %-3s  %s\n",
				tg.Name, tg.URL, tg.Interval.Duration().String(), boolYes(tg.Enabled), errCol)
		}
	case ScreenScrapes:
		b.WriteString("recent scrapes:\n")
		if len(m.scrapeRows) == 0 {
			b.WriteString("  (no targets yet)\n")
		}
		for _, r := range m.scrapeRows {
			if r.at == "" {
				fmt.Fprintf(&b, "  %-16s not scraped yet\n", r.name)
				continue
			}
			status := "ok"
			if !r.ok {
				status = "FAIL: " + r.err
			}
			fmt.Fprintf(&b, "  %-16s %s  %3dms  %3d samples  %s\n",
				r.name, r.at, r.duration, r.samples, status)
		}
	case ScreenQuery:
		fmt.Fprintf(&b, "query (documented subset — see docs/query.md):\n  > %s\n\n", m.input)
		if m.queryOut != "" {
			b.WriteString(m.queryOut)
		} else {
			b.WriteString("  (type an expression and press enter)\n")
		}
	case ScreenAlerts:
		b.WriteString("alert rules:\n")
		if len(m.alerts) == 0 {
			b.WriteString("  (none — add one via the GUI or API)\n")
		}
		for _, a := range m.alerts {
			cond := fmt.Sprintf("%s %s %v", a.rule.Expr, a.rule.Op, a.rule.Value)
			if a.rule.ForSecs > 0 {
				cond += fmt.Sprintf(" for %ds", a.rule.ForSecs)
			}
			fmt.Fprintf(&b, "  %-16s %-40s %-8s value=%v since %s\n",
				a.rule.Name, cond, a.state, a.value, a.since)
		}
	case ScreenMigrate:
		fmt.Fprintf(&b, "datadog migrate dry-run (enter a dashboard export file path):\n  > %s\n\n", m.input)
		if m.migrateOut != "" {
			b.WriteString(m.migrateOut)
		} else {
			b.WriteString("  (enter a path and press enter)\n")
		}
	case ScreenDonate:
		b.WriteString(donate.PlainText())
		b.WriteString("\nNo paid tier. No subscriptions. Nothing is gated on a donation.\n")
	}

	b.WriteString("\n" + strings.Repeat("─", 60) + "\n")
	if m.screen == ScreenQuery || m.screen == ScreenMigrate {
		b.WriteString("tab next screen · shift+tab back · enter submit · ctrl+c quit\n")
	} else {
		b.WriteString("tab next screen · shift+tab back · q quit\n")
	}
	return b.String()
}

func boolYes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// readDashboardFile loads a dashboard export, walking ancestor
// directories for repo-relative paths (same behavior as the CLI).
func readDashboardFile(path string) ([]byte, error) {
	if raw, err := os.ReadFile(path); err == nil {
		return raw, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", path, err)
	}
	for {
		if raw, err := os.ReadFile(dir + "/" + path); err == nil {
			return raw, nil
		}
		parent := filepathDir(dir)
		if parent == dir {
			return nil, fmt.Errorf("cannot read %q: not found in this or any parent directory", path)
		}
		dir = parent
	}
}

// decodeDashboard parses the export into the migrate package's shape.
// The refusal names the PATH — a generic "input" tells the operator
// nothing about which file was bad.
func decodeDashboard(raw []byte, path string) (datadog.Dashboard, error) {
	var d datadog.Dashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return d, fmt.Errorf("%s is not a valid Datadog dashboard export: %w", path, err)
	}
	return d, nil
}

func filepathDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			if i == 0 {
				return "/"
			}
			return p[:i]
		}
	}
	return "."
}
