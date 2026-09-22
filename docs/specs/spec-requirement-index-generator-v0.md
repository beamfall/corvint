# Spec requirement index generator V0

Owner: Russell Lewis
Date: 2026-09-12
Requirement prefix: `SRG-V0`
Intent status: accepted (decision 0105 amendment 2026-09-12)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariant 8, `../SPEC-DRIVEN-DEVELOPMENT.md` "Required
capability-spec shape", `decision-number-gate-v0.md` (whose drift rule pins that a gate must
measure what a fresh clone contains), `requirement-definition-gate-v0.md` (the consumer that
reads every row this generator emits), and `../agent-memory/fixes.md` (which lists the gate
scripts still without an owning spec).

## Agent digest
- Claim: One generated index names, for every requirement id in every tracked spec, the single best place that defines it.
- Status: accepted (decision 0105 amendment 2026-09-12) / implemented
- Exists: `script/gen-spec-requirements.sh`, `make` targets `spec-requirements-check` and `spec-requirements-test`, both wired into `make gate`.
- Blocked on: nothing for this gate. Decision 0105 withheld acceptance on `SRG-V0-005`; its amendment accepted the spec once character truncation landed with evidence.
- Read next: Verified current state; Requirements; Trust boundary, limits, and failure modes.

## Human intent and scope

`docs/specs/REQUIREMENTS.tsv` is the repository's lookup from a requirement id to the exact file
and line that defines it. `AGENTS.md` routes agents to it before opening any spec body, and
`requirement-definition-gate-v0.md` reads every row it emits to prove no id survives as a table
row alone. The index is therefore load-bearing for agent routing and for a second gate, and it is
generated rather than hand-maintained so that a spec edit cannot silently desynchronise it.

`script/gen-spec-requirements.sh` is that generator. It prints the whole index to stdout;
`spec-requirements-check` regenerates it into a temporary file and byte-compares that against the
committed `docs/specs/REQUIREMENTS.tsv` (`Makefile:47`), so the gate fails whenever the committed
index does not match what the current spec corpus would produce.

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
`git ls-files -z -- docs/specs` and reads each file's bytes as `:<path>` through one `git cat-file --batch`
(`script/gen-spec-requirements.sh:68`), so neither an untracked spec, a tracked spec deleted from
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

No requirement below diverges from the implementation.

## Requirements

- `SRG-V0-001`: The generator MUST derive its input set from the paths Git reports as tracked
  under `docs/specs/` matching `*.md`, not from a worktree directory listing, so that an untracked
  file contributes no row and a tracked file deleted from the worktree alone still contributes its
  rows. It MUST exclude `docs/specs/README.md`, which is the index of specs rather than a spec.
- `SRG-V0-002`: The generator MUST recognise a requirement id as a token matching an uppercase
  letter followed by uppercase letters, digits, and hyphens, then a hyphen and exactly three
  digits, bounded by word boundaries (`script/gen-spec-requirements.sh:86`). An id with a suffix
  that is not exactly three digits MUST NOT be indexed.
- `SRG-V0-003`: The generator MUST classify each occurrence of an id into exactly one of three
  ranks: rank 0 when the line begins with the id, optionally preceded by a list bullet and
  optionally wrapped in bold or backticks, and followed either by a period and whitespace or by a
  colon; rank 1 when the line begins with a table cell holding the id; rank 2 otherwise. It MUST
  discard every rank 2 occurrence, so a passing mention of an id in prose never becomes its
  indexed location.
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
  repository-relative path and its one-based line number.
- `SRG-V0-007`: The generator MUST write no file, mutate no repository state, and produce
  identical output regardless of the working directory it is invoked from, resolving its own
  location to find the repository root first.
- `SRG-V0-008`: `spec-requirements-check` MUST regenerate the index and byte-compare it against
  the committed `docs/specs/REQUIREMENTS.tsv`, failing on any difference, and MUST remove its
  temporary file on normal exit and on interrupt. `docs/specs/REQUIREMENTS.tsv` MUST be
  regenerated whole by this generator and MUST NOT be hand-edited.

## Non-goals and simpler baseline

The simpler baseline is a hand-maintained index, which desynchronises on the first spec edit and
is what generation exists to prevent. A second baseline is indexing every occurrence of an id
rather than one winner; that produces several rows per id and destroys the one-lookup contract
`AGENTS.md` routes agents through. The generator does not validate that an id's prefix matches its
spec's declared requirement prefix, that ids are contiguous or gap-free, that a requirement is
traced to a test, or that a cited id exists at all. It does not read Git history, and it does not
parse Markdown structurally beyond the line-shape ranks in SRG-V0-003.

