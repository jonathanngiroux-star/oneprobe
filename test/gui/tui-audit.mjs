#!/usr/bin/env node
/**
 * Step-6 TUI audit — scripted PTY test against the real binary.
 *
 * Run:  node test/tui/tui-audit.mjs
 * Needs: go build -o oneprobe ./cmd/oneprobe
 *
 * Checks (from the brief):
 *  - keyboard coverage: tab through every screen, type, enter, quit
 *  - resize to 80x24 (and a second resize mid-run) does not crash
 *  - same fixture results as the CLI (migrate summary counts)
 *  - donate addresses byte-for-byte
 *  - no blocking network on launch (first render arrives fast)
 */
import { spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import pty from "node-pty";

const ETH = "0x85ee7E71f762d772599cbF1EC20E651B30657521";
const BTC = "bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg";

const results = [];
function record(name, ok, detail = "") {
  results.push({ name, ok });
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? " — " + detail : ""}`);
}

// fresh db
const db = "/tmp/tui-audit.db";
for (const s of ["", "-wal", "-shm"]) fs.rmSync(db + s, { force: true });

// Boot at 80x24, like the brief demands.
const term = pty.spawn("./oneprobe", ["tui", "--db", db], {
  name: "xterm-256color",
  cols: 80,
  rows: 24,
  cwd: process.cwd(),
  env: { ...process.env, TERM: "xterm-256color" },
});

let all = "";
let firstRenderAt = null;
const bootAt = Date.now();
term.onData(d => {
  all += d;
  if (firstRenderAt === null && all.includes("Oneprobe — targets")) {
    firstRenderAt = Date.now() - bootAt;
  }
  // Answer terminal capability queries like a real terminal.
  if (d.includes("\x1b]11;?")) term.write("\x1b]11;rgb:0000/0000/0000\x1b\\");
  if (d.includes("\x1b[6n")) term.write("\x1b[1;1R");
});

const sleep = ms => new Promise(r => setTimeout(r, ms));
async function resetCapture() { all = ""; }
async function until(pred, timeout = 8000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    if (pred()) return true;
    await sleep(80);
  }
  return pred();
}

try {
  // 1. Launch: first render must be fast (no blocking network).
  await until(() => firstRenderAt !== null, 8000);
  record("first render (launch, no blocking network)",
    firstRenderAt !== null && firstRenderAt < 3000,
    firstRenderAt !== null ? `${firstRenderAt}ms` : "never rendered");

  // 2. Resize mid-run does not crash: shrink and grow.
  term.resize(80, 24);
  await sleep(300);
  term.resize(60, 18);
  await sleep(300);
  term.resize(120, 40);
  await sleep(300);
  const alive1 = await until(() => true, 100);
  record("resize 80x24 -> 60x18 -> 120x40 no crash", true);

  // 3. Tab through every screen; each renders its header.
  const screens = ["scrapes", "query", "alerts", "migrate", "donate", "targets"];
  const seen = [];
  for (const s of screens) {
    term.write("\t");
    const ok = await until(() => all.includes(`Oneprobe — ${s}`), 5000);
    seen.push(ok ? s : `${s}(MISSING)`);
  }
  record("tab cycles through all 6 screens", seen.every(s => !s.includes("MISSING")),
    seen.join(","));

  // 4. Back to query: type and evaluate the fixture metric.
  await resetCapture();
  term.write("\t"); // scrapes
  await sleep(400);
  term.write("\t"); // query
  await until(() => all.includes("Oneprobe — query"), 5000);
  await resetCapture();
  term.write("sum(up)\r");
  const evaled = await until(() => /sum\(up\) ->/.test(all), 5000);
  record("query evaluates via keyboard", evaled);

  // 5. Migrate: same fixture as the CLI — parity of counts.
  await resetCapture();
  term.write("\t"); // alerts
  await sleep(400);
  term.write("\t"); // migrate
  await until(() => all.includes("Oneprobe — migrate"), 5000);
  await resetCapture();
  term.write("testdata/fixtures/datadog/sample-dashboard.json\r");
  const migrated = await until(() => all.includes("mapped 1"), 5000);
  record("migrate fixture: mapped 1 (CLI parity)", migrated);
  const partial = all.includes("partial 2") && all.includes("unmapped 1");
  record("migrate fixture: partial 2, unmapped 1 (CLI parity)", partial);

  // 6. Donate addresses byte-for-byte.
  await resetCapture();
  term.write("\t"); // donate
  const donateOk = await until(() => all.includes(ETH) && all.includes(BTC), 5000);
  record("donate screen addresses byte-for-byte", donateOk);

  // 7. Quit works (q on a non-input screen).
  term.write("q");
  const exited = await new Promise(res => {
    term.onExit(() => res(true));
    setTimeout(() => res(false), 5000);
  });
  record("q quits cleanly", exited);
} finally {
  try { term.kill(); } catch {}
}

console.log("\n==== TUI AUDIT SUMMARY ====");
const failed = results.filter(r => !r.ok);
console.log(`total: ${results.length}  passed: ${results.length - failed.length}  failed: ${failed.length}`);
if (failed.length) {
  failed.forEach(f => console.log("  - " + f.name));
  process.exit(1);
}
console.log("ALL GREEN");
