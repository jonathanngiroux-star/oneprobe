#!/usr/bin/env python3
"""GUI test driver for the Oneprobe in-app test bridge.

Usage:
  gui_driver.py run <suite.py>   # run a suite against a live armed app
  gui_driver.py one '{"op":"ping"}'

Contract (Proofspan pattern):
  - commands: JSONL lines in $ONEPROBE_GUI_CMD_FILE
  - results: `TEST-RESULT {json}` in $ONEPROBE_GUI_RESULT_FILE
  - offset-based reads + unique ids: no stale reads, no crossings
Nothing is mocked: real DOM handlers, real fetches to the in-process
API, real SQLite persistence.
"""
import json
import os
import sys
import time
import uuid

CMD_FILE = os.environ.get("ONEPROBE_GUI_CMD_FILE", "/tmp/oneprobe-gui-cmd.jsonl")
RESULT_FILE = os.environ.get("ONEPROBE_GUI_RESULT_FILE", "/tmp/oneprobe-gui-result.log")


class GUI:
    def __init__(self, timeout=90.0):
        self.timeout = timeout
        self._offset = 0
        self._seq = 0
        if os.path.exists(RESULT_FILE):
            self._offset = os.path.getsize(RESULT_FILE)

    def _read_fresh(self):
        try:
            size = os.path.getsize(RESULT_FILE)
        except OSError:
            return []
        if size <= self._offset:
            return []
        with open(RESULT_FILE, "rb") as f:
            f.seek(self._offset)
            data = f.read()
        self._offset += len(data)
        out = []
        for raw in data.split(b"\n"):
            line = raw.decode("utf-8", "replace").strip()
            if line.startswith("TEST-RESULT "):
                try:
                    out.append(json.loads(line[len("TEST-RESULT "):]))
                except json.JSONDecodeError:
                    pass
        return out

    def cmd(self, op, timeout=None, **fields):
        self._seq += 1
        cmd = {"id": f"c{self._seq}-{uuid.uuid4().hex[:8]}", "op": op}
        cmd.update(fields)
        with open(CMD_FILE, "a") as f:
            f.write(json.dumps(cmd) + "\n")
        deadline = time.time() + (timeout or self.timeout)
        while time.time() < deadline:
            for r in self._read_fresh():
                if r.get("id") == cmd["id"]:
                    return r
            time.sleep(0.05)
        return {"id": cmd["id"], "ok": False, "error": "timeout waiting for result"}

    def click(self, sel): return self.cmd("click", sel=sel)
    def fill(self, sel, text): return self.cmd("fill", sel=sel, text=text)
    def select(self, sel, value): return self.cmd("select", sel=sel, value=value)
    def read(self, sel, attr=None):
        return self.cmd("read", sel=sel, **({"attr": attr} if attr else {}))
    def count(self, sel): return self.cmd("count", sel=sel)
    def texts(self, sel): return self.cmd("texts", sel=sel)
    def key(self, key, event="keydown"): return self.cmd("key", key=key, event=event)
    def clipboard(self): return self.cmd("clipboard", timeout=15)
    def ping(self): return self.cmd("ping", timeout=30)

    def expect(self, label, cond, detail=""):
        ok = bool(cond)
        print(("PASS " if ok else "FAIL ") + label + (f" — {detail}" if detail and not ok else ""))
        return ok


def wait_until_bridge(gui, timeout=40):
    deadline = time.time() + timeout
    while time.time() < deadline:
        r = gui.ping()
        if r.get("ok") and r.get("value") == "pong":
            return True
        time.sleep(0.4)
    return False


def summarize(checks):
    passed = sum(1 for c in checks if c)
    print(f"\nSUITE: {passed}/{len(checks)} checks passed")
    return 0 if passed == len(checks) else 1


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    mode = sys.argv[1]
    if mode == "one":
        gui = GUI()
        cmd = json.loads(sys.argv[2])
        print(json.dumps(gui.cmd(cmd.pop("op"), **cmd), indent=2))
        return 0
    if mode == "run":
        path = sys.argv[2]
        ns = {"__name__": "suite", "__file__": path}
        with open(path) as f:
            code = compile(f.read(), path, "exec")
        exec(code, ns)
        if "suite" not in ns:
            print("suite file must define suite(gui) -> exit code")
            return 2
        gui = GUI()
        if not wait_until_bridge(gui):
            print("BRIDGE NOT UP — is the app running with ONEPROBE_GUI_TESTBRIDGE=1?")
            return 3
        return ns["suite"](gui)
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main())
