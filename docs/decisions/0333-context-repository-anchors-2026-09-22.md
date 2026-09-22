# Decision 0333 — Repository anchors as a verbatim lexical field in `context`

Date: 2026-09-22. Status: proposed, experimental (ticket V1-0084; TCP-V0-022). Amends nothing; it
adds an opt-in field to the task-context packet's lexical slot and sets no default.

## Context

The term tokeniser (`termCounter.count`) keeps maximal ASCII alphanumeric runs, splits camel case
and lowercases, and the identifier table (`Words`) keeps word runs as written. Neither can answer
an error string, a URL, a `Scope::Value` enum, a dotted configuration key or a `file.ext:line`
frame as one term: the literal is split into common words before BM25 sees it, so a task that
quotes the exact string ranks the file carrying it no higher than files carrying the words apart.
The ticket asks for an anchor class that keeps such literals verbatim, explains anchor hits with
their own reason, and never lets them outrank project-owned authority.

Adding an anchor table to the index would change the term-table section of the pack and the
snapshot; the frozen retrieval evaluation that would justify a default-on ranking change reads
the external `agent_retrieval_bench` corpus, which is not on the build host.

## Decision

1. Anchors are a query-side field (`internal/contextindex/context_anchors.go`): five fixed
   classes are extracted from the task in order, and each anchor is verified verbatim against the
   bounded bodies of the sources whose `Words` postings share every word run of the anchor. No
   index, snapshot or pack table is added.
2. Verified occurrences are credited inside the lexical slot with the body-term BM25 form, and
   the row's reason is prefixed `anchor: `literal` xN verbatim; `. The row keeps score 300 and
   authority `vocabulary`, so a reserved TCP-V0-008/009 row always precedes it.
3. The field is behind `CORVINT_CONTEXT_ANCHORS=on`, the TCP-V0-019 shape: unset or any other
   value preserves the existing packet bytes (`TestContextAnchorsDefaultBytes`). Promotion to
   default requires the TCP-V0-022 falsifier on the frozen bench plus decision 0070's ladder.
4. Bounds: literal 4 to 256 bytes, sixteen anchors per task, 512 candidates per anchor; beyond
   the candidate cap the anchor abstains.

## Consequences

Anchor-bearing tasks gain exact evidence only when the operator opts in; the default product is
byte-identical. The evaluation criterion of V1-0084 (no regression on the frozen bench, anchor
subset measured separately) is not met by this change: the corpus is absent here and the bench
has no anchor-bearing subset yet. Both are recorded as open in `docs/BUILD-LOG.md`.

## Rollback

Delete `context_anchors.go` and `context_anchors_test.go`, the `anchors` fields and anchor loop
in `lexicalHits`, and the `field >= 2` widening in `queryTermGain`.
