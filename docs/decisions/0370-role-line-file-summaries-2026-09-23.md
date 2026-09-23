# Decision 0370 — Role lines from doc comments as a lexical field in `context`

Date: 2026-09-23. Status: proposed, experimental (ticket V1-0096; TCP-V0-039..042). Amends
nothing; it adds an opt-in field to the task-context packet's lexical slot and sets no default.

## Context

A lexical row points at a whole file, and its score is the task terms spread through the body.
A file's own statement of its role (a Go package comment, a Python module docstring, a Rust `//!`
block, a Javadoc or JSDoc block) is the densest description the repository gives of it, and the
ticket asks for that line as a retrieval unit with its own reason, derived only from repository
content at the indexed revision, never generated (prior art: arXiv 2607.11046, per-file summaries
as retrieval units). The line must be byte-reproducible, and rows it admits must name the comment
they came from.

Storing the line in the index would change the pack and snapshot encoding, which parallel tickets
edit concurrently, and would force an `analyzerSchemaID` bump for a field whose value is unproven.

## Decision

1. The role line is extracted from the pinned blob by a fixed per-suffix rule
   (`internal/contextindex/rolesummary.go`, TCP-V0-039): package or module doc comment first,
   then top-level doc comments, first sentence, at most 160 bytes, licence and generator headers
   refused, at most 64 KiB scanned. It is computed per `context` call and never persisted, so the
   index encoding and `analyzerSchemaID` are unchanged.
2. With `CORVINT_CONTEXT_ROLES=on` (TCP-V0-040), the role lines of the 512 highest-scoring
   lexical sources are read and each task term a line carries is credited once at its body idf,
   the path field's form (gain 1.0, frozen before the evaluation). The credit lives inside the
   lexical slot, so a role row keeps score 300 and authority `vocabulary` and never precedes a
   reserved authority row.
3. The row's reason is prefixed `role: "LINE" (PATH:START-END) matches ...; ` (TCP-V0-041).
4. Gating (TCP-V0-042): the field ships behind the flag unless the frozen bench shows no
   recall@20 loss on every one of the four positive subsets. The 2026-09-23 run lost recall@20 on
   `v2_comment2context` (0.5042 to 0.4938, 1 win and 4 losses over 80 positives), so the choice is
   **opt-in**: `CORVINT_CONTEXT_ROLES=on` enables the field and every other value keeps the default
   packet. Per-subset figures and losing sample IDs are in `docs/BUILD-LOG.md`.

## Consequences

The default packet is byte-identical (`TestContextRolesDefaultBytes`). The 512-candidate bound
means the field re-orders sources the body already found; it cannot admit a file no task term
reaches, and a file past the bound is not read for a role line. Promotion beyond this decision
needs decision 0070's paired ladder.

## Rollback

Unset `CORVINT_CONTEXT_ROLES`, or delete `rolesummary.go`, its test and golden, the `roles`
field, the role credit loop in `lexicalHits` and the `role` reason prefix in `lexicalRows`.
