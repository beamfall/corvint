# SPDX-License-Identifier: Apache-2.0
import importlib.util
import pathlib
import os
import subprocess
import tempfile
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

    def test_CINV04625_indirect_build_constraints_and_switch_cases(self):
        cases = {
            "unix": ("//go:build unix", "darwin-build"),
            "not_linux": ("//go:build !linux", "darwin-build"),
            "not_windows": ("//go:build !windows", "darwin-build"),
            "bsd": ("//go:build bsd", "darwin-build"),
            "bsd_family": ("//go:build freebsd || netbsd || openbsd || darwin", "darwin-build"),
            "legacy_unix": ("// +build unix", "darwin-build"),
            "legacy_not_linux": ("// +build !linux", "darwin-build"),
            "legacy_not_windows": ("// +build !windows", "darwin-build"),
            "legacy_bsd": ("// +build bsd", "darwin-build"),
            "custom_constraint": ("//go:build cgo && go1.27", "darwin-build"),
            "switch": ('switch runtime.GOOS { case "darwin": }', "darwin-branch"),
            "switch_multiple": ('switch runtime.GOOS { case "linux", "darwin": }', "darwin-branch"),
            "switch_multiline": ('switch runtime.GOOS {\ncase "freebsd",\n "darwin": }', "darwin-branch"),
        }
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            for case, (source, reason) in cases.items():
                with self.subTest(case=case):
                    (root / "platform.go").write_text(source)
                    pkg = {"Dir": tmp, "ImportPath": "example/" + case,
                           "Module": {"Path": "example"}, "GoFiles": ["platform.go"]}
                    selected = m.inventory([pkg])
                    self.assertEqual(selected[0]["package"], pkg["ImportPath"])
                    self.assertEqual(selected[0]["reasons"], ["platform.go:" + reason])

    def record(self):
        return {"profile": "corvint-pr-narrowing/0", "mode": "NARROWED", "target": "target",
                "head": "head", "runId": 42, "shard": 0, "shards": 1, "reasons": [],
                "selected": ["example/p"], "executed": ["example/p"]}

    def test_CINV04625_fail_closed(self):
        self.assertEqual(m.narrow(self.record(), "target", "head", "42"), {"example/p"})
        for field, value in {"profile": "bad", "mode": "FULL", "target": "other", "head": "other",
                             "runId": 43, "shard": 1, "shards": 16, "reasons": ["unreadable"],
                             "executed": [], "selected": None}.items():
            with self.subTest(field=field):
                record = self.record()
                record[field] = value
                self.assertIsNone(m.narrow(record, "target", "head", "42"))

    def test_CINV04625_nested_module_and_empty_selection(self):
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

    def test_CINV04625_workflow_wiring(self):
        ci = pathlib.Path(".github/workflows/ci.yml").read_text()
        workflow = pathlib.Path(".github/workflows/go-macos.yml").read_text()
        job = workflow.split("  go-macos-shard:\n")[1].split("\n  go-macos:\n")[0]
        self.assertIn("runs-on: macos-15", job)
        self.assertIn("needs.narrowing-verdict.outputs.verdict == 'MET'", job)
        self.assertIn('sh "$TRUSTED/.github/forward-rows/narrow.sh"', job)
        self.assertIn('.github/macos/shard.py', job)
        self.assertIn('script/heavy.sh', pathlib.Path('.github/macos/shard.py').read_text())
        self.assertIn('if [ "$NATIVE_NARROW_RESULT" = success ]; then', job)
        self.assertIn("github.event_name == 'pull_request'", job)
        aggregate = ci.split("  go-product:\n")[1].split("\n  ci-tested-tree-record:\n")[0]
        self.assertNotIn('go-macos', aggregate)
        self.assertNotIn('MACOS_RESULT', aggregate)
        self.assertNotIn('  go-macos:', ci)
        self.assertIn('  pull_request:', workflow)
        self.assertNotIn('workflow_run:', workflow)
        self.assertIn('name: macos-selection\n', job)
        self.assertIn('name: macos-selection-shard-${{ matrix.shard }}', job)
        self.assertIn('shard: [0, 1, 2, 3]', job)
        self.assertIn('fail-fast: false', job)
        self.assertNotIn('while IFS=', job)


    def test_CINV04625_summary_retains_actual_shard_failures(self):
        workflow = pathlib.Path(".github/workflows/go-macos.yml").read_text()
        block = workflow.split("      - name: Record the shard outcome\n")[1]
        shell = block.split("        run: |\n")[1].split("      - name:")[0]
        shell = "\n".join(line[10:] for line in shell.splitlines())
        for test, docs, docs_result, job, expected in [
                ("success", "FULL", "success", "success", "success"),
                ("skipped", "DOCS", "success", "success", "success"),
                ("failure", "FULL", "success", "failure", "failure"),
                ("skipped", "DOCS", "success", "failure", "failure"),
                ("skipped", "DOCS", "failure", "success", "failure"),
                ("skipped", "FULL", "success", "success", "failure")]:
            with self.subTest(test=test, docs=docs, job=job), tempfile.TemporaryDirectory() as tmp:
                env = dict(os.environ, RUNNER_TEMP=tmp, MACOS_SHARD="0", TEST_RESULT=test,
                           DOCS_MODE=docs, DOCS_RESULT=docs_result, JOB_RESULT=job)
                subprocess.run(["bash", "-eu", "-c", shell], env=env, check=True)
                self.assertEqual((pathlib.Path(tmp) / "macos-selection/shard-0/result.txt").read_text(),
                                 expected + "\n")


if __name__ == "__main__":
    unittest.main()
