# Documentation citation gate V0

Owner: Russell Lewis
Date: 2026-09-07
Requirement prefix: `DCG-V0`
Intent status: accepted (owner instruction 2026-09-07)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariants 1, 2 and 8, `../SPEC-DRIVEN-DEVELOPMENT.md`,
`../DOGFOOD.md` §4, `falsifiable-packet-v0.md` FPK-V0-004 (the packet-side citation check this
gate mirrors for prose), `../decisions/0136-extensionless-root-line-citations-2026-09-12.md`,
`../decisions/0154-line-citation-ratchet-and-new-document-anchors-2026-09-12.md`,
`../decisions/0176-line-citations-scan-decisions-2026-09-13.md`, and
`../BUILD-LOG.md` heading 'Citations may carry a content anchor'.

## Agent digest
- Claim: Path:line citations resolve against the Git index, including decisions; malformed tokens fail, legacy unpinned debt cannot grow, new docs need content anchors.
- Status: accepted (owner instruction 2026-09-07) / implemented
- Exists: `script/check-line-citations.sh`, `script/check-line-citations_test.sh`, `make` targets `line-citations-check` and `line-citations-test`.
- Blocked on: nothing for this gate. The three sibling gate scripts named in `../agent-memory/fixes.md` remain unspecified.
- Read next: Requirements; Non-goals; Failure modes.

## Human intent and scope

Corvint documentation cites repository content by `path:line`. Specs are written against a working
tree and committed with the code they cite, so a citation can be stale the moment it lands. The
2026-09-05 audit repinned 330 citations across nine documents and found that none were out of range
and most landed on a blank line, a lone brace, or the wrong function. The first two shapes are
mechanically checkable and were already gated. The third — a citation that still lands on a real,
non-trivial line that is no longer the line the sentence means — was the most common and was
invisible, because nothing recorded what the author had actually read.

Affected user: an agent or engineer acting on a documented claim. Measurable job: for one committed
tree, refuse a citation that cannot resolve or is malformed after a tracked-path line prefix,
refuse a pinned citation whose cited bytes changed, keep legacy unpinned debt from growing, and
require anchors in new documents without mechanically repinning the existing corpus.

The owner instruction of 2026-09-07 places repository gate tooling in scope for AGENTS.md invariant
8. This spec discharges that for the citation gate only; the other four gate scripts are a
follow-up and remain governed by nothing.

## Verified current state

`script/check-line-citations.sh` is wired into `make gate` through `line-citations-check` and its
test through `line-citations-test`. At `bf81ec2` the corpus held 433 backticked full-path citations
across `docs/specs/*.md` and `docs/agent-memory/*.md`, of which one was pinned (`admittedEntries` in
`../agent-memory/fixes.md`); as of 2026-09-07 it holds 438, of which seven are pinned. The gate
passes.

On 2026-09-12 a scratch semantic sweep, never committed, compared each citation's sentence
identifiers with the cited lines in the two trees, and a blame comparison checked each cited span
against the revision that last touched the citing line. It found about 130 citation tokens,
across 16 documents, that landed on real but wrong lines. All of them passed this gate, because
DCG-V0-003 can see only blank and bracket-only lines. The same pass widened the scanned set to the
current-contract documents named in DCG-V0-001. Those held three full-path line citations: one
drifted, one into the removed Python oracle, and one describing retired code. All three were
repaired before the widening, so the widened gate passes.

Decision 0154 resolves broader anchor adoption without rewriting legacy evidence: the current
unpinned count of 941 becomes the committed ceiling, and the 138 documents already scanned at acceptance
form a closed legacy allowlist. A scanned document absent from that index-read list is new and all
of its citations require anchors.

