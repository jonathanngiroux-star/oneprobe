// Package ui holds Oneprobe's embedded single-page interface: targets,
// recent scrapes with errors, the query graph, alert rules with
// history, the Datadog migrate dry-run screen, the retention note, and
// the donate footer. It is intentionally plain — no framework, no
// build step, one file per page. The HTML/JS below must not contain
// backticks (they would terminate Go raw strings).
package ui

import (
	"strings"

	"github.com/jonathanngiroux-star/oneprobe/internal/donate"
)

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
  :root { color-scheme: light dark; }
  body { font-family: system-ui, sans-serif; margin: 0 auto; max-width: 960px; padding: 1rem; }
  h1 { font-size: 1.2rem; }
  h2 { font-size: 1rem; margin-top: 1.75rem; }
  h3 { font-size: .95rem; margin-top: 1.25rem; }
  table { border-collapse: collapse; width: 100%; margin-top: .5rem; }
  th, td { text-align: left; padding: .35rem .5rem; border-bottom: 1px solid color-mix(in srgb, currentColor 15%, transparent); font-size: .85rem; }
  form { display: flex; gap: .5rem; margin-top: .5rem; flex-wrap: wrap; }
  input, select, textarea { padding: .3rem .4rem; }
  textarea { width: 100%; font-family: ui-monospace, monospace; font-size: .8rem; }
  button { padding: .3rem .7rem; cursor: pointer; }
  #graph { margin-top: .5rem; min-height: 220px; }
  footer { margin-top: 2rem; padding-top: .75rem; border-top: 1px solid color-mix(in srgb, currentColor 15%, transparent); font-size: .8rem; }
  .error { color: #b00; }
  .ok { opacity: .75; }
  .note { font-size: .8rem; opacity: .8; margin-top: .35rem; }
  .bucket-mapped { color: #080; }
  .bucket-partial { color: #a60; }
  .bucket-unmapped { color: #b00; }
  code { font-family: ui-monospace, monospace; font-size: .8rem; }
</style>
</head>
<body>
<h1>Oneprobe</h1>

<h2 id="targets-heading">Targets</h2>
<form id="add-target" aria-labelledby="targets-heading">
  <label>Name <input name="name" placeholder="name" required></label>
  <label>URL <input name="url" placeholder="http://host:9100/metrics" required size="36"></label>
  <label>Interval&nbsp;ms <input name="interval" type="number" value="15000" min="1000"></label>
  <button type="submit">Add target</button>
</form>
<table><thead><tr><th scope="col">name</th><th scope="col">url</th><th scope="col">interval</th><th scope="col">enabled</th><th scope="col">last error</th><th scope="col"></th></tr></thead>
<tbody id="targets"></tbody></table>

<h2 id="scrapes-heading">Recent scrapes</h2>
<table aria-labelledby="scrapes-heading"><thead><tr><th scope="col">target</th><th scope="col">at</th><th scope="col">duration</th><th scope="col">samples</th><th scope="col">status</th></tr></thead>
<tbody id="scrapes"></tbody></table>

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

<h2 id="graph-heading">Graph</h2>
<form id="query-form" aria-labelledby="graph-heading">
  <label>Expression <input name="expr" placeholder="e.g. up or sum by (code) (rate(http_requests_total[5m]))" size="44"></label>
  <button type="submit">Plot</button>
</form>
<p class="note">Documented subset: selectors, = and =~ matchers, rate/increase over a range, sum/avg/max/min with by, comparisons. Anything else is refused with the supported set.</p>
<div id="graph" aria-live="polite"><svg width="100%" height="200" role="img" aria-label="time series graph"></svg></div>

<h2 id="migrate-heading">Datadog migrate (dry-run)</h2>
<p class="note">Paste a Datadog dashboard export JSON or choose a file. Nothing is written; the report shows what would translate and what would not.</p>
<form id="migrate-form" aria-labelledby="migrate-heading">
  <button type="submit">Dry-run pasted JSON</button>
  <button type="button" id="migrate-file-pick">Choose file...</button>
  <input type="file" id="migrate-file" accept=".json,application/json" aria-label="Datadog dashboard export JSON file" hidden>
</form>
<textarea id="migrate-input" rows="6" aria-label="Datadog dashboard export JSON" placeholder="paste dashboard export JSON here"></textarea>
<div id="migrate-report" aria-live="polite"></div>

<h2 id="retention-heading">Retention and backup</h2>
<p class="note" id="retention-note">Samples are kept until deleted. To back up or move Oneprobe, copy the
SQLite file (the path printed at startup, e.g. <code>oneprobe.db</code>) while the process is stopped —
that single file is the whole database. There is no hosted retention product and never will be.</p>

<footer>
  <div>Oneprobe is free to self-host. If it replaces a Datadog dashboard week, <a href="/donate">donate</a>.</div>
`)
	for _, a := range donate.Addresses() {
		b.WriteString("  <div>" + a.Chain + ": <code>" + a.Address + "</code></div>\n")
	}
	b.WriteString(`</footer>
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

refreshTargets();
refreshScrapes();
refreshAlerts();
setInterval(refreshScrapes, 5000);
setInterval(refreshAlerts, 5000);
</script>
</body>
</html>`)
	return b.String()
}
