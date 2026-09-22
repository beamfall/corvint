# Decision 0209 — Human Documentation Compiler V0 re-review amendments

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The independent re-review `docs/reviews/hdc-v0-rereview-2026-09-13.md` found four HIGH defects in
`docs/specs/human-documentation-compiler-v0.md` (intent accepted by decision 0047):

- the canonical-JSON clause `HDCV0-041` was superseded while the capsule clauses and the
  cross-audience slice cite it;
- the plan (`HDCV0-027`) had to contain the build state and artifact digests of the receipt that
  binds it, which is a digest cycle;
- the clause-admission vocabularies of `HDCV0-023`/`HDCV0-024` were open, so no test could decide
  admission;
- the execution capsule was not scoped outside the default local product (AGENTS.md invariant 7).

The call, applied to the spec in the same change:

(a) `HDCV0-041` stays normative for every environment kind; only `HDCV0-040` is superseded by
`HDCV0-CAP-011..013`. `HDCV0-041` now fixes the exact escape set, the signed 64-bit integer range,
no insignificant whitespace, and the 8 MiB bound; its determinism sentence moves to `HDCV0-043`.

(b) `HDCV0-027`: build state, offline state, and artifact digests are receipt members only.

(c) `HDCV0-023`/`HDCV0-024`/`HDCV0-025`: closed clause kind (`PRESCRIPTIVE`, `DESCRIPTIVE`), scope
(`OBSERVED`, `GENERAL`; undeclared is `GENERAL`), frontier (`NO_QUALIFYING_SOURCE`, `STALE_ANCHOR`,
`RESOLVER_UNDECIDED`), anchor members, and authority classes (`ACCEPTED_INTENT`, `PINNED_SOURCE`,
`PINNED_TEST`, `EXECUTION_RECEIPT`). A disqualified anchor cannot support `SUPPORTED` or
`CONFLICTED`; `CONFLICTED` needs two distinct qualifying anchors; generated documentation, commit
messages, history, learned traces, model output, and dirty bytes are never an anchor authority.

(d) `HDCV0-026`: rendered prose is admitted clause text plus a closed template set only.
`HDCV0-029`/`HDCV0-030`: `edit_nav` is the only replacing operation, limited to the span the MkDocs
authority snapshot proves; a lexical `nav` span is not that proof.

(e) A plan-only request is never `EXECUTABLE_UNSUPPORTED`; a receipt binding its plan records build
`NOT_RUN` and offline `NOT_OBSERVED`. The capsule is an opt-in capability outside the default local
product, and all `HDCV0-CAP-*` delivery is deferred.

(f) LOW corrections: the `HDCV0-CAP-006` ustar body bound is 33,555,968 bytes; `HDCV0-049` commands
carry `-timeout 30m`; "exactly `docs compile`" is scoped to HDC surfaces; the traceability text no
longer claims the portable core is implemented.

No requirement IDs are added or removed. Renderer and capsule qualification remain `NOT_RUN`.

Consequences: requirements deferred by the review cannot be claimed by any pure slice. The existing
experimental `internal/doccompiler` plan digest and receipt bytes, and the `internal/docviews` plan
re-binding, hash `encoding/json` output that is not `HDCV0-041`-canonical; the canonical emitter
change must migrate them together (filed in `docs/agent-memory/fixes.md`).
The first delivered slice under the amended text is `HDCV0-041` alone: the pure verifier
`VerifyCanonicalJSON`, with delivery status `partial (HDCV0-041)`.

Rollback: revert the commit. That restores the pre-amendment clause text and supersession table;
no code depends on the amended wording.
