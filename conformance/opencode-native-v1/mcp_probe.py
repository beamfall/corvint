"""One-off live probe: stock OpenCode 2.x calls Corvint through mcp.servers or the native plugin.

Foreground only: the loopback provider runs in a thread and OpenCode is a bounded subprocess in its
own process group, which is terminated on timeout, SIGINT, SIGTERM and every exit path.
Usage: python3 -I mcp_probe.py OPENCODE CORVINT_MCP OUTDIR [global] [modern] [native=CORVINT]
OUTDIR must not exist; the probe creates it and never deletes an existing directory.
"""
import hashlib, http.server, json, os, pathlib, signal, subprocess, sys, threading

host, mcp, out = sys.argv[1], sys.argv[2], pathlib.Path(sys.argv[3])
modes = sys.argv[4:]
native = next((m.split("=", 1)[1] for m in modes if m.startswith("native=")), None)
try:
    out.mkdir(parents=False, exist_ok=False)
except FileExistsError:
    sys.exit(f"mcp_probe: refusing existing OUTDIR {out}; pass a path that does not exist")
home, repo = out / "home", out / "repo"
for p in (home, repo):
    p.mkdir()
children = []


def stop_group(proc):
    """Bounded process-tree cleanup: SIGTERM the group, then SIGKILL it after five seconds."""
    if proc.poll() is not None:
        return
    for sig, wait in ((signal.SIGTERM, 5), (signal.SIGKILL, 5)):
        try:
            os.killpg(proc.pid, sig)
        except ProcessLookupError:
            return
        try:
            proc.wait(timeout=wait)
            return
        except subprocess.TimeoutExpired:
            pass


def run_bounded(argv, timeout, **kw):
    proc = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            text=True, start_new_session=True, **kw)
    children.append(proc)
    try:
        stdout, stderr = proc.communicate(timeout=timeout)
        return proc.returncode, stdout, stderr
    except subprocess.TimeoutExpired:
        stop_group(proc)
        stdout, stderr = proc.communicate()
        return "TIMEOUT", stdout or "", stderr or ""
    finally:
        stop_group(proc)


def interrupted(signum, _frame):
    raise KeyboardInterrupt(f"signal {signum}")


signal.signal(signal.SIGINT, interrupted)
signal.signal(signal.SIGTERM, interrupted)

(repo / "go.mod").write_text("module example.com/mcpprobe\n\ngo 1.27.2\n")
(repo / "add.go").write_text("package mcpprobe\n\nfunc Add(a, b int) int { return a + b }\n")
(repo / "AGENTS.md").write_text("# Project instructions\nRun git diff --check.\n")
git = ["git", "-C", str(repo), "-c", "user.name=probe", "-c", "user.email=probe@example.invalid"]
subprocess.run(git + ["init", "-q"], check=True, stdin=subprocess.DEVNULL, timeout=60)
(repo / ".git/info/exclude").write_text("opencode.json\n")
subprocess.run(git + ["add", "."], check=True, stdin=subprocess.DEVNULL, timeout=60)
subprocess.run(git + ["commit", "-qm", "fixture"], check=True, stdin=subprocess.DEVNULL, timeout=60)

requests = []
steps = [
    'return Object.keys(tools.corvint ?? {}).sort().map(k => "corvint." + k).join(",")',
    'const k = Object.keys(tools.corvint).filter(k => /status/.test(k)).sort()[0];'
    ' const r = await tools.corvint[k]({}); return JSON.stringify({tool: "corvint." + k, result: r}).slice(0, 4000)',
]
if native:
    # The native plugin tool, not the MCP server tool of the same name.
    steps = [
        'return Object.keys(tools).filter(k => k.startsWith("corvint_")).sort().join(",")',
        'const r = await tools.corvint_status({}); return JSON.stringify({tool: "corvint_status", result: r}).slice(0, 4000)',
    ]


