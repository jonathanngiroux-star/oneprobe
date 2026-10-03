package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// newTestModel builds a TUI model over a real temp SQLite store, with
// accelerated tick for tests. The guided tour is closed (a first-run
// user who has already seen it) so app-driving tests get the normal
// screens; wizard tests construct their own models or reopen with ?.
func newTestModel(t *testing.T) (Model, *store.Store) {
	t.Helper()
	s := openTUIStore(t)
	m := New(s, WithTickInterval(10*time.Millisecond))
	if m.wizard {
		m = m.update(keyMsg("esc"))
	}
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

// TestBackspaceEditsInput: backspace trims the last rune of the input.
func TestBackspaceEditsInput(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = ScreenQuery
	m = typeString(m, "sum(up")
	m = m.update(keyMsg("backspace"))
	m = m.update(keyMsg("backspace"))
	// Input is now "sum(" — typed text after backspace appends to it.
	m = typeString(m, "up)")
	if m.input != "sum(up)" {
		t.Fatalf("input = %q, want sum(up)", m.input)
	}
	m = m.update(keyMsg("enter"))
	if !containsString(m.View(), "sum(up)") {
		t.Errorf("backspace editing broken; view:\n%s", m.View())
	}
}

// TestEnterOnEmptyInputIsNoop: enter with nothing typed must not error,
// panic, or leave a stale prompt.
func TestEnterOnEmptyInputIsNoop(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = ScreenQuery
	m = m.update(keyMsg("enter"))
	if m.queryOut != "" {
		t.Errorf("empty enter produced output %q", m.queryOut)
	}
	m.screen = ScreenMigrate
	m = m.update(keyMsg("enter"))
	if m.migrateOut != "" {
		t.Errorf("empty enter produced migrate output %q", m.migrateOut)
	}
}

// TestRefreshTickDoesNotClearInput: a data-refresh tick landing between
// keystrokes must not wipe what the user has typed (the tick fires every
// 2s in production — typing is slower than that).
func TestRefreshTickDoesNotClearInput(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = ScreenQuery
	m = typeString(m, "sum(")
	m = m.update(refreshMsg{}) // the tick lands mid-typing
	m = typeString(m, "up)")
	if m.input != "sum(up)" {
		t.Fatalf("input = %q, want sum(up) (tick cleared it)", m.input)
	}
	m = m.update(keyMsg("enter"))
	if !containsString(m.View(), "sum(up)") {
		t.Errorf("evaluation lost after tick; view:\n%s", m.View())
	}
}

// TestScreenSwitchClearsInput: tabbing away from a half-typed input
// abandons it — the next input screen starts empty.
func TestScreenSwitchClearsInput(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = ScreenQuery
	m = typeString(m, "sum(up")
	m = m.update(keyMsg("tab"))
	if m.input != "" {
		t.Errorf("input survived screen switch: %q", m.input)
	}
}

// TestMigrateGarbageJSONNamesPath: a file that exists but is not a
// dashboard export must be refused naming the PATH, not a generic word.
func TestMigrateGarbageJSONNamesPath(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/garbage.json"
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	m, _ := newTestModel(t)
	m.screen = ScreenMigrate
	m = typeString(m, p)
	m = m.update(keyMsg("enter"))
	view := m.View()
	if !containsString(view, p) {
		t.Errorf("garbage-JSON refusal must name the path %q\nview:\n%s", p, view)
	}
	if !containsString(view, "not a valid Datadog dashboard export") {
		t.Errorf("refusal must say what's wrong\nview:\n%s", view)
	}
}

// TestUnicodeAndLongContentRender: target names with wide runes and a
// very long URL must render without panicking (View is format-only).
func TestUnicodeAndLongContentRender(t *testing.T) {
	m, s := newTestModel(t)
	long := "http://127.0.0.1:9100/" + strings.Repeat("verylongpathsegment/", 20) + "metrics"
	if _, err := s.AddTarget(store.Target{
		Name: "日本語ノード", URL: long,
		Interval: scrape.Millis(30 * time.Second), Enabled: true,
	}); err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	m = m.update(refreshMsg{})
	view := m.View() // must not panic
	if !containsString(view, "日本語ノード") {
		t.Errorf("unicode name not rendered\nview:\n%s", view)
	}
}

// TestQueryResultPersistsAcrossScreens: tabbing away and back keeps the
// last evaluation on screen.
func TestQueryResultPersistsAcrossScreens(t *testing.T) {
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
		{TargetID: tg.ID, Name: "persist_total", Ts: now, Value: 5},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	m = m.update(refreshMsg{})
	m.screen = ScreenQuery
	m = typeString(m, "persist_total")
	m = m.update(keyMsg("enter"))
	if !containsString(m.View(), "5") {
		t.Fatalf("setup: no result\n%s", m.View())
	}
	m = m.update(keyMsg("tab"))
	m = m.update(keyMsg("shift+tab"))
	if !containsString(m.View(), "5") {
		t.Errorf("result lost across screen switches\n%s", m.View())
	}
}

// newWizardModel builds a model WITHOUT closing the guided tour — for
// tests that assert the tour's own behavior.
func newWizardModel(t *testing.T) Model {
	t.Helper()
	s := openTUIStore(t)
	return New(s, WithTickInterval(10*time.Millisecond))
}

// TestWizardStartsOnEmptyStore: a fresh db (no targets, no rules)
// auto-opens the guided tour.
func TestWizardStartsOnEmptyStore(t *testing.T) {
	m := newWizardModel(t)
	if !m.wizard {
		t.Fatal("wizard must auto-start on an empty store")
	}
	view := m.View()
	if !containsString(view, "getting started") {
		t.Errorf("wizard view missing title\n%s", view)
	}
}

// TestWizardDoesNotStartOnNonEmptyStore: a db with a target skips the
// auto-tour (? still opens it).
func TestWizardDoesNotStartOnNonEmptyStore(t *testing.T) {
	_, s := newTestModel(t)
	if _, err := s.AddTarget(store.Target{
		Name: "x", URL: "http://127.0.0.1:9999/metrics",
		Interval: scrape.Millis(time.Minute), Enabled: true,
	}); err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	m2 := New(s, WithTickInterval(10*time.Millisecond))
	if m2.wizard {
		t.Error("wizard must not auto-start when data exists")
	}
	// ? opens it manually.
	m2 = m2.update(keyMsg("?"))
	if !m2.wizard {
		t.Error("? must open the tour")
	}
}

// TestWizardEnterAdvancesAndFinishes: enter steps through every page;
// the last enter closes the tour on the targets screen.
func TestWizardEnterAdvancesAndFinishes(t *testing.T) {
	m := newWizardModel(t)
	steps := 0
	for i := 0; i < 40 && m.wizard; i++ {
		view := m.View()
		if !containsString(view, "step") {
			t.Fatalf("wizard page %d missing step counter\n%s", i, view)
		}
		m = m.update(keyMsg("enter"))
		steps++
	}
	if m.wizard {
		t.Fatalf("wizard never finished after %d enters", steps)
	}
	if m.screen != ScreenTargets {
		t.Errorf("finished tour must land on targets, got %v", m.screen)
	}
	if steps < 6 {
		t.Errorf("only %d steps - too few to cover every screen", steps)
	}
}

// TestWizardEscSkips: esc closes the tour immediately.
func TestWizardEscSkips(t *testing.T) {
	m := newWizardModel(t)
	m = m.update(keyMsg("esc"))
	if m.wizard {
		t.Error("esc must skip the tour")
	}
}

// TestWizardShowsEveryWorkflow: the tour's pages mention each workflow
// the software has.
func TestWizardShowsEveryWorkflow(t *testing.T) {
	m := newWizardModel(t)
	seen := ""
	for i := 0; i < 40 && m.wizard; i++ {
		seen += m.View()
		m = m.update(keyMsg("enter"))
	}
	for _, want := range []string{"target", "scrape", "quer", "alert", "migrate", "donate"} {
		if !containsString(seen, want) {
			t.Errorf("tour never mentions %q", want)
		}
	}
	// Donate page must show both addresses byte-for-byte.
	m = newWizardModel(t)
	donateView := ""
	for i := 0; i < 40 && m.wizard; i++ {
		if containsString(m.View(), "donate") {
			donateView = m.View()
		}
		m = m.update(keyMsg("enter"))
	}
	if donateView == "" || !containsString(donateView, "0x85ee7E71f762d772599cbF1EC20E651B30657521") {
		t.Errorf("tour donate page must carry the ETH address")
	}
}

// TestWizardQuestionMarkTogglesFromAnyScreen.
func TestWizardQuestionMarkTogglesFromAnyScreen(t *testing.T) {
	m, s := newTestModel(t)
	if _, err := s.AddTarget(store.Target{
		Name: "x", URL: "http://127.0.0.1:9999/metrics",
		Interval: scrape.Millis(time.Minute), Enabled: true,
	}); err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	m = New(s, WithTickInterval(10*time.Millisecond))
	m.screen = ScreenQuery
	m = m.update(keyMsg("?"))
	if !m.wizard {
		t.Fatal("? must open the tour from any screen")
	}
	m = m.update(keyMsg("?"))
	if m.wizard {
		t.Error("second ? must close the tour")
	}
}

// TestWizardIsAWorkingGuide: the tour steps that map to workflows must
// offer a real action — the user completes the workflow from the tour.
func TestWizardIsAWorkingGuide(t *testing.T) {
	s := openTUIStore(t)
	m := New(s, WithTickInterval(10*time.Millisecond))
	// Step through to the query step and run a REAL query from the tour.
	stepsSeen := []string{}
	for i := 0; i < 20 && m.wizard; i++ {
		view := m.View()
		stepsSeen = append(stepsSeen, view)
		m = m.update(keyMsg("enter"))
	}
	// The query step must offer an action ("t" to try it), and running
	// it must evaluate the store for real.
	found := false
	for _, v := range stepsSeen {
		if containsString(v, "Press t to run sum(up)") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("query step never offers a live action (press t)")
	}

	// Reopen the tour, walk to the query step, run the live action:
	// it must evaluate the REAL store (sum(tour_total) exists below).
	if _, err := s.AddTarget(store.Target{
		Name: "seed", URL: "http://127.0.0.1:9999/metrics",
		Interval: scrape.Millis(time.Minute), Enabled: false,
	}); err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	targets, err := s.ListTargets()
	if err != nil || len(targets) == 0 {
		t.Fatalf("ListTargets: %v (%d)", err, len(targets))
	}
	tg := targets[len(targets)-1]
	now := time.Now().UTC().UnixMilli()
	if err := s.InsertSamples([]store.Sample{
		{TargetID: tg.ID, Name: "up", Ts: now, Value: 42},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	m = New(s, WithTickInterval(10*time.Millisecond))
	if m.wizard {
		m = m.update(keyMsg("esc"))
	}
	m = m.update(keyMsg("?"))
	// walk to the query step (step 4 of the tour -> index 3)
	for i := 0; i < 3; i++ {
		m = m.update(keyMsg("enter"))
	}
	if !containsString(m.View(), "Press t to run sum(up)") {
		t.Fatalf("walked to wrong step:\n%s", m.View())
	}
	m = m.update(keyMsg("t"))
	view := m.View()
	if !containsString(view, "sum(up) -> 1 series, current value 42") {
		t.Errorf("live query did not run against the real store\n%s", view)
	}
}

func TestKeysHelpShowsTourHint(t *testing.T) {
	m, _ := newTestModel(t)
	for _, sc := range []Screen{ScreenTargets, ScreenScrapes, ScreenQuery, ScreenAlerts, ScreenMigrate, ScreenDonate} {
		m.screen = sc
		if !containsString(m.View(), "? tour") {
			t.Errorf("screen %v footer missing '? tour' hint", sc)
		}
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
