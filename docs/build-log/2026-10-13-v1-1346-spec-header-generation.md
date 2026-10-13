# 2026-10-13 Generated spec header rows and a per-requirement acceptance mode (V1-1346, V1-1347)

Branch `claude/v1-1346-spec-header-generation` from `origin/main` 523fa4ce6 (the merge of #859).
Requirements `SRG-V0-009..013` in `docs/specs/spec-requirement-index-generator-v0.md`, each
written under the owner's standing auto-accept of 2026-10-11 and therefore `standing` until the
owner reads them back. They implement clauses (c) and (d) of `PRS-V1-018`. Tickets V1-1346 and
V1-1347 stay open here; the orchestrator holds their lease and completes them from the merge.

## Intent

Two lanes that add requirements to one spec used to collide at five places: the spec's
requirement list, the header `Intent status:` tail, the Agent digest `Status:` line, the
INDEX.json and README.md rows, and the generated `REQUIREMENTS.tsv`. Only the first is real
overlap. The resolution rule for the rest lived in agent memory (V1-1346). Separately, since the
2026-10-11 standing auto-accept every lane writes `accepted` for its own requirements, so the word
no longer separates intent the owner read from intent an agent wrote (V1-1347).

This change makes the header and digest hold only the current status plus a pointer to where the
history lives. `make spec-requirements` now projects INDEX.json and the README.md table from the
staged spec headers, so a lane edits only its spec. Each requirement row also records its
acceptance mode, and one audit lists what the owner has not read back.

## Mode and parse rule

The mode comes from the leading parenthetical of a requirement's definition line, which is the
rank-0 title the generator already selects. The parenthetical runs to its balanced close or to the
end of the line, and the closed modes are:

- `standing` if it contains the word `standing`.
- `owner` if, once negated phrases are removed, it pairs an affirmative word (`accepted`,
  `approved`, `owner decision`) with a decision number or an ISO date.
- `unrecorded` otherwise, which covers every legacy line without a marker.

`acceptedBy` joins the anchors the parenthetical names (`decision NNNN`, a date, a `V<n>-NNNN`
ticket, `#N`) with `; `, and is `-` for `unrecorded` or when no anchor is present. The TSV header
becomes `id file line title acceptance acceptedBy`, tab-separated.

## Decisions

- **A per-requirement marker instead of a new status word.** A header word such as
  `accepted-standing` describes the whole spec. One spec routinely mixes owner-read and
  agent-written requirements, so a spec-level word would be wrong for some of them. The inline
  marker was already being written by every lane since 2026-10-11, so parsing it records the
  existing practice without a mass edit. A new header word would also have created another shared
  line for every lane to append to.
- **Generation, not un-committing the TSV.** Leaving REQUIREMENTS.tsv untracked would break every
  reader that opens it from a checkout, including `AGENTS.md` routing, `internal/specindex`,
  `script/spec-coverage-audit` and the OCM spec test. It would also stop a reviewer from seeing a
  requirement move in the diff. A committed file that `spec-requirements-check` binds to its
  regeneration keeps both. The same reasoning extends generation to INDEX.json and the README
  table instead of deleting them.
- **Only intent, delivery and claim are projected.** Title, requirement prefix, `dependsOn`,
  `supersededBy`, `closableToday`, `implementation` and the README Scope, Contract and
  description-tail cells stay index-owned and byte-preserved. Deriving the others would rewrite
  committed bytes on BASE:
  - 6 of the 202 INDEX.json titles differ from their spec's H1.
  - 144 specs declare no `Requirement prefix:`.
  - Only 3 specs name a dependency in their first 25 lines, while 174 INDEX.json entries carry a
    non-empty `dependsOn`.
- **The README table is found by its header row, not by marker comments.** Adding marker comments
  would itself have changed README.md, so BASE could no longer regenerate byte-identically. The
  table is the only one whose header row starts `| Scope | Contract | Intent | Delivery |`.
- **A claim must be one sentence.** The README description starts with the digest claim followed by
  a hand-written tail. The generator splits the two at the claim's first sentence end, `[.?!]`
  followed by whitespace or the end of the text. It refuses a multi-sentence claim, because such a
  claim cannot be split from the tail without guessing.
- **A spec new to the index is appended with defaults.** That is the one remaining shared insertion
  point. It is a single row at the end of INDEX.json and of the table, which two new specs in two
  lanes can still conflict on.
- **The audit is a script, not `corvint docs requirements`.** This is a deviation from the charter's
  first choice. `corvint docs` has no requirements verb, and adding one would mean editing
  `main.go`, `help.go` and the receipt pins, would bring SDD-V0/CCF ownership into scope, and would
  ship a repository-specific convention in the product binary (AGENTS.md invariant 6). The charter
  names `script/spec-acceptance-audit.sh` as the fallback. It is perl in `sh`, with no Python.
- **`UNPARSED_STANDING` instead of a silent undercount.** A marker that follows the text rather than
  leading it is `unrecorded` by the parse rule. The audit counts and names each such requirement
  whose definition block holds an `accepted under ... standing` marker. The first heuristic matched
  any `standing approval` phrase and so counted `PRS-V1-018`, which only describes the feature.
  The fixture now pins that a clause mentioning a standing approval is not counted.

## Failure modes

The SRG spec's failure table holds the full list, which is 11 new rows. The important ones:

- A malformed or unbalanced parenthetical gives `unrecorded`, never a guessed mode.
- A negated phrase (`not accepted`, `never approved`, `not an owner decision`) cannot make a line
  `owner`, even when an inner parenthesis splits it, as in `(not (yet) accepted, decision 0478)`.
- A decision anchor does not swallow the year of an ISO date (`owner decision 2026-10-09` names a
  date, not decision 2026).
- A stale INDEX.json, README.md or TSV fails `spec-requirements-check` with one message naming all
  three.
- A failure in any of the three generators copies nothing.
- A two-sentence claim is refused with a non-zero exit that names the spec, and an unknown
  generator mode exits 2.
- The audit exits 0 whatever it finds, 2 for a mode outside the closed set, and 1 for an
  unreadable or malformed TSV or INDEX.json, including a six-column row whose id, file or line is
  malformed.

## Acceptance evidence

Byte fidelity on BASE: the new generator was copied into a clone checked out at 523fa4ce6.

- `--index` was byte-identical to the committed INDEX.json.
- `--readme` was byte-identical to the committed README.md.
- The TSV's first four columns were identical for all 3,780 rows, and the header gained the two
  columns.

Audit counts on BASE, using the new generator and audit:

| Mode | Requirements | Specs |
| --- | --- | --- |
| `owner` | 247 | 31 |
| `standing` | 20 | 4 |
| `unrecorded` | 3,513 | 197 |
| `UNPARSED_STANDING` (inside `unrecorded`) | 52 | 5 |
| `NOT_PROMOTABLE` | 0 | 0 |

The 52 trailing markers are in `application-navigation-ledger-v0` (10), `cem-reviewer-trial-v0`
(6), `corvint-tasks-obligation-ledger-v0` (10), `daily-change-evidence-workflow-v0` (1) and
`documentation-corpus-v1` (25). After this change there are 3,785 rows: `standing` is 25 in
5 specs (SRG-V0-009..013 added), `owner` and `unrecorded` are unchanged, and `make
spec-requirements` is a fixed point.

Tests:

- `script/gen-spec-requirements_test.sh`:
  - Case 6 covers the mode rule over twelve marker shapes.
  - Case 7 checks the projection against a full expected INDEX.json and README.md, including the
    fixed point.
  - Case 8 covers the audit output, `UNPARSED_STANDING`, `NOT_PROMOTABLE`, the refused mode
    (exit 2), a TSV without the acceptance columns and a six-column row with an empty id (exit 1
    each), and the false-positive guard.
- `script/spec-requirements-conflict_test.sh` (SRG-V0-013) builds a temporary repository with
  three lanes from one base: two add a requirement to the same spec and one adds to another spec.
  - The same-spec merge conflicts in exactly that spec and REQUIREMENTS.tsv.
  - The markers lie inside `## Requirements` and the TSV conflict lines are only the two new rows.
  - The resolved tree regenerates to itself, and the other-spec lane merges cleanly.
  - Swapping the projection for a hand-appended digest line makes the test fail, so it
    discriminates.
  - The test runs the real `make spec-requirements` target, so it is declared in
    `.corvint/unbounded-readers.json` and every affected plan selects it. It takes about 2.4 s.
- `internal/specindex` `TestRequirementsAcceptanceColumns` pins the header and the per-mode
  `acceptedBy` shape of the committed TSV.

The two TSV consumers that matched the whole header or an exact column count now check the first
four columns: `script/spec-coverage-audit/main.go` and `internal/lrfrepo/ocm_spec_test.go`. That
edit is outside the charter's file list and is the smallest change that keeps them reading the
same columns. The other Go readers need no change, because each one splits rows on tabs and
requires only a minimum field count: `internal/dashboard/roadmap` and `internal/console` need 4,
`internal/compactionkernel` 3 and `internal/dogfoodflow` 2. The shell readers
`check-requirement-definitions.sh` and `check-line-citations.sh` pass in the doc gates below.
Decision 0105 holds one anchored citation of the generator, and its cited comment moved from line
113 to line 212. The content hash `ec359190` is unchanged, so only the line number was repinned.

The lane then met a real merge. `origin/main` moved to 77f9364e3 (#862, V1-1345), which added OCM
requirements and edited the OCM header, INDEX.json and README.md rows. Merging it into this branch
auto-merged INDEX.json and README.md and conflicted only in REQUIREMENTS.tsv, whose every row this
change rewrites. `make spec-requirements` resolved that conflict, and the result is a fixed point
that passes `spec-requirements-check`. On the merged tree the audit counts `standing` 27 in 6
specs, with `owner`, `unrecorded` and `UNPARSED_STANDING` unchanged, and the change is bound against
77f9364e3.

## Independent review

One Codex round (`codex exec -s read-only`) reviewed the commit against five criteria: generator
byte-fidelity, unchanged TSV consumers, no Python, closed modes and the SDD text against invariant
8. Its verdict was REJECT, with one blocker and four should-fix findings.

Codex confirmed that every TSV reader it inspected still reads the original columns, that the change
adds no Python, that the status guidance keeps generated rows from granting intent and requires an
owner read-back for promotion, and that the conflict test checks the conflicted paths. Its sandbox
refused the generator's here-document temporary file, so it could not replay the BASE regeneration.
The fidelity evidence above is this lane's own run.

- **Blocker, fixed.** `(not (yet) accepted, decision 0478)` parsed as `owner`, because the negation
  removal stopped at the inner parenthesis. The removal now runs to the nearest `;` or `,` across
  inner parentheses, and it also covers `owner decision`. Fixture rows `ACC-011` and `ACC-012` fail
  against the old pattern and pass now. Regenerating the live corpus changed no row, so the counts
  above stand.
- **Audit row shape, fixed.** A six-column row with an empty id or file, or a non-numeric line,
  passed the audit. It now requires a requirement id, a `docs/specs/*.md` file and a positive line,
  and exits 1 otherwise. The new case 8 check fails against the old audit.
- **INDEX.json layout, recorded.** `--index` decodes INDEX.json and writes it back in one canonical
  layout, so a valid hand edit in another layout or escape form is normalized. The values are kept.
  `SRG-V0-012` now says the values are kept in one canonical layout, and the failure table names the
  case. A layout-preserving JSON editor was not worth its size here, because the check already
  makes any such edit visible.
- **README claim boundary, recorded.** The old claim is cut at its first sentence end. A digest
  Claim cannot contain `.`, `?` or `!` followed by whitespace, so a generated description always
  splits correctly. Only a hand-edited description whose first sentence holds an abbreviation such
  as `Dr.` mis-splits, and the regenerated README.md diff shows it. This is in the failure table and
  the limits.
- **Superseded new spec, recorded.** A spec new to the index whose Agent digest is superseded has no
  `Claim`, so appending it is refused, naming the spec. The refusal fails closed. Its entry and row
  are added by hand once, and only its statuses are projected after that. This is in the failure
  table and the limits.

## Limits

- Existing header tails are not migrated. Only this spec's header was cut back to the current
  status and the history pointer, and the other specs keep their appended history until their
  owners edit them.
- Promotion from `standing` to `owner` is manual. It is a marker edit in the same commit as a
  build-log Owner acceptances entry, and no tool checks that pairing yet.
- The 52 trailing markers stay `unrecorded` until they are moved to the front of their lines.
  That is a change to five other specs and is left as a follow-up.
- Only the first, leading parenthetical is read. A second marker later in the line is ignored.
- `NOT_PROMOTABLE` is a report and fails no gate.
- A spec new to the index with a superseded Agent digest is refused rather than appended. Its
  INDEX.json entry and README.md row are added by hand once.
- A hand-edited README.md description whose first sentence holds an abbreviation is mis-split at
  the abbreviation. The regenerated diff shows it.
- INDEX.json is written in one canonical layout, so a hand edit in another layout is normalized.
- REQUIREMENTS.tsv grew from 504,963 to 559,657 bytes. The compaction kernel reads it through
  `internal/contextindex`, whose `maxSourceBytes` cap is 1,000,000 bytes, so the file is still
  under the cap but closer to it.
- At session start, Corvint's local hook reported `FALLBACK degraded:
  corvint-event-rejected:dogfood-event-deadline`. Coding continued, and the bind and seal below
  run through the Make targets.

## Rollback

Revert the commit. The generator returns to four columns and the Make targets to the TSV alone.
INDEX.json and README.md are then hand-edited again. Because projection was byte-identical on BASE,
reverting changes no other spec's row. The two consumer edits are compatible with both TSV shapes,
so they can stay or go.
