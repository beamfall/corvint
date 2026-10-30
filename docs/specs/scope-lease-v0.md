# Scope Lease V0

Owner: Russell Lewis
Frozen: 2026-09-11
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `AGENTS.md` (invariant 4), `docs/DOGFOOD.md`, `docs/PRODUCT.md`,
`docs/agent-memory/fixes.md` (2026-09-08 interleaved-commit entry), `LICENSE`, and `LICENSING.md`

## Agent digest
- Claim: `corvint lease` records bounded local scope claims so concurrent agents on one worktree refuse overlapping file or ticket scope before they edit.
- Status: proposed/experimental
- Exists: `cmd/corvint/lease.go`, `internal/scopelease`, and per-requirement Go tests.
- Blocked on: a CEM or dogfood consumer for `scopelease.Check`, and measured evidence that leases remove interleaved-commit gaps.
- Read next: User and measurable job; Requirements; Failure modes; Traceability.

## User and measurable job

Several agents work one Corvint worktree at once. `.corvint/change.cem.json` is a single tracked path and
`make dogfood-change` pins the target to `HEAD`, so two sessions committing to one worktree serialise
by each taking the other's `HEAD` as its base, and the commits in between fall in a gap no CEM binds.
The observed failure is recorded in `docs/agent-memory/fixes.md` (2026-09-08).

A scope lease is the cheap upstream half of that problem: before a session starts editing, it claims
the path globs and the ticket it intends to own, and a second session claiming overlapping scope is
refused with a report naming the live holder. The measurable job is that two concurrent sessions
cannot both believe they own `internal/x/**` or ticket `AT-42` without one of them being told.

A lease is an advisory local record. It does not enforce edits, does not gate commits, and carries no
authority. It is useful if it turns silent overlap into an explicit refusal at the moment of claim; it
is killed if sessions route around it or if the refusal never fires in real concurrent work.

## Truth boundary

Lease state lives in `.corvint/leases/` and is private derived state under `AGENTS.md` invariant 4: it
is never an input to ranking, learning, evidence, or authority, and no evidence row, receipt, or
verdict may cite it. `acquire`, `release`, and `renew` are explicit local mutations like `record`.
`status` and `list` are read commands: they write nothing at all, including no reaping of expired
leases, and they do not create `.corvint/leases` when it is absent.

A lease proves only that a holder claimed a scope during a bounded interval. It does not prove the
holder edited that scope, did not edit outside it, or still exists. An expired or crashed holder's
lease is reclaimed by age alone, so a live lease is a coordination hint, never an exclusion witness.

## Command surface

```text
corvint [--root PATH] lease acquire --holder H --ttl D [--path GLOB]... [--ticket ID] [--note TEXT]
corvint [--root PATH] lease release --id ID --holder H
corvint [--root PATH] lease renew   --id ID --holder H --ttl D
corvint [--root PATH] lease status  --id ID
corvint [--root PATH] lease list
```

`D` is a Go duration (`90s`, `2h`); a bare number is refused. `lease --help` restates this surface,
pinned by `TestLeaseHelpMatchesCommandSurface` (`cmd/corvint/lease_test.go`).

Every action prints one canonical JSON line. Exit 0 is success, 1 is a refused claim or an unknown or
foreign lease, and 2 is an argument or local-state error emitted as `{"error": ..., "ok": false}` on
stderr.

## Overlap rule

Two path globs overlap when they are literal-equal, or when the leading run of glob-free path
segments of either is a path prefix of the other's. `internal/**` overlaps `internal/scopelease/x.go`;
`internal/a/**` does not overlap `internal/b/**`; `*.go` has an empty literal prefix and therefore
overlaps every scope. Both comparisons are case-insensitive on every host (decision 0168), since a
case-insensitive volume names one file for `Internal/x` and `internal/x`. A shared non-empty
`--ticket` always conflicts. The rule is deliberately
conservative: an undecidable case is a conflict, because a false refusal costs one renamed scope and a
false admission costs a silent concurrent edit.

## Requirements

- `SCL-V0-001`: `corvint [--root PATH] lease acquire` MUST write exactly one canonical JSON lease
  document at `.corvint/leases/<lease-id>.json` and print it, where the lease id is the leading 16 hex
  characters of the SHA-256 of holder, normalized scope, ticket, and acquisition nanoseconds. The
  document binds schema version, holder, sorted normalized paths, ticket, note, acquisition and
  expiry instants, and the tree revision observed at acquisition. The revision is informational and
  is never compared, verified, or used to admit or refuse a claim. It is read from `.git/HEAD` and
  at most one loose ref, each a regular file read to at most 256 bytes; a symbolic `HEAD` naming
  anything but a ref under `refs/` without an empty or dot-leading segment, a non-regular file, a
  ref held only in `packed-refs`, a linked worktree (whose `.git` is a file naming a Git directory
  elsewhere), or a value that is not 8 to 64 hex digits of even length is recorded as the empty
  string, so the read never leaves `.git` or blocks under the writer lock (security amendment
  2026-09-13).
