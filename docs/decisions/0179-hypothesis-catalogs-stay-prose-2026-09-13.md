# Decision 0179 — hypothesis catalogs stay prose-only

Date: 2026-09-13. Status: accepted, delegated coordinator call. Authority: repository owner
delegation to make owner calls and record them (orchestration thread, 2026-09-12).

`docs/agent-memory/questions.md` asked (2026-09-12): should
`docs/specs/applied-intelligence-breakthroughs-v0.md`, `docs/specs/context-evolution-program-v0.md`,
and `docs/specs/js-live-test-provider-v0.md` ever gain canonical `` - `PREFIX-NNN`: text`` clauses,
so a future accepted slice can bind an OCM change map to them, or do they stay permanently
prose-only? Each is already header-labelled `Document kind:` (hypothesis catalog / pre-qualification
experiment profile), so `script/gen-spec-requirements.sh`, `check-requirement-definitions.sh`, and
OCM correctly abstain from inventing coverage today (AGENTS.md invariant 8).

The call: the three catalogs stay prose-only, permanently — not merely until some future slice
needs them. A hypothesis catalog records candidate ideas under evaluation; it is not itself the
human-owned intent AGENTS.md invariant 8 requires numbered requirements to trace to. Relabelling
catalog prose into `PREFIX-NNN` clauses would let inferred or exploratory text govern as if it were
accepted intent, which invariant 8 forbids outright ("generated or inferred documentation can
propose intent but cannot accept or govern it"). A hypothesis gains numbered clauses only when a
new or amended *accepted* spec slice adopts it — written as that slice's own `PREFIX-NNN`
requirements with their own traceability, never by mechanically reformatting the catalog's prose
into requirement bullets in place. This preserves the distinction `gen-spec-requirements.sh` and
`check-requirement-definitions.sh` already enforce: a catalog is a source of ideas, not a source of
requirements.

Consequences: each of the three catalogs' header `Document kind:` note gets one sentence citing
this decision, stating that promotion happens only via a separate accepting slice's own
requirements, never by reformatting the catalog itself.
`docs/agent-memory/questions.md`'s 2026-09-12 entry is removed; no requirement IDs are added, no
`REQUIREMENTS.tsv` regeneration is triggered by this decision alone.

Rollback: revert the three header-note sentences and this file; the underlying gate behavior
(abstention on catalog documents) is unchanged either way, since it already follows from each
catalog's existing `Document kind:` prose label.
