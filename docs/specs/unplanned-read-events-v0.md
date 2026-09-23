# Unplanned Read Events V0

Owner: Russell Lewis
Frozen: 2026-09-11
Requirement prefix: `URE-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `AGENTS.md` invariants 4 and 7, `docs/specs/self-observation-ledger-v0.md`
(the bounds this ledger copies), `docs/specs/task-context-packet-v0.md` (the packet whose paths
define "planned"), `docs/PRODUCT.md`, `docs/DOGFOOD.md`

## Agent digest
- Claim: an explicitly opt-in local ledger counts reads Corvint failed to prevent, so packet quality has a number instead of an impression.
- Status: proposed/experimental
- Exists: `internal/unplannedread`, `corvint reads`, `corvint reads enable|disable`, and the marker-gated Claude Code adapter call sites (`URE-V0-008`, `URE-V0-009`).
- Blocked on: an enabled-ledger sample of at least ten registered sessions for the gate below.
- Read next: Requirements; Non-goals and authority; Proposed invariant 4 amendment; Failure modes.

## User and measurable job

An agent receives a context packet and then reads anyway. Every such read is a file Corvint should
have carried and did not: it costs a tool round trip, tokens, and the agent's attention. The
owner's complete-cost target depends on that count, and today nothing measures it. The job is one
number per session — the share of read-shaped tool calls whose target the delivered packet did not
already contain — plus the paths that cost the most bytes, so packet construction has a target to
move. It is useful when a packet change moves the share. It is killed when the share is dominated
by legitimate exploration that no packet could have anticipated.

## Requirements

- **URE-V0-001.** `Classify(packetPaths, toolName, toolInput, root)` MUST return an `Event` for a
  read-shaped tool call whose target resolves strictly inside the worktree, and `false` otherwise.
  Resolve the root, including symlink roots and platform aliases. For an absent target, resolve and
  contain the nearest existing ancestor before appending suffix components; each component MUST
  differ from `..` and its appended path MUST return `ENOENT` from `Lstat`. A dangling symlink exists
  and its target MUST satisfy the same containment rule, with at most 40 dangling-link expansions.
  Escapes, unresolved roots, exhausted link bounds, and other filesystem errors MUST abstain.
  Each existing component of the contained path MUST take the name its directory stores: the exact
  entry, else a case-folded entry naming the same file, so a case-insensitive volume never splits one
  file across spellings (decision 0185); an existing component matched by no stored name, such as a
  Unicode normalization alias, MUST abstain, and a verified-absent suffix keeps its spelling. A target
  equal to a packet path, or under a packet path treated as a directory prefix, is `planned`; a
  planned row carries a tool, timestamp and packet digest but no path and no byte count. An
  unplanned row carries the worktree-relative slash path and the file's size, or `0` with
  `size_known=false` when the path is unreadable or a directory.
- **URE-V0-002.** The recognized tools are `Read` (`file_path`), `Grep` and `Glob` (`path`), and
  `Bash` under one bounded heuristic: the command is split on `|`, `;`, `&` and newline into at
  most eight segments of at most 32 fields; the first segment whose command basename is `cat`,
  `sed`, `head` or `tail` contributes its first file operand, where leading-`-` fields are flags,
  purely numeric fields (optionally `+`-prefixed, as in `tail -n +5`) are flag values, and `sed`'s
  first non-flag operand is its script. That segment MUST abstain when any field holds an odd
  count of `'` or `"` (the split cut a quoted word), when `sed` supplies its script through an
  `e` or `f` short flag, `--expression` or `--file`, or when the operand, after one wrapping quote
  pair, still contains shell syntax (`<>$*?[]{}()~`, backslash, backtick or a quote). A segment
  MUST also abstain when the segments before it hold an odd count of `'` or `"` (the break fell
  inside a quoted word), contain `<<` (later lines are a heredoc body), or begin with `cd`, `pushd`
  or `popd` after optional `(` or `{` (relative operands no longer resolve from the worktree root);
  the whole command MUST abstain when any segment so begins, since the host's reported working
  directory already reflects a later change. `HookPostTool` and `HookPostToolSession` resolve a
  relative target from the payload's absolute `cwd`, which the host moves after a `cd`, and from
  the root only when no absolute `cwd` is present; `Classify` resolves from the root.
  A Bash operand that does not name an existing non-directory file MUST abstain: the split cannot
  tell such an operand from a flag value or script (`head -c 1k`, `sed -i ''`). The
  heuristic is deliberately incomplete: a missed read under-counts, it never invents one.
