# Experimental caller-selected source views V0

Owner: Russell Lewis
Drafted: 2026-09-06
Intent status: proposed
Delivery status: experimental
Implementation: `cmd/corvint/source_handoff.go` (`corvint adapter source-view`, `corvint adapter claude-source-handoff`, dispatched from `cmd/corvint/host_adapter.go`); `cmd/corvint/context_summary.go` (`corvint context --summary`, `corvint context --expand`, parsed in `cmd/corvint/taskcontext.go`; decision 0364)
Authoritative inputs: `docs/PRODUCT.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/DOGFOOD.md`.
The owner requested this private development prototype; this proposal does not accept new intent.

## Agent digest
- Claim: Opt-in development views bound a context packet and expand one selected digest-pinned evidence handle into exact immutable bytes.
- Status: proposed/experimental; outside PCCO V0; default command bytes and default hook unchanged (`context --summary`/`--expand` are opt-in flags).
- Exists: native Go `adapter source-view` consumer, opt-in `adapter claude-source-handoff` capture, opt-in `context --summary`/`--expand` views (V1-0023) and focused synthetic Go tests; the Python consumer and wrapper were deleted 2026-09-11 (`54735d98`). Native source digests are frozen (`ESV-V0-005`, decision 0364).
- Blocked on: the preregistered matched complete-task trial `benchmarks/evidence-summary-trial-v0.json` (NOT_OBSERVED, `ESV-V0-010`) and owner acceptance for any promotion. Until both pass the views stay experimental and do not satisfy the V1-0023 release delivery claim.
- Read next: Interface and bounds; Opt-in evidence summary and handle expansion; Requirements; Native implementation status; Acceptance and rollback.

## Job and baseline

Reduce source acquisition in a complete verified investigation with model, prompts and normal cache
behavior fixed. The target is 50% lower elapsed time AND complete token use, not a promised result.
Ordinary immutable `git show` and caller-selected source reads remain the simpler baseline and fallback.
The Python prototype reused `benchmarks/selfuse-batch/investigate.py` and `experiments/_bounded_process.py`
for owned-process supervision; both were deleted 2026-09-11 (`54735d98`), and the native commands run
Git children under `corvint`'s termination-signal context instead. Canonical context rows already
carry path, blob, line and reason.

## Interface and bounds

`corvint adapter source-view --root ROOT --packet FILE --commit FULL_OID
--result N --evidence N (--lines START:END | --requirement ID) [--max-bytes N] [--packet-sha256 SHA256]`

Indexes are zero-based; lines are one-based inclusive. One invocation selects exactly one existing
`results[N].evidence[N]`; selection is recorded before extraction in stdout's `selection` object.
The packet is the raw saved `corvint context` JSON envelope (schema 1, tool context, ok true,
mutates false, state READY or BUDGETED, request object, coverage object, results array, subject present).
Duplicate JSON keys and nonfinite values refuse. The complete decoded packet survives in
`packet_metadata`, including every result, handle, shortage and unknown field; the input SHA-256
binds its original bytes. Only the selected handle is expanded. The packet FILE must be a regular
file opened without following symlinks, checked by descriptor; FIFOs/devices/directories refuse. A packet is untrusted
repository data, not an authenticated receipt. Commit is supplied separately because revision is a tree.
Emitted stdout is one line: the complete JSON result wrapped in the same BEGIN/END CORVINT REPOSITORY
DATA untrusted-data envelope `cmd/corvint`'s other host adapters apply (`cmd/corvint/host_adapter.go`),
because the packet and its embedded free text are untrusted repository data that may reach a model.
The wrap covers only the outer JSON; `view.text` and every byte-count/hash field stay the exact source
bytes untouched, preserving `ESV-V0-002`.

