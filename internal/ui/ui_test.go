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
		`id="tour"`,
		`id="tour-backdrop"`,
		`dataset.step`,
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

// TestTourWalksEveryWorkflow: the GUI guided tour must have one step
// per workflow, next/skip controls, and it must be reopenable.
func TestTourWalksEveryWorkflow(t *testing.T) {
	page := Page()
	for _, want := range []string{
		`id="tour"`,    // tour container
		`tourSteps`,    // steps array
		`Add a target`, // step 1: targets
		`scrape`,       // step 2: scrapes
		`query`,        // step 3: graph
		`alert`,        // step 4: alerts
		`Datadog`,      // step 5: migrate
		`donate`,       // step 6: donate
		`0x85ee7E71f762d772599cbF1EC20E651B30657521`, // donate step carries the address
		`Next`,      // next button
		`Skip tour`, // skip control
		`tour-open`, // reopen button (header)
		`tourDone`,  // localStorage persistence
	} {
		if !strings.Contains(page, want) {
			t.Errorf("tour missing %q", want)
		}
	}
}

// TestTourIsAWorkingGuide: every step that corresponds to a workflow
// must carry REAL fields and a REAL action — the tour performs the
// actual work, it is not prose.
func TestTourIsAWorkingGuide(t *testing.T) {
	page := Page()
	for _, want := range []string{
		`id="tour-action"`, // action area per step
		`id="tour-result"`, // inline result of the action
		`id="tour-f-name"`, // target fields
		`id="tour-f-url"`,
		`id="tour-f-interval"`,
		`id="tour-act-target"`, // create-target action button
		`id="tour-f-expr"`,     // query field
		`id="tour-act-query"`,  // run-query action button
		`id="tour-f-aname"`,    // alert fields
		`id="tour-f-aexpr"`,
		`id="tour-f-aop"`,
		`id="tour-f-avalue"`,
		`id="tour-act-alert"`,   // create-alert action button
		`id="tour-act-migrate"`, // dry-run action button
		`TOUR_SAMPLE`,           // embedded sample dashboard for migrate
		`tourDoCreateTarget`,    // handlers hit the real API
		`tourDoQuery`,
		`tourDoCreateAlert`,
		`tourDoMigrate`,
		`/api/targets`,
		`/api/alerts`,
		`/api/query?expr=`,
		`/api/migrate/datadog`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("working-guide tour missing %q", want)
		}
	}
}

func TestPageShowsDonateAddresses(t *testing.T) {
	page := Page()
	for _, a := range donate.Addresses() {
		if !strings.Contains(page, a.Address) {
			t.Errorf("UI footer missing address %s", a.Address)
		}
	}
	if !strings.Contains(page, "donate") {
		t.Error("UI must link the donate surface")
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
