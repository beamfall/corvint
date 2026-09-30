# Authoring Step Scope V0

Owner: Russell Lewis
Date: 2026-09-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner-authored GitHub issue #393, native ticket V1-0540;
repository AGENTS.md and read-command invariants.

## Agent digest
- Claim: Local step verification binds observed checkout writes to a declared authoring scope.
- Status: Proposed intent; experimental source tests/vet and independent repair review pass; final binding pending.
- Exists: Experimental descriptor observer, strict records and publicly dispatched experimental handler.
- Blocked on: CLI integration, bound qualification and owner technical acceptance.
- Read next: Requirements; Declaration and receipts; Limits; Acceptance and rollback.

## Job and current state

Trusted generators must not run agent-modified scripts, fixtures or policy accidentally.
Existing per-host pathspec gates are the baseline. Core has no step declaration or receipt yet.
This capability adds explicit before/after observations and deterministic scope verification,
with enforcement retained by the host and all raw file contents withheld from receipts.

## Requirements

- `ASS-V0-001`: A strict versioned declaration must list allowed write paths/prefixes, guarded
  paths/prefixes, read-only checkout identities/roots and exposed environment keys. Guarded scope
  takes precedence over writable scope. Environment declarations contain keys only and must
  contain no outward-write credentials; supported host credential classification must be explicit.
- `ASS-V0-002`: Pre- and post-state observation must be read-only and bind full commit/tree,
  staged/index state and dirty content/mode/symlink digest for the authoring checkout and each
  declared read-only checkout. Tracked, untracked and ignored writes must be observable or cause
  a closed unsupported/bounds result. Missing checkout or metadata evidence cannot yield PASS.
- `ASS-V0-003`: Verification must name every observed out-of-scope write, guarded modification and
  read-only checkout modification deterministically, including adds, deletes, renames, mode and
  symlink changes. Compare against the actual pre-state, preserving initial dirt. It must also
  report changed commit/staged state and unsupported administrative state without losing writes.
- `ASS-V0-004`: A deterministic receipt must bind the exact declaration, both complete state
  digests, commit/tree and dirty digests, findings and explicit unknowns. Stale, cross-declaration,
  malformed or incomplete states fail closed. An untrusted author's claimed digest is not a
  trusted observation and cannot attest confinement or authorize the next step.
- `ASS-V0-005`: Optional preflight environment verification must reject every undeclared key and
  prohibited outward-write credential class without reading or returning environment values.
  An absent environment observation stays NOT_OBSERVED and cannot prove secret isolation.
- `ASS-V0-006`: Publish fixtures for each violation class and local read-only CLI qualification.
  Verification must not execute scripts, hooks, filters or connectors, mutate repository/index/
  trace state, or enforce a host job stop. Unsupported filesystem/object/config states fail closed.

## Declaration and receipts

The declaration is host-owned, frozen before exposing an authoring capability. Literal scope
entries match exactly or by a trailing directory-prefix slash; no globs/pathspec expressions.
Absolute checkout roots are canonical local repository locations. Every checkout gets a bounded
identifier. Git administrative files are guarded by default; supported staged/commit changes
need explicit behavior in the technical contract. Declarations must live outside author write
access. The snapshot and verify observer also run outside that capability, after host quiescence.

`corvint step snapshot --declaration FILE --host FILE` emits a state to stdout; the host stores it outside
observed worktrees. `corvint step verify --declaration FILE --host FILE --before FILE` recaptures actual
post-state and emits a receipt. Failure findings use closed codes and literal path/checkout IDs,
never file content or secret values. Output exit 0 is PASS, 1 is observed policy failure, and 2
is malformed, unsupported or unavailable evidence. A receipt digest permits citations; it is
not an authenticated signature. Offline replay from two snapshots is a separate claim from
actual filesystem verification.

## Limits and uncertainty