On 2026-09-13, decision 0176 widened DCG-V0-001 to scan `docs/decisions/*.md`. A historically-framed
citation (decision 0105's `Makefile:129-136`, reviewed once against a named commit) converted to the
prose form `Makefile@<hash>` rather than being repinned. 40 other stale citations across 16 other
decisions were repinned or relocated-and-repinned against their current lines. Decision 0158 is
off-limits to that change and carries 30 unpinned citations, 3 of them landing on a since-shifted
blank or bracket-only line; it was admitted to the DCG-V0-018 legacy allowlist as the one-time
scope-widening admission DCG-V0-018 now names, and its 3 drifted citations are named in the
existing DCG-V0-020 drift-waiver file alongside decision 0170's rows, extending that mechanism to
also cover the trivial-line check rather than only the anchor-mismatch check. The unpinned count
after admission is 927 against the unchanged 941 ceiling. The gate passes. On merge into the integration branch, backlog hunt entries had added 29 unpinned citations; 17 were read and pinned, leaving 939.

## Requirements

- `DCG-V0-001`: The gate MUST check every backticked citation in `docs/specs/*.md`,
  `docs/agent-memory/*.md`, `docs/decisions/*.md`, and the current-contract documents `README.md`,
  `docs/DOGFOOD.md`, `docs/AGENT-ROUTES.md`, `docs/SELF-DEVELOPMENT.md`, and `conformance/*/README.md` (one directory
  below `conformance/`, so a dated results README beneath it is not scanned), covering the forms `dir/file.ext:N`, `dir/file.ext:N-M`,
  `dir/file.ext:N,M`, a basename
  the Git index tracks at the repository root such as `ROADMAP.md:343`, and a bare `:N` or `:N-M`
  continuation. A root basename names exactly one path, so it MUST be checked as that full path and
  MUST anchor its paragraph as a full-path citation does, including where `docs/specs` holds a file
  of the same name (`README.md`). Any other bare basename, such as `prove.go:767`, MUST NOT be
  checked, because its directory is a reading of the prose and not of the token.
- `DCG-V0-002`: For every checked citation the gate MUST fail when the named file is not tracked,
  when any cited line number exceeds the file's line count or is `0` (lines are 1-based), and when a
  range `N-M` has `M` below `N`. All three are absolute failures reported with the citing document, its line, and the token.
  A reversed range names no lines, so a pin on one would anchor the empty span and never detect
  drift; its failure text MUST name both endpoints and MUST differ from the past-the-end text.
- `DCG-V0-003`: The gate MUST additionally fail when a cited line is blank or contains only
  bracket, comma or semicolon characters. This is a drift proxy, not a proof of staleness, and its
  failure text MUST say which line was trivial.
- `DCG-V0-004`: A bare continuation MUST resolve only to an anchor its own paragraph states: the
  last full-path citation, a sibling spec cited by bare name, or a requirement id from
  `REQUIREMENTS.tsv` written immediately before it. A blank line MUST end the anchor. The gate MUST
  NOT guess a path.
- `DCG-V0-005`: Content inside a fenced code block MUST be exempt, and a Go standard-library path
  cited as `src/syscall/...` MUST be exempt because it is GOROOT-relative.
- `DCG-V0-006`: A citation MAY carry a content anchor written as `@<hex>` appended to its line
  span, where `<hex>` is 4 to 64 lowercase hexadecimal characters. A citation without an anchor
  MUST receive the checks in DCG-V0-002 and DCG-V0-003 plus the bounded adoption rules in
  DCG-V0-014, DCG-V0-017, and DCG-V0-018, so that adoption does not require repinning the legacy
  corpus at once.
- `DCG-V0-007`: The anchor value MUST be the SHA-256 of the cited lines, each stripped of trailing
  whitespace and joined by a single newline. Leading whitespace MUST be retained, because
  indentation is cited content; trailing whitespace MUST NOT be, because its churn is not a change
  to what was read.
- `DCG-V0-008`: The anchored line set MUST be the span the citation reads: for `N-M` the inclusive
  range `N..M`, and otherwise the listed lines `N` and each comma extra in written order.
- `DCG-V0-009`: A written anchor MUST compare equal when it is a prefix of the full digest, so that
  the same citation may be pinned at the abbreviated length the authoring mode prints or at full
  length, with identical meaning.
- `DCG-V0-010`: When a pinned citation's content no longer matches, the gate MUST fail and MUST
  report both the written anchor and the current digest. The failure text MUST direct the author to
  read the cited lines before repinning, because a matching anchor asserts only that the cited bytes
  are unchanged and never that they were the right bytes to cite.
- `DCG-V0-011`: The gate MUST offer `--hash <path:line[-line][,line]> ...`, which prints each
  citation with its anchor appended and exits zero. This is the only supported way to author an
  anchor. It MUST refuse a token that is not a citation, a missing file, a reversed range, and a
  line past the end, and MUST NOT edit any document.
- `DCG-V0-012`: The gate MUST be read-only over the repository on every path, including every
  failure path, and MUST exit non-zero when any citation fails so that `make gate` fails with it.
- `DCG-V0-013`: Every path the gate reads MUST be resolved against the Git index and never the
  working filesystem: the scanned document set and the set of resolvable citation targets are the
  paths `git ls-files` reports, and each file's bytes are the index blob. A file present on disk
  but untracked MUST NOT resolve, a tracked file deleted from the worktree alone MUST still
  resolve, and an unstaged edit to a cited file MUST NOT change any line count or anchor. The
  reference is the index rather than `HEAD`, so a file staged for addition in the same change as
  the document citing it resolves; this is the polarity `check-decision-numbers.sh` uses, and it
  keeps the authoring loop in DCG-V0-006's spirit, where `git add` and not a commit is what
  publishes a change to the gate.
- `DCG-V0-014`: A citation from any scanned document into `docs/BUILD-LOG.md` or a one-level
  `docs/agent-memory/*.md` file MUST carry a content anchor unless the citing and cited paths are
  the same file. These documents are prepended newest-entry-first, so an unpinned citation into one
  MUST fail with `<path> is prepended (newest entry on top)` and MUST instruct the author to `pin
  this citation with @<hash>` through `script/check-line-citations.sh --hash` or `cite a stable
  anchor instead of a line number`. A pinned citation and an exempt same-file self-citation MUST
  continue through the other applicable checks.
- `DCG-V0-015`: The gate MUST also check an extensionless path followed by `:N`, `:N-M`, or
  `:N,M` when the Git index tracks that exact path, including a repository-root basename such as
  `Makefile` or `LICENSE`. A tracked root basename names one unambiguous repository path, so it
  MUST anchor its paragraph as a full-path citation does; this does not admit a bare basename such
  as `prove.go` that is not tracked at the root. Because the extensionless form has no suffix to
  key on, an untracked token of the same shape MUST remain prose and MUST change neither the
  paragraph anchor nor the result.
- `DCG-V0-016`: A backticked token whose prefix is an exact tracked path in the DCG-V0-001 or
  DCG-V0-015 syntax followed by `:N` MUST be treated as citation-like. Its complete suffix after
  `N` MUST be either empty, a comma list `,M[,M...]`, or a range `-M`, followed optionally by one
  `@<hex>` anchor satisfying DCG-V0-006. Any other remainder MUST fail with the citing document,
  line, whole token, and the phrase `malformed citation-like token`; it MUST NOT be silently skipped
  as prose. This does not admit an untracked path prefix, and DCG-V0-005's fenced-code exemption
  still applies.
- `DCG-V0-017`: The gate MUST count every checked citation that carries no content anchor and MUST
  fail when the measured count exceeds the committed ceiling of 941. The count and ceiling MUST be
  reported together on failure. Lowering the ceiling is permitted without a new decision; raising
  it requires an explicit owner-approved contract amendment, so legacy unpinned debt can shrink but
  cannot grow invisibly. Malformed citation-like tokens are failures under DCG-V0-016 rather than
  checked citations and therefore do not enter this count.
- `DCG-V0-018`: `script/line-citation-legacy-documents.txt` MUST be the closed allowlist of scanned
  documents present when decision 0154 was accepted. Every checked citation in a scanned document
  absent from that allowlist MUST carry a content anchor and MUST fail with a diagnostic requiring
  `@<hex>` when it does not. The gate MUST read the allowlist from the Git index under DCG-V0-013;
  an unstaged allowlist edit cannot change classification. Later documents MUST NOT be added to the
  allowlist, so absence from it remains the durable, base-independent definition of a new document.
  The one exception is a bounded, one-time scope-widening admission: when a decision amends
  DCG-V0-001 to scan a document class that `scanned_doc()` previously excluded entirely, that same
  decision MAY add a named pre-existing document of the newly-scanned class to the allowlist,
  provided the decision states the exact count of unpinned citations admitted. This mirrors decision
  0154's own initial bulk admission of the corpus scanned at its acceptance; it does not reopen the
  allowlist to documents that were already in scope. Decision 0176 exercises this once, for
  `docs/decisions/0158-local-excludes-remain-status-policy-2026-09-12.md` (30 unpinned citations,
  off-limits to that change), when `docs/decisions/*.md` was first scanned.
- `DCG-V0-019`: A citation a citing document explicitly frames as historical — reviewed once against
  a named commit rather than asserted to hold at HEAD — MUST NOT be pinned or repinned against
  current content. It MUST instead be converted to a non-line prose reference, `path@<hex-or-hash>`
  with no trailing `:N`, so DCG-V0-016's citation-token match (every form requires a trailing line
  number) does not apply to it and the gate treats it as prose. This is the only sanctioned way to
  keep such a citation truthful without a new exemption mechanism.
