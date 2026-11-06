# SPDX-License-Identifier: Apache-2.0
import importlib.util
import json
import pathlib
import tempfile
import types
import unittest

spec = importlib.util.spec_from_file_location("macshard", pathlib.Path(__file__).with_name("shard.py"))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


def package(name, module="example", root="/repo"):
    return {"package": module + "/" + name, "module": module, "dir": root + "/" + name}


class DarwinShards(unittest.TestCase):
    def test_CINV04625_balanced_complete_deterministic_partition(self):
        packages = [package(str(i)) for i in range(17)]
        weights = {p["package"]: (18 - i) * 1000 for i, p in enumerate(packages)}
        bins, loads = m.partition(packages, weights)
        self.assertEqual(m.partition(list(reversed(packages)), weights), (bins, loads))
        self.assertEqual(sorted(p["package"] for b in bins for p in b), sorted(weights))
        self.assertEqual(len({p["package"] for b in bins for p in b}), len(packages))
        self.assertLessEqual(max(loads) - min(loads), max(weights.values()))
        # The four most expensive packages start on distinct runners.
        for i in range(4):
            self.assertEqual(bins[i][0], packages[i])
        self.assertEqual(m.partition([], weights), ([[], [], [], []], [0, 0, 0, 0]))
        with self.assertRaises(ValueError):
            m.partition([packages[0], packages[0]], weights)

    def test_CINV04625_unknown_and_unreadable_costs_cannot_remove_packages(self):
        packages = [package(str(i)) for i in range(7)]
        for weights in ({}, {"example/0": 1000}):
            bins, _ = m.partition(packages, weights)
            self.assertEqual(sorted(p["package"] for b in bins for p in b),
                             sorted(p["package"] for p in packages))
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "costs.json"
            self.assertEqual(m.costs(path), {})
            for record in ({}, [], {"profile": "bad", "milliseconds": {"x": 1}},
                           {"profile": "corvint-ci-package-costs/0", "milliseconds": {"x": 0}},
                           {"profile": "corvint-ci-package-costs/0", "milliseconds": {"x": True}}):
                path.write_text(json.dumps(record))
                self.assertEqual(m.costs(path), {})
        self.assertTrue(m.costs(".github/cishards/package-costs.json"))

    def test_CINV04625_one_concurrent_invocation_per_module_and_continue_after_failure(self):
        packages = [package("p"), package("q"), package("sub/p", "nested", "/repo/nested")]
        calls = []
        def runner(command, **kwargs):
            calls.append((command, kwargs))
            kwargs["stdout"].write("retained test output\n")
            return types.SimpleNamespace(returncode=1 if len(calls) == 1 else 0)
        with tempfile.TemporaryDirectory() as tmp:
            out = pathlib.Path(tmp)
            self.assertEqual(m.run(packages, out, pathlib.Path("/repo"), runner), 1)
            self.assertEqual(len(calls), 2)
            self.assertEqual(calls[0][0][-2:], ["example/p", "example/q"])
            self.assertEqual(calls[0][1]["cwd"], pathlib.Path("/repo"))
            self.assertEqual(calls[1][1]["cwd"], pathlib.Path("/repo/nested"))
            for command, kwargs in calls:
                self.assertEqual(command[:4], ["bash", "/repo/script/heavy.sh", "go", "test"])
                self.assertEqual(command[command.index("-p") + 1], "3")
                self.assertNotIn("-run", command)
                self.assertNotIn("-race", command)
                self.assertEqual(kwargs["env"]["HEAVY_REMOTE"], "0")
            self.assertEqual((out / "module-0.jsonl").read_text(), "retained test output\n")
            self.assertEqual(m.run([], out, pathlib.Path("/repo"), runner), 0)
            self.assertEqual(len(calls), 2)

    def test_CINV04625_package_arguments_and_module_roots(self):
        self.assertEqual(m.module_groups([{"package": "example", "module": "example", "dir": "/repo"}]),
                         {pathlib.Path("/repo"): ["example"]})
        with self.assertRaises(ValueError):
            m.module_groups([{"package": "-flag", "module": "example", "dir": "/repo"}])
        for name in ("bad\nname", "../bad", "a//b"):
            with self.assertRaises(ValueError):
                m.module_groups([package(name)])
        with self.assertRaises(ValueError):
            m.module_groups([{"package": "example/p", "module": "other", "dir": "/repo/p"}])
        with self.assertRaises(ValueError):
            m.module_groups([{"package": "example/p", "module": "example", "dir": "/repo/q"}])


if __name__ == "__main__":
    unittest.main()
