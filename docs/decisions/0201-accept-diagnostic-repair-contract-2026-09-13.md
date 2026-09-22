# Decision 0201 — accept the diagnostic repair contract, with eleven clause amendments

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`docs/specs/diagnostic-repair-contract-v0.md` (`DRC-V0`) was blocked only on owner acceptance of the
contract as a whole; the subject shape, the registry location and the ratchet start were already
settled by owner instruction of 2026-09-12. This decision accepts the whole contract. A critical
read against `AGENTS.md` invariants 2, 4 and 5 and `SOL-V0-007` found clauses that are unsound or
not mechanically decidable as written, so they are accepted in the amended form below, and the spec
is amended in the same change.

## What is accepted

`DRC-V0-001`..`012` as amended here: a covered refusal carries `subject` (`kind`/`value`),
`evidence`, `supported_fixes` drawn from `docs/specs/FIX-REGISTRY.tsv`, and `terminal` when no
repair exists; the fields are additive to the existing `{code, error, ok}` stderr envelope; the
first converted family is `unsupported-working-tree-impact-*` at seven sites; a Go-test gate
enforces the shape, the registry and a covered-site ratchet starting at seven.

## Amendments

(a) Scope of "every refusal" (`DRC-V0-001`, `002`, `003`, `005`). As written, acceptance would make
the whole tree nonconforming on the day of acceptance and contradict `DRC-V0-011`'s ratchet and the
spec's own failure row for uncovered sites. The shape obligations bind every *covered* site; an
uncovered site keeps `{code, message}`, stays outside the covered count, and MUST NOT emit a guessed
fix list.

(b) Prose in `subject.value` (`DRC-V0-001`). "No prose, no second fact" is not mechanically
decidable. The mechanical rule is: `value` is non-empty valid UTF-8 with no control character. The
semantic rule is enforced by a frozen per-site expectation, so a qualifier concatenated into `value`
fails the field-by-field comparison. Where the refused thing is a repository property no input
spells (a missing captured index, a module path), `value` is a stable property token and the value
actually read is `evidence`.

(c) Screen and bound outrank exact spelling (`DRC-V0-001` against `DRC-V0-008`). A subject value
that is redacted or truncated can no longer be the exact spelling. The screen wins (invariant 5), and
the disclosure in the value states that it is not the exact spelling, which is uncertainty rather
than invented certainty (invariant 2). Order: `secretscreen.Screen` first, then the 512-byte bound at
a rune boundary with a disclosure suffix counted inside the bound, so truncation can never cut a
secret the screen would have matched. Evidence names are bounded the same way. More than 16 pairs
keeps the first 15 and appends one `evidence-truncated` pair holding the dropped count.

(d) Fix entries (`DRC-V0-003`). An emitted entry is a bare registry identifier; the field, argument
or file it changes and the admissible target are the identifier's registry row (`target`,
`admissible` columns), not text in the emitted entry.

(e) Terminal reasons (`DRC-V0-005`). The closed tokens are `missing-authority`, `absent-evidence`,
`unsupported-platform` and `owner-decision-required`. `terminal` is emitted exactly when
`supported_fixes` is empty; a site with both non-empty, or neither, is a gate failure.

(f) Code inventory (`DRC-V0-006`). A full-tree code replay duplicates `ECO-V0`, which already owns
every emitted code. The survival replay is scoped to the codes whose envelope a conversion touches,
plus a byte-identical envelope for an unconverted refusal, plus the `SOL-V0-007` row for the family.

(g) Ledger boundary (`DRC-V0-007`, invariant 4). The ledger row keeps exactly `SOL-V0-007`'s shape
(code, intent, task hash); none of the new fields reaches it, and emission adds no other write.

(h) Repair loop (`DRC-V0-009`). The non-goal "no loop orchestration inside Corvint" stands. The stop
rule is stated in the spec and published as one pure function, `diagnostic.RepairStop`, that an
adapter may call; Corvint never drives it.

(i) Layers (`DRC-V0-010`). Only the CLI carries the converted family: `batch` does not route
`--working-tree-untracked` and the host adapters do not call `impact`. The assertion runs at the CLI
layer and extends to a layer when a family that layer carries converts.

(j) Site location (`DRC-V0-011`). The seven sites stay at their cited `compiler.go` lines, each
calling one constructor whose `diagnostic.Refusal` literal sits in
`internal/worktreeimpact/diagnostics.go`. The gate enumerates `diagnostic.Refusal` composite
literals in non-test Go under `cmd/` and `internal/` from the worktree; unconverted refusal sites
are already enumerated by `ECO-V0`. The recorded count lives in
`script/diagnostic-coverage.count` and the gate fails on any difference from the measured count, in
either direction, and on a record below seven.

(k) Message parsing (`DRC-V0-012`, closes the spec's one open question). The mechanically checkable
part is bounded: a non-test Go file that imports `internal/diagnostic` MUST NOT pass `.Message` or
`.Error()` to a `strings`, `bytes` or `regexp` function or compare either with `==`/`!=`. Consumers
outside Go (hooks, external adapters) remain review-only; that residual is accepted.

The two `docs/agent-memory/fixes.md` entries the spec cites were already closed before this
decision (the coordinator-input entry by `f59e6ada`, removed in `bc24ba87`; the replay-window
truncation-count entry by `7723cc66` and `017d3f97`, removed in `017d3f97`), so there is no
backlog entry to remove.

## Alternatives weighed

Accept verbatim: rejected, because (a) and (c) are internal contradictions and (k) would leave an
unfalsifiable requirement. Keep the spec proposed until an agent-caller study exists: rejected; the
promotion and kill criteria already gate `validated`, and acceptance here is of the contract, not of
its value. Carry the diagnostic on `gokernel.Error` as a new field: rejected in favour of a wrapping
`diagnostic.Error` with `Unwrap`, which leaves the shared kernel type and every `errors.As` caller
unchanged. A shell gate over `git grep`: rejected, because a multi-line composite literal cannot be
checked for its keys without a parser.

## Rollback

Revert the implementation commits (envelope package, registry, CLI serialization, family
conversion, gate and its Makefile wiring) and return the spec's Intent status to proposed with this
decision marked superseded. The fields are additive, so reverting breaks no `code` consumer.