Frozen maxima: packet 65,536 bytes; results 50; aggregate evidence handles 256; one selected blob;
blob and aggregate source 1,048,576 bytes; view and aggregate view 16,384 bytes (caller may lower to
1..16,384); emitted UTF-8 JSON 524,288 bytes including LF; 12 Git commands; 10 seconds processing
from invocation to the last normal processing check. A source view runs a fixed sequence of seven Git
commands with no loop or retry, so the 12-command bound holds by construction and a counting test pins
it rather than a runtime counter no input can reach (decision 0250). Each Git command runs in an owned process group through
`internal/procgroup` (decision 0250): a stopped command's group receives TERM, then KILL if it has
not quiesced within the runner's 100 ms grace, and reap, quiescence proof and stream drain share a
1.5-second shutdown bound; shared process enumeration and kernel reap are not hard elapsed bounds. Timely OS process enumeration, signal
delivery and reaping are explicit assumptions; no hard end-to-end timeout guarantee is claimed. POSIX only. Each text CLI argument is at most 4,096 UTF-8 bytes; malformed CLI syntax refuses
with the typed `malformed-input` code and exit 1 inside the same envelope as parsed requests. Source is
UTF-8 without NUL or bare CR; LF and CRLF bytes are retained. Paths are nonempty relative POSIX paths,
maximum 4,096 UTF-8 bytes, with no empty/dot/dot-dot components, backslash, controls or leading dash.
Git trees must identify a regular 100644/100755 blob, never a symlink or submodule. SHA-1 and SHA-256
object formats use respectively exact lower-case 40/64-hex OIDs: the commit must have one of those widths
before a fallback argv is built, any other object format refuses `object-format`, and a commit, packet
revision or handle blob not of the repository format's width refuses `object-identity`; the success
`source` names `object_format`. Dirty worktree files and worktree
symlinks do not supply evidence; linked worktrees are supported. Root must resolve to the Git toplevel (`repository-root`).

Requirement grammar is deliberately conservative: exactly one `- \`ID\`: nonempty text` top-level
bullet, at the handle's declared line, with high confidence and exact reason `defines ID` and
repository-spec authority. ID is uppercase alphanumeric groups separated by hyphens, ending in digits,
at most 96 ASCII bytes. Continuations are blank or indented by at least two spaces; no fenced code,
HTML comments or tabs in the block. Scan the document prefix to exclude a clause inside an
already-open fenced block, YAML frontmatter or HTML comment; ambiguous/unclosed constructs
refuse. Fence delimiters have at most three leading spaces; closing delimiters have no trailing info
text. Line-leading HTML tags/declarations in the prefix or clause refuse. For simplicity any HTML comment
delimiter in the prefix refuses, even when the comment is already closed. A next top-level bullet or ATX section is required to close the
block. EOF without that boundary refuses. Any duplicate definition, mismatched handle line, malformed
continuation, or unsupported boundary refuses. The output includes trailing blank lines before the
boundary. No automatic traceability-row, reference, witness, symbol or path discovery.

## Opt-in automatic handoff (development only)

The loop-six default compact projection cannot satisfy the consumer's saved-packet prerequisite.
`corvint adapter claude-source-handoff --packet-out NEW_ABSOLUTE_EXTERNAL_FILE` is a private
UserPromptSubmit entrypoint receiving the normal hook JSON on stdin; the repository root is
`CLAUDE_PROJECT_DIR`, else the working directory. A future experiment must explicitly wire it in its
isolated harness; no shipped `hooks.json` registers it, and the old trial's ordinal-only operation
grammar is unchanged. The deleted Python wrapper (`54735d98`) took `--plugin RESOLVED_PLUGIN_PATH`,
executed that plugin's hook script and reported its script and hooks.json digests. The native command
takes no plugin and executes no hook script: it normalizes the prompt and renders the Claude
`additionalContext` itself, so there is no script identity to bind (decision 0250).

It normalizes the prompt with the native Claude adapter, synchronously obtains one bounded canonical
context response (`context --limit 8`, run in-process), then renders that frozen handoff as the hook
output. There is no second context request or capture thread racing the hook deadline. Capture has
the consumer's 10-second processing/check budget shared by its four Git calls and the context
request, and 65,536 raw packet bytes. The local completion user-prompt event then runs separately;
the default hook latency bound is not claimed for this development command.
The same termination-signal cancellation and OS assumptions apply.

