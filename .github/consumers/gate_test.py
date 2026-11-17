# SPDX-License-Identifier: Apache-2.0
"""V1-5200: the consumer gate follows transitive Go dependencies and fails closed. Needs git and go."""
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import unittest

GATE = pathlib.Path(__file__).with_name("gate.py").resolve()
MOD = "example.invalid/m"
BASE = {
    "go.mod": "module %s\n\ngo 1.21\n" % MOD,
    "cmd/app/main.go": 'package main\n\nimport "%s/a"\n\nfunc main() { a.A() }\n' % MOD,
    "a/a.go": 'package a\n\nimport "%s/b"\n\nfunc A() { b.B() }\n' % MOD,
    "b/b.go": "package b\n\nfunc B() {}\n",
    "b/testdata/x.json": "{}\n",
    "t/t.go": "package t\n",
    "e2e/e2e.go": "package e2e\n",
    "e2e/e2e_test.go": 'package e2e\n\nimport (\n\t"testing"\n\n\t_ "%s/t"\n)\n\nfunc TestE(*testing.T) {}\n' % MOD,
    "other/o.go": "package other\n",
    "ext/index.ts": "1\n",
    "ext-other/x.ts": "1\n",
    "docs/x.md": "x\n",
}


TOOLS = bool(shutil.which("go") and shutil.which("git"))


class Tools(unittest.TestCase):
    def test_ci_has_go_and_git(self):
        # Outside CI a missing tool skips Gate; in CI (CI set) it fails here instead of skipping.
        if os.environ.get("CI") and not TOOLS:
            self.fail("CI is set but go or git is missing, so the consumer gate tests cannot run")


@unittest.skipUnless(TOOLS, "needs go and git")
class Gate(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = self.tmp.name
        self.git("init", "-q")
        self.git("config", "user.email", "ci@example.invalid")
        self.git("config", "user.name", "ci")
        self.commit(BASE)

    def tearDown(self):
        self.tmp.cleanup()

    def git(self, *args):
        subprocess.run(["git", "-C", self.repo, *args], check=True, stdout=subprocess.DEVNULL)

    def commit(self, files):
        for name, text in files.items():
            path = pathlib.Path(self.repo, name)
            if text is None:
                path.unlink()
                continue
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(text)
        self.git("add", "-A")
        self.git("commit", "-q", "--allow-empty", "-m", "c")

    def gate(self, *args):
        env = dict(os.environ, GOFLAGS="", GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local")
        run = subprocess.run([sys.executable, "-I", str(GATE), *args], cwd=self.repo, env=env,
                             stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.assertEqual(run.returncode, 0, run.stderr)
        return run.stdout.decode().strip()

    def case(self, files, *args):
        self.git("reset", "-q", "--hard", "HEAD")
        self.git("checkout", "-q", "--detach", self.base)
        self.commit(files)
        return self.gate(*args)

    def test_V15200_closure_paths_and_globals(self):
        self.base = subprocess.run(["git", "-C", self.repo, "rev-parse", "HEAD"], check=True,
                                   stdout=subprocess.PIPE).stdout.decode().strip()
        app = ("--path", "ext", "--go", "./cmd/app")
        for name, files, args, want in (
            ("transitive dependency", {"b/b.go": "package b\n\nfunc B() { _ = 1 }\n"}, app, "RUN"),
            ("direct dependency", {"a/a.go": BASE["a/a.go"] + "\n"}, app, "RUN"),
            ("testdata of a dependency", {"b/testdata/x.json": "[]\n"}, app, "RUN"),
            ("deleted dependency file", {"b/testdata/x.json": None}, app, "RUN"),
            ("the main package", {"cmd/app/main.go": BASE["cmd/app/main.go"] + "\n"}, app, "RUN"),
            ("go.mod", {"go.mod": BASE["go.mod"] + "\n"}, app, "RUN"),
            ("a --path", {"ext/index.ts": "2\n"}, app, "RUN"),
            ("unrelated package", {"other/o.go": "package other\n\n"}, app, "SKIP"),
            ("sibling of a --path", {"ext-other/x.ts": "2\n"}, app, "SKIP"),
            ("docs only", {"docs/x.md": "y\n"}, app, "SKIP"),
            ("test-only dependency, --go-test", {"t/t.go": "package t\n\n"}, ("--go-test", "./e2e"), "RUN"),
            ("test-only dependency, --go", {"t/t.go": "package t\n\n"}, ("--go", "./e2e"), "SKIP"),
            ("unlisted package dir", {"other/o.go": "package other\n\n"}, ("--go", "./missing"), "RUN"),
            ("unparsable dependency", {"a/a.go": "package a\n\nimport (\n", "docs/x.md": "y\n"}, app, "RUN"),
            ("empty diff", {}, app, "RUN"),
            ("bad arguments", {"docs/x.md": "y\n"}, ("--go",), "RUN"),
        ):
            with self.subTest(name):
                got = self.case(files, *args)
                self.assertTrue(got.startswith(want + ":"), got)

    def test_V15200_no_first_parent_runs(self):
        fresh = os.path.join(self.repo, "fresh")
        os.mkdir(fresh)
        subprocess.run(["git", "-C", fresh, "init", "-q"], check=True)
        pathlib.Path(fresh, "x").write_text("x\n")
        subprocess.run(["git", "-C", fresh, "add", "x"], check=True)
        subprocess.run(["git", "-C", fresh, "-c", "user.email=ci@example.invalid", "-c", "user.name=ci",
                        "commit", "-q", "-m", "c"], check=True)
        run = subprocess.run([sys.executable, "-I", str(GATE), "--path", "y"], cwd=fresh,
                             stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.assertTrue(run.stdout.decode().startswith("RUN:"), run.stdout)


if __name__ == "__main__":
    unittest.main()
