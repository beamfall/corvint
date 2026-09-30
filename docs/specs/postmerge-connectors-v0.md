# Post-merge Connectors V0

Owner: Russell Lewis
Date: 2026-09-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: human request https://github.com/beamfall/corvint/issues/392;
decision 0373; `docs/SPEC-DRIVEN-DEVELOPMENT.md`; `AGENTS.md`.

## Agent digest
- Claim: A separate companion reads pinned fixture context and renders bounded idempotent tracker and forge requests with recording and dry-run modes.
- Status: proposed/experimental; local references only, no remote service qualification.
- Exists: `internal/postmergeconnector`, `cmd/corvint-postmerge-connect`; conformance and CLI tests.
- Blocked on: owner acceptance and independent remote connector qualification; integration and native completion.
- Read next: Requirements; Trust boundary; Qualification and rollback.

## User and measurable job

A CI maintainer processes a merged change twice without duplicate follow-up items or drafts,
without allowing third-party prose into generated bodies or giving an author outward credentials.
The simpler baseline is handwritten tracker/forge glue. At base `25730eaa`, Core has no post-merge
connector command; network and mutable tracker state remain outside Core under decision 0373.
This proposed technical contract implements the human issue as a separately experimental companion.

## Requirements

- `PMC-V0-001`: Read a merged change with identity, base/merge commits, sorted changed path/status,
  opaque author and timestamps. Independently configured policy pins forge, repository, change,
  base and merge identities. Verify commits and the changed file set from immutable Git content.
- `PMC-V0-002`: Read the linked item and its complete parent chain to the configured level,
  bounded to 32 items. Missing items, duplicate IDs, cycles and depth truncation fail closed.
  Raw title/body goes only to a dedicated intake file outside the authoring worktree.
- `PMC-V0-003`: Render follow-up upsert from fixed templates over identifiers, enums, paths,
  allowlisted HTTPS URLs and nonnegative counts. Resolve parent and source links. Zero work
  means no item. Never interpolate raw context, caller body or caller template.
- `PMC-V0-004`: Findings require an allowlisted class and per-change cap, an existing regular
  path and positive line at the exact merge commit. Unknown classification blocks; security
  classification always routes to restricted triage. Stable keys make filing idempotent.
- `PMC-V0-005`: Draft requests use a stable derived branch/key, fixed generated body, and
  immutable `draft=true`; repeated upserts update the same request.
- `PMC-V0-006`: The separately qualified client contract supports `live`, `recording`, and
  `dry-run`. Recording and dry-run produce the same canonical JSONL request bytes. Exact repeats
  in one recording ledger add no duplicate events; an updated request retains its item key.
  Local fixture live references atomically save one tracker/forge state; remote live is unsupported.
- `PMC-V0-007`: Publish distinct tracker and forge reference connectors and a conformance kit.
  Keys hash an unambiguous canonical identity tuple. Validate the whole batch before effects;
  per-operation upsert reconciles partial connector failures on retry. No all-provider transaction
  or remote exactly-once guarantee is inferred.
- `PMC-V0-008`: Each connector declares minimum scopes; local references require none.
  Credentials are host-owned, available solely to deterministic writers. Read/intake/authoring
  steps may not receive outward-write credentials. This CLI reads no credential environment key.
- `PMC-V0-009`: Reject unknown JSON fields, duplicate keys, oversized inputs, malformed identities,
  bad policy, unsafe paths/URLs and unverified requests before any write. Raw-intake and mutable
  outputs require separate trusted private single-writer directories; reject input/output aliases,
  symlinks and nonregular files, publish with exclusive private temporary files and atomic replace.

## Trust boundary

Policy is a trusted CI input separate from author outputs. Classification is a trusted enum,
not an inferred security detector: a missing or unknown classification abstains. Callers must
supply trusted classifications and identity pins; matching pins do not authenticate a forge.
The CLI is an operator-started deterministic step, never registered as an agent/MCP write tool.
The host must isolate reader/author/writer environments and files; this process cannot attest a
sibling agent's environment or eliminate already disclosed secrets.

Raw fixture title/body is retained solely by `read-intake`; its writer requires the actual clean absolute author
worktree directory, and refuses destination overlap, source aliases and symlink ancestry. All write
plan fields are typed; the body has only fixed vocabulary and validated interpolations. Resource
limits: JSON 4 MiB, hierarchy 32, findings/drafts 64 each, Git blob 1 MiB, paths 512 bytes, IDs128,
URL2048, count at most one million, UTC timestamps, process timeout 10 seconds. Invalid inputs
produce no request or partial file. Mutable fixture/ledger publication is atomic per file; an
injected remote writer can have partial effects, safely reconciled by stable-key retries.

Single-writer trusted output directories and previously generated ledger/state bytes are explicit
prerequisites. Prior private data is preserved, not treated as fresh author input or sent to an
external writer. Static symlink checks and
final no-follow reads do not qualify hostile concurrent directory replacement, concurrent writers,
power-loss durability, remote retry/authentication semantics or network security. Such profiles
need separate qualification. Root-relative Git paths are verified from commit blobs, never the
worktree. Recording/state may not overwrite policy, plan, forge/tracker source or Git source files.
Protect the full product checkout even when invoked from a subdirectory, plus its actual Git
administrative/common directories when they are outside a linked worktree.

## Non-goals

Vendor API adapters, network calls, actual tracker/forge writes, running agents, accepting raw
prose as authority, automatically merging drafts, classification inference and Core registration.
References are different connector kinds (tracker and forge), not two qualified remote vendors.

## Acceptance evidence

`go test -count=1 -timeout 30m ./internal/postmergeconnector ./cmd/corvint-postmerge-connect`
is the published executable conformance kit. Tests create real Git commits, changed files, hierarchy
fixtures and reference state. They compare stable JSONL, replay repeated upserts, reject malicious
interpolations and nonexisting lines, restrict security routing, probe partial failures and unsafe
file destinations. Requirement IDs in test cases retain exact coverage; passing local fixtures
establishes no independent adopter or remote profile qualification.

## Traceability

| Requirements | Implementation | Evidence |
|---|---|---|
| PMC-V0-001, PMC-V0-002 | `read.go`, `git.go` | `TestConnectorConformance` |
| PMC-V0-003, PMC-V0-004, PMC-V0-005 | `plan.go` | `TestConnectorConformance` |
| PMC-V0-006, PMC-V0-007, PMC-V0-008 | `client.go`, `types.go` | `TestConnectorConformance`, `TestCLIReferenceWorkflow` |
| PMC-V0-009 | `decode.go`, `files.go` | `TestConnectorConformance`, `TestCLIReferenceWorkflow` |

## Qualification and rollback

Experimental local file references only. No credential, live remote or independent adopter run.
Promotion requires owner acceptance plus a separately reviewed authenticated remote adapter and
actual idempotency/rate-limit/credential-isolation qualification; uncertain transport stays blocked.
Rollback: stop invoking this optional companion, retain recorded evidence/state, and revert the
implementation commits. It adds no Core surface, installed binary configuration or store migration.
Maintenance: version incompatible wire changes; rerun the kit after policy/templates/reference
changes. Human issue intent remains separate from this proposed contract; tests cannot accept it.
