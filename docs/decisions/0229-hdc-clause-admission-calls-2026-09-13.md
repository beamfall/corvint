# Decision 0229 — HDC clause admission and prose template calls

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Decision 0209 closed the `HDCV0-023..026` vocabularies but left calls open that change admitted
states or rendered bytes. This decision makes those calls for the pure admission contract in
`internal/doccompiler/admission.go` (`AdmitClauses`, `RenderAdmittedProse`, `VerifyAdmittedProse`).

The call:

(a) Source identity. An anchor is checked against a Git-pinned `contextindex.Index`. The path must
be pinned; a pinned blob other than the anchor's `blob` makes the anchor stale. The pinned bytes must
be valid loaded text whose Git blob ID (SHA-1 for 40 hex digits, SHA-256 for 64) equals `blob`;
otherwise the anchor is disqualified. `blob` must be 40 or 64 lower hex, so a ref such as `HEAD` is
mutable and disqualified.

(b) Span bytes. `start_line..end_line` are one-based and inclusive. The span is those lines' bytes,
each with its terminating LF when present; a final LF does not open another line, and CR is not
normalized. `span_sha256` is the SHA-256 of those bytes. A span of exactly 8,192 bytes is admitted.

(c) Downgrade, not refusal. A `SUPPORTED` or `CONFLICTED` clause that its anchors do not establish is
emitted as `UNKNOWN` with frontier `STALE_ANCHOR` when any anchor is stale, else
`NO_QUALIFYING_SOURCE`. Its anchors are kept as review context. A caller `UNKNOWN` clause is emitted
unchanged and its anchors are not verified. `RESOLVER_UNDECIDED` is only ever caller-declared.

(d) Drift. `CONFLICTED` needs two fresh anchors at distinct `(path, blob, start_line, end_line)`
with at least one qualifying for the clause kind under `HDCV0-024`; the other may be the opposing
authority class. That is how intent/implementation drift is emitted with both anchors. Two
source anchors cannot make a `PRESCRIPTIVE` clause `CONFLICTED`.

(e) Scope. An undeclared scope is emitted as `GENERAL`. A `GENERAL` clause cannot be `SUPPORTED` by
`PINNED_TEST` or `EXECUTION_RECEIPT` anchors; for `CONFLICTED`, scope does not change qualification.

(f) Shape. A clause ID is 1..256 bytes of UTF-8 without whitespace or controls; review text is
non-empty UTF-8 of at most 1 MiB; a non-`UNKNOWN` clause carries no frontier. A malformed clause is
`invalid-clause`, a repeated ID is `duplicate-clause`, and more than 2,000 clauses or 10,000 anchors
is `clause-limit-exceeded`; the whole call is refused. Output clauses are ordered by clause ID.

(g) Templates. The closed set is `corvint-hdc-prose-templates/0`: a first line
`<!-- corvint-hdc-prose-templates/0 -->`, the heading `## Evidence and uncertainty`, then per clause a
blank line and `- Clause <id> is **<state>** (<kind>, <scope>[, frontier <frontier>]): <text>`, and
per anchor `  - Anchor <path> lines <start>-<end>, blob <blob>, authority <authority>`. Caller values
are rendered as Markdown code-span literals. `reason` is not rendered. A candidate is admitted only
when it is byte-equal to that rendering; anything else is `unadmitted-prose`.

No requirement IDs are added. `HDCV0-027..030` stay undelivered: the plan wire replaces the
experimental `PatchPlan` consumed by `internal/docviews`, `internal/docmaintain`, `cmd/corvint`
docs and the MCP docs bridge, and `HDCV0-030` needs the MkDocs authority snapshot.

Rollback: revert the commit. No other package calls the admission functions, and no wire that is
already emitted changes.
