# SPDX-License-Identifier: Apache-2.0
"""Explicit unittest profile; loading tests happens only in this invoked process."""
import json
import sys
import unittest


class Result(unittest.TextTestResult):
    def __init__(self, *args):
        super().__init__(*args)
        self.rows = []

    def record(self, test, state, err=None):
        row = {"ID": test.id(), "Name": str(test), "State": state}
        if err is not None:
            # TRE-V0-115: an exception raised in a named test fails that test
            # with cause UNKNOWN, naming only the exception class.
            row["FailureKind"] = "UNKNOWN"
            row["Message"] = ("Error: " + err[0].__name__)[:1024]
        self.rows.append(row)

    def addSuccess(self, test):
        super().addSuccess(test)
        self.record(test, "PASSED")

    def addFailure(self, test, err):
        super().addFailure(test, err)
        self.record(test, "FAILED")

    def addError(self, test, err):
        super().addError(test, err)
        # A class or module fixture error names no single test and stays UNKNOWN.
        if isinstance(test, unittest.suite._ErrorHolder):
            self.record(test, "UNKNOWN")
        else:
            self.record(test, "FAILED", err)

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
        if err is None:
            self.record(subtest, "PASSED")
        elif issubclass(err[0], test.failureException):
            self.record(subtest, "FAILED")
        else:
            self.record(subtest, "FAILED", err)


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
