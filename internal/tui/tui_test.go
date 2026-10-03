package tui

import (
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// newTestModel builds a TUI model over a real temp SQLite store, with
// accelerated tick for tests.
func newTestModel(t *testing.T) (Model, *store.Store) {
	t.Helper()
	s := openTUIStore(t)
	m := New(s, WithTickInterval(10*time.Millisecond))
	return m, s
}

func openTUIStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir() + "/tui.db")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestTabCyclesThroughScreens: the keyboard path between screens.
func TestTabCyclesThroughScreens(t *testing.T) {
	m, _ := newTestModel(t)
	want := []Screen{ScreenScrapes, ScreenQuery, ScreenAlerts, ScreenMigrate, ScreenDonate, ScreenTargets}
	for i, w := range want {
		m = m.update(keyMsg("tab"))
		if m.screen != w {
			t.Fatalf("tab %d: screen = %v, want %v", i+1, m.screen, w)
		}
	}
	// shift+tab walks backward.
	m = m.update(keyMsg("shift+tab"))
	if m.screen != ScreenDonate {
		t.Errorf("shift+tab from targets: screen = %v, want donate", m.screen)
	}
}

// TestTargetScreenListsTargets: the model loads targets from the store
// and renders rows with name, url, interval, enabled, last error.
func TestTargetScreenListsTargets(t *testing.T) {
	m, s := newTestModel(t)
	if _, err := s.AddTarget(store.Target{
		Name: "web-1", URL: "http://127.0.0.1:9100/metrics",
		Interval: scrape.Millis(15 * time.Second), Enabled: true,
	}); err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	m = m.update(refreshMsg{})

	view := m.View()
	for _, want := range []string{"web-1", "http://127.0.0.1:9100/metrics", "15s", "yes"} {
		if !containsString(view, want) {
			t.Errorf("targets view missing %q\nview:\n%s", want, view)
		}
	}
}

