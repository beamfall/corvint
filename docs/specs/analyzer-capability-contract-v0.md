# Analyzer Capability Contract V0

Owner: Russell Lewis
Date: 2026-08-25
Intent status: accepted
Delivery status: experimental (static in-memory registry/lock model and unqualified one-shot boundary; no persisted signed registry, accepted profile, or supported launch)
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/ARCHITECTURE.md`, `docs/EXTENSION-ECOSYSTEM.md`, and
`docs/specs/go-production-kernel-migration-v0.md`.
Owner amendment anchor: direct 2026-08-25 task instructions to build Corvint for Beamfall dogfood,
keep language support in separately installed plugins rather than one giant binary, and distinguish
language/library versions individually. This anchor permits only isolated unregistered experimental
candidates; it does not accept a profile or weaken any selection, trust, containment, or promotion
gate below.

## Agent digest
- Claim: Optional external analyzers may submit bounded candidates only through Corvint-owned admission, containment, and exact-verification boundaries.
- Status: accepted/experimental (static in-memory registry/lock model and unqualified one-shot boundary; no persisted signed registry, accepted profile, or supported launch)
- Exists: `internal/analyzercap` and the isolated `internal/analyzerexec` boundary.
- Blocked on: one accepted supported-launch qualification profile.
- Read next: `analyzer-candidate-profiles.md` and the selected family-specific candidate spec.

## User and measurable job

A Corvint user may opt into one narrowly qualified, native external analyzer when Core's bounded,
Git-only evidence compiler cannot extract a declared language or ecosystem fact mechanically. The
analyzer returns a proof-carrying candidate for a single frozen profile; Corvint independently admits
or rejects it. The job is successful only when the exact executable, protocol, profile, target,
inputs, and result can be reproduced or explicitly rejected offline.

This is a capability contract, not a Go Production Kernel (GPK) exception. Core's normal baseline
remains one local process with Git as its sole required executable. An analyzer is optional, is never
needed for a GPK parity claim, and is not invoked until this contract and the selected profile are
accepted. `OBSERVED` output and an unavailable analyzer never promote a Corvint claim.

## Verified current state

### Beamfall shadow dogfood (experimental evidence only)

The superseded `beamfall-shadow-dogfood-v1` fixture/receipt slice remains evidence-only: its
language-neutral manifest selected one closed tuple, staged descriptor-bound artifact bytes, and
recorded bound per-run receipts including artifact, harness, request, response/stderr, wall/RSS,
and cleanup identities. Literal external fixtures covered Go, JS/TS/React, HTML/CSS,
Swift/Objective-C/C/Metal, Kotlin/Java/JNI/GLSL, SQL, shell, Python, structured/web assets, Ruby,
and .NET. These fixtures and receipts establish neither selection, launch, admission, semantic PASS,
network denial, runtime qualification, nor support authority.

- An unqualified native-runtime candidate exists only behind the modular Go
  `internal/analyzercap` and `internal/analyzerexec` boundaries. It derives
  canonical request bytes in Core, records closed receipts, and has a
  Darwin-contained one-shot backend; it does not select a profile by default,
  ship registry storage/signatures, or qualify any analyzer tuple.
- The GPK contract permits no production Python, shell, network, package-manager, daemon, or
  non-Git executable dependency. Its experimental slices must retain exact Python parity where
  they claim it.
- The extension strategy permits thin, local process integrations but does not grant them evidence
  authority or a second graph. No extension or adapter may parse source and silently become an
  independent authority.

## Definitions

- **Core**: the Corvint executable and its existing Git-pinned evidence compiler. It owns policy,
  profile selection, confinement, limits, receipts, admission, and all final evidence states.
- **analyzer**: a signed, locally registered native executable launched once for one request. Its
  producer language and toolchain are not Core policy or parsing inputs: the versioned external
  plugin/profile qualifies an exact native binary. Interpreters, in-process or dynamic-library
  plugins, daemons, network services, downloads, and background workers are out of scope.
- **plugin**: one local registry identity that offers one or more named analyzer capabilities.
  Capability-to-plugin resolution happens before release resolution. A plugin is not an authority,
  shared provider kernel, or generic fallback mechanism.
- **release ID**, **build ID**, **manifest digest**, and **platform binary digest** are distinct,
  opaque, signed exact values. A release ID names a published release; a build ID names one build
  of that release; a manifest digest binds its declaration; and a platform binary digest binds the
  executable for one host tuple. None has ordering or version-nearness semantics.
- **profile**: one immutable, canonical, scoped, bounded set/graph of components. A component
  instance identity/key is `(scopeID, role, ecosystemCoordinate, stableInstanceID)`; its canonical
  component tuple is `(scopeID, role, ecosystemCoordinate, stableInstanceID, scheme,
  canonicalExactValue, sourceEvidenceDigest)`. The profile identity is its canonical ordered graph
  plus those full canonical component tuples. `stableInstanceID` distinguishes otherwise equal
  installed instances; all fields are mandatory and length-bounded. The signed
  selected-release manifest, not repository data, publishes the profile's correlated DNF and its
  closed declared input families. Repository data may supply only evidence selected by that closed
  declaration; it MUST NOT publish compatibility, clauses, or input-family relevance.
- **host platform**: the OS/architecture/ABI on which Core launches the analyzer. **target
  platform**: the OS/architecture/ABI described by one compilation unit. They are separate tuples;
  a host-compatible binary does not establish target compatibility.
- **compilation unit**: one exact source unit or manifest-resolved unit that owns a target profile.
  Each unit has a bounded, ordered set of exact target candidates and no implicit project-wide
  compatibility inheritance.
- **candidate**: untrusted analyzer output that has passed framing/schema checks but has not passed
  Corvint's independent admission verifier. **admission** is not exact semantic `PASS`.
- **capability projection**: the bounded canonical view that Core computes only after choosing an
  exact plugin release. It contains the release's closed declared input families and their exact
  selected evidence, not the entire repository profile or snapshot.

## Requirements

### Authority, process, and artifact identity

- `ACC-V0-001`: Core MUST remain Git-only when no accepted analyzer profile is explicitly selected.
  Analyzer support MUST be a modular package boundary; it MUST NOT pull analyzer libraries,
  toolchains, language parsers, SDKs, or ecosystem frameworks into the core binary.
- `ACC-V0-002`: V0 MUST launch only a local, native external executable from a trusted signed
  local registry. It MUST NOT use a repository-supplied executable path, a path in the profile or
  lock, a URL, a command string, shell parsing, `PATH` lookup, environment command expansion,
  interpreter, dynamic library, daemon, socket, network, download, package manager, or
  automatic installation. Before launch, Core MUST resolve the registry-owned executable to a
  canonical absolute path outside the repository, verify an owner-safe no-follow regular-file
  identity and all pinned digests (the artifact, executable, and staging parent are outside the
  repository only when neither the lexical path nor any ancestor directory's identity names the
  bound repository root, so a case-insensitive volume's differently spelled alias is inside),
  then execute those exact bytes through a stable file handle or
  owner-private content-addressed staging. Core validates the complete native object and
  executable mapping behavior for the signed host tuple, rather than magic bytes or a producer
  language/toolchain provenance. On Darwin that validation is exact and testable: the retained
  descriptor MUST parse as a little-endian Mach-O executable whose class agrees with its
  architecture; declare exactly one macOS platform (one `LC_BUILD_VERSION` with platform macOS or
  one legacy `LC_VERSION_MIN_MACOSX`, never both, never an iOS/tvOS/watchOS declaration, never a
  zero minimum or an SDK below the minimum); carry exactly one `LC_MAIN` whose `entryoff` falls
  inside the executable `__text` section's file-backed range; and map at most 128 segments whose
  file ranges lie inside the file, whose file-backed and virtual ranges do not overlap, whose
  initial protections are a subset of their maximum protections, and which are never initially
  writable and executable together. The candidate ABI encodes the declared minimum macOS version;
  Core observes the actual host version (`kern.osproductversion`) and launches only candidates
  whose minimum is not newer than the observed host. On Linux the candidate ELF tuple MUST equal
  the host tuple exactly. Darwin lacks `fexecve`: its retained descriptor and the exact
  owner-private (`0700`) staging entry MUST have equal identity and bytes immediately before the
  sandbox receives the canonical exact staged path. Staging is content-addressed and persists
  across invocations (decision 0148), because Darwin's first exec of a fresh inode alone exceeds
  the 100 ms plan cap: the one `0500` executable lives in its own owner-private directory named by
  the pinned executable digest, is published only by renaming a fully written and hash-verified
  private temporary directory, and on reuse Core still hashes the registry source against the pin
  and re-verifies the entry's directory, identity, mode, and bytes before sealing it. Cleanup closes
  every descriptor and removes an unpublished temporary directory or an entry that failed reuse
  verification; it does not remove a verified published entry, nor a reused entry whose staging
  stopped on cancellation, timeout, or a registry-source race or digest mismatch. Source traversal MUST close every
  directory root and temporary bound directory file on success and failure, including a failed
  post-open directory lookup, before returning; closure MUST NOT depend on garbage collection.
  A concurrent publish of the same digest is a terminal race for the losing invocation.
  Decision 0153 keeps the 100 ms
  invocation cap: the first invocation of a newly staged inode MAY report `TIMEOUT` while Darwin
  assesses it. This is an expected platform limitation, not a successful warmup or evidence;
  Core MUST NOT pre-execute the payload during staging, retry the invocation, or conceal its
  timeout. No non-payload staging assessment mechanism is qualified. Later invocations MAY
  benefit from OS assessment of the retained inode, but MUST independently obey the same cap.
  The approved Darwin trust boundary trusts the
  invoking same-UID host account; active same-UID replacement after that final revalidation is
  explicitly unsupported. All other path/descriptor, symlink, hard-link, permission, digest, and
  identity races are terminal. Directory ancestry cannot be observed atomically, so repository
  containment is decided before the no-follow open; a concurrent rename that moves an ancestor
  directory into the repository after that decision is not detected (decision 0184). It cannot
  change the executed bytes, which stay bound to an invoking-UID, single-link regular file and the
  pinned digests. A Darwin sandbox MUST deny `/System`, `/System/Volumes/Data`, and
  Mach lookup/registration by default. It MAY allow `file-read-data` on the literal root
  directory `/` and nothing beneath it, which a payload needs to start on macOS 26 (decision
  0145); it MUST NOT allow any `/` subpath. A future service allowance requires an explicit, exact
  profile-reviewed service filter. Core constructs an argument vector with no
  shell and reaps the one child and all descendants on success, failure, timeout, cancellation, and
  interrupt.
- `ACC-V0-003`: Registry trust is separate from repository authority. The local registry MUST have
  an owner-configured trust root, signed immutable snapshots, monotonically identified trust epochs,
  exact release ID, build ID, manifest digest, executable artifact digest, host-platform binary
  digest, and a canonical registry-snapshot digest. A missing, expired, invalid, revoked, duplicate,
  ambiguous, or unsupported registry entry is terminal and produces no launch.
- `ACC-V0-004`: Capability-to-plugin resolution MUST precede release resolution. For a requested
  capability, zero signed registry plugin IDs is `UNSUPPORTED(NO_CAPABILITY_PROVIDER)`; more than one is
  `AMBIGUOUS_PROVIDER` unless an enforced repository lock or accepted policy names exactly one
  plugin ID. Core MUST NOT select by registry/catalog order, popularity, install order, release
  ordering, or a default. Only after that selection may it resolve an opaque exact signed release.
- `ACC-V0-005`: The repository lock is a mandatory terminal input, not a preference. It MUST pin
  capability ID, selected plugin ID, opaque exact release ID, build ID, manifest digest, protocol
  version, executable artifact digest, host-platform binary digest, capability-projection digest,
  projection scheme version, resolver digest, and profile identity, but MUST contain no executable
  path, URL, command, or credentials. It MUST NOT pin or imply the full repository profile. If the
  lock is absent, malformed, stale, unsigned where policy requires a signature, inconsistent with
  the registry snapshot/trust epoch, names a missing candidate, or cannot bind one exact closed
  projection, Core MUST return `LOCK_UNAVAILABLE` or `LOCK_MISMATCH`. Zero or multiple plugin
  candidates are instead the scoped terminal results in `ACC-V0-004`. Core MUST NOT discover,
  search, select a nearest release, choose a default, or fall back to another analyzer.
- `ACC-V0-006`: Releases are exact and side-by-side. Release IDs are opaque and unordered: a new
  release ID, build ID, manifest digest, executable byte, host binary digest, registry snapshot,
  trust epoch, protocol, projection scheme, profile value, or source-evidence digest creates a
  distinct candidate. Mutable aliases and nearest-version fallback are forbidden. A lock pins one
  candidate set; no installed "latest" or replacement binary may satisfy it.

### Profile and compatibility semantics

- `ACC-V0-007`: Every profile MUST carry the full canonical component graph defined above, plus
  distinct host and target platform tuples. Every `scopeID`, role, ecosystem coordinate, instance
  ID, scheme, exact value, and source-evidence digest participates in profile identity and every
  relevant cache, receipt, lock, and conformance vector. The signed selected-release manifest MUST
  publish its bounded correlated DNF and closed input families before Core reads repository evidence.
  A profile without canonical exact value, immutable source evidence, or a closed manifest
  declaration is unavailable, never inferred. The component identity `(scopeID, role,
  ecosystemCoordinate, stableInstanceID)` is unique within a profile: `NewProfile` refuses two
  components sharing it, whatever their scheme, value, or evidence, with `PROFILE_UNAVAILABLE`,
  so a requirement can never match a value the profile also contradicts.
- `ACC-V0-008`: Compatibility is correlated DNF, evaluated independently for every compilation
  unit and target. The signed selected-release manifest publishes an ordered, bounded disjunction
  of clauses; each clause is a bounded conjunction over exact host tuple, exact target tuple,
  profile identity, analyzer release, protocol, feature state, and declared dependency edges. Core
  MUST preserve correlation within a clause; it MUST NOT union values across clauses, apply
  project-wide compatibility, accept repository-published clauses, or turn
  `(host=A,target=X) OR (host=B,target=Y)` into any cross-pair. Clause IDs are unique within a
  manifest: a manifest repeating a clause ID, whatever the bodies, is refused with
  `CAPABILITY_PROJECTION_UNAVAILABLE`, because resolution and receipts echo only the ID.
- `ACC-V0-009`: V0 supports only canonical exact equality schemes and exactly one frozen
  `semver-2.0.0-range/1` scheme. Its complete ASCII-byte grammar is:

  ```text
  range   = ">=" version " <=" version
  version = number "." number "." number
  number  = "0" | ("1".."9") *("0".."9")
  ```

  Both endpoints are mandatory and inclusive; the lower endpoint's SemVer 2.0.0 core precedence
  MUST be less than or equal to the upper endpoint's. There is exactly one ASCII space between
  endpoint tokens and no other whitespace, leading zero, coercion, normalization, or alternate
  spelling. V0 rejects prerelease and build metadata outright, so it does not silently apply their
  SemVer precedence/equality rules. An absent scheme is `SCHEME_MISSING`; a named scheme with
  malformed canonical value is `SCHEME_INVALID`; an unregistered scheme name is `SCHEME_UNKNOWN`;
  a recognizable `semver-2.0.0-range/N` where `N > 1` is `SCHEME_FUTURE_VERSION`; and a registered
  but V0-disallowed scheme is `SCHEME_UNSUPPORTED`. These codes classify the scheme field of both
  a profile component and a clause component requirement; an absent or malformed scheme there is
  never reported as `PROFILE_UNAVAILABLE` or `CAPABILITY_PROJECTION_UNAVAILABLE`. Omitted
  endpoints, wildcards, caret/tilde, aliases, and every malformed or negative scheme version are invalid or unsupported under those
  closed codes. Canonical grammar, endpoint inclusivity, each code path, and rejected
  prerelease/build forms require frozen positive and negative goldens. Every different language,
  library, framework, compiler/toolchain, or platform version requires an individually scoped
  profile and exact tested-tuple qualification.
- `ACC-V0-010`: Profile features have only `enabled`, `disabled`, or `unknown` state. `unknown` is
  incompatible with a clause that requires an enabled or disabled value and never means false,
  absent, default, or "any". A profile may match only one fully evaluated clause; zero matches is
  `NO_COMPATIBLE_CLAUSE`, while multiple matches is `AMBIGUOUS_CLAUSE`.
- `ACC-V0-011`: Dependencies are explicit bounded edges, not ambient discovery. Each edge names a
  dependent profile identity, relation kind, optionality, exact required state, and evidence digest;
  profile, clause, and whole-request edge counts have hard global bounds. If transitive resolution
  would exceed a bound, cross an unavailable edge, or lack a qualifying exact profile, Core MUST
  return `TRANSITIVE_PROFILE_UNAVAILABLE` and fail the whole requested profile. It MUST NOT recurse
  unboundedly, borrow a sibling's dependency, or continue with a partial graph.

### Invocation, protocol, and result boundaries

- `ACC-V0-012`: Core MUST provide the analyzer only an immutable, minimum, explicitly bounded
  request: protocol version, an authority-specific request ID, profile/lock/registry identities,
  selected compilation-unit and target identities, the exact closed capability projection, an input
  binding derived from the actual selected input bytes and projection identity, declared
  source-evidence handles and digests, feature states, and limits. The authority-specific request
  ID MUST domain-separate the Core instance, per-authority nonce, exact plugin ID, release ID,
  build ID, manifest digest, resolved release identity, and all other request inputs. Core MUST
  compute that projection from the chosen
  release's declared closed input families before launch; a missing, open, or changed relevance set
  is `CAPABILITY_PROJECTION_UNAVAILABLE`. Repository paths and file contents are supplied only when
  an accepted profile permits them and are confined to the captured Git snapshot. The analyzer has
  no authority API, credential discovery, Git command, arbitrary filesystem walk, tool execution,
  environment map, or network capability.
- `ACC-V0-013`: The protocol is framed canonical bytes with bounded input, output, nesting, fields,
  units, clauses, edges, time, memory, and child count. The analyzer MUST exactly echo protocol
  version, authority-specific request ID, profile identity, lock digest, registry-snapshot digest, trust epoch,
  resolver digest, selected clause ID, compilation-unit ID, target tuple, capability-projection
  digest and scheme version, Core's input binding, and every supplied input handle/digest. Any absent,
  reordered/noncanonical, forged, extra, mismatched, duplicate, or stale echo is
  `PROTOCOL_ECHO_MISMATCH`, produces no candidate, and is terminal for that run. The contained-launch
  plan MUST additionally bind the exact canonical request bytes, Core/authority identity, exact
  plugin/release/build/manifest identity, signed host tuple, and independently resolved target
  tuple before any staging work.
- `ACC-V0-014`: Analyzer output is hostile and may express only a bounded set of proposed facts,
  exact input references, profile evidence exactly equal to the request's input handles/digests, and
  diagnostics. It MUST NOT set Corvint authority,
  acceptance, spec state, test result, feature state, compatibility, policy, limits, file selection,
  command, profile identity, or receipt fields. Prompt-like/comment-like source or output remains
  inert data.
- `ACC-V0-015`: Core independently performs deterministic framing validation, exact lock/registry
  binding, profile/compatibility re-evaluation, input-coverage validation, and the named admission
  verifier. Admission yields `ADMITTED` or `REJECTED` only; it is distinct from an exact verifier's
  `PASS`, `FAIL`, `NOT_RUN`, or `UNSUPPORTED`. No analyzer output, `ADMITTED` candidate, or
  `OBSERVED` evidence may promote a Corvint claim, close a frontier, or satisfy an obligation.
- `ACC-V0-016`: Runtime observations may be recorded only as `OBSERVED` (or `none` when no result
  exists), with a scoped reason and local measurement. `OBSERVED` and `none` are non-promotable.
  A later exact verifier may independently produce `PASS|FAIL|NOT_RUN|UNSUPPORTED`; those states
  must name the verifier/profile/revision and cannot be synthesized from analyzer success, exit 0,
  installation, or admission.

### Failure, cache, diagnostics, and resource policy

- `ACC-V0-017`: Every denial, rejection, and no-result MUST carry exactly one scoped reason from
  this closed V0 taxonomy:
  `REGISTRY_UNTRUSTED`, `REGISTRY_SNAPSHOT_UNAVAILABLE`, `LOCK_UNAVAILABLE`, `LOCK_MISMATCH`,
  `NO_CAPABILITY_PROVIDER`, `AMBIGUOUS_PROVIDER`, `BINARY_DIGEST_MISMATCH`, `HOST_PLATFORM_UNSUPPORTED`,
  `TARGET_PLATFORM_UNSUPPORTED`, `EXECUTABLE_IDENTITY_UNSAFE`, `EXECUTABLE_RACE`,
  `PROFILE_UNAVAILABLE`, `CAPABILITY_PROJECTION_UNAVAILABLE`,
  `SCHEME_MISSING`, `SCHEME_INVALID`, `SCHEME_UNKNOWN`, `SCHEME_FUTURE_VERSION`,
  `SCHEME_UNSUPPORTED`, `NO_COMPATIBLE_CLAUSE`, `AMBIGUOUS_CLAUSE`,
  `TRANSITIVE_PROFILE_UNAVAILABLE`,
  `FEATURE_UNKNOWN`, `PROTOCOL_ECHO_MISMATCH`, `INPUT_EVIDENCE_MISMATCH`, `ADMISSION_REJECTED`,
  `EXACT_VERIFIER_NOT_RUN`, `EXACT_VERIFIER_UNSUPPORTED`, `LIMIT_EXCEEDED`, `TIMEOUT`,
  `CANCELLED`, or `ANALYZER_FAILURE`. Reasons MUST include only identities the caller may see and
  MUST distinguish host from target, missing/invalid/unknown/future/unsupported scheme, no matching
  clause from multiple matching clauses, and admission from exact verification.
- `ACC-V0-018`: Before every cache lookup, each invocation MUST validate the current registry
  signature, trust epoch, and revocations; recompute the complete bounded capability-to-plugin
  candidate set; resolve the terminal lock, selected release, profile, projection, resolver, host,
  target, clause, dependency, and compatibility identities; and only then derive the cache key.
  Cache keys and receipts MUST bind registry snapshot digest, candidate-set digest, trust epoch,
  resolver digest, selected clause ID, plugin ID, opaque release ID, build ID, manifest digest,
  protocol, executable artifact digest, host binary digest, full capability-profile identity,
  host/target tuples, compilation-unit/input digests, the full snapshot digest, capability input-set
  digest, capability-projection digest and scheme version, feature states, dependency-edge digest,
  request bytes, and admission/exact-verifier digests. A projection is computed only from the
  chosen release's closed declared input families; a missing or open relevance set is terminal. A
  changed component invalidates the entry. A cache hit may skip only analyzer execution; it MUST NOT
  skip registry/trust/revocation validation, provider resolution, lock/profile/resolver validation,
  compatibility, admission, or exact verification.
- `ACC-V0-019`: Hard global bounds apply before launch and throughout the request: exactly one
  analyzer launch when a nominated profile reaches launch, zero retries (including after a Darwin
  first-inode assessment timeout; decision 0153), no descendant survival,
  bounded profile/unit/clause/edge counts, source bytes,
  request/output bytes, nesting, wall time, CPU/memory where the host can enforce them, and receipt/
  cache bytes. Wall-time cancellation begins before authority source retrieval and is checked at
  root binding, native-object validation, every bounded hash/copy pass, staging, immediate
  pre-launch revalidation, process launch, protocol, and admission;
  production Core verification MUST be data-only or another Core-owned bounded implementation;
  caller-supplied in-process callbacks are rejected before invocation because a Go context cannot
  forcibly terminate an uncooperative goroutine;
  retained staging descriptors are close-on-exec; and a pre-start cleanup failure remains typed
  residue evidence in the terminal receipt. `NOT_RUN` states only actual no-start/no-cleanup facts;
  malformed backend cleanup observations are explicitly `REJECTED`, never normalized to
  `NOT_RUN`. Termination and cleanup-error observations are closed enums; an unrecognized backend
  value is malformed and rejected rather than retained in a receipt. Any bound, cancellation,
  process, validation, dependency, or exactness failure fails
  the whole requested profile; partial unit results, partial candidate sets, and partial receipts are
  not returned as usable analyzer evidence.
- `ACC-V0-020`: Diagnostics are stage-scoped and performance-ratcheted. The receipt records bounded
  local elapsed time and result for `lock`, `registry`, `resolve`, `compatibility`, `dependency`,
  `launch`, `protocol`, `admission`, and `exact-verification`; it contains no source body, command,
  path, URL, environment, or secret. Each accepted profile establishes a frozen per-stage p50/p95
  baseline from at least 100 fresh-process samples after five warmups. A later release may not raise
  a stage's p95, total p95, or failure rate by more than 10% without a newly accepted profile and
  a recorded reason; unmeasured stages are `NOT_RUN`, not waived. Reference-profile acceptance also
  requires absolute caps: non-nominated paths have p95 at most 5 ms and zero launches; analyzer
  startup at most 25 ms; complete invocation at most 100 ms; normal cleanup at most 10 ms; and
  cancellation cleanup at most 250 ms. Measure each cap under one fixed revision, accepted profile,
  cache state, and sanitized environment with at least five discarded warmups and 100 fresh
  processes, reporting p50/p95/max/failure count and both absolute-cap and 10% ratchet results.

### Promotion, modularity, and rollback

- `ACC-V0-021`: An analyzer profile is eligible only after an owner accepts its profile contract,
  lock format, registry trust root/snapshot policy, protocol fixture set, admission verifier, exact
  verifier or explicit `NOT_RUN` boundary, compatibility DNF, limits, and rollback procedure.
  Capability acceptance permits an optional one-shot process only; it does not amend GPK parity,
  default CLI selection, Core authority, or the Git-only baseline.
- `ACC-V0-022`: Promotion is per exact `(analyzer release, host binary digest, host tuple, target
  tuple, profile identity, clause ID, protocol, verifier)` tuple. Passing one host, language,
  library, framework, toolchain, platform, or compilation-unit tuple does not promote another.
  The registry must retain side-by-side known-good releases for rollback until the owner retires
  them explicitly.
- `ACC-V0-023`: Rollback disables the affected locked profile or selects its prior exact locked
  release without deleting repository evidence or rewriting locks. Core then returns the normal
  bounded `UNSUPPORTED`/`UNKNOWN` outcome for that profile; it never substitutes a nearby release,
  Core parser, Python fallback, dynamic library, network service, or partial analyzer answer.
  Registry/cache cleanup is disposable and cannot be required for rollback.

## Non-goals and simpler baseline

The simpler baseline is Core's existing Git-only deterministic evidence compiler and explicit
`UNKNOWN`/`UNSUPPORTED` results. V0 does not deliver a general plugin ABI, in-process plugins,
Python interpreters, in-process/shared-object plugins, analyzers from the repository,
automatic toolchain installation, remote registry, network service, daemon, watch mode, shell
adapter, PATH discovery, package manager, language server, source upload, telemetry, or semantic
authority delegation. It does not relax GPK exact parity or make an analyzer a requirement for any
ordinary `corvint` invocation.

## Failure modes and fail-closed behavior

| Condition | Required result |
|---|---|
| Missing/untrusted registry or invalid signature/epoch | No launch; terminal registry reason |
| Absent/malformed/mismatched lock | No discovery/fallback; terminal lock reason |
| Zero/multiple capability plugin candidates | `UNSUPPORTED(NO_CAPABILITY_PROVIDER)`/`AMBIGUOUS_PROVIDER`; no discovery/fallback |
| Repo path, URL, command, shell token, PATH-only binary, interpreter, `.so`, daemon, or network request | Reject before launch; no side effect |
| Noncanonical/in-repository/unsafe executable, or a rename/symlink/hard-link/replacement race (except an ancestor rename after the containment decision, `ACC-V0-002`) | No execution; `EXECUTABLE_IDENTITY_UNSAFE` or `EXECUTABLE_RACE` |
| Host binary digest/native-object behavior/platform or Core/nonce/plugin/release/manifest/request/target binding mismatch | No launch; host-scoped or closed pre-start reason |
| Caller-supplied snapshot/admission callback | Rejected before callback entry; no goroutine/process created |
| Missing/invalid/unknown/future/unsupported scheme | Exact closed scheme reason; no compatibility approximation |
| Target tuple, no/multiple DNF clause, feature, or dependency mismatch | Whole profile unavailable; no partial answer |
| Output/protocol echo mismatch or hostile extra authority field | Reject output; no candidate |
| Admission rejection | Candidate rejected; exact obligation unchanged |
| Exact verifier unavailable/not run | Preserve explicit `NOT_RUN`/`UNSUPPORTED`; no promotion |
| Timeout, cancellation, child/descendant failure, or any hard limit | Kill/reap; fail whole profile; retain bounded failure receipt |
| Cache miss/corruption/staleness | Recompute only from exact locked candidate or return terminal unavailability; never select another |

## Deterministic acceptance and adversarial matrix

| Vector | Required evidence |
|---|---|
| Signed snapshot, trust epoch, candidate-set, executable, and host-binary digest changes | Exact invalidation; no stale cache reuse or nearest fallback |
| Zero/multiple capability plugin IDs, enforced lock selection, or catalog-order permutations | `UNSUPPORTED(NO_CAPABILITY_PROVIDER)`/`AMBIGUOUS_PROVIDER` or the one locked plugin; never catalog selection |
| Lock missing, duplicate candidate, release alias, path/URL/command injection, PATH shadow | Terminal failure before process start |
| Closed projection omits/reorders/changes an input family, while unrelated repository content changes | Missing/changed relevance is terminal; only the declared projection binds the lock, while full snapshot/input-set digests bind cache and receipt |
| Different host/target tuples and correlated two-clause DNF cross-pair | Only exact correlated clause matches; cross-pair rejected |
| Exact, frozen semver range grammar/inclusivity, missing/invalid/unknown/future/unsupported scheme, wildcard/caret/tilde/open range/prerelease/build edge cases | Only V0 schemes accepted with their exact closed reason and canonical comparison |
| Enabled/disabled/unknown features | Unknown never satisfies a requirement |
| Missing, cyclic, oversized, or cross-clause dependency edges | `TRANSITIVE_PROFILE_UNAVAILABLE`; no partial candidate |
| Cross-Core or cross-nonce replay/echo forgery, stale lock/snapshot, changed plugin/release/build/manifest, or mismatched input digest | Rejection before admission |
| Cache hit after registry signature, trust epoch, revocation, candidate-set, lock, profile, resolver, or clause change | Revalidate before lookup; no cache reuse and no skipped admission/exact verifier |
| Analyzer claims PASS/authority/spec/test state or returns only observed data | Fields rejected/ignored; no promotion |
| Admission pass with exact verifier FAIL/NOT_RUN/UNSUPPORTED | State remains independent and visible |
| Staging-root symlink, group/other-readable stage, path/descriptor identity divergence, or pre-launch rename/hard-link/replacement | No execution; retained descriptor, directory entry, digest, and native object must agree immediately before launch |
| Reused content-addressed stage with divergent bytes, mode, identity, or directory | Terminal race with no execution; the divergent entry is removed so the next invocation restages |
| Darwin `/System` or Data alias, unfiltered Mach service, or ambient environment/service access | Deny by default; only exact staged path, loader, and reviewed service filters are available |
| Native magic-byte-only or ABI-mismatched executable | No execution; complete native object and executable-mapping behavior must match host tuple |
| Deadline/cancellation at authority, root, object, hash/copy, staging, pre-launch, process, protocol, or admission | Pre-start cancellation has no process; post-start cancellation reaps descendants; typed cleanup outcome remains truthful |
| Malformed backend cleanup observation | Terminal `REJECTED` cleanup evidence; never a manufactured `NOT_RUN` |
| Five-warmup/100-fresh-process absolute caps and induced regression | Non-nominated/startup/invocation/cleanup caps and 10% ratchet reported; failure remains `NOT_RUN`/`FAIL` |
| Side-by-side prior release rollback | Exact former lock succeeds or baseline remains explicit unknown; no cleanup dependency |

The Darwin contained Core integration test warms the same staged executable using synthetic
`probe` input only, after Core setup and before authority issuance starts the deadline for each
attempt's single `InvokeAnalyzer` call.
It waits for a successful, valid echo probe using at most 50 ms of the 100 ms cap, within a
separate 60 s test hang detector. Probe attempts are test setup, never production invocation
retries, admission results, or qualification evidence; remaining scheduler headroom is not
promised. Because the warm probe cannot guarantee the next schedule, a typed `TIMEOUT` from
issuance or from the invocation (with a `TIMEOUT` terminal receipt) starts a new test attempt
with a fresh single-use authority inside the same hang detector; each authority still launches
at most once, and every other result fails. Exhausting that hang detector fails the test. A parent sandbox refusal is an explicit
skip, not a measured payload result. `TestFirstInvocationTimeoutIsNotRetried` covers the terminal
first-launch timeout without executing a payload; `TestNewStaticCoreUsesContainedNativeInvocation`
covers the warm-probe precondition and one Core call per attempt.

## Initial adapter-family roadmap

1. **Protocol and registry probe:** freeze canonical request/echo/receipt bytes, signed local
   snapshot fixtures, exact lock parser, process-spy, digest, hostile-output, and reaping vectors.
   No analyzer ships or runs against a real repository.
2. **Go compilation-profile family:** qualify one native Go executable for one exact Go language,
   module/toolchain, host tuple, target tuple, and compilation-unit profile; begin with
   admission-only extracted dependency facts and an explicit exact-verifier boundary.
3. **Manifest/lock family:** qualify one manifest-oriented profile separately, with its own source
   evidence, DNF clauses, dependencies, features, and exact verifier. It cannot inherit Go support.
4. **Additional language/framework families:** add one accepted exact tuple at a time after the
   complete adversarial, performance, offline, side-by-side-release, and rollback matrix passes.
   No family is generalized by resemblance or parser reuse.

## Traceability and promotion/kill criteria

| Requirements | Planned implementation | Evidence before promotion |
|---|---|---|
| `ACC-V0-001..006` | capability resolver, signed local registry, terminal lock | offline trust/lock/provider/path/PATH/release/digest vectors |
| `ACC-V0-002` | source traversal descriptor cleanup | `TestSourceWalkClosesDescriptors`, `TestSourceWalkLstatFailureClosesBoundDescriptor`: depth-three traversal and post-open lookup failure retain zero descriptors with GC disabled |
| `ACC-V0-002` | repository containment by directory identity | `TestRunRejectsRepositoryAndStagingContainment`, `TestRunRejectsCaseAliasedContainment`: an executable or staging parent under a lexical or case-aliased repository root fails `identity-unsafe` with no start |
| `ACC-V0-002` | content-addressed staging reuse | `TestStageReusesVerifiedContentAddressedEntry`, `TestStageRemovesDivergentReusedEntry`, `TestStageKeepsReusedEntryWhenCancelled`: a cancelled reuse keeps the entry |
| `ACC-V0-007..011` | profile model and correlated DNF resolver | host/target/feature/semver/dependency hostile corpus |
| `ACC-V0-007` | unique component identity within a profile | `TestProfileRefusesRepeatedComponentIdentity`: two components sharing `(scopeID, role, ecosystemCoordinate, stableInstanceID)` with different exact values yield `PROFILE_UNAVAILABLE` through `NewProfile`; a distinct instance ID is accepted |
| `ACC-V0-008` | unique clause ID within a manifest | `TestManifestRefusesRepeatedClauseID`: two sorted clauses sharing one ID with different bodies yield `CAPABILITY_PROJECTION_UNAVAILABLE` through `Resolve` |
| `ACC-V0-009` | component and clause-requirement scheme classification | `TestComponentAndRequirementSchemesUseClosedSchemeReasons`: an absent or malformed scheme on a profile component or a clause requirement yields `SCHEME_MISSING` or `SCHEME_INVALID` through `NewProfile` and `Resolve` |
| `ACC-V0-012..016` | candidate one-shot runner, canonical protocol, independent admission | process spy, safe-handle/staging race, projection/echo, injection, admission-vs-exact vectors |
| `ACC-V0-017..020` | candidate scoped receipts, trust-before-cache, limit and diagnostic harness | `TestCleanupObservationIsClosed` rejects open termination and cleanup-error values; failure taxonomy, cache invalidation, reaping, absolute caps, five-warmup/100-fresh-process ratchets |
| `ACC-V0-021..023` | tuple ledger and release rollback controller | accepted profile review, qualified tuple, side-by-side rollback drill |

Kill or disable an affected profile immediately after an unpinned executable launch, repo-path/URL/
shell/PATH execution, network/download attempt, unresolved lock, fallback selection, host/target
confusion, DNF correlation error, escaped bound, unreaped child, protocol echo mismatch accepted as
valid, analyzer-driven promotion, or exact-verifier result falsely inferred from admission. The
baseline remains Core-only with explicit uncertainty.

## Unresolved decisions

- The first owner-approved Go compilation profile and its independently owned exact verifier.
- The local registry storage/signature format and key rotation/retention policy.
- Host-specific resource-control mechanisms and the conservative portable memory bound.

These block profile acceptance, registry selection, installation, admission, and launch, not an
isolated unregistered fact-extractor candidate governed by an explicitly experimental profile. Such
a candidate remains unavailable to Core and cannot establish language support.
