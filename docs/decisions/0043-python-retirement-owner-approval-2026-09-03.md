# Decision 0043 — owner approval for staged Python retirement is recorded

Date: 2026-09-03. Status: accepted. Authority: repository owner, verbatim instructions "I want the
current code to be the new oracle. the old python can go away" and "record the approval decision"
(2026-09-03).

## What is recorded

`GPK-V0-025` gates removal of the Python source and its CI lane on four conditions, the last of
which is "owner approval is recorded". This record satisfies that fourth condition and nothing else.
The owner has approved staged Python retirement as the intended direction and has stated that Go is
to become the reference implementation.

`GPK-V0-036` already makes retirement **per command**. This approval is therefore standing, not
per-command: it does not need to be re-sought for each command, and it does not admit any command on
its own.

## What this record does not authorize

It does not remove `src/`, and it does not permit removing it. The other three `GPK-V0-025`
conditions are objective and unmet at this revision:

1. every accepted wire has non-Python conformance authority,
2. every CLI row has a Go negative/adversarial suite,
3. rollback no longer depends on Python-generated state.

Nor are the staged preconditions met. `GPK-V0-025` requires Go to be default for one release while
Python remains a shipped fallback, and only then permits Python to become a development-only oracle,
after two consecutive release candidates, at least 100 Corvint changes, and at least 20 Beamfall tasks
show zero unexplained parity mismatch and zero Go-only critical miss. None of those windows has
opened. `internal/betarung/admissions.json` admits no command to the `GPK-V0-042` beta rung at this
revision, and retirement sits above beta in `GPK-V0-023`'s order: the ladder's first rung is not yet
cleared, so its last cannot be.

`GPK-V0-025` also holds the window shut "while `conformance/divergence-register.md` holds an open
entry for any command in the supported slice". One entry is OPEN — `DR-0014`, `frontier`/TCQ — with
a repair in flight but not yet gated. Until that lands the window cannot advance regardless of this
approval.

## The evidence that motivates retirement

The register is the argument. Of 21 entries, 5 are CLOSED and 1 is OPEN; 15 are LANDED, meaning
permanently known-divergent with `src/` frozen and never repaired. Of those 15, **11 are adjudicated
`python-defect`** — `DR-0001`, `DR-0002`, `DR-0008`, `DR-0009`, `DR-0011`, `DR-0012`, `DR-0015`,
`DR-0016`, `DR-0017`, `DR-0021`, and `DR-0018` as re-adjudicated by decision 0039. Three are
`spec-gap` (`DR-0003`, `DR-0004`, `DR-0007`) and one is a `go-defect` (`DR-0010`).

Eleven of fifteen permanent divergences are cases where the oracle is known wrong and cannot be
fixed. Every one of them is a place where the parity apparatus must treat a wrong answer as
authoritative. That is the standing cost this direction removes.

## The framing this record corrects

"Go becomes the oracle" is not what `GPK-V0-033` permits, and adopting it literally would lose the
property that makes parity worth running. `GPK-V0-033` already states that the conformance authority
is **the frozen spec, not the Python implementation**. Python is the reference implementation that
produces expected bytes; it was never the authority.

If Go's current output simply becomes the expectation, every expectation is true by construction and
no Go regression is detectable — the same blindness that let eleven Python defects sit behind a green
suite. The admissible form of this transition is therefore to replace oracle-captured expectations
with **spec-authored** expectations, clause by clause, so the expectation continues to come from a
source independent of the implementation being tested. `GPK-V0-033`'s prohibition on resolving a
divergence toward the candidate stays in force for as long as any expectation is oracle-captured;
retiring it wholesale, rather than per re-authored command, is not authorized by this record and
would require its own amendment.

## Rollback

Delete this file. No code, spec, manifest, or conformance artifact changes with it, and no gate
consults it: it is the recorded answer to one `GPK-V0-025` condition. Withdrawing the approval
restores the prior state exactly.
