# 2026-10-24 Content-word coverage, ID-slug names and bounded declaration derivation (GitHub #1665)

This lane works on the GitHub #1665 re-test of `ec41e4e` (V1-2598). Its base is `origin/main`
`ec41e4e9c901cb3546e95a82fc665bb8bc6c58e9`. The requirements are `DCP-V1-565..569` in
`docs/specs/documentation-corpus-v1.md`, all proposed and experimental. 565 was the next free ID
after `origin/main` (564) and the open PRs' diffs.

## Decisions

- **Question coverage counts content words only (`DCP-V1-565`).** The `DCP-V1-547` held-word
  count in a question skips question words and a short fixed list of frame words (`source`,
  `implemented`, `defined`, `work`, ...). Scores are unchanged, so only the order inside tier
  `terms` moves, and statements are untouched. A frame-word list rather than document frequency
  keeps the rule deterministic and independent of corpus size.
- **An ID slug names a subject (`DCP-V1-566`).** The slug is the ID after its last `/` or `:`
  without its dot suffix. It must hold at least two content words, contiguous in the question, so
  `flows/reservation` alone never reaches tier `named`.
- **The named tier needs the whole name (`DCP-V1-567`).** A multi-word name needs two content
  words, and a one-word name a capitalized word that no capitalized neighbour extends. This keeps
  `DCP-V1-548`'s `How does Waitlist work?`.
- **Derivation is bounded, not all or nothing (`DCP-V1-568`, `DCP-V1-569`).** The first compile
  derives up to the record bound. A compile the output byte bound refuses is retried once with the
  prefix whose estimated bytes fit, with a 4096-byte reserve. One gap on
  `<native>:derived-declarations` names each bound, its value, the counts and the first dropped
  declaration.
- **Why an over-bound build still compiles twice.** The output byte bound is known only once the
  whole artifact is encoded. The reporter's 4-repository corpus is 114 MB without declarations,
  against `MaxCorpusBytes` (128 MiB). A record-bound corpus now compiles once (it compiled twice
  before), while a byte-bound one compiles twice: the first compile measures the overshoot, and the
  second keeps the prefix that fits instead of none. Estimating the size before the first encode is
  filed as a follow-up optimization rather than done here.
- **Derivation 4.** The same over-bound inputs now build declarations and a different gap, so
  `DerivationVersion` is `4`. The verified read refuses a derivation 3 artifact with
  `derivation version differs (artifact 3, running builder 4); rebuild the corpus`. Rebuild with
  `corvint docs corpus build --artifact CORPUS.json`. The single-repository fixture digest is
  unchanged (pinned for `4` in `derivation_test.go`), and no artifact layout or schema changes.

## Evidence

Fail-first, on the unmodified code (linux/WSL2 via heavy.sh): all five new tests failed. 565 gave
the frame paragraph 3 held words against the subject's 2. In 566 the flow
`flows/reservation-cancellation` stayed in tier `terms`. In 567 `Cancellation` reached tier
`named`. 568 took 2 compiles for a record-bound corpus. 569 was then still asserted by
`TestDCPV1484DerivationNeverRefusesABuildableManifest` as deriving 0. After the change, the
`./internal/doccorpus` package passes. The general-search 59/64 case count is the reporter's
private corpus and is not reproduced here; it needs the reporter's re-test.

## Rollback

Revert the change and return `DerivationVersion` to `3`. Artifacts built at derivation 4 are then
refused as another derivation and rebuilt. No stored state needs migration.