class Provider(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        step = len(requests)
        requests.append(body)
        if step < len(steps):
            delta = {"role": "assistant", "tool_calls": [{"index": 0, "id": f"call_{step}", "type": "function",
                     "function": {"name": "execute", "arguments": json.dumps({"code": steps[step]})}}]}
            reason = "tool_calls"
        else:
            delta, reason = {"role": "assistant", "content": "MCP_PROBE_DONE"}, "stop"
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()
        for d, f in ((delta, None), ({}, reason)):
            chunk = {"id": f"p-{step}", "object": "chat.completion.chunk", "created": 1, "model": "probe",
                     "choices": [{"index": 0, "delta": d, "finish_reason": f}]}
            self.wfile.write(f"data: {json.dumps(chunk)}\n\n".encode())
        self.wfile.write(b"data: [DONE]\n\n")


server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Provider)
threading.Thread(target=server.serve_forever, daemon=True).start()
base = f"http://127.0.0.1:{server.server_address[1]}/v1"
config = {
    "model": "local/probe",
    "providers": {"local": {"name": "MCP probe loopback provider", "package": "@opencode/ai/providers/openai-compatible",
                            "settings": {"apiKey": "local-fixture", "baseURL": base},
                            "models": {"probe": {"modelID": "probe", "capabilities": {"tools": True, "input": ["text"], "output": ["text"]},
                                                 "limit": {"context": 131072, "output": 1024}}}}},
    "mcp": {"servers": {"corvint": {"type": "local", "command": [mcp, "--root", str(repo), "--protocol-version", "2025-11-25"]}}},
}
if "modern" in modes:
    config["mcp"] = {"servers": {"corvint": {"type": "local", "command": [mcp, "--root", str(repo)], "protocol": "2026-07-28"}}}
if native:
    plugin = pathlib.Path(__file__).resolve().parents[2] / "integrations/opencode/src"
    config.pop("mcp")
    config["plugins"] = [{"package": plugin.as_uri(), "options": {"corvintBinary": native}}]
if "global" in modes and "mcp" in config:
    (home / "config/opencode").mkdir(parents=True)
    (home / "config/opencode/opencode.json").write_text(json.dumps({"mcp": config.pop("mcp")}))
(repo / "opencode.json").write_text(json.dumps(config))
env = {"PATH": os.environ["PATH"], "HOME": str(home), "XDG_CONFIG_HOME": str(home / "config"), "XDG_DATA_HOME": str(home / "data"),
       "XDG_CACHE_HOME": str(home / "cache"), "XDG_STATE_HOME": str(home / "state"), "TMPDIR": str(out), "NO_COLOR": "1"}
try:
    rc, stdout, stderr = run_bounded([host, "run", "--standalone", "--format", "json", "--model", "local/probe",
                                      "--title", "MCP probe", "Report Corvint status."], 120, cwd=repo, env=env)
    version = run_bounded([host, "--version"], 30, env=env)[1].strip()
except KeyboardInterrupt as e:
    sys.exit(f"mcp_probe: interrupted ({e}); OpenCode process group terminated")
finally:
    for proc in children:
        stop_group(proc)
    server.shutdown()
(out / "requests.jsonl").write_text("".join(json.dumps(r) + "\n" for r in requests))
(out / "stdout.jsonl").write_text(stdout)
(out / "stderr.txt").write_text(stderr)


def tool_results(req):
    for m in req.get("messages", []):
        if m.get("role") == "tool":
            c = m.get("content")
            yield c if isinstance(c, str) else json.dumps(c)


sha = lambda p: hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()
listed = next(iter(tool_results(requests[1])), None) if len(requests) > 1 else None
called = list(tool_results(requests[2])) if len(requests) > 2 else []
receipt = {
    "profile": "opencode-native-status-live-probe/0" if native else "opencode-mcp-live-probe/0",
    "hostVersion": version, "hostSHA256": sha(host),
}
if native:
    receipt["corvintSHA256"] = sha(native)
else:
    receipt["corvintMcpSHA256"] = sha(mcp)
receipt.update({"exit": rc, "providerRequests": len(requests),
                "nativeToolsListed" if native else "mcpToolsListed": listed,
                "statusCallResult": (called[-1][:1500] if called else None)})
if native:
    full = '\\"integrationSupport\\":\\"FULL\\"' in called[-1] if called else False
    receipt["result"] = "PASS" if rc == 0 and full and '"tool":"corvint_status"' in called[-1] else "FAIL"
else:
    receipt["result"] = "PASS" if rc == 0 and called and '"tool":"corvint.corvint_status"' in called[-1] and '"state":"READY"' in called[-1] else "FAIL"
(out / "receipt.json").write_text(json.dumps(receipt, indent=1) + "\n")
print(json.dumps(receipt, indent=1))