- `SCL-V0-002`: Overlap MUST follow the conservative rule above: literal equality, glob-free-prefix
  containment in either direction, or a shared non-empty ticket. An undecidable comparison MUST be
  treated as an overlap. Paths are compared after lexical cleaning, so equivalent spellings
  (`./a`, `a//b`, `a/./b`, `a/`) name the same path, and after lowercasing, so case-fold-equal
  scopes overlap (decision 0168). A normalization-insensitive volume (APFS) also names one file for
  an NFC and an NFD spelling of a non-ASCII path; without a normalization table this cannot be
  decided, so any non-ASCII byte in either compared literal prefix MUST also be treated as an
  overlap (decision 0168 addendum, 2026-09-12).
- `SCL-V0-003`: An acquisition overlapping any live lease MUST fail with a non-zero exit and a JSON
  conflict report naming, per conflict, the live lease id, its holder, the reason, and the
  overlapping scope. A refused acquisition MUST leave lease state unchanged.
- `SCL-V0-004`: `lease release --id ID --holder H` MUST remove the lease only when `H` is the
  recorded holder; a foreign or unknown id fails non-zero and changes nothing.
- `SCL-V0-005`: `lease renew --id ID --holder H --ttl D` MUST extend only the recorded holder's lease
  and MUST bound `D` to at most 24 hours, as `acquire` does. The renewed expiry is the later of now
  plus `D` and the recorded expiry, so a renewal never moves expiry earlier (decision 0254).
- `SCL-V0-006`: `lease status` and `lease list` MUST NOT write, create, or delete any path, including
  no reaping and no creation of `.corvint/leases`. They MUST report a lease whose expiry has passed as
  `state: "expired"` rather than hiding or removing it.
- `SCL-V0-007`: Expired leases MUST be reaped only inside `acquire`, `release`, and `renew`, under
  the writer lock, before overlap is evaluated.
- `SCL-V0-008`: Mutating actions MUST serialize through one single-writer lock at
  `.corvint/leases/.lock` with a bounded wait. On darwin and linux the lock MUST be an exclusive `flock`
  on that never-removed file, released by the kernel when its holder exits, so no reclamation step
  can remove or bypass a live holder's lock. Deferred platforms (decision 0061) keep an `O_EXCL` lock
  with age-based reclamation that is not race-free. Every lease document MUST be published as a
  synced temporary file, a rename, and a directory sync, and `release` MUST sync the directory after
  removal, so no partial document is observable and a reported mutation is durable. A temporary
  file a crash leaves behind is not a `.json` document, so it is never read and never blocks a claim;
  nothing removes it. Repository
  content is untrusted, so `acquire`, `release`, and `renew` MUST refuse, before creating the lock
  or any document, when `.corvint` or `.corvint/leases` exists as a symlink or other non-directory
  (security amendment 2026-09-13).
- `SCL-V0-009`: `scopelease.Check(root, paths)` MUST report, without writing, every touched path that
  no live lease covers or that two or more live leases cover. A scope covers a path when they are
  equal or match segment by segment: a whole `**` segment matches zero or more segments (at least
  one when it is the last segment), and any other segment matches exactly one segment, so `*` never
  crosses `/` and `a**` is not `a/**`. Coverage is case-exact. It is a helper for a future CEM or
  dogfood consumer and MUST NOT be wired into ranking, evidence, authority, or any existing gate.
- `SCL-V0-010`: Every action MUST emit one deterministic canonical JSON line on stdout with a stable
  member set, sorted lease listings, and the documented exit codes. An invalid action MUST be
  refused before any flag is parsed, as argparse consumes the action positional first, and a request
  `scopelease` refuses before reading lease state (`ErrInvalidRequest`: empty holder, no path or
  ticket, TTL out of bounds, malformed path or lease id, a lease document over 1 MiB) MUST carry
  code `invalid-arguments` with its own message (decision 0205).
- `SCL-V0-011`: An unreadable or unparsable lease document MUST NOT block actions on other leases.
  A document whose `lease_id` differs from its file name is unreadable, so it can never cause
  reaping, release, or renewal to act on another lease's file. A document that is not a regular
  file (a symlink, FIFO, or device) is unreadable and is never opened, so no action blocks on or
  reads without end through it (security amendment 2026-09-13). A document larger than 1 MiB is
  unreadable and is read no further than that bound, and `acquire` MUST refuse as
  `invalid-arguments` a request whose document would exceed it (amendment 2026-09-13).
  Its scope is undecidable, so `acquire` MUST refuse every claim with a conflict whose reason is
  `unreadable-lease` naming the document's lease id; `list` MUST report it with
  `state: "unreadable"`; `Check` MUST report it against every touched path; `release` and `renew` of
  any other lease MUST proceed; reaping MUST NOT delete it. Recovery is deleting that one document.

