"""One-off live probe: stock OpenCode 2.x discovers and calls corvint-mcp through mcp.servers.

Foreground only: the loopback provider runs in a thread and OpenCode is a bounded subprocess.
Usage: python3 -I mcp_probe.py OPENCODE CORVINT_MCP OUTDIR
"""
import hashlib, http.server, json, os, pathlib, shutil, subprocess, sys, threading

host, mcp, out = sys.argv[1], sys.argv[2], pathlib.Path(sys.argv[3])
if out.exists():
    shutil.rmtree(out)
home, repo = out / "home", out / "repo"
for p in (home, repo):
    p.mkdir(parents=True)
(repo / "go.mod").write_text("module example.com/mcpprobe\n\ngo 1.27.2\n")
(repo / "add.go").write_text("package mcpprobe\n\nfunc Add(a, b int) int { return a + b }\n")
(repo / "AGENTS.md").write_text("# Project instructions\nRun git diff --check.\n")
git = ["git", "-C", str(repo), "-c", "user.name=probe", "-c", "user.email=probe@example.invalid"]
subprocess.run(git + ["init", "-q"], check=True, stdin=subprocess.DEVNULL)
(repo / ".git/info/exclude").write_text("opencode.json\n")
subprocess.run(git + ["add", "."], check=True, stdin=subprocess.DEVNULL)
subprocess.run(git + ["commit", "-qm", "fixture"], check=True, stdin=subprocess.DEVNULL)

requests = []
steps = [
    'return Object.keys(tools.corvint ?? {}).sort().map(k => "corvint." + k).join(",")',
    'const k = Object.keys(tools.corvint).filter(k => /status/.test(k)).sort()[0];'
    ' const r = await tools.corvint[k]({}); return JSON.stringify({tool: "corvint." + k, result: r}).slice(0, 4000)',
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
if len(sys.argv) > 5 and sys.argv[5] == "modern":
    config["mcp"] = {"servers": {"corvint": {"type": "local", "command": [mcp, "--root", str(repo)], "protocol": "2026-07-28"}}}
if len(sys.argv) > 4 and sys.argv[4] == "global":
    (home / "config/opencode").mkdir(parents=True)
    (home / "config/opencode/opencode.json").write_text(json.dumps({"mcp": config.pop("mcp")}))
(repo / "opencode.json").write_text(json.dumps(config))
env = {"PATH": os.environ["PATH"], "HOME": str(home), "XDG_CONFIG_HOME": str(home / "config"), "XDG_DATA_HOME": str(home / "data"),
       "XDG_CACHE_HOME": str(home / "cache"), "XDG_STATE_HOME": str(home / "state"), "TMPDIR": str(out), "NO_COLOR": "1"}
try:
    run = subprocess.run([host, "run", "--standalone", "--format", "json", "--model", "local/probe", "--title", "MCP probe", "Report Corvint status."],
                         cwd=repo, env=env, stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=120)
    rc, stdout, stderr = run.returncode, run.stdout, run.stderr
except subprocess.TimeoutExpired as e:
    rc, stdout, stderr = "TIMEOUT", str(e.stdout or ""), str(e.stderr or "")
server.shutdown()
(out / "requests.jsonl").write_text("".join(json.dumps(r) + "\n" for r in requests))
(out / "stdout.jsonl").write_text(stdout)
(out / "stderr.txt").write_text(stderr)


def tool_results(req):
    for m in req.get("messages", []):
        if m.get("role") == "tool":
            c = m.get("content")
            yield c if isinstance(c, str) else json.dumps(c)


version = subprocess.run([host, "--version"], capture_output=True, text=True, stdin=subprocess.DEVNULL).stdout.strip()
sha = lambda p: hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()
listed = next(iter(tool_results(requests[1])), None) if len(requests) > 1 else None
called = list(tool_results(requests[2])) if len(requests) > 2 else []
receipt = {
    "profile": "opencode-mcp-live-probe/0",
    "hostVersion": version, "hostSHA256": sha(host), "corvintMcpSHA256": sha(mcp),
    "exit": rc, "providerRequests": len(requests),
    "mcpToolsListed": listed, "statusCallResult": (called[-1][:1500] if called else None),
}
receipt["result"] = "PASS" if rc == 0 and called and '"tool":"corvint.corvint_status"' in called[-1] and '"state":"READY"' in called[-1] else "FAIL"
(out / "receipt.json").write_text(json.dumps(receipt, indent=1) + "\n")
print(json.dumps(receipt, indent=1))