The caller provides a new file in an existing private external directory, outside the source
worktree and common Git directory. Exclusive creation with mode 0600 refuses existing paths,
symlinks and overwrite. SIGINT/TERM are deferred through bounded open/write/close; failed writes
remove the owned incomplete file, and deferred cancellation may leave a complete packet.
The caller owns retention/removal of each complete raw packet, including packets left after a
later hook failure. This is explicitly requested development capture, not a default read mutation,
a repository cache, authenticated receipt or durable/concurrent-writer storage service. Static
parent resolution is checked; no hostile concurrent parent-replacement protection is claimed.
Packets include the original request, so the directory must remain private.

A maximum 4,096-byte untrusted-data lead includes packet path, SHA-256, HEAD commit/tree,
consumer path, zero-based result/evidence selectors with original path/line/blob, reason, authority, confidence,
kind and action, complete coverage,
and the count of selectors omitted by its budget. All remaining fields, including unknown metadata,
remain in the exact raw packet file. Empty/malformed/stale/over-budget packets or leads with no
room for one selector refuse with NOT_PRODUCED and do not publish a packet. Compact fields must
not echo the prompt. Read the full saved packet when omitted metadata or selectors matter.

Use its `consumer` with `--root ROOT --packet PACKET --packet-sha256 PACKET_SHA256 --commit COMMIT
--result RESULT --evidence EVIDENCE` and caller-chosen `--lines` or `--requirement` plus optional
`--max-bytes`. The digest is required by this handoff protocol, though optional for legacy explicit
packet use. A mismatch refuses before any source read and retains decoded metadata/raw fallback.
No source span is chosen automatically. Consumer revision/blob checks remain authoritative.

## Opt-in evidence summary and handle expansion

V1-0023 adds two opt-in views to the existing `context` verb (decision 0364). No root verb is added.
Without these flags the command prints exactly the packet it printed at base `1894b9e5`
(`TCP-V0-024`).

`corvint [--root PATH] context --task TEXT [--subject PATH] [--limit N] --summary [--summary-bytes N]`
compiles the ordinary packet and then projects its exact default stdout bytes:

- The budget runs from 1024 to 65536 bytes, default 8192, and includes the trailing LF.
- Every top-level member except `results` is kept verbatim. That covers `coverage` (with
  `critical`, `critical_missing` and `unexamined`), `request`, `subject` (with its `evidence_gap`),
  `state`, `revision`, `ok` and `mutates`.
- Each result becomes one compact row: `index`, `kind`, `id`, `critical`, and from its single
  evidence row `authority`, `trust`, `confidence`, `reason`, `line`, `blob_hash`, and
  `evidence_gap` when present. The packet's `action`, `evidence`, `score` and `summary` fields are
  dropped and listed in `summary.row_fields_omitted`.
- Rows stay in packet order, so reserved governing and spec-mentioned rows come first. Rows are
  kept until the next row would exceed the budget.
- A budget that cannot hold the fixed members plus every row through the last critical row refuses
  with `summary-budget` and says how many bytes are needed. It never drops a critical row.
