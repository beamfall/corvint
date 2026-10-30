# Decision 0100 — doc-compiler kebab codes are secondary details; unowned codes are a shrinking ratchet

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

Two backlog items met in one sweep. `docs/agent-memory/questions.md` asked whether
`internal/doccompiler`'s kebab-case failure codes should map onto the closed UPPER_SNAKE set of
`HDCV0-052` (`docs/specs/human-documentation-compiler-v0.md`) or whether the requirement should
change. `docs/agent-memory/fixes.md` recorded that hundreds of emitted kebab codes are named by no
spec, with no mechanism stopping new ones.

## Call 1: the two code sets are two layers, and neither is renamed

- `HDCV0-052`'s closed UPPER_SNAKE set stays the primary code of the canonical compiler output.
  That set is load-bearing inside the same spec: the capsule's stage order and precedence rules
  (`PROCESS_UNCONTAINED` overrides earlier failures, `STALE_SOURCE` overrides a build result, a
  deadline or byte breach is `LIMIT_EXCEEDED`) are written against it. Delivery is `not-started`, so
  no emitter of a primary code exists yet, and none is invented here.
- The `internal/doccompiler` `Error.Code` values are the bounded "secondary details" `HDCV0-052`
  already permits. They keep their spelling. The spec now enumerates every one of them in a
  secondary-detail table whose rows describe only what the emitting code path checks.
- The mapping from each secondary code to exactly one primary code is required in the change that
  delivers the canonical output emitter, together with its failure-code vectors, not before. Written
  now it would be 82 normative mappings with no emitter and no vector to hold them.
- Set aside: renaming `HDCV0-052` to kebab codes, which would discard the capsule precedence rules
  for a surface not yet built; and authoring the mapping now, for the reason above.

## Call 2: emitted-but-unowned codes become a ratchet in `make gate`

- `script/check-error-code-ownership.sh` extracts emitted kebab codes mechanically (four syntactic
  positions in tracked non-test Go under `cmd/` and `internal/`) and checks each against the tracked
  `docs/specs/*.md` token set. `script/error-code-ownership.allowlist` holds exactly the codes that
  are emitted and unowned; the gate fails when the two differ in either direction, so a new unowned
  code fails and an owned or retired code must leave the list. The contract is
  `docs/specs/error-code-ownership-gate-v0.md` (`ECO-V0`), accepted by this decision.
- The extraction is positional and syntactic, not a type-checked wire inventory. It over-counts
  internal reasons that never reach an envelope and under-counts codes routed through named
  constants; both limits are recorded in the spec rather than hidden. The natural long-term owner of
  a typed inventory remains `DRC-V0-006`/`DRC-V0-011` once that contract is accepted.
- Owning a code means a spec row that describes what its emitting code path does, cited by
  `path:line`. A row MUST NOT assign semantics the code does not implement.

Rollback: revert this decision's commit series. That drops the `make gate` prerequisite, the script,
its test, the allowlist and `ECO-V0`, removes the doc-compiler secondary-detail table, and restores
both backlog entries; no Go or runtime bytes change.
