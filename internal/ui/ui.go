// Package ui holds Oneprobe's embedded single-page interface: targets,
// recent scrapes, one graph, and the donate footer. It is intentionally
// plain — no framework, no build step, one file. The HTML/JS below
// must not contain backticks (they would terminate Go raw strings).
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
  h2 { font-size: 1rem; margin-top: 1.5rem; }
  table { border-collapse: collapse; width: 100%; margin-top: .5rem; }
  th, td { text-align: left; padding: .35rem .5rem; border-bottom: 1px solid color-mix(in srgb, currentColor 15%, transparent); font-size: .85rem; }
  form { display: flex; gap: .5rem; margin-top: .5rem; flex-wrap: wrap; }
  input { padding: .3rem .4rem; }
  button { padding: .3rem .7rem; cursor: pointer; }
  #graph { margin-top: .5rem; min-height: 220px; }
  footer { margin-top: 2rem; padding-top: .75rem; border-top: 1px solid color-mix(in srgb, currentColor 15%, transparent); font-size: .8rem; }
  .error { color: #b00; }
  .ok { opacity: .75; }
</style>
</head>
<body>
<h1>Oneprobe</h1>

<h2>Targets</h2>
<form id="add-target">
  <input name="name" placeholder="name" required>
  <input name="url" placeholder="http://host:9100/metrics" required size="36">
  <input name="interval" type="number" value="15000" min="1000" title="interval ms">
  <button>Add target</button>
</form>
<table><thead><tr><th>name</th><th>url</th><th>interval</th><th>enabled</th><th></th></tr></thead>
<tbody id="targets"></tbody></table>

<h2>Recent scrapes</h2>
<table><thead><tr><th>target</th><th>at</th><th>duration</th><th>samples</th><th>status</th></tr></thead>
<tbody id="scrapes"></tbody></table>

<h2>Alerts</h2>
<form id="add-alert">
  <input name="name" placeholder="name" required>
  <input name="expr" placeholder="expr, e.g. up" required size="24">
  <select name="op"><option><</option><option><=</option><option>></option><option>>=</option><option>==</option><option>!=</option></select>
  <input name="value" type="number" step="any" placeholder="0" required size="6">
  <input name="for_secs" type="number" value="0" min="0" title="for (seconds)" size="4">
  <button>Add alert rule</button>
</form>
<table><thead><tr><th>name</th><th>condition</th><th>state</th><th>value</th><th>since</th><th></th></tr></thead>
<tbody id="alerts"></tbody></table>

<h2>Graph</h2>
<form id="query-form">
  <input name="metric" placeholder="metric name, e.g. up" required>
  <button>Plot last hour</button>
</form>
<div id="graph"><svg width="100%" height="200"></svg></div>

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

function refreshTargets() {
  var tbody = $("#targets");
  tbody.innerHTML = "<tr><td colspan=5 class=ok>loading...</td></tr>";
  fetch("/api/targets").then(function (r) { return r.json(); }).then(function (targets) {
    if (!Array.isArray(targets) || targets.length === 0) {
      tbody.innerHTML = "<tr><td colspan=5 class=ok>no targets yet — add one above</td></tr>";
      return;
    }
    var rows = "";
    for (var i = 0; i < targets.length; i++) {
      var t = targets[i];
      rows += "<tr><td>" + esc(t.name) + "</td><td>" + esc(t.url) + "</td><td>" +
        (t.interval / 1000) + "s</td><td>" + (t.enabled ? "yes" : "no") +
        "</td><td><button data-id=\"" + t.id + "\" class=\"del\">remove</button></td></tr>";
    }
    tbody.innerHTML = rows;
  }).catch(function (e) {
    tbody.innerHTML = "<tr><td colspan=5 class=error>" + esc(e) + "</td></tr>";
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
      html += "<tr><td>" + s.target_id + "</td><td>" + new Date(s.at).toISOString() +
        "</td><td>" + s.duration_ms + "ms</td><td>" + s.sample_count +
        "</td><td class=\"" + (s.ok ? "ok" : "error") + "\">" + esc(s.ok ? "ok" : s.error) + "</td></tr>";
    }
    tbody.innerHTML = html;
  }).catch(function (e) {
    tbody.innerHTML = "<tr><td colspan=5 class=error>" + esc(e) + "</td></tr>";
  });
}

