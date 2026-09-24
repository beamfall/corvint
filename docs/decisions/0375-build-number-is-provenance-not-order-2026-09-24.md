# Decision 0375: build number is provenance, not order

Date: 2026-09-24. Status: accepted. Authority: expert decision at the owner's request (release-
engineering review, 2026-09-24; ticket V1-0149). Amends decision 0314 and `PUB-V0-021` in
`docs/specs/public-release-v0.md`.

## Context

Decision 0331 (clean public history with a private archive) restarted `origin/main`'s first-parent
commit count at 1. `PUB-V0-021` stamps the build number from that count, so the restart broke the
assumption that a later build always carries a larger number: `origin/main` HEAD stamped build 12
while the already-published `0.7.0` prerelease is `Corvint 0.7.0 (build 46)`
(`docs/BUILD-LOG.md:2391@231c2812`, "V1-0017 decision 0360 / SOP-V0-003 / SOP-V0-009" entry, 2026-09-23).
Ticket V1-0149 asked the owner to decide whether build numbers should restart from the public
lineage or carry an offset.

A second, independent defect surfaced during this review: `0.7.0`'s released commit
`41f2b68934ce0d7b2ee6f0b22e31dab41ddffa25` is not on `origin/main`'s first-parent chain at all
(`git rev-list --first-parent --count` finds it 0 times on that chain, though it is a normal
ancestor). `docs/RELEASE-NOTES.md:110-111@6a77dbd2` records why: "the release source was a clean local clone of
the integration branch at that commit rather than a GitHub clone, because nothing was pushed before
tagging." The commit that actually sits at first-parent position 46 on `origin/main` is an unrelated
merge, `e9a6e5456cfb0e7dbc72482fc3c43688c4f1372a` ("Merge pull request #131 from
beamfall/claude/v1-0213-no-python"), so `origin/main` currently stamps two unrelated commits with
the identical banner `Corvint 0.7.0 (build 46)` if either were built today. That is the real defect:
a release built from a commit that was never merged onto `origin/main`'s first-parent chain, not the
act of the count restarting.

Neither defect is reachable by a consumer today. Every citing consumer checks the build number for
shape or exact equality, never order:
`conformance/release-artifact-v0/smoke.go:26@1bcee6df` and `internal/releasecandidate/candidate.go:252@ffe1e5e9`
require an exact expected banner; `internal/releasecandidate/install.go:109@0a2f615a` and
`cmd/corvint/work_executable_binding.go:73@d5b9289a` check the manifest/generated-binding shape;
`extensions/vscode/src/executable.ts:13@29803a42` and `internal/companionrelease/core_smoke.go:52@d62aa33e` match a
`(build N)` regex/prefix with no comparison across builds; `script/dogfood-check.sh:27@a5af256a` matches the
expected version's exact banner. `docs/specs/vscode-extension-v0.md:164@2ced530d` already states "the build
number is not part of the pin." `SOP-V0-003` (`docs/specs/stable-operations-v0.md:84-92@feb4322f`, amended by
decision 0360) compares upgrade/cold-index packet bytes, never build numbers, and a real N-1
lifecycle upgrade from `0.6.0 (build 90)` into the installed `0.7.0 (build 46)` passed
(`docs/BUILD-LOG.md:2374@b85d8a61`) even though the published build numbers went down (0.6.0 build 90, 0.7.0
build 46, `docs/RELEASE-NOTES.md:101,124@1528a86b`). `origin/main`'s current first-parent count already exceeds
46, so the two published builds are not even the closest collision risk going forward.

## Decision

Do not add a build-number offset, and do not otherwise change how the build number is computed.
`PUB-V0-021` is amended in place, not superseded:

1. The build number remains the first-parent commit count of the built commit on `origin/main`,
   stamped exactly as decision 0314 describes. It is monotonic only along `origin/main`'s
   first-parent chain, and only since decision 0331 restarted that chain's count.
2. The build number is neither an ordering key nor an identity key. `VERSION` (`PUB-V0-001`) orders
   releases; the released commit plus the installed executable digest identify a build. No consumer
   may be written to compare build numbers across releases to order or authenticate them.
3. A release artifact MUST be built from a commit that is on `origin/main`'s first-parent chain at
   release time (`git rev-list --first-parent origin/main` contains `FULL_COMMIT`). This is the
   actual fix: it closes the way `0.7.0` was built off-lineage and stamped a number that collided
   with a later, unrelated `origin/main` commit. `docs/RELEASE-RUNBOOK.md` gains this check before
   the tag step.

## Alternatives weighed

- *Carry a fixed offset added to the first-parent count*: rejected. Every existing consumer already
  treats the number as opaque provenance, not an order; an offset invents an ordering guarantee the
  product never promised and never enforced, adds a hand-maintained constant to keep in sync with
  the pre-reset lineage, and does nothing about the actual defect (a release built off the
  first-parent chain).
- *Make the build number monotonic across the reset by reading the private pre-reset archive*:
  rejected under decision 0331, which retains that lineage privately and explicitly keeps it out of
  the new public history's derived state ("New indexes, repository-identity bindings, change maps,
  and checks must use the new history").
- *Leave `PUB-V0-021` unchanged and only fix the runbook*: rejected; the spec text already implied an
  ordering property strongly enough to generate this ticket, so the requirement itself needs the
  provenance-not-order language, not only the process fix.

## Consequences

No wire, stamping, or consumer-check change. `docs/specs/public-release-v0.md` `PUB-V0-021` and its
amendments line are updated; `docs/RELEASE-RUNBOOK.md` gains a first-parent-membership check before
tagging, so future releases cannot repeat the `41f2b68`/`e9a6e54` collision. `docs/specs/REQUIREMENTS.tsv`
is regenerated for the amendment line. Ticket V1-0149's acceptance criterion (owner records the rule)
is satisfied by this decision and the spec amendment.

## Applied

`docs/specs/public-release-v0.md` (`PUB-V0-021` and amendments line), `docs/RELEASE-RUNBOOK.md`
(pre-tag first-parent check), `docs/specs/REQUIREMENTS.tsv` (regenerated), `docs/decisions/README.md`
(index row), `docs/BUILD-LOG.md` (this change's entry).

## Rollback

Revert this documentation commit. No persisted state, wire format, or stamped artifact depends on
the build number's ordering or on the runbook's pre-tag check; reverting restores the prior spec
text and runbook steps with no other effect.