A finite observer cannot attest writes that were made and reverted between observations. A hostile
concurrent writer is outside this profile; observations detect available drift and refuse instead
of claiming an atomic filesystem snapshot. Supported Git administration needs an exact evidence
boundary; repository objects themselves may not be treated as agent-editable trusted executables.
Local receipts do not prove host isolation, future script safety, secret clearance or authority.
Fail closed for unsafe entries/FIFO/devices, depth/file/byte limits, missing roots and input drift.
Full native qualification is per OS/profile; cross-build success does not prove runtime behavior.

## Acceptance and rollback

| Requirement | Proposed witness |
|---|---|
| ASS-V0-001 | Declaration shape, unsafe paths, guarded/write overlap, credential fixtures |
| ASS-V0-002 | Initially dirty/staged/ignored/symlink/mode states, missing checkout, resource refusal |
| ASS-V0-003 | Every violation class plus allowed edit, rename/delete, read-only and metadata drift |
| ASS-V0-004 | Determinism, stale/cross-binding/corrupt states, dirty content changing without path changes |
| ASS-V0-005 | Exact allowed keys, extra keys, prohibited credentials, absent observation |
| ASS-V0-006 | Native CLI read-only qualification and trap fixtures for hook/filter/FIFO behavior |

Implementation owner: internal/stepverify, cmd/corvint/step.go and protocol/step. Required delivery
includes focused tests, focused-docs gate, independent review and change evidence. Source witness tests are implemented; focused handler/observer fixtures pass locally. Final
Public dispatch/help/maturity registration and focused Core/MCP conformance passed; final bound checks and native qualification remain pending. Roll back the optional command/caller wiring and retain receipts; no state
migration. Schema/observer changes invalidate prior qualification. Owner acceptance of this
technical profile and candidate evidence is required before promotion beyond experimental.

## Provisional technical resolution

The sibling containment-plan-resolution.md resolves the reviewed fixture oracle, raw filesystem
entry observer, admitted metadata profile, aliases, trusted host observations and safe executor.
It supersedes ambiguous implementation details here without changing owner intent. Its claims
remain proposed and its tests NOT_RUN until native admission and candidate evidence.

## Frozen experimental observer profile

The separately host-owned observation pins an absolute Git executable and its SHA-256 outside
the authoring/read-only checkout roots. It binds the exact declaration, session and capability
IDs, metadata protection and external confinement assertions; environment observations are null
or key/class pairs without values. These are unauthenticated host assertions, not enforcement.

Native Darwin/Linux descriptor-relative reads refuse symlink ancestors, hardlinked regular files
and unsafe entries. Each inventory binds all directories, regular files and symlink text, including
ignored/untracked/empty entries, with modes/content/identity digests. Race brackets and repeated
inventories must agree. Bounds are 20,000 entries, depth64, 32 MiB/file and128 MiB per pass.
The admitted ordinary/reciprocal Git profile excludes shallow/partial/alternates/submodules/sparse/
split-index/reftable/unknown extensions. Only index v2 with checksum validation and optional TREE extension is admitted.
Index, refs, hooks and declared metadata are fingerprinted;
objects, volatile logs, other-worktree metadata and private ledgers stay
HOST_PROTECTED_NOT_INVENTORIED. ACL, xattr and timestamp changes are not observed. Existing
scope directory ancestors bind identity/mode and cannot be replaced even under allowed scope.

A complete before-state is required. If post admin observation becomes unsupported while its
worktree inventory completes, the receipt retains all observed worktree findings and that component
digest, sets the whole post-state digest null, adds explicit admin unknown, and returns UNSUPPORTED
with exit2. Incomplete observations never imply no writes or yield PASS. Null environment
observation remains separately NOT_OBSERVED; an explicitly observed empty array may pass. Transient/reverted writes and hostile concurrency
remain outside this quiescent host-confined profile.

The handler is registered atomically with Core command/help/maturity/spec registry integration under fresh native admission67. Protocol schema, tests and receipts remain proposed/experimental until final evidence
closeout and owner acceptance. Original Gate A and current delta have no HIGH; descriptor traversal,
partial-post semantics and per-repository Git pin propagation are required acceptance witnesses.

## Candidate witness map