- `DCG-V0-020`: `script/line-citation-drift-waivers.txt` MAY name one citation, as
  `<citing-doc><TAB><path>:<first>[-<last>]` per row, whose trivial-line (DCG-V0-003) and
  anchor-mismatch (DCG-V0-010) checks are both exempted, when fixing the citation would require
  editing either the citing document or the cited lines and both are off-limits to the current
  change (a different in-flight lane owns them). The gate MUST read this file from the Git index
  under DCG-V0-013. It MUST NOT exempt the file-existence or line-range checks (DCG-V0-002) for a
  waived citation, and a row MUST be removed once the citation is repinned or fixed by whichever
  change owns the document.

## Non-goals and simpler baseline

The simpler baseline is the three structural checks alone, retained for legacy unpinned citations
subject to DCG-V0-014 and the DCG-V0-017 ceiling. This gate does not judge whether a citation is relevant, well chosen, or supports the
sentence around it; an anchor is a staleness detector, not a correctness proof. It does not rewrite
or repin documents, does not check citations outside the DCG-V0-001 document set, does not resolve a bare
basename that the index does not track at the repository root, does not read Git history or any revision other than the index, and does not
extend to the machine-readable packet, whose equivalent check is FPK-V0-004's `history-consistent`
against `blob_hash` at the packet revision. `docs/plans`, `docs/reviews`, `docs/BUILD-LOG.md` and
`docs/build-log`, and nested conformance result READMEs stay outside the set. They are dated
evidence: a citation in them records what its author read at that revision. Repinning one to
today's content would change the record, and gating one would force either that change or a false
failure against since-deleted Python-era files. A 2026-09-12 sweep of those trees found about 300
such decayed citations and left them as written.

