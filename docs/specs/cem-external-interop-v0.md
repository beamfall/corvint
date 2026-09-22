# CEM external consumer interoperability V0

Owner: Russell Lewis
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `ROADMAP.md`, `docs/specs/cem-pilot-kit.md`,
`interop/cem-0.1/ADAPTER.md`, `interop/cem-0.1/ALGORITHMS.md`, and
`interop/cem-0.1/manifest.json`

## Agent digest
- Claim: A Corvint-free runner lets independent CEM 0.1 consumers test the frozen packet without creating an independence claim.
- Status: proposed/experimental
- Exists: the active native `tools/cem-interop-runner`, the historical `interop/cem-0.1/runner.py` packet member (re-pinned only by erratum 1), frozen packet checks, and retained observations.
- Blocked on: an independently authored consumer matrix entry.
- Read next: `cem-pilot-kit.md` and `../../interop/cem-0.1/SUBMIT.md`.


## Intent

An unrelated implementer must be able to start a clean-room CEM 0.1 consumer experiment within ten
minutes and test it locally, without installing or executing Corvint, against every frozen public
valid, invalid, and drift vector. Completing a strict verifier may take up to one engineer-day. The
experiment must retain failures, preserve privacy, and never turn a reference self-test into an
independence claim. Failure retention covers every completed 32-case matrix; an operational abort
before matrix completion is emitted to the invoking terminal and is not added to the observation.

## Requirements

- `CEM-EXT-001`: the packet MUST provide a Corvint-free Python-standard-library runner that uses only
  local Git and invokes a consumer solely through the exact `ADAPTER.md` process ABI. It is the sole
  executable adjacent to the raw kit, is non-normative, and MUST NOT become an implementation
  dependency for a conforming consumer.
- `CEM-EXT-002`: `doctor`, `start`, and `consumer` MUST verify the raw frozen manifest SHA-256
  `2655258e73d569e35dffb36cc6d3ae2e738801848d9f57decb774463b92f34bd` before parsing, then verify
  its shape and all 51 exact artifact digests before any implementation execution. Manifest-only,
  artifact-only, and combined tampering MUST fail before invocation. All manifest-declared bytes
  MUST be captured once through bounded descriptor-relative, no-symlink reads; repository and case
  execution MUST use only that verified private snapshot.
- `CEM-EXT-003`: preflight and execution MUST reconstruct and verify the exact SHA-1, SHA-256, and
  six target revisions. Missing SHA-256 support is failure, not conformance or a skipped case.
- `CEM-EXT-004`: `consumer` MUST run all 7 valid, 19 invalid, and 6 drift cases. It MUST require the
  declared exit/decision, and exact ordered drift records, for every case. PASS means only exact
  agreement with this public matrix. The frozen suite has no multi-evidence drift-ordering vector,
  so strict multi-evidence conformance remains a versioned next-suite gap.
- `CEM-EXT-005`: implementation and Git processes MUST have bounded time and output and run in
  fresh process groups. On timeout, interruption, or completion, the runner MUST terminate the
  members remaining in the process group it created. A descendant that creates a new session or
  process group is outside this guarantee and requires an OS sandbox or equivalent supervisor.
  SIGTERM MUST enter the same cleanup path and emit the stable `interrupted` outcome.
- `CEM-EXT-006`: execution MUST use only the synthetic fixture repository. The runner MUST initiate
  no network operation, strip proxy/package injection state from the child environment, execute a
  fresh private read/execute-only copy of the initially captured implementation bytes for every
  case, and state honestly that it is not an OS network or same-UID sandbox for the implementation.
- `CEM-EXT-007`: observations MUST be closed, bounded, atomically written owner-only JSON. They MUST
  omit source and patch bodies, stdout/stderr, argv, prompts, environment contents, implementation
  paths, and absolute temporary paths. Creation MUST be atomic no-replace. A persistent owner-only
  POSIX `fcntl` sibling lock MUST cover each start transaction and the complete consumer
  load/run/write transaction; replacement MUST compare the source observation digest before its
  atomic write. Lock acquisition MUST be nonblocking with a five-second deadline and stable failure.
  The observation profile MUST bind the framed public packet digest, detected Corvint commit and tree
  state, and explicit `COMMITTED` publication state. Once link or replacement publishes the exact
  bytes, a post-commit cleanup, directory-fsync error, SIGINT, or SIGTERM MUST reconcile to that
  committed document; an unverifiable post-publication state MUST fail as
  `observation-commit-uncertain`, never as a plain interruption.
  `COMMITTED` means the exact bytes are observable at the destination; after a directory-fsync
  failure it does not claim crash durability.
- `CEM-EXT-008`: `start` MUST refuse an existing output. `consumer` MUST retain the complete first
  32-case result, refuse implicit replacement, reject `--retry` before a first completed run, allow
  exactly one explicit `--retry`, and refuse a third run. Operational aborts before all 32 cases
  complete MUST leave the prior observation unchanged.
- `CEM-EXT-009`: `inspect` MUST validate the closed observation before emitting only its bounded
  state, counts, sequence, durations, packet digest, publication state, and Corvint source identity.
- `CEM-EXT-010`: documentation MUST distinguish the ten-minute start target from the one-engineer-
  day implementation bound and MUST require source/ownership review before an independence cell is
  changed.
- `CEM-EXT-011`: producer execution and sealed anti-copying qualification MUST remain visibly
  `NOT_BUILT`. No runner result may fill `P1`, claim gaming resistance, or claim product value.

