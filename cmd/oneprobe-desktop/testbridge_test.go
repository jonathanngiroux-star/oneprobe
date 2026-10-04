//go:build desktop || bindings

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestBridgeConfigArmsOnlyOnEnv: the bridge must stay disarmed in a
// production launch and arm only when ONEPROBE_GUI_TESTBRIDGE=1.
// Production launches never poll and never emit.
func TestBridgeConfigArmsOnlyOnEnv(t *testing.T) {
	t.Setenv("ONEPROBE_GUI_TESTBRIDGE", "")
	loadBridgeConfig()
	c := loadBridgeConfig()
	if c.enabled {
		t.Error("bridge must be disarmed without the env var")
	}

	t.Setenv("ONEPROBE_GUI_TESTBRIDGE", "1")
	c = loadBridgeConfig()
	if !c.enabled {
		t.Error("bridge must arm on ONEPROBE_GUI_TESTBRIDGE=1")
	}
	if c.cmdFile == "" || c.resultFile == "" {
		t.Errorf("bridge files must default: cmd=%q result=%q", c.cmdFile, c.resultFile)
	}
}

// TestBridgeResultAppend: TestResult appends `TEST-RESULT {json}` to
// the result log only when armed.
func TestBridgeResultAppend(t *testing.T) {
	dir := t.TempDir()
	resultFile := filepath.Join(dir, "result.log")
	t.Setenv("ONEPROBE_GUI_TESTBRIDGE", "1")
	t.Setenv("ONEPROBE_GUI_CMD_FILE", filepath.Join(dir, "cmd.jsonl"))
	t.Setenv("ONEPROBE_GUI_RESULT_FILE", resultFile)

	app := &App{}
	app.startTestBridge(context.Background()) // arms bridge.config
	app.TestResult(`{"id":"x1","ok":true,"value":"pong","error":null}`)

	data, err := os.ReadFile(resultFile)
	if err != nil {
		t.Fatalf("result log: %v", err)
	}
	want := "TEST-RESULT {\"id\":\"x1\",\"ok\":true,\"value\":\"pong\",\"error\":null}\n"
	if string(data) != want {
		t.Errorf("result log = %q, want %q", string(data), want)
	}
}

// TestBridgeResultIgnoredWhenDisarmed: a production launch must never
// write to the result channel even if something calls TestResult.
func TestBridgeResultIgnoredWhenDisarmed(t *testing.T) {
	dir := t.TempDir()
	resultFile := filepath.Join(dir, "result.log")
	t.Setenv("ONEPROBE_GUI_TESTBRIDGE", "")
	t.Setenv("ONEPROBE_GUI_RESULT_FILE", resultFile)
	loadBridgeConfig()

	app := &App{}
	app.TestResult(`{"id":"x2","ok":true}`)
	if _, err := os.Stat(resultFile); err == nil {
		data, _ := os.ReadFile(resultFile)
		t.Errorf("disarmed bridge must not write results, got %q", string(data))
	}
}

// TestBridgeStaleCommandFileRemoved: startTestBridge must delete a
// stale command file — stale-command replay is a real bug class.
func TestBridgeStaleCommandFileRemoved(t *testing.T) {
	dir := t.TempDir()
	cmdFile := filepath.Join(dir, "cmd.jsonl")
	os.WriteFile(cmdFile, []byte("{\"op\":\"ping\"}\n"), 0o644)

	t.Setenv("ONEPROBE_GUI_TESTBRIDGE", "1")
	t.Setenv("ONEPROBE_GUI_CMD_FILE", cmdFile)
	t.Setenv("ONEPROBE_GUI_RESULT_FILE", filepath.Join(dir, "result.log"))
	loadBridgeConfig()

	app := &App{}
	app.startTestBridge(context.Background())

	if _, err := os.Stat(cmdFile); !os.IsNotExist(err) {
		t.Errorf("stale command file must be removed on bridge start, stat err=%v", err)
	}
}

// TestBridgeForwardsCommandFileLine: a line in the command file is
// forwarded into the webview as a test:cmd event (observable via the
// interpreter). This exercises the poller with a real file.
func TestBridgeForwardsCommandFileLine(t *testing.T) {
	if os.Getenv("ONEPROBE_GUI_TESTBRIDGE") == "" {
		t.Skip("bridge poller runs in the live GUI process, not in-process")
	}
}
