# SPDX-License-Identifier: Apache-2.0
"""Explicit core-invoked adapter; stdout uses existing in-process Go types.

No execution occurs on import. The core owns containment, exit status, output
caps and artifact capture. This adapter grants neither trust nor qualification.
"""
import contextlib
import inspect
import json
import pathlib
import sys
import unittest

LIMIT = 4096


def source_path(filename):
    return pathlib.Path(filename).absolute().relative_to(pathlib.Path.cwd()).as_posix()


def flatten(suite, depth=0):
    if depth > 128:
        raise ValueError("python-suite-depth")
    for test in suite:
        if isinstance(test, unittest.TestSuite):
            yield from flatten(test, depth + 1)
        else:
            yield test


class UnitResult(unittest.TextTestResult):
    def __init__(self, *args):
        super().__init__(*args)
        self.rows = {}
        self.active_test = None
        self.unproven = False

    def startTest(self, test):
        super().startTest(test)
        self.active_test = test

    def stopTest(self, test):
        super().stopTest(test)
        self.active_test = None

    def record(self, test, outcome, reason=""):
        # RSP-V0-003: certify only the active test or unittest's exact native
        # subtest form. Never infer a parent by parsing a display identity or
        # trusting a lookalike object's test_case attribute.
        if type(test) is unittest.case._SubTest and test.test_case is self.active_test:
            test = self.active_test
        if test is not self.active_test or test is None:
            self.unproven = True
            return
        ident = test.id()
        if ident not in self.rows and len(self.rows) >= LIMIT:
            raise ValueError("python-result-cap")
        previous = self.rows.get(ident, {})
        # Aggregate all subtests into one terminal parent row. Failure wins;
        # skipped work cannot become a pass, and every skip reason survives.
        rank = {"passed": 0, "skipped": 1, "failed": 2}
        if rank.get(previous.get("Outcome"), -1) > rank[outcome]:
            outcome = previous["Outcome"]
        reasons = [r for r in (previous.get("Reason", ""), reason) if r]
        reason = "\n".join(dict.fromkeys(reasons))
        self.rows[ident] = {"ID": ident, "Outcome": outcome, "Reason": reason}

    def addSuccess(self, test):
        super().addSuccess(test)
        self.record(test, "passed")

    def addFailure(self, test, err):
        super().addFailure(test, err)
        self.record(test, "failed")

    def addError(self, test, err):
        super().addError(test, err)
        self.record(test, "failed")

    def addSkip(self, test, reason):
        super().addSkip(test, reason)
        self.record(test, "skipped", str(reason))

    def addExpectedFailure(self, test, err):
        super().addExpectedFailure(test, err)
        self.record(test, "skipped", "unittest expected failure")

    def addUnexpectedSuccess(self, test):
        super().addUnexpectedSuccess(test)
        self.record(test, "failed")

    def addSubTest(self, test, subtest, err):
        super().addSubTest(test, subtest, err)
        if err is not None:
            self.record(test, "failed")


def unit_capture(purpose, names):
    loader = unittest.TestLoader()
    suite = loader.loadTestsFromNames(names) if names is not None else loader.discover(".")
    if loader.errors:
        raise ValueError("python-discovery-failed")
    tests = []
    seen = set()
    for test in flatten(suite):
        if len(tests) >= LIMIT or test.id() in seen:
            raise ValueError("python-inventory-cap-or-duplicate")
        seen.add(test.id())
        tests.append({"ID": test.id(), "Path": source_path(inspect.getsourcefile(type(test)))})
    if purpose == "inventory":
        return tests, 0
    result = unittest.TextTestRunner(resultclass=UnitResult, verbosity=2).run(suite)
    # Unknown callback forms leave passes and skips missing so core
    # reconciliation fails closed, including on its FULL retry, using the
    # existing contract. RSP-V0-003: failures already certified for the active
    # test stay, so a passing FULL retry cannot erase them.
    rows = list(result.rows.values())
    if result.unproven:
        rows = [row for row in rows if row["Outcome"] == "failed"]
    return rows, 0 if result.wasSuccessful() else 1


def pytest_capture(purpose, names):
    import pytest

    class Capture:
        def __init__(self):
            self.tests = []
            self.pending = {}
            self.results = []

        def pytest_collection_finish(self, session):
            if len(session.items) > LIMIT:
                raise ValueError("python-inventory-cap")
            self.tests = [{"ID": item.nodeid, "Path": source_path(item.path)}
                          for item in session.items]

        def pytest_runtest_logreport(self, report):
            ident = report.nodeid
            if report.failed:
                self.pending[ident] = {"ID": ident, "Outcome": "failed", "Reason": ""}
            elif self.pending.get(ident, {}).get("Outcome") != "failed":
                if report.skipped:
                    self.pending[ident] = {"ID": ident, "Outcome": "skipped", "Reason": str(report.longrepr)}
                elif report.when == "call":
                    self.pending[ident] = {"ID": ident, "Outcome": "passed", "Reason": ""}
            if report.when == "teardown" and ident in self.pending:
                if len(self.results) >= LIMIT:
                    raise ValueError("python-result-cap")
                self.results.append(self.pending.pop(ident))

    capture = Capture()
    args = ["-q", "-p", "no:cacheprovider"]
    if purpose == "inventory":
        args += ["--collect-only"]
    if names is not None:
        args += ["--"] + names
    code = int(pytest.main(args, plugins=[capture]))
    if purpose == "inventory":
        if code not in (0, 5):
            raise ValueError("python-discovery-failed")
        return capture.tests, 0
    return capture.results, code


def main():
    runner, purpose, key_json, tests_json, coverage_path = sys.argv[1:]
    key, names = json.loads(key_json), json.loads(tests_json)
    coverage = None
    with contextlib.redirect_stdout(sys.stderr):
        if purpose == "collect":
            import coverage as coverage_module
            # In-memory data: no mutable coverage database in the checkout.
            coverage = coverage_module.Coverage(data_file=None, config_file=False)
            coverage.start()
        try:
            rows, code = (pytest_capture if runner == "pytest" else unit_capture)(purpose, names)
        finally:
            if coverage is not None:
                coverage.stop()
        if coverage is not None:
            coverage.json_report(outfile=coverage_path)
    if purpose == "inventory":
        rows = {"Key": key, "Tests": rows, "Complete": True, "Reason": ""}
    print(json.dumps(rows, sort_keys=True))
    return code


if __name__ == "__main__":
    sys.exit(main())
