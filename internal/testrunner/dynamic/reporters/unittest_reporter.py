# SPDX-License-Identifier: AGPL-3.0-or-later
"""Explicit unittest profile; loading tests happens only in this invoked process."""
import json
import sys
import unittest


class Result(unittest.TextTestResult):
    def __init__(self, *args):
        super().__init__(*args)
        self.rows = []

    def record(self, test, state):
        self.rows.append({"ID": test.id(), "Name": str(test), "State": state})

    def addSuccess(self, test):
        super().addSuccess(test)
        self.record(test, "PASSED")

    def addFailure(self, test, err):
        super().addFailure(test, err)
        self.record(test, "FAILED")

    def addError(self, test, err):
        super().addError(test, err)
        self.record(test, "UNKNOWN")

    def addSkip(self, test, reason):
        super().addSkip(test, reason)
        self.record(test, "SKIPPED")

    def addExpectedFailure(self, test, err):
        super().addExpectedFailure(test, err)
        self.record(test, "SKIPPED")

    def addUnexpectedSuccess(self, test):
        super().addUnexpectedSuccess(test)
        self.record(test, "FAILED")

    def addSubTest(self, test, subtest, err):
        super().addSubTest(test, subtest, err)
        # Subtests are distinct native outcomes; a successful parent may follow.
        self.record(subtest, "PASSED" if err is None else
                    ("FAILED" if issubclass(err[0], test.failureException) else "UNKNOWN"))


if __name__ == "__main__":
    output, *names = sys.argv[1:]
    sys.path.insert(0, ".")
    loader = unittest.defaultTestLoader
    suite = loader.loadTestsFromNames(names) if names else loader.discover(".")
    result = unittest.TextTestRunner(resultclass=Result).run(suite)
    problems = [{"Code": "collection-error", "Detail": str(e)} for e in loader.errors]
    with open(output, "x", encoding="utf8") as stream:
        json.dump({"Profile": "corvint-unittest/0", "Complete": True,
                   "Count": len(result.rows), "Tests": result.rows,
                   "Problems": problems}, stream)
    sys.exit(0 if result.wasSuccessful() else 1)
