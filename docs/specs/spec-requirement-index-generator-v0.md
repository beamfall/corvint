# Spec requirement index generator V0

Owner: Russell Lewis
Date: 2026-09-12
Requirement prefix: `SRG-V0`
Intent status: accepted (history: REQUIREMENTS.tsv acceptance columns and docs/build-log)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariant 8, `../SPEC-DRIVEN-DEVELOPMENT.md` "Required
capability-spec shape", `decision-number-gate-v0.md` (whose drift rule pins that a gate must
measure what a fresh clone contains), `requirement-definition-gate-v0.md` (the consumer that
reads every row this generator emits), and `../agent-memory/fixes.md` (which lists the gate
scripts still without an owning spec).

## Agent digest
- Claim: One generator derives, from the staged specs, the requirement index with each requirement's acceptance mode and the INDEX.json and README.md status rows.
- Status: accepted (history: REQUIREMENTS.tsv acceptance columns and docs/build-log) / implemented
- Exists: `script/gen-spec-requirements.sh` (the index, `--index`, `--readme`, `--all DIR` for all three in one pass, `--write` and `--check` for the make targets, and the uncommitted `--lines` locator view), `script/spec-acceptance-audit.sh`, `make` targets `spec-requirements`, `spec-requirements-check` and `spec-requirements-test` (the last two in `make gate`) and `spec-acceptance-audit` (a report).
- Blocked on: nothing for this gate. Decision 0105 withheld acceptance on `SRG-V0-005`; its amendment (2026-09-12) accepted the spec once character truncation landed with evidence. SRG-V0-009..018 are `standing` until the owner reads them back.
- Read next: Verified current state; Requirements; Trust boundary, limits, and failure modes.

## Human intent and scope

`docs/specs/REQUIREMENTS.tsv` is the repository's lookup from a requirement id to the file that
defines it; the defining line is derived on demand (SRG-V0-015, SRG-V0-017). `AGENTS.md` routes agents to it before opening any spec body, and
`requirement-definition-gate-v0.md` reads every row it emits to prove no id survives as a table
row alone. The index is therefore load-bearing for agent routing and for a second gate, and it is
generated rather than hand-maintained so that a spec edit cannot silently desynchronise it.

`script/gen-spec-requirements.sh` is that generator. It prints the whole index to stdout;
`spec-requirements-check` regenerates it into a temporary file and byte-compares that against the
committed `docs/specs/REQUIREMENTS.tsv`, so the gate fails whenever the committed index does not
match what the current spec corpus would produce. With `--index` and `--readme` the same script
projects the spec headers onto `docs/specs/INDEX.json` and the `docs/specs/README.md` spec table,
and `make spec-requirements` writes all three.

Two lanes adding requirements to one spec used to conflict at four shared insertion points: the
header `Intent status:` tail, the Agent digest `Status:` line, the INDEX.json and README.md rows,
and the generated TSV (V1-1346). And since the owner's standing auto-accept of 2026-10-11 every lane
writes `accepted` for its own requirements, so the header word no longer separates intent the owner
read from intent an agent wrote (V1-1347). SRG-V0-009..013 move status history out of the header,
generate the rows, and record a closed acceptance mode per requirement with an audit of the
requirements the owner has not read back. They are 1.0 scope under `PRS-V1-018` clauses (c) and
(d) in `corvint-1.0-product-and-release-v1.md`.

Affected user: an agent or engineer adding, renaming, or moving a requirement id. Measurable job:
for the spec corpus a fresh clone contains, emit exactly one row per requirement id naming the
highest-ranked definition site, deterministically, so that the committed index is reproducible
from the sources alone.

The owner instruction of 2026-09-07 places repository gate tooling in scope for AGENTS.md
invariant 8. This spec discharges that for the requirement-index generator only.

## Verified current state

At `0ec169f1` the generator is wired into `make gate` through `spec-requirements-check`, and that
target passes: the committed `docs/specs/REQUIREMENTS.tsv` is byte-identical to a fresh
regeneration.

SRG-V0-001 is met as of this commit. The generator derives its input set from
`git ls-files -z -- docs/specs` and reads each file's bytes as `:<path>` through one pipelined
`git cat-file --batch` (`index_blobs` in `script/gen-spec-requirements.sh`, SRG-V0-018), so neither an untracked spec, a tracked spec deleted from
the worktree alone, nor an unstaged edit can change the index a fresh clone reproduces. Before
this commit the enumeration was the worktree glob `sort glob('docs/specs/*.md')`, the same class
of defect that `decision-number-gate-v0.md` pins in its drift rule and that
`check-decision-numbers.sh` was repaired for: the gate must measure the set a fresh clone
contains (`Makefile:14`), and a worktree glob does not.

The defect was reproduced at `5b496119` in both directions, and both directions are now covered by
`script/gen-spec-requirements_test.sh`. An untracked `docs/specs/*.md` file holding a requirement
id added a row to the generated index and made `spec-requirements-check` fail locally although a
fresh clone of the same commit passed. The inverse was the harmful direction: regenerating the
index in a worktree carrying an untracked or not-yet-staged spec and committing the resulting
`REQUIREMENTS.tsv` produced an index no fresh clone could reproduce, so the gate failed for
everyone else at a commit whose author saw it pass. A tracked spec deleted from the worktree but
not staged was likewise dropped. Regenerating at this commit reproduced the committed
`REQUIREMENTS.tsv` byte for byte, so the repair moved no row.

At `523fa4ce` (the base of V1-1346 and V1-1347) the generator's `--index` and `--readme` output was
byte-identical to the committed `docs/specs/INDEX.json` and `docs/specs/README.md`, so projecting
the headers moved no byte, and the regenerated `REQUIREMENTS.tsv` kept the first four columns of
all 3,780 rows. Of those rows 247 were `owner`, 20 `standing` and 3,513 `unrecorded`; a further 52
`unrecorded` requirements in five specs carried a standing marker after their text rather than
before it, which the audit names as `UNPARSED_STANDING`.

No requirement below diverges from the implementation.

## Requirements

- `SRG-V0-001`: The generator MUST derive its input set from the paths Git reports as tracked
  under `docs/specs/` matching `*.md`, not from a worktree directory listing, so that an untracked
  file contributes no row and a tracked file deleted from the worktree alone still contributes its
  rows. It MUST exclude `docs/specs/README.md`, which is the index of specs rather than a spec.
- `SRG-V0-002`: (accepted, decision 0506) The generator MUST recognise a requirement id as a token matching an uppercase
  letter followed by uppercase letters, digits, and hyphens, then a hyphen and exactly three
  digits, bounded by word boundaries (`script/gen-spec-requirements.sh:433@e23d2d1f`). An id with a suffix
  that is not exactly three digits MUST NOT be indexed. Matching the token alone MUST NOT
  establish a definition; the occurrence MUST also meet SRG-V0-003.
  Amended 2026-11-01 by SRG-V0-790 (V1-3793): the token accepts three or more digits, and an ID of
  four or more digits is indexed only under a prefix that has a three-digit ID.
- `SRG-V0-003`: (accepted, decision 0506) The generator MUST classify each occurrence of an id into exactly one of three
  ranks: rank 0 when the line begins with the id, optionally preceded by a list bullet and
  optionally wrapped in bold or backticks, and followed either by a period and whitespace or by a
  colon; rank 1 when the line begins with a table cell holding the id; rank 2 otherwise. It MUST
  discard every rank 2 occurrence, so a passing mention of an id in prose never becomes its
  indexed location. An unmarked definition (no list bullet, bold, or backtick prefix) MUST
  begin a paragraph: the first line of the file or a line following a blank line. A wrapped
  prose continuation MUST NOT become rank 0 merely because its first token matches the ID
  pattern and separator. Marked definitions and table-cell ranking remain unchanged.
- `SRG-V0-004`: Among retained occurrences of one id the generator MUST select exactly one
  winner, ordering by rank ascending, then by file path ascending, then by line number ascending
  with the number zero-padded so the comparison is numeric rather than lexical. The first
  occurrence in that order MUST win and a later equal-ranked occurrence MUST NOT displace it.
- `SRG-V0-005`: For a rank 0 winner the generator MUST strip the leading bullet, emphasis,
  backticks, id, and its period or colon separator from the title; for a rank 1 winner it MUST
  retain the line as found. In both cases it MUST collapse every tab, carriage return, and newline
  to a single space, trim leading and trailing whitespace, truncate the result to at most 80
  characters, and trim any trailing whitespace the truncation exposes.
- `SRG-V0-006`: The generator MUST write to stdout a single header line `id`, `file`, `line`,
  `title` separated by tabs, followed by one tab-separated row per indexed id, ordered by id under
  a collation that does not vary with the caller's locale. Each row MUST carry the winner's
  repository-relative path and its one-based line number. Amended by SRG-V0-009, which appends
  the columns `acceptance` and `acceptedBy` after `title`, and by SRG-V0-015, under which this
  id-ordered, line-bearing layout is the uncommitted `--lines` view and the committed index has no
  line column.
- `SRG-V0-007`: The generator MUST write no file, mutate no repository state, and produce
  identical output regardless of the working directory it is invoked from, resolving its own
  location to find the repository root first. Amended by SRG-V0-018: `--all DIR` writes its three
  named files into the directory its caller names, and nothing else.
- `SRG-V0-008`: `spec-requirements-check` MUST regenerate the index and byte-compare it against
  the committed `docs/specs/REQUIREMENTS.tsv`, failing on any difference, and MUST remove its
  temporary file on normal exit and on interrupt. `docs/specs/REQUIREMENTS.tsv` MUST be
  regenerated whole by this generator and MUST NOT be hand-edited. SRG-V0-012 extends the
  comparison to `docs/specs/INDEX.json` and `docs/specs/README.md`.
- `SRG-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1347) For every
  rank 0 winner the generator MUST derive a closed acceptance mode from the leading parenthetical of
  its stripped, untruncated definition text: from an opening parenthesis at the start of that text
  to its balanced closing parenthesis, or to the end of the line when none closes. The mode MUST be
  `standing` when the parenthetical holds the word `standing`; otherwise `owner` when, once every
  phrase negating `accepted`, `approved` or `owner decision` with `not` or `never` is removed (up to
  the nearest `;` or `,` and across any inner parenthesis), it holds `accepted`, `approved` or
  `owner decision` together with a decision number (`decision NNNN`) or an ISO date;
  otherwise, and for every rank 1 winner and every definition with no leading parenthetical,
  `unrecorded`, which inherits the spec header's intent status. `acceptedBy` MUST list the
  decision numbers (as `decision NNNN`), ISO dates, ticket ids (`V<n>-NNNN`) and issue numbers
  (`#N`) the parenthetical names, in order of first appearance and without repeats, joined by
  `; `, or `-` when it names none or the mode is `unrecorded`. The header MUST append `acceptance`
  and `acceptedBy` after `title`, so the first four columns of every row are unchanged.
