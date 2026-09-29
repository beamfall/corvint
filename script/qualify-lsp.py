#!/usr/bin/env python3
"""Required live CLI/MCP Go semantic qualification (TCP-V0-053, MCPV0-030).

Uses only temporary, committed modules and local dependencies. Missing gopls
fails; no installation or dependency fetch is performed. This is synthetic
semantic/transport evidence, not a paired agent-productivity evaluation.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import selectors
import shutil
import signal
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
META = {"io.modelcontextprotocol/protocolVersion": "2026-07-28",
        "io.modelcontextprotocol/clientCapabilities": {}}


def retire(process):
    if process.poll() is not None:
        return
    # The CLI/MCP signal context must retire gopls's separate process group.
    process.terminate()
    try:
        process.communicate(timeout=30)
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGKILL)
        process.communicate()
        raise RuntimeError("owned process failed graceful cleanup; descendant cleanup unproved")


def run(argv, cwd=ROOT):
    process = subprocess.Popen(argv, cwd=cwd, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, start_new_session=True)
    try:
        stdout, stderr = process.communicate(timeout=120)
    except BaseException:
        retire(process)
        raise
    if process.returncode:
        raise subprocess.CalledProcessError(process.returncode, argv, stdout, stderr)
    return stdout


def interrupted(signum, frame):
    raise KeyboardInterrupt()


def install_signal_handlers():
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, interrupted)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def fixture(root, workspace):
    files = {
        "go.mod": "module example.test/m\n\ngo 1.22\n",
        "a/a.go": 'package a\nimport "example.test/m/b"\nfunc A() int { return b.B() }\n',
        "b/b.go": 'package b\nimport "example.test/m/c"\nfunc B() int { return c.C() }\n',
        "c/c.go": 'package c\nfunc C() int { return 1 }\n',
        "d/d.go": 'package d\nimport "example.test/m/a"\nfunc D() int { return a.A() }\n',
    }
    if workspace:
        files["go.mod"] += "require example.test/extra v0.0.0\n"
        files["go.work"] = "go 1.22\nuse (\n .\n ./extra\n)\n"
        files["extra/go.mod"] = "module example.test/extra\n\ngo 1.22\n"
        files["extra/extra.go"] = 'package extra\nfunc Value() int { return 7 }\n'
        files["c/c.go"] = 'package c\nimport "example.test/extra"\nfunc C() int { return extra.Value() }\n'
    for path, text in files.items():
        target = root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)
    for args in (["init", "-q"], ["config", "user.name", "LSP Qualification"],
                 ["config", "user.email", "lsp@example.invalid"], ["add", "."],
                 ["-c", "core.hooksPath=/dev/null", "commit", "-qm", "fixture"]):
        run(["git", *args], root)


class MCP:
    def __init__(self, binary, root):
        self.process = subprocess.Popen([str(binary), "--root", str(root),
                                         "--tool-profile", "task-review-lsp"],
                                        stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.DEVNULL, start_new_session=True)
        self.selector = selectors.DefaultSelector()
        self.selector.register(self.process.stdout, selectors.EVENT_READ)
        self.buffer = b""
        self.sequence = 0

    def call(self, method, params):
        self.sequence += 1
        request = {"jsonrpc": "2.0", "id": self.sequence, "method": method,
                   "params": {"_meta": META, **params}}
        self.process.stdin.write(json.dumps(request).encode() + b"\n")
        self.process.stdin.flush()
        deadline = time.monotonic() + 60
        while b"\n" not in self.buffer:
            remaining = deadline - time.monotonic()
            if remaining <= 0 or not self.selector.select(remaining):
                raise RuntimeError("MCP response timeout")
            chunk = os.read(self.process.stdout.fileno(), 65536)
            if not chunk:
                raise RuntimeError("MCP exited without response")
            self.buffer += chunk
            if len(self.buffer) > 1048576:
                raise RuntimeError("MCP response overflow")
        line, self.buffer = self.buffer.split(b"\n", 1)
        value = json.loads(line)
        assert value.get("id") == self.sequence, value
        assert "error" not in value, value
        result = value["result"]
        assert not result.get("isError"), result
        return result

    def close(self):
        # SIGTERM reaches the server's cancellation context; it owns and retires
        # gopls's separate process group before exiting.
        retire(self.process)
        self.selector.close()
        self.process.stdin.close()
        self.process.stdout.close()


def qualify(binary, mcp_binary, root, workspace):
    fixture(root, workspace)
    args = [str(binary), "--root", str(root), "context", "--task",
            "Find the dependencies and callers of A", "--subject", "a/a.go", "--limit", "20"]
    observations = {}
    packets = {}
    for mode in ("off", "gopls"):
        start = time.monotonic()
        raw = run([*args, "--lsp", mode])
        observations[mode] = {"wallSeconds": round(time.monotonic() - start, 4),
                              "bytes": len(raw), "sha256": digest(raw)}
        packets[mode] = json.loads(raw)
    enabled = packets["gopls"]
    assert {k: v for k, v in enabled.items() if k != "external"} == packets["off"]
    external = enabled["external"]
    assert external["providers"][0]["state"] == "loaded", external
    rows = external["path_relations"]
    edges = set()
    for row in rows:
        assert row["authority"] == "external-provider", row
        relation = row["relation"]
        assert relation["from"]["blob"] and relation["to"]["blob"], row
        edges.add((relation["from"]["path"], relation["to"]["path"], relation["type"]))
    required = {("a/a.go", "b/b.go", "gopls:uses-definition"),
                ("a/a.go", "d/d.go", "gopls:referenced-by")}
    assert required <= edges, (required, edges)
    if workspace:
        assert any(to == "extra/extra.go" for _, to, _ in edges), edges
    server = MCP(mcp_binary, root)
    try:
        catalogue = server.call("tools/list", {})
        tool = next(x for x in catalogue["tools"] if x["name"] == "corvint.context")
        assert tool["inputSchema"]["properties"]["lsp"]["enum"] == ["off", "gopls"]
        arguments = {"task": "Find the dependencies and callers of A", "subject": "a/a.go", "limit": 20}
        start = time.monotonic()
        response = server.call("tools/call", {"name": "corvint.context", "arguments": {**arguments, "lsp": "gopls"}})
        observations["mcp"] = {"wallSeconds": round(time.monotonic() - start, 4),
                               "bytes": len(json.dumps(response).encode())}
        result = response["structuredContent"]
        assert result["receipt"] == enabled, "CLI/MCP packet mismatch"
        assert "BEGIN CORVINT REPOSITORY DATA" in response["content"][0]["text"]
        off = server.call("tools/call", {"name": "corvint.context", "arguments": arguments})
        assert "external" not in off["structuredContent"]["receipt"]
        (root / "c/c.go").write_text((root / "c/c.go").read_text() + "\n// dirty\n")
        dirty = server.call("tools/call", {"name": "corvint.context", "arguments": {**arguments, "lsp": "gopls"}})
        dirty_external = dirty["structuredContent"]["receipt"]["external"]
        assert dirty_external["query"]["omitted_rows"] > 0, dirty_external
        for row in dirty_external["path_relations"]:
            rel = row["relation"]
            assert rel["from"]["path"] != "c/c.go" and rel["to"]["path"] != "c/c.go", row
    finally:
        server.close()
    return {"layout": "go.work" if workspace else "module", "observations": observations,
            "semanticWitnesses": sorted(edges), "cliMcpExactPacketParity": True,
            "defaultOff": True, "dirtyFileOmitted": True,
            "fixtureCommit": run(["git", "rev-parse", "HEAD"], root).decode().strip()}


def interruption_check(binary, work):
    """Interrupt this harness while its CLI owns a real fake-server descendant."""
    root = work / "interrupt-module"
    root.mkdir()
    fixture(root, False)
    fake_dir = work / "interrupt-bin"
    fake_dir.mkdir()
    pidfile, cachefile = work / "descendant.pid", work / "descendant.cache"
    fake = fake_dir / "gopls"
    fake.write_text("#!/bin/sh\nsleep 60 &\nprintf '%s' $! > '" + str(pidfile) +
                    "'\nprintf '%s' \"$GOPLSCACHE\" > '" + str(cachefile) + "'\nwait\n")
    fake.chmod(0o755)
    child = ("import importlib.util; "
             "s=importlib.util.spec_from_file_location('qualification'," + repr(str(Path(__file__).resolve())) + "); "
             "m=importlib.util.module_from_spec(s); s.loader.exec_module(m); m.install_signal_handlers(); "
             "m.run(" + repr([str(binary), "--root", str(root), "context", "--task", "Find A callers",
                              "--subject", "a/a.go", "--lsp", "gopls"]) + ")")
    env = dict(os.environ, PATH=str(fake_dir) + os.pathsep + os.environ["PATH"])
    process = subprocess.Popen([sys.executable, "-c", child], env=env, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, start_new_session=True)
    try:
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            if pidfile.exists() and cachefile.exists() and cachefile.read_text():
                break
            if process.poll() is not None:
                raise RuntimeError("interruption fixture exited early")
            time.sleep(0.02)
        else:
            raise RuntimeError("interruption fixture never started its descendant")
        pid = int(pidfile.read_text())
        cache = Path(cachefile.read_text())
        process.terminate()
        process.communicate(timeout=15)
        try:
            os.kill(pid, 0)
        except ProcessLookupError:
            pass
        else:
            raise RuntimeError("descendant survived harness interruption")
        assert not cache.exists(), "provider cache survived harness interruption"
        return {"signal": "SIGTERM", "descendantRetired": True, "providerCacheRemoved": True}
    finally:
        retire(process)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    gopls = shutil.which("gopls")
    if not gopls:
        raise SystemExit("required live qualification: gopls is missing")
    os.environ["GOTELEMETRY"] = "off"
    with tempfile.TemporaryDirectory(prefix="corvint-lsp-qualification-") as tmp:
        work = Path(tmp)
        # Keep version probes and the live provider's disposable caches local.
        os.environ["GOPLSCACHE"] = str(work / "gopls-cache")
        binaries = [work / "corvint", work / "corvint-mcp"]
        for binary in binaries:
            run(["go", "build", "-o", str(binary), "./cmd/" + binary.name])
        report = {"profile": "corvint-lsp-qualification/0", "claim": "synthetic-semantic-and-transport-only",
                  "agentProductivity": "NOT_OBSERVED", "gopls": run([gopls, "version"]).decode().strip(),
                  "go": run(["go", "version"]).decode().strip(),
                  "candidateCommit": run(["git", "rev-parse", "HEAD"]).decode().strip(),
                  "candidateDiffSha256": digest(run(["git", "diff", "HEAD", "--"])),
                  "binarySha256": {b.name: digest(b.read_bytes()) for b in binaries}, "cases": []}
        for workspace in (False, True):
            root = work / ("workspace" if workspace else "module")
            root.mkdir()
            report["cases"].append(qualify(*binaries, root, workspace))
        report["interruption"] = interruption_check(binaries[0], work)
        Path(args.output).write_text(json.dumps(report, indent=2) + "\n")
        print(json.dumps({"outcome": "PASS", "report": args.output, "cases": len(report["cases"])}))


if __name__ == "__main__":
    install_signal_handlers()
    main()
