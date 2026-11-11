# Traceability gate: a retired spec's requirements named in prose elsewhere (V1-4803)

Found while merging two unpushed batches into one. TTG-V0-962 (proposed, experimental; change
`traceability-gate-upgrade`) and the retirement of `cross-audience-truth-nondivergence-v0.md`
(V1-4804) had each passed their gates alone. Together the range form failed:

```text
CATN-V0-001: last test citation removed
CATN-V0-014: last test citation removed
CATN-V0-015: last test citation removed
```

## Cause

The rule exempted a requirement cited at the merge base and not at HEAD only when no spec that is
not retired at HEAD contained the ID as text. `human-documentation-compiler-v0.md` names the three
IDs in prose (`removed by V1-4804`), so the retired spec's own requirements were reported. The
existing retired-spec fixture had one spec file and did not reach this.

## Decision

TTG-V0-962 gains one exemption: every spec file that cited the requirement at the merge base is
retired at HEAD, and no traceability row of a spec that is not retired at HEAD holds the ID in full
in its first cell. Prose does not keep a retired requirement; a checked row does, and so does a
spec that cited it at the merge base and is not retired. The rule keeps failing for a deleted spec
file whose IDs are still named, since a deleted file is not a retired one.

Rejected: stripping the IDs from the prose of the other spec. It would pass the gate and lose the
record of why the code left.

Limit: a spec renamed in the same range as its retirement is not matched to its old path and
fails; the row marker `(untested: REASON)` is the route.

## Evidence

- `script/check-traceability-tests_test.sh`: three added fixtures (prose passes; a checked row
  fails; a second citing spec that is not retired fails, named by its own row). Removing the
  exemption, its row clause or its citing-file clause each fails the script.
- `script/check-traceability-tests.sh --range origin/main..HEAD` on the merged batch: exit 0,
  2568 requirements cited at the merge base.

The intent stays proposed: nothing here accepts TTG-V0-960 to TTG-V0-963.
