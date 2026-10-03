package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionPrintsBinaryNameAndVersionExitsZero(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"version"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %q)", code, errb.String())
	}
	if !strings.HasPrefix(out.String(), "oneprobe ") {
		t.Errorf("version output %q, want prefix \"oneprobe \"", out.String())
	}
}

func TestDonatePrintsBothAddressesExactlyAndExitsZero(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"donate"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %q)", code, errb.String())
	}
	for _, want := range []string{
		"0x85ee7E71f762d772599cbF1EC20E651B30657521",
		"bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg",
		"Oneprobe is free to self-host.",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("donate output missing %q\ngot:\n%s", want, out.String())
		}
	}
}

func TestTUICommandOpensStoreAndRefusesBadDB(t *testing.T) {
	// --db pointing at an unopenable path must exit nonzero naming it.
	var out, errb bytes.Buffer
	code := run([]string{"tui", "--db", "/nonexistent-dir/oneprobe.db"}, &out, &errb)
	if code == 0 {
		t.Fatalf("tui with a bad --db must not succeed (stderr: %q)", errb.String())
	}
	if !strings.Contains(errb.String(), "/nonexistent-dir/oneprobe.db") {
		t.Errorf("refusal must name the db path, got: %q", errb.String())
	}
	// No --db: falls back to ONEPROBE_DB.
	t.Setenv("ONEPROBE_DB", filepath.Join(t.TempDir(), "env.db"))
	var out2, errb2 bytes.Buffer
	// tui would block on a terminal; we only test arg resolution here by
	// giving it a bad value shape: nonexistent dir again but via env.
	t.Setenv("ONEPROBE_DB", "/nonexistent-dir2/x.db")
	code = run([]string{"tui"}, &out2, &errb2)
	if code == 0 {
		t.Error("tui with bad ONEPROBE_DB must fail")
	}
	if !strings.Contains(errb2.String(), "/nonexistent-dir2/x.db") {
		t.Errorf("refusal must name the env db path, got: %q", errb2.String())
	}
}

func TestUnknownCommandExitsTwoAndNamesIt(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"frobnicate"}, &out, &errb)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "unknown command \"frobnicate\"") {
		t.Errorf("stderr must name the rejected command, got %q", errb.String())
	}
}

func TestNoArgsPrintsUsageExitsTwo(t *testing.T) {
	var out, errb bytes.Buffer
	code := run(nil, &out, &errb)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	for _, want := range []string{"oneprobe", "serve", "migrate", "donate", "version"} {
		if !strings.Contains(errb.String(), want) {
			t.Errorf("usage missing %q\ngot:\n%s", want, errb.String())
		}
	}
}

