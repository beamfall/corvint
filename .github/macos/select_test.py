# SPDX-License-Identifier: Apache-2.0
import importlib.util
import pathlib
import tempfile
import subprocess
import unittest

spec = importlib.util.spec_from_file_location("macselect", pathlib.Path(__file__).with_name("select.py"))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class DarwinSelection(unittest.TestCase):
    def test_CINV04625_inventory_and_drift(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            cases = {"native_darwin_arm64.go": "package p", "tag.go": "//go:build darwin && cgo\npackage p",
                     "runtime_test.go": 'if runtime.GOOS ==\n "darwin" {}',
                     "reverse_test.go": 'if "darwin" != runtime.GOOS {}',
                     "path_test.go": '// macOS /var -> /private/var', "plain.go": "package p"}
            for name, source in cases.items():
                (root / name).write_text(source)
            pkg = {"Dir": tmp, "ImportPath": "example/p", "Module": {"Path": "example"},
                   "GoFiles": ["plain.go"], "IgnoredGoFiles": ["native_darwin_arm64.go", "tag.go"],
                   "TestGoFiles": ["runtime_test.go", "reverse_test.go", "path_test.go"]}
            reasons = m.inventory([pkg])[0]["reasons"]
            self.assertEqual(len(reasons), 5)
            self.assertIn("path_test.go:var-alias", reasons)
            # A newly introduced file changes the inventory without editing a package allowlist.
            (root / "new_test.go").write_text('if runtime.GOOS == "darwin" {}')
            pkg["TestGoFiles"].append("new_test.go")
            self.assertEqual(len(m.inventory([pkg])[0]["reasons"]), 6)

    def record(self):
        return {"profile": "corvint-pr-narrowing/0", "mode": "NARROWED", "target": "target",
                "head": "head", "runId": 42, "shard": 0, "shards": 1, "reasons": [],
                "selected": ["example/p"], "executed": ["example/p"]}

    def test_CINV04626_fail_closed(self):
        self.assertEqual(m.narrow(self.record(), "target", "head", "42"), {"example/p"})
        for field, value in {"profile": "bad", "mode": "FULL", "target": "other", "head": "other",
                             "runId": 43, "shard": 1, "shards": 16, "reasons": ["unreadable"],
                             "executed": [], "selected": None}.items():
            with self.subTest(field=field):
                record = self.record()
                record[field] = value
                self.assertIsNone(m.narrow(record, "target", "head", "42"))

    def test_CINV04626_nested_module_and_empty_selection(self):
        packages = [{"package": "root/p", "module": "github.com/Beamfall/corvint"},
                    {"package": "nested/p", "module": "nested"}]
        self.assertEqual(m.selection(packages, None), packages)
        self.assertEqual(m.selection(packages, set()), packages[1:])

    def test_CINV04625_go_list_errors_and_stream(self):
        self.assertEqual(list(m.documents(' {"x":1}\n {"x":2}\n')), [{"x": 1}, {"x": 2}])
        with self.assertRaises(ValueError):
            m.inventory([])
        with self.assertRaises(ValueError):
            m.inventory([{"ImportPath": "bad", "Error": {"Err": "failed"}}])

    def test_CINV04626_required_check_failure_and_batched_pr(self):
        ci = pathlib.Path(".github/workflows/ci.yml").read_text()
        guard = ci.split("          # CIN-V0-4626: a Darwin failure")[1].split("          # AFP-V0-027")[0]
        guard = guard[guard.index("          if "):]
        for event in ("pull_request", "merge_group", "push"):
            for result in ("success", "failure", "cancelled", "skipped", ""):
                with self.subTest(event=event, result=result):
                    proc = subprocess.run(["bash", "-e", "-c", guard], env={
                        "GITHUB_EVENT_NAME": event, "MACOS_RESULT": result, "BATCHED": "true"})
                    self.assertEqual(proc.returncode == 0, result == ("success" if event == "pull_request" else "skipped"))

    def test_CINV04626_workflow_wiring(self):
        ci = pathlib.Path(".github/workflows/ci.yml").read_text()
        job = ci.split("  go-macos:\n")[1].split("\n  go-product:\n")[0]
        self.assertIn("runs-on: macos-15", job)
        self.assertIn("needs.narrowing-verdict.outputs.verdict == 'MET'", job)
        self.assertIn('sh "$TRUSTED/.github/forward-rows/narrow.sh"', job)
        self.assertIn('script/heavy.sh', job)
        self.assertIn('if [ "$NATIVE_NARROW_RESULT" = success ]; then', job)
        self.assertIn('docs-plan, static-reuse, go-macos]', ci)
        self.assertIn('test "$MACOS_RESULT" = success', ci)
        self.assertIn('test "$MACOS_RESULT" = skipped', ci)


if __name__ == "__main__":
    unittest.main()