- The `summary` member records:
  - `view` (`experimental-evidence-summary/0`);
  - `packet_sha256` and `packet_bytes` of the full default stdout;
  - `results_total`, `results_shown` and `results_omitted`;
  - `budget_bytes`;
  - `evidence_complete: "UNKNOWN"`;
  - a `continuation` route (rerun without `--summary`; `results[results_shown:]` are the omitted
    rows, and the rerun's sha256 must equal `packet_sha256`);
  - the `expand` usage.

Each row whose path is safe and whose packet has a revision and blob carries the handle
`cv1:TREE:BLOB:RANGE:PATH`. Otherwise the handle is `null`. The handle fields are:

- TREE: the packet's full `revision` tree OID.
- BLOB: 7 to 64 lower-case hex digits.
- RANGE: `all`, or `START-END` with one-based inclusive lines of at most nine digits and
  START ≤ END.
- PATH: repository-relative, and must pass the source-view safe-path rule.

`corvint [--root PATH] context --expand HANDLE [--max-bytes N]` accepts no other context flag.
`--max-bytes` runs from 1 to 1048576, default 65536. The command:

1. Parses the handle. It must be UTF-8 and at most 4352 bytes.
2. Requires the root to be the Git toplevel, and the TREE width to equal the repository's object
   format width.
3. Requires HEAD's tree to equal TREE.
4. Lists TREE at PATH. The entry must be one regular-file blob.
5. Resolves BLOB against the whole object database with `git rev-parse --disambiguate`. The result
   must be exactly one object, and it must equal the tree entry's blob.
6. Reads the blob from Git objects, never from the worktree, and recomputes its object ID.
7. Cuts the LF-delimited line range and requires the selected bytes to be UTF-8 and within
   `--max-bytes`.
8. Rechecks HEAD's tree before printing.

The output is one canonical JSON object with these members:

- `view` (`experimental-evidence-expand/0`);
- `handle`;
- `source`: `object_format`, `tree`, `path`, `blob`, `blob_verified: true`, `bytes`, and `sha256`
  of the whole blob;
- `selection`: `range`, `complete`, `line_start`, `line_end`, `byte_start`, `byte_end`, `bytes`,
  `sha256` of the selected bytes, and `text`.

Every refusal exits 2, prints nothing on stdout, and prints one typed error on stderr.

| Code | Condition |
|---|---|
| `summary-budget` | `--summary-bytes` outside 1024..65536, or the budget cannot hold every critical row |
| `summary-shape` | the packet does not decode, is not a `context` packet, already has `summary`, or a result lacks exactly one evidence row |
| `invalid-handle` | the grammar, hex, width, path or range fails; the range exceeds the blob's lines; the path is not a regular-file blob; or the handle's blob is not the tree's blob at PATH |
| `stale-handle` | HEAD's tree is not the handle's TREE, before the read or at the recheck |
| `missing-handle` | TREE has no entry at PATH, or no object has the BLOB identity |
| `ambiguous-handle` | an abbreviated BLOB names more than one object |
| `source-digest-mismatch` | the read bytes do not recompute the blob object ID |
| `unsupported-text` | the selected bytes are not UTF-8 |
| `expand-budget` | `--max-bytes` is outside 1..1048576, the blob is over 1048576 bytes, or the selection exceeds `--max-bytes` |
| `repository-root` | the root is not its Git toplevel |
| `git-failed` | Git cannot read HEAD's tree or resolve the blob identity |

Non-goals:

- No change to the default packet, `query`, `impact`, the adapters, the hook output or any
  `protocol/**` wire.
- No summarising of source text, no ranking change, and no automatic expansion.
- No re-resolution of a stale handle against current content. A stale or ambiguous handle never
  falls back to HEAD, the worktree or the index.
- No AHI-004 envelope on the expand output, which matches the plain `context` stdout.
- No persisted state. Summary and expansion write nothing under the repository or `.corvint/`.
- No token-cost or task-quality claim before the trial in `ESV-V0-010`.

Failure modes:

- A summary could silently drop a critical row. The view refuses instead (`summary-budget`).
- A consumer could mistake the summary for complete evidence. `evidence_complete` stays `UNKNOWN`,
  and the totals and continuation route are always present.
- A handle could be replayed after HEAD moves. It refuses as `stale-handle`, even when the blob
  still exists.
- An abbreviated blob could collide. It refuses as `ambiguous-handle` rather than being resolved
  through the current tree.
- A hostile handle could use traversal, an absolute path, a leading dash, control bytes, non-UTF-8
  text, oversize input or an overflowing range. It refuses as `invalid-handle` before any Git read.

## Requirements

- `ESV-V0-001`: Bind the bounded canonical packet and exact caller selector to object format,
  expected HEAD commit, packet tree revision, exact path/blob (the read bytes must recompute that blob
  object ID under the object format before extraction, else `stale-blob`), source SHA-256, and zero-based
  half-open byte offsets plus inclusive line bounds. The packet must carry its request object,
  coverage object and subject member. Verify HEAD again before emitting success.
- `ESV-V0-002`: Return one exact contiguous immutable source range as JSON `text` whose UTF-8
  re-encoding reproduces those bytes. Requirement mode returns only a complete plain block under
  the declared grammar. `selector_complete=true` never means task evidence complete; the latter
  remains `UNKNOWN`. Never summarize, project JSON, infer a source or use task-specific facts.
- `ESV-V0-003`: Enforce every frozen bound with sanitized, replacement-disabled, non-lazy Git
  reads using the existing bounded runner (`internal/procgroup`) and owned-descendant cleanup on exit, INT and TERM.
  Read no worktree source and write no repository/index/cache/trace/ledger state, including Python
  bytecode files. Raw fallback also disables pagers and requires the returned sanitized environment.
- `ESV-V0-004`: Refuse malformed input, stale commit/tree/blob, missing path, unsafe/nonregular
  path, ambiguous/duplicate/incomplete requirement, unsupported text, capture/time/count/byte
  overflow with typed errors and no partial view. The Go handoff's requirement and text codes are
  `ambiguous-requirement`, `unsupported-requirement`, `incomplete-boundary` and `unsupported-text`
  (`cmd/corvint/source_handoff.go:441,450,477,481`). Preserve available packet metadata including
  shortages, omissions, coverage and unknown fields. After a packet-size/parse refusal no decoded packet is available; disclose this
  instead of claiming metadata preservation. Supply a raw immutable Git recovery argv
  when a syntactically safe commit/path exists; otherwise declare fallback unavailable and why.
  The Go handoff's stdout JSON is framed by the single `AHI-004` envelope rule
  (`internal/repoenvelope.Frame`): hidden characters become literal `\uXXXX` text that decodes to the
  unchanged JSON value, so `view.text` and every hash/byte-count field keep their exact bytes. When the
  emitted JSON contains the envelope terminator, source-view refuses whole with
  `corvint-envelope-terminator-collision` and emits no view or packet metadata.
- `ESV-V0-005`: Remain opt-in under `experimental-source-view/0`, and leave the bytes of the
  existing public context/query/impact/batch/default command producers unchanged. The only
  producer-side additions are the opt-in `context --summary` and `context --expand` flags
  (`ESV-V0-008`, `ESV-V0-009`). Without those flags the `context` bytes are unchanged, and a golden
  test captured from the base binary proves it. Rollback removes these consumers and their
  registration without migration. The source digests of the native producer files
  (`cmd/corvint/source_handoff.go`, `cmd/corvint/context_summary.go`) are frozen in
  `benchmarks/selfuse-batch/source-views-manifest.json` `currentState.nativeSourceDigests`. A test
  fails when either file changes without re-freezing its digest (amended by decision 0364; the two
  Python supervisor files were deleted in `54735d98`).
- `ESV-V0-006`: Treat focused synthetic correctness checks as development evidence only. The
  coordinator owns the preregistered matched task, blind R1–R5 quality gate, complete lifecycle
  accounting, one repair maximum, canonical gate and post-commit CEM binding. Unrun evaluations
  stay NOT_RUN; incomplete all-worker retry/token coverage leaves the joint target NOT ESTABLISHED.

- `ESV-V0-007`: The opt-in development wrapper captures exactly one automatic canonical packet
  to an explicitly selected external file and emits a bounded directly consumable immutable lead.
  Bind the captured commit/tree and exact packet bytes (the native wrapper executes no plugin script,
  so no script identity exists to bind; decision 0250); preserve
  complete raw metadata, coverage, omitted-selector counts and explicit refusal state. Require the
  lead digest before source expansion and retain caller-selected spans, byte bounds and raw fallback.
  Keep existing producer/default hook bytes and configuration unchanged. Direct deterministic
  hook-output evidence does not establish host model-visible delivery, adoption or task-cost benefit.
- `ESV-V0-008`: `context --summary` prints at most `--summary-bytes` bytes (1024..65536,
  default 8192, LF included) for the exact default packet. It keeps every non-`results` member
  verbatim, keeps each shown row's identity, authority, trust, freshness (`blob_hash` under the
  packet `revision`), `evidence_gap` and a `cv1:` handle, and reports `results_total`,
  `results_shown`, `results_omitted`, the full packet's sha256 and the continuation route. It
  refuses with `summary-budget` rather than omit any row through the last critical row, and it
  keeps `evidence_complete` `UNKNOWN`.
