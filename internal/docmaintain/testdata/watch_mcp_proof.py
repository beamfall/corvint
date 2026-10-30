"""Exercise caller-supplied watch/MCP binaries; retain the actual bounded wire frames."""
import argparse
import hashlib
import json
import os
import pathlib
import selectors
import signal
import subprocess
import tempfile
import time

CAP = 1 << 20
META = {"io.modelcontextprotocol/protocolVersion": "2026-07-28",
        "io.modelcontextprotocol/clientCapabilities": {}}


def verify_pins():
    path = os.environ.get("CORVINT_CORE_PINS")
    if not path:
        return
    if os.stat(path).st_size > CAP:
        raise RuntimeError("runtime-pin-manifest-overflow")
    pins = json.loads(pathlib.Path(path).read_bytes())
    if not isinstance(pins, list) or not 0 < len(pins) <= 32:
        raise RuntimeError("invalid-runtime-pins")
    for pin in pins:
        path = pathlib.Path(pin["path"])
        before = path.lstat()
        if path.is_symlink() or not path.is_file() or before.st_size > 512 << 20:
            raise RuntimeError("invalid-runtime-file")
        digest = hashlib.sha256()
        with path.open("rb") as stream:
            opened = os.fstat(stream.fileno())
            if (before.st_dev, before.st_ino) != (opened.st_dev, opened.st_ino):
                raise RuntimeError("runtime-replaced")
            total = 0
            while True:
                chunk = stream.read(65536)
                if not chunk:
                    break
                total += len(chunk)
                if total > 512 << 20:
                    raise RuntimeError("runtime-overflow")
                digest.update(chunk)
        after = path.lstat()
        if digest.hexdigest() != pin["sha256"] or (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) != (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns):
            raise RuntimeError("runtime-drift")


def checked_output(*args, **kwargs):
    verify_pins()
    try:
        return subprocess.check_output(*args, **kwargs)
    finally:
        verify_pins()


class Child:
    def __init__(self, argv):
        verify_pins()
        self.process = subprocess.Popen(argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE, start_new_session=True)
        self.selector = selectors.DefaultSelector()
        self.output = bytearray()
        self.stderr = bytearray()
        self.total = 0
        for stream in (self.process.stdout, self.process.stderr):
            os.set_blocking(stream.fileno(), False)
            self.selector.register(stream, selectors.EVENT_READ)
        os.set_blocking(self.process.stdin.fileno(), False)

    def pump(self, timeout):
        for key, _ in self.selector.select(max(0, timeout)):
            data = os.read(key.fd, 8192)
            if not data:
                self.selector.unregister(key.fileobj)
                continue
            self.total += len(data)
            if self.total > CAP:
                raise RuntimeError("child-output-overflow")
            target = self.output if key.fileobj is self.process.stdout else self.stderr
            target.extend(data)

    def send(self, raw, deadline):
        if len(raw) > CAP:
            raise RuntimeError("request-overflow")
        pending = memoryview(raw)
        while pending:
            if time.monotonic() >= deadline:
                raise RuntimeError("request-timeout")
            try:
                pending = pending[os.write(self.process.stdin.fileno(), pending):]
            except BlockingIOError:
                self.pump(min(.01, deadline - time.monotonic()))

    def line(self, deadline):
        while b"\n" not in self.output:
            if time.monotonic() >= deadline:
                raise RuntimeError("response-timeout")
            if not self.selector.get_map():
                raise RuntimeError("incomplete-response")
            self.pump(min(.05, deadline - time.monotonic()))
        end = self.output.index(b"\n") + 1
        raw = bytes(self.output[:end])
        del self.output[:end]
        return raw

    def finish(self, timeout, expected_exit=0):
        deadline = time.monotonic() + timeout
        while self.process.poll() is None or self.selector.get_map():
            if time.monotonic() >= deadline:
                raise RuntimeError("child-exit-timeout")
            self.pump(min(.05, deadline - time.monotonic()))
        if self.process.wait() != expected_exit:
            raise RuntimeError("child-nonzero-exit")
        verify_pins()
        return bytes(self.output)

    def close(self):
        try:
            if self.process.poll() is None:
                os.killpg(self.process.pid, signal.SIGTERM)
                try:
                    self.process.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    os.killpg(self.process.pid, signal.SIGKILL)
                    self.process.wait(timeout=2)
            try:
                os.killpg(self.process.pid, 0)
            except ProcessLookupError:
                return
            os.killpg(self.process.pid, signal.SIGKILL)
            # Descendants can need a scheduler turn to be reaped after SIGKILL.
            deadline = time.monotonic() + 2
            while time.monotonic() < deadline:
                try:
                    os.killpg(self.process.pid, 0)
                except ProcessLookupError:
                    return
                time.sleep(.01)
            raise RuntimeError("descendants-remained-after-cleanup")
        finally:
            self.selector.close()
            for stream in (self.process.stdin, self.process.stdout, self.process.stderr):
                stream.close()


