//go:build desktop || bindings

// In-app test bridge for the Oneprobe desktop GUI (the Proofspan
// pattern). When the app is launched with ONEPROBE_GUI_TESTBRIDGE=1,
// a goroutine polls a JSONL command file (~100ms) and forwards each
// line into the webview as a "test:cmd" event. The frontend
// interpreter (served at /testbridge.js, injected only when armed)
// executes the command against the real DOM — real handlers, real
// fetches to the in-process API — and returns the result through the
// TestResult binding, which appends `TEST-RESULT {json}` to a log the
// external Python driver reads with a seek offset.
//
// Why a bridge: a Wayland webview cannot be driven from outside
// (xdotool never reaches WebKitGTK; no browser driver attaches to the
// wails:// origin). The app tests itself.
//
// Security: the interpreter is a fixed switch — no eval. The bridge
// arms ONLY on the env var; production launches never poll, never
// serve the interpreter script, never emit.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	bridgeDefaultCmdFile    = "/tmp/oneprobe-gui-cmd.jsonl"
	bridgeDefaultResultFile = "/tmp/oneprobe-gui-result.log"
)

type bridgeConfig struct {
	enabled    bool
	cmdFile    string
	resultFile string
}

var bridge struct {
	mu     sync.Mutex
	config bridgeConfig
}

func loadBridgeConfig() bridgeConfig {
	c := bridgeConfig{
		enabled:    os.Getenv("ONEPROBE_GUI_TESTBRIDGE") == "1",
		cmdFile:    os.Getenv("ONEPROBE_GUI_CMD_FILE"),
		resultFile: os.Getenv("ONEPROBE_GUI_RESULT_FILE"),
	}
	if c.cmdFile == "" {
		c.cmdFile = bridgeDefaultCmdFile
	}
	if c.resultFile == "" {
		c.resultFile = bridgeDefaultResultFile
	}
	return c
}

// startTestBridge arms the bridge and begins polling the command
// file. No-op in a normal launch.
func (a *App) startTestBridge(ctx context.Context) {
	bridge.mu.Lock()
	bridge.config = loadBridgeConfig()
	c := bridge.config
	bridge.mu.Unlock()
	if !c.enabled {
		return
	}
	// stale commands from a previous run must never fire
	_ = os.Remove(c.cmdFile)
	if f, err := os.OpenFile(c.resultFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		f.Close()
	} else {
		return // result channel broken -> bridge stays off
	}
	go a.pollBridgeCommands(ctx, c)
}

// pollBridgeCommands forwards command-file lines into the webview.
// The driver keeps ONE command in flight (waits for each result), so
// read-all-then-truncate cannot drop a command.
func (a *App) pollBridgeCommands(ctx context.Context, c bridgeConfig) {
	for {
		time.Sleep(100 * time.Millisecond)
		data, err := os.ReadFile(c.cmdFile)
		if err != nil || len(data) == 0 {
			continue
		}
		_ = os.Truncate(c.cmdFile, 0)
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			var cmd map[string]any
			if err := json.Unmarshal([]byte(line), &cmd); err != nil {
				a.appendBridgeResult(`{"id":"","ok":false,"error":"bad command json: ` + err.Error() + `"}`)
				continue
			}
			runtime.EventsEmit(ctx, "test:cmd", cmd)
		}
	}
}

// TestResult receives one interpreter result from the frontend and
// appends it to the result log. Bridge-only; silently ignored in a
// normal launch.
func (a *App) TestResult(resultJSON string) {
	bridge.mu.Lock()
	c := bridge.config
	bridge.mu.Unlock()
	if !c.enabled {
		return
	}
	f, err := os.OpenFile(c.resultFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "TEST-RESULT %s\n", resultJSON)
}

// appendBridgeResult is the Go-side error path (malformed command).
func (a *App) appendBridgeResult(json string) {
	bridge.mu.Lock()
	c := bridge.config
	bridge.mu.Unlock()
	if !c.enabled {
		return
	}
	if f, err := os.OpenFile(c.resultFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintf(f, "TEST-RESULT %s\n", json)
		f.Close()
	}
}