- `ESV-V0-009`: `context --expand HANDLE` returns only the handle's line range, read from Git
  objects at the pinned tree, with the recomputed blob object ID, whole-blob and selection sha256,
  and byte and line offsets, within `--max-bytes` (1..1048576). It refuses with `invalid-handle`,
  `stale-handle`, `missing-handle`, `ambiguous-handle`, `source-digest-mismatch`,
  `unsupported-text` or `expand-budget`, with nothing on stdout. It never substitutes HEAD,
  worktree or index content, and it writes no repository or `.corvint/` state.
- `ESV-V0-010`: The summary and expansion views stay experimental until the matched complete-task
  trial preregistered in `benchmarks/evidence-summary-trial-v0.json` runs and the owner accepts
  it. The trial compares current packets against summary plus expansion with model, effort,
  framing and cache policy fixed. Until then its outcome is NOT_OBSERVED, and no token, cost,
  latency or quality claim is made. These views do not satisfy the V1-0023 release delivery claim.

### Source handoff error codes

The Go source handoff emits the kebab-case codes below (decision 0100). Each row cites the first
emitting site and states only the condition checked there.

| Code | First emitting site | At the cited site |
|---|---|---|
| `invalid-source-handoff-arguments` | `cmd/corvint/source_handoff.go:62` | the argument list does not contain exactly `--packet-out` followed by one output path |
| `no-safe-immutable-handle` | `cmd/corvint/source_handoff.go:275` | no safe immutable fallback handle has been established, so the fallback argv is `nil` |
| `raw-immutable-source` | `cmd/corvint/source_handoff.go:315` | a safe path and resolved repository root permit a raw immutable `git show` fallback argv |
| `view-budget` | `cmd/corvint/source_handoff.go:276@37ae3a0a` | the source view byte budget is below 1 or above 16384 |