- `SRG-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1347)
  `script/spec-acceptance-audit.sh` (`make spec-acceptance-audit`) MUST list the requirements of one
  mode, `standing` unless `--acceptance owner` or `--acceptance unrecorded` is given, grouped by spec
  with a count per spec and totals for all three modes. It MUST report each spec whose INDEX.json
  delivery status begins with `validated` and that holds a `standing` requirement as
  `NOT_PROMOTABLE`, and MUST count, naming them in the `standing` listing, the `unrecorded`
  requirements whose definition block holds an `accepted under ... standing` marker outside its
  leading parenthetical as `UNPARSED_STANDING`. It MUST exit 0 whatever it finds, 2 for a mode
  outside the closed set, and 1 for an unreadable or malformed input, including a row whose id, file
  or line is malformed. A `standing` marker MUST become `owner` only through an edit to the marker
  in the same commit as the build-log Owner acceptances entry that records the owner's read-back; a
  lane MUST NOT write an `owner` marker for its own requirement. Amended by SRG-V0-017: a committed
  row has no line, so the listed `file:line` takes the line from the `--lines` view, and a row the
  staged specs no longer define is malformed input.
- `SRG-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1346) A spec
  header's `Intent status:` and its Agent digest `Status:` MUST hold the current status only,
  optionally followed by the pointer `(history: REQUIREMENTS.tsv acceptance columns and
  docs/build-log)`. A change that accepts or amends requirements MUST record that in each
  requirement's marker and its build-log entry rather than by appending to the header. Existing
  header tails stay until a change touches that header.
- `SRG-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1346)
  `make spec-requirements` MUST regenerate `docs/specs/REQUIREMENTS.tsv`, `docs/specs/INDEX.json`
  and the `docs/specs/README.md` spec table from the staged specs, copying nothing unless all three
  regenerate, and `spec-requirements-check` MUST compare all three and fail when any differs. The
  projection MUST take each spec's intent and delivery status from its header and its claim from its
  Agent digest; MUST keep the value of every other INDEX.json field (written in one canonical
  layout), every other README.md cell, the tail of each README.md description after its
  one-sentence claim, and every byte outside the table unchanged;
  MUST append an entry and a row built from the header for a tracked spec that has neither; and
  MUST refuse a header value holding a table pipe or control character, a claim that is not one
  sentence ending in `.`, `?` or `!`, and an INDEX.json entry that is malformed, duplicated or
  names an untracked spec. Where the rows already agree with the headers the output MUST be
  byte-identical to the committed files.
- `SRG-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1346) Two changes
  from one base that each add one requirement to the same spec and run `make spec-requirements`
  MUST conflict, if at all, only inside that spec's requirement list and the REQUIREMENTS.tsv rows;
  the header, the Agent digest, INDEX.json and README.md MUST merge cleanly. The TSV conflict MUST be
  resolved by resolving the spec and regenerating, never by hand, and a change on another spec MUST
  merge with no conflict. `script/spec-requirements-conflict_test.sh` MUST show this through the
  real `make` target. Extended by SRG-V0-016.
- `SRG-V0-014`: (accepted under the owner's standing auto-accept (2026-10-15); V1-1409) A spec whose
  `Delivery status:` header holds more than one word (a qualified delivery) MUST have its Agent
  digest `Status:` carry that header text verbatim, followed by nothing or by a sentence boundary
  (`.`). Anything else after it, a `;`, `,` or further words, is a stale or truncated header and
  `TestIndexCoversSpecsAndHeaders` MUST fail. A one-word header (`experimental`, `deferred`) may be
  elaborated in the digest `Status:` after the word. Elaboration after a qualified header starts a
  new sentence.
- `SRG-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1525) The
  committed `docs/specs/REQUIREMENTS.tsv` MUST carry the header `id`, `file`, `title`,
  `acceptance`, `acceptedBy` and no line column, with one row per indexed id ordered by file path
  ascending, then by the winner's line ascending, then by id, under a collation that does not vary
  with the caller's locale. `script/gen-spec-requirements.sh --lines` MUST print the derived
  locator view: the SRG-V0-006 header with the SRG-V0-009 columns, one row per indexed id ordered by
  id, each with its winner's line. The view MUST NOT be committed, and `spec-requirements-check`
  MUST NOT compare it. An edit that only shifts a spec's lines therefore rewrites no committed row.
- `SRG-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1525) Two changes
  from one base that each run `make spec-requirements` MUST merge in either order with no textual
  conflict in `REQUIREMENTS.tsv`, `INDEX.json` or `README.md`, and each merged generated file MUST
  equal its regeneration, when the changes add requirements to different specs, or add requirements
  to one spec whose new rows are separated in the committed order by at least one row neither
  change adds, or when one only edits prose that leaves every indexed field (`id`, `file`,
  `title`, `acceptance`, `acceptedBy`) of every row unchanged. This holds even where each change
  shifts the other's definition lines. `script/spec-requirements-conflict_test.sh` MUST show each
  case through the real `make` target. The guarantee does not cover these cases:
  - New rows that land next to each other in the committed order, for example two changes that
    both append to one section, or one that appends at a section's end while the other inserts at
    the next section's start with no row between.
  - Two new specs.
  - An edit that changes an indexed field next to another change's new row. For example, a
    reflow that keeps a requirement's meaning but changes its first line, and so its title.
    `script/spec-requirements-conflict_test.sh` shows that case as an expected conflict.

  Each such conflict is resolved by resolving and staging the spec, then running
  `make spec-requirements-resolve` (SRG-V0-041), which regenerates the index as SRG-V0-013 states.
- `SRG-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1525) A reader
  that needs a requirement's line MUST take it from the spec, never from the committed index. Go
  readers MUST use `specindex.DefinitionLine`, which applies the SRG-V0-003 and SRG-V0-004 ranks
  to the winner's file: its first rank 0 line, else its first rank 1 line. Shell readers MUST use
  the `--lines` view, and MUST join on both the id and the file: a committed row whose file is not
  the one the view defines its id in is stale input, never a citation of the row's file at the
  other file's line. Go readers MUST parse the index through `specindex.ParseIndex`, which reads
  both the committed layout and a line-bearing one, so that an index pinned before SRG-V0-015 still
  resolves. An id that its spec no longer defines MUST surface as stale input: a console gap, a
  compaction-kernel refusal, a roadmap link with no line, or an audit exit 1. It MUST NOT be given
  an invented line. These readers are the console clause lookup, the compaction kernel (from the
  pinned spec blob), the roadmap evidence links, `script/spec-coverage-audit` and
  `script/spec-acceptance-audit.sh`.
- `SRG-V0-018`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1641) `--all DIR`
  MUST write `REQUIREMENTS.tsv`, `INDEX.json` and `README.md` into the existing directory DIR from
  one enumeration and one read of the staged specs, each byte-identical to what the default,
  `--index` and `--readme` modes print for the same Git index. It MUST build all three before
  writing any, so a refused projection writes nothing, and MUST exit 2 when DIR is not given or
  does not exist. `spec-requirements` and `spec-requirements-check` MUST generate through one
  `--all` invocation. Every mode MUST read its index blobs through one `git cat-file --batch`
  co-process with the requests pipelined, written by a separate writer while the replies are read,
  never one synchronous request per file. A path the batch cannot frame or serve as a blob falls
  back to the one-shot `git cat-file blob` read, which fails as the per-file read did. SRG-V0-001
  is unchanged: the input is still the Git index alone.
- `SRG-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2141) Every
  generator mode MUST refuse an Agent digest Claim longer than 160 characters, the bound the
  `internal/specindex` INDEX test enforces, naming the spec path and the claim's character length, so
  `make spec-requirements` fails at the first spec step rather than at `spec-requirements-check`.
- `SRG-V0-041`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1758) During a
  merge, `make spec-requirements-resolve` (`script/spec-requirements-resolve.sh`) MUST exit 1 and
  name the paths when any unmerged path is not `docs/specs/REQUIREMENTS.tsv`, `INDEX.json` or
  `README.md` (with no path unmerged it regenerates, `SRG-V0-920`). Otherwise it MUST restart each unmerged generated file
  from the current branch's side, regenerate all three from the staged specs exactly as
  `make spec-requirements` does, and stage them, so the merged files equal a fresh generation. This
  is the one-command resolution for the SRG-V0-016 limits: two changes that append adjacent rows to
  one section (after the conflicted spec is resolved and staged) and two changes that each add a
  new spec. `script/spec-requirements-conflict_test.sh` MUST show both cases, the second merged in
  both orders. Non-goal: it does not merge a conflicted spec, and a new spec from the other side
  gets the INDEX.json entry and README.md row its header derives, so hand-set fields of that entry
  are re-applied by hand. Rollback: delete the script and the target; the manual
  take-a-side-and-regenerate resolution is unchanged.
- `SRG-V0-042`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2160) Every
  generator mode MUST refuse, before writing anything, a rank 0 winning definition whose text opens
  with an acceptance note SRG-V0-009 would misread, printing `<file>:<line>: <id>: <reason>` for
  each and then the accepted forms. The near misses are: a leading parenthetical that names an
  auto-acceptance (`auto-accept`, `auto-acceptance`) but does not classify as `standing`; a leading
  parenthetical that affirms `accepted`, `approved` or `owner decision` (after the negation rule)
  but records no decision number or date, so it classifies as `unrecorded`; and an unparenthesized
  leading `accepted` or `approved` followed by `,`, `:`, `by`, `under`, `on`, `(` or a date, which
  the reader never sees. A leading adjective such as "Accepted closed inputs" and a note anywhere
  but the start of the definition are not near misses, except the trailing `Status:` note of
  SRG-V0-044 (the audit's `UNPARSED_STANDING` still reports a trailing standing note). The accepted forms are listed in
  `docs/SPEC-DRIVEN-DEVELOPMENT.md`. Non-goal: no new acceptance form is accepted, and no note is
  rewritten. Rollback: delete `acceptance_near_miss` and its refusal block; the misread notes
  silently classify again.
- `SRG-V0-043`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2146) In every
  README row SRG-V0-012 projects, a requirement range written right after the Contract cell's link,
  as `PREFIX-NNN` or `PREFIX-NNN..MMM` and followed only by the cell end or a `;`, MUST be rewritten
  to the lowest and highest ids of that prefix the spec defines (one id prints alone), so
  `spec-requirements-check` fails when a row's range disagrees with REQUIREMENTS.tsv. A range whose
  prefix the spec defines no id of MUST be refused naming the spec and the prefix. A row with no
  range keeps none, and the range is a span: it does not list gaps. Rollback: delete the range
  block in `readme_text`; the ranges become hand-kept again.
- `SRG-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2309) The
  SRG-V0-042 refusal MUST also name a rank 0 winning definition whose definition line carries a
  trailing `Status: (accepted ...)` or `Status: (approved ...)` note, with the reason that the note
  trails the definition as `Status: (...)` and is not read, because SRG-V0-009 reads only the
  leading parenthetical and such a definition otherwise classifies as `unrecorded`. Non-goal: a
  note on a wrapped continuation line is not seen, since the reader keeps only the definition line;
  no note is rewritten. Rollback: delete the trailing-note branch of `acceptance_near_miss`; the
  trailing notes silently classify as `unrecorded` again.
- `SRG-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0515) Every
  spec-metadata input of the index/digest/README contract check `TestIndexCoversSpecsAndHeaders`
  (`docs/specs/INDEX.json`, `docs/specs/README.md`, `docs/specs/REQUIREMENTS.tsv` and every spec an
  INDEX.json entry names) MUST lie in the declared read scope of `internal/specindex` under the
  `InReadScope` predicate `corvint affected` applies, so a change to that metadata alone selects
  the check. The check MUST stay one function over a repository root and MUST fail a fixture whose
  217-character INDEX claim neither the Agent digest nor the README row carries, on length, digest
  and README, and pass the aligned fixture. `spec-requirements-check` byte-compares the generated
  files and then runs the same package; the generator comparison alone is a requirements check, not
  evidence that this contract passed. Selection uncertainty stays explicit: a plan whose
  `plan.scope` is `UNKNOWN` keeps its unknowns. Non-goal: no selector rule changes; this pins the
  declaration and the check. Rollback: delete `internal/specindex/selection_test.go` and its
  `.corvint/test-read-scopes.json` and `.corvint/test-read-scope-literals.json` entries;
  `checkIndex` may stay as the test body.
