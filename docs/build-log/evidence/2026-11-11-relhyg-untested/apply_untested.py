#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Write `(untested: REASON)` into the traceability rows of UNTESTED requirements (V1-5333).

Reads classify.py's TSV on stdin and edits docs/specs/*.md in the working tree. For each
UNTESTED id it marks every row whose first cell holds the id and which names a BASE test (or
the first such row when none does). The reason attributes the removal to CUT when every cited
test was declared at CUT^1 and not at CUT; it names no test, since every Test* word in a row is
read as a citation.
usage: apply_untested.py CUT CUT_LABEL < classification.tsv
"""
import re
import subprocess
import sys

cut, label = sys.argv[1], sys.argv[2]


def declared(rev):
    out = subprocess.run(["git", "grep", "-h", "-o", "-E", r"^func (Test[A-Za-z0-9_]+)\(", rev,
                          "--", "*_test.go"], check=True, capture_output=True, text=True).stdout
    return {l[5:-1] for l in out.splitlines()}


before, after = declared(cut + "^1"), declared(cut)
header = sys.stdin.readline().rstrip("\n").split("\t")
edits = {}
for raw in sys.stdin:
    row = dict(zip(header, raw.rstrip("\n").split("\t")))
    if row["class"] != "UNTESTED":
        continue
    tests = [t for t in row["base_tests"].split(",") if t]
    # A row counts every Test* word as a citation, so the reason names no test.
    noun = "its last citing tests" if len(tests) > 1 else "its last citing test"
    if tests and all(t in before and t not in after for t in tests):
        why = f"{label} removed {noun}"
    else:
        why = f"{noun} left before {label}"
    if row["impl_at_head"]:
        still = "a path or function its row names remains"
    else:
        still = "whether its implementation remains is NOT_VERIFIED"
    reason = f"(untested: {why}; {still}; no test cites it now, V1-5333)"
    edits.setdefault(row["base_row"].split(":")[0], []).append((row["id"], tests, reason))

for path, items in sorted(edits.items()):
    with open(path, encoding="utf-8") as handle:
        lines = handle.read().split("\n")
    for ident, tests, reason in items:
        pat = re.compile(r"(?<![A-Za-z0-9-])" + re.escape(ident) + r"(?![A-Za-z0-9])")
        hits = [i for i, text in enumerate(lines)
                if (m := re.match(r"^\s*\|([^|]*)", text)) and pat.search(m.group(1))]
        if any("(untested:" in lines[i] for i in hits):
            continue
        chosen = [i for i in hits if any(t in lines[i] for t in tests)] or hits[:1]
        if not chosen:
            sys.exit(f"{path}: no row for {ident}")
        for i in chosen:
            text = lines[i].rstrip()
            if text.endswith("|"):
                lines[i] = text[:-1].rstrip() + " " + reason + " |"
            else:
                lines[i] = text + " " + reason
    with open(path, "w", encoding="utf-8") as handle:
        handle.write("\n".join(lines))
