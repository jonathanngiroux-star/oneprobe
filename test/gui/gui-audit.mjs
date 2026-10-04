/**
 * Step-4 GUI audit — Playwright against the real oneprobe binary.
 *
 * Run:  node test/gui/gui-audit.mjs
 * Needs: go build -o oneprobe ./cmd/oneprobe  (done automatically)
 *
 * Every check mirrors the brief:
 *  - add target, scrape fixture, see a point on the graph
 *  - create alert, force fire, see history row
 *  - migrate fixture renders unmapped list
 *  - donate page contains both addresses byte-for-byte
 *  - no console errors on the happy path
 *  - keyboard path for primary actions
 *  - empty/error/loading states
 *  - no secrets in the DOM
 *  - JS-disabled: donate + health work
 */
import { chromium } from "playwright";
import { spawn } from "node:child_process";
import http from "node:http";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const ETH = "0x85ee7E71f762d772599cbF1EC20E651B30657521";
const BTC = "bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg";

const results = [];
const failures = [];
async function gotoSection(name) {
  await page.click(`#menu button[data-section=${name}]`);
  await page.waitForFunction(
    (n) => document.querySelector(".section.active").id === "section-" + n, name);
}

function record(name, ok, detail = "") {
  results.push({ name, ok, detail });
  if (!ok) failures.push(`${name}${detail ? ": " + detail : ""}`);
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? " — " + detail : ""}`);
}

// --- fixture exporter ---------------------------------------------------
const fixtDir = fs.mkdtempSync(path.join(os.tmpdir(), "oneprobe-gui-fixt-"));
fs.writeFileSync(path.join(fixtDir, "metrics"),
  "# HELP up Up\n# TYPE up gauge\n" +
  "up 1\n" +
  'audit_metric{note="a # b"} 42\n' +
  "gui_counter_total 100\n");
const exporter = http.createServer((req, res) => {
  res.writeHead(200, { "Content-Type": "text/plain" });
  res.end(fs.readFileSync(path.join(fixtDir, "metrics")));
});
await new Promise(r => exporter.listen(0, "127.0.0.1", r));
const exporterPort = exporter.address().port;

// --- oneprobe binary ----------------------------------------------------
// A stale WAL/SHM from a previous run corrupts a fresh db open.
for (const suf of ["", "-wal", "-shm"]) fs.rmSync("/tmp/gui-audit.db" + suf, { force: true });
const proc = spawn("./oneprobe", ["serve", "--db", "/tmp/gui-audit.db", "--http", "127.0.0.1:18090"], {
  cwd: path.join(process.cwd()), stdio: ["ignore", "pipe", "pipe"],
});
let bootline = "";
proc.stdout.on("data", d => { bootline += d; });
const base = "http://127.0.0.1:18090";
for (let i = 0; i < 50; i++) {
  try { await fetch(base + "/healthz"); break; } catch { await new Promise(r => setTimeout(r, 200)); }
}

const browser = await chromium.launch();
const ctx = await browser.newContext({ colorScheme: "dark" });
const consoleErrors = [];
ctx.on("console", msg => { if (msg.type() === "error") consoleErrors.push(msg.text()); });
const badResponses = [];
ctx.on("response", resp => { if (resp.status() >= 400) badResponses.push(resp.status() + " " + resp.url()); });
const page = await ctx.newPage();

try {
  // === Screen walk: targets ============================================
  await page.goto(base + "/");
  await page.waitForSelector("#targets");

  // Empty state shows before anything is added (fresh db).
  const emptyTargets = await page.locator("#targets td").first().textContent();
  record("targets empty state", /no targets yet/.test(emptyTargets), JSON.stringify(emptyTargets));

  // Fresh db => the wizard does NOT auto-open; the first-run hint
  // shows instead. The wizard is explicit-activation only.
  const wizardHidden = await page.$eval("#wizard", el => el.hidden);
  record("wizard does not auto-open", wizardHidden === true);
  const hintShown = await page.$eval("#first-run-hint", el => !el.hidden);
  record("first-run hint offers the wizard", hintShown === true);
  await page.click("#wizard-open");
  const wizardOpen = await page.$eval("#wizard", el => !el.hidden);
  record("sidebar button opens the wizard", wizardOpen === true);
  const cardDark = await page.$eval("#wizard-card", el => getComputedStyle(el).backgroundColor);
  record("wizard card uses the dark palette", cardDark !== "rgb(255, 255, 255)", cardDark);
  await page.keyboard.press("Escape");
  await page.waitForFunction(() => document.querySelector("#wizard").hidden);
  record("esc closes the wizard", true);
  // section menu: switch and verify
  await page.click("#menu button[data-section=scrapes]");
  const activeSec = await page.$eval(".section.active", el => el.id);
  record("menu switches to scrapes section", activeSec === "section-scrapes", activeSec);
  await page.click("#menu button[data-section=targets]"); // back for the form checks


  // Keyboard path: focus the form, fill via keyboard, submit with Enter.
  await page.locator('#add-target input[name="name"]').click();
  await page.keyboard.type("gui-fixture");
  await page.locator('#add-target input[name="url"]').click();
  await page.keyboard.type(`http://127.0.0.1:${exporterPort}/metrics`);
  await page.locator('#add-target input[name="interval"]').fill("1000");
  await page.locator('#add-target button[type="submit"]').focus();
  await page.keyboard.press("Enter");
  await page.waitForFunction(() =>
    document.querySelectorAll("#targets tr").length > 0 &&
    !document.querySelector("#targets td[colspan]"), null, { timeout: 10000 });
  const targetRow = await page.locator("#targets tr").first().textContent();
  record("add target via keyboard", /gui-fixture/.test(targetRow), "row=" + targetRow.trim());

  // Scrape happens within ~2s (1s interval). Recent-scrapes panel.
  await page.waitForFunction(() => {
    const t = document.querySelector("#scrapes");
    return t && /ok/.test(t.textContent) && !/loading/.test(t.textContent);
  }, null, { timeout: 15000 });
  const scrapesText = await page.locator("#scrapes").textContent();
  record("scrape visible in scrapes panel", /ok/.test(scrapesText), scrapesText.trim().slice(0, 80));

  // === Graph ============================================================
  await gotoSection("graph");
  await page.locator('#query-form input[name="expr"]').click();
  await page.keyboard.type("up");
  await page.locator('#query-form button[type="submit"]').focus();
  await page.keyboard.press("Enter");
  await page.waitForFunction(() => {
    const svg = document.querySelector("#graph svg");
    return svg && svg.querySelector("path") !== null;
  }, null, { timeout: 15000 });
  const graphText = await page.locator("#graph svg").textContent();
  record("graph plots a point for `up`", /up/.test(graphText) && /1 series/.test(graphText), graphText.trim().slice(0, 90));

  // Console errors are measured on the HAPPY path only. Everything
  // below this point deliberately provokes error states (400s are
  // logged by Chromium as console errors). Measure now, then reset.
  record("no console errors on happy path", consoleErrors.length === 0,
    "console=" + consoleErrors.slice(0, 3).join(" | ") + " http=" + badResponses.slice(0, 3).join(" | "));
  consoleErrors.length = 0;
  badResponses.length = 0;

  // Empty graph state: unknown metric.
  await page.locator('#query-form input[name="expr"]').fill("no_such_metric_xyz");
  await page.locator('#query-form button[type="submit"]').click();
  await page.waitForFunction(() => /no (series|samples) for no_such_metric_xyz/.test(document.querySelector("#graph").textContent));
  record("graph empty state names the metric", true);

  // Error graph state: undocumented function.
  await page.locator('#query-form input[name="expr"]').fill("histogram_quantile(0.9, x)");
  await page.locator('#query-form button[type="submit"]').click();
  await page.waitForFunction(() => /unsupported function/.test(document.querySelector("#graph").textContent));
  record("graph error state names the rejected function", true);

  // === Alerts: create, force fire, history =============================
  await gotoSection("alerts");
  await page.locator('#add-alert input[name="name"]').click();
  await page.keyboard.type("gui-up-down");
  await page.locator('#add-alert input[name="expr"]').fill("up");
  await page.locator('#add-alert select[name="op"]').selectOption("<");
  await page.locator('#add-alert input[name="value"]').fill("1");
  await page.locator('#add-alert button[type="submit"]').focus();
  await page.keyboard.press("Enter");
  await page.waitForFunction(() => document.querySelectorAll("#alerts tr").length > 0 &&
    /gui-up-down/.test(document.querySelector("#alerts").textContent), null, { timeout: 5000 });
  record("create alert via keyboard", true);

  // Force fire: flip the fixture to up=0 so the rule (up < 1) fires.
  fs.writeFileSync(path.join(fixtDir, "metrics"), "# HELP up Up\n# TYPE up gauge\nup 0\n");
  await page.waitForFunction(() => {
    const a = document.querySelector("#alerts");
    return a && /firing/.test(a.textContent);
  }, null, { timeout: 30000 });
  record("alert fires and shows firing state", true);

  // History row.
  await page.locator('#alerts button.ahist').first().click();
  await page.waitForFunction(() => {
    const h = document.querySelector("#alert-history");
    return h && /firing/.test(h.textContent) && !/loading/.test(h.textContent);
  }, null, { timeout: 10000 });
  const histText = await page.locator("#alert-history").textContent();
  record("alert history row appears", /firing/.test(histText), histText.trim().slice(0, 80));

  // Restore fixture.
  fs.writeFileSync(path.join(fixtDir, "metrics"), "# HELP up Up\n# TYPE up gauge\nup 1\n");

  // === Migrate screen ====================================================
  const dash = {
    id: "pw-1", title: "Playwright Fixture",
    widgets: [
      { id: 1, type: "timeseries", title: "CPU", requests: [{ q: "avg:system.cpu.user{host:web-1}" }] },
      { id: 2, type: "check_status", title: "Synthetics", requests: [{ q: '"synthetics" in ("check_state")' }] },
    ],
  };
  await gotoSection("migrate");
  await page.locator("#migrate-input").fill(JSON.stringify(dash));
  await page.locator('#migrate-form button[type="submit"]').focus();
  await page.keyboard.press("Enter");
  await page.waitForFunction(() => {
    const m = document.querySelector("#migrate-report");
    return m && m.querySelector("table") !== null;
  }, null, { timeout: 10000 });
  const migrateText = await page.locator("#migrate-report").textContent();
  record("migrate renders mapped", /mapped 1/.test(migrateText), "");
  record("migrate renders unmapped list", /unmapped 1/.test(migrateText) && /Synthetics/.test(migrateText),
    migrateText.slice(0, 90));

  // Migrate error state: garbage.
  await page.locator("#migrate-input").fill("{bad json");
  await page.locator('#migrate-form button[type="submit"]').click();
  await page.waitForFunction(() => /not a valid Datadog dashboard export/.test(
    document.querySelector("#migrate-report").textContent));
  record("migrate error state names the problem", true);

  // === Donate page ======================================================
  await page.goto(base + "/donate");
  const donateHtml = await page.content();
  record("donate has ETH address byte-for-byte", donateHtml.includes(ETH));
  record("donate has BTC address byte-for-byte", donateHtml.includes(BTC));
  record("donate has copy buttons", /Copy/i.test(donateHtml));
  record("donate has no payment form", !/stripe|checkout|payment form/i.test(donateHtml));

  // === No secrets in the DOM ============================================
  const mainHtml = await page.goto(base + "/").then(() => page.content());
  record("no secrets in the DOM", !/api[_-]?key|secret|password|token/i.test(mainHtml));

} finally {
  await browser.close();
}

// === JS disabled: donate + health ========================================
const b2 = await chromium.launch({ javaScriptEnabled: false });
try {
  const p2 = await b2.newPage();
  const resp1 = await p2.goto(base + "/donate");
  const donateNoJS = await p2.content();
  record("donate works with JS disabled (200)", resp1.status() === 200);
  record("donate addresses visible without JS", donateNoJS.includes(ETH) && donateNoJS.includes(BTC));
  const resp2 = await p2.goto(base + "/healthz");
  const health = await resp2.text();
  record("healthz works with JS disabled", resp2.status() === 200 && health.trim() === "ok");
} finally {
  await b2.close();
}

proc.kill("SIGKILL");
exporter.close();

console.log("\n==== GUI AUDIT SUMMARY ====");
console.log(`total: ${results.length}  passed: ${results.filter(r => r.ok).length}  failed: ${failures.length}`);
if (failures.length) {
  console.log("FAILURES:");
  failures.forEach(f => console.log("  - " + f));
  process.exit(1);
}
console.log("ALL GREEN");