## Non-goals

No daemon, resident process, watcher, or background reaper. No cross-machine, cross-worktree, or
networked coordination, and no shared lock service. No enforcement of edits: a lease never blocks a
write, a commit, a gate, or a command. No ranking, evidence, authority, receipt, learning, or CEM
input. No lease is a verification, exclusion, or possession claim.

## Failure modes

| Failure | Required response | Recovery |
|---|---|---|
| stale lock from a crashed writer | the kernel released the `flock` when the writer exited; a leftover `.lock` file is reused, never removed | the next mutating action proceeds normally |
| lock held past the bounded wait | fail non-zero with an explicit busy error; write nothing | retry |
| crashed lease holder | the lease expires by TTL and is reaped by the next mutating action | reacquire the scope |
| clock skew or backwards clock | expiry is compared as an absolute instant; an unparsable expiry is treated as expired | reacquire the scope |
| corrupt or truncated lease document | refuse every new claim with an `unreadable-lease` conflict naming it; list it as `unreadable`; other leases stay releasable and renewable | delete that one document |
| lease directory absent | read commands report an empty store; mutating actions create it | none needed |
| `.corvint` or `.corvint/leases` is a symlink or non-directory | mutating actions refuse non-zero before creating the lock or any document | replace it with a real directory |

### Conflict reason codes

The lease conflict check (`internal/scopelease`) emits the kebab-case codes below (decision 0100).
Each row cites the first emitting site and states only the condition checked there.

| Code | First emitting site | At the cited site |
|---|---|---|
| `multiple-leases` | `internal/scopelease/lease.go:247` | more than one live lease covers a requested path; one conflict is appended per covering lease, carrying its id and holder (zero covering leases give `uncovered` instead) |
| `path-overlap` | `internal/scopelease/lease.go:329` | on `Acquire`, a path held by a live lease overlaps a requested path; one conflict is appended per overlapping held and requested path pair, carrying the lease id, holder, and the held path |

## Traceability

| Requirement | Evidence |
|---|---|
| `SCL-V0-001` | `TestAcquireWritesOneLeaseDocument`, `TestAcquireRecordsOnlyARevisionGitWouldResolve`, `TestAcquireNeverBlocksOnANonRegularRef` |
| `SCL-V0-002` | `TestOverlapDecisionIsConservative`, `TestAcquireNormalizesEquivalentPathSpellings` |
| `SCL-V0-003` | `TestAcquireRefusesOverlappingScopeAndTicket` |
| `SCL-V0-004` | `TestReleaseRequiresMatchingHolder` |
| `SCL-V0-005` | `TestRenewExtendsExpiryForHolderOnly`, `TestRenewNeverShortensALease` |
| `SCL-V0-006` | `TestStatusAndListReportExpiredWithoutWriting`, `TestReadCommandsCreateNoLeaseDirectory` |
| `SCL-V0-007` | `TestAcquireReapsExpiredLease` |
| `SCL-V0-008` | `TestConcurrentAcquireAdmitsExactlyOneHolder`, `TestStaleLockIsReclaimed`, `TestStaleLockContentionGrantsAtMostOneLease`, `TestMutatingActionsRefuseSymlinkedLeaseDirectory` |
| `SCL-V0-009` | `TestCheckReportsUncoveredAndMultiplyCoveredPaths`, `TestCoversMatchesDoubleStarOnlyAsWholeSegment`, `TestCoversManyDoubleStarsInBoundedTime` |
| `SCL-V0-010` | `TestLeaseCommandAcquireRefusesOverlapAndListsWithoutWriting`, `TestLeaseListIsSortedByIdentifier`, `TestLeaseCommandRejectsMissingArguments`, `TestLeaseCommandLabelsRequestRefusalsAsInvalidArguments`, `TestLeaseCommandReportsAnInvalidActionBeforeItsFlags` |
| `SCL-V0-011` | `TestUnreadableLeaseDocumentBlocksOnlyNewClaims`, `TestMismatchedLeaseIdentifierCannotRemoveAnotherLease`, `TestListReportsASymlinkedLeaseDocumentUnreadableWithoutReadingIt`, `TestAnOversizeLeaseDocumentIsNeverWrittenOrRead` |

## Rollback

Delete `.corvint/leases`. No tracked file, index, trace, receipt, CEM, or gate depends on lease state,
so removal returns the repository to the current uncoordinated behavior with no migration.
