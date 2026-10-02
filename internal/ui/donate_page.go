package ui

import (
	"strings"

	"github.com/jonathanngiroux-star/oneprobe/internal/donate"
)

// DonatePage returns the HTML donate page served at /donate: both
// addresses, copy buttons, no payment form. Same rules as Page(): no
// backticks (Go raw strings), entities only in JS escapes.
func DonatePage() string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Donate — Oneprobe</title>
<style>
  :root { color-scheme: light dark; }
  body { font-family: system-ui, sans-serif; margin: 0 auto; max-width: 720px; padding: 1rem; }
  h1 { font-size: 1.2rem; }
  .addr { display: flex; align-items: center; gap: .5rem; margin: .75rem 0; flex-wrap: wrap; }
  .addr code { font-family: ui-monospace, monospace; font-size: .8rem; word-break: break-all;
               padding: .4rem .5rem; border: 1px solid color-mix(in srgb, currentColor 25%, transparent); flex: 1 1 auto; }
  button { padding: .35rem .8rem; cursor: pointer; }
  .note { font-size: .85rem; opacity: .8; margin-top: 1.5rem; }
  footer { margin-top: 2rem; padding-top: .75rem; border-top: 1px solid color-mix(in srgb, currentColor 15%, transparent); font-size: .8rem; }
  a { color: inherit; }
</style>
</head>
<body>
<h1>Donate to Oneprobe</h1>
<p>Oneprobe is free to self-host. If it replaces a Datadog dashboard
week, donate — optional, and no feature is ever gated on it.</p>
`)
	for _, a := range donate.Addresses() {
		b.WriteString("<div class=\"addr\"><span>" + a.Chain + "</span>" +
			"<code id=\"addr-" + addrSlug(a.Chain) + "\">" + a.Address + "</code>" +
			"<button data-target=\"addr-" + addrSlug(a.Chain) + "\" class=\"copy\">Copy</button></div>\n")
	}
	b.WriteString(`
<p class="note">Optional memo: include a GitHub handle in your
wallet's memo field if you want a credit in the thank-you list. It
changes nothing about the software.</p>

<footer>
  <div><a href="/">Back to Oneprobe</a></div>
  <div>There is no paid tier, no subscription, and no "contact sales."</div>
</footer>

<script>
function esc(s) {
  return String(s)
    .replace(/&/g, "\u0026amp;")
    .replace(/</g, "\u0026lt;")
    .replace(/>/g, "\u0026gt;")
    .replace(/"/g, "\u0026quot;");
}
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
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(done, function () { fallback(); });
  } else {
    fallback();
  }
  function fallback() {
    var ta = document.createElement("textarea");
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand("copy"); done(); } catch (e) { alert("Copy failed - select the address manually."); }
    document.body.removeChild(ta);
  }
});
</script>
</body>
</html>`)
	return b.String()
}

// addrSlug makes a DOM-id-safe slug from a chain label.
func addrSlug(chain string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(chain) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
