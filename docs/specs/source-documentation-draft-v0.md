# Source Documentation Draft V0

Owner: Russell Lewis
Drafted: 2026-09-05
Intent status: proposed
Delivery status: experimental
Implementation: `internal/doccompiler/draft.go`, `cmd/corvint/docs.go`, `internal/docmaintain`, `cmd/corvint/docs_maintain.go`
Authoritative inputs: `docs/PRODUCT.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `ROADMAP.md` AT-15

## Agent digest
- Claim: Corvint drafts source-pinned Markdown from owner prose and indexed Go declarations, then consumes its bytes after original-source rederivation.
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
`_test.go`. Label these tracked non-test declarations, never active production code. Preserve the
indexed kind vocabulary and exact original declaration line. Include positive tracked non-test Go
import edges to the selected module-qualified package with original import anchors. Omit facts
from sources with `Unparsed` diagnostics. Build applicability, runtime callers and behavioral
validation remain UNKNOWN; zero observations establish no absence claim.

- `SDD-V0-003`: Every entry carries original path, blob, SHA-256 and line span. The page binds immutable
commit/tree plus profile/policy. Its GENERATED derivation never promotes source authority. Report
index exclusions and incomplete extraction as coverage limitations. All source facts remain quoted
observations, independent of the user's query or generated orientation text.
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

- `SDD-V0-005`: Bound source preamble/digest to 64 lines, declarations and importers to 64 entries each,
entry excerpts to 4 KiB, draft output and consume input to 64 KiB, encoded consume JSON including
its trailing newline to 64 KiB, and query to 1024 UTF-8 bytes. Refuse unsupported
sources and overflow; do not silently omit due to limits. Deterministic ranking uses case-insensitive
alphanumeric term overlap, then entry ID. No matches yields NO_CANDIDATES with explicit uncertainty.

- `SDD-V0-006`: An MCP client can call `corvint.docs_draft` and `corvint.docs_consume`
(`cmd/corvint-docs-mcp`, package `internal/mcp/docsbridge`) and receive the same claims,
provenance and refusal codes as `corvint docs draft`/`docs consume`, calling the same
native `contextindex.BuildContext`/`doccompiler.DraftSources`/`doccompiler.ConsumeDraft`
functions directly rather than shelling out to the CLI binary. This is a separate,
additive server from `cmd/corvint-mcp`: that server's exact three-tool surface is frozen by
`docs/specs/mcp-server-2026-07-28-v0.md` requirement `MCPV0-008` ("V0 advertises exactly
these tools") and enforced by the closed `conformance/mcp-2026-07-28` suite, and this
docs-tool slice remains proposed/experimental, not delivered V0; it must not extend or
alter that frozen surface. `docs_draft`'s Markdown output and `docs_consume`'s canonical
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
compiler-v0.md` remains not-started, and `docs/specs/deployment-neutral-index-platform-v0.md`'s
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

## CLI contract

```
corvint [--root PATH] docs draft --source PATH --package DIRECTORY
corvint [--root PATH] docs consume --source PATH --package DIRECTORY --task TEXT < draft.md
```

Help must disclose the admission prerequisites: committed HEAD sources; admitted tracked,
nongenerated UTF-8 LF Markdown with exactly one literal digest outside fenced code; the combined
64-line/4-KiB preamble and digest ceiling; and a repository-relative Go package directory other than
`.` containing supported tracked non-test exported declarations. Help supplies a draft/consume
example and this contract's path. From a Corvint checkout with committed sources:

```sh
corvint docs draft --source docs/specs/source-documentation-draft-v0.md --package internal/doccompiler > /tmp/corvint-source-draft.md
corvint docs consume --source docs/specs/source-documentation-draft-v0.md --package internal/doccompiler --task Plan < /tmp/corvint-source-draft.md
```

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
| `apply-not-authorized` | `internal/docmaintain/docmaintain.go:219` | `Apply` was called with `policy.Apply` false |
| `internal-error` | `internal/docmaintain/docmaintain.go:283` | a `DraftSources` failure is not a `doccompiler.Error` |
| `invalid-page-path` | `internal/docmaintain/docmaintain.go:124` | the joined page path is not contained in the root, crosses a symlink, has a component naming `.git` in any letter case, or does not end in `.md` (`Preview` and `Apply`; `TestGitMetadataPageIsRefused`, `TestNonMarkdownPageIsRefused`) |
| `invalid-root` | `internal/docmaintain/docmaintain.go:120` | the root cannot be made absolute (`Preview` and `Apply`) |
| `invalid-selector` | `internal/docmaintain/docmaintain.go:153` | a selector source or package contains `-->` or any character Go `%q` escapes (a double quote, backslash, or non-printable rune), since the marker records the escaped value and the block could never be re-found (`TestSelectorCommentBreakoutRefused`, `TestSelectorThatQuotingChangesIsRefused`) |
| `maintenance-conflict` | `internal/docmaintain/docmaintain.go:253` | at `Apply`, the page existence or SHA256 differs from what `Preview` started from; the result is returned with `Conflict` set and nothing is written |
| `maintenance-session-disabled` | `internal/docmaintain/docmaintain.go:107` | `Preview` was called with `policy.Enabled` false |
| `marker-in-content` | `internal/docmaintain/docmaintain.go:161` | the new draft Markdown, or the body of the existing block for that selector, contains the begin-marker prefix or the end marker |
| `no-preview` | `internal/docmaintain/docmaintain.go:216` | `Apply` was called with a nil preview |
| `no-selectors` | `internal/docmaintain/docmaintain.go:110` | `Preview` was called with no selectors |
| `page-unreadable` | `internal/docmaintain/docmaintain.go:130` | reading the page failed for a reason other than absence (`Preview`, and `Apply` before its conflict check) |
| `repository-unavailable` | `internal/docmaintain/docmaintain.go:136` | `contextindex.BuildContext` cannot index the root, including a root that is not a Git work tree (`TestNonRepositoryRootIsRefused`) |
| `write-failed` | `internal/docmaintain/docmaintain.go:256` | the atomic write of the proposed page failed |

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
