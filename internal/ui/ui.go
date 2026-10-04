// Package ui holds Oneprobe's embedded single-page interface: targets,
// recent scrapes with errors, the query graph, alert rules with
// history, the Datadog migrate dry-run screen, the retention note, and
// the donate footer. It is intentionally plain — no framework, no
// build step, one file per page. The HTML/JS below must not contain
// backticks (they would terminate Go raw strings).
package ui

import (
	"os"
	"strings"

	_ "embed"

	"github.com/jonathanngiroux-star/oneprobe/internal/donate"
)

// testbridgeJS is the in-app test-bridge interpreter. It is served at
// /testbridge.js ONLY when the app is launched for testing
// (ONEPROBE_GUI_TESTBRIDGE=1); production pages never reference it.
//
//go:embed testbridge.js
var testbridgeJS string

// BridgeArmed reports whether the test bridge is enabled for this
// process. Desktop launches set it via env; the server serves the
// interpreter only when armed.
func BridgeArmed() bool { return os.Getenv("ONEPROBE_GUI_TESTBRIDGE") == "1" }

// TestBridgeJS returns the interpreter source (bridge-only use).
func TestBridgeJS() string { return testbridgeJS }

// Page returns the full HTML document served at /.
func Page() string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Oneprobe</title>
<style>
  :root {
    color-scheme: light dark;
    --bg: #f5f5f4; --panel: #ffffff; --border: rgba(0,0,0,.14); --text: #1c1917; --muted: #57534e;
    --accent: #2563eb; --ok: #15803d; --warn: #b45309; --bad: #b91c1c;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #141417; --panel: #1d1d22; --border: rgba(255,255,255,.14); --text: #e7e5e4; --muted: #a8a29e;
      --accent: #60a5fa; --ok: #4ade80; --warn: #fbbf24; --bad: #f87171;
    }
  }
  body { font-family: system-ui, sans-serif; margin: 0; padding: 0; background: var(--bg); color: var(--text); }
  #layout { display: flex; min-height: 100vh; }
  #menu { width: 168px; flex: 0 0 auto; padding: .8rem .6rem; border-right: 1px solid var(--border);
          background: var(--panel); position: sticky; top: 0; height: 100vh; box-sizing: border-box; }
  #menu h1 { font-size: 1.05rem; margin: .2rem .2rem .8rem .2rem; }
  #menu button { display: block; width: 100%; text-align: left; margin-bottom: .25rem; padding: .45rem .6rem;
                 border: 1px solid transparent; border-radius: 6px; background: transparent; color: var(--text);
                 cursor: pointer; font-size: .9rem; }
  #menu button:hover { background: color-mix(in srgb, var(--accent) 12%, transparent); }
  #menu button.active { background: color-mix(in srgb, var(--accent) 18%, transparent);
                         border-color: color-mix(in srgb, var(--accent) 45%, transparent); font-weight: 600; }
  #menu .spacer { flex: 1; }
  #content { flex: 1; padding: 1rem 1.4rem; max-width: 860px; min-width: 0; }
  #first-run-hint { display: flex; gap: .6rem; align-items: center; padding: .55rem .75rem; margin-bottom: .9rem;
                    border: 1px solid color-mix(in srgb, var(--accent) 45%, transparent);
                    background: color-mix(in srgb, var(--accent) 10%, transparent); border-radius: 6px; font-size: .88rem; }
  #first-run-hint[hidden] { display: none; }
  .section { display: none; }
  .section.active { display: block; }
  h1 { font-size: 1.2rem; }
  h2 { font-size: 1rem; margin-top: 0; }
  h3 { font-size: .95rem; margin-top: 1.25rem; }
  table { border-collapse: collapse; width: 100%; margin-top: .5rem; }
  th, td { text-align: left; padding: .35rem .5rem; border-bottom: 1px solid var(--border); font-size: .85rem; }
  form { display: flex; gap: .5rem; margin-top: .5rem; flex-wrap: wrap; }
  input, select, textarea { padding: .3rem .4rem; }
  textarea { width: 100%; font-family: ui-monospace, monospace; font-size: .8rem; }
  button { padding: .3rem .7rem; cursor: pointer; }
  #graph { margin-top: .5rem; min-height: 220px; }
  #wizard { position: fixed; inset: 0; z-index: 50; }
  #wizard[hidden] { display: none; }
  #wizard-backdrop { position: absolute; inset: 0; background: rgba(0,0,0,.55); }
  #wizard-card { position: relative; margin: 8vh auto; max-width: 760px; padding: 1rem 1.25rem;
                display: flex; gap: 1.1rem; border: 1px solid var(--border);
                background: var(--panel); border-radius: 8px; }
  #wizard-steps { flex: 0 0 190px; border-right: 1px solid var(--border); padding-right: .9rem; }
  #wizard-steps button { display: flex; width: 100%; text-align: left; margin-bottom: .3rem; padding: .4rem .55rem;
                         border: 1px solid transparent; border-radius: 6px; background: transparent;
                         color: var(--text); cursor: pointer; font-size: .85rem; gap: .45rem; }
  #wizard-steps button.current { background: color-mix(in srgb, var(--accent) 18%, transparent);
                                 border-color: color-mix(in srgb, var(--accent) 45%, transparent); }
  #wizard-steps button .tick { color: var(--ok); font-weight: 700; }
  #wizard-main { flex: 1; min-width: 0; }
  #wizard-controls { display: flex; gap: .5rem; margin-top: .9rem; }
  #wizard label { display: inline-flex; flex-direction: column; gap: .15rem; font-size: .82rem;
                  color: var(--muted); margin-right: .8rem; }
  #wizard input, #wizard select { background: var(--bg); color: var(--text);
                                  border: 1px solid var(--border); border-radius: 4px; padding: .3rem .45rem; }
  #wizard button[type=button] { background: var(--panel); color: var(--text); border: 1px solid var(--border); }
  #wizard-result { color: var(--ok); }
  footer { margin-top: 2rem; padding-top: .75rem; border-top: 1px solid var(--border); font-size: .8rem; }
  footer .addr { display: flex; align-items: center; gap: .5rem; margin: .35rem 0; flex-wrap: wrap; }
  footer .addr code { font-family: ui-monospace, monospace; font-size: .8rem; word-break: break-all;
                      padding: .25rem .4rem; border: 1px solid color-mix(in srgb, currentColor 20%, transparent); flex: 1 1 auto; }
  footer .addr button { padding: .2rem .6rem; cursor: pointer; }
  .error { color: var(--bad); }
  .ok { color: var(--muted); }
  .note { font-size: .8rem; opacity: .8; margin-top: .35rem; }
  .bucket-mapped { color: var(--ok); }
  .bucket-partial { color: var(--warn); }
  .bucket-unmapped { color: var(--bad); }
  code { font-family: ui-monospace, monospace; font-size: .8rem; }