`docs/decisions/*.md` is no longer in this excluded set: decision 0176 (2026-09-13) brought it into
DCG-V0-001's scanned set, because a decision's citations are meant to resolve like any other
document's, with the DCG-V0-019 prose conversion available for the narrower case of a citation a
decision explicitly frames as a historical, commit-pinned review rather than a claim about current
content.

## Trust boundary, limits, and failure modes

Document bytes, the legacy allowlist, and cited file bytes are untrusted input. The gate reads only Git index blobs under
the repository root and spawns only `git ls-files`, one `git cat-file --batch` co-process that serves
every index read, and a one-shot `git cat-file blob` for a path the batch cannot serve (a newline in
the path, or a missing or non-blob entry); it performs no write. A batch co-process that exits
non-zero or ends its stream before a full header and body fails the gate, as a failing one-shot read
does. One co-process measured 0.22 s against 9.81 s for one `git cat-file blob` per file on
darwin/arm64 under load (2026-09-12). Every failure is collected and reported
together, then the gate exits non-zero; there is no partial-success exit. `--hash` fails closed on
a malformed token, an untracked file, a reversed range, or an out-of-range line, and prints nothing for that token.

| Failure | Behavior |
|---|---|
| Cited file not tracked in the index | fail, naming the path |
| Cited file untracked on disk, or tracked and deleted from the worktree | fail and pass respectively, by DCG-V0-013 |
| Cited line past end | fail, naming the line and the file's length |
| Cited range reversed (`N-M` with `M` below `N`) | fail, naming both endpoints; `--hash` refuses it too |
| Cited line blank or bracket-only | fail, naming the line |
| Pinned content changed | fail, naming written anchor and current digest |
| Tracked-path `:N` prefix followed by a malformed remainder | fail, naming the whole malformed citation-like token, by DCG-V0-016 |
| Unpinned citation, content changed | pass, by DCG-V0-006 |
| Unpinned citation into `docs/BUILD-LOG.md` or one-level `docs/agent-memory/*.md` from another document | fail, saying the target is prepended and directing the author to `--hash` or a stable anchor, by DCG-V0-014 |
| Total checked unpinned citations exceeds the committed ceiling | fail, reporting the count and ceiling, by DCG-V0-017 |
| Unpinned citation in a scanned document absent from the legacy allowlist | fail, requiring an `@<hex>` content anchor, by DCG-V0-018 |
| Non-line reference `path@<hash>` with no trailing `:N` (the DCG-V0-019 historical-citation form) | not matched as a citation token; no check applies |
| Cited content drifted where fixing it is off-limits to the current change | trivial-line and anchor-mismatch checks exempted for the named citation, by DCG-V0-020; file-existence and line-range checks still apply |

