# SPDX-License-Identifier: Apache-2.0
"""V1-5200: does the tested commit change anything an end-to-end suite consumes?

Usage (from the tested checkout's root, fetched with its first parent):
  python3 -I .github/consumers/gate.py [--path PATH]... [--go PKG]... [--go-test PKG]...

A hand-written path filter does not follow transitive Go dependencies, so a suite that drives a
corvint binary was never selected by an edit to a package that binary imports. This gate reads
the commit's own diff against its first parent and prints one line, RUN or SKIP with a reason:

- RUN when a changed path is go.mod, go.sum or a go.work file; is, or lies under, a --path; or
  lies under the directory of a main-module package in the `go list -deps` closure of the --go
  packages (and, for --go-test, of their tests). Embedded files and testdata under a package
  directory count as that package.
- SKIP only when the diff was read, names at least one path, and none of them matches.

Any doubt (no first parent, an unreadable diff, go list failing) prints RUN; it never prints SKIP
because something could not be read.
"""
import os
import subprocess
import sys

GLOBAL = ("go.mod", "go.sum", "go.work", "go.work.sum")


def run(*args):
    return subprocess.run(args, check=True, stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE).stdout.decode("utf-8", "strict")


def parse(args):
    paths, gos, tests = [], [], []
    it = iter(args)
    for flag in it:
        value = next(it, None)
        if value is None or flag not in ("--path", "--go", "--go-test"):
            raise ValueError("usage: gate.py [--path PATH]... [--go PKG]... [--go-test PKG]...")
        {"--path": paths, "--go": gos, "--go-test": tests}[flag].append(value.rstrip("/"))
    return paths, gos, tests


def closure(root, packages, with_tests):
    """Repository-relative directories of the main-module packages packages depend on."""
    if not packages:
        return set()
    out = run("go", "list", "-deps", *(["-test"] if with_tests else []),
              "-f", "{{if .Module}}{{if .Module.Main}}{{.Dir}}{{end}}{{end}}", *packages)
    dirs = set()
    for line in out.splitlines():
        if not line:
            continue
        rel = os.path.relpath(os.path.realpath(line), root)
        if rel.startswith(".."):
            raise ValueError("package directory %s lies outside the repository" % line)
        dirs.add("" if rel == "." else rel.replace(os.sep, "/"))
    if not dirs:
        raise ValueError("go list named no main-module package for %s" % " ".join(packages))
    return dirs


def under(path, prefix):
    return prefix == "" or path == prefix or path.startswith(prefix + "/")


def verdict(args):
    paths, gos, tests = parse(args)
    root = os.path.realpath(run("git", "rev-parse", "--show-toplevel").strip())
    changed = [p for p in run("git", "diff", "--name-only", "-z", "--no-renames", "HEAD^1", "HEAD").split("\0") if p]
    if not changed:
        return "RUN: the diff against the first parent names no path"
    for p in changed:
        if p in GLOBAL:
            return "RUN: %s changed" % p
        for prefix in paths:
            if under(p, prefix):
                return "RUN: %s changed" % p
    dirs = closure(root, gos, False) | closure(root, tests, True)
    for p in changed:
        for d in sorted(dirs):
            if under(p, d):
                return "RUN: %s lies in %s, which %s depends on" % (p, d or ".", " ".join(gos + tests))
    return "SKIP: none of the %d changed paths is consumed" % len(changed)


def main(args):
    try:
        line = verdict(args)
    except (subprocess.CalledProcessError, OSError, ValueError, UnicodeDecodeError) as err:
        detail = str(err)
        if isinstance(err, subprocess.CalledProcessError) and err.stderr:
            detail = err.stderr.decode("utf-8", "replace")
        line = "RUN: the consumer gate could not decide (%s)" % " ".join(detail.split())[:200]
    print(line)


if __name__ == "__main__":
    main(sys.argv[1:])