</style>
<div id="layout">
<nav id="menu" aria-label="sections">
  <h1>Oneprobe</h1>
  <button type="button" data-section="targets" class="active">Targets</button>
  <button type="button" data-section="scrapes">Scrapes</button>
  <button type="button" data-section="graph">Graph</button>
  <button type="button" data-section="alerts">Alerts</button>
  <button type="button" data-section="migrate">Migrate</button>
  <button type="button" data-section="backup">Backup</button>
  <button type="button" data-section="donate">Donate</button>
  <div class="spacer"></div>
  <button type="button" id="wizard-open" title="setup wizard" aria-label="Run setup wizard">Run setup wizard</button>
</nav>
<div id="content">

<div id="first-run-hint" hidden>
  <span>New here? The setup wizard walks every workflow with real actions.</span>
  <button type="button" id="hint-wizard-open">Run setup wizard</button>
  <button type="button" id="hint-dismiss" aria-label="Dismiss">x</button>
</div>

<div id="wizard" hidden aria-role="dialog" aria-label="setup wizard">
  <div id="wizard-backdrop"></div>
  <div id="wizard-card" role="dialog" aria-modal="true" aria-labelledby="wizard-title">
    <div id="wizard-steps" aria-label="wizard steps"></div>
    <div id="wizard-main">
      <p class="note" id="wizard-count"></p>
      <h3 id="wizard-title"></h3>
      <div id="wizard-body"></div>
      <div id="wizard-action"></div>
      <div id="wizard-result" class="note" aria-live="polite"></div>
      <div id="wizard-controls">
        <button type="button" id="wizard-skip">Skip wizard</button>
        <span style="flex:1"></span>
        <button type="button" id="wizard-prev">Back</button>
        <button type="button" id="wizard-next">Next</button>
      </div>
    </div>
  </div>
</div>

<div class="section active" id="section-targets">
<h2 id="targets-heading">Targets</h2>
<form id="add-target" aria-labelledby="targets-heading">
  <label>Name <input name="name" placeholder="name" required></label>
  <label>URL <input name="url" placeholder="http://host:9100/metrics" required size="36"></label>
  <label>Interval&nbsp;ms <input name="interval" type="number" value="15000" min="1000"></label>
  <button type="submit">Add target</button>
</form>
<table><thead><tr><th scope="col">name</th><th scope="col">url</th><th scope="col">interval</th><th scope="col">enabled</th><th scope="col">last error</th><th scope="col"></th></tr></thead>
<tbody id="targets"></tbody></table>
</div>

<div class="section" id="section-scrapes">
<h2 id="scrapes-heading">Recent scrapes</h2>
<table aria-labelledby="scrapes-heading"><thead><tr><th scope="col">target</th><th scope="col">at</th><th scope="col">duration</th><th scope="col">samples</th><th scope="col">status</th></tr></thead>
<tbody id="scrapes"></tbody></table>
</div>

<div class="section" id="section-alerts">
<h2 id="alerts-heading">Alerts</h2>
<form id="add-alert" aria-labelledby="alerts-heading">
  <label>Name <input name="name" placeholder="name" required></label>
  <label>Expr <input name="expr" placeholder="e.g. up or rate(errors_total[5m])" required size="28"></label>
  <label>Op
    <select name="op">
      <option value="<"><</option><option value="<="><=</option>
      <option value=">">></option><option value=">=">>=</option>
      <option value="==">==</option><option value="!=">!=</option>
    </select>
  </label>
  <label>Value <input name="value" type="number" step="any" placeholder="0" required size="6"></label>
  <label>For&nbsp;s <input name="for_secs" type="number" value="0" min="0" size="4" title="condition must hold this many seconds"></label>
  <label>Webhook <input name="webhook_url" placeholder="optional http://..." size="18"></label>
  <button type="submit">Add alert rule</button>
