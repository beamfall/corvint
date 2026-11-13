# SPDX-License-Identifier: Apache-2.0
"""CIN-V0-8720: authorize forgiveness after flaky.sh's diagnostic rerun checks.

Usage: python3 known-flakes.py STREAM FAILED_PACKAGES EXPECTED
KNOWN_FLAKES_REVISION must be the full immutable event-base commit from ci.yml.
The list is .github/forward-rows/known-flakes.tsv at that commit, not a working file, and
ci.yml runs this script itself from that commit (a copy outside the tested tree), so the
tested change can edit neither the list nor the rule that judges it.
Exit 0 authorizes forgiveness; any unreadable, unsupported or capped input refuses it.
This does not change the corvint-ci-flaky/0 diagnostic or rerun completeness contract.
"""

from collections import Counter
import datetime
import os
import pathlib
import re
import subprocess
import sys

LIST_PATH = ".github/forward-rows/known-flakes.tsv"
MAX_LIST = 16384
MAX_ENTRIES = 128
MAX_STREAM = 256 * 1024 * 1024
MAX_LINE = 1024 * 1024
MAX_EXPECTED_BYTES = 1024 * 1024
MAX_INVOCATIONS = 4096
MAX_PACKAGES = 8  # outcomes.sh's diagnostic rerun invocation bound


class Refusal(ValueError):
    pass


def trusted_list(revision):
    if not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise Refusal("missing or malformed event-base revision")
    env = dict(os.environ, GIT_NO_REPLACE_OBJECTS="1", GIT_OPTIONAL_LOCKS="0",
               GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull)

    def git(*args):
        return subprocess.check_output(["git", *args], env=env, timeout=10,
                                       stderr=subprocess.DEVNULL)

    if git("cat-file", "-t", revision).strip() != b"commit":
        raise Refusal("event base is not a commit")
    entry = git("ls-tree", revision, "--", LIST_PATH)
    if not re.fullmatch(rb"100644 blob [0-9a-f]{40}\t" + re.escape(LIST_PATH.encode()) + rb"\n", entry):
        raise Refusal("known-flake list is not a regular file at the event base")
    obj = revision + ":" + LIST_PATH
    if git("cat-file", "-t", obj).strip() != b"blob":
        raise Refusal("known-flake list is not a blob")
    size = int(git("cat-file", "-s", obj))
    if size > MAX_LIST:
        raise Refusal("known-flake list exceeds byte bound")
    data = git("cat-file", "blob", obj)
    if len(data) != size:
        raise Refusal("known-flake list size mismatch")
    return data


def exceptions(data, today):
    if len(data) > MAX_LIST:
        raise Refusal("known-flake list exceeds byte bound")
    if not data.endswith(b"\n") or b"\r" in data:
        raise Refusal("known-flake list is not LF terminated")
    allowed = set()
    for line in data.decode("utf-8").splitlines():
        if line.startswith("#"):
            continue
        fields = line.split("\t")
        if len(fields) != 4:
            raise Refusal("malformed known-flake entry")
        package, test, ticket, expiry = fields
        if (not re.fullmatch(r"[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+", package)
                or any(p in (".", "..") for p in package.split("/"))
                or not re.fullmatch(r"Test[^\s\x00-\x1f*?\[\]]+", test)
                or not re.fullmatch(r"V1-[0-9]{4,}", ticket)
                or not re.fullmatch(r"[0-9]{4}-[0-9]{2}-[0-9]{2}", expiry)):
            raise Refusal("malformed known-flake identity, ticket or expiry")
        try:
            expires = datetime.date.fromisoformat(expiry)
        except ValueError as exc:
            raise Refusal("invalid known-flake expiry date") from exc
        if expires <= today:
            raise Refusal("expired known-flake entry: " + package + "." + test)
        identity = (package, test)
        if identity in allowed:
            raise Refusal("duplicate known-flake identity")
        allowed.add(identity)
        if len(allowed) > MAX_ENTRIES:
            raise Refusal("known-flake list exceeds entry bound")
    return allowed


def failed_packages(path):
    with open(path, "rb") as source:
        data = source.read(MAX_LIST + 1)
    if len(data) > MAX_LIST or not data.endswith(b"\n"):
        raise Refusal("failed package list is capped or unterminated")
    packages = data.decode("utf-8").splitlines()
    if (not 0 < len(packages) <= MAX_PACKAGES or len(set(packages)) != len(packages)
            or any(not p or any(c.isspace() for c in p) for p in packages)):
        raise Refusal("failed package list is malformed or capped")
    return set(packages)


def expected_invocations(path):
    # The workflow writes one import path per scheduled package invocation, including
    # repeated slices. Certify only this closed, LF-terminated form; never infer the
    # schedule from the stream whose omissions we are checking.
    with open(path, "rb") as source:
        data = source.read(MAX_EXPECTED_BYTES + 1)
    if len(data) > MAX_EXPECTED_BYTES or not data.endswith(b"\n"):
        raise Refusal("missing failed invocation or test outcome")
    rows = data.decode("utf-8").split("\n")[:-1]
    if (not 0 < len(rows) <= MAX_INVOCATIONS
            or any(not re.fullmatch(r"[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+", p)
                   or any(part in (".", "..") for part in p.split("/")) for p in rows)):
        raise Refusal("missing failed invocation or test outcome")
    return Counter(rows)