| Requirement | Source witness | Local status |
|---|---|---|
| ASS-V0-001 | TestStepDeclaration / strict shape and scopes; TestStepCLI / strict CLI options | Source fixtures PASS; final bound check pending |
| ASS-V0-002 | TestStepWrites / read only checkout; TestStepNativeRefusals / existing staged state is preserved | Initial observer source fixtures PASS; final bound check pending |
| ASS-V0-003 | TestStepWrites / violation matrix; TestStepNativeRefusals / read only admin changes | Source fixtures PASS; final bound check pending |
| ASS-V0-004 | TestStepStateAndUnsafe / corrupt binding refusal; TestStepWrites / incomplete post retains writes | Source fixtures PASS; final bound check pending |
| ASS-V0-005 | TestStepDeclaration / keys only preflight | Source fixtures PASS; final bound check pending |
| ASS-V0-006 | TestStepCLI / read only snapshot verify and env preflight; TestStepNativeRefusals; TestPinnedRepositoryGitAllReadPaths | Handler fixtures PASS; public command/native qualification pending |

Wire records follow protocol/step/schema.json and its README semantic rules. Digests use the
Corvint strict canonical JSON encoding, with each record's own SHA-256 field blank before hashing.

## Closed observation vocabulary

Fixed CLI error codes are `STEP_INPUT` and `STEP_UNSUPPORTED`; the internal race sentinel
`STEP_DRIFT` is emitted as STEP_UNSUPPORTED. Errors never return input content or parse detail.
Finding codes are `ENVIRONMENT_PROHIBITED`, `ENVIRONMENT_UNDECLARED`, `READ_ONLY_CHANGED`,
`GUARDED_WRITE`, `OUT_OF_SCOPE_WRITE`, `SCOPE_ANCESTOR_CHANGED`, `COMMIT_CHANGED`,
`INDEX_CHANGED` and `ADMIN_CHANGED`. A path/checkout labels each observed finding.

Unknown codes are `CONTENT_UNSUPPORTED`, `CHECKOUT_ALIAS_UNSUPPORTED`,
`SCOPE_ANCESTOR_UNSUPPORTED`, `GIT_PIN_UNAVAILABLE`, `ADMIN_UNSUPPORTED`, `CONTENT_DRIFT`,
`ADMIN_OR_BINDING_DRIFT`, `GIT_BINARY_DRIFT`, `HOST_ASSERTIONS_UNAUTHENTICATED`,
`EXTERNAL_AND_TRANSIENT_WRITES_NOT_OBSERVED`, `CONCURRENCY_NOT_ATTESTED`,
`ADMIN_OBJECTS_LOGS_OTHER_WORKTREES_AND_PRIVATE_LEDGERS_HOST_PROTECTED_NOT_INVENTORIED`,
`ACL_XATTR_AND_TIMESTAMP_CHANGES_NOT_OBSERVED`, `POST_STATE_INCOMPLETE`,
`ENVIRONMENT_VALUES_NOT_READ` and `ENVIRONMENT_NOT_OBSERVED`.

State completeness is a boolean. Write verdicts are PASS, FAIL or UNSUPPORTED; environment
verdicts are PASS, FAIL or NOT_OBSERVED. Null observation never proves environment isolation.
Additional final conformance evidence includes error-code-ownership-check (its syntactic gate
covers kebab-case error codes, while this spec explicitly owns these uppercase profile codes),
root-help/maturity/public-command and pinned-plumbing tests, unchanged MCP tools-list golden
checks, and filtered immutable CLI parity replay. Existing frozen enrollment is retained;
supplemental checks do not replace or silently change it.

The declared environment classes are NONE and READ_ONLY; host observations may additionally
report OUTWARD_WRITE or UNKNOWN, which always fail. Admitted top-level administrative regular
files are HEAD, config, config.worktree, index, packed-refs, description, COMMIT_EDITMSG, FETCH_HEAD,
ORIG_HEAD, commondir, gitdir and locked. Admitted inventoried directories are refs, hooks, info
and branches; info admits only exclude, attributes and grafts. Objects, logs, worktrees, corvint
and taskman must be real directories when present but their contents remain host-protected
and unobserved. All other top-level administration refuses rather than becoming hidden evidence.