</form>
<table><thead><tr><th scope="col">name</th><th scope="col">condition</th><th scope="col">state</th><th scope="col">value</th><th scope="col">since</th><th scope="col"></th></tr></thead>
<tbody id="alerts"></tbody></table>

<h3 id="alert-history-heading">Alert history</h3>
<p class="note">Use a rule's history button to see its firing/resolved events.</p>
<table aria-labelledby="alert-history-heading"><thead><tr><th scope="col">rule</th><th scope="col">state</th><th scope="col">value</th><th scope="col">at</th></tr></thead>
<tbody id="alert-history"><tr><td colspan=4 class=ok>no rule selected</td></tr></tbody></table>
</div>

<div class="section" id="section-graph">
<h2 id="graph-heading">Graph</h2>
<form id="query-form" aria-labelledby="graph-heading">
  <label>Expression <input name="expr" placeholder="e.g. up or sum by (code) (rate(http_requests_total[5m]))" size="44"></label>
  <button type="submit">Plot</button>
</form>
<p class="note">Documented subset: selectors, = and =~ matchers, rate/increase over a range, sum/avg/max/min with by, comparisons. Anything else is refused with the supported set.</p>
<div id="graph" aria-live="polite"><svg width="100%" height="200" role="img" aria-label="time series graph"></svg></div>
</div>

<div class="section" id="section-migrate">
<h2 id="migrate-heading">Datadog migrate (dry-run)</h2>
<p class="note">Paste a Datadog dashboard export JSON or choose a file. Nothing is written; the report shows what would translate and what would not.</p>
<form id="migrate-form" aria-labelledby="migrate-heading">
  <button type="submit">Dry-run pasted JSON</button>
  <button type="button" id="migrate-file-pick">Choose file...</button>
  <input type="file" id="migrate-file" accept=".json,application/json" aria-label="Datadog dashboard export JSON file" hidden>
</form>
<textarea id="migrate-input" rows="6" aria-label="Datadog dashboard export JSON" placeholder="paste dashboard export JSON here"></textarea>
<div id="migrate-report" aria-live="polite"></div>
</div>

<div class="section" id="section-backup">
<h2 id="retention-heading">Retention and backup</h2>
<p class="note" id="retention-note">Samples are kept until deleted. To back up or move Oneprobe, copy the
SQLite file (the path printed at startup, e.g. <code>oneprobe.db</code>) while the process is stopped —
that single file is the whole database. There is no hosted retention product and never will be.</p>
</div>

<div class="section" id="section-donate">
<h2 id="donate-heading">Donate</h2>
<p class="note">Oneprobe is free to self-host. If it replaces a Datadog dashboard week,
donate — optional, and no feature is ever gated on it.</p>
`)

	for _, a := range donate.Addresses() {
		id := "footer-" + addrSlug(a.Chain)
		b.WriteString("  <div class=\"addr\"><span>" + a.Chain + ":</span>" +
			"<code id=\"" + id + "\">" + a.Address + "</code>" +
			"<button type=\"button\" class=\"copy\" data-target=\"" + id + "\" aria-label=\"Copy " + a.Chain + " address\">Copy</button></div>\n")
	}
	b.WriteString(`</div>

<footer>
  <div>Oneprobe is free to self-host. If it replaces a Datadog dashboard week, <a href="/donate">donate</a>.</div>
`)

	b.WriteString(`</footer>
</div>
</div>
<script>
var $ = function (sel) { return document.querySelector(sel); };
function esc(s) {
  // Entities are built from JS unicode escapes (\u0026 = "&") so the
  // Go source never contains raw "&...;" sequences. & must be replaced
  // first or the entities double-escape.
  return String(s)
    .replace(/&/g, "\u0026amp;")
    .replace(/</g, "\u0026lt;")
    .replace(/>/g, "\u0026gt;")
    .replace(/"/g, "\u0026quot;");
}

// Footer + donate copy buttons: clipboard API with a textarea
// fallback (non-secure contexts, older WebViews). Button flashes
// "Copied" for 1.2s so the user sees the action land.
document.addEventListener("click", function (ev) {
  var btn = ev.target;
  if (!btn.classList || !btn.classList.contains("copy")) return;
  var el = document.getElementById(btn.getAttribute("data-target"));
  if (!el) return;
  var text = el.textContent;
  function done() {
    var old = btn.textContent;
    btn.textContent = "Copied";
    setTimeout(function () { btn.textContent = old; }, 1200);
  }
  function fallback() {
    var ta = document.createElement("textarea");
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand("copy"); done(); } catch (e) { alert("Copy failed - select the address manually."); }
    document.body.removeChild(ta);
  }
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(done, fallback);
  } else {
    fallback();
  }
});
function fmt(v) {
  if (typeof v !== "number" || !isFinite(v)) return String(v);
  return Math.abs(v) >= 100 ? v.toFixed(0) : String(v);
}

