# Linecov fix round 1: isolated reach and diagnostic fidelity

Base: `d436dff76de1b8e75c427069ae7f80003ae99bc2`.
Review input: lane head `bb590a7e812d4cf2ccefb04ad362a2ed0c109c22`.
Scope: the five requested findings only; the non-Go child rejection-rate finding
remains outside this round and belongs to the orchestrator.

## Decisions and corrections

For V1-4379 / LPCV-V0-7600, isolated reach is keyed by package and test before
in-order profiles are appended. Order-only reach compares to the target test's
own isolated profile. Another test's independent reach cannot remove a preceding
prerequisite. `TestInOrderWindows/shared-isolated-block` has A establish state,
B call g only with that state, C call g independently, and unrelated D. A change
to g retains A, B and C while excluding D. The unrelated original case remains.

Invalid window conversion now stops attribution for that package, preserving
`window-dump-invalid` before order or missing-window processing can overwrite it.
`TestInvalidWindowDumpReason` covers both present and absent order markers.

LPCV-V0-7600 now explicitly retains the residual that a leaked goroutine's
post-dump counts can be lost at the next reset. This is a documented limit, not
a claim of concurrent attribution safety. For V1-4421 / LPCV-V0-7606, chose to
correct the contract to `linecov.Decode`; `Load` is the Open + Decode convenience
path. No API behavior changes. Gofmt repairs the requested Ruby table comments
and Linux watch header. Spec growth relocates three unchanged-content citations;
the citation gate's own repinner updates those references.

## Verification and retained evidence

Evidence root: `/home/russell/lanes/linecov-evidence`, files prefixed `fix1-`.
The negative-control run temporarily used the prior windows implementation with
the new tests: the shared-block plan selected B and C but dropped A, and both
invalid-dump tests failed with overwritten reasons. The corrected focused tests
pass for internal/linecov, internal/rubylinecov, internal/jslinecov (all eight
runner fault witnesses) and tools/linecov. Each command uses script/heavy.sh and
one package. Linux and Windows go vet pass on each of those four packages.
Exact successful commands are in `fix1-verify.txt`; complete output is retained
in `fix1-go.log`, `fix1-ruby.log`, `fix1-js.log`, `fix1-tools.log` and the eight
`fix1-vet-*.log` files. Gofmt prints no paths for all 24 Go files changed from
BASE. `make spec-requirements` passes with no generated changes; definition and
traceability checks pass. Citation checking first detected three unchanged
moved spans; they were repinned and checked again.

## Self-use, limits and rollback

Used context, query, impact and affected against the actual lane tree. Original
receipts are retained as `fix1-context.json`, `fix1-query.json`, `fix1-impact.json`
and `fix1-affected.json`. Context reported no-specific-terms, 16 omissions and a
critical missing spec-cited path; query omitted two ranked results; impact
omitted 25. The named source, test and owning requirement were read directly.
Affected selects all four requested packages among 117 selected units, with
scope UNKNOWN and 95 unknowns (89 language-frontier, one no-selectable-test,
five unowned-dirty-path). This round's selected checks do not resolve that full
base-range frontier or claim exhaustive coverage.

Start dogfood-change ran as required; its missing final inputs remain an
expected incomplete initial pass. A new prechange receipt refuses
`prechange-after-change` because this is a repair on an already committed lane;
the original bound prechange evidence remains intact, with the repair's direct
context/query/impact receipts retained separately. First spec generation omitted
the scratch overrides and refused the unwritable host lock/log paths; rerunning
with the prescribed environment passed. Independent review, Tasks writes,
publication and seal are NOT_RUN by instruction. No new promotion or comparative
cost claim is made. Existing qualification limits remain unchanged. Final CEM
binding/checking is post-commit; the handback records its exact result.

Rollback reverts this repair and recollects coverage indexes. The earlier
package-union isolated attribution bug would return, so rollback does not retain
this round's prerequisite guarantee.
