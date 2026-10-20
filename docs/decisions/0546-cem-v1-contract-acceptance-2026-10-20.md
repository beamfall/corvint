# Decision 0546 — CEM 1.0 technical contract acceptance and 1.0 recorded limits

Date: 2026-10-20. Status: accepted. Authority: the owner answered these four questions directly on
2026-10-20, choosing each recommended option. The answers cover only these four questions. They
are not a standing grant, and they do not cover promoting the stable CEM profile, tagging,
publication, hosted release-gate dispatch or V1-0019/0020/0021.

## Context

A read-only audit of the open CEM 1.0 tickets (V1-0591, V1-0595, V1-0596, V1-0609) on
`origin/main` f3ca72248 found three things:
- 147 of the 211 `CEM-V1` requirements had no recorded acceptance.
- `cem-stable-v1.md` still said "technical contract proposed".
- Three 1.0 forks were open: runner-tuple breadth, the external consumer, and metadata-only diffs.

V1-0595 also brings the spec text in line with decision 0525 §6, so the accepted text no longer
promises 0.x readability or 0.x rollback.

## Decisions

1. **Technical contract accepted.** The owner accepts the `cem-stable-v1.md` technical contract,
   including every `CEM-V1` requirement that had no recorded acceptance, under the 2026-10-11
   standing auto-accept of lane requirements. Each such definition carries the marker
   "(accepted by decision 0546, 2026-10-20)". Sections still labelled "proposed" are recorded as
   accepted by this decision. Requirements superseded by later amendments keep their amended
   reading, for example `CEM-V1-019` under decision 0525 §6. Experimental companion profiles
   stay experimental: acceptance of their text does not promote them. This accepts the contract
   only. Promoting the stable profile remains with the owner (decision 0544).
2. **Runner-tuple gaps ship as recorded limits.** As in decision 0523 §3 for rc.3, 1.0 ships these
   gaps as recorded limits instead of blockers:
   - Linux and Windows tuples for the 18 runner profiles that lack them.
   - Detox devices.
   - The local TestCafe browser.
   - Storybook T/R.

   Local qualification may still close a gap before release. An unqualified tuple keeps failing
   closed as it does today.
3. **`CEM-V1-008` ships as an open recorded limit.** Only the in-repo portable consumer
   (`interop/cem01-go`) exists. Corvint 1.0 claims no independent CEM consumer and no external
   outcome evidence, consistent with `PRS-V1-007` and the V1-0014 post-1.0 disposition.
4. **V1-0609: typed refusal in 1.0, with representation after 1.0.** A diff with no text hunks
   cannot be represented in `cem/1.0`: an empty file created or deleted, or a mode-only change.
   1.0 refuses such a diff with an explicit typed code naming the path and the kind of change.
   Representation would change the wire, so it moves past 1.0, and the frozen `cem/1.0` bytes
   do not change.

## Rollback

Revert this file, its index row and the V1-0595 spec amendments by their merge commit. The
acceptance markers then return to `unrecorded`. Decisions 2 to 4 have no runtime effect beyond
the V1-0609 refusal, which reverts with its own change.
