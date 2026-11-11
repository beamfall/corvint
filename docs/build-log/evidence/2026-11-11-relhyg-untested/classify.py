#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Classify requirements that lost their last test citation (V1-5333).

Input: the "last test citation removed" lines that the TTG-V0-962 range form of
script/check-traceability-tests.sh prints, on stdin. Reads Git objects only.
Output: one TSV row per requirement on stdout:
  id, class, base_row, base_tests, tests_at_head, impl_paths_at_base, impl_paths_at_head,
  head_row_removed_marker
Classes:
  NOT_REQUIREMENT  a ticket ID (V1-NNNN) written in a first cell; not a requirement.
  RESTORE          a test the BASE row cited is still declared at HEAD; cite it again.
  RESTORE also covers a HEAD test whose name carries the compact ID (LCPV0004).
  RETIRE           no cited test, and none of the row's named implementation paths or Go
                   functions exist at HEAD.
  UNTESTED         no cited test survives, but an implementation path does (or none is named):
                   the row needs `(untested: REASON)` or a new test.
usage: classify.py BASE HEAD < range.log > classification.tsv
"""
import re
import subprocess
import sys

base, head = sys.argv[1], sys.argv[2]
line_re = re.compile(r"^\s+(\S+): last test citation removed \((\w+) ([^:]+):(\d+): (\S+)\)$")
test_re = re.compile(r"\b(Test[A-Za-z0-9_]+)")
path_re = re.compile(r"(?<![A-Za-z0-9_./-])((?:cmd|internal|script|tools|conformance|interop|pkg|apps|plugins)/[A-Za-z0-9_./-]*[A-Za-z0-9_])")
func_re = re.compile(r"`(?:[A-Za-z_][A-Za-z0-9_]*\.)?([A-Za-z_][A-Za-z0-9_]*)`")
go_keywords = {"true", "false", "nil", "experimental", "implemented", "removed", "proposed"}


def git(*args):
    return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout


def declared(rev):
    out = git("grep", "-h", "-o", "-E", r"^func (Test[A-Za-z0-9_]+)\(", rev, "--", "*_test.go")
    return {l[5:-1] for l in out.splitlines()}


def functions(rev):
    out = git("grep", "-h", "-o", "-E", r"^func (\([^)]*\) )?[A-Za-z_][A-Za-z0-9_]*", rev, "--", "*.go", ":!*_test.go")
    return {re.sub(r"^func (\([^)]*\) )?", "", l) for l in out.splitlines()}


def tree_paths(rev):
    paths = set()
    for p in git("ls-tree", "-r", "--name-only", rev).splitlines():
        paths.add(p)
        parts = p.split("/")
        for i in range(1, len(parts)):
            paths.add("/".join(parts[:i]) + "/")
            paths.add("/".join(parts[:i]))
    return paths


blobs = {}


def lines(rev, path):
    key = (rev, path)
    if key not in blobs:
        try:
            blobs[key] = git("show", f"{rev}:{path}").splitlines()
        except subprocess.CalledProcessError:
            blobs[key] = []
    return blobs[key]


def rows(rev, path, ident):
    pat = re.compile(r"(?<![A-Za-z0-9-])" + re.escape(ident) + r"(?![A-Za-z0-9])")
    found = []
    for text in lines(rev, path):
        m = re.match(r"^\s*\|([^|]*)", text)
        if m and pat.search(m.group(1)):
            found.append(text)
    return found


head_tests = declared(head)
head_tree = tree_paths(head)
base_funcs, head_funcs = functions(base), functions(head)
print("\t".join(["id", "class", "base_row", "base_tests", "tests_at_head", "impl_named",
                 "impl_at_head", "head_row_removed", "renamed_test_at_head"]))
for raw in sys.stdin:
    m = line_re.match(raw.rstrip("\n"))
    if not m:
        continue
    ident, _, path, number, _ = m.groups()
    base_rows = rows(base, path, ident)
    tests = sorted({t for r in base_rows for t in test_re.findall(r)})
    alive = [t for t in tests if t in head_tests]
    impl = sorted({p.rstrip("/.") for r in base_rows for p in path_re.findall(r)})
    funcs = sorted({f for r in base_rows for f in func_re.findall(r)
                    if not f.startswith("Test") and f not in go_keywords and f in base_funcs})
    impl_head = [p for p in impl if p in head_tree] + [f + "()" for f in funcs if f in head_funcs]
    impl = impl + [f + "()" for f in funcs]
    compact = ident.replace("-", "")
    renamed = sorted(t for t in head_tests if compact in t)
    head_rows = rows(head, path, ident)
    removed = any("(removed" in r for r in head_rows)
    if re.fullmatch(r"V1-\d+", ident):
        cls = "NOT_REQUIREMENT"
    elif alive or renamed:
        cls = "RESTORE"
    elif impl and not impl_head:
        cls = "RETIRE"
    else:
        cls = "UNTESTED"
    print("\t".join([ident, cls, f"{path}:{number}", ",".join(tests), ",".join(alive),
                     ",".join(impl), ",".join(impl_head), "yes" if removed else "no", ",".join(renamed)]))
