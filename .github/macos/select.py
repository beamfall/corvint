# SPDX-License-Identifier: Apache-2.0
"""CIN-V0-4625: native Darwin inventory, optionally intersected with AFP-V0-098.

Usage: python3 .github/macos/select.py LIST_JSON NARROW_JSON TARGET HEAD RUN_ID OUT
LIST_JSON is concatenated `go list -json ./...` output from the native root module
and the two nested modules CI already tests. OUT retains reasons and exact argv inputs.
"""
import json
import pathlib
import re
import sys

DARWIN_FILE = re.compile(r"_darwin(?:_[a-z0-9]+)?(?:_test)?\.go$")
# Conservatively retain every build constraint: unix, negated OS and custom BSD
# tags can admit Darwin without spelling darwin. Over-selection is safe.
DARWIN_TAG = re.compile(r"^//(?:go:build|\s*\+build)\s+[^\n]+", re.M)
DARWIN_BRANCH = re.compile(r'\bGOOS\s*(?:==|!=)\s*"darwin"|"darwin"\s*(?:==|!=)\s*\w+\.GOOS|\bcase[^:]*"darwin"')
PATH_ALIAS = re.compile(r"/private/var|/var/folders")


def documents(text):
    decoder = json.JSONDecoder()
    while text.strip():
        obj, end = decoder.raw_decode(text.lstrip())
        yield obj
        text = text.lstrip()[end:]


def inventory(packages):
    result = []
    for pkg in packages:
        if pkg.get("Error") or pkg.get("DepsErrors"):
            raise ValueError("go list errors: " + pkg["ImportPath"])
        reasons = []
        for name in sorted(set(sum((pkg.get(k, []) for k in
                                   ("GoFiles", "CgoFiles", "TestGoFiles", "XTestGoFiles", "IgnoredGoFiles")), []))):
            source = (pathlib.Path(pkg["Dir"]) / name).read_text()
            kinds = []
            if DARWIN_FILE.search(name) or DARWIN_TAG.search(source):
                kinds.append("darwin-build")
            if DARWIN_BRANCH.search(source):
                kinds.append("darwin-branch")
            if PATH_ALIAS.search(source):
                kinds.append("var-alias")
            reasons.extend(name + ":" + kind for kind in kinds)
        if reasons:
            result.append({"package": pkg["ImportPath"], "dir": pkg["Dir"],
                           "module": pkg["Module"]["Path"], "reasons": reasons})
    if not result:
        raise ValueError("empty Darwin inventory")
    return sorted(result, key=lambda p: p["package"])


def narrow(record, target, head, run_id):
    # Linux selection artifacts are never inputs. The caller runs the native trusted
    # narrow.sh only after the shared MET verdict; every record mismatch is FULL.
    if (record.get("profile") != "corvint-pr-narrowing/0" or record.get("mode") != "NARROWED"
            or record.get("target") != target or record.get("head") != head
            or str(record.get("runId")) != run_id or record.get("shard") != 0
            or record.get("shards") != 1 or record.get("reasons") != []):
        return None
    selected, executed = record.get("selected"), record.get("executed")
    if (not isinstance(selected, list) or not all(isinstance(p, str) for p in selected)
            or selected != sorted(set(selected)) or executed != selected):
        return None
    return set(selected)


def selection(packages, allowed):
    # The root planner does not own nested modules: never narrow them with its record.
    return [p for p in packages if allowed is None or p["module"] != "github.com/Beamfall/corvint"
            or p["package"] in allowed]


def main(args):
    listing, record_path, target, head, run_id, out = args
    packages = inventory(documents(pathlib.Path(listing).read_text()))
    try:
        record = json.loads(pathlib.Path(record_path).read_text())
        allowed = narrow(record, target, head, run_id)
    except (OSError, ValueError, TypeError, AttributeError):
        allowed = None
    chosen = selection(packages, allowed)
    report = {"profile": "corvint-macos-selection/0", "target": target,
              "mode": "FULL" if allowed is None else "NARROWED",
              "inventory": packages, "selected": chosen}
    pathlib.Path(out).write_text(json.dumps(report, indent=2) + "\n")
    for pkg in chosen:
        # go list supplies these words; reject control characters/options before the shell reads them.
        if any(c in pkg["dir"] for c in "\t\n\r") or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._~/-]*", pkg["package"]):
            raise ValueError("unsafe Go package")
        print(pkg["dir"] + "\t" + pkg["package"])


if __name__ == "__main__":
    main(sys.argv[1:])