## Native implementation status

`54735d98` (2026-09-11) replaced the Python consumer, wrapper and their unittest suites with the
native commands and `cmd/corvint/source_handoff_test.go`. Decision 0250 closed the other port gaps
by implementation or amendment.

The ESV-V0-005 gap is resolved by decision 0364 (V1-0023, 2026-09-23). The manifest's historical
Python digests stay `STALE_HISTORICAL`. `currentState.nativeSourceDigests` now freezes the sha256 of
`cmd/corvint/source_handoff.go` and `cmd/corvint/context_summary.go`.
`TestSourceViewNativeSourceDigestsAreFrozen` recomputes both digests. `cmd/corvint/host_adapter.go`
is excluded because it also carries unrelated adapter surfaces; the `SourceHandoff` tests cover its
dispatch.

## Acceptance and rollback

| Requirement | Implementation | Evidence |
|---|---|---|
| ESV-V0-001 | `executeSourceView`/`selectSourceHandle` packet and identity boundary | `TestHostAdapterSourceViewPacketAndIdentityRefusals` stale tree/blob and missing path; `TestHostAdapterSourceViewSafeguards` stale commit; `TestSourceViewReverifiesHeadBeforeSuccess` HEAD moved during the read; `TestSourceViewRequiresPacketRequestCoverageAndSubject`; `TestSourceViewRequiresGitToplevelRoot` subdirectory root; `TestSourceViewBindsObjectFormatAndOIDWidth`; `TestSourceViewVerifiesBlobObjectDigest` substituted blob bytes |
| ESV-V0-002 | `extractSourceView` span selection | `TestHostAdapterSourceViewSafeguards` exact CRLF/UTF-8 requirement block; `TestExtractSourceViewRefusesTextItCannotEmitExactly`; `TestSourceViewEnvelopeEscapesHiddenCharacters`; `TestSourceViewRequirementPrefixGrammar` frontmatter, fence and HTML prefix scan |
| ESV-V0-003 | `sourceGit` sanitized bounded Git reads through `procgroup.Run` under the 10-second context; pager-disabled raw fallback with its environment | `TestHostAdapterSourceViewSafeguards` view budget; `TestHostAdapterSourceViewPacketAndIdentityRefusals` oversized packet; `TestClaudeSourceHandoffCLI` repository bytes unchanged; `TestSourceViewCancellationTerminatesGitProcessGroup` TERM and descendant reap; `TestSourceViewRawFallbackDisablesPagerWithSanitizedEnvironment`; `TestSourceViewBoundsArgumentBytes`; `TestSourceViewRefusesSymlinkedPacket`; `TestSourceViewRefusesFIFOPacket`; `TestSourceViewStaysWithinGitCommandBudget` |
| ESV-V0-004 | `runSourceViewAdapter` typed refusal envelope | `TestHostAdapterSourceViewPacketAndIdentityRefusals` unsafe paths without fallback, malformed packets; `TestHostAdapterSourceViewSafeguards` digest mismatch keeps metadata and fallback; `TestSourceViewRefusesEnvelopeTerminatorCollision` |
| ESV-V0-005 | opt-in `adapter source-view`, `adapter claude-source-handoff` and `context --summary`/`--expand` dispatch only | `TestContextDefaultWireIsTheGolden` default context bytes equal the base-binary golden; `TestSourceViewNativeSourceDigestsAreFrozen` native source digests |
| ESV-V0-006 | coordinator experiment | NOT_RUN; no whole-task performance claim |
| ESV-V0-007 | `runClaudeSourceHandoff`/`captureSourceHandoff` explicit captured-packet seam and `executeSourceView` digest check | `TestClaudeSourceHandoffCLI` adapter capture, lead digest and source-view consumption; `TestHostAdapterSourceHandoffPublicationRefusals` in-repository and existing output refusals; plugin script identity amended away (decision 0250) |
| ESV-V0-008 | `summarizeContextPacket` in `cmd/corvint/context_summary.go` | `TestContextSummaryKeepsIdentityCoverageAndCriticalRows` budget sweep, verbatim members, row identities and handles; `TestContextSummaryTruncationReportsTotalsAndRefusesToDropCriticalRows` totals, continuation and critical-row refusal; `TestContextSummaryAndExpandAreReadOnly` |
| ESV-V0-009 | `expandContextHandle`/`parseContextHandle` in `cmd/corvint/context_summary.go` | `TestContextExpandReturnsExactPinnedBytes` exact bytes with a dirty worktree; `TestContextExpandRefusesWithoutSubstitutingContent` invalid, hostile, missing, non-UTF-8 and budget cases; `TestContextExpandRefusesAStaleHandleAfterHeadMoves`; `TestContextExpandRefusesAnAmbiguousAbbreviatedBlob`; `TestContextSummaryAndExpandAreReadOnly` no `.corvint/` write |
| ESV-V0-010 | preregistration `benchmarks/evidence-summary-trial-v0.json` | NOT_OBSERVED; no trial run; no owner acceptance |

