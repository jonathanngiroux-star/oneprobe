package serve

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestTestBridgeRouteServedOnlyWhenArmed: /testbridge.js must 404 in a
// production launch and serve the interpreter only when
// ONEPROBE_GUI_TESTBRIDGE=1. Same for the page's script tag — a
// production page must never reference the bridge.
func TestTestBridgeRouteServedOnlyWhenArmed(t *testing.T) {
	// Disarmed: route 404s, page has no bridge reference.
	os.Unsetenv("ONEPROBE_GUI_TESTBRIDGE")
	srv := httptest.NewServer(newTestServer(t))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/testbridge.js")
	if err != nil {
		t.Fatalf("GET /testbridge.js: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("disarmed /testbridge.js -> %d, want 404", resp.StatusCode)
	}
	pageResp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	pageBytes, _ := io.ReadAll(pageResp.Body)
	pageResp.Body.Close()
	if strings.Contains(string(pageBytes), "testbridge") {
		t.Error("disarmed page must not reference the bridge")
	}
}

// TestTestBridgeRouteServesInterpreterWhenArmed: with the env var set,
// the route serves the interpreter and the page injects the tag.
func TestTestBridgeRouteServesInterpreterWhenArmed(t *testing.T) {
	t.Setenv("ONEPROBE_GUI_TESTBRIDGE", "1")
	srv := httptest.NewServer(newTestServer(t))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/testbridge.js")
	if err != nil {
		t.Fatalf("GET /testbridge.js: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("armed /testbridge.js -> %d, want 200", resp.StatusCode)
	}
	body := make([]byte, 1<<16)
	n, _ := resp.Body.Read(body)
	script := string(body[:n])
	if !strings.Contains(script, "test:cmd") {
		t.Error("interpreter must listen for the test:cmd event")
	}
	if strings.Contains(script, "eval(") {
		t.Error("interpreter must not use eval (fixed switch only)")
	}

	pageResp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer pageResp.Body.Close()
	buf := new(strings.Builder)
	b := make([]byte, 1<<16)
	for {
		r, err := pageResp.Body.Read(b)
		buf.Write(b[:r])
		if err != nil || r == 0 {
			break
		}
	}
	if !strings.Contains(buf.String(), "/testbridge.js") {
		t.Error("armed page must inject the bridge script tag")
	}
}