var lastScrapeErrors = {};
function refreshTargets() {
  var tbody = $("#targets");
  tbody.innerHTML = "<tr><td colspan=6 class=ok>loading...</td></tr>";
  fetch("/api/targets").then(function (r) { return r.json(); }).then(function (targets) {
    if (!Array.isArray(targets) || targets.length === 0) {
      tbody.innerHTML = "<tr><td colspan=6 class=ok>no targets yet - add one above</td></tr>";
      return;
    }
    var rows = "";
    for (var i = 0; i < targets.length; i++) {
      var t = targets[i];
      var err = lastScrapeErrors[t.id] || "";
      rows += "<tr><td>" + esc(t.name) + "</td><td>" + esc(t.url) + "</td><td>" +
        (t.interval / 1000) + "s</td><td>" + (t.enabled ? "yes" : "no") +
        "</td><td class=\"" + (err ? "error" : "ok") + "\">" + esc(err || "-") +
        "</td><td><button data-id=\"" + t.id + "\" class=\"del\" type=\"button\">remove</button></td></tr>";
    }
    tbody.innerHTML = rows;
  }).catch(function (e) {
    tbody.innerHTML = "<tr><td colspan=6 class=error>" + esc(e) + "</td></tr>";
  });
}

function refreshScrapes() {
  var tbody = $("#scrapes");
  tbody.innerHTML = "<tr><td colspan=5 class=ok>loading...</td></tr>";
  fetch("/api/scrapes").then(function (r) { return r.json(); }).then(function (rows) {
    if (!Array.isArray(rows) || rows.length === 0) {
      tbody.innerHTML = "<tr><td colspan=5 class=ok>no scrapes yet</td></tr>";
      return;
    }
    var html = "";
    for (var i = 0; i < rows.length; i++) {
      var r = rows[i];
      if (!r.scrapes || r.scrapes.length === 0) {
        html += "<tr><td>" + r.target_id + "</td><td colspan=4 class=ok>not scraped yet</td></tr>";
        continue;
      }
      var s = r.scrapes[0];
      lastScrapeErrors[r.target_id] = s.ok ? "" : (s.error || "scrape failed");
      html += "<tr><td>" + s.target_id + "</td><td>" + new Date(s.at).toISOString() +
        "</td><td>" + s.duration_ms + "ms</td><td>" + s.sample_count +
        "</td><td class=\"" + (s.ok ? "ok" : "error") + "\">" + esc(s.ok ? "ok" : s.error) + "</td></tr>";
    }
    tbody.innerHTML = html;
    refreshTargets(); // re-render with fresh error column
  }).catch(function (e) {
    tbody.innerHTML = "<tr><td colspan=5 class=error>" + esc(e) + "</td></tr>";
  });
}

function refreshAlerts() {
  var tbody = $("#alerts");
  tbody.innerHTML = "<tr><td colspan=6 class=ok>loading...</td></tr>";
  fetch("/api/alerts").then(function (r) { return r.json(); }).then(function (rules) {
    if (!Array.isArray(rules) || rules.length === 0) {
      tbody.innerHTML = "<tr><td colspan=6 class=ok>no alert rules yet - add one above</td></tr>";
      return;
    }
    var html = "";
    for (var i = 0; i < rules.length; i++) {
      var a = rules[i];
      var since = a.changed_at ? new Date(a.changed_at).toISOString() : "-";
      html += "<tr><td>" + esc(a.name) + "</td><td>" + esc(a.expr) + " " + esc(a.op) +
        " " + fmt(a.value) + (a.for_secs ? " for " + a.for_secs + "s" : "") + "</td>" +
        "<td class=\"" + (a.state === "firing" ? "error" : "ok") + "\">" + esc(a.state) + "</td>" +
        "<td>" + fmt(a.last_value) + "</td><td>" + since + "</td>" +
        "<td><button data-id=\"" + a.id + "\" class=\"adel\" type=\"button\">remove</button> " +
        "<button data-id=\"" + a.id + "\" class=\"ahist\" type=\"button\">history</button></td></tr>";
    }
    tbody.innerHTML = html;
  }).catch(function (e) {
    tbody.innerHTML = "<tr><td colspan=6 class=error>" + esc(e) + "</td></tr>";
  });
}

function loadAlertHistory(ruleId) {
  var tbody = $("#alert-history");
  tbody.innerHTML = "<tr><td colspan=4 class=ok>loading...</td></tr>";
  fetch("/api/alerts/" + ruleId + "/history").then(function (r) { return r.json(); }).then(function (events) {
    if (!Array.isArray(events) || events.length === 0) {
      tbody.innerHTML = "<tr><td colspan=4 class=ok>no events yet for rule " + ruleId + "</td></tr>";
      return;
    }
    var html = "";
    for (var i = 0; i < events.length; i++) {
      var e = events[i];
      html += "<tr><td>" + e.rule_id + "</td><td class=\"" + (e.state === "firing" ? "error" : "ok") + "\">" +
        esc(e.state) + "</td><td>" + fmt(e.value) + "</td><td>" + new Date(e.at).toISOString() + "</td></tr>";
    }
    tbody.innerHTML = html;
  }).catch(function (e) {
    tbody.innerHTML = "<tr><td colspan=4 class=error>" + esc(e) + "</td></tr>";
  });
}

