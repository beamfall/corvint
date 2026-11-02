# Source Documentation Draft V0

Owner: Russell Lewis
Drafted: 2026-09-05
Intent status: proposed
Delivery status: experimental
Implementation: `internal/doccompiler/draft.go`, `cmd/corvint/docs.go`, `internal/docmaintain`, `cmd/corvint/docs_maintain.go`
Authoritative inputs: `docs/PRODUCT.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `ROADMAP.md` AT-15

## Agent digest
- Claim: Corvint drafts source-pinned Markdown from owner prose and indexed symbol-extracted declarations, then consumes its bytes after original-source rederivation.
- Status: proposed/experimental
- Exists: bounded CLI draft/consume and explicitly enabled foreground maintenance development slice.
- Blocked on: owner acceptance and independent outcome validation; full AT-15 maintenance and HDC qualification remain open.
- Read next: Requirements; CLI contract; Acceptance and rollback.

## User job and verified current state

The user requested Corvint generate and consume its own documentation; Corvint must be its first user. Existing `doccompiler.Plan` requires an actual
MkDocs environment; `docviews` consumes a real HDC plan. Neither seam justifies fabricating a plan
or qualification receipt. This smaller experimental profile produces an ordinary Markdown
orientation draft using Corvint's immutable source index and checks actual emitted bytes when the
next task consumes it. It does not implement a Markdown renderer or claim HDC delivery.

## Requirements

- `SDD-V0-001`: `docs draft` reads admitted, tracked, immutable Git source through `contextindex.BuildContext`.
The requested owner Markdown must contain exactly one literal `## Agent digest` section outside
fenced code. Top-level backtick and tilde fences may have up to three leading spaces and at least
three matching markers; only the same marker with at least the opening length and trailing spaces
or tabs closes the fence. Backtick opening info cannot contain backticks. Fenced headings neither
start a digest nor end its section; refuse an unclosed fence explicitly. Copy its
preamble and digest verbatim as source excerpts; use the exact Agent digest heading as the entry
title, never invent source semantics or accept generated text as owner intent.

- `SDD-V0-002`: Add indexed exported-name declarations from exactly the selected Go directory, excluding
`_test.go` (other languages: `SDD-V0-018`). Label these tracked non-test declarations, never active production code. Preserve the
indexed kind vocabulary and exact original declaration line. Include positive tracked non-test Go
import edges to the selected module-qualified package with original import anchors. Omit facts
from sources with `Unparsed` diagnostics. Build applicability, runtime callers and behavioral
validation remain UNKNOWN; zero observations establish no absence claim.

