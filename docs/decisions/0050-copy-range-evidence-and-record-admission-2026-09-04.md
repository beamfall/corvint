# Decision 0050 — copy ranges carry evidence; `record` admits the repository files it names

Date: 2026-09-04. Status: accepted. Authority: repository owner, verbatim instruction "I want you
to make the calls for me and keep the codex work going" (2026-09-04). Two capability gaps in
`docs/agent-memory/ideas.md` say to close by spec amendment and never by runtime self-widening;
this record is that amendment.

## 1. A copied path is a range member with its source named

`GPK-V0-030` rejects rename and copy statuses outright, so `corvint impact --base` refuses any
range Git detects with `C` or `R`. That refusal cost a real dogfood run today: the wave-17 range
copied `benchmarks/results/v4-development.json` to a Go-specific artifact, and range impact stayed
`unsupported-impact-range` for the whole change, which makes `dogfood-check` fail on an otherwise
complete report.

`GPK-V0-030` is amended: a `C` (copy) or `R` (rename) member whose similarity index is reported by
Git is admitted when its source path is bound beside it. The compiler MUST bind, for such a member,
the status letter, the similarity score, the source path, and the source blob, in addition to
everything a modified member binds; a copy's admitted hunks are computed against its named source
blob, exactly as a modification's are against its base blob. Every other rejection in the clause
stands: delete, type change, mode change, symlink, submodule, binary content, shell-unsafe path.
A copy or rename whose source Git does not report stays refused as `unsupported-impact-range`.

Why admit rather than keep refusing: a copy is the one status where the base content exists and is
named, so the evidence a hunk needs is present. Nothing here widens what the packet claims; it
widens which ranges can be described at all.

## 2. `record` admits `.gitignore` beside its source paths

`corvint record` admits only tracked context-index sources, so a change whose only non-source file
is `.gitignore` records `no-source-paths` and cannot bind its own outcome. `.gitignore` is
repository-owned authority over what Corvint may see, and a change to it is exactly the kind of change
a trace should carry.

Decided: `record` admits `.gitignore` at any depth as a changed path, in addition to the tracked
context-index sources it admits today. The admitted set stays sorted and digest-bound, and no other
non-source file is admitted by this record; a future non-source file needs its own amendment naming
it. Admission does not make `.gitignore` an index source: it is never parsed, never indexed, and
contributes no symbols.

## Requirements

- `GPK-V0-051`: `corvint impact --base FULL_COMMIT_ID` admits a `C` or `R` range member when Git
  reports its similarity score and source path, binding status, similarity, source path, and source
  blob beside the member's own fields, computing admitted hunks against the named source blob.
  A copy or rename without a reported source stays `unsupported-impact-range`. Every other
  `GPK-V0-030` rejection is unchanged.
- `GPK-V0-050`: `corvint record` and `corvint dogfood-record` admit a changed `.gitignore` at any
  depth as a changed path beside tracked context-index sources. The file is never indexed or parsed.
  No other non-source repository file is admitted without an amendment naming it.

## Rollback

Restore the `GPK-V0-030` sentence that rejects rename and copy, remove `GPK-V0-051` and
`GPK-V0-050`, regenerate `docs/specs/REQUIREMENTS.tsv`, revert the code and tests that land against
them, and remove this record from `docs/decisions/README.md`. Both gaps return to their
`ideas.md` entries.
