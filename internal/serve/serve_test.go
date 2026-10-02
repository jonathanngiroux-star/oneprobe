package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	db := filepath.Join(t.TempDir(), "oneprobe.db")
	h, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { h.Close() })
	return h.Handler()
}

func TestHealthzReturnsOK(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if body := readAll(t, resp); body != "ok" {
		t.Errorf("body = %q, want \"ok\"", body)
	}
}

func TestDonateServesBothAddressesAsPlainText(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/donate")
	if err != nil {
		t.Fatalf("GET /donate: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	body := readAll(t, resp)
	for _, want := range []string{
		"0x85ee7E71f762d772599cbF1EC20E651B30657521",
		"bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/donate body missing %q", want)
		}
	}
}

func TestUnknownPathIs404(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t))
	defer srv.Close()

	// The unbuilt surface stays 404 — the catch-all must not swallow
	// unknown API paths into some other handler.
	resp, err := http.Get(srv.URL + "/api/logs")
	if err != nil {
		t.Fatalf("GET /api/logs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404 (unbuilt surface stays 404)", resp.StatusCode)
	}
}

func TestTargetsAPIIntervalSerializesAsMilliseconds(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/targets", "application/json",
		strings.NewReader(`{"name":"t","url":"http://127.0.0.1:9999/metrics","interval":2000}`))
	if err != nil {
		t.Fatalf("POST /api/targets: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, want := created["interval"], float64(2000); got != want {
		t.Errorf("interval = %v, want %v (milliseconds, not nanoseconds)", got, want)
	}
}

func TestServeCreatesSQLiteFile(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "oneprobe.db")
	_, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := readDBFile(db); err != nil {
		t.Errorf("serve must create the SQLite file at --db: %v", err)
	}
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	var b strings.Builder
	buf := make([]byte, 1024)
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

func readDBFile(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return string(raw[:6]) == "SQLite", nil
}
