package ui

import (
	"strings"
	"testing"

	"github.com/jonathanngiroux-star/oneprobe/internal/donate"
)

func TestPageContainsThePanels(t *testing.T) {
	page := Page()
	for _, want := range []string{
		`id="targets"`,
		`id="scrapes"`,
		`id="graph"`,
		`id="alerts"`,
		`id="migrate-heading"`,
		`id="alert-history"`,
		`id="retention-note"`,
		`id="wizard"`,
		`id="wizard-backdrop"`,
		`wizardRenderSteps(`,
		`localStorage`,
		`fetch("/api/targets")`,
		`fetch("/api/scrapes")`,
		`fetch("/api/alerts")`,
		`fetch("/api/migrate/datadog"`,
		`/api/query?expr=`,
		`/api/alerts/" + ruleId + "/history`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("UI page missing %q", want)
		}
	}
}

// TestWizardWalksEveryWorkflow: the setup wizard has one step per
// workflow, next/skip controls, and a reopenable activation button.
func TestWizardWalksEveryWorkflow(t *testing.T) {
	page := Page()
	for _, want := range []string{
		`id="wizard"`,
		`wizardSteps`,
		`Add a target`,
		`scrape`,
		`query`,
		`alert`,
		`Datadog`,
		`donate`,
		`0x85ee7E71f762d772599cbF1EC20E651B30657521`,
		`Next`,
		`Skip wizard`,
		`wizard-open`,
		`wizardDone`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("wizard missing %q", want)
		}
	}
}

// TestWizardIsAWorkingGuide: every step that corresponds to a workflow
// must carry REAL fields and a REAL action.
func TestWizardIsAWorkingGuide(t *testing.T) {
	page := Page()
	for _, want := range []string{
		`id="wizard-action"`,
		`id="wizard-result"`,
		`id="wizard-f-name"`,
		`id="wizard-f-url"`,
		`id="wizard-f-interval"`,
		`id="wizard-act-target"`,
		`id="wizard-f-expr"`,
		`id="wizard-act-query"`,
		`id="wizard-f-aname"`,
		`id="wizard-f-aexpr"`,
		`id="wizard-f-aop"`,
		`id="wizard-f-avalue"`,
		`id="wizard-act-alert"`,
		`id="wizard-act-migrate"`,
		`WIZARD_SAMPLE`,
		`wizardDoCreateTarget`,
		`wizardDoQuery`,
		`wizardDoCreateAlert`,
		`wizardDoMigrate`,
		`/api/targets`,
		`/api/alerts`,
		`/api/query?expr=`,
		`/api/migrate/datadog`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("working-guide wizard missing %q", want)
		}
	}
}

// TestPageHasSectionMenu: the window is split by a menu; every section
// is a data-section panel and switchSection drives which one shows.
func TestPageHasSectionMenu(t *testing.T) {
	page := Page()
	for _, want := range []string{
		`<nav id="menu"`,
		`data-section="targets"`,
		`data-section="scrapes"`,
		`data-section="graph"`,
		`data-section="alerts"`,
		`data-section="migrate"`,
		`data-section="backup"`,
		`data-section="donate"`,
		`id="section-targets"`,
		`id="section-donate"`,
		`switchSection(`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("section menu missing %q", want)
		}
	}
}

// TestWizardActivationNotAutoOpen: the wizard is user-activated (menu
// button + first-run hint), never auto-opened on load.
func TestWizardActivationNotAutoOpen(t *testing.T) {
	page := Page()
	for _, want := range []string{
		`id="wizard-open"`,
		`Run setup wizard`,
		`id="first-run-hint"`,
		`hint-wizard-open`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("wizard activation missing %q", want)
		}
	}
}

// TestWizardDarkMode: explicit dark palette; the wizard card paints
// itself with palette variables, never hardcoded white.
func TestWizardDarkMode(t *testing.T) {
	page := Page()
	if !strings.Contains(page, "prefers-color-scheme: dark") {
		t.Error("page must define a dark palette via prefers-color-scheme")
	}
	if !strings.Contains(page, "--panel") {
		t.Error("page must define a --panel CSS variable")
	}
	if strings.Contains(page, "#fff)") {
		t.Error("wizard card must not fall back to hardcoded white")
	}
	if !strings.Contains(page, "background: var(--panel)") {
		t.Error("wizard card must paint itself with the palette variable")
	}
}

// TestWizardHasStepMenu: the wizard window is split by a step menu
// with per-step completion state.
func TestWizardHasStepMenu(t *testing.T) {
	page := Page()
	for _, want := range []string{
		`id="wizard-steps"`,
		`wizardRenderSteps(`,
		`wizardSucceeded`,
		`data-step=`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("wizard step menu missing %q", want)
		}
	}
}

func TestPageShowsDonateAddresses(t *testing.T) {
	page := Page()
	// The donate section carries both addresses, each with its copy button.
	secStart := strings.Index(page, `id="section-donate"`)
	if secStart == -1 {
		t.Fatal("page has no donate section")
	}
	section := page[secStart:]
	sectionEnd := strings.Index(section, `</div>
<footer>`)
	if sectionEnd != -1 {
		section = section[:sectionEnd]
	}
	for _, a := range donate.Addresses() {
		if !strings.Contains(section, a.Address) {
			t.Errorf("donate section missing address %s", a.Address)
		}
		id := "footer-" + addrSlug(a.Chain)
		if !strings.Contains(section, "id=\""+id+"\"") {
			t.Errorf("donate section missing element id %q", id)
		}
	}
	// The first page (targets) must not show the raw addresses: the
	// footer keeps only a link to the donate section.
	fi := strings.LastIndex(page, "<footer>")
	if fi == -1 {
		t.Fatal("no footer")
	}
	footer := page[fi : strings.Index(page[fi:], "</footer>")+fi]
	for _, a := range donate.Addresses() {
		if strings.Contains(footer, a.Address) {
			t.Errorf("footer must not carry raw address %s - donate has its own section", a.Address)
		}
	}
	if !strings.Contains(footer, "donate") {
		t.Error("footer must still link the donate surface")
	}
}

// TestFooterHasCopyButtons: every address in the main GUI footer must
// have a working copy button (clipboard + fallback) so users can
// copy/paste the crypto addresses without manual selection.
func TestFooterHasCopyButtons(t *testing.T) {
	page := Page()
	for _, a := range donate.Addresses() {
		id := "footer-" + addrSlug(a.Chain)
		if !strings.Contains(page, "id=\""+id+"\"") {
			t.Errorf("footer address %s missing element id %q", a.Chain, id)
		}
		if !strings.Contains(page, "data-target=\""+id+"\"") {
			t.Errorf("footer address %s missing copy button targeting %q", a.Chain, id)
		}
	}
	if !strings.Contains(page, "class=\"copy\"") {
		t.Error("footer copy buttons must use class copy")
	}
	if !strings.Contains(page, "navigator.clipboard") {
		t.Error("copy buttons must use the clipboard API with fallback")
	}
}

func TestPageIsCompleteHTML(t *testing.T) {
	page := Page()
	if !strings.HasPrefix(page, "<!DOCTYPE html>") {
		t.Error("page must start with a doctype")
	}
	if !strings.HasSuffix(strings.TrimSpace(page), "</html>") {
		t.Error("page must end with </html>")
	}
}