function plotExpr(expr) {
  fetch("/api/query?expr=" + encodeURIComponent(expr))
    .then(function (r) {
      if (!r.ok) return r.text().then(function (t) { throw t; });
      return r.json();
    })
    .then(function (data) {
      var svg = $("#graph svg");
      var series = data.series || [];
      var nPts = 0, min = Infinity, max = -Infinity, t0 = Infinity, t1 = -Infinity;
      for (var s = 0; s < series.length; s++) {
        var pts = series[s].points || [];
        nPts += pts.length;
        for (var i = 0; i < pts.length; i++) {
          if (pts[i].value < min) min = pts[i].value;
          if (pts[i].value > max) max = pts[i].value;
          if (pts[i].ts < t0) t0 = pts[i].ts;
          if (pts[i].ts > t1) t1 = pts[i].ts;
        }
      }
      if (nPts === 0) {
        svg.innerHTML = "<text x=\"8\" y=\"24\" class=\"ok\">no samples for " + esc(expr) + " in the lookback window</text>";
        return;
      }
      var W = 940, H = 200, pad = 10;
      if (max === min) { max += 1; min -= 1; }
      if (t1 <= t0) t1 = t0 + 1;
      function X(t) { return pad + (W - 2 * pad) * (t - t0) / (t1 - t0); }
      function Y(v) { return H - pad - (H - 2 * pad) * (v - min) / (max - min); }
      var inner = "";
      var hues = [210, 130, 30, 270, 350, 50, 190, 310];
      for (var s = 0; s < series.length; s++) {
        var pts = series[s].points || [];
        var path = "";
        for (var i = 0; i < pts.length; i++) {
          path += (path ? " L" : "M") + X(pts[i].ts).toFixed(1) + " " + Y(pts[i].value).toFixed(1);
        }
        inner += "<path d=\"" + path + "\" fill=\"none\" stroke=\"hsl(" + (hues[s % hues.length]) + ",70%,50%)\" stroke-width=\"1.5\"/>";
      }
      inner += "<text x=\"8\" y=\"16\" font-size=\"12\">" + esc(expr) + " - " + series.length +
        " series, " + nPts + " points, min " + fmt(min) + " max " + fmt(max) + "</text>";
      svg.innerHTML = inner;
    })
    .catch(function (e) {
      $("#graph svg").innerHTML = "<text x=\"8\" y=\"24\" class=\"error\">" + esc(e) + "</text>";
    });
}

function renderMigrateReport(report) {
  var el = $("#migrate-report");
  var counts = [
    ["mapped", (report.mapped || []).length],
    ["partial", (report.partial || []).length],
    ["unmapped", (report.unmapped || []).length],
    ["risks", (report.risks || []).length]
  ];
  var html = "<p><strong>" + esc(report.input ? report.input.title : "?") + "</strong>: " +
    counts.map(function (c) { return "<span class=\"bucket-" + c[0] + "\">" + c[0] + " " + c[1] + "</span>"; }).join(", ") +
    "</p><table><thead><tr><th>bucket</th><th>widget</th><th>detail</th></tr></thead><tbody>";
  function row(bucket, w) {
    var detail = w.reason || w.note || "";
    var q = w.oneprobe_query || "";
    return "<tr><td class=\"bucket-" + bucket + "\">" + bucket + "</td><td>" +
      esc(w.title || ("widget " + w.widget_id)) + "</td><td>" + esc(detail) +
      (q ? " to <code>" + esc(q) + "</code>" : "") + "</td></tr>";
  }
  var i;
  for (i = 0; i < (report.mapped || []).length; i++) html += row("mapped", report.mapped[i]);
  for (i = 0; i < (report.partial || []).length; i++) html += row("partial", report.partial[i]);
  for (i = 0; i < (report.unmapped || []).length; i++) html += row("unmapped", report.unmapped[i]);
  for (i = 0; i < (report.risks || []).length; i++) {
    html += "<tr><td class=\"bucket-partial\">risk</td><td>" + esc(report.risks[i].key) +
      "</td><td>" + esc(report.risks[i].note) + "</td></tr>";
  }
  html += "</tbody></table>";
  el.innerHTML = html;
}

function runMigrate(jsonText) {
  fetch("/api/migrate/datadog", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: jsonText
  }).then(function (r) {
    if (!r.ok) return r.text().then(function (t) { throw t; });
    return r.json();
  }).then(renderMigrateReport)
    .catch(function (e) {
      $("#migrate-report").innerHTML = "<p class=\"error\">" + esc(e) + "</p>";
    });
}

$("#add-target").addEventListener("submit", function (ev) {
  ev.preventDefault();
  var f = ev.target;
  var body = {
    name: f.elements["name"].value,
    url: f.elements["url"].value,
    interval: parseInt(f.elements["interval"].value, 10) || 15000
  };
  fetch("/api/targets", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  }).then(function (r) {
    if (!r.ok) return r.text().then(function (t) { throw t; });
    f.reset();
    f.elements["interval"].value = 15000;
    refreshTargets(); refreshScrapes();
  }).catch(function (t) { alert("add target failed: " + t); });
});

