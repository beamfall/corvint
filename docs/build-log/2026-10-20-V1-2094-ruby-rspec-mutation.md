# 2026-10-20 Ruby/RSpec mutation under `prove --mutate` (V1-2094)

Human-owned intent: `docs/specs/falsifiable-packet-v0.md`, `FPK-V0-068` to `FPK-V0-073`, accepted
under the owner's standing auto-accept (2026-10-11). They address GitHub #1321.

BASE: `a5890f88ee073d38bc0a1529fe819cc92fb6582a` (origin/main `94d212c8c04c6c702ff238f7fade2ab7d3b2f06e`
merged with the #1326 head `097f8d3f`).

## Problem

A `test-kills-mutant` row whose test is an RSpec file got no mutation verdict under `--mutate`.
Nothing marked it unsupported, so the falsifier was silently absent.

## Decision

- New engine `internal/liveverify/rbmutate`, which runs on the existing `mutate.Export` sandbox,
  budget and line confinement.
  - Its planner works over `analyzerruby.MaskSource`, so strings, comments, regexes and heredocs are
    never mutated.
  - It applies a bounded operator set: relational boundary and negation, arithmetic swap,
    integer constant +/-1, and condition negation.
  - It plans at most twelve mutants, in source order.
- RSpec runs the cited spec file against the baseline and against every selected mutant.
  - Each run's JSON report decides the outcome: killed, survived, unloadable or timed out.
  - Each run keeps a receipt with the exit status, example and failure counts, failed example ids
    and the mutated source's SHA-256.
- The row carries a `mutation` record with every per-mutant verdict and the baseline receipt.
- The witness is never `PRODUCED`, because replay is Go-only.
  - A judged row says `ruby-mutation-witness-not-produced`.
  - A missing runner gives `UNSUPPORTED` / `ruby-runner-unavailable`.
  - An unusable baseline gives `UNSUPPORTED` / `ruby-baseline-unusable`.
- Minitest stays under the `FPK-V0-057` unsupported marking.

The narrowest safe reading was taken. There is no container runner in the CLI path: the command is
`bundle exec rspec` when a Gemfile exists, otherwise `rspec` on PATH. When neither starts in the
sandbox, the row is the typed `UNSUPPORTED` witness.

## Evidence

- `go test` passed for `./internal/liveverify/rbmutate/`, `./internal/analyzerruby/` and
  `./internal/specindex/`.
- The focused `cmd/corvint` run (`Prove|FPKV0|Mutat|Ruby|Python|RSpec|AFPV0066`, then `Help|FPKV007`)
  passed.
- Live qualification `TestFPKV0069LiveRSpecQualification` used real RSpec 3.13 on Ruby 3.4 in
  `corvint-rspec-qual:2026-10-20` with `--network none`.
  - It ran the issue's weak-spec fixture: 6 of 14 mutants were killed and 8 survived.
  - These outcomes are identical to the in-repo `minirspec` double.

## Limits

- No replayable Ruby witness exists.
- `attack-tests` is still Go-only.
- The live test is opt-in (`CORVINT_TEST_RSPEC_IMAGE`). On macOS, Docker Desktop also needs
  `CORVINT_TEST_RSPEC_DIR` under a shared path.
- The `prove.go` and `help.go` line citations in three specs were repinned mechanically. Each one
  moved only by the inserted lines, and its original content anchor still matches.

## Independent review

One Codex round (read-only) reported five findings. All five were confirmed and fixed in this
change.

- A failed example whose exception is a Ruby `ScriptError` (a lazily loaded mutant that no longer
  parses) was being counted as a kill. It is now `unloadable`.
- The host's `~/.rspec` and XDG options could reach runs. Every run now names an options file in
  the export that holds only the revision's `.rspec` and `.rspec-local`.
- The planner split a string at an inner `#`. It now ends a line's code only at comment offsets
  recorded by `MaskSource`.
- A condition closed by a bare `end` on its own line (`def f; x if y end`) was negated into
  invalid Ruby. Such a condition is now skipped.
- A float exponent such as `1e-3` was split into separate sites. It is now kept as part of the
  float.

After the fixes, the live RSpec qualification gave the same result: 6 of 14 mutants killed and 8
survived.