## Trust boundary, limits, and failure modes

The spec corpus is the only input and it is trusted repository content; the generator never reads
anything outside `docs/specs/*.md` and never executes content it reads. It spawns one `perl`
interpreter and performs no write.

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
requirement text; a consumer that needs the full clause MUST open the cited file and line.

| Failure | Behavior |
|---|---|
| No tracked spec holds any id matching the pattern | header line only, exit 0 |
| An id appears only in prose (rank 2) | not indexed at all; `requirement-definition-gate-v0.md` is what catches a row with no definition |
| An id appears in both a definition line and a table row | the definition line wins regardless of file or line order |
| An id appears in two definition lines in different files | the lexically first path wins; the duplicate is reported by `requirement-definitions-check`, not here |
| An untracked spec file is present in the worktree | it is not in the Git index, so it contributes no row and `spec-requirements-check` is unaffected |
| A tracked spec is deleted from the worktree but not staged | its bytes still come from the Git index, so its rows are unchanged |
| The committed index differs from a regeneration | `spec-requirements-check` fails with `cmp` output naming the first differing byte |

## Acceptance criteria and testing matrix

Evidence is `script/gen-spec-requirements_test.sh`, run by `make spec-requirements-test` in
`make gate`, plus the `spec-requirements-check` invocation over the live spec corpus.

| Requirement | Evidence |
|---|---|
| SRG-V0-001 | `MET` at `0ec169f1`: `script/gen-spec-requirements_test.sh` asserts an untracked spec, a worktree-only deletion, and an unstaged edit all leave the index unchanged while a staged edit changes it, and that `docs/specs/README.md` is excluded |
| SRG-V0-002, SRG-V0-003 | the id regex and the three rank patterns in `script/gen-spec-requirements.sh`; the committed index contains no row whose cited line is a prose mention |
| SRG-V0-004 | the zero-padded sort key and the `le` comparison that keeps the first winner; deterministic regeneration at `5b496119` |
| SRG-V0-005 | `MET`: the title is decoded as UTF-8 around the `substr` cut, so truncation counts characters; `script/gen-spec-requirements_test.sh` case 4 requires a 90-character `é` title to come out as exactly 80 characters (it failed at 40 characters, 80 bytes, before the repair). Decision 0105 recorded the byte-truncation defect; regeneration lengthened ten committed rows whose titles carry a multibyte character before the cut. A title that is not valid UTF-8 cannot be decoded and is still cut by byte |
| SRG-V0-006 | `make spec-requirements-check` passing at `5b496119` against the committed index |
| SRG-V0-007 | the script's root resolution and read-only body; no write path exists in the script |
| SRG-V0-008 | the `spec-requirements-check` recipe and its `gate` membership; the temporary file trap on `EXIT INT TERM` |

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

## Rollout, rollback, and drift

This document is ownership over an existing generator. The SRG-V0-001 repair landed with it:
the worktree glob became the tracked-path enumeration, and regenerating the index reproduced the
committed `REQUIREMENTS.tsv` byte for byte, so no row moved. Rollback restores the glob and drops
`script/gen-spec-requirements_test.sh` and its `make` target, which reinstates the fresh-clone
divergence and is therefore not a rollback anyone should take.

Drift rule: enumeration MUST measure the Git index, never a worktree listing, for the same reason
`decision-number-gate-v0.md` pins it. A future change that reintroduces a directory listing,
adds a filesystem walk, or reads untracked files re-opens exactly the fresh-clone divergence
recorded above. A change to any rank pattern in SRG-V0-003 silently moves cited lines for existing
ids, so it MUST be landed together with a regenerated index in the same commit.

## Unresolved

SRG-V0-001 as implemented does both: it enumerates tracked paths and reads blob content, so a
tracked file with unstaged edits is indexed at its staged content rather than its worktree
content. The reference is the Git index and not `HEAD`, which is the polarity
`script/check-line-citations.sh` already uses, so a spec staged for addition in the same change as
the regenerated index resolves. What is still not decided is whether the reference should instead
be `HEAD`, which would make the index exactly reproducible from a commit alone but would stop a
spec and its index row landing in one commit. The same question applies to the sibling
tracked-path gates and should be answered for all of them together, not here.