$("#targets").addEventListener("click", function (ev) {
  if (!ev.target.classList.contains("del")) return;
  var id = ev.target.dataset.id;
  fetch("/api/targets/" + id, { method: "DELETE" }).then(function (r) {
    if (!r.ok) return r.text().then(function (t) { throw t; });
    delete lastScrapeErrors[id];
    refreshTargets(); refreshScrapes();
  }).catch(function (t) { alert("delete failed: " + t); });
});

$("#query-form").addEventListener("submit", function (ev) {
  ev.preventDefault();
  var expr = ev.target.elements["expr"].value.trim();
  if (!expr) return;
  plotExpr(expr);
});

$("#add-alert").addEventListener("submit", function (ev) {
  ev.preventDefault();
  var f = ev.target;
  var body = {
    name: f.elements["name"].value,
    expr: f.elements["expr"].value,
    op: f.elements["op"].value,
    value: parseFloat(f.elements["value"].value),
    for_secs: parseInt(f.elements["for_secs"].value, 10) || 0,
    webhook_url: f.elements["webhook_url"].value || ""
  };
  fetch("/api/alerts", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  }).then(function (r) {
    if (!r.ok) return r.text().then(function (t) { throw t; });
    f.reset();
    f.elements["for_secs"].value = 0;
    refreshAlerts();
  }).catch(function (t) { alert("add alert failed: " + t); });
});

$("#alerts").addEventListener("click", function (ev) {
  if (ev.target.classList.contains("adel")) {
    fetch("/api/alerts/" + ev.target.dataset.id, { method: "DELETE" }).then(function (r) {
      if (!r.ok) return r.text().then(function (t) { throw t; });
      refreshAlerts();
    }).catch(function (t) { alert("delete failed: " + t); });
    return;
  }
  if (ev.target.classList.contains("ahist")) {
    loadAlertHistory(ev.target.dataset.id);
  }
});

$("#migrate-form").addEventListener("submit", function (ev) {
  ev.preventDefault();
  var text = $("#migrate-input").value.trim();
  if (!text) { alert("paste a dashboard export JSON first"); return; }
  runMigrate(text);
});

$("#migrate-file-pick").addEventListener("click", function () {
  $("#migrate-file").click();
});

$("#migrate-file").addEventListener("change", function (ev) {
  var file = ev.target.files[0];
  if (!file) return;
  var reader = new FileReader();
  reader.onload = function () {
    $("#migrate-input").value = reader.result;
    runMigrate(reader.result);
  };
  reader.readAsText(file);
});

// ---- setup wizard ----------------------------------------------------
// One step per workflow. The wizard is user-activated (sidebar
// button / first-run hint) — never auto-opened — remembers
// completion in localStorage, and reopens any time.
var ETH_ADDR = "0x85ee7E71f762d772599cbF1EC20E651B30657521";
var BTC_ADDR = "bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg";
var wizardSteps = [
  {
    title: "Welcome to Oneprobe",
    body: "This wizard completes every workflow for real: you will " +
          "create a target, watch it scrape, run a query, set an " +
          "alert, and dry-run a Datadog migration - all inside this " +
          "guide. One binary, your own SQLite, free to self-host.",
    action: "",
  },
  {
    title: "Step 1 - Add a target",
    body: "Give Oneprobe a Prometheus-format endpoint to scrape. " +
          "Fill the fields and press Create target - it happens " +
          "right here in the wizard.",
    action: '<label>Name <input id="wizard-f-name" value="my-node"></label>' +
            '<label>URL <input id="wizard-f-url" value="http://127.0.0.1:8080/healthz" size="34"></label>' +
            '<label>Interval&nbsp;ms <input id="wizard-f-interval" type="number" value="5000"></label>' +
            '<button type="button" id="wizard-act-target">Create target</button>',
  },
  {
    title: "Step 2 - Watch scrapes land",
    body: "Your target is now scraping on its interval. Recent " +
          "outcomes appear below - hit refresh to see the latest. " +
          "Failures show the exact error text.",
    action: '<button type="button" id="wizard-act-refresh">Refresh scrapes</button>',
  },
  {
    title: "Step 3 - Query the data",
    body: "Type a documented-subset expression and press Run query. " +
          "The result below comes from the real engine.",
    action: '<label>Expression <input id="wizard-f-expr" value="sum(up)" size="30"></label>' +
            '<button type="button" id="wizard-act-query">Run query</button>',
  },
  {
    title: "Step 4 - Set an alert",
    body: "Create a real rule: expression, comparison, value. It " +
          "evaluates locally every 15s from now on.",
    action: '<label>Name <input id="wizard-f-aname" value="up-down"></label>' +
            '<label>Expr <input id="wizard-f-aexpr" value="up" size="18"></label>' +
            '<label>Op <select id="wizard-f-aop">' +
            '<option value="<"><</option><option value="<="><=</option>' +
            '<option value=">">></option><option value=">=">>=</option>' +
            '<option value="==">==</option><option value="!=">!=</option></select></label>' +
            '<label>Value <input id="wizard-f-avalue" type="number" step="any" value="1" size="5"></label>' +
            '<button type="button" id="wizard-act-alert">Create alert rule</button>',
  },
  {
    title: "Step 5 - Dry-run a Datadog migration",
    body: "Press Dry-run to run the committed sample dashboard " +
          "through the real classifier. Nothing is written - you " +
          "get the honest mapped/partial/unmapped/risks report.",
    action: '<button type="button" id="wizard-act-migrate">Dry-run sample dashboard</button>',
  },
  {
    title: "Step 6 - Donate (optional)",
    body: "Everything you just did is free forever. If Oneprobe " +
          "replaces a Datadog dashboard week, donate - no feature " +
          "is ever gated on it.<br>" +
          "Ethereum / USDC (ERC-20): <code>" + ETH_ADDR + "</code><br>" +
          "Bitcoin: <code>" + BTC_ADDR + "</code>",
    action: "",
  },
];

