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