- `SRG-V0-046`: (accepted under the owner's standing lane auto-accept (2026-10-11); V1-2859) When
  an SRG-V0-042 near miss (an auto-acceptance without `standing`, or an affirmed note with no
  decision number or date) has a leading parenthetical still open at the end of the definition
  line, the refusal MUST give that as its reason instead: only the definition line is read, so a
  decision number, date or `standing` on a later line is not seen, and the note must close on the
  id's line, with the example `(accepted by the owner, owner chat YYYY-MM-DD; V1-NNNN)`. The
  no-anchor reason MUST give the examples `(accepted, decision NNNN; V1-NNNN)` and
  `(accepted YYYY-MM-DD; V1-NNNN)`, and the closing line of the refusal MUST also show that a longer
  clause may carry the anchor, as in `(accepted by the owner, amending X-V0-001 (owner chat,
  YYYY-MM-DD); V1-NNNN)`. A dated owner-chat note nested in a longer clause and closed on the id's
  line classifies as `owner` with its date and ticket, unchanged. Non-goals: the reader still keeps
  only the definition line (SRG-V0-044), no new acceptance form is accepted, no note is rewritten,
  and no definition that passed before is refused. Failure modes: a wrapped note that already
  classifies as `standing` or `owner` from its first line is no near miss and keeps the anchors of
  that line only. Falsifier: a wrapped near miss refused with the no-anchor or auto-acceptance reason,
  or a nested dated note on one line that is refused or not `owner`. Rollback: delete the
  open-note branch and the examples in `acceptance_near_miss` and the refusal block; the earlier
  reasons return.
- `SRG-V0-047`: (proposed; V1-2867; experimental under the standing lane auto-accept) When a
  spec's Agent digest `- Status:` value does not contain both its header `Intent status:` and
  `Delivery status:` values, `TestIndexCoversSpecsAndHeaders` (and so `spec-requirements-check`)
  MUST report one finding that quotes the digest field and both header fields, each prefixed with
  its `path:line`, and states that these fields are edited together. Non-goal: the agreement rule
  itself, the generator and the other digest findings are unchanged. Failure mode: a header the
  check cannot find reports line 0 beside the existing missing-header finding. Rollback: revert
  `statusDriftMessage` and `headerLine` in `internal/specindex/index_test.go` and delete
  `internal/specindex/status_drift_test.go`; the check reports the previous one-line message.
- `SRG-V0-970`: (experimental under the standing lane auto-accept; V1-2830; amends `SRG-V0-047`)
  The SRG-V0-047 finding MUST also name the expected digest line, `- Status: INTENT / DELIVERY`
  built from the two header values, and say that an acceptance or amendment clause belongs on the
  requirement's own definition line (`SRG-V0-011`, `SRG-V0-981`), not in either status field.
  `docs/SPEC-DRIVEN-DEVELOPMENT.md` MUST state the one rule: the digest `Status:` contains the
  header `Intent status:` value verbatim and the `Delivery status:` value, so a clause kept in the
  header must be repeated in the digest, and neither field takes a new clause. Non-goal: the
  agreement rule itself is unchanged, so every existing spec passes unchanged. Rollback: revert
  `statusDriftMessage` to the SRG-V0-047 wording and drop the sentence.
- `SRG-V0-547`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3049)
  When the staged specs hold more than one unwrapped rank 0 definition line for one requirement
  id, in one spec or across specs, the generator MUST exit non-zero in every mode, before writing
  any output, and print one `ID: defined more than once: path:line path:line ...` line per such id
  naming every definition, then say to renumber or remove the extra definitions, so
  `make spec-requirements` and `spec-requirements-check` both fail. Non-goal: comparing the
  definitions' text; an identical repeat is refused too. Failure mode: before this, the
  lowest-ranked definition won and the other was silently dropped, so a collision between two
  lanes was visible only by diffing the merged `REQUIREMENTS.tsv`. Rollback: delete the
  `%definitions` collection and its refusal block in `script/gen-spec-requirements.sh`.
- `SRG-V0-548`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3049)
  A rank 0 line that RDG-V0-023 calls wrapped (an unbulleted line directly after a non-blank,
  non-heading line outside a fence, with the fence tracked as
  `script/check-requirement-definitions.sh` tracks it) MUST rank after every unwrapped rank 0
  line of the same id and MUST NOT count as a definition under SRG-V0-547. Non-goal: a wrapped
  rank 0 line that is an id's only definition keeps its row, as SRG-V0-003 requires. Failure
  mode: before this, a wrapped citation of an id earlier in a spec won
  over the id's real definition, so the row carried the wrong title and acceptance (`AHI-246`,
  `DCI-V1-030`). `specindex.DefinitionLine`, the Go reader of SRG-V0-017, MUST rank the same way.
  Rollback: drop the wrapped flag from the `%best` key, the duplicate count and `DefinitionLine`.
- `SRG-V0-800`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3379;
  refines `SRG-V0-001` and `DCW-V0-352`) `spec-requirements-check`, and so `make doc-gates`, MUST
  also fail when a tracked `docs/specs/*.md` edit is unstaged and regenerating from a private copy
  of the index with those edits added would change `REQUIREMENTS.tsv`, `INDEX.json` or
  `README.md`. The refusal MUST name the stale files, each unstaged spec and the fix (`git add` the
  spec, then rerun `make spec-requirements`). The check MUST NOT change the real index or any
  tracked file, and is skipped under a private `GIT_INDEX_FILE` (`dogfood-finish.sh`), whose
  snapshot is already exact. The generator itself still reads only the index (`SRG-V0-001`).
  Design: running `make spec-requirements` before staging changed nothing, and the check compared
  the same staged view, so `doc-gates` passed with only the `DCW-V0-352` warning while the
  dogfood pre-flight refused `spec-requirements-stale`. Non-goals: untracked specs, which keep the
  warning only; a prose-only unstaged edit that changes no generated byte passes. Failure modes: a
  locked or unreadable index fails the check closed. Rollback: delete
  `script/spec-requirements-worktree-check.sh`, its test and their two `Makefile` lines.
- `SRG-V0-900`: (experimental under the standing lane auto-accept, 2026-10-31; tasks-w1 lane
  friction) `spec-requirements-check`, and so `make doc-gates`, MUST also run
  `TestSpecsStayWithinTheOCMObligationBound` (`internal/lrfrepo`), which fails for every
  `docs/specs/*.md` whose `## Requirements` section `ocm prepare` would refuse as
  `too-many-obligations` (more than 256 requirement IDs, OCM-V0-028), naming the spec and saying
  to define the new requirement elsewhere. Specs already over the bound when the check was added
  are listed in `overBoundSpecs` and must stay over it, so the list cannot go stale. Failure mode:
  before this, adding one requirement to the agent-leases spec, which sat at the bound, passed the
  doc gates and was caught only by `internal/lrfrepo`'s enumeration tests in the closeout
  batch-precheck, after the CEM was sealed. Rollback: drop the test and its Makefile line.