- **URE-V0-003.** `Append(root, event)` MUST be a no-op returning `nil` unless the opt-in marker
  `.corvint/unplanned-reads.enabled` exists. A default install therefore gains no file and no write
  from any Corvint command. `Enable` and every append MUST refuse without writing unless one ignore
  file, the root `.gitignore` or `.corvint/.gitignore`, covers the marker, the ledger and its
  `.unplanned-reads.*` temporaries under the self-observation ledger's conservative rule
  (`SOL-V0-001`), so a marker a repository commits never enables a ledger git would track.
- **URE-V0-004.** When enabled, `Append` and `RecordPacket` MUST write one JSON row per line to
  `.corvint/unplanned-reads.jsonl` under the self-observation ledger's own bounds: at most 2048 bytes
  per row, at most 128 KiB per file, oldest-first truncation to half the cap on overflow, atomic
  temporary-and-rename replacement, and secret screening of the stored path. An append or `Enable`
  MUST fail without writing when `.corvint` is not a real directory or the ledger exists and is not a
  regular file, so a symlink never moves a write or a previous-row read outside the worktree; `Read`
  and packet lookup MUST open the ledger only under the same condition, verified against the opened
  file. When an append repairs
  an externally oversized ledger, it MUST read at most half the file cap from the ledger's tail and
  align at the next row boundary exactly as oldest-first truncation does. A read row stores only
  timestamp, tool name, path, byte count, size-known flag, packet digest and the planned flag. A
  packet row (`URE-V0-008`) stores only timestamp, `kind: "packet"`, a 16-hex SHA-256 prefix of the
  session key, the packet digest, and a 16-hex SHA-256 prefix of each cleaned packet path, or, for a
  refused packet (`URE-V0-008`), `paths_sha256: null` and `refused: true`. An error
  row (`URE-V0-006`) stores only timestamp, `kind: "error"`, and one closed `reason` code:
  `row-exceeds-bound`, `planned-row-path` or `write-failed`. Never file contents, task prose, packet
  paths, session identifiers, error text, or absolute paths.
- **URE-V0-005.** `Read(root)` MUST be read-only and return the count of unplanned and planned
  rows, total unplanned bytes, per-tool counts, paths ranked by bytes, the oversize rows skipped,
  and, from the retained error rows alone, the newest dropped hook error's reason code and the count
  of retained error rows; no process-local state contributes. Packet rows, error rows, and rows of
  any other non-empty `kind` are not reads and are never counted. `Ratio` is unplanned over unplanned-plus-planned to three
  decimals, or `n/a` when nothing is recorded. A malformed row is skipped, never guessed at. A
  ledger refused by `URE-V0-004`'s directory and regular-file condition is an error, never a digest.
- **URE-V0-006.** `HookPostTool(root, packetPaths, payload)` MUST return `nil` on every path,
  enabled or not, for every payload shape. An append failure of a read or packet row is dropped and
  persisted best-effort as one error row, so a digest rendered by another process reports it as
  `LAST-ERROR reason=<code> count=<n>`; when the ledger itself cannot be written the error row is
  dropped too. The hook can never fail a host tool call.
- **URE-V0-007.** `corvint [--root PATH] reads [--limit N]` MUST print the digest and write
  nothing, mirroring `corvint observations`' argument parsing, `--limit` bounds (1..120) and
  `emitError` status-2 emission. `corvint [--root PATH] reads enable|disable` creates or removes
  the marker and is the only mutation in the verb, declared as such in help. `--limit` belongs to
  the digest alone, so it MUST be refused beside `enable` or `disable` in either order with
  `unrecognized arguments: <token>` (decision 0205).
