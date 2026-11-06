<!-- SPDX-License-Identifier: Apache-2.0 -->
# azu-small merge-forward onto c5379b4

The owner requested a merge of mainline `c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514`
into lane head `a9c644a68c5a903ac2ca3944ffe6230b1184297c`, preserving both behaviours,
then a fresh binding against that mainline commit. The owner explicitly stops this round after
`dogfood-check`, without a new seal or publication. Earlier commits and seal bytes remain unchanged.

## Resolution decisions

Merge commit `2f00687fd0661d2149e20f776363660e89c42a40` preserves the lane's declared-consumer
kind and argv alongside mainline driver-target argv and CI workflow/job fields in
`cmd/corvint/affected_compact.go`. The shared selection type in
`internal/liveverify/affected/select.go` had an automatically merged duplicate `Kind` declaration;
one shared field now serves both kinds. `internal/sessiontest/plan.go` retains per-consumer
commands and excludes mainline driver units from language-adapter discovery.

The other conflicted paths were:

- `conformance/use-cases-v0/ledger.json` and
  `conformance/use-cases-v0/receipts/UC-CHANGE-CONSEQUENCE/implementation.json`: take mainline
  generated pins, then regenerate merged subject digests with `repin-use-case-receipts.sh --merged`.
  The lane's hostile-test receipt remains and its ledger pin is regenerated too.
- `docs/CAPABILITIES.md`: retain declared-consumer argv, uncovered coverage gaps and the ownership
  audit interface, output and exit conditions.
- `docs/specs/README.md`: retain the mainline table's complete entries and requirement range,
  then regenerate the table from merged spec headers.
- `docs/specs/REQUIREMENTS.tsv`: take mainline then regenerate lane requirement rows and positions.
  `make spec-requirements` owns this file, `INDEX.json` and the README table together; no separate
  spec-index target is needed. `INDEX.json` was likewise restored from mainline before regeneration.
- `docs/specs/core-compatibility-freeze-v1.md` and `docs/specs/falsifiable-packet-v0.md`:
  both sides cite identical source text at different lines; preserve the prose and hashes,
  then relocate with `check-line-citations.sh --repin`.

No newly introduced lane requirement definition collides with a mainline ID; no IDs are renumbered.
Existing shared requirements retain both sides' refinements.

## Evidence and limits

The previous independent review at `/home/russell/lanes/azu-small.review.md` approved the lane's
reader and CI consumer repairs at the pre-merge source. It does not qualify this composition.
This round retains raw merge, generator, focused-test, affected-plan, citation-review and dogfood
logs under `/home/russell/lanes/azu-small.tmp/merge-round/`; the final handback records exit codes
and final binding identity. The owner limits tests to the packages with manually resolved Go files
(`cmd/corvint`, `internal/sessiontest`, `internal/liveverify/affected`) and the specification and
line-citation checks. Wider host verification and batch CI are outside this round and remain
`NOT_RUN` here; merged interaction beyond those checks is `NOT_OBSERVED`.

## Friction and rollback

Git printed a read-only `packed-refs.lock` error during the merge commit, which nevertheless
succeeded with both requested parents. Shared Git metadata outside the permitted roots is not
changed to suppress that diagnostic. Existing old-base CEMs remain historical evidence; current
citations are relocated and reviewed against the new base, with unsupported claims kept unknown.
Rollback is an owner-directed revert of this round's commits; earlier history is never rewritten.