var wizardSucceeded = {}; // step index -> true once its action ran
function wizardRenderSteps() {
  var el = $("#wizard-steps");
  var html = "";
  for (var i = 0; i < wizardSteps.length; i++) {
    html += '<button type="button" data-step=' + i + ' class="' +
      (i === wizardIdx ? "current" : "") + '">' +
      (wizardSucceeded[i] ? '<span class="tick">&#10003;</span> ' : (i + 1) + ". ") +
      wizardSteps[i].title + "</button>";
  }
  el.innerHTML = html;
  var btns = el.querySelectorAll("button[data-step]");
  for (var k = 0; k < btns.length; k++) {
    btns[k].addEventListener("click", function (ev) {
      var n = parseInt(ev.currentTarget.getAttribute("data-step"), 10);
      if (n <= wizardIdx || wizardSucceeded[n]) { // only completed/current steps are reachable
        wizardIdx = n;
        wizardShow(wizardIdx);
      }
    });
  }
}
function wizardShow(i) {
  var step = wizardSteps[i];
  $("#wizard-count").textContent = "step " + (i + 1) + " of " + wizardSteps.length;
  $("#wizard-title").textContent = step.title;
  $("#wizard-body").innerHTML = step.body;
  $("#wizard-action").innerHTML = step.action || "";
  $("#wizard-result").textContent = "";
  $("#wizard-next").textContent = i === wizardSteps.length - 1 ? "Finish" : "Next";
  $("#wizard-prev").disabled = i === 0;
  $("#wizard").dataset.step = String(i);
  $("#wizard").hidden = false;
  wizardRenderSteps();
  wizardWireStep();
  $("#wizard-next").focus();
}

function wizardDone(msg) {
  $("#wizard-result").textContent = msg;
  if (wizardIdx > 0) { wizardSucceeded[wizardIdx] = true; wizardRenderSteps(); }
}

// The committed sample dashboard, embedded for the migrate step.
var WIZARD_SAMPLE = {"id":"wizard-1","title":"Oneprobe Sample","widgets":[
  {"id":1,"type":"timeseries","title":"CPU","requests":[{"q":"avg:system.cpu.user{host:web-1}"}]},
  {"id":2,"type":"check_status","title":"Synthetics","requests":[{"q":"\"synthetics\" in (\"check_state\")"}]}
]};

function wizardDoCreateTarget() {
  var body = {
    name: $("#wizard-f-name").value || "my-node",
    url: $("#wizard-f-url").value,
    interval: parseInt($("#wizard-f-interval").value, 10) || 5000
  };
  fetch("/api/targets", { method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  }).then(function (r) {
    if (!r.ok) return r.text().then(function (t) { throw t; });
    return r.json();
  }).then(function (t) {
    var msg = t.created
      ? "Target created (id " + t.id + ") - now scraping every " + (t.interval / 1000) + "s. Press Next."
      : "A target with this URL already existed - updated it instead (id " + t.id + ", now named " + t.name + "). Press Next.";
    wizardDone(msg);
    refreshTargets(); refreshScrapes();
  }).catch(function (e) { wizardDone("failed: " + e); });
}

function wizardDoQuery() {
  var expr = $("#wizard-f-expr").value;
  fetch("/api/query?expr=" + encodeURIComponent(expr))
    .then(function (r) { if (!r.ok) return r.text().then(function (t) { throw t; }); return r.json(); })
    .then(function (data) {
      var n = (data.series || []).length;
      wizardDone(expr + " -> " + n + " series" + (n ? " (see values in the result rows)" : " - no samples in this store yet, that is a valid answer"));
    })
    .catch(function (e) { wizardDone("query refused: " + e); });
}

function wizardDoCreateAlert() {
  var body = {
    name: $("#wizard-f-aname").value || "up-down",
    expr: $("#wizard-f-aexpr").value || "up",
    op: $("#wizard-f-aop").value || "<",
    value: parseFloat($("#wizard-f-avalue").value) || 1,
    enabled: true
  };
  fetch("/api/alerts", { method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  }).then(function (r) {
    if (!r.ok) return r.text().then(function (t) { throw t; });
    return r.json();
  }).then(function (a) {
    wizardDone("Alert rule created (id " + a.id + "): " + body.expr + " " + body.op + " " + body.value + " - evaluating locally every 15s. Press Next.");
    refreshAlerts();
  }).catch(function (e) { wizardDone("failed: " + e); });
}