- **URE-V0-008.** After a Claude Code `user-prompt` event renders a delivered context,
  `corvint host-adapter claude-code user-prompt` MUST call `RecordPacket(root, sessionIdSha256,
  paths)`, where `paths` is the union of the `path` members of the delivered packet's `governance`,
  `declared_scope` and `task_evidence` rows. `RecordPacket` is a no-op without the marker, records
  nothing for an empty session key, and is log-and-drop like `HookPostTool`. A degraded
  `user-prompt` output, including a refused adapter input or event whose session identity is still
  valid, delivered no packet and MUST record, through `RefusePacket(root, sessionIdSha256)`, a
  refused packet row for that session. A packet row that cannot be written (over-bound or
  failed) is refused: `RecordPacket` persists the error row and then, best-effort, a refused packet
  row for the session, so later reads in that session abstain rather than score against a truncated
  planned set or the session's older packet.
- **URE-V0-009.** `corvint host-adapter claude-code post-tool` MUST call
  `HookPostToolSession(root, sessionIdSha256, payload)` before its existing harness event, and the
  call MUST NOT change that event's output. The hook judges the read against the newest retained
  packet row whose session prefix matches, treating a packet path as a directory prefix exactly as
  `URE-V0-001` does; a newest matching row marked `refused` MUST abstain, as MUST a lookup refused by
  `URE-V0-004`'s ledger condition. Packet lookup MUST read at most the ledger's 128 KiB file cap from its tail
  and discard the leading boundary fragment. When no complete matching row exists in that bounded
  tail (never recorded, another session's, truncated away, or outside an externally oversized
  ledger's tail) it MUST abstain and write nothing: a read with no known packet is not an unplanned
  read (invariant 2). It returns `nil` on every path. Codex exposes no post-tool hook event to this
  adapter, so no Codex call site exists.

## Non-goals and authority

This ledger is private derived state. It is never an input to ranking, retrieval, evidence
selection, authority, or any receipt, nor to learning except through the operator-invoked
`corvint eval --learn-slot-weights` step, whose output reaches ranking only after the frozen held-out
gate admits it (`LTA-V0-009` to `LTA-V0-012`; decision 0368, ratified by decision 0373). No `context`
or `query` result changes because the ledger exists, is absent, is corrupt, or is deleted; only an
admitted slot-weights file, written by that step, changes a `context` packet, and it discloses its
digest. It stores no file contents, no command text, no task prose, and
no packet path list — only a digest naming the packet. It is not telemetry: nothing leaves the
worktree. It does not block, delay, or fail any tool call, and it makes no claim that an unplanned
read was a mistake; it counts, and a human reads the count.

The ledger ships disabled: without the operator-created marker the two adapter call sites stat one
file and write nothing, so the default adapter output and worktree are unchanged.

## Invariant 4 amendment

Ratified by `docs/decisions/0099-experimental-adapter-call-sites-2026-09-12.md` and applied to
`AGENTS.md` invariant 4, after its `SOL-V0-007` sentence, as exactly:

> A second private ledger `.corvint/unplanned-reads.jsonl` is written only while the operator-created
> marker `.corvint/unplanned-reads.enabled` exists; like the self-observation ledger it is bounded,
> local, derived state and never an input to ranking, learning, evidence, or authority.

Decision 0373 (item 12, ratifying decision 0368) replaced the final clause of that sentence in
`AGENTS.md` invariant 4 with the current wording: never an input to ranking, evidence or authority,
nor to learning except through the operator-invoked `corvint eval --learn-slot-weights` step gated
by `LTA-V0-009` to `LTA-V0-012`. The non-goals above carry the same clause.

## Failure modes

| Failure | Required response | Recovery |
|---|---|---|
| marker absent | every write path is a silent no-op | operator runs `reads enable` |
| row over 2048 bytes | row refused, `row-exceeds-bound` error row appended | none needed; the read is under-counted |
| file at the 128 KiB cap | oldest rows truncated to half the cap | digest counts only retained rows and says so |
| malformed or partial row | skipped by the reader | none needed |
| target size unavailable after containment succeeds | `size_known=false`, bytes `0` | none needed |
| target containment unresolved or escaping | read abstains, nothing written | none needed |
| existing path component matched by no stored directory entry | read abstains, nothing written | none needed; the read is under-counted |
| the marker, ledger or temporaries are not safely gitignored | `Enable` and every append refuse and write nothing; the hook drops the failure | add the `.gitignore` entries |
| no packet row for the session at post-tool | read abstains, nothing written | none needed; the read is under-counted |
| packet row over 2048 bytes or unwritable | row refused, error row and refused packet row appended; the session's reads abstain | none needed |
| `.corvint` or the ledger is a symlink or other non-regular entry | append and `Enable` fail and write nothing; `Read` fails; packet lookup abstains; the hook drops the failure | replace the link with a real directory or file |
| Bash operand is not an existing file, follows an unclosed quote or heredoc, or shares its command with a directory change | read abstains, nothing written | none needed; the read is under-counted |
| packet row truncated away at the cap | later reads in that session abstain until the next prompt | none needed |
| degraded user-prompt output | refused packet row recorded; the session's reads abstain until the next delivered prompt | none needed |
| a `corvint` built before `kind`-tagged rows were skipped reads a newer ledger | each packet or error row decodes as a zero-byte, empty-tool unplanned read and is miscounted; accepted, because the slice is experimental, no tagged release contains the ledger, and rows carry no version marker to gate on | read with a current binary, or delete the ledger (Rollback) |
| share dominated by exploration | the metric is not measuring packet quality | kill the slice per the criteria below |

## Gate and kill criteria

Keep the slice only if a deliberate packet-construction change moves the recorded unplanned share
in the expected direction across at least ten registered sessions, with the packet digest showing
the change actually reached the measured sessions. Kill it if the share does not respond to packet
quality, or if manual review of the top-ranked paths finds that most unplanned reads are
legitimate exploration no packet could have anticipated. A moved share is engineering evidence, not
a product, interoperability, or benchmark claim.

## Rollback

Delete `.corvint/unplanned-reads.enabled` and `.corvint/unplanned-reads.jsonl`. No index, trace,
receipt, cache, or repository state refers to either file, so nothing migrates and no result
changes.
Removing the adapter call sites deletes `recordDeliveredPacket` and the `HookPostToolSession` call
from `cmd/corvint/host_adapter.go`; no other output depends on them.

## Traceability

| Requirement | Delivery | First authoritative evidence |
|---|---:|---|
| `URE-V0-001` | experimental | `TestClassifyPlannedAndUnplanned`, `TestClassifyAbsentPathContainment`, `TestRelativeAliasesAndUncertainty`, `TestClassifyStoredSpelling` |
| `URE-V0-002` | experimental | `TestBashHeuristic`, `TestBashHeuristicNeverInventsPath`, `TestBashLaterSegmentContext`, `TestHookResolvesRelativeOperandFromPayloadCwd` |
| `URE-V0-003` | experimental | `TestAppendDisabledIsNoOp`, `TestLedgerRequiresIgnoreEntries` |
| `URE-V0-004` | experimental | `TestAppendEnabledRotatesAtCap`, `TestSymlinkedLedgerIsRefused`, `TestLedgerRequiresIgnoreEntries` |
| `URE-V0-005` | experimental | `TestDigestMath`, `TestSymlinkedLedgerIsRefused`, `TestDroppedHookErrorSurfacesAcrossProcesses` |
| `URE-V0-006` | experimental | `TestHookPostToolNeverErrors`, `TestDroppedHookErrorSurfacesAcrossProcesses` |
| `URE-V0-007` | experimental | `TestRunReadsDigestAndToggle`, `TestRunReadsRefusesLimitBesideAToggleInEitherOrder` |
| `URE-V0-008` | experimental | `TestSessionPacketDenominator`, `TestRefusedPacketAbstains`; `cmd/corvint`: `TestClaudeAdapterUnplannedReadCallSites` |
| `URE-V0-009` | experimental | `TestSessionPacketDenominator`; `TestSymlinkedLedgerIsRefused`; `TestNewestPacketBoundsOversizeLedger`; `cmd/corvint`: `TestClaudeAdapterUnplannedReadCallSites` |

## Unresolved decisions

- Whether planned rows are worth their storage, or whether the denominator should come from the
  host's own tool-call count.
