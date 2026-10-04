// In-app test-bridge interpreter for Oneprobe (Proofspan pattern).
//
// Listens for the "test:cmd" event — which the Go side emits ONLY when
// the app was launched with ONEPROBE_GUI_TESTBRIDGE=1 — and runs each
// command against the REAL DOM: clicks fire real handlers, fill hits
// real inputs (with input/change events so the app's listeners
// react), read returns live state. Results go back through the
// TestResult binding -> `TEST-RESULT {json}` in the result log.
//
// The interpreter is a fixed switch — no eval — so no new attack
// surface exists. In a production launch this file is never served
// and the event never fires.
(function () {
  "use strict";

  if (!window.runtime || typeof window.runtime.EventsOn !== "function") return;

  function result(cmd, ok, value, error) {
    var payload;
    try {
      payload = JSON.stringify({
        id: cmd && cmd.id ? cmd.id : "",
        ok: !!ok,
        value: value === undefined ? null : value,
        error: error || null,
      });
    } catch (e) {
      payload = JSON.stringify({ id: "", ok: false, value: null, error: "serialize: " + e });
    }
    if (window.go && window.go.main && window.go.main.App && window.go.main.App.TestResult) {
      window.go.main.App.TestResult(payload).catch(function () {});
    }
  }

  function fire(el, type) {
    el.dispatchEvent(new Event(type, { bubbles: true }));
  }

  function clickEl(el) {
    el.scrollIntoView({ block: "center" });
    el.click();
  }

  function selector(sel) {
    return document.querySelector(sel);
  }

  function fillEl(el, text) {
    el.focus();
    el.value = "";
    document.execCommand && document.execCommand("insertText", false, text);
    if (el.value !== text) el.value = text; // execCommand refused
    fire(el, "input");
    fire(el, "change");
  }

  function readClipboard() {
    // Clipboard readback for copy-button tests: WebKitGTK exposes
    // navigator.clipboard.readText only with permission; the paste
    // fallback reads the DOM only. Return null when unreadable —
    // the driver treats it as SKIP, not FAIL.
    try {
      return navigator.clipboard.readText();
    } catch (e) {
      return null;
    }
  }

  function interpret(cmd) {
    switch (cmd.op) {
      case "click": {
        var el = selector(cmd.sel);
        if (!el) return result(cmd, false, null, "no element for " + cmd.sel);
        clickEl(el);
        return result(cmd, true, null, null);
      }
      case "clickText": {
        var root = cmd.sel ? selector(cmd.sel) : document;
        if (!root) return result(cmd, false, null, "no scope for " + cmd.sel);
        var want = (cmd.text || "").trim().toLowerCase();
        var candidates = root.querySelectorAll("button, a, [role=button], input[type=submit], input[type=button], li, summary");
        var hit = null;
        for (var i = 0; i < candidates.length; i++) {
          var t = (candidates[i].textContent || candidates[i].value || "").trim().toLowerCase();
          if (want && t.indexOf(want) !== -1) { hit = candidates[i]; break; }
        }
        if (!hit) return result(cmd, false, null, "no clickable with text " + cmd.text);
        clickEl(hit);
        return result(cmd, true, null, null);
      }
      case "fill": {
        var f = selector(cmd.sel);
        if (!f) return result(cmd, false, null, "no element for " + cmd.sel);
        fillEl(f, String(cmd.text !== undefined ? cmd.text : ""));
        return result(cmd, true, f.value, null);
      }
      case "select": {
        var s = selector(cmd.sel);
        if (!s) return result(cmd, false, null, "no element for " + cmd.sel);
        var opt = null;
        for (var j = 0; j < s.options.length; j++) {
          if (s.options[j].value === cmd.value || s.options[j].textContent === cmd.value) { opt = s.options[j]; break; }
        }
        if (!opt) return result(cmd, false, null, "no option " + cmd.value);
        s.value = opt.value;
        fire(s, "input");
        fire(s, "change");
        return result(cmd, true, s.value, null);
      }
      case "check": {
        var c = selector(cmd.sel);
        if (!c) return result(cmd, false, null, "no element for " + cmd.sel);
        if (cmd.checked !== undefined) {
          if (c.checked !== !!cmd.checked) { clickEl(c); }
        } else if (!c.checked) {
          clickEl(c);
        }
        return result(cmd, true, c.checked, null);
      }
      case "key": {
        var target = document.activeElement || document.body;
        var ev = new KeyboardEvent(cmd.event || "keydown", {
          key: cmd.key || "", code: cmd.code || "", keyCode: cmd.keycode || 0,
          which: cmd.keycode || 0, bubbles: true, cancelable: true,
        });
        target.dispatchEvent(ev);
        if (cmd.event === "keydown" && cmd.key === "Enter") {
          var form = target.closest ? target.closest("form") : null;
          if (form && target.tagName === "INPUT") {
            var submit = form.querySelector('button[type="submit"], button:not([type])');
            if (submit) clickEl(submit);
          }
        }
        return result(cmd, true, null, null);
      }
      case "read": {
        var r = selector(cmd.sel);
        if (!r) return result(cmd, false, null, "no element for " + cmd.sel);
        var cs;
        try { cs = getComputedStyle(r); } catch (e) { cs = null; }
        var out = {
          text: r.textContent,
          value: r.value !== undefined ? r.value : null,
          checked: r.checked !== undefined ? !!r.checked : null,
          display: cs ? cs.display : null,
          background: cs ? cs.backgroundColor : null,
          color: cs ? cs.color : null,
          cls: r.className || "",
          disabled: !!r.disabled,
        };
        if (cmd.attr) out.attr = r.getAttribute(cmd.attr);
        return result(cmd, true, out, null);
      }
      case "count": {
        return result(cmd, true, document.querySelectorAll(cmd.sel).length, null);
      }
      case "texts": {
        var nodes = document.querySelectorAll(cmd.sel);
        var arr = [];
        for (var k = 0; k < nodes.length; k++) arr.push(nodes[k].textContent.trim());
        return result(cmd, true, arr, null);
      }
      case "clipboard": {
        // copy-button verification: read the system clipboard if the
        // webview allows; null = unreadable (driver SKIPs)
        var self = this;
        if (navigator.clipboard && navigator.clipboard.readText) {
          navigator.clipboard.readText().then(
            function (t) { result(cmd, true, t, null); },
            function (e) { result(cmd, true, null, "unreadable: " + e); }
          );
        } else {
          return result(cmd, true, null, "no clipboard API");
        }
        return;
      }
      case "ping":
        return result(cmd, true, "pong", null);
      default:
        return result(cmd, false, null, "unknown op: " + cmd.op);
    }
  }

  window.runtime.EventsOn("test:cmd", function (cmd) {
    try {
      interpret(cmd || {});
    } catch (e) {
      result(cmd, false, null, "interpreter: " + e);
    }
  });
})();