function wizardDoMigrate() {
  fetch("/api/migrate/datadog", { method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify(WIZARD_SAMPLE)
  }).then(function (r) { if (!r.ok) return r.text().then(function (t) { throw t; }); return r.json(); })
    .then(function (rep) {
      wizardDone("Sample dry-run: mapped " + (rep.mapped || []).length + ", partial " + (rep.partial || []).length +
               ", unmapped " + (rep.unmapped || []).length + ", risks " + (rep.risks || []).length +
               " - the full table is below in the migrate section.");
    })
    .catch(function (e) { wizardDone("failed: " + e); });
}

function wizardWireStep() {
  var el = $("#wizard-action");
  el.onclick = function (ev) {
    var id = ev.target && ev.target.id;
    if (id === "wizard-act-target") wizardDoCreateTarget();
    else if (id === "wizard-act-refresh") { refreshScrapes(); wizardDone("Scrape list refreshed - the latest outcomes are in the Recent scrapes panel."); }
    else if (id === "wizard-act-query") wizardDoQuery();
    else if (id === "wizard-act-alert") wizardDoCreateAlert();
    else if (id === "wizard-act-migrate") wizardDoMigrate();
  };
}
function wizardClose(remember) {
  $("#wizard").hidden = true;
  if (remember) {
    try { localStorage.setItem("wizardDone", "1"); } catch (e) {}
  }
}
var wizardIdx = 0;

$("#wizard-next").addEventListener("click", function () {
  wizardIdx++;
  if (wizardIdx >= wizardSteps.length) { wizardClose(true); return; }
  wizardShow(wizardIdx);
});
$("#wizard-prev").addEventListener("click", function () {
  if (wizardIdx > 0) { wizardIdx--; wizardShow(wizardIdx); }
});
$("#wizard-skip").addEventListener("click", function () { wizardClose(true); });
$("#wizard-backdrop").addEventListener("click", function () { wizardClose(true); });
document.addEventListener("keydown", function (ev) {
  if ($("#wizard").hidden) return;
  if (ev.key === "Escape") { wizardClose(true); }
  else if (ev.key === "ArrowRight") { $("#wizard-next").click(); }
  else if (ev.key === "ArrowLeft") { $("#wizard-prev").click(); }
  // Enter is deliberately NOT handled here: the focused Next/Finish
  // button receives it natively — handling it here as well double-
  // advances the wizard (found in the live browser audit).
});

// ---- section menu -------------------------------------------------------
function switchSection(name) {
  var btns = document.querySelectorAll("#menu button[data-section]");
  for (var i = 0; i < btns.length; i++) {
    var b = btns[i];
    if (b.getAttribute("data-section") === name) b.classList.add("active");
    else b.classList.remove("active");
  }
  var secs = document.querySelectorAll(".section");
  for (var j = 0; j < secs.length; j++) {
    if (secs[j].id === "section-" + name) secs[j].classList.add("active");
    else secs[j].classList.remove("active");
  }
  try { localStorage.setItem("section", name); } catch (e) {}
}
(function wireMenu() {
  var btns = document.querySelectorAll("#menu button[data-section]");
  for (var i = 0; i < btns.length; i++) {
    btns[i].addEventListener("click", function (ev) {
      switchSection(ev.currentTarget.getAttribute("data-section"));
    });
  }
  var saved = null;
  try { saved = localStorage.getItem("section"); } catch (e) {}
  if (saved) switchSection(saved);
})();

// First-run hint: on an empty store and a wizard never completed, show
// the hint bar with a Run setup wizard button. NEVER auto-open.
(function firstRunHint() {
  var done = false;
  try { done = localStorage.getItem("wizardDone") === "1"; } catch (e) {}
  if (done) return;
  fetch("/api/targets").then(function (r) { return r.json(); }).then(function (targets) {
    if (Array.isArray(targets) && targets.length === 0) {
      $("#first-run-hint").hidden = false;
    }
  }).catch(function () {});
})();
$("#hint-wizard-open").addEventListener("click", function () { $("#first-run-hint").hidden = true; $("#wizard-open").click(); });
$("#hint-dismiss").addEventListener("click", function () { $("#first-run-hint").hidden = true; });
$("#wizard-open").addEventListener("click", function () {
  $("#first-run-hint").hidden = true; // the user found the wizard
  wizardIdx = 0;
  wizardShow(wizardIdx);
});

refreshTargets();
refreshScrapes();
refreshAlerts();
setInterval(refreshScrapes, 5000);
setInterval(refreshAlerts, 5000);
</script>
` + bridgeTail())
	return b.String()
}

// bridgeTail closes the page body. When the test bridge is armed it
// injects the interpreter script tag; production pages stay clean.
func bridgeTail() string {
	if BridgeArmed() {
		return "<script src=\"/testbridge.js\"></script>\n</body>\n</html>"
	}
	return "</body>\n</html>"
}