def authorize(stream, packages, allowed, expected):
    # Attribute a test failure to each failed invocation, not just to the package as a whole.
    # An unrelated listed test cannot cover a build failure or a missing/timeout invocation.
    import json

    def unique_members(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise Refusal("duplicate test stream member")
            result[key] = value
        return result

    def nonfinite(_):
        raise Refusal("nonfinite test stream number")

    # A package may run repeatedly (slices). Only start -> complete test lifecycles ->
    # terminal package outcome is certified; no failure from another invocation carries.
    active = {}
    observed = Counter()
    failed = set()
    size = 0
    for line in iter(lambda: stream.readline(MAX_LINE + 1), b""):
        size += len(line)
        if len(line) > MAX_LINE or size > MAX_STREAM:
            raise Refusal("test stream exceeds byte bound")
        try:
            event = json.loads(line, object_pairs_hook=unique_members, parse_constant=nonfinite)
        except (ValueError, UnicodeError) as exc:
            raise Refusal("malformed test stream") from exc
        if (not isinstance(event, dict) or not isinstance(event.get("Package"), str)
                or not event["Package"]):
            raise Refusal("test stream event lacks a package")
        package, test, action = event["Package"], event.get("Test"), event.get("Action")
        if set(event) - {"Time", "Action", "Package", "Test", "Elapsed", "Output", "OutputType"}:
            raise Refusal("unsupported test stream action")
        if (("Time" in event and not isinstance(event["Time"], str))
                or ("Elapsed" in event and (type(event["Elapsed"]) not in (int, float)
                                            or event["Elapsed"] < 0))
                or (action != "output" and ("Output" in event or "OutputType" in event))
                or ("OutputType" in event and event["OutputType"] not in
                    ("frame", "error", "error-continue"))):
            raise Refusal("malformed test stream")
        if action not in ("start", "run", "pause", "cont", "pass", "fail", "output", "skip"):
            raise Refusal("unsupported test stream action")
        if test is not None and (not isinstance(test, str) or not test):
            raise Refusal("malformed test identity")
        if action == "start":
            if package not in expected or observed[package] >= expected[package]:
                raise Refusal("missing failed invocation or test outcome")
            if test is not None or package in active:
                raise Refusal("new invocation before failed package outcome")
            active[package] = {"tests": {}, "failed": False}
            continue
        if package not in active:
            raise Refusal("missing failed invocation or test outcome")
        invocation = active[package]
        tests = invocation["tests"]
        if action == "output":
            # Certify only testing's control frames and indented testing log lines.
            # Arbitrary stdout (including fatal runtime diagnostics) is unproven, even
            # when test2json emits a terminal test failure after a panic. Do not try to
            # enumerate panic/timeout/signal spellings: unknown output refuses as well.
            output = event.get("Output")
            if not isinstance(output, str) or not output.endswith("\n"):
                raise Refusal("malformed test stream")
            if test is not None:
                if tests.get(test) not in ("run", "pause"):
                    raise Refusal("missing failed invocation or test outcome")
                name = re.escape(test)
                supported = (r"(?:=== (?:RUN|PAUSE|CONT|NAME) +" + name
                             + r"\n|[ \t]*--- (?:PASS|FAIL|SKIP): " + name
                             + r" \([0-9]+\.[0-9]+s\)\n|    [^\n]*\n)")
            else:
                supported = (r"(?:PASS\n|FAIL\n|=== NAME  \n|(?:ok  |FAIL)\t"
                             + re.escape(package) + r"\t[0-9]+\.[0-9]+s(?: \[no tests to run\])?\n"
                             + r"|\?   \t" + re.escape(package) + r"\t\[no test files\]\n)")
            if re.fullmatch(supported, output) is None:
                raise Refusal("unsupported test stream action")
        elif test is not None:
            previous = tests.get(test)
            if action == "run" and previous is None:
                tests[test] = "run"
            elif (previous, action) in (("run", "pause"), ("pause", "cont")):
                tests[test] = "pause" if action == "pause" else "run"
            elif action in ("pass", "fail", "skip") and previous == "run":
                tests[test] = action
                if action == "fail":
                    if package not in packages or (package, test) not in allowed:
                        raise Refusal("unlisted test failure: " + package + "." + test)
                    invocation["failed"] = True
            else:
                raise Refusal("missing failed invocation or test outcome")
        elif action in ("fail", "pass", "skip"):
            if any(state in ("run", "pause") for state in tests.values()):
                raise Refusal("missing failed invocation or test outcome")
            if action == "fail":
                if package not in packages or not invocation["failed"]:
                    raise Refusal("failed invocation has no listed test failure: " + package)
                failed.add(package)
            elif invocation["failed"]:
                raise Refusal("test failure has no failed package outcome")
            observed[package] += 1
            del active[package]
        else:
            raise Refusal("unsupported test stream action")
    if active or failed != packages or observed != expected:
        raise Refusal("missing failed invocation or test outcome")


def main(argv):
    try:
        if len(argv) != 3:
            raise Refusal("expected STREAM, FAILED_PACKAGES and EXPECTED")
        allowed = exceptions(trusted_list(os.environ.get("KNOWN_FLAKES_REVISION", "")),
                             datetime.datetime.now(datetime.timezone.utc).date())
        packages = failed_packages(pathlib.Path(argv[1]))
        expected = expected_invocations(pathlib.Path(argv[2]))
        with open(argv[0], "rb") as stream:
            authorize(stream, packages, allowed, expected)
    except (OSError, ValueError, UnicodeError, RecursionError, subprocess.SubprocessError) as exc:
        print("CIN-V0-8720: rerun not forgiven: " + str(exc))
        return 1
    print("CIN-V0-8720: every failed invocation has a listed, unexpired test exception")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