## Non-goals

- publishing a release archive, package, license, workflow, hosted result, or telemetry;
- executing producer jobs or filling `P1`;
- private or generated sealed qualification fixtures;
- claiming strict multi-evidence behavior before a versioned suite adds such a vector;
- preventing an implementation from making network syscalls without an OS sandbox;
- proving semantic support, correctness, usefulness, demand, or mature standardization; or
- modifying the frozen manifest, algorithms, schemas, maps, patches, repositories, or targets,
  except a recorded clarifying erratum that leaves every prior fixture byte and expected record
  unchanged (erratum 1, decision 0098 amendment, listed in `interop/cem-0.1/IMPLEMENTATIONS.md`).

## Compatibility note

The earlier raw-kit neutrality check rejected every adjacent executable. The historical packet
narrowly admitted `runner.py`; its bytes are packet data changed only by a recorded kit erratum.
Erratum 1 (decision 0098 amendment) re-pinned its manifest digest, 51 artifacts, and 7/19/6 matrix
so it accepts the kit's own manifest; no other runner behavior changed, and receipts made before
the erratum describe only the earlier packet. Active execution moved to
the native command outside the raw kit, without changing wire acceptance. Wire acceptance remains
defined only by `ADAPTER.md`, `ALGORITHMS.md`, and `manifest.json`; the harness has no Corvint
implementation dependency and is not part of the manifest-pinned normative artifact set.

## Failure and privacy model

The packet and consumer are untrusted inputs. The raw manifest root digest and every declared public
artifact are captured once with descriptor-relative no-symlink reads and checked before an
implementation runs. Traversal, total bytes, JSON nesting, integer digits, inputs, and outputs are
bounded; duplicate JSON keys and normalized decoder resource errors fail. Git is locally configured
without hooks, credentials, lazy fetch, or file protocol. The runner reads one bounded regular
executable entry point once, creates a fresh private read/execute-only copy for each case, passes a
small environment without proxy/package variables, rechecks that copy, and never supplies real
repository content.

The lock coordinates cooperating runner processes and the source-digest comparison detects a
changed observation before replacement; neither prevents a hostile same-UID process from bypassing
advisory locks or racing filesystem operations. The implementation interpreter and runtime
dependencies remain declared external inputs. The operating system, Go runtime, Git executable,
and those dependencies remain trusted. Process-group cleanup covers only members remaining in the
group the runner created; a detached descendant and network/filesystem access require an OS sandbox.
Observation digests establish byte identity, not authorship. A completed matrix retains all case
failures. A runner-level abort is printed as a bounded error and leaves the prior observation intact;
partial case progress and terminal stderr are deliberately not retained.

Each observation case has a closed state/failure invariant: `PASS` requires `failureCode: null`;
`FAIL` and `UNSUPPORTED` require a non-null stable failure code. Signal delivery is blocked across
the real process spawn and assignment into runner-owned state, then restored; a pending signal can
therefore enter cleanup only after the new process group is owned.

Observation profile 1 identifies the adjacent public packet by SHA-256 over the sorted top-level
packet filenames and bytes. Each entry is framed as a four-byte big-endian filename length, filename,
eight-byte big-endian content length, and content. The digest covers `ADAPTER.md`, `ALGORITHMS.md`,
`IMPLEMENTATIONS.md`, `START-HERE.md`, `SUBMIT.md`, `manifest.json`, `observation.schema.json`, and
`runner.py`; the manifest transitively pins the 51 fixtures. `corvintCommit` and `corvintTreeState`
identify the checkout context when present. A reviewed matrix submission requires `clean` plus a
public commit whose recomputed packet digest matches; `modified` and `standalone` remain local-only.
These identity fields bind experiment bytes but do not make the runner normative CEM behavior.

## Acceptance evidence

| Requirement | Implementation | Required evidence |
|---|---|---|
| CEM-EXT-001..004 | `tools/cem-interop-runner`; historical packet member `interop/cem-0.1/runner.py` (re-pinned by erratum 1) | native doctor/start/consumer/inspect, frozen-root and artifact tamper-before-invoke, bounded snapshot and process tests, CAS/lock/retry tests, exact ordered 7/19/6 identity, SHA-1/SHA-256, and documented multi-evidence gap; prior Python packet receipts remain historical and are not relabelled current |
| CEM-EXT-005..006 | runner process/Git supervisors | timeout/interruption/SIGTERM process-group cleanup, detached-child boundary, direct output and JSON resource bounds, fresh per-case executable, clean environment, and no-network-surface tests |
| CEM-EXT-007..009 | observation writer/parser and schema | permissions, bounded lock contention, direct existing-destination no-replace creation, concurrent start/retry, source-digest CAS, post-publication fault reconciliation, packet/source identity, privacy, schema, retry and inspect tests |
| CEM-EXT-010..011 | `START-HERE.md`, `SUBMIT.md`, `ADAPTER.md` | documentation assertions and explicit `NOT_BUILT` checks |

Local acceptance requires the focused native Go test package and diff check to pass. An
external consumer matrix entry additionally requires the evidence and independence review in
`SUBMIT.md`; this implementation cannot self-award that entry.

## Rollback

Delete the standalone runner, observation schema, packet documentation, focused tests, and this
spec. The frozen CEM 0.1 wire, manifest, and fixtures remain unchanged and require no migration.