Implementation sequence: freeze this spec and independent read-only plan review; implement only
consumer/tests; focused identity/extraction/refusal/lifecycle checks; fresh independent review;
local commit and source-digest handoff. Initial dogfood uses base
`40c5358a63021c6e57cbab07cb9c4c2f7df014d6`; coordinator performs final binding and one full gate.
Failure to preserve bytes or uncertainty kills this prototype. A negative matched raw screen may
retire the intervention; a positive one permits a preregistered repeat, not product promotion.
Human acceptance, general Markdown parsing, additional selectors and public integration remain out
of scope. Remove `cmd/corvint/source_handoff.go`, its tests, the two `cmd/corvint/host_adapter.go` dispatch
entries and their help text to roll back; the caller deletes explicitly captured external packets. There is no default saved consumer state or migration.
To roll back the V1-0023 views to current packets, do the following. There is no persisted state,
cache or migration, and saved handles simply stop resolving.

- Delete `cmd/corvint/context_summary.go`, `cmd/corvint/context_summary_test.go` and
  `cmd/corvint/testdata/context-default-wire.golden`.
- Remove the view fields, flag cases, `checkContextViewArguments`, the two dispatch branches and
  the help paragraph from `cmd/corvint/taskcontext.go`, restoring the `--task` required check.
- Drop `cmd/corvint/context_summary.go` from the manifest's `implementation` and
  `nativeSourceDigests`.


## Development screen outcome — 2026-09-06

One fresh same-task source-view availability screen ran at source commit `6800bb55`. Neither
investigator invoked the consumer or explicit Corvint operations, and both missed three frozen
critical audit criteria. The treatment was slower and reported more investigator token units.
The initial pair is retained as unsuccessful; it neither measures a consumed-view effect nor
establishes the joint 50% target. Planned Fable reviewers were replaced by blinded Codex reviewers
under the user's cost constraint; their usage and complete denominators remain unknown.
The automatic-context-to-saved-packet handoff and resolved-hook identity require attention before
another attributable screen. See `docs/evidence/optimization-loop6-2026-09-06.md`.
