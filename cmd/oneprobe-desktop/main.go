//go:build desktop || bindings

// cmd/oneprobe-desktop is the Wails desktop GUI for Oneprobe.
//
// It is a build-tagged variant because Wails requires CGO + webkit on
// Linux (and platform equivalents elsewhere), while the default
// oneprobe build stays pure Go and CGO-free for the scratch Docker
// image. The tag set: `wails build` compiles it with desktop(+production);
// `wails build`'s bindings step compiles it with just `bindings`; the
// default `go build ./...` excludes it entirely (no webkit dragged
// into the server binary). The desktop app embeds the SAME tested UI
// page and drives the SAME tested store/serve/scrape/alert code as
// `oneprobe serve` — one surface, two shells.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/jonathanngiroux-star/oneprobe/internal/serve"
)

// App is the Wails binding surface. The webview talks to the same
// in-process HTTP API the browser GUI does (via the AssetServer
// handler below), so the UI code is identical.
type App struct {
	handler http.Handler
	server  *serve.Server
}

// NewApp wires the Wails asset server to the real oneprobe HTTP API
// mux. Every /api/* route, /donate, and the GUI page itself are served
// by the same handler `oneprobe serve` uses.
func NewApp(dbPath string) (*App, error) {
	srv, err := serve.New(dbPath)
	if err != nil {
		return nil, err
	}
	return &App{handler: srv.Handler(), server: srv}, nil
}

// Shutdown stops the scrape loop, alert engine, and store.
func (a *App) Shutdown(ctx context.Context) {
	_ = a.server.Close()
}

func main() {
	dbPath := defaultDBPath()
	app, err := NewApp(dbPath)
	if err != nil {
		log.Fatalf("oneprobe-desktop: %v", err)
	}

	err = wails.Run(&options.App{
		Title:     "Oneprobe",
		Width:     1024,
		Height:    768,
		MinWidth:  720,
		MinHeight: 520,
		AssetServer: &assetserver.Options{
			// Assets is nil: every GET (and all non-GETs) route through
			// the real oneprobe handler — the GUI page, /api/*, and
			// /donate are the same code the browser GUI uses.
			Handler: app.handler,
		},
		OnStartup: func(ctx context.Context) {
			app.startTestBridge(ctx) // no-op unless ONEPROBE_GUI_TESTBRIDGE=1
		},
		OnShutdown: func(ctx context.Context) { app.Shutdown(ctx) },
		Bind:       []interface{}{app},
	})
	if err != nil {
		log.Fatalf("oneprobe-desktop: %v", err)
	}
}

// defaultDBPath picks the SQLite location: $ONEPROBE_DB, else
// ~/.config/oneprobe/oneprobe.db (per-user desktop convention). The
// parent directory is created so a first run (and wails' bindings
// binary, which executes main()) works out of the box.
func defaultDBPath() string {
	if p := os.Getenv("ONEPROBE_DB"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "oneprobe.db"
	}
	p := filepath.Join(home, ".config", "oneprobe", "oneprobe.db")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "oneprobe.db"
	}
	return p
}
