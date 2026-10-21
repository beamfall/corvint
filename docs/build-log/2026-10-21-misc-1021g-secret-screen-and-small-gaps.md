# 2026-10-21 misc-1021g: secret-screened source text and small correctness gaps

Lane misc-1021g, base `ae3b418152fe4c42f52ac7cfdc16ecc12159607e`. New requirements are accepted
under the owner's standing auto-accept (2026-10-11).

## Problem

- V1-0426: `context --expand`, the adapter source view (`claude-source-handoff`, `source-view`) and
  `context grep` printed tracked source lines verbatim, including secret-bearing lines. A fixture
  file with an API key and a password assignment showed both in `--expand` output and in
  `context grep` `lines[].text`. The `context` packet and the MCP tools (packets only) carry no file
  text, and the probe found no secret in them.
- V1-2132: `wire.Errorf(wire.CodeOf(err), ...)` re-wraps drop a cause's not-retryable mark.
- V1-0770: the operator-notes rollback text did not say that full journal audit decodes every
  historical policy POST.
- V1-2264: `.corvint/unbounded-readers.json` still listed the deleted
  `script/pre-push-windows-vet_test.sh`.

## Decisions

- `ESV-V0-012` (experimental-source-views; drafted as ESV-V0-011, renumbered in batch 1021h because #1446 took 011): screen the whole pinned blob with
  `internal/secretscreen`. Replace each selected line that a match overlaps with `[REDACTED]`,
  keeping the line's own ending, and disclose the replaced lines in `redacted_lines`. `bytes`,
  `sha256`, the blob digest and the offsets keep describing the exact pinned bytes. Clean output is
  byte-unchanged. `ESV-V0-002`/`ESV-V0-009` now cite the exception.
  `cmd/corvint/source_screen.go` joins the frozen native source digests (`ESV-V0-005`). All three
  digests are re-frozen.
- `TCP-V0-160` (task-context-packet): `context grep` quotes an overlapped line as `[REDACTED]` with
  `"redacted": true`, keeping its `line` and `terms`.
- False positives from the assignment grammar (`token := x`) are accepted and disclosed. About 1,089
  Go lines in this repository match it.
- V1-2132: `wire.Annotatef` and the 19 site replacements were already in base (`73d457f0a`, V1-0908).
  This change adds the missing guard: `TestCALV0078_NoRewrapDropsNotRetryable` is an AST walk over
  every non-test Go file that fails on any `Errorf(CodeOf(x), ...)` outside `wire.Annotatef`.
  `TestCALV0078_RewrapControls` holds its positive and negative controls.
- V1-0770: the operator-notes rollout section now records the audit constraint. Once an accepted
  policy has named `NOTE_SET`/`NOTE_CLEAR`, rollback keeps the vocabulary and disables only the
  writer and CLI. No new requirement ID.
- V1-2264: the stale entry is removed. A stale unit stays advisory by design (AFP-V0-025;
  `tools/unbounded-readers/main_test.go` "stale unit").

## Evidence

- The new tests fail without their fixes:
  - Neutralising the span lookup in `screenSourceLines` and `matchingLines` fails the
    `TestESVV0012*` and `TestTCPV0160*` tests.
  - Restoring `wire.Errorf(wire.CodeOf(err), ...)` at `internal/tasks/importer.go` fails
    `TestCALV0078_NoRewrapDropsNotRetryable`.
- Focused and affected test packages, vet, gofmt, windows-vet and the doc gates: see the PR body.

## NOT_RUN

- Tier-2 affected units, left for batch CI.
- V1-2132 AC2: a regression test that drives a former site with a not-retryable cause. No former
  site has a reachable not-retryable cause on main (the `WithoutRetry` producers are
  authority/session.go, store/workflow.go and cli/run_batch.go). `TestCALV0078_AnnotateKeepsNotRetryable`
  covers the helper.
- V1-0770: no native witness binary was run; the rule is read from the decoder path.

## Rollback

Revert this change. The screen adds only the `redacted_lines`/`redacted` members and placeholder
text. Re-freeze the digests to the reverted files. No index, snapshot or store migration is needed.
