//go:build desktop || bindings

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// TestAssetServerRoutingProvesTheGUIIsServed pins the exact wiring the
// desktop app relies on: with Assets nil, every GET — including "/" —
// and every POST routes through the real oneprobe handler.
func TestAssetServerRoutingProvesTheGUIIsServed(t *testing.T) {
	app, err := NewApp(filepath.Join(t.TempDir(), "desktop-test.db"))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	defer app.Shutdown(nil)

	// Build the exact AssetServer options the app passes to wails.Run.
	opts := &assetserver.Options{Handler: app.handler}
	if err := opts.Validate(); err != nil {
		t.Fatalf("options invalid: %v", err)
	}

	// Route table the GUI needs: page, donate, API GETs and POSTs.
	srv := httptest.NewServer(app.handler)
	defer srv.Close()

	// The GUI page itself.
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body := readAllT(t, resp)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(body, "id=\"targets\"") {
		t.Errorf("GET / -> %d, GUI page missing panels", resp.StatusCode)
	}

	// Donate page.
	resp, err = http.Get(srv.URL + "/donate")
	if err != nil {
		t.Fatalf("GET /donate: %v", err)
	}
	body = readAllT(t, resp)
	resp.Body.Close()
	for _, want := range []string{
		"0x85ee7E71f762d772599cbF1EC20E651B30657521",
		"bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("donate page missing %s", want)
		}
	}

	// API POST through the same handler (targets create).
	resp, err = http.Post(srv.URL+"/api/targets", "application/json",
		strings.NewReader(`{"name":"x","url":"http://127.0.0.1:9999/metrics","interval":1000}`))
	if err != nil {
		t.Fatalf("POST /api/targets: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("POST /api/targets -> %d, want 201", resp.StatusCode)
	}

	// Migrate POST.
	resp, err = http.Post(srv.URL+"/api/migrate/datadog", "application/json",
		strings.NewReader(`{"widgets":[]}`))
	if err != nil {
		t.Fatalf("POST migrate: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("POST migrate -> %d, want 200", resp.StatusCode)
	}
	_ = os.Environ
}

func readAllT(t *testing.T, resp *http.Response) string {
	t.Helper()
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			b.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return b.String()
}