## Acceptance criteria and testing matrix

`script/check-line-citations_test.sh` builds a fixture repository and covers the anchor behavior;
`make line-citations-check` covers the corpus itself.

| Requirement | Evidence |
|---|---|
| DCG-V0-001..005 | `make line-citations-check` over 433 citations at `bf81ec2`; the root-basename form by test case 15; the reversed-range refusal, in the gate and in `--hash`, by test case 14; the line-0 refusal, in the gate and in `--hash`, by test case 19; the current-contract document set by `make line-citations-check` after the 2026-09-12 widening; the `docs/decisions/*.md` document class by test case 20 and `line-citations-check` after the 2026-09-13 widening |
| DCG-V0-006 | test case 4, an unpinned citation surviving a changed cited line |
| DCG-V0-007 | test case 2, trailing whitespace not breaking a matching anchor |
| DCG-V0-008 | test case 5, a change to the last line of a cited range failing |
| DCG-V0-009 | test case 6, a full-length pin comparing equal |
| DCG-V0-010 | test case 3, drift failing and naming the written anchor |
| DCG-V0-011 | test case 1, the printed anchor round-tripping into a passing pin |
| DCG-V0-012 | `make gate` membership; the script performs no write |
| DCG-V0-013 | test case 7, an untracked file on disk failing and resolving once staged; test case 8, an unstaged truncation and a worktree-only deletion leaving the result unchanged |
| DCG-V0-014 | test case 11, an unpinned citation into a prepended agent-memory file failing with the explanation and the same citation passing once pinned |
| DCG-V0-015 | test case 9, a real `Makefile:N` passing and a blank-line `Makefile:N` failing; test case 10, an untracked extensionless token remaining prose |
| DCG-V0-016 | test case 16, a second full citation accidentally replacing an anchor and failing as a malformed citation-like token |
| DCG-V0-017 | test case 17, one unpinned citation failing against a zero ceiling with both values reported; `line-citations-check` at the committed corpus ceiling |
| DCG-V0-018 | test case 18, an unpinned citation in a new document failing despite an unstaged allowlist edit, then passing once pinned; decision 0176's one-time scope-widening admission of decision 0158, 30 unpinned citations, is manual evidence recorded in that decision, not a fixture case |
| DCG-V0-019 | decision 0105's `Makefile@3c41cad3` conversion, verified by inspection of the citation-token regex (every alternative requires a trailing `:digit`, so a suffix-less `path@hash` never matches) and by `line-citations-check` passing with that citation present |
| DCG-V0-020 | `line-citation-drift-waivers.txt`'s decision 0170 rows (pre-existing) and decision 0158 rows (added by decision 0176); `line-citations-check` passing with all 5 rows present |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| DCG-V0-001, DCG-V0-004, DCG-V0-005, DCG-V0-015 | the citation scanner loop in `script/check-line-citations.sh` | `line-citations-check`; cases 9 and 10 |
| DCG-V0-002, DCG-V0-003 | `check` in `script/check-line-citations.sh:244-248@ac0b3493` and `:260-263@087dfc00` | `line-citations-check`; case 14 |
| DCG-V0-006, DCG-V0-009, DCG-V0-010 | the anchor comparison in `check` | `line-citations_test.sh` cases 1-4, 6 |
| DCG-V0-007 | `anchor_of`, `script/check-line-citations.sh:167-173@ca6f1248` | case 2 |
| DCG-V0-008 | `anchored_numbers`, `script/check-line-citations.sh:159-165@9a3cc62d` | case 5 |
| DCG-V0-011 | the `--hash` mode, `script/check-line-citations.sh:271-291@3a897fb5` | cases 1 and 14 |
| DCG-V0-012 | `make gate` target list | gate run 2026-09-07 |
| DCG-V0-013 | the tracked set, the `index_blob` batch reader, and `lines_of`, `script/check-line-citations.sh:75-142@c733a5fa` | cases 7, 8, and 13 |
| DCG-V0-014 | `prepended_doc` and the unpinned-target branch in `check`, `script/check-line-citations.sh:175-180@2c93e31d` and `:238-243@99bc16be` | case 11 |
| DCG-V0-016 | `check_malformed` and the backticked-token pre-scan in `script/check-line-citations.sh` | case 16; `line-citations-check` |
| DCG-V0-017 | the unpinned counter and committed ceiling in `script/check-line-citations.sh` | case 17; `line-citations-check` |
| DCG-V0-018 | `script/line-citation-legacy-documents.txt` and the new-document branch in `check` | case 18; decision 0176's manual admission of decision 0158 |
| DCG-V0-019 | the citation-token regex in the scanner loop (no alternative matches a suffix-less token) | decision 0105's `Makefile@3c41cad3` conversion; `line-citations-check` |
| DCG-V0-020 | the drift-waiver loading block and `$waived` gating in `check`, `script/check-line-citations.sh:194-212@5e2147f9` and `:259@851bb9b7` | `script/line-citation-drift-waivers.txt`; `line-citations-check` |