- `SRG-V0-980`: (experimental under the owner's standing auto-accept (2026-10-31); V1-3458) Every
  generator mode MUST refuse, before printing or writing anything, a tracked spec whose header
  `Intent status:` or Agent digest `Status:` names a requirement ID (the SRG-V0-003 token, and
  each `..NNN` or `/NNN` suffix after it, read as the same prefix) that the spec's row in `docs/specs/INTENT-STATUS-FROZEN.tsv` does not list, or whose `Intent status:` is
  longer than 160 characters and longer than that row's frozen length. Each refusal MUST name the
  spec, the field and the ID or the two lengths, and MUST say to record the acceptance in the
  requirement's own leading note (SRG-V0-009, SRG-V0-981) and its build-log entry instead. The
  frozen file is tab-separated `path`, `chars` (the header's character length) and `ids`
  (space-separated), sorted by path, read from the Git index like the specs, and absent means
  empty. It holds a row for every spec whose header or digest `Status:` at the freeze named an ID,
  or whose header exceeded 160 characters; it is never appended to, so a header may shrink or drop
  IDs without an edit to it, and its only edit is moving a renamed spec's row to the new path in
  the rename's commit. The guard is lexical: a segment naming no requirement ID passes while the
  header stays within 160 characters or its frozen length. Non-goal: deciding which old tails to cut back. Failure mode: before this, every
  amendment appended a segment to the one-line header (up to about 14.5 KB), which the generator
  mirrored into the digest, INDEX.json and README.md, so two lanes amending one spec conflicted in
  four files. Rollback: delete the frozen file and the refusal block.
- `SRG-V0-981`: (experimental under the owner's standing auto-accept (2026-10-31); V1-3458) A change
  that amends an existing requirement MUST record the amendment in that requirement's leading note
  on its definition line, naming the standing approval or the owner decision and its anchor, so the
  SRG-V0-009 reader classifies it and REQUIREMENTS.tsv carries its anchors in `acceptedBy`. An
  `owner` requirement amended under the standing approval therefore reads `standing` until the owner
  reads the amendment back. Rollback: none needed; the rule only names where the record goes.
- `SRG-V0-982`: (experimental under the owner's standing auto-accept (2026-10-31); V1-3458) The
  repository `Makefile` MUST declare each phony target on its own `.PHONY:` line, placed with that
  target's rule (kept by review, not checked), and `make spec-requirements-test` MUST fail naming
  each `.PHONY:` line that lists more than one target. Failure mode: one shared `.PHONY:` line of about 1,560 characters took a
  word from every lane that added a target, so any two such lanes conflicted on it. Rollback:
  rejoin the lines and drop the check.
- `SRG-V0-983`: (experimental under the owner's standing auto-accept (2026-10-31); V1-3458)
  `script/spec-requirements-conflict_test.sh` MUST replay two branches of one synthetic repository
  that each amend a different requirement of one spec under SRG-V0-981 and each add a different
  phony make target under SRG-V0-982, and MUST fail unless they merge in both orders with no
  conflict in any file and every merged generated file equals the regenerated one. It MUST also
  record the limit: amendments to adjacent definition lines conflict only in those lines of the
  spec and the two REQUIREMENTS.tsv rows. As negative
  controls it MUST show that the same amendments written as header segments, and the same targets
  appended to one shared `.PHONY:` line, do conflict, and that the generator refuses the new header
  segment under SRG-V0-980. Rollback: drop the case.
- `SRG-V0-890`: (experimental under the standing lane auto-accept, 2026-10-31; V1-3575; refines
  SRG-V0-980) Each SRG-V0-980 ID refusal MUST name the frozen line as `SPEC:LINE` (the line of the
  `Intent status:` header or of the Agent digest `- Status:`), say that the line is frozen by
  `docs/specs/INTENT-STATUS-FROZEN.tsv`, and name where the new ID belongs: its own definition line
  inside the spec's `## Requirements` section, with that section's line range when the spec has
  exactly one. `docs/LANE-ORCHESTRATION.md`'s requirement-ID block paragraph MUST state the rule for
  lane charters: a new range is never named in the frozen `Intent status:` or digest `Status:`
  line. Non-goals: relaxing the freeze.
- `SRG-V0-891`: (experimental under the standing lane auto-accept, 2026-10-31; V1-3295) Every
  generator mode MUST refuse, before printing or writing anything, an OCM-form definition
  (`- `ID`: `, `- **ID:** `, `- **ID.** `) outside a fence after or before the one `## Requirements`
  section of a spec that has exactly one, unless `script/requirement-definitions-outside-section.txt`
  (the frozen RDG-V0-021 baseline, read from the Git index) lists `SPEC ID`, naming `SPEC:LINE`,
  the ID and the section's line range, so `make spec-requirements` refuses what `make doc-gates`
  (`requirement-definitions-check`) would. Non-goals: a second baseline; changing RDG-V0-021.
  Failure mode: a spec with no or several Requirements sections is left to doc-gates, as RDG-V0-021
  leaves it.
- `SRG-V0-920`: (experimental under the owner's standing auto-accept (2026-10-31); V1-3448; amends
  `SRG-V0-041`) With no unmerged path (only a spec conflicted and was resolved, the generated files
  having merged cleanly, or no merge in progress), `make spec-requirements-resolve` MUST regenerate
  the three generated files from the staged specs, stage them, print `no unmerged path; regenerating
  anyway` and exit 0, instead of exiting 1. It still exits 1 naming any unmerged path it does not
  derive.
- `SRG-V0-921`: (experimental under the owner's standing auto-accept (2026-10-31); V1-3338; extends
  `SRG-V0-041`) The resolve MUST also take the use-case receipts and their ledger
  (`conformance/use-cases-v0/ledger.json`, `conformance/use-cases-v0/receipts/**`) as derived: an
  unmerged one restarts from the current side; then, after the spec regeneration, each ledger pin
  whose receipt's current bytes are exactly those the other side's ledger (`MERGE_HEAD`) pins takes
  that pin, `script/repin-use-case-receipts.sh` repins every receipt whose subject changed, and the
  receipts and ledger are staged, so a batch merge of generated files is finished by `make
  spec-requirements-resolve` alone. Design: a `.gitattributes` merge driver needs per-clone
  configuration and would run per file mid-merge, before the specs it derives from are resolved;
  resolving after the merge reads the resolved specs. Non-goals: other derived files (the capability
  catalog, analyzer pins: `make batch-regen`). Failure modes: a pin that is not a unique string in
  the ledger is refused; a receipt neither side pins as it now is stays a repin refusal for review.
  The resolve MUST restart an unmerged receipt or ledger from the current side only when the other
  side's change to it is digest-only: its `MERGE_HEAD` version (stage 3) equals the merge base's
  (stage 1, or the current side's when the base has none) once every `"sha256"` and
  `"repositoryRevision"` value is blanked. Otherwise (a new evidence reference, a status, claim or
  any other field the other side changed, or a deletion on either side) it MUST refuse before
  writing anything, naming each such path for a hand merge, because the reconciliation above only
  carries digests (review fix, 2026-10-31).
- `SRG-V0-922`: (experimental under the owner's standing auto-accept (2026-10-31); V1-3437) `make
  spec-requirements` MUST, before the generator runs, refuse with exit 2 and one line
  `spec-requirements: generated file(s) unmerged or holding conflict markers: PATH... - run make
  spec-requirements-resolve` when a generated file is unmerged or is staged holding a
  conflict-marker line; the resolve restarts such a staged file from `HEAD`. Non-goals: resolving
  within `make spec-requirements` itself.
- `SRG-V0-923`: (experimental under the owner's standing auto-accept (2026-10-31); V1-3362; refines
  `SRG-V0-800`) `make spec-requirements-check` (through
  `script/spec-requirements-worktree-check.sh`, outside a private `GIT_INDEX_FILE`) MUST fail when
  `docs/specs/REQUIREMENTS.tsv`, `INDEX.json` or `README.md` differs between the working tree and
  the index, naming each and the fix (`git add` them after `make spec-requirements`, or restore
  them), because CI compares the committed bytes and a regenerated file left unstaged passed
  locally. Non-goals: comparing against HEAD (a staged regeneration is what the commit will hold).
- `SRG-V0-950`: (experimental under the owner's standing auto-accept (2026-11-01); V1-3575;
  extends `SRG-V0-980`) Each SRG-V0-980 refusal line MUST name the refused field as `path:line`
  (the header `Intent status:` line or the Agent digest `- Status:` line), and the repair text MUST
  say that a new requirement range belongs on each requirement's own definition line (its leading
  note), in the spec's acceptance and traceability rows and in the build-log entry, and never in
  that frozen line. `docs/LANE-ORCHESTRATION.md` section 3 MUST state the same rule for charter
  authors. Failure mode: a lane named `AMAP-V0-780..782` on a Status line and learned the rule only
  from the refusal, which named neither the line nor the place for the range. Rollback: revert the
  message and the paragraph.
- `SRG-V0-951`: (experimental under the owner's standing auto-accept (2026-11-01); V1-3602) One
  generator run MUST report every refusal it can find before exiting, not only the first: the
  claim-length (SRG-V0-040), frozen-header (SRG-V0-980), duplicate-definition (SRG-V0-547),
  acceptance-note (SRG-V0-042, SRG-V0-953) and, in the `--index`, `--readme`, `--all`, `--write`
  and `--check` modes, header-projection refusals are collected, printed one per line with each
  category's repair once, and the run then exits non-zero having printed and written nothing. A
  refusal that leaves no index to project (an unreadable INDEX.json or README.md table) still stops
  at once. Failure mode: two malformed requirement bullets took two runs to find. Rollback: revert
  to the per-category `die`.
- `SRG-V0-952`: (experimental under the owner's standing auto-accept (2026-11-01); V1-3563) An
  unbulleted line indented by at least two spaces that starts a paragraph after a blank line, while
  a list item is still open (no unindented paragraph, heading or fence has followed the last
  `- ` or `* ` item), continues that item, so a marked ID that starts it, such as a note paragraph
  opening `` `X-V0-001`: ``, ranks like a SRG-V0-548 wrapped citation: never a second definition and
  never chosen over a real one. A genuine second definition (a bullet, or an unindented paragraph)
  is still refused under SRG-V0-547. Failure mode: an acceptance-note paragraph under a requirement
  that opened with its own backticked ID failed with `defined more than once`. Rollback: drop the
  open-item tracking.
- `SRG-V0-953`: (experimental under the owner's standing auto-accept (2026-11-01); V1-3553;
  extends `SRG-V0-046`) A rank 0 definition whose own line carries no leading note and whose next
  line, a continuation of it, opens with a parenthesized acceptance note (one naming `accepted`,
  `approved`, `standing` or an auto-accept anywhere in the parenthetical, nested parentheses
  included, up to its matching close or the end of the line) MUST be refused in every mode, naming the spec, the
  continuation line, the ID and the repair: move the note to the start of the text on the ID's
  line. Only the definition line is read (SRG-V0-046), so the note would otherwise be silently
  `unrecorded`. Failure mode: a wrapped requirement whose note landed on its second line failed
  later gates with no message naming the line. Rollback: drop the branch.
- `SRG-V0-954`: (experimental under the owner's standing auto-accept (2026-11-01); V1-3617, V1-3510)
  `script/gen-spec-requirements.sh --check` MUST regenerate the three outputs into a private
  directory and byte-compare them with `docs/specs/`, exiting 1 naming each stale file and the
  repair, and `--write` MUST regenerate them into `docs/specs/` only after refusing, with exit 1 and
  nothing written, every `docs/specs/*.md` (other than README.md) that is modified but unstaged or
  untracked, naming each and the `git add --` command. A caller with a private `GIT_INDEX_FILE` is
  exempt. `make spec-requirements` runs `--write` and `spec-requirements-check` runs `--check`;
  `--all DIR` keeps its DCW-V0-352 warning for its other callers, and the usage text names every
  mode. Failure modes: `--check` was a usage error, so the check worked only through make; an
  unstaged spec edit produced only a warning and stale generated files that the check then
  refused. Rollback: revert the two modes and the two recipes.
- `SRG-V0-965`: (experimental under the standing lane auto-accept (2026-11-01); V1-3660)
  `make spec-requirements` MUST run the SRG-V0-900 obligation-bound check
  (`TestSpecsStayWithinTheOCMObligationBound`) after it writes the generated files, so a spec whose
  `## Requirements` section exceeds the 256-obligation OCM bound fails at the step that added the
  requirement instead of first at `doc-gates`, naming the spec and how to re-home the requirement.
- `SRG-V0-966`: (experimental under the standing lane auto-accept (2026-11-01); V1-3660) A line
  under a heading whose text starts with `Rollback` (any level from `##`, outside a fence) MUST NOT
  rank as a definition (rank 0) of an ID it starts with, bulleted (`- `ID`: revert ...`) or not, so
  a rollback restatement is neither a second definition under SRG-V0-547, nor an out-of-section
  definition under SRG-V0-891, nor the row's title. `script/check-requirement-definitions.sh`
  (RDG-V0-021, `requirement-definitions-check`) MUST apply the same exclusion to its duplicate and
  outside-section checks. The section ends at the next heading. Non-goal: rollback prose inside a
  requirement's own bullet, which never starts a line with the ID.
- `SRG-V0-967`: (experimental under the standing lane auto-accept (2026-11-01); V1-3660)
  `make spec-requirements` MUST print one line on success: `spec-requirements: no changes` when every
  generated file already equals its regenerated bytes, else `spec-requirements: updated FILE ...`
  naming each rewritten file, so a prose-only amendment shows that the run happened. The SRG-V0-980
  frozen-status refusal already states its rule (the header holds the current status only; record
  the acceptance in the requirement's own leading note) and is unchanged.
- `SRG-V0-760`: (experimental under the standing lane auto-accept (2026-11-01); V1-4000; refines
  SRG-V0-003) Inside a spec's `## Requirements` section (or anywhere in a spec without one), outside
  fences and rollback sections, a bulleted line whose note sits between the id and its colon, as in
  `- `ID` (KEEP): text` or `- **ID** (note): text`, defines nothing, so every generator mode MUST
  refuse it as `path:line: ID: the note sits before the colon` with the `- `ID`: (note) text` repair
  when the id has no definition elsewhere, before printing or writing anything, instead of
  leaving the id out of REQUIREMENTS.tsv. A restatement of a defined id is not refused.
- `SRG-V0-761`: (experimental under the standing lane auto-accept (2026-11-01); V1-4001; refines
  SRG-V0-012) `--readme`, and the README projection of `--all`, `--write` and `--check`, MUST replace
  only the claim a row was projected from: a description that starts with the spec's current
  `- Claim:` or with the claim its staged INDEX.json entry records keeps everything after it; a
  spec with no INDEX.json entry keeps the first-sentence rule; any other row was edited by hand and
  MUST be refused naming `docs/specs/README.md:LINE`, the spec and the repair, never rewritten.
- `SRG-V0-762`: (experimental under the standing lane auto-accept (2026-11-01); V1-2310; refines
  SRG-V0-043) A spec row whose Contract cell is its bare link, alone or before a `;`, MUST gain the
  range of the ids the spec defines right after the link, `PREFIX-NNN..MMM`, with the prefix from
  its INDEX.json `reqPrefix` or else the SRG-V0-012 default prefix; a row appended for a new spec
  carries it too. A spec defining no id of that prefix, and a cell with other text after the link,
  are left unchanged. SRG-V0-043 then keeps the range current.
- `SRG-V0-763`: (experimental under the standing lane auto-accept (2026-11-01); V1-3306; refines
  SRG-V0-040) The over-long Claim refusal MUST name the Claim's `path:line` and
  `docs/SPEC-DRIVEN-DEVELOPMENT.md` "Required capability-spec shape", whose first item states the
  one-sentence, 160-character Claim limit.
- `SRG-V0-764`: (experimental under the standing lane auto-accept (2026-11-01); V1-3389) `script/requirement-next-id.sh
  PREFIX` (`make requirement-next-id PREFIX=`) MUST print `next PREFIX-NNN after ID FILE:LINE`, one
  above the highest staged definition of PREFIX, read through `--lines` so a spec's ID order cannot
  mislead it; then `out-of-order ID FILE:LINE after ID LINE` for each definition numbered below one
  defined above it in the same spec, and `out-of-order=N`. It reports and exits 0, because by this
  rule 95 ASD-V0 definitions alone were out of order on 2026-11-01, and a reorder is a separate
  per-spec change; a malformed PREFIX exits 2.
- `SRG-V0-968`: (experimental under the standing lane auto-accept (2026-11-01); V1-3661; refines
  SRG-V0-012) Every spec's `## Agent digest` MUST carry a `- Claim:` line unless it is a
  superseded digest (a `- Successor:` line, which `internal/specindex` limits to Successor and Why
  and checks against INDEX `supersededBy`), and every generator mode MUST refuse any other digest
  without one, naming the spec, before printing or writing anything. With it, the leading summary
  sentence of every live spec's README row is projected from its digest and checked by
  `spec-requirements-check`; only superseded rows keep a hand-written retirement note. Rollback:
  drop the refusal; a claimless live spec's README description is again hand-written.
- `SRG-V0-790`: (experimental under the standing lane auto-accept, 2026-11-01; V1-3793; refines
  SRG-V0-003) The SRG-V0-003 token MUST accept three or more digits after the prefix, and an ID
  of four or more digits MUST be indexed only when its prefix (the ID without its digits) also
  has a three-digit ID on a rank 0 line, so ticket IDs (`V1-0789`) and record IDs (`DR-0046`) in
  table rows and prose stay text. Every generated ordering keyed by ID (`--lines`, ties of the
  TSV order, the README range) MUST compare the prefix as text and the digits as a number, so
  `CIN-V0-999` precedes `CIN-V0-1010`, and MUST break a tie of equal numbers written in different
  widths (`CIN-V0-001`, `CIN-V0-0001`) by comparing the full text, so no ordering follows hash
  order. Non-goals: a prefix that has only four-digit IDs; a
  registry of prefixes. Failure mode: before this, `- **CIN-V0-1010:** x` produced no row and no
  error, so a requirement past 999 silently left REQUIREMENTS.tsv and every check that reads it.
  Rollback: restore `[0-9]{3}` and the text sorts.
- `SRG-V0-791`: (experimental under the standing lane auto-accept, 2026-11-01; V1-3793) Every
  generator mode MUST refuse, before printing or writing anything, a definition-shaped line (a
  definition boundary, not a wrapped continuation, whose bold `**ID:**`, `**ID.**` or backticked
  `` `ID`: `` names an uppercase prefix ending in `-` followed by a digit) whose ID is not that
  prefix followed by three or more digits only, when the prefix is one SRG-V0-790 knows, naming
  `SPEC:LINE`, the ID and the repair. The prefix of a malformed ID is the longest known prefix it
  begins with before a digit, so a hyphenated tail (`CIN-V0-1010-bad`) is refused too. Non-goals: prose mentions; unknown prefixes, which stay
  text; a lettered sub-requirement such as `VAF-009A` (three or more digits and one uppercase
  letter), which stays unindexed under its parent row as before. Failure mode: before this,
  `CIN-V0-01` or `CIN-V0-1010a` written as a definition produced no row and no error. Rollback: drop the refusal block.
- `SRG-V0-979`: (experimental under the standing lane auto-accept (2026-11-01); V1-3797; refines
  SRG-V0-012) The INDEX.json projection MUST take `supersededBy` from a superseded digest's
  `- Successor:` line, as its first backticked `*.md` path, a bare file name naming a sibling in
  `docs/specs/`, and MUST leave the INDEX value unchanged for a digest without one. Superseding a
  spec (its Delivery status `superseded` and a digest of Successor and Why) then regenerates to an
  index that `internal/specindex` accepts with no hand edit of INDEX.json. Rollback: drop the
  projection; `supersededBy` is again hand-written.
- `SRG-V0-820`: (experimental under the standing lane auto-accept (2026-11-01); V1-4068; amends
  SRG-V0-954 and SRG-V0-967) `--write` (`make spec-requirements`) MUST stage each generated file
  (`REQUIREMENTS.tsv`, `INDEX.json`, `README.md`) whose worktree bytes differ from the Git index, or
  that is untracked, and print `spec-requirements: staged FILE ...` naming them after its
  `updated`/`no changes` line, so a commit of the staged spec carries the regenerated files. A caller
  that sets `GIT_INDEX_FILE` (a private snapshot index, as `dogfood-finish.sh` uses) stages nothing.
  A failed `git add` refuses, naming the file. Design: lanes left `REQUIREMENTS.tsv` unstaged after
  `make spec-requirements` (V1-4068 item 3). Non-goals: staging specs or any other path. Evidence:
  `script/gen-spec-requirements_test.sh` case 17 (both `--write` runs leave no unstaged generated
  file and the second names the staged index). Rollback: drop the staging loop; stage by hand.
- `SRG-V0-740`: (experimental under the standing lane auto-accept (2026-11-01); V1-3769; refines
  SRG-V0-965) Every generator mode MUST count each spec's `## Requirements` obligations as
  `internal/lrfrepo/ocm.go` counts them, warn on standard error from 240 to 256 naming the spec,
  its count and the companion-spec route, and refuse a section over 256 before printing or writing
  anything, so the limit is seen at authoring time rather than first at the Go test after the TSV
  is written. Rollback: drop the obligation count and its warning and refusal; SRG-V0-965 still
  refuses after the write.
- `SRG-V0-743`: (experimental under the standing lane auto-accept (2026-11-01); V1-3388) The
  header of `script/gen-spec-requirements.sh` MUST document the minimal Git-index layout each mode
  needs, and the missing-README refusal MUST name the modes that need it and point at that
  comment. Rollback: revert the comment and the message.

## Non-goals and simpler baseline

The simpler baseline is a hand-maintained index, which desynchronises on the first spec edit and
is what generation exists to prevent. A second baseline is indexing every occurrence of an id
rather than one winner; that produces several rows per id and destroys the one-lookup contract
`AGENTS.md` routes agents through. The generator does not validate that an id's prefix matches its
spec's declared requirement prefix, that ids are contiguous or gap-free, that a requirement is
traced to a test, or that a cited id exists at all. It does not read Git history, and it does not
parse Markdown structurally beyond the line-shape ranks in SRG-V0-003.

The acceptance mode records what a requirement's marker says, not who read it: an owner read-back
is evidenced by the build-log entry the promoting commit adds, and the audit is a report, not a
gate. The projection derives no title, prefix, dependency or implementation list from a spec;
those INDEX.json fields and the README.md Scope, Contract and description-tail cells stay
hand-owned, because six of the 202 INDEX.json titles differ from their spec's H1 and 144 specs
declare no `Requirement prefix:`, so a generated prefix would be a guess from the first id.

## Trust boundary, limits, and failure modes

The spec corpus is the only input and it is trusted repository content; the generator never reads
anything outside `docs/specs/*.md` and never executes content it reads. It spawns one `perl`
interpreter and performs no write, except that `--all DIR` writes its three outputs into the
directory its caller names (SRG-V0-018).

Enumeration is the boundary that matters here, and the answer to the fresh-clone question is yes:
the generator measures the Git index, so a dirty worktree cannot change its output in either
direction and `git add` is what publishes a spec to the index. A `spec-requirements-check` failure
is therefore unambiguous: it is real drift between the committed index and the staged spec corpus,
not local untracked files.

One narrower limit remains. Ranking by file path ascending means an id defined in two specs is
attributed to whichever path sorts first, which is stable but arbitrary, and
`requirement-definition-gate-v0.md` is the gate that rejects that duplication rather than this
generator.

Title truncation at 80 characters is lossy by design and the index is a locator, not a
requirement text; a consumer that needs the full clause MUST open the cited file at its
definition line (SRG-V0-017).

| Failure | Behavior |
|---|---|
| No tracked spec holds any id matching the pattern | header line only, exit 0 |
| An id appears only in prose (rank 2) | not indexed at all; `requirement-definition-gate-v0.md` is what catches a row with no definition |
| An id appears in both a definition line and a table row | the definition line wins regardless of file or line order |
| An id appears in two definition lines in different files | the lexically first path wins; the duplicate is reported by `requirement-definitions-check`, not here |
| An untracked spec file is present in the worktree | it is not in the Git index, so it contributes no row and `spec-requirements-check` is unaffected |
| A tracked spec is deleted from the worktree but not staged | its bytes still come from the Git index, so its rows are unchanged |
| The committed index differs from a regeneration | `spec-requirements-check` fails with `cmp` output naming the first differing byte |
| INDEX.json or README.md differs from the projection | `spec-requirements-check` fails the same way and names all three files |
| A spec lacks `Intent status:` or `Delivery status:` | `--index` and `--readme` exit non-zero naming the spec; `make spec-requirements` copies nothing |
| A header value holds a `\|` or control character, or a digest claim is not one sentence | refused, naming the spec, because the README.md cell or claim boundary could not be rebuilt |
| INDEX.json has a duplicate, malformed or untracked entry | refused, naming the entry |
| INDEX.json is hand-edited into another valid JSON layout or escape form | its values are kept and written back in the one canonical layout, so `spec-requirements-check` fails until it is regenerated |
| A new tracked spec has no INDEX.json entry or README.md row | one is appended from the header: H1 title, declared or first rank 0 prefix, no dependencies |
| A new tracked spec has a superseded Agent digest, with no `Claim` | refused, naming the spec; its INDEX.json entry and README.md row are added by hand once, and only its statuses are projected after that |
| A README.md description is hand-edited so that its first sentence holds an abbreviation such as `Dr.` | the old claim is cut at the abbreviation and the rest is kept as the tail; the mis-split shows in the regenerated README.md diff |
| Two lanes each add a new spec | both append at the end of INDEX.json and the README.md table and conflict there; `make spec-requirements-resolve` takes this side and regenerates (SRG-V0-041) |
| Two lanes change the same spec's header status | a real conflict in the header, INDEX.json and README.md, resolved by the people who disagree |
| A marker's parenthetical never closes | it runs to the end of the line |
| A negation spans an inner parenthesis, as in `(not (yet) accepted, decision 0478)` | the negated phrase is removed whole and the mode is `unrecorded` |
| A standing marker follows the requirement text instead of leading it | the requirement is `unrecorded`; the audit counts and names it as `UNPARSED_STANDING` |
| A spec with `validated` delivery holds a `standing` requirement | the audit prints `NOT_PROMOTABLE` and still exits 0 |
| A REQUIREMENTS.tsv row has five columns but a malformed id or file | the audit exits 1 naming the line |
| A committed row names an id its spec no longer defines (a stale index) | the console reports a gap, the compaction kernel refuses with `unsupported-kernel-requirement`, a roadmap link keeps its file and title with no line, and the audit exits 1; `spec-requirements-check` fails on the stale index |
| A reader is handed an index pinned before SRG-V0-015, with a line column | `specindex.ParseIndex` reads the line from that column, so a kernel or console over an old revision still resolves |
| Two lanes add requirements whose rows land next to each other in the committed order | a conflict between those rows of `REQUIREMENTS.tsv` only; resolve the spec and regenerate (SRG-V0-013) |
| One lane reflows a requirement so its first line, and so its title, changes, and another inserts a requirement right after it | a conflict between those rows of `REQUIREMENTS.tsv`, even when the spec merges cleanly; a reflow is not a prose-only edit under SRG-V0-016; resolve the spec and regenerate (SRG-V0-013) |
| A committed row names a different file than the one the staged specs define its id in | the acceptance audit exits 1 naming both files, instead of citing the row's file at the other file's line |
| A change touches only INDEX.json, a spec's Agent digest or the README table | `corvint affected` selects `internal/specindex`, whose `TestIndexCoversSpecsAndHeaders` fails on a claim over 160 characters or one the digest and README do not carry (SRG-V0-045) |
| The `internal/specindex` read scope is narrowed so a spec-metadata input leaves it | `TestSRGV0045SpecMetadataStaysInSelectedScope` fails naming the input and the scope |
| A lane names a new requirement range on a frozen `Intent status:` or digest `- Status:` line | the refusal names that line as `path:line` and says the range belongs on each requirement's definition line, the acceptance and traceability rows and the build log (SRG-V0-950) |
| One spec holds two malformed requirement bullets, a duplicate and an over-long claim | one run prints every refusal with each repair once and exits non-zero, writing nothing (SRG-V0-951) |
| A note paragraph indented under a requirement opens with that requirement's backticked ID | it is a continuation of the item, not a second definition; a real second bullet is still refused (SRG-V0-952) |
| A requirement's acceptance note starts on its continuation line | every mode refuses naming the spec, the line and the ID, saying to move the note onto the ID's line (SRG-V0-953) |
| `make spec-requirements` runs with a spec edit left unstaged | it exits 1 naming each unstaged spec and the `git add --` command and writes nothing (SRG-V0-954) |
| An author edits a spec's header Intent status but not its Agent digest Status, or the reverse | `spec-requirements-check` names the digest `- Status:` line and both header status lines as `path:line` and says to edit them together (SRG-V0-047) |
| Two specs, or two lines of one spec, define the same requirement id | every generator mode exits non-zero naming each `path:line`, so `make spec-requirements` and `spec-requirements-check` fail instead of keeping one definition (SRG-V0-547) |
| A lane appends an acceptance segment naming a new requirement ID to a spec's `Intent status:` or digest `Status:`, or grows a header past 160 characters and its frozen length | every generator mode exits non-zero naming the spec, the field and the ID or lengths, and says to record the acceptance in the requirement's leading note and build-log entry (SRG-V0-980) |
| A lane adds a phony make target to a shared multi-target `.PHONY:` line | `make spec-requirements-test` fails naming the line (SRG-V0-982) |
| Two lanes re-pin the same line citations in `docs/AUTOMATION.md`, or the same use-case receipt digest in `conformance/use-cases-v0/ledger.json`, after both edit a cited file | a conflict in those derived pins only; take either side and run `make batch-regen`, which re-pins both; not prevented here, since dropping the content pin would weaken the evidence (V1-3458 build log) |
| An agent needs a requirement's line | `rg -n -F` on the id in the row's file, or `script/gen-spec-requirements.sh --lines` |

## Acceptance criteria and testing matrix

Evidence is `script/gen-spec-requirements_test.sh`, run by `make spec-requirements-test` in
`make gate`, plus the `spec-requirements-check` invocation over the live spec corpus.

| Requirement | Evidence |
|---|---|
| SRG-V0-001 | `MET` at `0ec169f1`: `script/gen-spec-requirements_test.sh` asserts an untracked spec, a worktree-only deletion, and an unstaged edit all leave the index unchanged while a staged edit changes it, and that `docs/specs/README.md` is excluded |
| SRG-V0-002, SRG-V0-003 | the id regex and the three rank patterns in `script/gen-spec-requirements.sh`; `script/gen-spec-requirements_test.sh` case 5 rejects wrapped prose and preserves marked, paragraph-start and table definitions |
| SRG-V0-004 | the zero-padded sort key and the `le` comparison that keeps the first winner; deterministic regeneration at `5b496119` |
| SRG-V0-005 | `MET`: the title is decoded as UTF-8 around the `substr` cut, so truncation counts characters; `script/gen-spec-requirements_test.sh` case 4 requires a 90-character `é` title to come out as exactly 80 characters (it failed at 40 characters, 80 bytes, before the repair). Decision 0105 recorded the byte-truncation defect; regeneration lengthened ten committed rows whose titles carry a multibyte character before the cut. A title that is not valid UTF-8 cannot be decoded and is still cut by byte |
| SRG-V0-006 | `make spec-requirements-check` passing at `5b496119` against the committed index |
| SRG-V0-007 | the script's root resolution and read-only body; no write path exists in the script |
| SRG-V0-008 | the `spec-requirements-check` recipe and its `gate` membership; the temporary file trap on `EXIT INT TERM` |
| SRG-V0-009 | `script/gen-spec-requirements_test.sh` case 6: standing, owner by decision and by date, a negated "not accepted", a negation across an inner parenthesis, a negated owner decision, an unbalanced marker, no marker, no anchor, a table row and an anchor past the title cut; `TestRequirementsAcceptanceColumns` checks the header and the closed modes over the committed index |
| SRG-V0-010 | `script/gen-spec-requirements_test.sh` case 8: the standing listing, `UNPARSED_STANDING`, `NOT_PROMOTABLE` with exit 0, the `unrecorded` listing, the refused mode (exit 2), a TSV without the acceptance columns and a five-column row with an empty id (exit 1 each); the live counts in the V1-1346/V1-1347 build-log entry |
| SRG-V0-011 | this spec's own header and digest, the worked example; `TestIndexCoversSpecsAndHeaders` still requires the digest `Status:` to hold both statuses |
| SRG-V0-012 | `script/gen-spec-requirements_test.sh` case 7: projection, kept fields and cells, appended entry and row, fixed point and the refused two-sentence claim; byte-identical `--index` and `--readme` output at `523fa4ce` |
| SRG-V0-013 | `script/spec-requirements-conflict_test.sh`: the conflicted paths are exactly the spec and `REQUIREMENTS.tsv`, every marker sits in the requirement list or between new TSV rows, regeneration resolves the TSV, and a lane on another spec merges cleanly |
| SRG-V0-014 | `TestDeliveryClauseConvention` (rule table with the stale-prefix case) and `TestIndexCoversSpecsAndHeaders` over every spec |
| SRG-V0-015 | `TestSRGV0015IndexOrderedByDefinitionPosition` over the committed index; `script/gen-spec-requirements_test.sh` case 9: the committed header, definition order within a spec, file order, the id-ordered `--lines` view, and a line shift that leaves the committed index byte for byte. At the V1-1525 base the regenerated `--lines` view was byte-identical to the previous committed index |
| SRG-V0-016 | `script/spec-requirements-conflict_test.sh`: lanes on two sections of one spec, a prose-only lane against each, and a lane on another spec merge in both orders with every generated file equal to its regeneration. Against the base generator three of those four pairs conflicted in `REQUIREMENTS.tsv`. A title-changing reflow against a row inserted after it conflicts in `REQUIREMENTS.tsv` only, in both orders, and regenerates to both rows (the recorded limit) |
| SRG-V0-017 | `TestSRGV0017DefinitionLineLocatesEveryRow` (every committed row), `TestSRGV0017DefinitionLineFollowsGeneratorRanks`, `TestSRGV0017ParseIndexReadsBothLayouts`, `TestSRGV0017SpecLookupLocatesCommittedLayout`, `TestSRGV0017KernelLocatesCommittedLayoutLines`, `TestSRGV0017RoadmapLocatesCommittedLayoutLines`, `TestSRGV0017LoadRequirementsReadsBothLayouts`; `script/spec-acceptance-audit.sh` output byte-identical before and after for all three modes; `script/gen-spec-requirements_test.sh` case 8: a row naming another file than its staged definition makes the audit exit 1 |
| SRG-V0-018 | `script/gen-spec-requirements_test.sh` case 10: `--all` output equals each mode's output byte for byte, a refused projection leaves DIR empty, a missing or absent DIR exits 2, and a newline path and a gitlink entry take the one-shot fallback beside the batch. On the live corpus at `f3ca7224` the `--all` files were byte-identical to the three-pass output and the committed files; the V1-1641 build-log entry holds the before and after timings |
| SRG-V0-040 | `script/gen-spec-requirements_test.sh`: a 165-character claim fails `--all` with `<path>: the Agent digest Claim is 165 characters; the limit is 160` and leaves DIR empty, and the TSV, `--lines`, `--index` and `--readme` modes refuse it with the same message (the TSV mode now reads spec headers too) |
| SRG-V0-042 | `script/gen-spec-requirements_test.sh` case 11: an auto-acceptance without `standing`, an affirmed note without an anchor and an unparenthesized note are each refused with line, id, reason and the accepted forms, while the exact standing and owner forms and the adjective "Accepted" pass; the test fails against the generator without the refusal. 74 near misses on main at `ef2a01db` were rewritten to the exact forms in the same change (V1-2160 build log) |
| SRG-V0-043 | `script/gen-spec-requirements_test.sh` case 7: a stale `ONE-V0-001..009` becomes `ONE-V0-001`, a range before `;` is rewritten with the tail kept, and an undefined prefix is refused; 13 drifted rows on main were regenerated by `make spec-requirements` |
| SRG-V0-041 | `script/spec-requirements-conflict_test.sh`: the target refuses while the one-section spec is still conflicted and resolves the TSV once it is staged; two new specs conflict only in generated files and resolve in both merge orders to the fresh generation with both specs indexed |
| SRG-V0-045 | `TestSRGV0045SpecMetadataStaysInSelectedScope` over the committed `.corvint/test-read-scopes.json` and INDEX.json; `TestSRGV0045IndexCheckWitnesses`: the mismatched fixture reports exactly the length, digest and README findings, the aligned one none; on the repository, an INDEX-only 217-character claim is selected by `corvint affected` and fails `TestIndexCoversSpecsAndHeaders` with the same three findings, and the aligned claim in all three files is selected and passes (V1-0515 PR evidence) |
| SRG-V0-047 | `TestSRGV0047StatusDriftNamesBothFieldsWithLines`: a fixture whose digest Status says `accepted` under an `Intent status: proposed` header yields exactly one finding, pinned byte for byte, naming `fixture-v0.md:7`, `:3` and `:4` and saying to edit the fields together |
| SRG-V0-970 | `TestSRGV0047StatusDriftNamesBothFieldsWithLines`: the same finding names `- Status: proposed / experimental` and the requirement-line rule |
| SRG-V0-044 | `script/gen-spec-requirements_test.sh` case 11: a definition line ending in `Status: (accepted, decision 0481; GitHub #715).` is refused with its line and id; the case fails on the pre-change generator |
| SRG-V0-547 | `script/gen-spec-requirements_test.sh` case 12: `DUP-V0-001` defined in two specs and `DUP-V0-002` twice in one are refused in the TSV, `--lines`, `--index` and `--readme` modes naming both `path:line` locations, and `--all` writes nothing; the case fails on the pre-change generator |
| SRG-V0-548 | `script/gen-spec-requirements_test.sh` case 12: a wrapped `` `DUP-V0-001`. `` citation before the real definition is neither refused nor chosen as the row; regenerating the committed index moves only the `AHI-246` and `DCI-V1-030` rows to their real definitions |
| SRG-V0-965 | the `spec-requirements` recipe in `Makefile` runs `TestSpecsStayWithinTheOCMObligationBound` after writing; `script/gen-spec-requirements_test.sh` case 15 checks the recipe names it |
| SRG-V0-966 | `script/gen-spec-requirements_test.sh` case 15: `RBK-V0-001:` repeated under `### Rollback` is neither a duplicate nor the title; `script/check-requirement-definitions_test.sh` rollback case: a fixture with bulleted and unbulleted restatements under `## Rollback` and `### Rollback` passes the generator (no SRG-V0-891 refusal) and then `check-requirement-definitions.sh` |
| SRG-V0-967 | `script/gen-spec-requirements_test.sh` case 15: the recipe's `no changes` and `updated` lines |
| SRG-V0-968 | `script/gen-spec-requirements_test.sh` case 16: a spec whose digest has no Claim is refused in every mode, naming it, and a superseded digest (Successor and Why) passes |
| SRG-V0-979 | `TestSRGV0979SupersededSpecRegeneratesToACheckedIndex` in `internal/specindex/superseded_test.go`: a live spec superseded in a fixture repository regenerates through `--index` and `--readme` to an index `checkIndex` accepts, and `supersededBy` names the Successor |
| SRG-V0-820 | `script/gen-spec-requirements_test.sh` case 17: `--write` stages the regenerated files and prints the `staged` line |
| SRG-V0-740 | `script/gen-spec-requirements_test.sh` case 50: 241 section definitions warn naming the count and the companion-spec route and still print; 257 are refused in the TSV and `--all` modes, and `--all` writes nothing |
| SRG-V0-743 | `script/gen-spec-requirements_test.sh` case 51: a fixture with specs but no README prints the TSV and is refused in `--readme` naming the layout comment |
| SRG-V0-800 | `script/spec-requirements-worktree-check_test.sh`: an unstaged requirement edit left behind by `make spec-requirements` fails naming `REQUIREMENTS.tsv`, the spec and the fix, with the real index unchanged; a private `GIT_INDEX_FILE`, a staged-and-regenerated edit and a prose-only edit pass; fails on BASE, which has no check |
| SRG-V0-046 | `script/gen-spec-requirements_test.sh` case 11: `NEAR-008`, an owner-chat note whose date wraps to the next line, is refused as still open at the end of the definition line with the closed-note example, the no-anchor reason and closing line carry their examples, and `NEAR-009`, the same note nested and closed on one line, classifies as `owner` with `2026-10-27; V1-2763`; the open-note and example cases fail on the pre-change generator |
| SRG-V0-980 | `script/gen-spec-requirements_test.sh` case 13: a header and a digest that name an ID absent from the frozen row, and a header grown past 160 characters and its frozen length, are refused in the TSV and `--all` modes with the spec, field, ID or lengths and the repair, and `--all` writes nothing; a frozen header, a shortened one and an absent frozen file with a short ID-free header pass; the case fails on the pre-change generator. The committed frozen file regenerates the live corpus unchanged |
| SRG-V0-950 | `script/gen-spec-requirements_test.sh` case 13: the Intent status and digest refusals name the fixture spec's header line 3 and digest line 8 as `path:line` and the definition-line repair |
| SRG-V0-951 | `script/gen-spec-requirements_test.sh` case 14: two malformed bullets, a duplicate and an over-long claim are all reported by one run of each mode, and `--all` writes nothing |
| SRG-V0-952 | `script/gen-spec-requirements_test.sh` case 15: an indented note paragraph opening with the item's ID is not a definition, while an unindented and a bulleted second definition are refused; the committed index regenerates unchanged |
| SRG-V0-953 | `script/gen-spec-requirements_test.sh` case 15: a note on the continuation line, including one whose acceptance word follows a nested parenthetical, is refused naming `path:line`, the ID and the repair, while a nested aside naming no acceptance passes; the committed corpus regenerates unchanged |
| SRG-V0-954 | `script/gen-spec-requirements_test.sh` case 16: `--check` passes on fresh outputs and names a stale file; `--write` refuses an unstaged and an untracked spec naming both and the `git add --` command, writing nothing, and writes once they are staged |
| SRG-V0-760 | `script/gen-spec-requirements_test.sh` case 60: `- `KPT-V0-001` (KEEP):` with no definition is refused in every mode naming `kept.md:3` and the rule; a restated defined id and a fenced line are not named |
| SRG-V0-761 | `script/gen-spec-requirements_test.sh` case 61: a row starting with the INDEX.json claim gains the new Claim and keeps its tail, a tail with `e.g.` survives, and a hand-written first sentence is refused naming its README line; fails on the pre-change generator, which rewrote it |
| SRG-V0-762 | `script/gen-spec-requirements_test.sh` cases 7 and 61: a bare-link Contract cell gains `SA-V0-001..003`, a `;` cell keeps its tail, an appended row carries its range, and the result is a fixed point; the regenerated `docs/specs/README.md` gains a range on 196 rows; case 7 fails on the pre-change generator |
| SRG-V0-763 | `script/gen-spec-requirements_test.sh` the SRG-V0-040 long-claim checks and case 15: the long-Claim refusal names `two.md:8` and `multi.md:7` |
| SRG-V0-764 | `script/requirement-next-id_test.sh`, run by `spec-requirements-test`: the next ID follows the highest definition, not the last line, two out-of-order definitions are named by line, an unused prefix starts at 001, and a bad prefix exits 2 |
| SRG-V0-981 | `script/spec-requirements-conflict_test.sh` replay: each branch's amended note yields a `standing` row whose `acceptedBy` carries the amendment's date and ticket |
| SRG-V0-982 | `script/spec-requirements-conflict_test.sh`: the repository `Makefile` has no multi-target `.PHONY:` line, and a fixture with one is named; `make -pn` lists the same phony set before and after the split |
| SRG-V0-983 | `script/spec-requirements-conflict_test.sh` replay: the line-amending branches merge in both orders with no conflict and regenerate to the merged bytes, while the header-segment and shared-`.PHONY:` controls conflict |
| SRG-V0-890 | `script/gen-spec-requirements_test.sh` case 13: the Intent status refusal names line 3 of the fixture spec `frozen.md`, the frozen file and the Requirements section `(lines 10..13)`; the digest refusal names its line 8. |
| SRG-V0-891 | `script/gen-spec-requirements_test.sh` case 14: a definition after the section is refused in the TSV and `--all` modes naming line 13 of the fixture spec `placed.md`, `PLC-V0-002` and the section `(lines 3..6)`; the inside and fenced definitions are not named; a baselined one and a spec without the section pass. |
| SRG-V0-920 | `script/spec-requirements-conflict_test.sh`: a merge that conflicts only in a spec's prose resolves with exit 0 and the `no unmerged path` line; outside a merge a staged spec edit is regenerated and staged |
| SRG-V0-921 | `script/spec-requirements-conflict_test.sh`: two lanes that each add a new spec and repin a different receipt conflict in INDEX.json and the ledger; the resolve alone finishes the merge, every generated file equals the regenerated one, both new requirements are rows and both receipts are current |
| SRG-V0-922 | `script/spec-requirements-conflict_test.sh`: in that merge `make spec-requirements` refuses with one line naming INDEX.json and `make spec-requirements-resolve`, and again once INDEX.json is staged with its markers |
| SRG-V0-923 | `script/spec-requirements-worktree-check_test.sh`: a staged spec edit whose regenerated INDEX.json is left unstaged fails naming INDEX.json and the fix; staging it passes |
| SRG-V0-790 | `script/gen-spec-requirements_test.sh` case 16: `- **CIN-V0-1010:** x` beside `CIN-V0-999` yields a `CIN-V0-1010` row after `CIN-V0-999` in the TSV and `--lines` modes, while `V1-0789` and `DR-0046` table rows of prefixes without a three-digit definition yield none; `--readme` over `CIN-V0-001`, `CIN-V0-0001`, `CIN-V0-999` and `CIN-V0-1010` writes the range `CIN-V0-0001..1010` in ten of ten runs; the case fails on the pre-change generator. The committed corpus regenerates unchanged |
| SRG-V0-791 | `script/gen-spec-requirements_test.sh` case 16: `- **CIN-V0-01:** x`, `- **CIN-V0-1010a:** x` and `- **CIN-V0-1010-bad:** x` are refused in the TSV and `--all` modes naming `SPEC:LINE` and the ID, and `--all` writes nothing; a lettered `- **CIN-V0-999A:** x` passes; the case fails on the pre-change generator |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| SRG-V0-001 | the `git ls-files`/`git cat-file --batch` enumeration and the `README.md` skip in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-002 | the bounded id regex in `script/gen-spec-requirements.sh` | `spec-requirements-check` |
| SRG-V0-003 | the period, colon, and table-cell rank patterns and the rank 2 discard | `spec-requirements-check` |
| SRG-V0-004 | the composite sort key and its comparison in `script/gen-spec-requirements.sh` | `spec-requirements-check` |
| SRG-V0-005 | the title substitutions and the UTF-8-decoded `substr` truncation | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-006 | the header and the sorted emission loop | `spec-requirements-check` |
| SRG-V0-007 | the `root=$(...)`/`cd` prelude and the stdout-only body | code inspection |
| SRG-V0-008 | the `Makefile` `spec-requirements-check` target and its listing in the `gate` target | `make gate` membership |
| SRG-V0-009 | `acceptance` and the six-column emission in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `TestRequirementsAcceptanceColumns` |
| SRG-V0-010 | `script/spec-acceptance-audit.sh` and the `spec-acceptance-audit` target | `spec-requirements-test` |
| SRG-V0-011 | `docs/SPEC-DRIVEN-DEVELOPMENT.md` Status model; this spec's header | `TestIndexCoversSpecsAndHeaders` |
| SRG-V0-012 | the `--index` and `--readme` modes and the `spec-requirements` and `spec-requirements-check` recipes | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-013 | `script/spec-requirements-conflict_test.sh` | `spec-requirements-test` |
| SRG-V0-014 | `internal/specindex` tests | `TestIndexCoversSpecsAndHeaders` |
| SRG-V0-015 | the `tsv` and `lines` emission blocks in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `TestSRGV0015IndexOrderedByDefinitionPosition` |
| SRG-V0-016 | `script/spec-requirements-conflict_test.sh` | `spec-requirements-test` |
| SRG-V0-017 | `internal/specindex/requirements.go`; `internal/console/spec.go`; `internal/compactionkernel/kernel.go`; `internal/dashboard/roadmap/requirements.go`; `script/spec-coverage-audit/main.go`; `script/spec-acceptance-audit.sh` | `TestSRGV0017DefinitionLineLocatesEveryRow`; `TestSRGV0017ParseIndexReadsBothLayouts`; `TestSRGV0017DefinitionLineFollowsGeneratorRanks`; `TestSRGV0017SpecLookupLocatesCommittedLayout`; `TestSRGV0017KernelLocatesCommittedLayoutLines`; `TestSRGV0017RoadmapLocatesCommittedLayoutLines`; `TestSRGV0017LoadRequirementsReadsBothLayouts`; `spec-requirements-test` |
| SRG-V0-018 | `index_blobs` and the `--all` mode, which `--write` and `--check` run, in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-040 | the claim-length loop before the TSV and `--lines` exits in `script/gen-spec-requirements.sh` | `spec-requirements-test` |
| SRG-V0-041 | `script/spec-requirements-resolve.sh` and the `spec-requirements-resolve` target in the `Makefile` | `spec-requirements-test` |
| SRG-V0-042 | `acceptance_near_miss` and the refusal block before the TSV exit in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-043 | the Contract-cell range block in `readme_text` in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-044 | the trailing `Status:` branch of `acceptance_near_miss` in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-046 | the open-note branch and the examples in `acceptance_near_miss`, and the refusal block, in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-045 | the `internal/specindex` entry in `.corvint/test-read-scopes.json`; `checkIndex` in `internal/specindex/index_test.go` | `TestSRGV0045SpecMetadataStaysInSelectedScope`; `TestSRGV0045IndexCheckWitnesses`; `spec-requirements-check` |
| SRG-V0-547 | the `%definitions` collection and its refusal block in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-548 | the fence and continuation tracking and the wrapped flag in the `%best` key in `script/gen-spec-requirements.sh`; `DefinitionLine` in `internal/specindex/requirements.go` | `spec-requirements-test`; `spec-requirements-check`; `TestSRGV0017DefinitionLineFollowsGeneratorRanks`, `TestSRGV0017DefinitionLineLocatesEveryRow` |
| SRG-V0-800 | `script/spec-requirements-worktree-check.sh` and its `spec-requirements-check` line in `Makefile` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-047 | `statusDriftMessage` and `headerLine` in `internal/specindex/index_test.go` | `TestSRGV0047StatusDriftNamesBothFieldsWithLines`; `spec-requirements-check` |
| SRG-V0-970 | `statusDriftMessage` in `internal/specindex/index_test.go`; `docs/SPEC-DRIVEN-DEVELOPMENT.md` Status model | `TestSRGV0047StatusDriftNamesBothFieldsWithLines` |
| SRG-V0-900 | `TestSpecsStayWithinTheOCMObligationBound` in `internal/lrfrepo/ocm_spec_bound_test.go`, run by `spec-requirements-check` |
| SRG-V0-980 | `docs/specs/INTENT-STATUS-FROZEN.tsv`; the frozen-header refusal block in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-950 | the frozen-header refusal block in `script/gen-spec-requirements.sh`; `docs/LANE-ORCHESTRATION.md` section 3 | `spec-requirements-test` |
| SRG-V0-951 | the `@refusals` collection and its single exit in `script/gen-spec-requirements.sh` | `spec-requirements-test` |
| SRG-V0-952 | the open-item tracking beside the wrapped flag in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-953 | the continuation-note check in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-954 | the `--check` and `--write` modes in `script/gen-spec-requirements.sh` and the `spec-requirements` and `spec-requirements-check` recipes in `Makefile` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-760 | the noted-definition collection and refusal in `script/gen-spec-requirements.sh`; `docs/SPEC-DRIVEN-DEVELOPMENT.md` Accepted forms | `spec-requirements-test` |
| SRG-V0-761 | `index_entries` and the claim match in `readme_text`, `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-762 | `spec_range` and the bare-link branch in `readme_text`, `script/gen-spec-requirements.sh`; `docs/specs/README.md` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-763 | the SRG-V0-040 refusal in `script/gen-spec-requirements.sh`; `docs/SPEC-DRIVEN-DEVELOPMENT.md` Required capability-spec shape | `spec-requirements-test` |
| SRG-V0-764 | `script/requirement-next-id.sh`; the `requirement-next-id` target in `Makefile` | `spec-requirements-test` |
| SRG-V0-981 | `docs/SPEC-DRIVEN-DEVELOPMENT.md` Status model; the SRG-V0-009 reader | `spec-requirements-test` |
| SRG-V0-982 | the per-target `.PHONY:` lines in `Makefile`; the `.PHONY` check in `script/spec-requirements-conflict_test.sh` | `spec-requirements-test` |
| SRG-V0-983 | the replay case in `script/spec-requirements-conflict_test.sh` | `spec-requirements-test` |
| SRG-V0-920 | `script/spec-requirements-resolve.sh` | `spec-requirements-test` |
| SRG-V0-921 | `script/spec-requirements-resolve.sh` ledger reconcile and repin step | `spec-requirements-test` |
| SRG-V0-922 | the `spec-requirements` target's unmerged check in `Makefile`; the staged-marker restart in `script/spec-requirements-resolve.sh` | `spec-requirements-test` |
| SRG-V0-923 | the generated-file index comparison in `script/spec-requirements-worktree-check.sh` | `spec-requirements-test` |
| SRG-V0-790 | the token, `requirement_id` and `id_order` in `script/gen-spec-requirements.sh` | `spec-requirements-test`; `spec-requirements-check` |
| SRG-V0-791 | the malformed-definition refusal in `script/gen-spec-requirements.sh` | `spec-requirements-test` |
| SRG-V0-740 | `%obligations` and the obligation warning and refusal before the SRG-V0-547 check in `script/gen-spec-requirements.sh` | `spec-requirements-test` |
| SRG-V0-743 | the minimal fixture layout comment and the `readme_text` refusal in `script/gen-spec-requirements.sh` | `spec-requirements-test` |

## Rollout, rollback, and drift

This document is ownership over an existing generator. The SRG-V0-001 repair landed with it:
the worktree glob became the tracked-path enumeration, and regenerating the index reproduced the
committed `REQUIREMENTS.tsv` byte for byte, so no row moved. Rollback restores the glob and drops
`script/gen-spec-requirements_test.sh` and its `make` target, which reinstates the fresh-clone
divergence and is therefore not a rollback anyone should take.

Requirements SRG-V0-009..013 roll back as one revert of their commit: the generator returns to
four columns and one mode, the audit and conflict test go, and `make spec-requirements`
regenerates the four-column index. INDEX.json and README.md then become hand-maintained again,
which is safe because the projection left their bytes unchanged at its base. The two strict
readers relaxed to accept extra columns (`script/spec-coverage-audit` and the OCM spec test in
`internal/lrfrepo`) keep working with four.

Requirements SRG-V0-015..017 roll back as one revert of their commit (decision 0543). The generator
then prints the line column in id order again, `make spec-requirements` regenerates it, and the
readers return to reading that column. No state outside the repository changes. The revert
restores the line-number conflicts that decision 0543 records.

SRG-V0-018 rolls back as one revert of its commit: `SPEC_GENERATE` returns to three generator
invocations and the reader to one synchronous batch request per file. Every mode's output is
unchanged by the revert, so no generated file moves.

SRG-V0-047 rolls back as one revert of its commit: only the wording of one test finding changes,
so no generated file moves.

SRG-V0-547 and SRG-V0-548 roll back as one revert of their commit: the duplicate refusal goes away
and the `AHI-246` and `DCI-V1-030` rows return to their wrapped citations after `make
spec-requirements`.

SRG-V0-890 rolls back by restoring the refusal text in `script/gen-spec-requirements.sh` and case 13; SRG-V0-891 by removing the outside-section block, the baseline read and case 14.

SRG-V0-968 rolls back by removing the claimless-digest refusal and case 16.

SRG-V0-979 rolls back by removing the `successor_path` projection and
`TestSRGV0979SupersededSpecRegeneratesToACheckedIndex`; every committed `supersededBy` already
equals its projection, so no generated file moves.
SRG-V0-740 rolls back by removing the obligation count, its warning and refusal, and case 50.
SRG-V0-743 rolls back by reverting the layout comment, the README refusal text and case 51.

SRG-V0-965..967 roll back as one revert of their commit: the recipe loses its bound check and its
summary line, and a rollback-section line ranks as before.

SRG-V0-980..983 roll back as one revert of their commits: the frozen file and refusal go, the
`.PHONY:` lines may be rejoined, and no generated file moves, since the freeze changed no header.

SRG-V0-950..954 roll back as one revert of their commits: the messages return to the first
refusal per run, the indented-paragraph and continuation-note rules go, and the recipes return to
`--all`; no generated file moves, since the committed corpus regenerates unchanged under them.

SRG-V0-790 and SRG-V0-791 roll back as one revert of their commit; no generated file moves, since
the committed corpus had no four-digit requirement ID when they landed.

SRG-V0-760..764 roll back as one revert of their commits: the noted-line refusal, the claim match
and the range insertion go, `make spec-requirements` drops the inserted README ranges again, and
`script/requirement-next-id.sh` and its target are deleted; REQUIREMENTS.tsv and INDEX.json do not move.

Drift rule: enumeration MUST measure the Git index, never a worktree listing, for the same reason
`decision-number-gate-v0.md` pins it. A future change that reintroduces a directory listing,
adds a filesystem walk, or reads untracked files re-opens exactly the fresh-clone divergence
recorded above. A change to any rank pattern in SRG-V0-003 silently moves cited lines for existing
ids, so it MUST be landed together with a regenerated index in the same commit. A change to the
acceptance rule in SRG-V0-009 can move rows between modes, so it MUST land with the regenerated
index and the audit counts before and after in its build-log entry.

## Unresolved

SRG-V0-001 as implemented does both: it enumerates tracked paths and reads blob content, so a
tracked file with unstaged edits is indexed at its staged content rather than its worktree
content. The reference is the Git index and not `HEAD`, which is the polarity
`script/check-line-citations.sh` already uses, so a spec staged for addition in the same change as
the regenerated index resolves. What is still not decided is whether the reference should instead
be `HEAD`, which would make the index exactly reproducible from a commit alone but would stop a
spec and its index row landing in one commit. The same question applies to the sibling
tracked-path gates and should be answered for all of them together, not here.

Three questions stay open after SRG-V0-009..013. Whether the 52 trailing standing markers should be
moved to the front of their lines, which would make them `standing`, is a change to five other
specs and belongs to their owners. Whether old header tails should be cut back to the current
status is the same kind of edit. Whether `NOT_PROMOTABLE` should fail a gate is for the owner to
decide once the standing backlog has been read back.
