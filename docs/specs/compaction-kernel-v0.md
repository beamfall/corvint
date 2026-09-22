# Compaction Kernel V0

Owner: Russell Lewis
Frozen: 2026-09-11
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `AGENTS.md`, `ROADMAP.md`,
`docs/specs/local-completion-protocol-v0.md`, `docs/specs/task-context-packet-v0.md`,
`docs/PRODUCT.md`, `docs/DOGFOOD.md`, `LICENSE`, and `LICENSING.md`

## Agent digest
- Claim: A bounded, pinned kernel block makes "governing requirements survived compaction" a checkable property rather than a hope.
- Status: proposed/experimental
- Exists: `internal/compactionkernel`, `cmd/corvint/kernel.go`, the opt-in adapter injection (`CKN-V0-009`), and their requirement-traced Go tests.
- Blocked on: a frozen compaction-survival case set, and the `PreCompact`/`SessionStart(compact)` verify hook configuration.
- Read next: User and measurable job; Requirements; Traceability.

## User and measurable job

ROADMAP.md states the owner target directly: governing requirements and unresolved obligations must
survive context reduction and compaction. Today nothing checks that. A host compacts a session, the
summary keeps whatever the summarizer thought mattered, and the agent continues with no way to tell
whether `AGENTS.md` still governs it or whether the authority it remembers is the authority the
repository now has.

The job is one small artifact that (a) states what governs, pinned to immutable Git content, (b) is
cheap enough that no compaction budget can justify dropping it, and (c) can be found again in
arbitrary surviving text and checked against the current repository. The outcome is a verdict:
`intact`, `stale`, `missing`, or `corrupt`, with the specific mismatching paths named.

It is useful when a compaction that dropped or aged the governing authority is detected instead of
silently tolerated. It is killed if hosts give no compaction signal at all and the kernel therefore
never observes the transition it exists to check.

## Truth boundary

The kernel is a pinned pointer set, not a copy. It carries paths, blob hashes, requirement ids and a
digest; it carries no source content, so an `intact` verdict proves that the named blobs are still
the blobs the index reads, never that the agent read or obeyed them. `missing` is evidence that this
transcript no longer carries the kernel; it is not evidence that the authority changed. A host that
emits no compaction signal degrades to session-start injection alone, and the absence of a verify
call is reported as `NOT_RUN`, never as survival.

## Requirements

- **CKN-V0-001:** `Build` MUST compile a kernel from an immutable index, pinning each governing
  authority to the blob hash that index read, and MUST refuse (`unsupported-kernel-authority`) an
  authority the index did not pin or an empty governance array. A non-immutable index MUST be
  refused with `invalid-kernel-index`. The kernel MUST record the index revision.

- **CKN-V0-002:** The canonical JSON document, envelope excluded, MUST be at most 600 bytes. When a
  requested requirement list does not fit, rows MUST be dropped from the tail and the dropped count
  recorded in `omitted`; when the authorities alone exceed the bound the build MUST refuse with
  `unsupported-kernel-budget` rather than emit an over-budget kernel.

- **CKN-V0-003:** Requirement ids MUST resolve through the pinned `docs/specs/REQUIREMENTS.tsv`
  blob and the pinned blob of the spec that row names, never the worktree. An id absent from the
  pinned table, or a spec absent from the indexed revision, MUST be refused with
  `unsupported-kernel-requirement`; the kernel never substitutes a worktree reading for a pinned one.
  A repeated id is one requirement and MUST be pinned once.

- **CKN-V0-004:** `digest` MUST be the SHA-256 of the canonical JSON of every other member and MUST
  be deterministic for equal inputs. Any change to the authorities, requirements, revision or
  omitted count MUST change the digest.

- **CKN-V0-005:** `Render` MUST emit exactly one fenced envelope: the fence, `BEGIN CORVINT KERNEL`,
  the canonical JSON on one line, `END CORVINT KERNEL`, the closing fence. `InjectionBlock` MUST
  produce that same block for the session-start and user-prompt adapter paths, with no requirement
  list.

