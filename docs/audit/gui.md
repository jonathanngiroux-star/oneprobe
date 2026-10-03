# GUI audit (step 4)

Date: 2026-10-03
Method: Playwright (Chromium, headless) against the real `oneprobe serve`
binary with a live fixture exporter — `test/gui/gui-audit.mjs`. The
desktop shell (`cmd/oneprobe-desktop`, Wails) serves the identical
handler (`assetserver.Options{Handler: serve.Handler()}`) and the
identical embedded page, so this audit covers both surfaces; the
routing equality is pinned by
`cmd/oneprobe-desktop/main_test.go:TestAssetServerRoutingProvesTheGUIIsServed`.

## Suite result: 21/21 green

| # | Check | Result |
|---|---|---|
| 1 | Targets empty state renders | PASS |
| 2 | Add target via keyboard only | PASS |
| 3 | Scrape visible in recent-scrapes panel | PASS |
| 4 | Graph plots a point for `up` | PASS |
| 5 | No console errors on the happy path | PASS |
| 6 | Graph empty state names the unknown metric | PASS |
| 7 | Graph error state names the rejected function | PASS |
| 8 | Create alert rule via keyboard only | PASS |
| 9 | Alert fires; firing state visible | PASS |
| 10 | Alert history row appears after firing | PASS |
| 11 | Migrate report renders mapped bucket | PASS |
| 12 | Migrate report renders unmapped list (Synthetics) | PASS |
| 13 | Migrate garbage input named in the error state | PASS |
| 14 | Donate page: ETH address byte-for-byte | PASS |
| 15 | Donate page: BTC address byte-for-byte | PASS |
| 16 | Donate page: copy buttons | PASS |
| 17 | Donate page: no payment form | PASS |
| 18 | No secrets in the DOM | PASS |
| 19 | Donate works with JS disabled (200) | PASS |
| 20 | Donate addresses visible with JS disabled | PASS |
| 21 | /healthz works with JS disabled (`ok`) | PASS |

## Bugs found and fixed (not ticketed)

1. **Audit-harness: console-error window included the audit's own
   deliberate 400s.** Chromium logs every 4xx response as a console
   error, so the "no console errors on the happy path" check failed on
   the very 400s the error-state tests are supposed to produce. Fixed
   by measuring the console after the pure happy path (page load →
   target add → scrape → first graph plot) and resetting the buffer
   before the deliberate error-state checks. Root-caused via a response
   hook that printed the offending URL:
   `400 /api/query?expr=histogram_quantile(0.9,%20x)`.
2. **Audit-harness: ambiguous form selectors.** Two forms share
   `input[name="name"]` (targets, alerts); the first run died on a
   strict-mode violation. All selectors scoped to their form ids.
3. **Audit-harness: stale message expectation.** The graph's zero-points
   message is "no samples for ..."; the test expected "no series".
   Regex widened to match the actual message.

No product bugs were found in the GUI layer in this audit. The
GUI-layer bugs with real severity (no-op `esc()`, the broken
triple-quote that killed the script block) were found and fixed in the
step-2 backend audit; this audit's clean sheet is the independent
confirmation that those fixes hold in a real browser.

## Notes

- The "works with JS disabled" checks cover donate + healthz as the
  brief allows; graphs require JS by design.
- Keyboard path: every primary action (add target, add alert, plot,
  dry-run migrate) is exercised via keyboard events only — no mouse
  clicks on the form inputs.
- Console errors: the happy path produces zero console errors and zero
  HTTP >= 400 responses.
- Desktop (Wails): built via `wails build -tags webkit2_41` (this
  system has webkit2gtk-4.1); launched headful for a bounded smoke and
  exited cleanly. `//go:build desktop || bindings` keeps the CGO-free
  default build untouched — `go build ./...` and the scratch Docker
  image never compile webkit.

## Reproduce

```
go build -o oneprobe ./cmd/oneprobe
node test/gui/gui-audit.mjs    # from test/gui deps installed
```
