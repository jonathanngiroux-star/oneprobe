// Oneprobe is a single-binary infrastructure monitor: scrape
// Prometheus-format targets, store samples in embedded SQLite, query a
// documented PromQL subset, alert locally, and dry-run Datadog dashboard
// migrations. Free to self-host; donations only — no paid tier.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jonathanngiroux-star/oneprobe/internal/donate"
	"github.com/jonathanngiroux-star/oneprobe/internal/migrate/datadog"
	"github.com/jonathanngiroux-star/oneprobe/internal/serve"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
	"github.com/jonathanngiroux-star/oneprobe/internal/tui"
)

// version is stamped at build time (-ldflags -X main.version=X.Y.Z).
var version = "dev"

const usage = `oneprobe — single-binary infrastructure monitor

Usage: oneprobe <command> [flags]

Commands:
  serve    Start the HTTP server (scrape, GUI, /healthz, /donate)
  tui      Terminal UI over the same store (tab/shift+tab screens, q quit)
  migrate  Dry-run a migration plan from an incumbent (e.g. Datadog)
  query    Query stored samples (planned — see docs/query.md)
  version  Print the binary name and version
  donate   Print donation addresses and exit

Oneprobe is free to self-host. Donations are optional and gate nothing.
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "oneprobe %s\n", version)
		return 0
	case "donate":
		fmt.Fprint(stdout, donate.PlainText())
		return 0
	case "migrate":
		return runMigrate(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "tui":
		return runTUI(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}

// defaultFixture is the committed sample dashboard, used when migrate is
// invoked without --input (the README quickstart depends on this).
const defaultFixture = "testdata/fixtures/datadog/sample-dashboard.json"

func runMigrate(args []string, stdout, stderr io.Writer) int {
	var from, format, input string
	dryRun := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--dry-run":
			dryRun = true
		case strings.HasPrefix(a, "--from="):
			from = strings.TrimPrefix(a, "--from=")
		case a == "--from" && i+1 < len(args):
			i++
			from = args[i]
		case strings.HasPrefix(a, "--format="):
			format = strings.TrimPrefix(a, "--format=")
		case a == "--format" && i+1 < len(args):
			i++
			format = args[i]
		case strings.HasPrefix(a, "--input="):
			input = strings.TrimPrefix(a, "--input=")
		case a == "--input" && i+1 < len(args):
			i++
			input = args[i]
		default:
			fmt.Fprintf(stderr, "migrate: unknown flag %q\n", a)
			return 2
		}
	}
	if !dryRun {
		fmt.Fprintln(stderr, "migrate: v0.1 only reports; pass --dry-run (no data is written either way)")
		return 2
	}
	if from == "" {
		fmt.Fprintln(stderr, "migrate: --from is required (supported: datadog)")
		return 2
	}
	if from != "datadog" {
		fmt.Fprintf(stderr, "migrate: unsupported --from=%s (supported: datadog)\n", from)
		return 2
	}
	if format != "" && format != "json" {
		fmt.Fprintf(stderr, "migrate: unsupported --format=%s (supported: json)\n", format)
		return 2
	}
	if input == "" {
		input = defaultFixture
	}
	raw, err := readDashboard(input)
	if err != nil {
		fmt.Fprintf(stderr, "migrate: %v\n", err)
		return 1
	}
	var dash datadog.Dashboard
	if err := json.Unmarshal(raw, &dash); err != nil {
		fmt.Fprintf(stderr, "migrate: %s is not a valid Datadog dashboard export: %v\n", input, err)
		return 1
	}
	report := datadog.Diff(dash)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(stderr, "migrate: encode report: %v\n", err)
		return 1
	}
	return 0
}

// readDashboard loads the dashboard export. A relative path that does not
// resolve from the working directory is retried from each ancestor
// directory, so `oneprobe migrate` works from any subdirectory of the
// repo as well as from the repo root.
func readDashboard(path string) ([]byte, error) {
	if raw, err := os.ReadFile(path); err == nil {
		return raw, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read input %q: %w", path, err)
	}
	dir := filepath.Dir(abs)
	for {
		candidate := filepath.Join(dir, path)
		if raw, err := os.ReadFile(candidate); err == nil {
			return raw, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("cannot read input %q: not found in this or any parent directory", path)
		}
		dir = parent
	}
}

// resolveServeOpts parses serve flags over env-var defaults. Explicit
// flags always win. Pure: no bind, no store open.
func resolveServeOpts(args []string) (httpAddr, dbPath string, err error) {
	httpAddr = os.Getenv("ONEPROBE_HTTP")
	if httpAddr == "" {
		httpAddr = "127.0.0.1:8080"
	}
	dbPath = os.Getenv("ONEPROBE_DB")
	if dbPath == "" {
		dbPath = "oneprobe.db"
	}
	// Flags: --http=addr, --http addr, --db=path, --db path.
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--http="):
			httpAddr = strings.TrimPrefix(a, "--http=")
		case a == "--http" && i+1 < len(args):
			i++
			httpAddr = args[i]
		case strings.HasPrefix(a, "--db="):
			dbPath = strings.TrimPrefix(a, "--db=")
		case a == "--db" && i+1 < len(args):
			i++
			dbPath = args[i]
		default:
			return "", "", fmt.Errorf("serve: unknown flag %q", a)
		}
	}
	return httpAddr, dbPath, nil
}

// runServe starts the HTTP server. --http=127.0.0.1:0 (or :0) binds an
// ephemeral port and exits immediately after a successful bind (used by
// tests and smoke checks); any other address serves until the process is
// stopped.
func runServe(args []string, stdout, stderr io.Writer) int {
	httpAddr, dbPath, err := resolveServeOpts(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	srv, err := serve.New(dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}
	defer srv.Close()

	ln, err := net.Listen("tcp", httpAddr)
	if err != nil {
		fmt.Fprintf(stderr, "serve: cannot listen on %s: %v\n", httpAddr, err)
		return 1
	}
	fmt.Fprintf(stdout, "oneprobe %s listening on http://%s (db: %s)\n", version, ln.Addr(), dbPath)

	// Ephemeral-port smoke mode: prove the bind worked, then exit 0.
	if _, port, err := net.SplitHostPort(httpAddr); err == nil && port == "0" {
		ln.Close()
		return 0
	}

	httpSrv := &http.Server{Handler: srv.Handler()}
	if err := httpSrv.Serve(ln); err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}
	return 0
}

// runTUI launches the terminal UI against the shared store. Same db
// as serve: --db / $ONEPROBE_DB / oneprobe.db.
func runTUI(args []string, stdout, stderr io.Writer) int {
	dbPath := os.Getenv("ONEPROBE_DB")
	if dbPath == "" {
		dbPath = "oneprobe.db"
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--db="):
			dbPath = strings.TrimPrefix(a, "--db=")
		case a == "--db" && i+1 < len(args):
			i++
			dbPath = args[i]
		default:
			fmt.Fprintf(stderr, "tui: unknown flag %q\n", a)
			return 2
		}
	}
	// Fail BEFORE taking over the terminal when the db cannot open.
	st, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "tui: cannot open %s: %v\n", dbPath, err)
		return 1
	}
	defer st.Close()
	if err := tui.Run(st); err != nil {
		fmt.Fprintf(stderr, "tui: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