- **CKN-V0-006:** `Verify` MUST locate the envelope inside arbitrary text, including a
  post-compaction summary with prose before and after it, and MUST report exactly one of:
  `missing` (no `BEGIN CORVINT KERNEL` marker), `corrupt` (unterminated envelope, an envelope whose
  body differs from an earlier one in the same text, non-canonical or
  trailing JSON, absent digest, a digest that does not cover the body, or a body `Build` could not
  emit: a foreign `profile`, a `revision` that is not an object id, no governing authority, a pin
  row without a path and object id, an `omitted` that is not a non-negative decimal integer, or a
  requirement `line` that is not a positive decimal integer; `Build` likewise pins no requirement
  row whose line is not positive),
  `stale` (digest verifies but a pinned blob hash no longer matches the current index, or its path
  is no longer tracked), or `intact`. A `stale` verdict MUST name each mismatching path with its
  expected and actual blob hash.

- **CKN-V0-007:** Every operation MUST be read-only, per AGENTS.md invariant 4: no index, trace,
  cache or ledger write, no process spawn beyond the index build the host verb already performs, and
  no network. The kernel MUST carry no source content.

- **CKN-V0-008:** `corvint [--root PATH] kernel [--requirements ID,...]` MUST print the rendered
  block and exit 0. `corvint [--root PATH] kernel verify` MUST read text from stdin, print the
  verdict as one canonical JSON line, and exit 0 only on `intact`; every other state MUST exit
  non-zero. Verify stdin is limited to 131,072 bytes (`gokernel.MaxInputBytes`); a longer input MUST
  be refused with `invalid-kernel-input`, exit 2, before recovery. Unrecognized arguments, and
  `--requirements` given to `verify`, MUST be refused with `invalid-arguments` before any repository
  read.

- **CKN-V0-009:** `corvint host-adapter codex` and `corvint host-adapter claude-code` MUST append
  the kernel to the `session-start` and `user-prompt` `additionalContext` only when the environment
  variable `CORVINT_EXPERIMENTAL_COMPACTION_KERNEL` is exactly `1`; with it unset or any other value
  the adapter output MUST be unchanged, and no other event carries the kernel. The block MUST be
  `InjectionBlock` over the fresh index snapshot and the governance the `kernel` verb selects, so it
  equals the verb's output for that index, and it MUST reach the host only inside
  `repoenvelope.Frame` (`AHI-004`), after the receipt envelope and behind the trusted label
  `Corvint experimental compaction kernel (CKN-V0 proposed, not delivered):`. The adapter MUST NOT
  build an index for it: a missing or unusable snapshot, a refused kernel build, a 250 ms deadline, or a
  terminator collision replaces the block with the fixed trusted line
  `Corvint experimental compaction kernel NOT_RUN: <code>`, carrying only an error code. The framed
  block's escaped bytes MUST be reserved from the event budget before the receipt is compiled, and a
  degraded adapter output carries neither block nor line.

## Host integration

Claude Code's `PreCompact` hook calls `corvint kernel` and carries the block into the material
compaction preserves. The following `SessionStart` hook with source `compact` pipes the surviving
text into `corvint kernel verify` and surfaces the verdict; that hook configuration is not shipped.
Under the operator's opt-in (`CKN-V0-009`), the session-start and user-prompt adapter paths call
`compactionkernel.InjectionBlock` so the kernel is present before any compaction occurs. The opt-in
exists to collect the frozen case set; it is not promotion. No host-specific compaction API is required: on a host with no compaction signal,
only the injection runs.

## Failure modes and degradation

| Failure | Behavior |
|---|---|
| Host emits no compaction signal | Session-start injection only; survival is `NOT_RUN`, never `intact` |
| Opt-in set but no fresh index snapshot | `NOT_RUN: index-snapshot-unavailable` line; no index build in the hook |
| Opt-in kernel work pushes the hook past its host kill | The adapter watchdog of `AHI-017` (`agent-harness-integration-v0.md`) emits `adapter-host-kill-deadline` at the declared kill less the process reserve and coding continues; only a process start slower than that reserve still lets the host drop the output. Unset the variable to remove the kernel work |
| Governance or block refused at injection | `NOT_RUN: <code>` line; no invented kernel (invariant 2) |
| Summary dropped the block | `missing`, exit non-zero; the agent re-injects from a fresh `kernel` call |
| Summarizer edited the JSON | `corrupt`; the recovered body is never trusted as evidence |
| Summarizer re-spaced the JSON, or a self-digested body pins no authority | `corrupt`; a matching digest over decoded members is not enough to verify |
| Authority blob changed since the kernel was built | `stale` with the path, expected and actual hash |
| Pinned path absent from the compared index | `stale` with an empty actual hash and reason `authority-unavailable` or `requirement-unavailable` |
| Requirement id absent from the pinned table | Build refuses; no invented citation (invariant 2) |
| Digest collision | Not defended by construction; SHA-256 preimage resistance is the assumption, and the pinned blob check is an independent second gate |