func TestMigrateDryRunEmitsStableJSONContract(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{
		"migrate", "--from=datadog", "--dry-run", "--format=json",
		"--input", filepath.Join("testdata", "fixtures", "datadog", "sample-dashboard.json"),
	}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %q)", code, errb.String())
	}
	var report struct {
		Source   string `json:"source"`
		DryRun   bool   `json:"dry_run"`
		Mapped   []any  `json:"mapped"`
		Partial  []any  `json:"partial"`
		Unmapped []any  `json:"unmapped"`
		Risks    []any  `json:"risks"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if report.Source != "datadog" || !report.DryRun {
		t.Errorf("source/dry_run wrong: %q %v", report.Source, report.DryRun)
	}
	if len(report.Mapped) != 1 || len(report.Partial) != 2 || len(report.Unmapped) != 1 {
		t.Errorf("bucket sizes on fixture: mapped=%d partial=%d unmapped=%d, want 1/2/1",
			len(report.Mapped), len(report.Partial), len(report.Unmapped))
	}
	if len(report.Risks) == 0 {
		t.Error("risks must be non-empty on the sample fixture")
	}
}

func TestMigrateRejectsUnknownFromNamingIt(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"migrate", "--from=grafana", "--dry-run"}, &out, &errb)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errb.String(), `--from=grafana`) {
		t.Errorf("refusal must name the rejected --from value, got: %q", errb.String())
	}
}

func TestServeWiresHTTPServer(t *testing.T) {
	// serve is exercised end-to-end in the serve package; here we only
	// assert the CLI accepts the documented flags and refuses bad ones.
	var out, errb bytes.Buffer
	code := run([]string{"serve", "--http=127.0.0.1:0", "--db", filepath.Join(t.TempDir(), "oneprobe.db")}, &out, &errb)
	if code != 0 {
		t.Fatalf("serve with --http=127.0.0.1:0 should bind and exit cleanly, exit=%d stderr=%q", code, errb.String())
	}
}

func TestServeRejectsBadAddr(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"serve", "--http=not-an-addr", "--db", filepath.Join(t.TempDir(), "oneprobe.db")}, &out, &errb)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 for an unbindable address", code)
	}
	if !strings.Contains(errb.String(), "not-an-addr") {
		t.Errorf("refusal must name the bad address, got: %q", errb.String())
	}
}

func TestServeEnvDefaultsAndFlagPrecedence(t *testing.T) {
	// The Docker image sets ONEPROBE_HTTP=0.0.0.0:8080; serve must honor
	// env vars as defaults so the documented port mapping reaches the UI.
	// Resolution is pure — no bind happens here.
	t.Setenv("ONEPROBE_HTTP", "0.0.0.0:8080")
	t.Setenv("ONEPROBE_DB", "/data/from-env.db")
	addr, db, err := resolveServeOpts([]string{})
	if err != nil {
		t.Fatalf("resolveServeOpts: %v", err)
	}
	if addr != "0.0.0.0:8080" || db != "/data/from-env.db" {
		t.Errorf("env defaults not applied: addr=%q db=%q", addr, db)
	}

	// Explicit flags win over env vars.
	addr, db, err = resolveServeOpts([]string{"--http=127.0.0.1:9", "--db", "/tmp/from-flag.db"})
	if err != nil {
		t.Fatalf("resolveServeOpts: %v", err)
	}
	if addr != "127.0.0.1:9" || db != "/tmp/from-flag.db" {
		t.Errorf("flags must override env: addr=%q db=%q", addr, db)
	}

	// A bindable env addr of :0 exercises env + ephemeral-port smoke end to end.
	envDB := filepath.Join(t.TempDir(), "from-env.db")
	t.Setenv("ONEPROBE_HTTP", "127.0.0.1:0")
	t.Setenv("ONEPROBE_DB", envDB)
	var out, errb bytes.Buffer
	code := run([]string{"serve"}, &out, &errb)
	if code != 0 {
		t.Fatalf("serve with ONEPROBE_HTTP=127.0.0.1:0 should bind and exit, exit=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "from-env.db") {
		t.Errorf("serve must use ONEPROBE_DB as the db default, got: %q", out.String())
	}
}

func TestMigrateDefaultsToCommittedFixtureAndRequiresDryRun(t *testing.T) {
	var out, errb bytes.Buffer
	// No --input: falls back to the committed sample fixture.
	code := run([]string{"migrate", "--from=datadog", "--dry-run", "--format=json"}, &out, &errb)
	if code != 0 {
		t.Fatalf("default input should be the committed fixture, exit=%d stderr=%q", code, errb.String())
	}
	if !json.Valid(out.Bytes()) {
		t.Errorf("expected JSON output, got: %q", out.String())
	}
	// Without --dry-run, migrate refuses: v0.1 only ever reports.
	var out2, errb2 bytes.Buffer
	code = run([]string{"migrate", "--from=datadog"}, &out2, &errb2)
	if code != 2 {
		t.Fatalf("migrate without --dry-run must refuse, exit=%d", code)
	}
	if !strings.Contains(errb2.String(), "--dry-run") {
		t.Errorf("refusal must name the required --dry-run flag, got: %q", errb2.String())
	}
}