// TestQueryScreenEvaluatesDocumentedSubset: typing an expr at the query
// prompt and pressing enter renders the evaluated result from the SAME
// engine the GUI uses.
func TestQueryScreenEvaluatesDocumentedSubset(t *testing.T) {
	m, s := newTestModel(t)
	tg, err := s.AddTarget(store.Target{
		Name: "seed", URL: "http://127.0.0.1:9999/metrics",
		Interval: scrape.Millis(time.Minute), Enabled: false,
	})
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	now := time.Now().UTC().UnixMilli()
	if err := s.InsertSamples([]store.Sample{
		{TargetID: tg.ID, Name: "audit_total", Labels: `{h="a"}`, Ts: now, Value: 3},
		{TargetID: tg.ID, Name: "audit_total", Labels: `{h="b"}`, Ts: now, Value: 4},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	m = m.update(refreshMsg{})
	m.screen = ScreenQuery
	m = typeString(m, "sum(audit_total)")
	m = m.update(keyMsg("enter"))

	view := m.View()
	if !containsString(view, "7") {
		t.Errorf("query result missing sum=7\nview:\n%s", view)
	}
}

// TestQueryScreenRejectsUndocumentedFunction: an expr outside the
// subset shows the parser's named refusal, not a crash.
func TestQueryScreenRejectsUndocumentedFunction(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = ScreenQuery
	m = typeString(m, "histogram_quantile(0.9, x)")
	m = m.update(keyMsg("enter"))
	view := m.View()
	if !containsString(view, `unsupported function "histogram_quantile"`) {
		t.Errorf("refusal must name the function\nview:\n%s", view)
	}
}

// TestAlertScreenListsRulesAndState.
func TestAlertScreenListsRulesAndState(t *testing.T) {
	m, s := newTestModel(t)
	if _, err := s.AddAlertRule(store.AlertRule{
		Name: "host-down", Expr: "up", Op: "<", Value: 1, Enabled: true,
	}); err != nil {
		t.Fatalf("AddAlertRule: %v", err)
	}
	if err := s.SetAlertState(1, "firing", 0, time.Unix(1759305600, 0).UTC()); err != nil {
		t.Fatalf("SetAlertState: %v", err)
	}
	m = m.update(refreshMsg{})
	m.screen = ScreenAlerts
	view := m.View()
	for _, want := range []string{"host-down", "up < 1", "firing"} {
		if !containsString(view, want) {
			t.Errorf("alerts view missing %q\nview:\n%s", want, view)
		}
	}
}

// TestMigrateScreenRunsDryRunFromPath: entering a fixture path shows the
// mapped/partial/unmapped/risks summary.
func TestMigrateScreenRunsDryRunFromPath(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = ScreenMigrate
	m = typeString(m, "testdata/fixtures/datadog/sample-dashboard.json")
	m = m.update(keyMsg("enter"))
	view := m.View()
	for _, want := range []string{"mapped 1", "partial 2", "unmapped 1", "Web Overview"} {
		if !containsString(view, want) {
			t.Errorf("migrate view missing %q\nview:\n%s", want, view)
		}
	}
}

// TestMigrateScreenBadPathNamesIt.
func TestMigrateScreenBadPathNamesIt(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = ScreenMigrate
	m = typeString(m, "/no/such/file.json")
	m = m.update(keyMsg("enter"))
	view := m.View()
	if !containsString(view, "/no/such/file.json") {
		t.Errorf("refusal must name the path\nview:\n%s", view)
	}
}

// TestDonateScreenShowsBothAddresses: the donate screen renders both
// addresses byte-for-byte plus the ask.
func TestDonateScreenShowsBothAddresses(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = ScreenDonate
	view := m.View()
	for _, want := range []string{
		"0x85ee7E71f762d772599cbF1EC20E651B30657521",
		"bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg",
		"Oneprobe is free to self-host",
	} {
		if !containsString(view, want) {
			t.Errorf("donate view missing %q\nview:\n%s", want, view)
		}
	}
}

// TestScrapeScreenShowsStatus: the scrapes screen lists per-target last
// scrape outcome with ok/failure text.
func TestScrapeScreenShowsStatus(t *testing.T) {
	m, s := newTestModel(t)
	tg, err := s.AddTarget(store.Target{
		Name: "ok-target", URL: "http://127.0.0.1:9999/metrics",
		Interval: scrape.Millis(time.Minute), Enabled: true,
	})
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	fail := "connection refused"
	_ = s.RecordScrape(tg.ID, time.Unix(1759305600, 0).UTC(), 12, 3, nil)
	_ = s.RecordScrape(tg.ID, time.Unix(1759305615, 0).UTC(), 0, 0, &fail)

	m = m.update(refreshMsg{})
	m.screen = ScreenScrapes
	view := m.View()
	if !containsString(view, "ok-target") || !containsString(view, "connection refused") {
		t.Errorf("scrapes view missing target or error\nview:\n%s", view)
	}
}

// TestKeysHelpShownOnEveryScreen.
func TestKeysHelpShownOnEveryScreen(t *testing.T) {
	m, _ := newTestModel(t)
	for _, sc := range []Screen{ScreenTargets, ScreenScrapes, ScreenQuery, ScreenAlerts, ScreenMigrate, ScreenDonate} {
		m.screen = sc
		view := m.View()
		if !containsString(view, "tab") {
			t.Errorf("screen %v: key help missing", sc)
		}
	}
}

// TestQuitKeys: ctrl+c quits everywhere; q quits outside input screens
// and TYPES on query/migrate (typing histogram_quantile must not exit).
func TestQuitKeys(t *testing.T) {
	m, _ := newTestModel(t)
	m = m.update(keyMsg("ctrl+c"))
	if !m.quitting {
		t.Error("ctrl+c must quit")
	}

	m2, _ := newTestModel(t)
	m2.screen = ScreenTargets
	m2 = m2.update(keyMsg("q"))
	if !m2.quitting {
		t.Error("q must quit on non-input screens")
	}

	m3, _ := newTestModel(t)
	m3.screen = ScreenQuery
	m3 = m3.update(keyMsg("q"))
	if m3.quitting {
		t.Error("q on the query screen must type, not quit")
	}
	if m3.input != "q" {
		t.Errorf("input = %q, want q", m3.input)
	}
}

// helpers ------------------------------------------------------------

// typeString feeds each rune as a keyMsg, then returns the model.
func typeString(m Model, s string) Model {
	for _, r := range s {
		m = m.update(keyMsg(string(r)))
	}
	return m
}

func containsString(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
