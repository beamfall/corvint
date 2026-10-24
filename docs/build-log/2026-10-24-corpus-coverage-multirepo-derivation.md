# 2026-10-24 Corpus coverage order, multi-repository declarations, derivation 3 and whole-repository extraction

This lane works on the GitHub #1665, #1667 and #1700 re-tests of `a681846`. Its base is
`origin/main` `a681846031b87279f5f4edfe7135ce0e7986bde0`. The requirements are `DCP-V1-547..550`
and `DCP-V1-560..564` in `docs/specs/documentation-corpus-v1.md`. All are proposed and
experimental. IDs 551..553 were already taken by the peer lane in `/private/tmp/p1-scan` when
these were numbered.

## Decisions

- **Coverage is the first key inside tier `terms` (`DCP-V1-547`, V1-2575).** It is not a
  multiplier. A result holding more query words orders first, and the score breaks ties within an
  equal count. A group is one tier, one held-word count and one score, so the served `group` never
  joins results with different coverage. Before the change, the re-test's 5-word file ranked 4th
  in the reproduction fixture. After it, the file ranks 1st, and the 10-of-11 file ranks 1st for
  the long description.
- **A one-word subject is named only by a capitalized non-first word of a question
  (`DCP-V1-548`, V1-2568).** This keeps the `DCP-V1-543` guard against common words in
  statements.
- **Derived declarations cover each additional repository's declared revision (`DCP-V1-549`,
  V1-2567).** These records are new for the same multi-repository inputs, so `DerivationVersion`
  is now `3` (`DCP-V1-550`). Every derivation 2 artifact is refused until it is rebuilt with
  `corvint docs corpus build --artifact CORPUS.json`. The single-repository fixture digest is
  unchanged. Because of the bump, the reporter's #1667 cross-update readability confirmation
  restarts with this version.
- **Whole-repository extraction uses the existing adoption schema rather than shards
  (`DCP-V1-560..562`, V1-2574).** `extract --schema 2` mirrors `manifest --schema 2`. It emits a
  `corvint-corpus-adoption-provider/1` record, which a manifest `/2` already admits up to 100,000
  records and 64 MiB. The default `/1` output is byte-identical. Sharded extractor output is a
  non-goal. A record over 64 MiB is refused, and the refusal names its bytes and the bound. Each
  record-bound refusal names the count reached, the bound, the extractor and the path. A record over
  its schema's byte bound is refused at extraction instead of at build.
- **Delta and carry-forward decide the current side by flag presence (`DCP-V1-563`, V1-2543).**
  This matches `build` (`DCP-V1-495`).
- **Derivation release notes are checked by a test (`DCP-V1-564`, V1-2545).**
  `TestDCPV1564ReleaseNotesListEveryDerivation` fails until each version from 1 to the running
  derivation has a `- Derivation N: #PR` entry in `docs/RELEASE-NOTES.md`. The owner decides the
  section's final location and format.

## Evidence

Focused tests ran on WSL2 linux/amd64 with go1.27.3 through `heavy.sh`.

The 14,000-site extraction fixture (14 files of 1,000 definitions, past #1700's 13,051 sites) has
these results:
- It is refused by default with `4097 sites at extractor "handlers" path app/syms_..., over the
  bound of 4096`.
- With `--schema 2`, extraction succeeds, is deterministic, and builds through a manifest `/2`
  with 14,000 symbol subjects.
- The whole test runs in under 1 s.

The in-repository ranking fixtures stay at #1597 12/12, #1665 rows 8/8 and the leftover rows 8/8.

The reporter's frozen search and claims evaluations (general 63/64, exact-file 19/24, claims
42/52 on `a681846`) are private to the reporter, so their post-change numbers are `NOT_OBSERVED`
here. The reporter must re-test that exact-file stays at or above 20/24.

## Not delivered

- **V1-2525: the OCM 256-obligation bound still blocks `dogfood bind` for
  `documentation-corpus-v1.md`.** The spec has about 352 requirements. The bounds are at
  `internal/lrfrepo/ocm.go` and `internal/dogfoodocm/aggregate.go`. A fix needs an intent form
  that selects a `###` subsection of `## Requirements`. That means amending OCM-V0-001/003/013 and
  OIF-V0-001/009, adding a CLI `--intent-form`, and changing the dogfood intents-file syntax. The
  alternative is an owner-reviewed spec split. Neither fits this lane.
- **V1-2564: whether a Go toolchain change should still force a full rebuild is an owner decision.**
  The options are recorded in the PR.
