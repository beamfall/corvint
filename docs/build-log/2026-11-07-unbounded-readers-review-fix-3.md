<!-- SPDX-License-Identifier: Apache-2.0 -->
# Unbounded readers: printed text keeps rule (d) (V1-4283)

Lane `unbrd`, fix round 3 over `2026-11-07-unbounded-readers-review-fix-2`, base
`4052b8540b5fff818a819db37b55371ea5428c2c`, starting at `c083f6927`. The fix-only review confirmed
round 2's six corrections and raised one blocking P1. This is the third narrow fix to the same
exemption, so this round asked whether a closed proof exists at all, rather than patching one more
case.

## Finding (held)

An imported helper bypassed the capture proof. The command `tool` prints `"../config%s\n"` with
`".go"`. Its test calls `capture.Read(main)`, and the helper package `capture` redirects `os.Stdout`
to a pipe, runs the callback, trims the captured text and opens it, which reads the repository's
`config.go`. AFP-V0-2960 looked for capture evidence only in the command's own directory. The
helper carries that evidence but has no climbing literal, so `tool` kept its exemption, and an edit
of `config.go` selected nothing.

The regressions were written first and run at `c083f6927`. Each failed:

- `TestImportedCaptureKeepsClimb_AFPV02962` (planner): `tool` had no rule (d) reason and no
  `locatesRoot`, and an edit of `config.go` selected `map[]`.
- `TestImportedCaptureParity_AFPV02962` (CI selector): rule (d) was `[]` cold, warm and in the
  index, against `[tool]`. The verdict for an edit of `config.go` was `run example.com/fixture`,
  without `tool`.
- `TestPrintedTextReachesItsRunner_AFPV02962` (planner): neither `quiet` nor the make target
  `read-printed` (`cd quiet && cat "$(go run .)"`) was selected, for an edit of `config.go` or for
  an unrelated edit.

## Approach: withdraw the exemption

The review offered two options. The first was to exempt a printed literal only when no package in
the process can capture the stream: the transitive import closure of the command and its test
packages, under the same evidence rule. The second was to remove the exemption if that closure
proof is impractical. This round chose removal (AFP-V0-2962), for four reasons:

1. **The standard library defeats the closure rule.** `fmt` imports `os`, which defines `Pipe` and
   `NewFile`. Every test binary links `testing`, whose `run_example.go` sets `os.Stdout` to an
   `os.Pipe` writer to check examples (checked in the go1.27.3 GOROOT). Under the AFP-V0-2960
   evidence rule, every command's closure carries capture evidence, so the proof would exempt
   nothing. The only way out is a list of standard packages proven inert, kept for each Go release,
   and that list is a claim the lane cannot verify.
2. **The stream leaves the process.** A make target, a shell substitution or a test that runs the
   binary can open what the command printed. No import closure includes those processes.
   `TestPrintedTextReachesItsRunner_AFPV02962` pins one such case. The closure proof would still
   have missed it, so a fourth patch would have been needed.
3. **The CI selector cannot see the closure.** It reads only repository sources, so it would have
   to refuse every command whose closure reaches a module or standard package, which is every
   command.
4. **The exemption bought nothing.** Round 2 measured no repository unit using it, and the plans
   below are identical with it removed.

The code now matches the base rule (d) for printed text, in both analyzers:
- `outputproof.go` (`outputOnlyLiterals`, `capturesStream` and their helpers) is deleted from
  `internal/liveverify/affected/golang` and `internal/gateselect`.
- The planner drops `textLiterals`, `fileScan.textOnly`, `printedText` and the capture pass of
  `observeDirectory`.
- The selector drops `sourceScan.text`, `scannedFile.textOnly`/`main`/`captures`, `printedText`,
  the post-record pass, and the `TextOnly`, `Main` and `Captures` memo fields. `scanmemo.go` is
  byte-identical to the base.
- The AFP-V0-2950 and AFP-V0-2951 code is untouched.

The parity test (`agree` in `internal/gateselect/locator_parity_test.go`) holds the two analyzers
equal on every fixture, cold and warm.

The spec marks AFP-V0-2952, AFP-V0-2959 and AFP-V0-2960 as superseded by AFP-V0-2962, following the
AFP-V0-466 pattern. The earlier fixtures are kept, and every case now keeps rule (d):
- `TestPrintedClimbKeepsRuleD_AFPV02962` and `TestPrintedClimbParity_AFPV02962` replace the two
  AFPV02952 tests. The commands `printed`, `streamed` and `raw`, formerly exempt, now keep rule (d).
- `TestCapturedStreamKeepsClimb_AFPV02962` and `TestCapturedStreamParity_AFPV02962` replace the
  AFPV02960 tests. `quiet` keeps rule (d) and is selected by `config.go`.

## Measured plans

The method matches round 2: `corvint affected --worktree --path PATH --no-cache --format json`,
run on the same clean tree at `e11f37af8` with one binary built from `c083f6927` and one built from
the fix. The columns are selected / `witness.anyDirtyPath` / distinct `witness.cause.unit`.

| Change | `c083f6927` | Fix round 3 | Whole Go packages |
|---|---|---|---|
| (a) `.github/workflows/ci.yml` | 266 / 77 / 24 | 266 / 77 / 24 | 29 → 29 |
| (b) `script/heavy.sh` | 295 / 11 / 11 | 295 / 11 / 11 | 48 → 48 |
| (c) `docs/TICKET-PRIORITY.md` | 252 / 113 / 28 | 252 / 113 / 28 | 25 → 25 |
| (c') `docs/DOGFOOD.md` | 261 / 37 / 21 | 261 / 37 / 21 | 31 → 31 |

- **Plans.** The selected units, their witnesses and the unknowns are identical, unit by unit, for
  all four changes. An unbounded reader is selected on every change, so the identical any-path sets
  show that no repository unit gained rule (d).
- **CI selector.** `gate-affected-select -unresolved` over the repository gives the same 41 lines
  from both builds (sha256 `70d66f24…`).
- **Records.** `make unbounded-readers-check` and `make test-read-scopes-check` pass with the records
  unchanged.

## Limits

- Rule (d) does not follow namer edges (V1-2727). A Go test that runs a root-locating command by
  its import path, such as `go run example.com/m/tool`, is linked to the command only through that
  path token, so the command's root locator does not make the test an unbounded reader. That is true of every root locator, so withdrawing the exemption
  changes nothing here. The gap is reported to the orchestrator, not fixed in this round.
- With the exemption gone, a command can no longer print a climbing path in help text without
  joining the floor. Today no such command exists. If one appears, it needs a new proof that bounds
  every reader of its output, not a revived output proof.
