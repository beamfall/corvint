# Decision 0080 — A document's status is a labelled field, read wherever the repository writes it

Date: 2026-09-07. Status: accepted. Authority: repository owner, verbatim instruction "now fix the
documentStatus bug" (2026-09-07), acting on the defect filed in `docs/agent-memory/bugs.md` the same
day.

## Context

AGENTS.md invariant 3 puts project-owned authority above syntax, history, and learned traces. The
label that carries it — `accepted-decision`, `accepted-spec`, `partially-superseded-contract` —
depends entirely on a document's `status` field, extracted by `documentStatus`
(`internal/contextindex/parse.go`) and its oracle twin in `src/context_corvint_index.py`.

Both matched only `^status:`: the field name bare, at column zero. This repository does not write it
that way. Measured across the 165 instruction, decision and spec documents at `68ca0c9`: 83 write
`Date: 2026-09-05. Status: accepted. Authority: ...`, 72 write `Intent status: accepted (...)`, four
write `- Status: ...`, two write it bare, and four carry no status field at all. Four documents of
165 were detected.

Detection alone was not sufficient either. `binding` compares the captured value against a fixed
vocabulary by equality, and the capture ran to end of line, so `Status: accepted — option 2
(2026-09-06)` was detected and still not binding.

Between them, detection and equality left **exactly one document of 165 binding**. Invariant 3
therefore ranked nothing on Corvint's own corpus: `impact` labelled every accepted decision
`non-binding-decision` and every accepted spec `repository-spec`, and the authority trigger index
(`ATI-V0`) inherited it.

## Decision

A document's status is a **labelled field**, and both implementations read it wherever this
repository writes such a field, capturing the status token alone.

1. The field name may be preceded, at column zero, by a list marker (`-`, `*`, `+`), by emphasis
   markers, by one qualifier word (`Intent status:`, `Delivery status:`), or by a run of preceding
   `Key: value.` field segments on the same line.
2. A preceding segment MUST itself look like `Key: value.` — a colon, then a dot-free value, then a
   full stop and whitespace. Prose that merely ends in a full stop therefore cannot introduce a
   status.
3. The field still starts at column zero. An indented `status:` is a nested key inside some other
   block, not the document's status, and is not read. This preserves the behaviour
   `TestDocumentStatusIsAnchoredAndHeadingsTruncateByRune` has always pinned.
4. The captured value is the status token: letters, digits and hyphens, optionally followed by
   `:argument`, with optional spaces after that colon, so both `partially-superseded-by:0031` and
   the quoted `"partially-superseded-by: 0042"` survive whole — the prefix test that reads them
   requires the colon. Everything after the token — a parenthetical, a citation, the rest of the
   sentence — is not part of the status.
5. The Go pattern and the Python oracle's pattern are the same pattern. Neither may be changed
   without the other, per `GPK-V0-002`.

## Alternatives rejected

**Leave it and treat the corpus as the problem.** Rewriting 160 documents into `Status: accepted`
would make the extractor right by making the documentation worse, and any future document written
the natural way would silently lose its authority again.

**Match `status:` anywhere on any line.** This admits prose. Since the first match in the file wins,
one sentence containing the word would decide a document's authority.

**Keep the value to end of line and loosen `binding` instead.** The wire field `status` would then
carry `accepted. Authority: repository owner, delegated call; the owner instructed...` — a sentence
where consumers expect a status. The field's value should be the field's value.

## Consequences

Detection goes from 4 documents of 165 to 161, and binding documents from 1 to 113. The four misses
are the two `README.md` files and `AGENTS.md`/`CLAUDE.md`, none of which declare a status;
instructions are binding by kind regardless. Captured values become a clean vocabulary: 113
`accepted`, 43 `proposed`, three `superseded`, one `not-applicable`, one `rejected-as-specified`.

Output changes for any repository whose documents use the newly-read shapes: `authority` and
`confidence` in `impact` and `query` evidence, the `status` member of a document result, and the
`+20` ranking bonus in `query`/`eval_query`. `analyzerSchemaID` advances to `corvint-analyzer/17`,
because extracted facts genuinely changed.

The parity corpus is unaffected: of 21 fixtures exactly one, `impact-adr`, carries a status-bearing
Markdown document, and it writes `status: accepted` at column zero, which both the old and the new
pattern read identically. `conformance/cli-parity-v0` passes with its pinned stdout digests
unchanged, which is the evidence that this repair moved no oracle-pinned byte.

## Rollback

Revert the two patterns and the `analyzerSchemaID` bump together. Nothing else depends on the change,
and no document was rewritten to suit it.