## Rollout, rollback, and drift

The anchor grammar is additive: reverting it leaves every unpinned citation except a
DCG-V0-014 target checked exactly as before, and each pinned citation degrades to an unrecognized
token that the scanner ignores rather than mis-parses. DCG-V0-016 separately makes a tracked-path
line prefix fail closed when its remainder is malformed; reverting that check restores the former
silent skip. Rollback of the anchor grammar is reverting `1b6c32e` plus unpinning those citations. Drift
rule: an anchor is repinned only after a human reads the cited lines and confirms the sentence
still holds; a mechanical repin sweep would defeat DCG-V0-010 and MUST NOT be added. Second drift
rule: no path may be resolved with a filesystem test such as `-f` or `-e`, in this gate or in a
sibling gate script, because that is the defect DCG-V0-013 closes and it fails silently in the
passing direction. The DCG-V0-002 and DCG-V0-003 traceability pin was repinned to `check`'s two
absolute-failure branches and its trivial-line branch; the earlier range began inside the anchor
comparison, which those two requirements do not govern. On 2026-09-12 the reversed-range branch
was added ahead of the prepended-document branch, so that pin became two spans that skip it. The
same day the root-basename form was added; every script pin in the traceability table moved four
lines and was repinned after the digests were confirmed unchanged.

Decision 0154 adds two monotone drift rules. The committed unpinned ceiling may fall but requires
an owner-approved contract amendment to rise. The legacy-document allowlist is closed: a later
document is never added to it. Existing citations are pinned only after their cited lines are read;
a mechanical bulk repin remains forbidden.

Decision 0176 (2026-09-13) exercises a narrower lever than the one 0154 closed: 0154's allowlist was
computed once, over the document classes `scanned_doc()` matched that day, which excluded
`docs/decisions` entirely. Widening `scanned_doc()` itself to add a document class is a DCG-V0-001
contract change, not an addition to an unchanged allowlist; decision 0176 uses that widening to
admit one pre-existing, off-limits document (decision 0158) as a one-time scope-widening admission
under the DCG-V0-018 exception, with its exact unpinned count (30) stated there. No other document
is added, and the closed-allowlist rule otherwise stands unchanged for the classes already scanned
before this widening.

## Unresolved

The anchor-adoption question is resolved by decision 0154 and DCG-V0-017/018: legacy unpinned debt
is capped, while citations in new scanned documents require anchors. Three sibling gate scripts
remain unspecified; they are outside this decision.
