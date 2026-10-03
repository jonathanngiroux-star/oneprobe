# TUI audit (step 6)

Date: 2026-10-03
Method: two layers.
1. In-process model tests (`internal/tui/tui_test.go`) — drive
   `Model.update` with synthetic key/tick messages; no terminal.
2. Scripted PTY audit (`test/gui/tui-audit.mjs`, node-pty) — spawn the
   real `oneprobe tui` in an 80x24 pty, answer OSC-11/DSR like a real
   terminal, send keys, assert rendered bytes.

## Results

Model tests: 18/18 green.
PTY audit: 8/8 green.

| PTY check | Result | Detail |
|---|---|---|
| Launch → first render | PASS | 22ms (no blocking network on launch) |
| Resize 80x24 → 60x18 → 120x40 | PASS | no crash, keeps rendering |
| Tab cycles all 6 screens | PASS | scrapes, query, alerts, migrate, donate, targets |
| Query evaluates via keyboard | PASS | `sum(up) -> 0 series` on an empty db — correct |
| Migrate fixture = CLI parity | PASS | mapped 1, partial 2, unmapped 1, risks 3 + per-widget detail |
| Donate addresses byte-for-byte | PASS | ETH + BTC exact |
| q quits cleanly | PASS | exit observed |

## Bugs found and fixed (not ticketed)

1. **Multi-rune input silently dropped (product bug, real).** Bubble
   Tea coalesces fast terminal input (and pastes) into a single
   `KeyRunes` message carrying several characters. Both the adapter
   (`normalizeKey` — returned `""` for anything longer than 1 char)
   and the Model (appended only `len(msg)==1`) dropped them, so typing
   at normal speed into the query/migrate prompts lost characters.
   The step-5 smoke passed only because it typed one character per
   write with sleeps. Fixed: the adapter passes the whole rune string;
   the Model appends it whole. The PTY audit now types in bursts and
   passes.
2. **Weak refusal in migrate garbage-JSON path (product bug).**
   `decodeDashboard` formatted the refusal with a hardcoded `"input"`
   placeholder — the operator saw `error: input is not a valid Datadog
   dashboard export` with no idea which file. Fixed: the path is
   passed through and named (`<path> is not a valid ...`). RED test:
   TestMigrateGarbageJSONNamesPath.
3. **Test bug (fixed in the test):** TestBackspaceEditsInput asserted a
   wrong string (`sum((up)` arithmetic error in the test itself);
   rewritten to assert the input buffer directly before evaluating.

## Failure modes probed and clean

- Keyboard coverage: tab/shift+tab all screens, typing, backspace,
  enter, empty-enter no-op, q/ctrl+c quit — 18 model tests.
- Refresh tick landing mid-typing does NOT clear the input
  (TestRefreshTickDoesNotClearInput).
- Screen switch abandons a half-typed input; last query result
  persists across screen switches.
- Unicode target names (日本語ノード) and very long URLs render without
  panic.
- Resize at launch and mid-run; no crash, continuous rendering.
- No network on launch: first render 22ms in the PTY.

## Reproduce

```
go build -o oneprobe ./cmd/oneprobe
node test/gui/tui-audit.mjs
go test ./internal/tui/
```