class MCP:
    def __init__(self, child, destination, timeout=10):
        self.child, self.destination, self.timeout = child, destination, timeout
        self.next_id = 0

    def request(self, method, arguments, name):
        self.next_id += 1
        deadline = time.monotonic() + self.timeout
        request = {"jsonrpc": "2.0", "id": self.next_id, "method": method,
                   "params": {"_meta": META, **arguments}}
        self.child.send(json.dumps(request).encode() + b"\n", deadline)
        raw = self.child.line(deadline)
        (self.destination / (name + ".json")).write_bytes(raw)
        response = json.loads(raw)
        if response.get("jsonrpc") != "2.0" or type(response.get("id")) is not int:
            raise RuntimeError("invalid-response-envelope")
        if response["id"] != self.next_id or "error" in response or "result" not in response:
            raise RuntimeError("response-id-or-result-mismatch")
        return response["result"]


def proof(args):
    root = pathlib.Path(tempfile.mkdtemp(prefix="corvint-docs-watch-proof-", dir=args.output_root))
    children = []

    def git(*arguments):
        return checked_output(["git", "-C", str(root), *arguments],
                                       stderr=subprocess.STDOUT, timeout=10).decode().strip()

    def write(path, content):
        target = root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content)

    def commit(message):
        git("add", "-A")
        git("-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", message)

    def wait_page(text, child):
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            child.pump(.05)
            if page.exists():
                if page.stat().st_size > CAP:
                    raise RuntimeError("page-overflow")
                content = page.read_text()
                if text in content:
                    return content
            if child.process.poll() is not None:
                raise RuntimeError("watch-exited-before-update")
        raise RuntimeError("automatic-update-timeout")

    try:
        write("go.mod", "module example.test/proof\n\ngo 1.27.0\n")
        write("owner.md", "# Owner\n\n## Agent digest\n- Claim: source pinned widget.\n")
        write("widget/w.go", "package widget\nfunc Initial() {}\n")
        write("docs/page.md", "# Human page\nKeep this prose.\n")
        git("init", "-q")
        commit("initial")
        page = root / "docs/page.md"
        if args.core:
            rows = [[str(path.relative_to(root)), "file", hashlib.sha256(path.read_bytes()).hexdigest()]
                    for path in sorted(root.rglob("*")) if path.is_file() and ".git" not in path.relative_to(root).parts]
            identity = {"name": "docs", "treeSha256": hashlib.sha256(json.dumps(rows, separators=(",", ":")).encode()).hexdigest(),
                        "packageSha256": hashlib.sha256((root / "go.mod").read_bytes()).hexdigest(), "lockSha256": "",
                        "configSha256": hashlib.sha256((root / "owner.md").read_bytes()).hexdigest(), "testSha256": ""}
            (root / "fixture-identity.json").write_text(json.dumps(identity) + "\n")
        watcher = Child([args.corvint, "--root", str(root), "docs", "maintain", "--page",
                         "docs/page.md", "--source", "owner.md", "--package", "widget", "--enable",
                         "--apply", "--watch", "--max-writes", "2", "--max-wall-clock", "20s"])
        children.append(watcher)
        wait_page("Initial", watcher)
        write("widget/w.go", "package widget\nfunc Changed() {}\n")
        commit("eligible change")
        maintained = wait_page("Changed", watcher)
        assert "Keep this prose." in maintained
        raw = watcher.finish(10)
        (root / "watch-receipt.json").write_bytes(raw)
        receipt = json.loads(raw)
        assert receipt["writes"] == 2 and receipt["stopped_reason"] == "max-writes", receipt
        child = Child([args.docs_mcp, "--root", str(root)])
        children.append(child)
        mcp = MCP(child, root)
        discovered = mcp.request("server/discover", {}, "mcp-discover")
        info = discovered["_meta"]["io.modelcontextprotocol/serverInfo"]
        assert info["name"] == "corvint-docs-mcp" and info["version"] == "0.1.0-experimental", info
        assert discovered["supportedVersions"] == ["2026-07-28"], discovered
        listed = mcp.request("tools/list", {}, "mcp-list")
        assert sorted(tool["name"] for tool in listed["tools"]) == ["corvint.docs_consume", "corvint.docs_draft"]
        draft = mcp.request("tools/call", {"name": "corvint.docs_draft", "arguments": {
            "source": "owner.md", "package": "widget"}}, "mcp-draft")
        assert draft["isError"] is False, draft
        markdown = draft["structuredContent"]["markdown"]
        assert markdown in maintained, "maintained block differs from exact MCP draft"
        consumed = mcp.request("tools/call", {"name": "corvint.docs_consume", "arguments": {
            "source": "owner.md", "package": "widget", "task": "Changed", "draft": markdown}}, "mcp-consume")
        assert consumed["isError"] is False, consumed
        assert consumed["structuredContent"]["validation"] == "SOURCE_REDERIVED", consumed
        if args.core:
            conflict_proof(args, root, children)
        return {"state": "PASS", "fixture": str(root), "commit": git("rev-parse", "HEAD"),
                "page_sha256": hashlib.sha256(maintained.encode()).hexdigest(), "receipt": receipt,
                "mcp_validation": consumed["structuredContent"]["validation"]}
    finally:
        failures = []
        for child in children:
            try:
                child.close()
            except Exception as error:
                failures.append(str(error))
        if failures:
            raise RuntimeError("; ".join(failures))