### Kernel error codes

The compaction kernel emits the kebab-case codes below (decision 0100). Each row cites the first
emitting site and quotes the message returned there or states the condition checked there, which is
the whole of what the row asserts.

| Code | First emitting site | At the cited site |
|---|---|---|
| `authority-blob-changed` | `internal/compactionkernel/kernel.go:399@36487f20` | the `reason` of a mismatch when a pinned `authorities` path is tracked in the compared index with a blob hash other than the pinned one |
| `invalid-kernel-input` | `cmd/corvint/kernel.go:131` | "cannot read kernel text from stdin" when `kernel verify` cannot read stdin, or "kernel input exceeds its byte limit" when stdin exceeds 131,072 bytes; exit 2 |
| `requirement-blob-changed` | `internal/compactionkernel/kernel.go:400@03dcb538` | the `reason` of a mismatch when a pinned `requirements` path is tracked in the compared index with a blob hash other than the pinned one |
| `unsupported-kernel-envelope` | `internal/compactionkernel/kernel.go:384` | "kernel envelope is not one canonical JSON object" when the envelope body does not decode as one JSON object, or "kernel envelope carries trailing content" when content follows it; `Verify` keeps only the message, as a `corrupt` verdict |

## Non-goals

No daemon, resident process, watcher, or background task. No dependency on a host-specific
compaction API, transcript format, or summarizer behavior. The kernel carries no source content, no
task text, no history, and no learned state. It is not a context packet, not a replacement for
TaskContext, and grants no authority of its own: it points at the project-owned authority that
already outranks it (invariant 3).

## Rollback

The package and verb are additive and read-only. Rollback is removing the `kernel` dispatch line,
`cmd/corvint/host_adapter_experimental.go` with its two `experimentalKernelContext` call sites, and
the hook configuration; no stored state, index format, or wire contract of any other verb changes,
and no repository file is written at any point, so a partial rollback leaves nothing to repair.

## Promotion and kill criteria

Promotion out of experimental requires zero governing-requirement loss on the frozen
compaction-survival case set: every case whose transcript retained the block verifies `intact` or
names the exact stale path, and every case whose transcript dropped it verifies `missing` rather
than passing. Structural presence of the block is not itself promotion evidence.

Kill criterion: hosts give no compaction signal that can carry or recover the block, so the verify
half never runs in registered dogfood.

## Traceability

| Requirement group | Delivery | First authoritative evidence |
|---|---:|---|
| `CKN-V0-001` authority pinning | experimental | `internal/compactionkernel`: `TestBuildPinsGoverningAuthority` |
| `CKN-V0-002` byte bound | experimental | `TestKernelStaysUnderByteBound` |
| `CKN-V0-003` pinned requirement resolution | experimental | `TestRequirementsResolveFromPinnedBlob` |
| `CKN-V0-004` digest | experimental | `TestDigestIsDeterministicAndCovering` |
| `CKN-V0-005..006` envelope and verdicts | experimental | `TestVerifyReportsEachState`, `TestVerifyRefusesDisagreeingEnvelopes`, `TestVerifyRefusesNumbersBuildCannotEmit` |
| `CKN-V0-007` read-only and content-free | experimental | `TestKernelCarriesNoSourceContent`; process/network spy `TestPackageHasNoProcessOrNetworkImports` (`go test ./internal/compactionkernel/... -run TestPackageHasNoProcessOrNetworkImports -count=1 -timeout 30m`, PASS, 2026-09-13); `cmd/corvint`: `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` (CLI-level repository-byte assertion) |
| `CKN-V0-008` verb contract | experimental | `cmd/corvint`: `TestKernelVerbRendersAndVerifies`, `TestKernelVerifyRefusesOversizedInput`, `TestKernelInvocationParsing` |
| `CKN-V0-009` opt-in adapter injection | experimental | `cmd/corvint`: `TestExperimentalKernelAdapterContext` |

## Unresolved decisions

- Whether the user-prompt path should re-inject the kernel on every prompt or only when the previous
  verdict was not `intact`.
- Whether `stale` should distinguish "authority edited" from "authority moved", which currently both
  surface as a named path mismatch.