- `SDD-V0-003`: Every entry carries original path, blob, SHA-256 and line span. The page binds immutable
commit/tree plus profile/policy. Its GENERATED derivation never promotes source authority. Report
index exclusions and incomplete extraction as coverage limitations. All source facts remain quoted
observations, independent of the user's query or generated orientation text.
Owner spans count physical lines: a final LF terminates the last line rather than adding an
empty line; an unterminated final line and actual blank final lines still count. This applies
to tracked owners and `--dry-run` owners, including when no later `## ` heading ends the digest
(V1-3096, GH #1993; `TestSourceDraftOwnerPhysicalLines`).
Render variable metadata, including headings and paths, as single-line Go-quoted ASCII strings
inside Markdown code spans, with literal backticks encoded as `\x60`; preserve original entry fields
and exact indented excerpts unchanged. Emitted source excerpts must use LF without carriage returns;
refuse affected spans rather than rewrite original bytes, since Markdown newline normalization could
otherwise break indentation. Carriage returns outside emitted Go spans do not trigger this refusal.
Every entry supplies an executable local command
`git --no-pager --no-replace-objects show BLOB`, where BLOB is its validated canonical 40- or 64-digit
lowercase hexadecimal object ID. Run from the repository root to read the full immutable original,
including sections referenced by quoted prose, independently of dirty worktree bytes and replacement
refs. This route assumes no hosted repository or remote URL.

- `SDD-V0-004`: `docs consume` reads actual Markdown on stdin, freshly rederives the canonical draft from
original immutable sources, and rejects any byte mismatch before indexing or searching it. Source,
policy and source-set drift therefore invalidate the entire draft. Build ephemeral postings from
the verified entries, return at most three matches and original source excerpts; never add a
generated file to repository evidence or persist derived authority. Exact reconstruction establishes
source binding only, not behavioral truth or independent human review. The pure byte-comparison
helper cannot assert `SOURCE_REDERIVED`; only the CLI attaches that label after fresh Git reconstruction.

- `SDD-V0-005`: Bound source preamble/digest to 64 lines, declarations and importers to 64 entries each per page (`SDD-V0-632`),
entry excerpts to 4 KiB, draft output and consume input to 64 KiB, encoded consume JSON including
its trailing newline to 64 KiB, and query to 1024 UTF-8 bytes. Refuse unsupported
sources and overflow; do not silently omit due to limits. Deterministic ranking uses case-insensitive
alphanumeric term overlap, then entry ID. No matches yields NO_CANDIDATES with explicit uncertainty.
The CLI `docs consume` refusals at the consume-input and encoded-result bounds name the bound
(`DraftMaxBytes`), its value in bytes and the observed size; the input size is a lower bound, since
the bounded stdin read stops one byte past the limit (V1-2416,
`TestSDDV0005DocsLimitRefusalNamesLimitAndSize`, which failed against the earlier `draft exceeds
64 KiB` and `encoded consumption result exceeds 64 KiB` messages). The library refusals in
`internal/doccompiler/draft.go`, which `docs draft`, `docs consume` and `docs maintain` also return,
name the bound, its value and the observed size (`documentation-limit-exceeded`, V1-2416). A
`docs maintain` refusal carries that message to stderr unchanged instead of repeating the code
(`TestSDDV0005DocsMaintainLimitRefusalNamesLimitAndSize`).

- `SDD-V0-006`: An MCP client can call `corvint.docs_draft` and `corvint.docs_consume`
(`cmd/corvint-docs-mcp`, package `internal/mcp/docsbridge`) and receive the same claims,
provenance and refusal codes as `corvint docs draft`/`docs consume`, calling the same
native `contextindex.BuildContext`/`doccompiler.DraftSources`/`doccompiler.ConsumeDraft`
functions directly rather than shelling out to the CLI binary. This is a separate,
additive server from `cmd/corvint-mcp`. The separately owner-approved shared MCPV0-021..023
compatibility amendment permits `--protocol-version 2025-11-25` on this existing command;
modern stdio remains the default and draft/consume receipts and authority are unchanged.
For tool scope, that server's exact three-tool surface is frozen by
`docs/specs/mcp-server-2026-07-28-v0.md` requirement `MCPV0-008` ("V0 advertises exactly
these tools") and enforced by the closed `conformance/mcp-2026-07-28` suite, and this
docs-tool slice remains proposed/experimental, not delivered V0; it must not extend or
alter that frozen surface. Amendment (2026-10-23, `MCPV0-101` in
`mcp-server-2026-07-28-v0.md`): `corvint-docs-mcp` also advertises the shared binary-only
`corvint.help` tool of `MCPV0-100`, which reads no repository state and changes neither docs tool.
`docs_draft`'s Markdown output and `docs_consume`'s canonical
JSON output — the `structuredContent` returned alongside each tool result — must be
byte-identical to the CLI's stdout on the same fixture. Repository-authored source
excerpts and owner prose reach a model only through the MCP `content[0].text` block, so
that block, and only that block, is framed by the single `AHI-004` envelope rule
(`internal/repoenvelope.Frame`: hidden-character escaping and terminator refusal);
`structuredContent` is never wrapped or altered, so the byte-identical requirement above is
unaffected. Output whose text contains the envelope terminator is an `isError: true` tool
result with code `corvint-envelope-terminator-collision` and no repository text. Edited
source after the draft's commit, a tampered draft, an invalid or escaping path, and a non-Markdown
owner source each refuse: an invalid path or unsupported tool name is a JSON-RPC
`InvalidParams` protocol error; every other business-level refusal (stale draft, unsupported
source, missing package, malformed or unknown-member arguments) is an MCP tool result with
`isError: true` and the CLI's `*doccompiler.Error` code/message — never `isError: false`.
Rollback: delete `cmd/corvint-docs-mcp` and `internal/mcp/docsbridge`; no migration or
retained repository state exists.

- `SDD-V0-007`: An explicitly enabled, bounded local session (`internal/docmaintain`,
`corvint docs maintain --page PATH --source PATH --package DIRECTORY --enable
(--preview|--apply) [--max-writes N] [--max-wall-clock DURATION] [--watch]`) refreshes exactly one
generated block of one Markdown page (a page path not ending in `.md` is refused as
`invalid-page-path`), delimited by
`<!-- corvint:docmaintain begin source="…" package="…" commit="…" source_digest="…" -->` /
`<!-- corvint:docmaintain end -->`, using the same native `doccompiler.DraftSources` call
`docs draft` uses. Content outside generated markers, and any other selector's block, is
never read as a source of truth and never rewritten. This is a maintenance session over
the SDD-V0-001..006 draft contract, not a new renderer: `docs/specs/human-documentation-
compiler-v0.md` remains partial (HDCV0-023..029, HDCV0-041; its `docs/specs/INDEX.json` delivery,
checked by `SDD-V0-634`), and `docs/specs/deployment-neutral-index-platform-v0.md`'s
statement that this contract does not constitute a delivered continuous-maintenance system
still holds; no generated block is promoted to accepted intent (AGENTS.md invariant 8). An
applied write replaces the page atomically and keeps an existing page's permission bits
(`TestApplyPreservesPageMode`).

- `SDD-V0-008`: Eligibility is decided by comparing the draft's cited source identities — each
entry's path, blob and SHA-256, in `DraftSources`' deterministic order, hashed as one digest —
against the `source_digest` recorded in the page's existing marker for that selector, never by
hashing the rendered Markdown. The rendered Markdown embeds the repository's current commit and
tree, which change on every commit anywhere in the repository regardless of the cited source
content, so a markdown-hash comparison would report false eligibility on unrelated commits and
cannot be used for this bound. A missing block for a selector is always eligible.

- `SDD-V0-009`: `--preview` computes and returns the proposed page bytes, a unified diff against
the page read at session start, and a session receipt, without writing. `--apply` re-reads the
page immediately before writing and refuses with no write, recording a conflict, if its bytes no
longer equal what `--preview`'s computation started from; otherwise it replaces the page with a
temporary file in the same directory followed by a rename, so a reader never observes a partial
write. Applying an already-current block is a byte-identical no-op. The session receipt records,
per selector: existed/eligible/applied/skipped, old and new source digest, and commit/tree; and
for the page: digest before and after, applied vs. preview-only, conflict detail, and which of
`--max-writes`/`--max-wall-clock` (if any) stopped the session. Rollback: disable the session
(omit `--enable`) and restore the page's prior generated bytes from version control; human
prose outside generated markers is never touched and needs no restoration.

- `SDD-V0-010`: The session never runs unless `--enable` is given together with exactly one of
`--preview`/`--apply`; both are refused with an explicit argument error otherwise, before any
repository access. `--max-writes` and `--max-wall-clock` bound a session to a positive count of
block writes and a positive wall-clock duration; reaching either stops the session cleanly with
no partial block write and a receipt naming the stop reason. These bounds and the conflict check
in `SDD-V0-009` are session-local safety, not new authority: a maintenance session still cannot
turn generated prose into an accepted requirement, and every emitted claim still carries the
SDD-V0-003 blob/SHA-256 citation back to immutable Git content.

- `SDD-V0-011`: When `contextindex.BuildContext` refuses because its own Git deadline expired
(the typed `*contextindex.Error` whose `Cause` is `context.DeadlineExceeded`) while the MCP
caller's context is still live, `corvint.docs_draft` and `corvint.docs_consume` report the
bounded code `repository-index-timeout` instead of `repository-unavailable`: the index ran out
of time, the repository is not missing. The classification reads only the typed cause, never
message text. A caller cancellation or expired caller deadline stays `cancelled`; every other
Git read failure keeps `repository-unavailable` (`TestSDD011IndexDeadlineHasDistinctCode`).
Rollback: remove the branch in `transportOrToolFailure`; the overrun reads as
`repository-unavailable` again and no state is retained.

- `SDD-V0-012`: In the `SDD-V0-009` receipt, a block's `applied` member means its refresh is
included in the session's proposed page and counts toward `--max-writes`, in a `--preview` as in
an `--apply`; an ineligible or bounded block reports `applied: false` with its `skipped` reason.
Whether bytes were written is stated only by the page-level members: a preview reports
`applied: false`, `preview_only: true` and `page_digest_after` equal to `page_digest_before`,
and an apply that wrote reports `applied: true` and `preview_only: false`. The wire members are
unchanged (`TestSDDV0012PreviewBlockAppliedMeansIncluded`). Status: accepted under the owner's
standing auto-accept (2026-10-11); V1-2135.

- `SDD-V0-013`: `corvint-docs-mcp` accepts a linked-worktree root whose `.git` is a regular
gitfile, using the same root check as `corvint-mcp` (`internal/mcp/rootbind`, same rules as
`internal/mcp/bridge.New`): a real `.git` directory, or a gitfile whose worktree and common Git
directories exist and keep their identity for the session. A symlinked marker, a gitfile naming a
missing directory, or an identity change still refuses (`repository unavailable` /
`ROOT_IDENTITY_CHANGED`). Non-goal: no new tools and no change to the wire contract. Failure mode:
before this the server refused every linked worktree that corvint-mcp accepted. Status:
experimental under the standing lane auto-accept; V1-2816
(`TestSDD013LinkedWorktreeRoot`). Rollback: restore the directory-only marker check in
`docsbridge.New`.
- `SDD-V0-014`: A page holding `internal/docmaintain` generated blocks MAY declare Go examples that
the Go test provider checks, one line per example, outside every fence and every block body:
`<!-- corvint:docmaintain example source="S" package="DIR" name="ExampleX" -->`. Preview, Apply
and Watch treat the line as prose: they never parse, rewrite or run it, so a refresh keeps it byte
for byte and a second refresh is a no-op. `docmaintain.CheckExamples` reads the declarations
(`DeclaredExamples`, fence- and block-aware), runs each declared package once through an
`ExampleRunner`, and returns `corvint-docs-example-check/0` with each example `PASS`, `FAIL` or
`NOT_RUN` (not observed, for example a build failure, a missing name, or a nonzero `go test` exit
that recorded no `FAIL`, such as a `TestMain` teardown failure). A line that does not parse
(`example-malformed`), sits in a block body (`example-in-generated-body`), names no block on the page
(`example-unbound-block`) or names a package that is absolute, dash-led, or has an empty, `.`, `..`
or `.git` segment (`example-package-invalid`) is a problem; any problem, no declarations, or any
non-`PASS` example makes the verdict `FAIL`. `GoTestRunner` runs `go test -count=1 -json -run
'^(A|B)$' ./DIR` under the root, so Go itself compiles the examples and compares `// Output:`; it
reads at most 64 MiB of the event stream and drains the rest, so a child that keeps writing never
blocks on the pipe; a cancelled run closes its read end and bounds its pipe wait, so a descendant
of the go command that still holds the pipes cannot keep it from returning the context's error; Corvint imports and evaluates nothing. The check runs only from the operator harness
`go run ./tools/docs-blinded-task examples --root R --page P` and is never part of a read command
(`corvint docs draft|consume|maintain` and `corvint-docs-mcp` do not call it; product invariant 4).
Non-goals: `internal/flowdocs` pages, which are whole generated files with no owner prose to hold a
declaration; non-Go examples; wiring the check into `corvint docs maintain`. Failure mode (audit
section 12, V1-2957): generated blocks stated API shape with no executed example, so a stale usage
could read as current. Evidence: `TestSDDV0014ExamplesCheckedThroughGoTest` (a real `go test` run
gives PASS, FAIL and NOT_RUN; declarations survive a source refresh; one provider run per package),
`TestSDDV0014DeclarationProblems`, `TestSDDV0014GoTestRunnerDrainsOutputPastTheBound`,
`TestSDDV0014GoTestRunnerNonzeroExitWithoutFailIsNotRun`,
`TestSDDV0014GoTestRunnerCancelReturnsDespiteDescendantHoldingPipes`. Rollback: delete
`internal/docmaintain/examples.go` and the harness `examples` subcommand; pages keep the lines as inert comments. Status: experimental under
the standing lane auto-accept (V1-2957).
- `SDD-V0-015`: The blinded maintainer task (audit section 13) is defined by
`benchmarks/docs-blinded-maintainer-v0/`. `tasks.json` (`corvint-docs-blinded-task-set/0`) names
the generated reference (`corvint docs draft --source docs/specs/source-documentation-draft-v0.md
--package internal/docmaintain` at revision `aa3b6e4f0`), six declared questions about that
package, the pass ratio 5/6, and the SHA-256 of `answer-key.json`
(`corvint-docs-blinded-answer-key/0`), committed before any response exists so the key cannot be
fitted to responses. The answerer, a maintainer or agent with no prior access to the package, sees
only the task set and the generated reference, never the key or the package source. Responses are
`corvint-docs-blinded-responses/0` (participant, condition, answers by question ID). Scoring rule
(`docmaintain.ScoreBlinded`): a question is correct when its answer contains every required fact of
the key as a case-insensitive, whitespace-collapsed substring; a missing answer is incorrect; the
run passes when `correct * 6 >= total * 5`. The scorer refuses a key whose SHA-256 differs from the
committed one, a key that does not cover exactly the declared questions, and wrong profiles. The
harness `go run ./tools/docs-blinded-task score --tasks T --key K --responses R --out O` writes the
`corvint-docs-blinded-result/0` record (input digests, per-question missing facts, verdict) with an
exclusive create, whether the run passes or fails; results are retained under `results/`.
Non-goals: judging prose quality; a pass claims only that the reference answered these questions
for that participant. Evidence: `TestSDDV0015BlindedScoring`. The blinded run itself is `NOT_RUN`
(it needs a participant outside this lane); README wording about maintainer usability stays
unchanged until a retained result supports or refutes it. Rollback: delete
`internal/docmaintain/blinded.go`, the harness `score` subcommand and the benchmark directory.
Status: experimental under the standing lane auto-accept (V1-2957).
- `SDD-V0-018`: `docs draft`, `docs consume` and `corvint-docs-mcp` accept a module directory in
every language the native index extracts declarations from: Go, Python, TypeScript (`.ts`, `.tsx`),
JavaScript (`.js`, `.jsx`, `.mjs`, `.cjs`), Rust, C#, Swift, Kotlin and Ruby. Indexed symbols carry
no visibility, so the exported-name rule reads the declaration's own original line: Go keeps its
upper-case initial; TypeScript/JavaScript lines begin with `export`; Rust lines begin with `pub`
(`pub(crate)` is not exported); C# carries `public`; Swift carries `public` or `open`; Kotlin carries
none of `private`, `internal`, `protected`; Python and Ruby names do not begin with `_`. C#, Swift
and Kotlin modifiers are read only from the declaration prefix, the code before the first
whole-word occurrence of the declared name, with string literals and comments blanked, so a body,
literal or comment never decides visibility (V1-3074). An occurrence inside a `[...]` attribute or
after `@` or `.` is not the declaration; a C# verbatim string (`@"..."`, `$@"..."`, `@$"..."`)
ends at an undoubled quote, never after a backslash; Swift and Kotlin block comments nest, C# block
comments do not (V1-3077).
Test files are excluded by name: Go keeps exactly `_test.go`; other languages exclude `test_*`,
`*_test.*`, `*_spec.*`, `*.test.*`, `*.spec.*` and a stem ending `Test` or `Tests`. A directory whose
tracked files include no drafted language but do include code is refused with
`unsupported-documentation-source` and the message `unsupported language: the native index extracts
no declarations from L`, where L lists the language names (Java, C, C++, Scala, Shell, SQL, …) or,
for an unrecognised suffix, the suffix itself; documentation, data, stylesheet and template
suffixes never trigger it. The existing code is kept so the SOL-V0-007 closed ledger code set does
not widen. A directory with any non-Go drafted language carries an explicit limitation naming it
and any known unsupported-language code beside it; non-Go importers are `UNKNOWN`, because the
index resolves no non-Go import specifier, so import edges remain Go only. A package whose only
drafted language is Go keeps its previous draft bytes even beside cgo, assembly or script files, so
existing drafts stay current. Suffixes match case-sensitively, as the index dispatches its
extractors; a Go-name rule is decided before the source is read. Non-goals: parsing
visibility beyond the declaration line (Ruby `private` sections, TypeScript `export { a }` lists),
non-Go importer edges, and languages the index reads only as text.
Evidence: `TestDocsMCPDraftsTypeScriptModule` (a committed TypeScript module drafted and consumed
through the `corvint-docs-mcp` tool handler, refused before this change),
`TestDocsMCPRefusesUnsupportedLanguageByName`, `TestSDDV0018DraftLanguagesExportRules`,
`TestSDDV0018VisibilityReadsOnlyTheDeclarationPrefix`. Rollback:
remove the language table in `internal/doccompiler/draft.go`; Go drafts are unaffected. Status:
experimental under the standing lane auto-accept (V1-3072, GH #1974).
- `SDD-V0-019`: `docs draft --dry-run` and `corvint.docs_draft` with `dry_run: true` preview a first
draft before the owner Markdown is committed (V1-3081, GH #1978). The owner bytes come from the
working-tree file at the repository-relative `.md` path given as the source (no symlink component,
no `.git` component, regular file, at most 64 KiB; a FIFO or other non-regular file is refused
from its `lstat` before any blocking open, V1-3089) or inline: CLI `--owner-stdin` reads stdin, MCP
`owner_markdown` carries the string, and the source path then only names it. Every other
SDD-V0-001/002/005/018 rule still applies; declarations and importers still come only from the
admitted index, so the package must be tracked. The page is marked by its own profile
`corvint-source-documentation-draft-dry-run/0`, `Admissibility: NOT_ADMISSIBLE`, `Authority: NONE`
and a leading `DRY RUN` limitation; the owner entry has kind `dry-run-owner`, no Git blob and no
immutable original route. The MCP result adds `dry_run: true`, `admissibility: NOT_ADMISSIBLE` and
`authority: NONE`; a non-dry-run result and page keep their previous bytes. Dry-run output can
never be admitted or cited: `docs consume` and `corvint.docs_consume` refuse any page carrying the
dry-run profile line with `unsupported-documentation-source` (`dry-run draft is not admissible`),
`docs consume --dry-run` and `--owner-stdin` without `--dry-run` are argument errors, `owner_markdown`
without `dry_run` is a malformed-arguments tool failure, and the normal path still refuses an
untracked owner. A dry run writes no repository, index or trace state (product invariant 4); only
the existing SOL-V0-007 ledger on an `unsupported-*` refusal remains. Non-goals: admitting the
preview, maintenance (`internal/docmaintain` calls only `DraftSources`), an untracked package.
Evidence: `TestSDDV0019DryRunDraftIsMarkedAndNeverConsumed`, `TestSDDV0019ReadDryRunOwnerIsConfined`,
`TestSDDV0019ReadDryRunOwnerRefusesFIFO`,
`TestSDDV0019DocsCLIDryRunUntrackedAndInlineOwner`, `TestDocsMCPDryRunAcceptsUntrackedOrInlineOwner`.
Rollback: remove `DraftDryRun`, `ReadDryRunOwner` and the two flags/members; admitted drafts are
unaffected. Status: experimental under the standing lane auto-accept (V1-3081).
- `SDD-V0-700`: (experimental under the standing lane auto-accept; V1-3706) `corvint.docs_draft`
without `dry_run` and `corvint.docs_consume`, in `corvint-docs-mcp` and the `docs` group of
`corvint mcp`, MUST refuse before building the source index a package that is not one module
directory and an owner source that is not Markdown or is not in the HEAD commit the index reads
(`git cat-file -e HEAD:PATH`; untracked and staged-only files are absent), with exactly the
`unsupported-documentation-source` code and message the full rederivation returns; every other
input is decided by the index as before, and a Git probe that cannot run leaves the decision to
it. Such a refusal answers in under 100 ms instead of after a full index build (about 2 s on this
repository). Non-goals: pre-deciding package declarations (they need the index), the CLI `docs`
verbs. Evidence: `TestSDDV0700AbsentOwnerSourceRefusesBeforeTheIndex` (missing, untracked, staged
and non-Markdown sources, both tools: equal to `DraftSources` over a full index, under 100 ms, no
build; a committed owner still builds). Rollback: drop `PrecheckDraft` and `Registry.precheck`.

- `SDD-V0-630`: Every draft states whether its owner excerpt names the drafted package (V1-3719).
The relation is `NAMES_PACKAGE` when the owner excerpt contains the package directory as a whole
path token: the directory string not preceded by a letter, digit, `_`, `-`, `.` or `/` and followed
by end of text, `/`, a character other than a letter, digit, `_`, `-` or `.`, or a `.` that is
itself not followed by a letter, digit, `_` or `-` (prose punctuation). Otherwise it is
`OWNER_UNRELATED`. The header carries `Owner relation: NAMES_PACKAGE|OWNER_UNRELATED`; an unrelated
draft also leads its limitations with `OWNER_UNRELATED: the owner excerpt does not name this
package; its prose may describe a different capability and governs nothing here.` The label is
orientation, not a refusal, so existing owners and fixtures still draft; it never strengthens
authority. Evidence: `TestSDDV0630OwnerRelationLabelsUnrelatedOwner`. Rollback: remove the relation
line, limitation and helper. Status: experimental under the standing lane auto-accept (V1-3719).

- `SDD-V0-631`: A Go declaration entry spans its doc comment and its full declaration surface, not
one signature line (V1-3717): a function or method from its doc comment through the line of its
body's opening brace (the whole declaration when it has no body); a type from its doc comment
(the spec's own, or the enclosing single-spec `type` group's) through the end of its spec,
including struct fields and interface methods; a constant or variable spec from its doc comment
through the end of that spec; an ungrouped `type`, `const` or `var` declaration spans its whole
declaration including the keyword. The span is derived by `go/parser` over the same admitted blob;
a parse failure keeps the indexed single line, and a span whose excerpt exceeds 4 KiB or contains
a carriage return cites its indexed line with a counted limitation (the indexed line itself must
still pass every `SDD-V0-003` excerpt rule). Each Go package directory with a package doc comment adds
one `package-doc` entry (the first non-test file in path order whose `package` clause carries a doc
comment, from the comment through the clause; one over 4 KiB is omitted with a limitation naming
its file). Other languages keep the indexed line. Provenance is printed once per source file:
entries are grouped under one `## S<n> "PATH"` heading per cited file, `S1`… in path order, which
carries the file's blob, SHA-256 and its indented `git --no-pager --no-replace-objects show BLOB`
route; under it each entry, in line order, prints only its kind and title (a declaration prints
neither, since its excerpt shows the name) and its line span, then the exact excerpt indented four
spaces. A blobless dry-run owner keeps its own heading. Entry JSON (`DraftEntry`) keeps its
per-entry path, blob, SHA-256 and lines, so consumption results and the `SDD-V0-008` source
identity are unchanged. Acceptance: the `internal/docmaintain` draft is no larger than
`go doc -all ./internal/docmaintain` at the same commit, both measured and retained in the lane
build log. Evidence: `TestSDDV0631DeclarationsCarryDocCommentsAndFields`,
`TestSDDV0631ProvenanceOncePerFile`. Rollback: restore single-line spans and per-entry provenance.
Status: experimental under the standing lane auto-accept (V1-3717).

- `SDD-V0-632`: Declarations and importers are paged independently, 64 entries per list per page,
in entry-ID order (V1-3718). `docs draft` and `docs consume` accept `--page N` (default 1); the
page count is the larger of the two lists' page counts, at least 1. A page outside `1..count` is
refused as `documentation-limit-exceeded`, naming the page and the count. When either list
exceeds 64 entries the header carries `Page: N of P` and the limitations carry one line per cut
list, `TRUNCATED: declarations A–B of M on page N of P; the other M-k entries are UNKNOWN on this
page.` (importers likewise; a list with no entries on the page says `0 of M`).
A package whose lists both fit one page keeps its bytes free of any page or TRUNCATED line. The
`SDD-V0-005` byte bounds still apply per page. `corvint-docs-mcp` and `docs maintain` draft page 1,
so an over-64 package now yields a labelled first page there instead of a refusal. The `docs maintain`
source digest covers each TRUNCATED line as well as the page's entries, so an entry added or removed
on a later page refreshes the page-1 block instead of reporting it `unchanged`; an unpaged draft keeps
its earlier digest. Evidence: `TestSDDV0632PagesOverSixtyFourEntries`,
`TestSDDV0632ImportersCountSeparately`, `TestSDDV0632MaintainDigestCoversTruncation`. Rollback: restore
the combined refusal. Status: experimental under the standing lane auto-accept (V1-3718).

- `SDD-V0-633`: Foreign repositories need no Corvint-specific heading (V1-3721). `docs draft`,
`docs consume` and `docs draft --dry-run` accept `--owner-section HEADING`: the owner entry is then
exactly one literal `## HEADING` line outside fenced code, and its excerpt is that section alone
(heading through the line before the next `## ` line or end of file), at most 64 lines and 4 KiB.
HEADING is 1..200 bytes of printable UTF-8 without a newline or surrounding spaces. A non-default
section adds the header line `Owner section: ` followed by the backticked quoted heading, like the
other header metadata; the default `Agent digest` keeps the preamble-plus-digest excerpt and its
bytes. The product check lives in `internal/doccitations`. `corvint docs citations check [--root PATH] [PATH...]` is the product line-citation
check for any Git repository: it scans tracked Markdown (or the named tracked files) for backticked
`path:N`, `path:N-M` or `path:N,M[,…]` tokens with an optional `@HEX` anchor whose path is a tracked
file, reads each cited file's index blob, and reports per citation `out-of-range`, `trivial-span`
(only blank or closing-punctuation lines) or `anchor-mismatch` (the anchor is not a prefix of the
SHA-256 of the cited right-trimmed lines joined by LF, the same rule as `script/check-line-citations.sh`);
a token whose path has a `/` and whose first segment is a tracked directory but is not itself
tracked is `missing-file`. Output is one line per failure and a summary; exit 0 when clean, 1 on any
failure, 2 with `citations-check-failed` on argument or repository errors. It reads only and writes nothing. It does not replace
the repository's own `make line-citations-check`, whose extra Corvint scope rules remain script
policy. Evidence: `TestSDDV0633OwnerSectionDraftsWithoutAgentDigest`,
`TestSDDV0633CitationsCheckForeignRepository`. Rollback: remove the flag and the `citations`
subcommand. Status: experimental under the standing lane auto-accept (V1-3721).

- `SDD-V0-634`: Maturity statements in Go help strings and package docs of the form
``docs/specs/NAME.md` remains STATUS`` must agree with that spec's `delivery` in
`docs/specs/INDEX.json`: STATUS must equal the delivery field's leading word (`partial`,
`not-started`, `delivered`, ...) and, when the delivery names a parenthesized requirement range,
STATUS carries the same parenthesized text (V1-3720). Evidence:
`TestSDDV0634MaturityStringsMatchSpecIndex`, which scans the non-test Go files of `cmd/corvint` and
`internal/docmaintain` and fails on any disagreement. Rollback: remove the test. Status:
experimental under the standing lane auto-accept (V1-3720).


## CLI contract

```
corvint [--root PATH] docs draft --source PATH --package DIRECTORY [--page N] [--owner-section HEADING]
corvint [--root PATH] docs draft --dry-run [--owner-stdin] --source PATH --package DIRECTORY [--page N] [--owner-section HEADING]
corvint [--root PATH] docs consume --source PATH --package DIRECTORY [--page N] [--owner-section HEADING] --task TEXT < draft.md
corvint [--root PATH] docs citations check [PATH...]
```

Help must disclose the admission prerequisites: committed HEAD sources; admitted tracked,
nongenerated UTF-8 LF Markdown with exactly one literal digest outside fenced code; the combined
64-line/4-KiB preamble and digest ceiling; and a repository-relative Go package directory other than
`.` containing supported tracked non-test exported declarations. Help supplies a draft/consume
example and this contract's path. From a Corvint checkout with committed sources:

```sh
corvint docs draft --source docs/specs/source-documentation-draft-v0.md --package internal/docmaintain > /tmp/corvint-source-draft.md
corvint docs consume --source docs/specs/source-documentation-draft-v0.md --package internal/docmaintain --task Preview < /tmp/corvint-source-draft.md
```

`internal/doccompiler` itself exceeds the SDD-V0-005 64-declaration/importer bound per page; since
`SDD-V0-632` it drafts as labelled pages (V1-3718), while before that it was refused and not a
working example (V1-0030, 2026-09-22); `internal/docmaintain` is the smallest package this repository
keeps under that bound while still producing a non-empty `--task Preview` consumption result.

`draft` emits Markdown only. `consume` emits JSON fields `profile` (`corvint-documentation-consumption/0`),
`state` (`READY` or `NO_CANDIDATES`), `derivation` (`GENERATED`), `validation` (`SOURCE_REDERIVED`),
`behavior` (`UNKNOWN`), `commit`, `tree`, `draft_sha256`, `task`, `results`, and `limitations`.
A result contains `id`, `kind`, `title`, `excerpt`, `path`, `blob`, `sha256`, `start_line`, `end_line`.
The draft's internal profile is `corvint-source-documentation-draft/0`; policy is `source-orientation/0`.
This adds no fields or semantics to existing commands or frozen protocols.

Argument errors use the existing `invalid-arguments` stderr envelope. Draft failures use
`unsupported-documentation-source`, `documentation-limit-exceeded`, or `stale-documentation-draft`
in the existing JSON stderr envelope, nonzero exit and empty stdout. Git/context cancellation errors
retain existing error handling. Commands do not write repository, index or trace state except the
existing bounded local self-observation ledger on non-query `unsupported-*` failures (SOL-V0-007,
product invariant 4). Its closed writer code set admits exactly `unsupported-documentation-source`
for this slice; arbitrary neighboring codes and prose remain refused. This ledger never enters
ranking, learning, evidence or authority. Successful
commands and other refusals remain read-only.

### Git status safety

Both commands enable the same private-metadata status boundary as MCP before building the source
index. Unsafe or unsupported Git configuration and metadata fail closed: includes, executable
clean/process filters, external attributes files, worktree redirection, bare repositories,
ambiguous multiline configuration, split indexes and index gitlinks are not silently normalized or
ignored. Temporary status metadata stays outside the repository and Git directory and is removed
after use; cancellation releases its bounded status slot, one of two process-wide. These refusals retain the existing
repository-error path and do not add a self-observation event. Other standalone CLI commands retain
their existing execution profile. This is read safety, not renderer or HDC qualification.

## Non-goals and failure modes

No MkDocs execution/environment fiction, HDC plan/receipt, docviews qualification, accepted prose
rewrite, persistent generated index, human approval, code behavior inference, active-build selection,
callgraph, absence proof, incremental claim maintenance, tombstones/conflict lifecycle or publication.
Those remain in AT-15 and their owning contracts. Missing/invalid/generated sources, unparsed code,
limits and stale/tampered drafts cannot silently become stronger evidence. The unsupported-error
ledger exception records friction without strengthening evidence.

### Maintenance session refusal codes

The `SDD-V0-007` maintenance session (`internal/docmaintain`) refuses with the kebab-case codes
below (decision 0100). Each row cites the first emitting site and states only the condition checked
there. A `doccompiler.DraftSources` refusal at `internal/docmaintain/docmaintain.go:157` is also
returned under its own draft code (`unsupported-documentation-source` or
`documentation-limit-exceeded`), owned by the draft contract above.

| Code | First emitting site | At the cited site |
|---|---|---|
| `apply-not-authorized` | `internal/docmaintain/docmaintain.go:222@9451df78` | `Apply` was called with `policy.Apply` false |
| `internal-error` | `internal/docmaintain/docmaintain.go:286@fa68fda3` | a `DraftSources` failure is not a `doccompiler.Error` |
| `invalid-page-path` | `internal/docmaintain/docmaintain.go:124` | the joined page path is not contained in the root, crosses a symlink, has a component naming `.git` in any letter case, or does not end in `.md` (`Preview` and `Apply`; `TestGitMetadataPageIsRefused`, `TestNonMarkdownPageIsRefused`) |
| `invalid-root` | `internal/docmaintain/docmaintain.go:120` | the root cannot be made absolute (`Preview` and `Apply`) |
| `invalid-selector` | `internal/docmaintain/docmaintain.go:153` | a selector source or package contains `-->` or any character Go `%q` escapes (a double quote, backslash, or non-printable rune), since the marker records the escaped value and the block could never be re-found (`TestSelectorCommentBreakoutRefused`, `TestSelectorThatQuotingChangesIsRefused`) |
| `maintenance-conflict` | `internal/docmaintain/docmaintain.go:256` | at `Apply`, the page existence or SHA256 differs from what `Preview` started from; the result is returned with `Conflict` set and nothing is written |
| `maintenance-session-disabled` | `internal/docmaintain/docmaintain.go:107` | `Preview` was called with `policy.Enabled` false |
| `marker-in-content` | `internal/docmaintain/docmaintain.go:161` | the new draft Markdown, or the body of the existing block for that selector, contains the begin-marker prefix or the end marker |
| `duplicate-block` | `internal/docmaintain/render.go:160@0b71614d` | two generated blocks outside fenced code name the same selector; replacing only one would leave the other stale while later sessions report `unchanged`, so the whole page is validated before any splice and nothing is written (V1-0461, `TestDuplicateAndOrphanMarkersAreRefused`) |
| `malformed-block` | `internal/docmaintain/render.go:131@2b42622e` | outside fenced code, a marker line is not part of exactly one well-formed block: a begin without its end, an end without a begin, a begin nested in a block, or a marker line that does not parse. Appending or splicing would wedge every later session, so nothing is written (V1-0461, `TestMalformedSelectorBlockIsRefused`, `TestDuplicateAndOrphanMarkersAreRefused`). A begin marker followed directly by its end marker is an empty block and is replaced in place (`TestEmptySeededBlockIsReplacedInPlace`). Markers and the insertion point inside ``` or ~~~ fences (CommonMark length and closing rules) are prose (`TestMarkersInsideFencedCodeAreProse`) |
| `no-preview` | `internal/docmaintain/docmaintain.go:219` | `Apply` was called with a nil preview |
| `no-selectors` | `internal/docmaintain/docmaintain.go:110` | `Preview` was called with no selectors |
| `page-unreadable` | `internal/docmaintain/docmaintain.go:130` | reading the page failed for a reason other than absence (`Preview`, and `Apply` before its conflict check) |
| `repository-unavailable` | `internal/docmaintain/docmaintain.go:136` | `contextindex.BuildContext` cannot index the root, including a root that is not a Git work tree (`TestNonRepositoryRootIsRefused`) |
| `write-failed` | `internal/docmaintain/docmaintain.go:259@45c612cd` | the atomic write of the proposed page failed |

## Acceptance and rollback

Tests cover deterministic source-bound bytes, actual stdin consumption, immutable source despite
dirty worktree, unsupported/generated sources, tamper/source/policy drift, negative queries, bounds,
and original declaration/import source rederivation. Repair regressions cover hostile tracked owner,
declaration and importer paths without Markdown breakout, exact post-fence digest prose and rejection
of unclosed fences, actual emitted-byte consumption, execution of displayed immutable original routes
despite dirty worktree bytes and replacement refs, and the help's complete invocation on committed
fixtures. These structural and command checks are not a renderer or human usability qualification.
The four development tasks were registered
before implementation; count every baseline/candidate attempt, errors, complete response bytes,
latency and unknown token billing. Independently inspect originals for correctness. This different-
command comparison cannot establish product superiority or full AT-15/HDC qualification.

Remove the docs command and draft module to roll back; no migration or retained repository state exists.

The frozen four-question development screen and a subsequent committed self-use passed independent
original-source review, including the new importer and exact old-draft refusal. Canonical checks
qualified by the recorded full-suite run, focused repairs and remaining gate phases. Native OCM
consumption also exposed and repaired the owning spec's Markdown shape. These observations do not
promote intent, behavioral authority, full AT-15 or HDC delivery. See the improvement-loop review record.

## Proposed foreground watch extension (SDD-V0-007..010)

Intent remains proposed; this records implemented experimental behavior for owner review,
not new accepted authority. Without `--watch`, maintenance remains one preview/apply cycle.
The Unix-only bare, unique `--watch` option requires `--enable --apply`. Other platforms
refuse `unsupported-watch-platform` before repository or page access, even for an absent page. Duplicate boolean flags,
inline boolean values and preview plus apply refuse before repository access.

The foreground command applies an initial eligible block, then observes committed HEAD once
per second until a bound or refusal stops it. No daemon, service, external trigger, MCP
writer or additional runtime is installed. Dirty source bytes do not trigger maintenance.
Each identity observation freshly uses `gitauth.Open`, `Resolve(HEAD)` and `CommitTree`,
including reciprocal linked worktrees and packed refs through the existing bounded Git
reader. Rebuild only when commit/tree changes; eligibility remains the cited-source digest
in SDD-V0-008. Unrelated commits do not cause false writes. Removing a cited declaration can
refresh the block; removing the explicitly selected owner source refuses visibly.

Bind each preview's commit/tree to the tick's observed identity. Re-read HEAD immediately
before Apply: drift refuses `source-drift` without writing. Re-read after Apply: drift
reports `source-superseded` and the actual written identity with incomplete status, without
rollback. Failed post-write verification reports `source-incomplete`. Git and filesystem
mutation are not one transaction; this remaining race is explicit uncertainty.

Retain expected whole-page existence and digest across every tick, including unchanged HEAD
and no-eligible-write ticks. Only exact bytes from this session's successful write advance
that baseline. Any other edit or deletion stops `maintenance-conflict`; human edits are
never adopted silently as a new baseline. Existing pre-rename checks and page mode
preservation remain in force. This scope does not claim an atomic filesystem compare-and-swap.

Watch bounds are positive write count <=1024 and wall-clock <=24 hours, at most
`min(ceil(duration/1s), 86400)` cycles including the initial cycle, and one second of waiting
after each completed tick. A global deadline covers idle waits and source compilation.
Each identity observation admits four Git operations within ten seconds; existing compiler
Git reads retain their 30-second and output-size bounds. Existing Git runners contain and
join child groups on cancellation. SIGINT/SIGTERM stop the foreground command visibly.
Watch bounds regular page reads and proposed bytes to 1 MiB and refuses overflow. It skips
the one-shot preview's quadratic line-diff construction because it emits digest summaries.

The terminal `corvint-docmaintain-watch/0` receipt is <=64 KiB, with <=32 summaries containing
commit/tree/source digest/page digest/status, an omitted-summary count, cycles, actual writes,
`complete`, and the precise stop reason. No tick pages/diffs or unbounded history accumulate.
Normal bound stops are `max-writes`, `max-wall-clock`, `max-cycles`; interruption is
`interrupted`. Conflicts, refusals and superseded or unverifiable writes are incomplete.
Existing one-shot receipt and page-limit behavior remain compatible.

### Watch evidence and rollback

`internal/docmaintain/watch_test.go` covers committed changes, pre-write drift, post-write
supersession, idle human edits/deletion, unrelated/dirty changes, owner removal, bounds,
cancellation, packed refs and linked worktrees. `cmd/corvint/docs_maintain_signal_unix_test.go`
interrupts the actual command while a bounded Git child has a descendant and checks both exit.

`internal/docmaintain/testdata/watch_mcp_proof.py --corvint ABS --docs-mcp ABS --output-root DIR`
uses caller-supplied binaries without rebuilding: one watch invocation applies an initial
block and a later source commit, then actual MCP discovery with required `_meta`, draft and
exact-byte consume demonstrate `SOURCE_REDERIVED`. The fixture retains raw watch/draft/consume
receipts. The corrected source-built development proof passed actual server/discover, tools/list,
draft and exact consume with bounded frames and validated response IDs. Installed qualification remains NOT_RUN
in this leaf and belongs to the separate release artifact gate. Neither source binding nor
this test establishes human usefulness, behavioral truth, owner acceptance or HDC delivery.

Rollback: omit `--watch`. No durable watcher state, service or migration survives the command.

Watch refusals also name `invalid-watch-bounds`, `maintenance-incomplete` and
`page-too-large` explicitly; none authorizes a partial page write.