def conflict_proof(args, destination, children):
    root = destination / "conflict"
    root.mkdir()
    (root / "widget").mkdir()
    (root / "docs").mkdir()
    (root / "go.mod").write_text("module example.test/conflict\n\ngo 1.27.0\n")
    (root / "owner.md").write_text("# Owner\n\n## Agent digest\n- Claim: source pinned widget.\n")
    source = root / "widget/w.go"
    source.write_text("package widget\nfunc Initial() {}\n")
    page = root / "docs/page.md"
    page.write_text("# Human page\nKeep this prose.\n")
    def git(*argv):
        return checked_output(["git", "-C", str(root), *argv], timeout=10).decode().strip()
    def commit(message):
        git("add", "-A")
        git("-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", message)
    git("init", "-q")
    commit("initial conflict fixture")
    watcher = Child([args.corvint, "--root", str(root), "docs", "maintain", "--page",
                     "docs/page.md", "--source", "owner.md", "--package", "widget", "--enable",
                     "--apply", "--watch", "--max-writes", "3", "--max-wall-clock", "20s"])
    children.append(watcher)
    deadline = time.monotonic() + 10
    while "Initial" not in page.read_text():
        if time.monotonic() >= deadline:
            raise RuntimeError("conflict-initial-update-timeout")
        watcher.pump(.05)
    human = b"# Human edit owns this page\nDo not overwrite.\n"
    page.write_bytes(human)
    source.write_text("package widget\nfunc EligibleAfterHumanEdit() {}\n")
    commit("eligible source after human page edit")
    raw = watcher.finish(10, expected_exit=2)
    receipt = json.loads(raw)
    assert receipt["profile"] == "corvint-docmaintain-watch/0"
    assert receipt["stopped_reason"] == "maintenance-conflict" and receipt["writes"] == 1
    assert page.read_bytes() == human
    body = {"profile": "corvint-core-docs-conflict/0", "status": "PASS",
            "humanBeforeSha256": hashlib.sha256(human).hexdigest(),
            "humanAfterSha256": hashlib.sha256(page.read_bytes()).hexdigest(),
            "eligibleCommit": git("rev-parse", "HEAD"), "refusalExitCode": watcher.process.returncode,
            "refusalReceipt": raw.decode()}
    (destination / "conflict-receipt.json").write_text(json.dumps(body, separators=(",", ":")) + "\n")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--corvint", required=True)
    parser.add_argument("--docs-mcp", required=True)
    parser.add_argument("--output-root", required=True)
    parser.add_argument("--core", action="store_true")
    args = parser.parse_args()
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, lambda *_: (_ for _ in ()).throw(KeyboardInterrupt()))
    print(json.dumps(proof(args)))


if __name__ == "__main__":
    main()