function refreshAlerts() {
  var tbody = $("#alerts");
  tbody.innerHTML = "<tr><td colspan=6 class=ok>loading...</td></tr>";
  fetch("/api/alerts").then(function (r) { return r.json(); }).then(function (rules) {
    if (!Array.isArray(rules) || rules.length === 0) {
      tbody.innerHTML = "<tr><td colspan=6 class=ok>no alert rules yet — add one above</td></tr>";
      return;
    }
    var html = "";
    for (var i = 0; i < rules.length; i++) {
      var a = rules[i];
      var since = a.changed_at ? new Date(a.changed_at).toISOString() : "-";
      html += "<tr><td>" + esc(a.name) + "</td><td>" + esc(a.expr) + " " + esc(a.op) +
        " " + a.value + (a.for_secs ? " for " + a.for_secs + "s" : "") + "</td>" +
        "<td class=\"" + (a.state === "firing" ? "error" : "ok") + "\">" + esc(a.state) + "</td>" +
        "<td>" + a.last_value + "</td><td>" + since + "</td>" +
        "<td><button data-id=\"" + a.id + "\" class=\"adel\">remove</button></td></tr>";
    }
    tbody.innerHTML = html;
  }).catch(function (e) {
    tbody.innerHTML = "<tr><td colspan=6 class=error>" + esc(e) + "</td></tr>";
  });
}

function plotMetric(name) {
  fetch("/api/query?metric=" + encodeURIComponent(name))
    .then(function (r) { return r.json(); })
    .then(function (data) {
      var svg = $("#graph svg");
      var pts = data.points || [];
      if (pts.length === 0) {
        svg.innerHTML = "<text x=\"8\" y=\"24\" class=\"ok\">no samples for " + esc(name) + " in the last hour</text>";
        return;
      }
      var W = 940, H = 200, pad = 10;
      var min = Infinity, max = -Infinity;
      for (var i = 0; i < pts.length; i++) {
        if (pts[i].value < min) min = pts[i].value;
        if (pts[i].value > max) max = pts[i].value;
      }
      if (max === min) { max += 1; min -= 1; }
      var t0 = pts[0].ts, t1 = pts[pts.length - 1].ts || t0 + 1;
      function X(t) { return pad + (W - 2 * pad) * (t - t0) / (t1 - t0); }
      function Y(v) { return H - pad - (H - 2 * pad) * (v - min) / (max - min); }
      var path = "";
      for (var i = 0; i < pts.length; i++) {
        path += (path ? " L" : "M") + X(pts[i].ts).toFixed(1) + " " + Y(pts[i].value).toFixed(1);
      }
      svg.innerHTML = "<path d=\"" + path + "\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"1.5\"/>" +
        "<text x=\"8\" y=\"16\" font-size=\"12\">" + esc(name) + " — " + pts.length +
        " points, min " + min.toFixed(2) + " max " + max.toFixed(2) + "</text>";
    })
    .catch(function (e) {
      $("#graph svg").innerHTML = "<text x=\"8\" y=\"24\" class=\"error\">" + esc(e) + "</text>";
    });
}

$("#add-target").addEventListener("submit", function (ev) {
  ev.preventDefault();
  var f = ev.target;
  var body = {
    name: f.name.value, url: f.url.value,
    interval: parseInt(f.interval.value, 10) || 15000,
  };
  fetch("/api/targets", {
    method: "POST", headers: {"Content-Type": "application/json"},
    body: JSON.stringify(body),
  }).then(function (r) {
    if (!r.ok) return r.text().then(function (t) { throw t; });
    f.reset();
    refreshTargets(); refreshScrapes();
  }).catch(function (t) { alert("add target failed: " + t); });
});

$("#targets").addEventListener("click", function (ev) {
  if (!ev.target.classList.contains("del")) return;
  fetch("/api/targets/" + ev.target.dataset.id, {method: "DELETE"}).then(function (r) {
    if (!r.ok) return r.text().then(function (t) { throw t; });
    refreshTargets(); refreshScrapes();
  }).catch(function (t) { alert("delete failed: " + t); });
});

$("#query-form").addEventListener("submit", function (ev) {
  ev.preventDefault();
  plotMetric(ev.target.metric.value.trim());
});

$("#add-alert").addEventListener("submit", function (ev) {
  ev.preventDefault();
  var f = ev.target;
  var body = {
    name: f.name.value,
    expr: f.expr.value,
    op: f.op.value,
    value: parseFloat(f.value.value),
    for_secs: parseInt(f.for_secs.value, 10) || 0,
  };
  fetch("/api/alerts", {
    method: "POST", headers: {"Content-Type": "application/json"},
    body: JSON.stringify(body),
  }).then(function (r) {
    if (!r.ok) return r.text().then(function (t) { throw t; });
    f.reset();
    refreshAlerts();
  }).catch(function (t) { alert("add alert failed: " + t); });
});

$("#alerts").addEventListener("click", function (ev) {
  if (!ev.target.classList.contains("adel")) return;
  fetch("/api/alerts/" + ev.target.dataset.id, {method: "DELETE"}).then(function (r) {
    if (!r.ok) return r.text().then(function (t) { throw t; });
    refreshAlerts();
  }).catch(function (t) { alert("delete failed: " + t); });
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
