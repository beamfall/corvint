# Build log

Append-only record of material design decisions, independent findings, failed evaluations, and
promotion evidence, newest entry first. Each entry carries a date heading and the requirement or
decision IDs it concerns, so `rg -n '^## ' docs/BUILD-LOG.md` is the index.

## 2026-09-22 V1-0008 IDX-SNAP-V0-022, IDX-SNAP-V0-023, GENESIS-025: init, adopt and index lifecycle qualification

Ticket V1-0008 asked for three things: the two activation doors, the cold-versus-incremental index
lifecycle, and hostile snapshot states, each qualified with evidence. The three requirements are
proposed and record outcomes the code at base `1894b9e5c992d69a7cbefa4b305485494e5622c4` already
produces. No hostile case panicked or ran unbounded, so no behaviour changed. The corpus is this
repository at that base: 3,819 tracked files, tree `7aa62ddc4c6b1afa9c2cc3d9940d7d6c2632d45f`,
object format sha1.

Hosts. The Darwin host reports `uname -m` = `arm64` and `uname -sr` = `Darwin 25.6.0`. `sw_vers`
gives ProductName macOS, ProductVersion 26.6.2, BuildVersion 25G83. It is an Apple M2 Max with 12
CPUs and 64 GiB, running Apple Git 2.54.0. The Linux run used a `golang:1.27.1` container on that
host's Docker VM with `--network none`: `uname -m` = `aarch64`, `uname -sr` = `Linux
6.8.0-117-generic`, Debian GNU/Linux 13, 6 CPUs, git 2.47.3, go1.27.1 linux/arm64. Both
binaries were built from the base tree.

Activation timing (`GENESIS-025`, AC1). Each door ran 20 times on the corpus from a minimal
environment: `PATH=/usr/bin:/bin` and a scratch `HOME`. Network was denied on Darwin by
`sandbox-exec` with `(deny network*)` and on Linux by `--network none`. No model, account or build
step ran, and every receipt records `model.callCount` 0. p95 is the nearest rank, sample
`ceil(0.95n)` of the sorted samples, so the 19th of 20.

| Host | Door | min | median | p95 | max (s) |
|---|---|---|---|---|---|
| Darwin arm64 | `init` | 0.252 | 0.255 | 0.259 | 0.262 |
| Darwin arm64 | `adopt` | 0.252 | 0.254 | 0.257 | 0.259 |
| Linux arm64 | `init` | 0.579 | 0.685 | 0.843 | 0.893 |
| Linux arm64 | `adopt` | 0.459 | 0.633 | 0.774 | 0.783 |

All 80 runs exited 0 with `ok:true` and a `PARTIAL` receipt. `PARTIAL` comes from six `binary-asset`
gaps (`GENESIS-024`) out of 3,813 `INCLUDED` and 6 `UNSUPPORTED` entries. The receipts cite the
revision and tree. The receipt ID was identical on both hosts:
- `init`: `genesis-inventory:sha256:540e45d3ba2f39742fef72632a36b2606e2dfa994444c802486fde896b87f2ef`
- `adopt`: `genesis-inventory:sha256:b7d91a68c0bb1919cfc2eedac3ff13126184dd37bcf65a6e0d385a88a8fe2f8c`

Fallback. The default activation budget is 120 s, well under ten minutes.
`TestActivationFallsBackToABoundedReceiptWhenGitHangs` asserts this. It also uses a Git wrapper that
hangs after repository open, with a 0.5 s caller deadline. Under that wrapper each door returns
within about 0.6 s. The receipt is `PARTIAL`, still pins revision and tree, and carries the single
gap `git-timeout` with zero model calls.

Finding, recorded and not changed: a Git that hangs during repository open yields the `INVALID`
gap `invalid-repository`, not `git-timeout`. `openRepository` maps a failed layout probe to
`invalid-repository`. The outcome is bounded but names the wrong cause.

Cold versus incremental (`IDX-SNAP-V0-022`, AC2). `TestColdAndIncrementalSnapshotsAreByteIdentical`
indexes the target in a fresh clone. It then indexes it again in a clone that first indexed the
prior commit and moved ahead; that clone must probe not fresh before re-indexing. The test requires
the two served indexes to be byte-identical under a canonical JSON encoding with the worktree fields
cleared.

With `CORVINT_LIFECYCLE_CORPUS` set, the corpus subtest used target `1894b9e5`, prior `7e9b1856`.
It passed three of three runs on Darwin and three of three on Linux arm64. The canonical index was
87,658,954 bytes, sha256 `76b96397439184b02f7173942d76dfb87d5d7d35183a4d8b5c6bd0044ffdbfa9`, on every
run on both hosts. The fixture subtest, covering modify, add, delete and rename, runs unconditionally.

Negative result: the gob file itself is not byte-identical. Two `corvint index` writes of tree
`7aa62ddc` with engine `8084efe883cb0fe3` produced 68,770,546-byte files with sha256 `f3d1a145...`
and `9ec03e48...`, because gob encodes maps in iteration order. The spec Non-goals already state
this. A byte-deterministic file encoding therefore stays NOT_PRODUCED behind the
`deployment-neutral-index-platform-v0.md` format gate.

The default path has no incremental build: a moved repository rebuilds in full. The only
incremental path is blob shards (`IDX-SNAP-V0-016`, proposed/off), which is NOT_RUN here.

Hostile states (`IDX-SNAP-V0-023`, AC3). `TestSnapshotLifecycleHostileStatesHaveBoundedOutcomes`
asserts one exact outcome for each state and saw no panic or error return:
- Unsupported input: the exclusion `source exceeds size bound` for a source over 1,000,000 bytes, an
  unsupported-suffix count of 1 for a PNG, and a binary body under an admitted suffix that is loaded
  but not valid text. The snapshot round-trips equal.
- Corruption: an empty file, a torn header, a torn body, a file one byte short and a garbled header
  each produce a miss on load, probe and compact event load. Restoring the file serves the original
  index again.
- Staleness: produces a miss.
- Dirty state: the hit remains, `DirtyPaths` holds only the modified path, the committed body is
  served, and the snapshot directory is unchanged.
- Rollback (`reset --hard` to a retained prior commit): the hit is identical to the original and the
  probe reports fresh.
Focused tests passed on both hosts.

NOT_RUN:
- Linux amd64, and Linux outside a container VM.
- `make gate` and the full-gate required by the ticket (owner policy).
- The blob-shard incremental path.
- Timing under load, and timing on repositories other than this one.
- A hang during repository open in the timed runs.

NOT_OBSERVED: a qualified external corpus.

## 2026-09-22 AFU-V0-001..AFU-V0-012: experimental web flow understanding

The owner requested application-flow understanding, test-gap mapping and runtime confirmation, then
selected a safe web application and authorized implementation. The proposed AFU-V0 contract remains
experimental: pure `flows` reports compose immutable declared input, literal Playwright assertion
candidates and optional guided Chromium observations. `flows record` exclusively writes screened
caller-owned evidence. The separate companion installs no default runtime dependency and performs
no automatic intent acceptance, ranking change, production crawling or complete-flow claim.

The early end-to-end evaluation found the unchecked-save and missing-viewer-test gaps, discovered
eight structural controls and two test journeys in each healthy fixture, and detected both seeded
backend defects. A client cache deliberately hid failed persistence from reload; the independent
backend probe still contradicted it. A renamed-control/route variant, wrong served identity,
source drift, source-only scanning, private recording and HTTP/WebSocket sentinels passed. General
application accuracy, blinded discovery precision/recall, billed tokens and complete-command savings
remain NOT_OBSERVED. Raw task receipts are retained in the private task evidence directory
`/private/tmp/corvint-application-flows-20260922`; final exact-source checks belong to keyed dogfood
observations, rather than being asserted by this preliminary entry.

Independent plan review required a fresh owned-server nonce and frontend/backend/fixture identity.
The first browser attempt exposed incorrect Git executable/working-directory handling; the next
exposed that procgroup deliberately refuses its broader descendant-qualification profile. Both
failed evaluations were retained. The reviewed boundary instead requires non-PARTIAL owned-group
cleanup, completed browser close and observed server exit, with escaped daemon descendants and
non-HTTP transports explicitly unqualified. Final review found concurrent shutdown and ambiguous
route/role evidence; shared cleanup joins pending launches/repeated signals, conflicting route roles
are refused, and role identity stays caller-declared-unverified. The reviewer accepted both repairs;
real Chromium SIGINT/SIGTERM regressions observed all captured descendants retired.
The frozen repository gate then exposed missing root-help inventory and option-like `--root`
handling in the new dispatcher. The repair adds the command listing and reuses the shared root-value
classifier, checked by the existing all-command regressions. An earlier gate attempt hit Git-reader
timeouts; both the isolated cases and their full package subsequently passed unchanged. Failed
attempts remain in the private evidence; source and CEM are rebound before the next frozen gate.

Corvint self-use: pre-change query and initial dogfood attempt used; the initial no-diff/missing-intent
refusals remain visible. `affected` selected the new core and CLI with the full gate still mandatory.
Nonmutating `prove` retained UNPROVEN with twelve citation checks passing and mutation checks NOT_RUN.
The optional flow route was exercised through real compiled binaries in disposable repositories.
Mutation, retrieval experiments, documentation generation, native-host qualification and service
routes are not applicable to this slice; none is counted as adoption or qualification. Bootstrap
intent and partial requirement coverage retain their actual CEM/OCM dispositions. No promotion.

The public tree starts this log at the 0.4.0a4 alpha. Entries written before publication are internal
working records and are referenced from decisions and specifications as historical context only.

## 2026-09-22 decisions 0331 / 0332: 0.6.0 source reapplied onto the public history

The published `v0.6.0` prerelease (build 90, `a03321028e0254bb8d554a2ca1b70e4349568e5e`) was cut
on the history that predates decision 0331. Following that decision, its changes were reapplied,
not merged: the `v0.5.0a3`..`a033210` diff was applied three-way onto public main as one ordinary
commit, so no pre-snapshot commit enters main's ancestry. Four files conflicted only because both
lines added entries at the same place (this log, two agent-memory files, the spec index table);
each was resolved as a union. The contributor agreement, contribution, licensing and provenance
documents and decisions 0330/0331 keep the public bytes exactly. The 0.6 local-workflow decision is
renumbered 0330 → 0332 with every reference; the `v0.6.0` tag and release body still cite it as
0330. Sealed CEM records from the earlier history keep their original commit identities as
historical context. The build count on this history differs from the published build 90, so a
binary built from this commit is not the qualified artifact and requalifies nothing.

## 2026-09-22 decision 0331: clean public history with private provenance

The owner authorized a clean public snapshot after the existing public visibility and surviving
license grants were explained. The publication base is current public main, which contains 258
commits beyond the earlier licensing branch's base; taking that older branch directly would omit
public fixes. Decision 0330 renumbers the reviewed licensing decision because current main already
owns 0321. The agreement and contribution, licensing, and provenance documents retain the reviewed
bytes; only the two affected legal digests change in the current release manifest.

Independent review requires a verified private mirror and local-history bundle, retention of
working files and released assets, a parentless candidate with an exact allowed-path comparison,
and a guarded remote replacement. The final source gate is bound to the snapshot itself; the
older branch's passing gate is not reused as proof for the newer source tree. Historical receipts
retain their original identities and remain historical context when their objects are absent from
the public root. The Git-derived build count restarts at 1 without creating a new binary release.

The existing prereleases, tags, and additional branches await a separately recorded scope choice
before any withdrawal. GitHub-hosted references and third-party copies may survive a ref rewrite.
The prior manual-policy OCM refusal remains visible; no code or test is invented to assert legal
consent. Exact backup, gate, review, and post-publication outcomes are retained privately and must
be reported with their actual status. No source gate or publication success is claimed by this entry.

## 2026-09-22 decision 0330: preserve commercial licensing options

The owner authorized preserving future commercial licensing without changing the AGPL product or
Apache interoperability boundary, then expressly removed the proposed hired-counsel prerequisite.
The prior contribution policy granted only destination-path terms. Agreement version 1.0 now
provides retained ownership, explicit commercial sublicensing, scoped copyright/patent grants,
moral-rights consent within legal limits, successor duties, and authenticated acceptance of exact
text and contribution commits. Apache-only contributions keep the existing path terms.

The owner-directed agent review used Apache individual/corporate CLAs, Harmony's contributor
template, the GNU FAQ, and the actual public licenses (linked in decision 0330). It identified the
need for reciprocal public-license/record-handling commitments, moral-rights treatment, and a
usable acceptance channel. The result requires a contributor PR statement and authorized project
acknowledgment, with private retention of exact text, contribution bytes, and authority evidence.
This is an agent-performed review, not a professional opinion or guarantee of enforceability.

Independent review found that submitted commit IDs alone leave a squash/rebase gap. The repaired
process retains an accepted-to-merged mapping, checks preserved content, and requires rights for
conflict-resolution edits and other additions. Missing grants keep affected product contributions
on hold; no outside lawyer or signing service is required. A further independent review caught a
privacy instruction that conflicted with intentional public electronic acceptance; it now separates
the public statement from private supporting records. Publication is not a signed acceptance,
retroactive permission, third-party clearance, or authorization for a commercial release.

Corvint query and initial coordination were used in an isolated worktree at base
`cd9fec9ae5ed8199ee3c844884ca4212d3e4031c`. Original private receipts retain initial empty-change
and missing-input refusals. CEM citations identify existing license and ownership constraints,
not proof of the new agreement's legal effect. OCM refused this prose decision with
`invalid-requirements-section`; inventing executable test witnesses for consent would be unsound.
Separate non-Go path impact, affected tests, mutation, and ranking evaluations are inapplicable.
The first full gate was interrupted for the owner's material scope correction before source
changed; captured owned descendants were verified gone. The subsequent gate exposed stale
LICENSING.md and PROVENANCE.md hashes in the release-artifact manifest (`legal digest mismatch`).
That failed run was retained and stopped before refreshing only those two hashes to the exact
reviewed file bytes; public LICENSE texts and archive membership did not change. Final check outcomes remain separately
bound to the eventual clean commit. Professional legal approval and contributor acceptances are
NOT_PRODUCED; professional approval is not a required gate.

## 2026-09-22 MTV-V0-001 / SDD-V0-006: literal test anchors for the 0.6 completion

Native finish at 9e57c41 refused with `ocm-bindings-required` after all eight plan checks passed:
MTV-V0-001, SDD-V0-006, PUB-V0-010 and PUB-V0-020 carried explicit `no-test-claim` marks, which
local completion treats as an assessed gap. The owner selected anchoring: the unchanged
`TestToolCatalogueIsExactlyOneReadOnlyTool` body now runs as case `MTV-V0-001 exactly one
read-only tool`, and the existing docs round-trip profiles are named `SDD-V0-006 docs draft and
consume round trip <version>`, following the MCPV0-011 precedent (8d8eed2). No assertion changes.
PUB-V0-010 and PUB-V0-020 have no extractable test and stay unassessed; their no-test-claim
assessment remains in the review record and release packet. The refused finish receipt is retained.

## 2026-09-22 IPR-03: seed data re-pins the reconciled roadmap digest

The exact-target seed-fixture check refused because 9148240 edited the roadmap header, outcome and
historical-next-action lines without refining `script/seed-planning-store-data.json`. The old pin
is the digest of 9148240^, and all eleven IPR-01..IPR-11 sections the ticket bodies copy are
byte-identical at the new digest, so only `expectedDigestSha256` changes; no ticket text moves.
The failed seed-fixture receipt at a9854aa is retained.

## 2026-09-22 PPI-V0 / decision 0332: protected Pi source joins the 0.6 candidate

The owner directly approved one replacement of the integration enrollment to include the reviewed
protected Pi source as optional experimental FALLBACK. The 19fb7ea generation, its maps and reports
are preserved; it was cancelled once as NON-SUCCESS. The contract commit landed first because Begin
pins every intent at HEAD, then one Begin under the same key adopted the independently reviewed
thirteen-scope, eight-check plan. The runtime and lifecycle commits and the MCPV0-011 literal anchor
follow as source only; every runtime blob matches its reviewed commit, and only build-log, spec
index and citation metadata differ. The added Makefile target shifted three citations, which now
point at the same cited content; the unanchored compat-replay citation had already named the wrong
line and now names `spec-requirements-check`. This authorizes no protected installation, principal
admission, activation, FULL claim or compiled runtime distribution, whose notices remain incomplete.
The earlier entry that excludes the protected runtime records the previous generation's scope.

## 2026-09-22: selected completed source enters the 0.6 candidate

The owner selected the completed installation/recovery, candidate portable-proof, experimental
provider-kit and native Pi tool slices for 0.6, retaining their original qualification limits.
Their source commits, the exact-content gate repair, browser disclosure proof repair and MCP
empty-PID-file fixture repair are integrated without importing another task's CEM or local outcome.
Protected Pi runtime and formal FULL authority remain excluded. The previous integration enrollment
is retained as cancelled non-success; its single replacement keeps the existing base/checks and
adds the four owning scopes plus gate-ledger and releasecandidate race coverage.

Independent combined-source review found no blockers. The early metadata preflight found citation
line drift from the additive host/Makefile changes and an unanchored failure-backlog citation;
relocations preserve the exact cited content and historical references retain their named commit.
Corvint query, path impact and affected planning supplied change context; the unavailable first
impact path remains a retained refusal. Learning and provider ingestion are excluded from this
integration evidence. Final source/CEM binding, exact-target checks, artifacts, native/installed
hosts and the separately governed workflow campaign remain required; no slice result promotes 0.6.

## 2026-09-22 MCPV0-011: lifecycle fixture waits for PID publication

The c0f1eee full gate failed `TestClosedStdoutCancelsInFlightDescendantGroup` with `<nil>` at its
PID-file wait, before the stdout-close and descendant-cleanup assertions. The fixture's shell can
create its PID file before writing the bytes; the reader treated an existing empty file's nil
error as a fatal error. A focused empty-file-to-complete-publication regression reproduced that
failure immediately. Only non-nil unexpected read errors now fail the wait; empty or absent files
keep the existing bounded retry. Malformed PID data, deadlines, process cleanup and MCP runtime
behavior remain unchanged. The regression joins or stops its delayed writer during cleanup.
The failed full-gate receipt is retained; focused lifecycle validation and a fresh exact-candidate
full gate are required, with no scope or enrollment replacement and no relabeling of old evidence.

## 2026-09-22 PPI-V0-005..009: isolated Pi authority source and packaging

The experimental Pi lane adds closed root/3, campaign/2, qualified-pi-host/0 and QLF/2
without changing earlier profile meanings. Both native surfaces require separate evidence
bindings to one admitted image. Runtime checks bind protected full-image bytes, live hardened
code flags/CDHash, immediate parent and process birth, boot, OS/architecture and actual cwd.
Non-Stop reads retain their protected-publication privacy boundary. Candidate campaigns remain
900-second FALLBACK/UNQUALIFIED exercises; they cannot supply completed qualification.

The fixed SDK embeds its consumer hash, checks protected ancestry before spawning, verifies
closed receipts and waits for native idle completion before one permitted follow-up. Independent
review found stale Stop reuse after trust withdrawal, lost ordinary context on missing admission,
and silent recursive unresolved state. Fresh epoch-bound receipts/current trust, separate FALLBACK
requests with explicit degradation, and visible bounded unresolved status repair those findings.
Repair review found no additional issue; cancellation regression confirms no surviving descendant.
Focused Pi/Direct/Qualified Go tests and the rebuilt native TUI/RPC/reload/replacement/image/startup
suite pass. The final handler repair's rebuilt native run is still required at source freeze.

Optional Pi release preparation binds the host, consumer and build manifest in a distinct immutable
release profile. The installer refuses another host's admission; exact-file removal and revoked
reader withdrawal remain explicit. Focused packaging and the optional authority module's ordinary
suite pass; independent packaging review found no actionable issue. No protected state was changed.
Full gate, license/input closure audit, privileged installation, real protected OPEN/EMPTY/recursive
campaign, latency/recall and independent FULL admission remain NOT_PRODUCED/NOT_RUN.

## 2026-09-22 PPI-V0-001..004: closed Pi SDK runtime proof

The owner accepted the reviewed optional protected Pi direction, separately from privileged
installation or completed qualification. Bun standalone executables still accepted executable
`BUN_OPTIONS`/`BUN_BE_BUN` injection despite configuration-autoload switches. The implemented
alternative uses official Node22.23.2 SEA, fixed exec arguments, hardened runtime with only JIT
permission, exact Pi0.85.1 dependencies and embedded assets/worker/WASM. Ordinary Pi remains separate.

Native RPC and TUI prompts, reload, session replacement and a real image read/resize pass while
macOS denies reads of both global and build-time SDK modules. Hostile project/global extension
files remain unexecuted. NODE_OPTIONS, forged argv0, CLI eval/preload, DYLD and OpenSSL injection
negatives pass; executable Node/DYLD/OpenSSL controls prove the canaries work. SIGUSR1 does not
activate the inspector. Harness interruption reaps a TERM-ignoring descendant. These are local
source/runtime checks, not an admitted campaign or completed native qualification.

Independent review identified unresolved lazy OAuth/Bedrock imports and inherited argv keys;
static provider bundling and own-key argument parsing repair both. The repair review found no
additional issue in that boundary. A subsequent async credential-write audit found validation ran
before a Promise resolved. The guarded backend now validates after awaiting, rejects command keys
and credential environment overrides before storage, and preserves literal keys without SDK
interpolation. Explicit nonpersistence and literal-dollar regressions cover the repair.

Failed test attempts remain evidence: the TUI driver initially submitted reload before completion;
the image fixture initially exceeded the harness output cap and incorrectly selected repeated
tool calls. Corrected fixtures wait for visible readiness/bounded completion and use a tiny image
that still requires resizing. No production output limit was weakened. The full gate remains
coordinator-held. Pi authority profiles, independent admission, latency/recall, actual protected
OPEN/EMPTY/recursive behavior, installation and revocation remain NOT_PRODUCED/NOT_RUN.

## 2026-09-22 AHI-025: explicit Pi operations and full-support direction

The owner explicitly requested complete Pi support, including protected authority and formal FULL.
The additive functional slice reuses native context, immutable source-view validation and the
explicit trace writer. It adds bounded in-memory packet handles and typed supplied observations,
without changing legacy lifecycle authority or making automatic outcome writes. Existing enrollment
and its failed canonical query-fixture restoration check remain visible; the next source invalidates
prior checks. The functional slice is independently reviewable and remains separate from release
0.6 integration until selected. Native and focused regression results are retained in the Pi task
checkpoint; unrun qualification stays unclaimed.

Independent review found two functional defects: the explicit slash command discarded uncertain-
write guidance, and the context tool failed to forward its supplied evidence handle. Both are
repaired and the repair-only review found no remaining required issue. Focused native Pi/source
checks, 21 JavaScript regressions, and four actual Pi host/cleanup checks pass. Actual host evidence
includes print-mode query/expansion/edit/verification/explicit recording, RPC new-session recovery,
and native TUI prompt/shutdown. Canonical verification is queued against the frozen next target;
these focused results are not a full gate or protected qualification.

The protected-runtime feasibility review found the installed Node/JavaScript Pi cannot inherit the
Codex-only direct admission. The official Pi 0.85.1 standalone darwin-arm64 archive is a concrete
candidate, but immutable mapped code, external extension/resource closure and actual native launch
must be proved before a Pi-specific technical profile or execution-root admission is accepted.

## 2026-09-22 EEP-V0-016/017/018: frozen kit gate failed, no retry

The one enrolled full gate on `b835a7464836ee8d25ea828de0f2921ad672a254` failed with exit 2
(no timeout or cancellation). `internal/specindex` rejected the kit's overlong INDEX claim and
nonidentical INDEX/digest/README metadata. The repair restores the original bounded claim and
synchronizes the delivery/status copies; requirement semantics and executable source are unchanged.
The existing `TestIndexCoversSpecsAndHeaders` is the focused regression for that repair.

The same run separately failed the existing Core
`TestRepositoryQueryTraceStateFailuresAreTypedAndNonmutating/oversized` at the pre-query
`repositoryBytesDigest`: a temporary Git pack index disappeared during `lstat`, followed by a
TempDir `.git` directory-not-empty cleanup error. Cause remains UNKNOWN; no isolated retries or
Core repair were performed in this kit slice. Full `internal/extevidence` and `tools/gate-ledger`
packages passed in this run, which is distinct from the portable-proof gate-ledger failure.
The frozen logs and leftover fixture are retained in the private task checkpoint. Recorded gate
process handles exited. No further full gate is authorized; final completion and seal remain
blocked. A metadata repair does not turn the failed frozen gate into PASS.

## 2026-09-22 EEP-V0-016/017/018, EEP-TR-011: experimental local provider authoring kit

V1-0027 adds kit 0.1.0: a single-file standard-library Go provider and a separately built checker
that reuses existing strict record decoders and contained command execution. Consumer pins bind
exact schema, provider identity, repository revision/root and command executable SHA-256. No Core
flag/wire/version, authority, installation or transport promotion changes. The experimental record
window is exactly `/0`, `/1`, `/2` with today's consumer, not historical engine compatibility.
Provider/consumer examples and implementation remain AGPL; no Apache boundary expansion.

The smallest complete proof copied and authored the provider in scratch, built it offline with
local Go 1.27.1, and compared its `/0`, `/1`, `/2` file and command composition. The documented
focused command passed across kit, extevidence, procgroup and Core packages, including the existing
valid/stale/malformed/ambiguous/repository-mismatch/unsupported cases, exact-pin refusals, Core
separation, timeout/output/environment bounds and descendant interruption cleanup. An initial
fixture expectation used `app` where the retained fixture declares `application`; correcting the
test restored agreement. Independent Sol/low review found no HIGH/MED; stale digest wording was
corrected. The inherited test fixtures remain synthetic, not external validation.

V1-0013's portable proof freeze (itself awaiting V1-0010), incomplete native ticket coverage and
owner acceptance remain open. Kit MCP integration stays proposed/out of scope without downgrading
the already accepted separate MCP profile. Full/interop gate and CEM/OCM completion evidence are
produced after the source freeze; this entry does not claim those pending gates passed. Pre-change
dogfood at the empty base-to-HEAD range retained `cem-prepare git-diff-failed`, missing CEM/citations,
missing intent scope and missing outcome inputs. No billed-token or before-first-query measurement
was available; no efficiency or promotion claim follows. OCM correctly rejected hyphen-adjacent
requirement IDs in the new test labels as non-exact anchors; labels now use whitespace delimiters.
The source target was refrozen before any full gate.

## 2026-09-22 V1-0013 / CEM-CB-003: candidate portable packet finds cross-hunk interop defect

The candidate `protocol/cem-0.2` packet pins a synthetic SHA-1 base, six exact target commits
including their raw CEM sidecars, and 21 artifacts. Formula-derived expectations cover every drift
state, ordered mixed evidence and explicit unknowns. Native status verifies canonical authority and
sidecar bytes and refuses zero-unknown completion for the unknown case. The separate historical
reader rejects 0.2 and consumes independently decoded, pinned 0.1 exact-patch equivalents; the
original frozen 32-case matrix and wire profiles are unchanged.

The first historical-reader run rejected legal evidence/relation reuse across different hunks as
`duplicate-basis`. Its pair set lived outside the hunk loop. Independent contract review confirmed
that supported-hunk uniqueness is local to the hunk; moving the set preserves same-hunk rejection.
The supplemental process-boundary regression exercises both cases. This is reference portability
evidence, not independently authored 0.2 interoperability or a semantic-support claim.

Pre-change enrollment uses base `ab5310cb4f00d15c33fe112c0e0335fe28f9db20`. The original Corvint
query returned an unrelated accepted-spec lead and explicit omissions; scoped original contracts
supplied context. Initial `dogfood-change` retained `git-diff-failed` because base and target were
identical before implementation, missing intent-file input and absent outcome input; the corrected
enrollment retains the actual CEM intent. Measurement-before-first-query and billed task costs are
NOT_OBSERVED. `affected` retains nested-module, unowned-vector and language-frontier unknowns.
Applicable routes are query, affected, CEM/OCM, frontier and enrolled completion. Provider, mutation,
learning/retrieval evaluations and service routes are not applicable to this packet/reader repair.

Focused native and independent-reader vectors passed after the repair; the integrated independent
diff review found no blockers. Full/interop gates and
final CEM/OCM/report review are required on the final committed target; retain their exact receipts
in the task's private evidence rather than treating these focused passes as those gates. The native
store remains fixture-only with V1-0010 open and coverage incomplete. Formal minimum-wire freeze,
independently authored 0.2 consumer/producer, OCM/frontier portability qualification and owner
acceptance remain blockers. No version, release candidate, publication or runtime authority changes.

## 2026-09-22 PUB-V0-023/025/026: V1-0017 operational prerequisites

The owner's parallel V1-0017 instruction authorizes a prerequisite slice, not ticket completion or
future-release qualification. Native ticket audit retained V1-0008/V1-0015 OPEN, coverage INCOMPLETE,
and actor authentication, historical acceptance, runtime qualification, liveness and publication
NOT_OBSERVED. The initial native query abstained below its relevance floor; targeted Go impact and
original installer/spec sources supplied context. No current release candidate or user installation
was modified.

Audit found that candidate and installed version probes used unbounded `exec.Output` without owned
descendant cleanup, and installation followed static symlinked store components. Existing procgroup
supervision now bounds both probes and suppresses child output in errors; store admission refuses
symlink components, aliases, overlap and invalid existing paths before effects. Candidate input
materialization is bounded to its existing closed set. Independent plan review rejected WalkDir's
unbounded pre-callback enumeration; bounded ReadDir fixes that before implementation. Initial focused
fixtures caught sibling checks extending into a large unrelated temporary ancestor; checks now cover
the store name and managed descendants. No new release format, service or persisted-state migration
was introduced. Removing this change restores the preceding installer; retained candidates and
older installs need no migration.

Temporary shell/archive fixtures exercise coexistence, explicit rollback, corrupt-destination
refusal, backup reinstall, scoped removal, hostile paths, bounded probes and joined interruption.
These are mechanism evidence, not genuine future release artifact/native-platform qualification.
Focused package tests passed (2.015s), race tests passed (3.543s), and focused vet plus specification
checks passed. Independent final review found one test-only PID-reuse cleanup hazard and one
missing exact-version/nonzero-exit fixture. Identity-bound cleanup after cancel/join and the new
fixture passed targeted race tests (3.414s); the same reviewer accepted the repair with no remaining
findings. The release orchestrator explicitly holds the terminal full gate behind current release
qualification. It remains NOT_RUN here; native finish/seal and exact future-artifact qualification
remain pending. Exact source/CEM/OCM handles are retained privately for continuation. See
[the runbook](RELEASE-RUNBOOK.md) for remaining artifact/platform, support policy and predecessor
requirements. Existing alpha security policy is preserved; stable promises remain drafts. Store
ownership is exclusive; concurrent hostile renames, escaped groups and power-loss durability are
explicitly unqualified.

## 2026-09-22 PUB-V0-020 / GL-V0-001: approved patch qualification repairs

The owner approved the combined repair packet and exact replacement enrollment, retaining the
original enrollment as cancelled NON-SUCCESS and preserving all failed receipts. The patch adopts
only the reviewed statless-index source and requirement-anchor delimiters. Candidate c85881a's
installed browser proof omitted opening the existing roadmap disclosure before checking its text;
the corrected proof clicks that disclosure, retains all six assertions and reports missing text.
Primary failures are logged before unchanged cleanup. The temporary corrected browser proof passed
but does not qualify the original candidate. The prior installed attempt with a mode-0644 local
authority attachment was also retained as a failed runner setup; its corrected attachment is 0600.
A combined-source review, fresh immutable CEM, all selected checks and new source-bound installed
and OpenCode qualification remain required before exact-packet publication approval.

## 2026-09-22 GL-V0-001/005: exact bytes without cached index stats

A deterministic fixture reproduced a false ledger HIT and stale `one\n` blob after a same-size
`two\n` edit with restored mtime under coarse Git stat settings. The newer-index variant also
failed, so preserving only the copied index timestamp is insufficient. The original full-gate
failure's exact timing/configuration remains unknown; its failed source/enrollment is preserved.
Independent plan review selected a fresh private index imported from Git's NUL-delimited
mode/object/stage/path entries, retaining tracked membership while discarding cached stat data.
Private commands disable fsmonitor and ignorestat; the original index bytes and mtime stay intact.
Tests cover restored timestamps, ignored tracked and intent-to-add content, staged changes,
deletions, unusual paths, executable/symlink modes, unborn and linked worktrees, refusals and
failed-import cleanup. An initial cleanup assertion included Apple's unrelated `xcrun_db` cache;
it was corrected to assert only the ledger-owned private index/lock names. Corvint pre-change
query/impact, dirty affected/path impact and enrolled CEM/OCM are the applicable self-use routes;
learning/evaluation/provider routes are not applicable. Focused checks qualify this repair only;
the mandatory full gate remains pending on the coordinator's frozen integrated Core target.

## 2026-09-22 PUB-V0-001: prepare the 0.6.0 candidate version tuple

The owner-selected candidate moves the native version, archive smoke, VS Code exact admission
and live fixture together. Existing tuple and historical-identity tests carry explicit PUB-V0-001
claim anchors for OCM review. The draft notes retain UNPROVEN jobs and pending candidate gates;
no historical evidence, first-parent build calculation or optional alpha parser changes.
Independent review caught stale executable-test fixtures and the VSC-V0-007 admission clause;
the repair updates both, the extension README and the selected editor checks.

The retained prechange query located accepted decision 0072, with four ranked results omitted
and test symbols withheld; original sources and accepted decision 0332 supplied scope. The initial
empty-diff coordinator refused CEM preparation, as expected. This slice uses query, affected,
CEM/OCM/frontier and keyed completion; native/archive/host and sealed measurement are deferred
until the integrated candidate freezes. No paired savings or milestone qualification is claimed.

## 2026-09-22 decision 0332: Core candidate and evidence packet clarification

The accepted Core-only 0.6 scope now distinguishes immutable tested candidate `T` from
later evidence-only publication snapshot `E`. The public-release contract maps the
retained `PUB-V0-022..026` Core safeguards to existing native archive/report/checksum
inputs and keeps optional combined alpha machinery separate. The daily-workflow
acceptance and rollback clauses retain all six evidence classes, exact candidate
invalidation and literal archived `UNPROVEN` claims. This is a development contract
clarification, not candidate qualification, ledger promotion or publication.

## 2026-09-22 decision 0332: 0.6 portfolio and native status reconciliation

The 127-entry index and installed 0.5.0a3 build 45 command help were inventoried into
[the 0.6 portfolio](PORTFOLIO-0.6.md); the private coverage comparison retains both inventories.
Core is a restricted qualification target, not delivered intent or promotion. An initialized native
`.taskman` store supplies current execution status in the workspace where it exists; this checkout
has none, so the historical AT/E/U roadmap cannot serve as a live queue. The original directory's
fixture queue is `queue:corvint:main`; no V1 ticket completion is inferred. The broad integrated
outcome remains historical owner intent outside the accepted 0.6 Core prerequisite set.

Prechange query and full-base range impact returned receipts. Keyed enrollment returned
`operation-in-progress` while keyed status was inactive; initial `make dogfood-change` could not
write the linked Git directory under the sandbox. The development worker continued after that refusal, so its full workflow trial is incomplete.
The coordinator preserved the exact patch and restored its owned files before retrying enrollment.
The sealed prior worktree then correctly refused stale prior completion; a fresh isolated leaf was
enrolled from the sealed base before replaying the patch. No lifecycle record was removed or relabelled.
Command-owner mappings and supporting-gate classifications were corrected before review.
The worker's five focused documentation checks passed. Fresh Astra/medium review found no HIGH/MED
findings after the repair; final binding and enrolled checks follow. Raw worker/reviewer usage and
all refusals remain private development evidence, not sealed cost or savings evidence. The final
enrolled check exposed four shifted roadmap line citations; each was moved one line after an exact
old/new passage comparison, preserving the original AT-09/10 meaning and the failed check.

## 2026-09-22 DCW-V0 / UCV0-013: owner-selected 0.6 local-workflow scope

Decision 0332 records explicit owner acceptance of three narrowly scoped daily-workflow jobs,
retaining six evidence classes and separating installed Codex/Claude use from formal FULL authority.
The canonical ledger moves to `/1`; exact historical `/0` admission and bytes remain supported.
All twenty-two rows remain specified/UNPROVEN. No milestone, host or platform is promoted.

Independent Gate A review found the closed-ID compatibility risk, alpha-only companion-required
candidate admission, and freeze/enrollment sequencing constraints. The first slice resolves the
ledger contract and enrolls existing intent before editing; later candidate and evaluation work
remains required. Original query on the dirty primary abstained with unindexed-worktree-changes.
Initial no-diff dogfood preparation reported git-diff-failed/missing-intent-scope; enrollment then
exposed the required lexical intent order and succeeded after canonical ordering. These failed
attempts are retained in the private task evidence, not reclassified as successes.

Corvint feature routes used: query/index, tracked-Go path impact for the validator/tests, dirty-change
affected advice and keyed dogfood. The affected advice retains the terminal repository gate and
unknown scope; it does not replace that gate. No ranking change, sealed corpus access, mutation
trial or optional service is needed for this slice. Final CEM/OCM/frontier inspection remains required.
Focused conformance tests and the spec/requirement/traceability/decision/citation checks passed.
The actual historical/current reader matrix accepts historical `/0` in both readers, accepts `/1`
only in the new reader, and observes explicit `wrong-spec` from the old reader. Independent
implementation review found no blocker; its documentation corrections are included. No full gate,
workflow qualification or savings claim is reported.

## 2026-09-22 PUB-V0-001: v0.5.0a3 published

Tag `v0.5.0a3` points at `822888a`, the merge of pull request #59 (`codex/release-v050a3`), and the
GitHub prerelease carries the four non-Windows archives plus the gate-produced `SHA256SUMS`, all
built from the gated candidate `f441e96` with digests equal to the private archive witness
(decision 0329, publication record). Evidence chain: `make gate` exit 0 on clean `f441e96`
(Go-archive `verdict=PASS`); CEM bind `f441e96` (116 supported hunks, OCM linkage 0/26 unknown,
recorded as `NOT_OBSERVED` for the base window); `dogfood-check` PASS; seal `cc8792a`; independent
review with no blocker. Two limitations are recorded rather than hidden: the local trace record
`.context-corvint/traces/6b50c2e7....jsonl` from a discarded earlier bind commit was moved out of
the worktree so the rebind could read the trace store (`unsupported-query-trace-state`), and the
`cmd/corvint/query_test.go:139` flake seen once on the #56 gate did not recur and remains unproven.
Issues #53, #54, #55, #56 and #57 closed on merge. `v0.5.0a2` is unchanged.

## 2026-09-21 audit fix batch: thirteen defects closed before the v0.5.0a3 candidate gate

A pre-release audit of `cmd/corvint`, `internal/contextindex`, `internal/jstestprovider`,
`internal/behaviorfalsify` and `internal/doccorpus` recorded fifteen defects in
`docs/agent-memory/bugs.md`; thirteen are fixed on the candidate, each with a focused regression
test. Contract-visible changes: `dogfood-ocm`, `taskman-fixture` and `corpus` emit an
`output-failed` envelope when their own output cannot be written; the `corpus` `--root` preamble
refuses an option-like value (other native values stay uninterrupted because DCP-V1-012/018 require
`--task --corpus=x` to pass through); BBF-V0-010 now states that the declared wall-clock budget must
exceed the executor's cleanup reserve, and the per-receipt output limit has a floor equal to the
validator's accepted receipt bound; JLTP `report-not-written` also covers a Vitest report over the
4 MiB bounded-report limit and external config input over 256 entries is refused with
`report-output-overflow`; sensitive-input redaction orders values longest-first, and the
`sensitive-input-finding-bound-exceeded` slot-63 overwrite is retained as the spec-listed cap
behavior rather than treated as a defect; DCP-V1 reverse-link keys use a NUL separator, so
`lost_reverse_links` strings now carry `\u0000` between their parts, `missing-reverse-link` detail
ends with the test ID, and `artifacts()` refuses an observation whose input is unretained, whose
digest differs from its run, or whose input already serves another artifact role. Context-index
production edits move the analyzer schema to `corvint-analyzer/73` with a re-pinned input digest.
Not fixed: the DCP-V1-032 `--previous` refusal of a bundle carrying a real reconciliation finding
is an owner decision (`docs/agent-memory/questions.md`), and the `CPUPROFILE` read-command write
knob stays in `docs/agent-memory/fixes.md`. The 32-bit symbol-window overflow fix is confirmed by
inspection only; no 32-bit build was run.

## 2026-09-21 PUB-V0-001: v0.5.0a3 candidate integrates issues #53–#57

Decision 0329 moves the version tuple to `0.5.0a3` for the rerelease that integrates the sealed
issue branches #53, #54, #55, #56 and #57 plus `main`'s gate ledger. The token is the next alpha
increment; it awaits the owner's confirmation before any tag is pushed. Each issue branch carries
its own enrolled local-completion or recorded gate evidence and a sealed CEM under
`.corvint/changes/`; the integrated candidate must additionally pass the full repository gate on
its exact clean commit before publication. Unsigned prerelease, `NOT_VERIFIED` publisher identity,
four non-Windows archives plus gate-produced `SHA256SUMS`, no companion, and `NOT_RUN` live
Playwright `/2` and external MCP host qualification are retained unchanged.

## 2026-09-21 GLTP-V0-048/049: lifecycle test deadline and joined shutdown

`TestRunningFailedPassed` used the production-like fresh `GOCACHE` with both its runner and outer
terminal-event waits fixed at 20 seconds. The observed full-suite timeout is consistent with cold
compilation exhausting that budget. A fatal wait also cancelled the session without joining `Run`,
allowing temporary-file cleanup to race the runner. The test harness now sets the specified
`GOENV=off`, uses a five-minute
per-run hang-detector budget, fails immediately with the complete event sequence on an unexpected
terminal state, and unconditionally cancels and joins `Run` before `TempDir` cleanup. Production
defaults and session behavior are unchanged. The complete session package passed in 35.808 seconds,
and ten serial repetitions of the exact lifecycle test passed in 39.069 seconds.

## 2026-09-21 LTA-V0-004: verbose Go PASS marker exception

The pre-change Corvint query selected an unrelated decision and omitted the governing writer-screen
intent. Direct inspection found that the generic bare-`pass` assignment branch classified exact Go
verbose-test marker lines as secrets. The writer screen now masks only the structural `PASS:` prefix on complete
`[whitespace]--- PASS: TestName (seconds)` lines while still detecting a real `pass: value`, token, or other secret
inside the test name or elsewhere in the same output; `StoredV1Pattern` is unchanged. `TestGoVerbosePassMarkerBoundary` and the
local-completion `go-verbose-pass-log` regressions cover detector and executed-check behavior.
Because the writer-screen source is an analyzer input, the reviewed change advances
`analyzerSchemaID` from `corvint-analyzer/69` to `corvint-analyzer/71` and refreshes its audit pin.
## 2026-09-21 BBF-V0-001..012: criterion-level browser behavior falsification (issue 54)

The experimental `corvint-behavior-falsify` companion separates deterministic planning from exact
digest approval, stages caller-owned argument-free hooks, and runs them under bounded process-group
containment in a caller-marked disposable workspace. It records contract/criterion/assertion,
application/test/documentation revision, runner/browser/config/environment, perturbation,
attempt/retry, cleanup and artifact identities. Only the expected assertion failure with unrelated
criteria and setup still passing can classify `killed`; selector errors, unrelated failures, retry
masking, stale bindings and cleanup drift are invalid, while process/timeout loss remains
`infrastructure_failed`. The report retains all six raw statuses and always preserves
`full-relevant-suite` fallback.

Focused `go test -count=1 ./internal/behaviorfalsify ./cmd/corvint-behavior-falsify` and matching
`go vet` passed; the same packages also pass `go test -race`. The synthetic matrix covers the expected kill, tautology, hidden duplicate,
wrong-value survival, wrong assertion, unrelated failure, selector error, timeout, cleanup failure,
retry masking and stale revision/perturbation/artifact identities. A staged live test helper produced
and then cleaned a retained artifact with equal pre/post workspace digests; a separate one-second
timeout proved owned process-group cleanup and workspace restoration; missing descendant-observer
evidence remains infrastructure rather than success. These are authored synthetic fixtures, not a
real adopter or browser run. Hook semantics and absence of persistent external effects remain
caller-owned and unauthenticated; live utility is `NOT_OBSERVED`.

Independent review found cancellation could schedule untouched cleanup hooks, wall-clock accounting
did not reserve both process shutdown windows, process stdin retained a lower hidden default, plan
controls were duplicated, report/receipt output was not aggregate-bounded, infrastructure receipts
accepted contradictory caller strings, and the initial acceptance matrix was incomplete. Repairs
stop after the interrupted attempt, count only started controls, reserve hook/cleanup shutdown and
execution time, isolate and bound Git reads, divide the report budget across approved attempts, use
one non-HTML-escaping deterministic JSON encoding, close infrastructure reason/shape validation and
add live crash, overflow, cancellation, slow-termination, cleanup, stale-artifact and JSON-expansion
regressions. The final independent re-review returned `PASS`.

The first frozen canonical gate exposed two local evidence defects. Under concurrent host load the
timeout regression obtained owned process-group cleanup and restored the workspace but a transient
`ps` snapshot was unavailable; the already-fail-closed infrastructure result is now asserted without
turning observer availability into a test prerequisite. The new specification also used a prose
`Boundary` digest bullet instead of the required `Exists` and `Blocked on` fields, and its README
clause differed from the indexed claim. The digest and README now share the exact indexed claim.
The replacement gate passed the full suite, vet/cross-vet, archive and interop before detecting the
resulting stale requirement line numbers; `REQUIREMENTS.tsv` was regenerated from the repaired spec.
The next frozen gate passed those checks plus spec, traceability, EOL, CI, release and receipt policy
before the error-code ownership tail found the new `approved-plan-drift` code unnamed; the owning
spec now records that code and the shared authorization code explicitly.

Dogfood orientation exposed two limitations retained for review: the initial limit-one query ranked
the Go-kernel migration spec rather than the behavior-contract seam, and the later focused context
packet reported captured index revision `70ffae556ba8cecc499501a492b9051485310759` rather than the
worktree HEAD. Exact repository inspection found `documentation-corpus-v1.md` and
`internal/doccorpus/behavior.go`; no completeness claim is made for the stale context packet.

The first committed CEM and full gate passed, but enrolled `finish` exposed a distinct
traceability miss: Go test function names normalized the requirement numbers and yielded no exact
`BBF-V0-###` OCM claim anchors. Requirement-labelled test cases now bind the existing behavior
assertions, with added closed-vocabulary and report-limitation checks. The first enrolled full test
failed in unrelated process/timing tests under concurrent repository-wide runs; a clean serialized
retry passed. Both observations remain in the private completion evidence rather than being
reclassified as product behavior.

## 2026-09-21 GL-V0-001..GL-V0-008: gate ledger, one pass per distinct content

The owner asked that Corvint manage its own gate so parallel agents stop each running a full
`make gate` on content another worktree already proved. `docs/specs/gate-ledger-v0.md` is accepted
and implemented: `tools/gate-ledger` keys every step on a digest of its declared inputs over the
exact worktree (tracked and untracked, ignored excluded, written through a private Git index) plus
the gate's tooling, records only passes in a per-user `0700` directory, and skips a step only on
key equality. `go-archive-gate` always runs because GOC-V0-010 binds its witness to HEAD, and every
doubt (undeclared step, skip-worktree or assume-unchanged entry, ignored compiled `.go`, git
failure, shared ledger directory) runs the step and records nothing.

Independent finding that changed the design: Go's test cache never hits across worktrees, even
with `-trimpath`, because its test log hashes the absolute paths of files a test opens under the
module root (measured on this host with a second `git worktree` at the same commit). The proposal
had assumed the cache would deduplicate resolved packages across worktrees; it deduplicates only
same-worktree reruns. So `ledger/go-test` (GL-V0-004) runs the 93 packages the affected-plan index
resolves without `-count=1`, under Go's cache, and the 104 unresolved packages (`cmd/corvint`
among them, for one `os.Getwd`) with `-count=1` under one record keyed on the whole tree. A tree
change therefore still reruns the unresolved set; narrowing it is the affected tier's job
(`affected-plan-v0.md`), and a per-package cross-worktree key is recorded as a follow-up in
`agent-memory/ideas.md`. `make go-test`, `make gate-affected` and CI keep `-count=1` unchanged.

Measured on this host (Mac Studio, `-p 1`), from the baseline `go test -json ./...` at the base
commit: 197 packages, 2,167s of package time, of which the 104 unresolved packages take 1,634s and
the 93 resolved ones 533s; `cmd/corvint` alone is 174s. The partition on this tree lists 93
resolved and 105 unresolved packages (the module root counts once more than the baseline's
package list). Three cheap steps run through `ledger/` twice: the first pass ran and recorded all
three in 3.7s, the second hit all three in 2.0s, so the per-step ledger cost (worktree digest plus
`go run` start-up) is about 0.65s. `plan` prints `go-archive-gate: always runs` and `RUN` with
`no declared input scope` for an unknown step. A `go-test` run whose unresolved set failed (the
host-adapter test reading a pre-existing dirty `plugin.json`) recorded nothing, as GL-V0-002
requires. Full gate, measured twice in a clean `git worktree` at `d6626ae` with an empty ledger
directory: the first `make gate` ran and recorded all 28 keyed steps in 1492s and recorded the
receipt; the second hit all 28 (the full `ledger/go-test` among them), ran only `go-archive-gate`,
and recorded the receipt in 173s. The first attempt at `22208f6` found two defects the unit tests
had not: a linked worktree's index path is absolute, so the private-index copy was empty and no
step recorded (fixed, `TestRunStepRecordsFromLinkedWorktree`), and the Windows cross-vet rejected
`syscall.Stat_t` and `syscall.Flock` (fixed by build-tagged `platform_unix.go`/`platform_other.go`;
a non-Unix host refuses the ledger directory and records nothing).

## 2026-09-21 SEG-018..SEG-021: typed semantic choice decisions

The owner directed Corvint to adopt the useful typed-decision ideas from TypeSafe AI's System One
model announcement without adding Jev or another hosted dependency. The unwired
`internal/semescalate` experiment now has a separate provider-neutral choice path over mechanically
supplied anchored options. Providers return only an option ID and exact integer probability mass;
Core derives an explicitly uncalibrated winner margin, supports a reserved abstain option, rebuilds
the immutable proposal, and still requires the registered verifier before emitting an `INFERRED`
candidate. The legacy proposal request and schema are unchanged. No provider, network path, serving
integration, calibration corpus, authority, or product claim is added.

Focused provider-spy tests cover the successful end-to-end path, pre-call question refusal,
case-folded/duplicate/missing/fabricated distributions, non-unique maxima, low-confidence and
explicit abstention, schema separation, complete cache identity, and the unchanged authority ceiling.
Independent review identified shared-state races, mutable evidence aliases, and incomplete screening
of transmitted handles. The repaired gate serializes run reservations and ledger reuse, snapshots
selected evidence, and screens every transmitted caller-authored string; focused race tests cover
concurrent budget/cache behavior and mutation during provider latency.
The canonical gate then exposed one malformed wrapped Agent-digest bullet, which was repaired and
independently re-reviewed. Two subsequent exact-target gate runs failed only because the large
`contextindex` fixture used the benchmark Git helper, allowing detached auto-maintenance to recreate
`.git/info` during `t.TempDir` cleanup. A 20-run loop reproduced 16 failures; applying the existing
`testGit` synchronous-maintenance policy to benchmark fixtures made all 20 pass without changing
runtime behavior.
The frozen calibration, held-out replay, kill-gate, and first accepted extractor profile remain open;
this deterministic slice is not evidence that any model's probabilities are calibrated.

## 2026-09-21 AHI-022: OpenCode file-change burst fallback

Issue #55 reproduced two adapter-local failures: concurrent `file.edited` callbacks overlapped
Corvint subprocesses, and the structured `unsupported-impact-path-suffix` refusal reached the
terminal as a fault. The OpenCode adapter now shares one bounded file-change drain, coalesces
duplicate per-session paths, and records that expected refusal through `client.app.log` while
leaving the Core non-zero refusal unchanged. The adapter fixture asserts both no overlap across a
twenty-event burst and preservation of the structured refusal code. Node 16 was outside the
package's declared `>=20` runtime; supported-runtime verification used Node 22.23.2.

The first full gate passed the issue #55 adapter coverage, but
`TestBuildQueryAgreesAcrossWorkerCounts` failed cleanup once there and twice in isolated retries
while `t.TempDir` removed `.git`. The exact failing test passed with Git auto-maintenance disabled.
The helper now applies the same
`maintenance.autoDetach=false`, `gc.autoDetach=false`, and `gc.auto=0` fixture boundary as
`testGit`, so every fixture writer finishes before cleanup. After the repair, the test passed ten
consecutive isolated runs under concurrent gate load.

## 2026-09-20 LAC-V0-032: safe roadmap auto-recheck

The roadmap repeats its existing read-only request every 30 seconds. Eligibility remains derived by
Corvint Tasks: no mutation, approval, external/manual completion, unknown-evidence waiver,
admission, release candidacy, attestation, or promotion is added. Pages containing mutation forms
do not refresh automatically. A page-preserving pause/resume link prevents timed reloads from
interrupting deliberate inspection. `TestRoadmapSafeAutoRecheck` binds the refresh and hard-stop
notice on successful and refused reads and checks that paused roadmaps and the board remain stable.
The first full gate reached every package but failed three `internal/contextindex` tests during
`t.TempDir` cleanup: detached Git maintenance recreated `.git/objects/info/packs` and
`.git/info/refs` after removal began. The shared fixture Git helper now disables auto-gc and keeps
any maintenance synchronous; the exact combined reproducer and the full gate must pass after this
repair before the console change is qualified.

## 2026-09-21 PWP-V2: sensitive browser-input evidence boundary

GitHub issue #56 adds explicit `corvint-playwright-external/2` selection. The reporter redacts
default and bounded provider-added input actions before its private JSON write; the Go boundary
validates the untrusted report and retained canonical document again. Findings retain only a typed
code and structural path. `/0` and `/1` reject the new fields and keep their prior behavior.

Focused `internal/jstestprovider`, `internal/testvaliditydoc`, and
`cmd/corvint-js-test-provider` tests passed, including a deliberately leaking conformance payload
and an accepted redacted payload. The checked-in Node regression executes the actual reporter over
nested, retried, escaped, metadata-declared and deliberately leaking actions; its output retains no
fixture values. Go regressions cover whole-receipt validation, normalization, Unicode case-folding,
the depth, total-step, string and finding bounds, sibling risk fields, short-value structural
noninterference, and fixed value-free decoder failures. Raw-step bounds run before fixture recursion;
scrubbing is restricted to action titles, declared sensitive metadata, and error/attachment/failure
detail fields so status, identity and criterion text remain unchanged. `node --check` and
`node --test` passed. The live Playwright reporter matrix is `NOT_RUN`, so `/2` is explicitly non-promotable and
cannot project passing execution; existing `/0` and `/1` qualifications are unchanged.

The fresh repair reproduced three independent-review P1s before changing code: an already-redacted
action admitted arbitrary raw risk fields; receiver-prefixed unquoted values escaped sibling errors;
and retained-document unknown-property diagnostics echoed attacker text. Sensitive tests now require
canonical redaction of every nonempty diagnostic/attachment field across all retries, including when
no original value exists. Raw-title matching and extraction share one receiver-aware matcher without
case-transformed offsets. Malformed `/2` retained documents and inputs whose kind cannot be decoded
return a fixed typed `sensitive-input-document-invalid` finding. Successfully probed legacy `/0` and
`/1` closed-decode diagnostics retain their prior behavior. All three focused Go packages, six actual
Node reporter regressions, reporter syntax checking and focused vet passed after this repair; the
coordinator owns independent review, the frozen full gate and final dogfood binding.

The fresh task's next independent review reproduced punctuation/Unicode leading-action gaps and
missing custom/final-argument extraction, including cross-test echoes. Repair cycle 1 replaced
substring/ASCII-regexp classification with shared Go/JavaScript Unicode token semantics at a bounded
leading action or receiver position. The original rune sequence supplies the parsed tail; quoted
commas, escapes and nested parentheses cannot split an argument. Any sensitive action now protects
every risk field report-wide, including already-redacted manifests with no original candidate.
Regression controls preserve assertion/navigation prose containing embedded action names. All nine
actual Node reporter tests, the three focused Go packages and focused vet passed; the independent
overlay replay reported `leak=false` and successful sanitized document decoding for every reviewed
title. No live reporter qualification or promotion is inferred from these bounded tests.

Repair cycle 2 reproduced an admitted `custom+entry` pattern that the matcher ignored and a
zero-word `***` declaration that silently disabled its own detection. Normalization and matching
now share exactly the non-Unicode-letter/number/mark separator class, and both implementations
reject zero-word or oversized declarations before accepting evidence. A bounded lexical refusal
also closes call-bearing receiver expressions containing sensitive action tokens or quoted property
names. These expressions remain unsupported; rejection is typed and value-free and prevents the
reporter from writing a partial report after earlier tests. All eleven actual Node reporter tests,
the three focused Go packages, focused vet, and the independent boundary-check overlay passed.
The reviewer's unchanged latest replay now stops at `sensitive-input-policy-invalid` because it
adds `***` to every policy while still expecting successful serialization; the checked-in regression
asserts that required rejection explicitly. The live `/2` qualification hold remains in force.

## 2026-09-21 AFP-V0-019 / MCPV0-020: explicit immutable planning snapshot (issue #57)

The owner requested authoritative evidence for an explicit immutable snapshot while unrelated
checkout paths remain dirty. The experimental route binds current commit, complete tree, base,
exact changed paths and canonical path digest. It does not authenticate caller intent or accept
runtime coverage. CLI selectors consume bounded committed blobs in disposable private scratch;
MCP uses the existing immutable revision reader and preserves the outer mixed-worktree binding.
Query snapshot authority excludes mutable traces/history rather than weakening their drift gate.
Missing/stale/mismatched receipts fail closed; unsupported overlays and provider/discovery
composition remain explicit exclusions. Existing live-worktree routes are unchanged.

Pre-change native query succeeded at base `3191c0b95fb154a94f1e758d935b72fe237dfd65`;
raw evidence, usage baseline and keyed local-completion plan are in `/tmp/corvint-issue57` and the
worktree-private Git evidence directory. Initial dogfood retained `NOT_PRODUCED` reasons
`git-diff-failed` (empty range), `missing-intent-scope`, `cem-map-not-produced`, and
`outcome-input-not-provided`; these are not passing verification. `affected --base` was used
before tests. Query, affected, CEM/OCM and MCP are applicable routes; learning, providers, mutation,
console and external host qualification are not part of this source-selection change.

Focused snapshot regressions exposed query history's live-state binding; the repair retains that
binding for legacy queries and explicitly omits history in the immutable profile. Focused receipt,
CLI and MCP regressions passed. Final canonical checks and independent review are recorded by
keyed dogfood observations and the coordinating task, not inferred from these fixture results.
Independent review cycle 1 reproduced an MCP admission gap: symlink/gitlink rejection had
only guarded CLI materialization. Shared bounded tree validation now guards both routes, with
MCP query/impact regressions for both shapes. The same review found final coverage compilation
dropped the history/trace exclusion disclosure; the compiler now receives that disclosure and
the MCP wire test asserts it alongside authoritative evidence. The in-flight canonical root suite was cancelled
before repair (`verification-cancelled`), never counted as passing evidence.
The builder did not delegate under the sole-builder instruction; independent review belongs to
the coordinator. Gate-plan/gate-intent scripts are Beamfall workflow tooling, absent here; the
practical plan review and requirement-linked tests provide local review inputs, not an independent
gate claim. Rollback removes only explicit snapshot admission and its optional scope fields.

## 2026-09-20 PWP-V0-003/007/008: standard Playwright device-spread regression

GitHub issue #49 reported that the ordinary Playwright project form
`use: {...devices['Desktop Chrome']}` produced `report-identity-unknown`. An isolated issue branch
reproduced that exact failure before resolving Playwright's default bundled headless executable; the
same implementation area was then superseded on `main` by issue #50's stricter registry revision,
manifest version, executable suffix and SHA-256 qualification. The final integration retains that
stricter implementation and adds a separate real-browser regression for the original device spread.

The retained fixture requires project/browser identity, config digest, stable test ID, nonempty user
agent, 1280×720 viewport, bundled browser version/path and a passing projection in one receipt. It
passed with the pinned Playwright 1.63.0 modules and browser. The exact Golf checkout and hosted CI
remain `NOT_OBSERVED`; the minimal checked-in fixture proves the reported configuration shape, not
the unavailable consumer repository.

## 2026-09-20 PWP-V0-008: Playwright 1.63 bundled headless-shell qualification

GitHub issue #50 exposed that the only qualified Darwin arm64 / Node v22.23.2 Playwright 1.63 path
used auto-updating system Chrome. The reporter now distinguishes configured and Playwright-registry
executables. Its bundled path binds registry name `chromium-headless-shell`, revision `1243`, manifest
and observed browser version `153.0.8010.12`, a cache-root-independent executable suffix, and exact
SHA-256 `a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282`. Projection rejects any
changed Node version, revision, manifest/browser version, executable kind/path/digest, headed mode or
explicit override. Remote-browser connections also abstain because the local executable identity does
not describe the connected browser. The previous system-Chrome tuple remains a separate exact branch.

The real external-server matrix now runs against the bundled headless shell and retains pass,
assertion failure, timeout, retry, cancellation, browser infrastructure, two-project identity,
external-server survival and MCP discovery controls, then smoke-tests the previous system path.
Behavior and stability corpus fixtures consume the bundled tuple through the ordinary retained-
receipt qualification path. Additional Node/browser tuples require a spec amendment, exact lock and
registry identities, the complete live matrix and negative drift controls; no semver widening is
accepted. The live matrix passed in 25.91s with headed and remote-connection negative controls using
the preinstalled locked 1.63.0 modules and browser;
no package or browser download ran. The pre-change Corvint query selected an unrelated documentation
compiler and omitted four ranked results; targeted path impact identified the PWP spec, reporter,
tests and consumers instead.

Independent review found that `launchOptions.headless=false` and explicit or environment-selected
remote browser connections could initially retain the local bundled identity. The reporter now
matches Playwright's headless precedence and abstains for remote connections and custom launch
fixtures; retained projection rejects serialized connection options. The added live negatives pass,
and the repair re-review found no remaining actionable issue.

The first scoped local-completion plan could not retain the successful verbose live check because the
writer secret screen classified Go's `--- PASS: TestQualifiedPlaywrightLive` marker as a credential
assignment. That plan was cancelled without satisfaction, the confirmed false positive was added to
`docs/agent-memory/bugs.md`, and the same frozen live test was selected without `-v` for retained
evidence; this changes log verbosity, not execution or assertions.

## 2026-09-20 Integrated canonical-gate repair

The first combined `make gate` rejected the candidate before publication. The work-queue OCM
enumeration still stopped at WQO-V0-048 after WQO-V0-049..050 were added, and the Playwright
minimizer claim exceeded the 160-character index limit while its README and generated index had
diverged. Those conformance records now agree. Two context-index tests also exposed a repeatable
macOS cleanup race: Apple Git auto-maintenance could recreate `.git/objects/info/packs` while Go
removed a temporary repository. A second full-gate run exposed the same race in a different query
fixture, proving the first fixture-local repair too narrow. The shared context-index Git fixture
launchers now disable automatic GC and maintenance for every mutating test command. The failed
full-gate receipts remain retained and invalidated; a new commit-bound canonical gate is required.

## 2026-09-20 PSM-V0-004/008/009: original failure identity repair

Independent final review found that consistent failures of a different class could be called
reproductions of the original failure. Reproduction and candidate trials now require exact sorted
distinct failure-class sets; missing or additional classes retain observations but invalidate the
trial with `original-failure-signature-mismatch` and prevent confidence/minimality claims. Native
planning rejects caller classes inconsistent with the qualified original target and binds rederived
observations into the plan. The original receipt bytes/digest remain immutable; new-run evidence
digests and summaries are retained, not compared for impossible byte equality. Class-set equality
does not prove identical root cause. Isolation failures still stop minimization independently.
Synthetic assertion-to-fixture/synchronization controls cover reproduction and both candidate
kinds; multi-class controls cover missing/additional classes, ordering, duplicates and fresh evidence.
Focused minimizer and companion tests pass; no additional live-world claim or release promotion.

## 2026-09-20 Issues 42, 43, 47 and PUB-V0: integrated review repair

Stability accepts fully qualified `/1` application attestations, including a distinct application
repository, while preserving `/0` declaration semantics. Invalid attestation and contradictory
application revision refuse. The Docker fixture now selects the accepted explicit system-Chrome
tuple for Playwright 1.63. Its explicit qualification passed (23.829s) with installed modules at
`/private/tmp/corvint-pw163.589jVO/node_modules`; no runtime package was downloaded.

The separately built experimental minimizer now offers read-only planning and digest-approved
execution, rederives actual corpus stability evidence, qualifies actual `/1` receipts, constructs
provider selectors, compares observed schedule/topology, and retains complete native trial evidence.
Reset/cleanup run pinned operator commands with bounded output and cancellation cleanup. A separate
20ms PID/start observer terminates and verifies absence of observed escaped descendants; failures
invalidate the trial. It explicitly does not prove universal containment or unobserved fast-detach
absence. CRR-V0-003(c) and `RequireDescendantCleanup` continue to refuse before launch unchanged.
Detached-child, PID-reuse, authorization, evidence-tampering, reset-failure and cancellation controls
cover the new boundary. Real Docker/Playwright predecessor-failure then isolated-pass qualification
passed (11.807s). The other classification fixtures remain synthetic, not six claimed live worlds.

The owner selected `v0.5.0a1` for publication. Decision 0327 supersedes the pending version target
without rewriting historical decisions or measurement evidence; active release tuple, notes,
installation, editor admission and publication fixtures move together. Unsigned prerelease,
publisher `NOT_VERIFIED`, four non-Windows core archives, separately qualified optional companion,
and no-promotion semantics remain. Full integrated gate and publication remain coordinator-owned.

## 2026-09-20 Issues 39–47: consolidated integration

The completed issue branches are merged in dependency order: 39, 43, 41, 42 (including 40),
46, 47, 45 and 44. The integration preserves each branch history and sealed CEM; inherited shared
CEMs are removed so the coordinator can bind one combined change. Conflicts retain both independent
build-log entries and provider tests, the newest stability requirements, and both Playwright
consuming-path qualification and application-attestation contracts. The generated requirement index
is rebuilt from the merged specifications. Compilation caught two synthetic receipt fixtures that
still referenced the removed single-version constant; both explicitly retain their original 1.60.0
version. Spec-index validation caught a README claim-prefix mismatch, repaired without dropping the
issue-41 discovery status. The installed release smoke also supplies issue-45's explicit executable
binding when initializing its work queue. Focused conflict checks and compilation precede the coordinator-owned
combined review, frozen gate and separate release qualification; those remain required.

## 2026-09-20 PWP-V1: externally managed application attestation

Issue 43 adds `corvint-playwright-external/1` without changing `/0`. A generic bounded command
provider receives one canonical expectation document on stdin and emits the same closed canonical
application-attestation shape before and after Playwright. The receipt binds clean test-repository
root/revision/tree; application root/revision/tree and dirty policy; image, Compose configuration,
container/start generation and health; provider executable/config/output digests; runner, browser,
argv and declared environment. The provider executable runs from a private content copy. Corvint
owns only provider and Playwright process groups and has no application lifecycle verb.

The local qualification used `@playwright/test@1.63.0`, its installed Chromium, and a disposable
scratch-image Docker server built from the checked-in closed Compose JSON manifest. A healthy bound
run projected passed; healthy wrong-revision and wrong-image inputs stopped before Playwright, and a
fixture-harness restart changed container start generation and forced infrastructure. Generic command
tests also cover unavailable, unhealthy, missing and contradictory attestations. Docker 29.5.2 was
available; no Compose frontend was installed, so the qualification harness executed the manifest's
closed build/run/health/port subset through project-scoped Docker commands and retained the exact
manifest digest. Signal-aware cleanup removed the fixture container and image; no external app was
started, stopped or changed.

Independent review found that the first implementation sampled the test repository before the test,
accepted attestation on the managed-server path, inherited ambient Git repository redirects, allowed
mixed `/0` and `/1` fields, under-validated retained provider identities, and could consume Docker
cleanup before provisioning completed. The repaired qualification changes an otherwise unbound
tracked test-repository file during Playwright and cancels cleanup before provisioning; both remain
non-passing and the latter leaves no container or image. Provider/config hash syntax and canonical
configuration binding, profile shapes, external-only admission, post-run repository identity, and a
Git environment without `GIT_*` redirects have focused regressions.

The first frozen canonical gate exposed two fixture/metadata failures: the spec index repeated a
longer, non-identical digest, and the real local-completion fixture omitted the existing
`qualified-reporter.cjs` embed required by the issue-39 baseline. The digest is now one exact
sub-160-character value across the spec, index and README; the import-closure fixture copies that
embedded asset. Focused `internal/specindex` and real-evidence local-completion regressions cover
both repairs before the gate is rerun on the replacement frozen commit.
That replacement gate passed the full test suite, native/cross vet, archive and interop, then caught
the stale generated `REQUIREMENTS.tsv` summaries for the revised PWP-V1-001 and PWP-V1-006 clauses;
the registry was regenerated before the final frozen gate.

## 2026-09-20 PWP-V0: Playwright 1.63.0 external-server qualification

Issue 39 extends the accepted external-server profile's exact runner allowlist from Playwright
1.60.0 to 1.60.0 and 1.63.0. The checked-in real-browser matrix passed locally on Darwin with
macOS arm64, Node v22.23.2 and `@playwright/test@1.63.0` with system Google Chrome
153.0.8010.48 at `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`; no channel override
was used and the Playwright `chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell`
executable was present. It preserved pass, assertion failure,
test timeout, interruption, browser infrastructure failure, retry/attempt state, two-project
identity, inherited `webServer` suppression, external-server survival, cancellation cleanup, and
retained MCP discovery, setup dependencies, global use, project inheritance, two-worker execution
and repeat-each identities. Executable option metadata and a custom `page` fixture both produced
unknown identity/infrastructure and never a passing projection. Other Playwright versions remain
unqualified; Playwright 1.63 on another Node/platform/browser path also remains diagnostic-only.
The Linux amd64 installed/bundled-browser arm is `NOT_RUN`. A local `golf-e2e` checkout does not
exist, so its deterministic consumer fixture and CI observation are `NOT_OBSERVED`. The externally
managed application command for `http://127.0.0.1:3002` is recorded in
the accepted profile; its config owns the bound system-Chrome executable path, and the provider
neither starts nor stops that application.
## 2026-09-20 AFP-V0-018 / TJAA-V0-012..017: issue 41 discovery amendment

The owner authorized cancellation of the superseded TJAA-only enrollment and re-enrollment from
the same original base `536e560e1fa35573e49df644dde4257a8bb7e050` with AFP and TJAA. Original
enrollment, source commits and failed gate evidence remain archived; that gate failed in
`TestLocalCompletionRealEvidenceWorkflow/LCP-V0-007_completion` with
`dogfood-change REFUSE current-tree-corvint-build-failed` and is not passing evidence for this scope.

The amended profile gates all file argv on canonical caller-owned discovery reconciliation and
uses a single complete-config command when discovery is unproven. Project membership alone defines
candidate tests; helpers remain dependency sources. A matched universe with unknown reachability
widens to exactly its file/project pairs, eliminating the former helper-by-project Cartesian fallback.
The input binds revision, config and current source bytes; missing evidence remains explicit.

The assumed existing Playwright installation was unavailable. A temporary installation from the
repository's pinned interactive-alpha lockfile supplied Playwright 1.63.0 without browsers.
Its real unfiltered `--list --reporter=json` yielded exactly eight pairs: two spec files under
Chromium/Angular/React plus setup and cleanup. Reconciliation matched all eight; a page-object edit
selected five pairs and excluded the three unrelated spec variants, with no helper argv. Raw listing,
input receipt and CLI outputs are retained under `/private/tmp/issue-41-list*` and
`/private/tmp/issue-41-real-discovery.json`; no consumer-checkout or runtime-execution claim follows.
Frozen synthetic 117-file qualification also requires an independent 353-pair receipt. Canonical
repeatability, mismatch differences, stale bindings, malformed inputs, strict bounds and fallback
regressions pass focused checks. The exact golf-e2e checkout remains `NOT_OBSERVED`.

Corvint pre-change context and required dogfood preparation were used. Initial preparation retained
missing citation/scope/outcome reasons; final reports and the parent-owned serialized gate remain
required. Optional mutation/provider execution and runtime promotion are outside this static slice.

Independent review found custom config names lacked test-to-config edges, allowing a transitive
global-setup helper change to select nothing despite matched discovery. Repair explicitly binds
every admitted physical test to the selected config; a custom `e2e.config.ts` regression requires
the entire matched suite for its setup-helper change without making helpers executable units.

## 2026-09-20 TJAA-V0-012..017: golf-shaped Playwright selection (issue 41)

The owner-requested follow-up to issue 18 adds static global-use inheritance, nearest-tsconfig
baseUrl/paths resolution and global setup/teardown dependency edges to the opt-in profile. The
shared default adapter retains its previous alias frontier. Unsupported inheritance, loader-shaped
resolution, ambiguous or missing targets, computed imports and config still widen; application state
remains an execution unknown. No JavaScript/config is executed.

The synthetic golf-shaped qualification proves 41 selected units out of 353 for one changed cohort,
all 353 for global-setup helpers/config, distinct Chromium/Angular/React units, setup/cleanup closure,
and identical canonical bytes for identical inputs. The actual golf-e2e checkout was unavailable:
consumer configuration and consumer recall remain `NOT_OBSERVED`, with no runtime promotion claim.
The original qualification fixture and all shared TypeScript tests remain required gate inputs.

Corvint query and initial dogfood-change were used at base
`536e560e1fa35573e49df644dde4257a8bb7e050`; the query retained four omitted results and the initial
empty-change coordinator retained `NOT_PRODUCED` CEM/OCM/outcome reasons. A pre-first-query measurement
receipt was `NOT_OBSERVED`; no token/cost savings are claimed. The enrolled gate and final CEM/OCM
reports remain the authoritative completion evidence. Mutation, external providers and runtime
qualification are not applicable to this bounded static observer change. Independent review is owned
by the parent task, with no nested delegation.

Independent review found equal-prefix alias ordering, multiple existing alias targets, and explicit
browser inheritance across device spreads could differ from Playwright 1.63. Repair widens both
alias ambiguities and preserves explicit browserName over a device defaultBrowserType. Conflict
regressions cover both pattern orders, competing targets and inherited/same-layer browser defaults.
Final review also confirmed explicit `use.defaultBrowserType` cannot be ignored: the closed subset
now rejects that key with browser-identity uncertainty, covered at both global and project scope.

## 2026-09-20 MER-V0-001..012: revision-bound migration evidence ratchet (issue 46)

The experimental `migration-ratchet` profile compares digest-verified baseline and candidate
snapshots across stable migration identities and emits raw denominators plus separate addition,
removal, content, state, stale-evidence, reverse-link and unknown deltas. Repository policy prevents
new legacy debt, uncontracted test additions or changes, terminal regression, stale review/runtime
reuse and unresolved-denominator growth. Exact reviewed rules are required for otherwise
incomparable domains; exact owner-reviewed expiring exceptions never erase their underlying deltas.

The earliest synthetic baseline-to-candidate receipt advances one grandfathered unresolved identity
without growing its denominator and is byte-identical across repeated compilation. Six issue-46
negative controls and whole-input refusal controls pass in the focused package tests. The command
exit contract passes its focused CLI tests. These synthetic fixtures do not establish provider
honesty, contract adequacy, migration completeness or real consumer compatibility. The full shared
gate is intentionally NOT_RUN pending coordinator release.

The task-start query retained four omitted results and six withheld test-path candidates. The first
pre-change coordinator attempt at base `6098291c9ed84c0de5c1a76afa3599d6a6faa352` correctly remained
`not-complete` before any change existed. Dirty-diff `affected` selected only `cmd/corvint` and
`internal/migrationratchet`, while retaining its language-frontier and unowned-document unknowns.
The documentation-corpus, stability, mutation, provider and service execution routes were not
applicable: this profile compares caller-supplied immutable artifacts and executes none of them.

Independent review found five fail-closed defects in the first implementation: trailing scalar or
malformed JSON was not required to reach EOF; an explicit identity mapping could reuse an implicitly
paired candidate and omit another; one exception could waive every defect of the same class on an
identity; contradictory selections were order-dependent; and record order/duplicate links were not
canonical. The repair requires exact EOF, globally one-to-one candidate pairing, one canonical delta
digest per exception, unique validated selections, sorted records and unique sorted links. Focused
regressions exercise every repair. Re-review found two remaining holes: mapped evidence identities
could skip renewal comparison, and stale/reverse-link delta fingerprints omitted the exact content
bindings that distinguish two defects. The final repair derives baseline records through the
one-to-one pair set and retains expected/actual content plus relation and reverse-relation bindings;
focused regressions cover both findings.

## 2026-09-20 PSM-V0-001..013: bounded Playwright suite-interaction planning (issue 47)

The experimental `internal/playwrightminimize` package emits a deterministic digest-bound schedule
that reproduces the original suite failure and rechecks isolation before separately enumerating
ordered predecessor sequences and unordered load sets. Trial count, repetition and wall-clock limits
stay explicit; partial searches cannot claim complete minimality. Execution exists only as an
abstract synthetic qualification seam and refuses without separate operator approval bound to the
exact plan. No Playwright, browser, server or application process ran for this slice.

Every synthetic trial binds revision/config/runner/browser/project/order/topology/fixture/seed and
application-instance identity, declares its reset policy, and carries setup, assertion, retry,
cleanup, server-health, resource and failure evidence. Failed reset/cleanup, attestation change,
identity drift, malformed/duplicate receipts, and missing evidence invalidate without erasing the
observation. Reports retain all seven failure classes, separate ordered and load findings, label
members necessary only in the observed universe, and never claim global minimality.

Six synthetic qualifications cover a predecessor leak, load-only resource failure, restart,
cleanup failure, nondeterminism and isolated product regression; controls cover authorization,
`not_reproduced`, exact planning and bounded incompleteness. #39 runner qualification and #43
application attestation are divergent development refs rather than integrated frozen dependencies at
base `6098291`; missing #39/#42/#43 receipts therefore remain distinct confidence blockers. Live
integration and the shared full gate are `NOT_RUN` pending their owning coordination.

Independent review reproduced seven boundary defects: a late or partial reproduction could become
conclusive; runners lacked the remaining wall-clock deadline; errored runners dropped returned
receipts; singleton necessity crossed worker topologies; isolation infrastructure failures were
called product regressions; opposite baselines could share one digest; and findings omitted passing
receipts that supported minimality. The repair deadline-bounds and post-validates every call,
requires complete repetition groups, preserves invalid errored receipts, gates necessity on exact
topology, derives only an explicitly product-only isolation label, rejects duplicate baselines, and
retains all comparison receipts. Focused regressions `TestPSMV0012` through `TestPSMV0016` cover the
review cases; re-review is recorded separately by the task coordinator. Corvint `affected` selected
only the new Go package while preserving the mandatory repository gate and documentation unknowns;
the pre-commit `prove` expansion returned `unsupported-impact-path-suffix` because the new package
was absent from its pinned repository revision, so no proof-of-impact claim is made.
Re-review then found that cancellation during the final runner call could publish confidence and that
mixed classifications returned before inspecting a later invalid repetition. The final repair reads
the trial context before releasing its deadline, invalidates cancelled results, and validates every
repetition before comparing classifications. `TestPSMV0017` and `TestPSMV0018` retain both cases.

## 2026-09-20 DCP-V1-021..026: revision-bound Playwright stability evidence (issue 42)

The experimental behavior-stability provider keeps issue-40 behavior coverage and repeated-run
stability as separate artifact axes. A repository-owned digest-bound policy selects one-spec,
feature-batch or suite thresholds; reports preserve planned/started/completed and every outcome,
retry, cleanup and manual-rerun count plus all contributing receipt/attempt evidence. A failed first
attempt remains both failed and flaky after a later pass. Missing iterations, duplicate receipts,
silently consumed retries, cross-application revisions and contradictory identities refuse; failed
cleanup is retained as a non-clean verdict. Corpus and MCP expose the exact aggregate without an
adequacy, parity, freshness or narrowing claim.

The earliest end-to-end aggregate and five requested negative controls pass on synthetic qualified
receipts. The original task query preceded private measurement and remains `NOT_PRODUCED`; the later
required DOGFOOD enrollment pins base `06eb443565979b313ecb63f7316f06677a908f65` and the owning
documentation-corpus spec. Live repeated browser execution and consumer policy qualification are
NOT_RUN, so the feature remains proposed/experimental.

The focused `cmd/corvint` regression exposed that its minimal local-completion repository copied Go
sources but omitted the qualified Playwright reporter embedded by the now-reachable provider import.
The fixture now carries that production embed; the product binary and provider profile are unchanged.

Independent review found that the first aggregate accepted carried identities without rebinding
test/config bytes, could hide failed native runner cleanup behind a carried pass, established its
identity baseline after an earlier manual run, ignored manual cleanup, and counted only final timeout,
interruption and infrastructure states. The repair requires explicit source mappings and the exact
behavior-test anchor, recomputes and verifies native projections, fixes the baseline to planned
repetition one, applies cleanup to every contributor, and retains each earlier attempt category.
Adversarial regressions cover each finding plus noncontiguous retry ordinals.
Re-review found that an invalid receipt and a genuine infrastructure outcome shared the same native
projection. The final repair verifies qualified lifecycle/identity binding independently of outcome
classification; paired regressions accept a bound infrastructure outcome and refuse the same outcome
when runner cleanup failed.
The authorized final repair rejects state/failure-kind contradictions at both the qualified receipt
binding and stability classification boundaries; a passed attempt carrying assertion-failure
metadata and an artifact now refuses instead of contributing to a clean verdict.
Late coordination integrated the amended, evidence-bound issue-40 contract at `e54ffcb`, including
patch-equivalent copies of both shared local-completion fixture repairs. The earlier gate on `4a60483`
is superseded and failed at the error-code ownership ratchet because the accepted Playwright provider
had never enumerated its existing refusal vocabulary. The owning PWP-V0 spec now records those codes;
no error behavior or wire value changed.
Integration review found that receipt identity qualification did not derive the aggregate outcome
from the ordered attempts. The stability consumer now applies the reporter's exact terminal-state
rule, including requiring a prior non-passing attempt before `flaky`; contradictory terminal states
refuse before policy counting.
The later owner acceptance comment makes declared-versus-observed execution topology first-class.
The source, focused-check and review evidence at `6098291` remains retained but is superseded for
completion by this amendment. Repository policy and each observed run now bind separate canonical
full-file topology inputs covering CI nodes/shards, Playwright workers per node, database mode,
sorted project set, split algorithm/version and resource class; each observation also binds its
receipt digest. Exact mismatch refuses before counting, with an explicit six-declared/four-observed
negative witness plus deterministic per-dimension and source-binding controls. The comment reports
that contradiction in the consuming repository, but no exact policy, CircleCI or run artifacts were
provided, so the consumer-specific 6-vs-4 result remains `NOT_OBSERVED`. The amendment-start Corvint
query selected an unrelated public-release spec and omitted four ranked results; repository-owned
spec routing supplied the owning DCP contract instead.

## 2026-09-20 DCP-V1-004/007..013/019: experimental behavior contract ingestion (issue 40)

The optional behavior-provider profile joins bidirectional flow/criterion/test/project identities
and a separate ordered runtime witness to pinned native Playwright observations. It preserves
provider-reported review versus unreviewed joins, scoped denominators and full-suite fallback.
Existing title-only joins were insufficient for identical titles in different projects; optional
exact test/project selectors preserve the legacy profile while admitting an unambiguous join.

Synthetic fixture paths model the proposed registry and schema-2 migration manifest. The consumer's
actual fixture bytes were not supplied: compatibility and live runtime utility are NOT_OBSERVED.
Pre-change query succeeded with three omitted results. Measurement before that first call was
NOT_PRODUCED; the required coordinator retained its own receipts and reported no-change CEM/OCM
and outcome NOT_PRODUCED. The local completion plan is enrolled against the immutable issue base.
No browser, server or provider process was launched; live qualification is not claimed.
Independent review found that demanding a passed run projection and current E2E freshness made
recorded verification unreachable for native Playwright receipts. The repair uses the native per-test
execution projection, rebinds test/configuration inputs, and preserves the exact unknown app-freshness
axis. A full Build/Open regression imports a synthetic qualified receipt through native decoding and
projection; no manually assigned CURRENT projection is used by that end-to-end test.
The owner's subsequent issue-40 acceptance amendment strengthens assertion target/value identity,
ordered browser-context/page/frame navigation, live discovery denominator reconciliation and the
app/e2e/docs revision set. Synthetic negatives cover same matcher/wrong element or value, same route
without assertion, wrong project, reordered/scoped visits and stale revision members. The consumer's
463/117 inventory and local consumer fixtures remain NOT_OBSERVED; generated prose retains its label.
Amendment review required rejecting digest-valid but noncurrent behavior anchors and checking every
assertion, not only finding one qualifying assertion per criterion. End-to-end regressions retain
validly pinned alternate-revision source and matching extra runtime events while rejecting their
stale, unreviewed or undeclared joins.
Final amendment review also required the reverse runtime-assertion check: a runtime event and
matching expected order cannot invent an assertion absent from the validated test declaration.
The retained-run regression covers that previously one-way join explicitly.
The later legacy acceptance amendment adds an independently digested suite/file/case inventory,
executable/disabled state, extracted observable criteria and fixture/role preconditions. Exact
reviewed target-to-legacy relations and complete reverse criterion coverage distinguish target
journey verification from retained legacy runtime parity. Same/stronger preserve the original
observable tuple; new/obsolete/blocked remain non-parity. Synthetic Build/Open fixtures cover a
consolidated test dropping one legacy branch and a same-named target changing success criteria.
Unavailable or disabled legacy runtime remains unknown even with source/docs/product review.
The supported runtime qualifier reuses retained native Playwright receipts, not a fabricated
legacy runner adapter. Exact consumer legacy inputs and actual live legacy execution remain
NOT_OBSERVED; unsupported legacy runners cannot establish runtime parity.
Legacy amendment review found that a classified but ineligible mapping could hide a dropped branch
from another target's parity result. Reverse parity coverage now counts only fully eligible targets;
a multi-target regression preserves the first target's journey while rejecting migration parity
when another target drops the original observable.
## 2026-09-20 WQO-V0-049..050: explicit work-queue executable binding (issue 45)

Decision 0326 binds repository adoption to one operator-selected canonical external Corvint file.
The reviewed adapter records its absolute path, SHA-256, version/build output and Go module/VCS
source identity. The observer rejects relative, missing, linked, unsafe-parent, writable,
repository-controlled or drifted bindings before adapter execution. It privately materializes only
the already-opened verified bytes and passes that path as trusted adapter argv, preserving the fixed
VPO environment and avoiding ambient `PATH`. Darwin cannot make the VPO-V0-024 exact-object claim,
so the receipt remains honestly `UNQUALIFIED`; the binding is drift protection, not attestation.
`work rebind` changes only the generated adapter for explicit review and commit.

The smallest `~/.local/bin` init/observe/propose path passed before the wider matrix. Focused
`TestWork*` passed, including `~/.local/bin`, `/opt/homebrew/bin`, `/usr/local/bin`, changed binary,
reviewed rebind, symlink swap, repository-local, unsafe-parent, relative and missing executable
fixtures. The focused help/parser checks and `internal/companionrelease` package passed; its first
sandboxed run could not bind an `httptest` listener and the authorized unsandboxed rerun passed.
The canonical gate is deliberately enrolled but NOT_RUN pending the parent queue's explicit slot
release; these focused results do not substitute for it.

Actual self-development routes: original query selected the accepted WQO spec with four ranked
results and test-symbol candidates omitted; the clean pre-change impact was OUT_OF_SCOPE with zero
changed paths; start-time dogfood-change retained empty-range CEM/missing-scope outcomes as
NOT_PRODUCED; immutable local completion enrollment froze the WQO intent and focused/spec/full-gate
checks; dirty `affected` selected four Go packages while retaining unowned documentation paths and
language-frontier unknowns. Mutation, corpus, external-provider, service and learning routes are not
applicable. No paired baseline exists and no savings claim is made.

Independent security review found and the first repair cycle closed three issues: companion
`-buildvcs=false` binaries now retain an explicit no-VCS module/toolchain identity; rebind pins and
rechecks the opened `.corvint` directory plus exact adapter bytes before replacement; and bound
executable drift during an operation now maps to `SOURCE_UNQUALIFIED` rather than `ADAPTER_FAILED`.
## 2026-09-20 PUB-V0-022..026: closed qualified release candidate (issue 44, Corvint half)

The Corvint release path now closes the existing seven-file core archive-gate output and the
three-file companion retained output into one versioned candidate. The candidate verifier binds
the exact Corvint commit/tree/toolchain across both inputs, retains both source archives and gate
receipts, requires the installed `Corvint <version> (build N)` identity, and records explicit
platform/workflow `PASS` or `NOT_RUN` rows. The companion `/2` installed smoke now exercises
affected selection, external Playwright receipt discovery, documentation-corpus discovery and
repository work-queue observation through the extracted `corvint` binary. Legacy companion
profiles keep their historical smoke inventory.

The versioned installer reverifies the closed candidate, retains the host core archive at a unique
version/platform path, refuses replacement and never writes a current/latest selector. Focused
native and Linux cross-build checks pass. Independent review found that the first implementation
bound only compressed core archive bytes, reread companion inputs without verifying the completed
staging tree, used a fixed removable version-probe path, and omitted three release-note disclosures.
The repair decodes the exact six-member core archives, binds binary/checksum/build identities and
executes the host core version, verifies completed staging before no-replace promotion, uses only a
unique owned probe, refuses scratch/output overlap, and names local-Git trust, unmeasured performance
and unavailable hosted CI. Re-review then found that the host probe polluted the exact three-file
companion verification directory and lacked caller cancellation. The final repair isolates both
directories, propagates caller cancellation with a bounded probe, and adds a closed-candidate
regression that executes the host identity check and proves the companion verifier receives exactly
three files. The pre-change core archive gate passed. The pre-change
companion gate reached the separately owned Corvint Tasks checkout and failed before retention at
`corvint-tasks init`; therefore no combined candidate was produced and Linux installed workflows
remain `NOT_RUN`. A retained-scratch reproduction identified the refusal as
`INTENT_BRANCH_MISMATCH`: the closed Git environment initialized the smoke repository on `master`
while the companion-owned intent fixture requires `main`. The repair pins the fixture branch and
retains structured command stdout in failed smoke evidence so a typed refusal cannot be hidden by
an empty stderr stream. The repaired companion gate passed at Corvint `68c9bbc` against exact
Corvint Tasks commit `f6ec200337160545b5e120a7242a8e862ecbcff0` and tree
`7573360392d58000f30dc0f82e6716cba09d81d1`; its retained archive SHA-256 is
`9dac067f16da86b8dc8bcb97414c96472d2fe613947d8acdca95cfb2024ad2cd`. All native installed
smoke rows passed and the four non-native targets remain explicitly `NOT_RUN`. The canonical full
suite was run twice on the unchanged repair commit; both runs passed the changed companion and
release-candidate packages but `internal/liveverify/session` exceeded its fixed 20-second event
wait under full-suite load. The same package passed immediately in isolation (35.8 seconds total),
as did the initially load-affected `internal/procgroup`; full vet and the CEM interop test/vet gate
passed. No unrelated timing-test source was changed. Publication, tagging, pushing, signing, upload
and promotion were not attempted.

## 2026-09-20 EEP-MCP / EEP-REMOTE: complete issue 11 transport profiles

Owner approval accepts decisions 0324/0325: bounded local MCP stdio and a separately built opt-in
HTTPS adapter. MCP needs interactive stdin; extending the existing process-group lifecycle avoids
a nested detached adapter group that Core could fail to kill. The protocol pins 2025-11-25, one
tool and one text envelope. HTTPS has normal TLS plus SPKI validation, explicit optional CA roots,
private bearer files and no redirects/proxies/retries. Core never imports the HTTPS package.

The original EEP-V0/V1/V2 and ETS conformance fixture matrices passed unchanged through both
transports, including strict decode, stale references, learned exclusions and authority separation.
Focused hostile checks cover bounds, timeouts, unavailable transports, MCP requests/extra frames,
normal and interrupted descendant cleanup, credential permissions, TLS and HTTP failures.
Independent security review found JSON-escaped credential reflection and case-insensitive MCP
field coercion. Repairs screen decoded credential strings and require exact MCP key spelling and
boolean values; the regression places `isError:true` before `IsError:false` to exercise the bypass.
The owner explicitly selected the existing affected-package fast tier instead of the full gate.
Its non-executing preflight selected packages without FALLBACK on merged base `bf2685dd`;
the canceled earlier-base enrollment remains non-success. The selected gate and final CEM/OCM
outcomes are retained in enrolled private observations; focused results do not substitute for them.

Actual self-development routes: original query (three omitted results retained), start-time
dogfood-change (empty-range CEM and missing scope/outcome NOT_PRODUCED), immutable profile enrollment,
dirty affected advice, and final CEM/OCM reports. Original query preceded the measurement scratch
receipt; its latency, billed tokens and complete source-open counts are NOT_OBSERVED. New profile
intents needed a planning-only commit before enrollment could resolve them; implementation began
only after enrollment. No savings claim. Live remote Internet services, external MCP server adoption,
mutation campaigns, documentation corpus and learning qualification are not applicable to this
transport-conformance change; frozen existing provider fixtures remain the behavioral witnesses.

## 2026-09-20 PWP-V0: external-server Playwright receipt qualification

The owner accepted the bounded `corvint-playwright-external/0` profile for issue 19. A local Darwin
qualification ran the checked-in fixture with Playwright 1.60.0 and its installed Chromium browser.
The matrix observed passing, assertion-failing, timed-out and browser-infrastructure outcomes across
distinct Chromium and React project identities; inherited `webServer` was suppressed, relative
global hooks executed, cancellation left the externally managed server alive, and retained evidence
was rediscovered through the actual test-validity MCP registry with lifecycle and freshness unknowns
preserved. Literal `test.use` overrides were attributed to their effective browser and viewport;
executable or unsupported overrides abstained. Focused provider, projection, CLI and MCP tests passed.

Independent review reproduced four defects before promotion: project defaults could misattribute an
effective `test.use` override, removing the profile discriminator bypassed lifecycle validation,
malformed qualified identity/attempt evidence could still project passing, and a temporary config
wrapper broke relative global setup resolution. The repair binds the qualified 1.60.0 reporter ABI,
rejects downgrade and incomplete or contradictory evidence, resolves original-config-relative
modules, and adds adversarial and live regressions. The same reviewer reran those overlays and the
expanded real browser matrix and returned PASS. Playwright versions other than 1.60.0 remain
unqualified; declared application identity is not proof of served content, Vitest and LPCV authority
are unchanged. The owner subsequently selected changed-feature-only verification instead of the
full native gate. On merged base `4c5f0fa4dc1a24698062287d0ffa2e7f4129a639`, the affected preflight
selected 148 packages without FALLBACK; its conservative reader closure was retained as overbroad,
not executed. The owner explicitly replaced it with tests and vet for the five provider/MCP/spec
packages, six formatting/spec/traceability checks, and the separate real Playwright 1.60.0 matrix.
Both superseded enrollments are preserved as canceled non-success. Final selected-check and CEM/OCM
observations are retained by the replacement enrolled workflow rather than claimed passed here.

## 2026-09-19 DCP-V1: experimental revision-pinned documentation corpus

Issue 31 was explicitly scoped to all six phases. The change supplies a native immutable corpus,
separately attributed native read integrations and CEM citation sidecar, capability-gated stdio MCP,
Markdown/JSON rendering and explicit block maintenance, and an independent flow adapter outside Core.
Gate A required real native receipt decoding rather than trusting normalized verification labels,
separate source/provider commits, complete inventory before analyzer admission, and same-call
maintenance rederivation. Those boundaries govern the implementation. Generated intent remains
proposed; opaque Go run identity, E2E served-build identity, assertion adequacy and external utility
remain unknown. Passing synthetic journey fixtures do not claim real browser execution.

Independent review found cross-binary engine identity, scope additions, native range impact,
excluded-source handling, maintenance publication races and a CEM base-mode gap. The consolidated
repair uses shared compiler identity, native admission/receipt metadata and descriptor-confined
no-clobber publication with retained recovery inodes. Actual two-binary and concurrency regressions
cover the previously hidden boundaries.

Focused corpus, provider, maintenance, adapter, native/CEM and transport tests passed during development.
A new receipt-join regression exposed the host's `/var` versus `/private/var` root alias; reads now use
the caller's lexical receipt root and the same confined file reader. An E2E freshness review exposed
that source identity alone was insufficient; unknown served builds now remain unknown. These failed
iterations were repaired before final verification. Frozen synthetic labels report precision, recall,
abstention, false relationships, latency and bytes, without a savings or universal-quality claim.

The first full gate found help usage lines being interpreted as executable draft examples, an
unupdated analyzer-input fingerprint, and a README claim that differed from the spec digest. Usage
now includes the optional root prefix, the analyzer schema advances to 69 for the added immutable
reader inputs, and the README repeats the contract claim. These native gate failures are retained;
the source must be rebound and the full gate rerun before local completion.

Self-development routes used: actual pre-change query/context, dirty affected advice (UNKNOWN with
explicit frontiers), and the committed self-corpus fixture. Draft maintenance was exercised by the
new corpus path; existing draft host adoption and external provider/host interoperability remain
NOT_OBSERVED. CEM/OCM and clean selected-check receipts are retained by the enrolled local completion
session. Independent review and final gate results must be inspected before completion; the build
log records the contract, not an assertion that a pending gate already passed.

Integration with current main preserves the native work-init and fixture-planning additions.
Independent delta review identified three new native option values that the corpus wrapper must
leave untouched; an option-isolation regression covers that compatibility boundary. The merged
source requires fresh canonical verification and newly bound evidence against current main.

The integration gate exposed an existing Go provider admission hang: a missing capability descriptor
could be reused as a runtime pipe, and the byte-bounded read waited indefinitely before cancellation
was installed. The failed gate was stopped and its owned processes were confirmed gone. Under
GLTP-V0-026/028/040, admission now requires exactly 32 preloaded bytes plus EOF from a pipe and uses
raw nonblocking reads. Deterministic empty/open and complete/open pipe cases failed before repair;
closed short/complete/oversized cases preserve refusal and valid capability behavior. This restores
the existing bounded-refusal contract without changing authority or qualification claims. Fresh
evidence and full verification are required after this repair.

Issue 53 adds only an opt-in native producer/reconciler for the existing issue-40 profile. Gate A
rejected a conditional fallback, a CLI-only proof that stopped before the compiler, and silent
genericization of the legacy `golf_e2e` wire member. The accepted shape therefore retains
unconditional full-suite fallback and that compatibility field, and proves mapped input through
separately retained migration/discovery/runtime artifacts into the existing Build/Open boundary.
Nested evidence objects stay closed; only outer record and scalar/list field names are mapped.

The owner clarification added while implementation was in progress makes the caller-reviewed
documentation inventory normative. Gate A was rerun before continuing. The revised result therefore
retains a normalized projection keyed by globally unique variation IDs with explicit preconditions,
actions, observable facts, expected outcomes and allowed projects. Tests carry closed semantic claims;
reconciliation compares those claims, assertions, pages/events/controls, project executions and
runtime witnesses in both directions. Source/test proposals cannot mutate that projection. Semantic
mismatch, undocumented tested behavior, documented untested behavior and missing/extra project
witnesses remain explicit fail-closed frontier rows.

Pre-change `query` selected unrelated genesis evidence and retained four omissions; tracked-path
impact selected the corpus implementation/tests with 119 omissions. Dirty `affected` selected the
corpus and CLI packages, kept language/frontier unknowns, and independently required `make gate`.
The enrolled local completion session freezes the documentation-corpus spec plus focused, full Go,
vet, interop and requirement-definition checks. Exact consumer data and live browser execution remain
NOT_OBSERVED. Independent Sol/high review found incomplete lost-link deltas, observation-subject
repair, permissive previous-result validation and incomplete orphan/runtime diagnostics. Two bounded
repair passes closed those findings, including independently testable variation-to-flow,
variation-to-test and test-to-variation losses; focused adapter and CLI tests passed after repair.
Gate B then exposed that same-revision lineage rejected ordinary historical comparison and that some
mapped-input errors named only a logical field rather than its exact JSON pointer. The owner selected
the backward-compatible interpretation of the vocabulary criterion: `golf_e2e` remains attributed
legacy caller input, while all new mapping/result/diagnostic vocabulary stays domain-neutral. The
repair admits only self-consistent earlier revisions of the same repository identities, validates
retained artifact digests, and reports exact mapped record pointers. The second Gate B pass found
three remaining diagnostic defects: compound validation could name the wrong field, trailing empty
RFC-6901 tokens were collapsed, and map iteration made multi-field refusals nondeterministic. Ordered
field decoding, field-specific validation and literal pointer composition close those cases with
regressions. Final gate and CEM/OCM reports remain pending; no acceptance, runtime authenticity,
utility, narrowing or promotion claim is recorded here.

## 2026-09-19 NTP-V0 integration with repository work-queue adoption

The owner authorized merging the verified fixture extension. Current main `d9144000` also
implements WQO repository adoption; the combined command keeps native fixture dispatch separate
from adoption and shadow observation. Both build-log entries are preserved. The fixture decision
is renumbered from 0321 to 0322 because the adoption decision already owns 0321 on main; accepted
requirements and the original prerequisite approval are unchanged. Historical review receipts keep
their original identifiers. The completed `1f639857` gate and `66c33c02` seal remain prior evidence;
integration requires a new current-main CEM/OCM binding and checks on the combined clean target.
GP, live reservations, CONFIG_PIN, admission and production promotion remain held.

## 2026-09-19 NTP-V0 native taskman fixture planning

Decision 0322 records the owner's accepted prerequisite amendment and pre-edit baseline freeze.
The opt-in `work plan-fixture` reads a native fixture journal through fixed audit/status/export
commands, then emits a source-bound priority-first plan. WQO shadow selection is unchanged.
A built task-store fixture selected P0 touching A+B over the larger P1(A)/P2(B) wave; two complete
reads were byte-identical and all 57 source/store files stayed unchanged. Exact binary, snapshot,
source hashes and output are in `.agent-evidence/native-taskman/fixture-smoke-final.json` and its
adjacent receipt/source manifest. Fixture reservations/history remain caller-owned observations.

Independent review reproduced whole-repository exclusion escaping empty resource sets, native
Code enum drift and malformed observation/history acceptance. The repairs check whole scope before
pairwise resources, preserve the closed native Code enum (DEVELOPMENT_MODE for fixture SELECTED),
and reject contradictory identity/revision/resource/history input without releasing reservations.
`internal/taskman/review_test.go` retains the failure regressions; focused planner, adapter, WQO
boundary and cancellation tests passed. The exact committed native gate and CEM/OCM/local completion
reports are post-commit evidence; these focused results alone do not close that gate.
The first full gate at `865e554b4e5a601282423dbdbbf6264dfdda1b6e` failed only
`internal/specindex`: header/digest/README wording did not exactly match the registry. The
metadata was aligned and the focused registry check rerun before rebinding; the failed gate is
retained in the enrolled check history, never represented as a passing full gate.

Self-development routes used: original prechange query (including omissions), native fixture
planning, `affected` (two Go units advised; full gate mandatory), tracked-path `prove` (97 omissions),
and the enrolled keyed dogfood workflow. New untracked-path `prove` refused; no substitute success
is claimed for it. Snapshot indexing was used by fixture planning. Batch/mutation/learning/service,
foreign adapters and live-provider qualification were not applicable to this bounded fixture slice.
Exact ATCP history remains unrecovered. GP is NOT_RUN: retired harness replacement is diagnostic
only, complete workloads/allocation/I/O/environment witnesses and real runtime conditions are absent.
No executor admission, CONFIG_PIN, production completion, real-queue cutover or performance promotion
is claimed. The explicit executor dependency handoff remains open.
## 2026-09-19 TJAA-V0-010..017 / AFP-V0-018 Playwright project-aware affected selection (Beamfall/corvint#18)

Follow-up qualification (2026-09-20): the repository-owned manifest
`internal/liveverify/affected/typescript/testdata/playwright-qualification.tsv` independently fixes
117 file identities, nine feature cohorts and 468 cases. `TestPlaywrightQualification` checks the
generated inventory before selecting, then compares seven change scenarios against cohort-based
expected project/file sets. Page object, scenario builder and helper changes each select 41 units;
one spec selects five; shared fixture, setup and config each select all 353. The 1,187 required-unit
observations have zero misses and zero extra units (file-unit recall and precision both 100% on this
synthetic corpus). Untouched cohorts are negative controls. Four tag categories per file exercise
shared, Angular, React and Chromium case identity without inferring file exclusions from grep.
Dynamic imports and unknown membership must include every one of the 353 baseline units with no
exclusions; unknown projects emit no runnable approximation and require full-config fallback.
Every scenario repeats identical canonical bytes.

The existing reporter parser and pinned test-validity projector compose retained source/config
digests with a green report: matching E2E inputs remain freshness UNKNOWN, source mismatch and stale
build remain STALE, absent input remains UNKNOWN, ambiguous envelopes are refused, and strength
remains NOT_MEASURED. This is synthetic retained-evidence composition, not a provider run or an
authenticated cross-project join. The latter depends on #19. Actual consumer-repository recall,
runtime/framework/OS qualification and full-CI execution remain NOT_RUN; this work supplies the
issue's stated fixture acceptance only and does not relabel the general adapter as promoted.

Self-development routes: pre-change `query` and `make dogfood-change` were used against
833bbd3278dc485696d5b639bf4488f5a0cfe5bc. The initial empty change correctly left CEM preparation,
intent scope and outcome incomplete; the original evidence is retained in the worktree Git evidence
directory. Provider execution is unavailable for this source-only corpus; mutation, documentation
drafting, learning changes, console and service routes are not applicable. No savings claim is made.
The first canonical gate passed the qualification tests but failed `TestIndexCoversSpecsAndHeaders`:
the README summary must preserve the exact Agent digest claim as its prefix. The follow-up restores
that prefix; the failed gate remains retained and cannot qualify the corrected revision.

The existing TypeScript adapter assigns one physical path to one generic graph unit and deliberately
keeps executable Playwright config unresolved. Issue #18 requires the same file to remain attributable
under several projects, so changing `affected-plan/0` would either violate unique path ownership or
silently change its closed bytes. The experimental `playwright-affected/0` profile instead reuses the
generic graph for physical reachability, then expands reached tests into project-distinct units.

The static config subset binds the config digest, project fragment, grep, metadata, file membership,
dependency/teardown edges, browser/device, exact project argv, and a revalidated digest of every
source input observed by the TypeScript adapter. Unsupported dynamic config or
source reachability selects every statically known test/project pair and reports
`FULL_RELEVANT_SUITE`; an unknown project set emits no runnable approximation. Runtime feature flags
and external application state stay visible on the execution axis. The mixed Chromium/Angular/React
fixture covers page-object reachability, setup/dependent/teardown expansion, config widening,
dynamic-import widening, absolute-path matchers, globstar zero-directory matching, partial project
abstention, custom fixture-based test discovery, unsupported-glob widening, transitive setup expansion,
and repeat-byte identity. Real-repository recall and provider composition
remain `NOT_RUN`, so the profile is experimental and issue #18 is not yet promotion-complete.
## 2026-09-19 WQO-V0-046..048 repository work-queue adoption (decision 0321, Beamfall/corvint#20)

Before this change, `corvint work observe` could never return `VALIDATED_AT`. `workManifest.complete`
was never set, so WQO-V0-017 always added `SOURCE_UNQUALIFIED` and `propose-wave` always abstained.
`corvint work init` and `corvint work adapter` now give any repository a committed policy,
worklist, and adapter. The observer qualifies store scope only when it reproduces the
`repository-worklist-v0` documents byte for byte.

A first cut qualified both closed mappings. It turned Corvint's own `decision-0046-v0` observations
`VALIDATED_AT` and broke the WQO-V0-021/025/032 final-check witnesses in `cmd/corvint`, which rely
on an incomplete initial capture ("initial capture did not retain incomplete scope"; three
`WQO-V0-032` codes became `MALFORMED_INPUT`). Restricting qualification to the adoption mapping kept
those witnesses and the self-dogfood contract unchanged.

Pre-landing review reproduced a symlink escape and an interrupted-write residue in `work init`.
The repair roots every write with `os.Root`, rejects a symlinked `.corvint`, delays success output,
and rolls back files created by a failed or racing initialization.

A second independent review found that init still accepted a plain directory or repository
subdirectory, a partial committed adoption without the worklist surfaced `ADAPTER_FAILED` instead
of `SOURCE_UNQUALIFIED`, and the generated adapter unnecessarily required Bash. The repair refuses
non-root targets before writing, preflights the committed adoption worklist with the other source
inputs, and emits the POSIX-only adapter with `/bin/sh`.

The first canonical-gate attempt after that repair intentionally did not qualify: the release
artifact conformance test refused the staged repair as a dirty worktree. The repair was committed
unchanged before rerunning the gate from clean, frozen source.

After the final evidence seal, current `main` advanced with the accepted corvid artwork. Merging it
correctly invalidated the pinned README workflow citation because the themed logo moved that span by
one line. The canonical gate refused the stale pin; the repair relocates it to
`README.md:188-198@3297e31e` without changing the cited workflow.

`TestWorkAdoptedRepositoryWorklist` starts from a clean fixture and runs init, a refused second
init, commit, observe, and propose-wave over four verification tickets. One is a suite batch, one a
failure-classification repair, one a test-validity receipt, and one a cleanup/retry that shares
`internal/parser`. The result is `VALIDATED_AT/UNCHANGED_OBSERVED`, `ELIGIBLE_AT` with three
tickets selected, the cleanup/retry ticket `EXCLUDED` with its collision group, and a byte-identical
repository manifest. A scratch-repository probe of the built binary recorded the unknowns:
`ERROR/SOURCE_UNQUALIFIED` with no policy, with an uncommitted adoption, and with a tracked
modification, and `ERROR/ADAPTER_FAILED` when `corvint` is absent from the fixed `PATH`.

## 2026-09-19 CRB-V0-014: owner-selected corvid artwork

The owner selected the first, corvid direction from three generated concepts and then approved
adoption ("look good. use it"). The chosen silhouette
was redrawn as editable SVG with a custom lowercase wordmark; the light, dark and universal mark
and lockup variants share geometry. The README selects a theme-appropriate lockup; the 24 px editor
icon inherits its host color. `assets/brand/README.md` records usage, raster dimensions and rollback.

Manual asset checks passed: SVG parsing, accessible titles/descriptions, no font or external-image
dependencies, PNG dimensions/alpha, shared variant geometry and README references. Independent
Codex review passed with no material findings, including comparison to the selected concept,
24 px legibility and exact SVG-to-PNG rendering parity. Requirement-index regeneration and focused
spec-requirement, requirement-definition and decision-number checks passed using an isolated index
containing the updated spec; `git diff --check` passed. Go/runtime code is unchanged; the full Go
and release gates were not run for this manual artwork slice, and installed editor qualification
remains unclaimed.

Pre-change Corvint query ran against `6a423ac091d848b8ac5b49e8002c61c252993ac3`, with three ranked
results omitted and two test-path candidates withheld. Dirty-Go advice, mutation and retrieval
evaluations are not applicable. The initial `make dogfood-change` returned `not-complete`
(`cem-prepare: git-diff-failed`, missing intent scope and outcome input); it is not passing evidence.
Post-commit CEM/OCM qualification is NOT_PRODUCED for this manual asset slice; native
CEM's PNG binary-patch limitation remains explicit. Original query, generation prompts and manual
asset hashes are retained in `/tmp/corvint-logo-20260919/` for this task.

## 2026-09-19 AFP-V0-012 fast tier: the CEM sidecar's readers, the cutover-test frontier, and the unresolved floor

Decision 0323 narrows rule (c) for `.corvint/change.cem.json` to readers whose literal resolves
to it or can form it under rule (c)'s outer partial-component semantics. The independent review
found that exact resolved-string comparison omitted a package constructing the path as
`filepath.Join("..", ".corvint/change.cem") + ".json"`; the repaired selector conservatively pairs
a naming fragment with a root-climbing or compatible root-anchored token in the same package.
These are selector-only package counts, with no tests run, on `Russells-Mac-Studio.local`,
`go1.27.1`. Selection counts come from `tools/gate-affected-select` over one `corvint affected`
receipt. The script end to end with `GO_TEST_COMMAND=:` agrees. A branch from `3f30a02` changing
`internal/touchsurprise/compute.go` plus the sidecar: 121 → 116 packages, where 116 is the count
for the Go change alone. With only the sidecar dirty: 112 → 104. `feat/build-number` against
`05e17d0` (29 changed paths): 150 → 148.

The report that "the sidecar selects nearly every package" overstates its share. The audit prints
only the first cause per package, so 105 `reader` lines do not mean 105 packages added by
literals. Of the 30 packages that named the sidecar by fixture tokens, most were also reached by
another rule. The dominant width is rule (d): 103 packages are `unresolved` at `3f30a02` and are
selected whenever any path is dirty. 37 call `runtime.Caller` or `os.Getwd` themselves. The rest
carry a root-reaching literal or depend on a package whose non-test code locates the root, 10 of
them through `cmd/corvint`. With 2 or 3 packages from the plan, any change therefore selects at
least about 105 packages.

`cmd/corvint/go_only_cutover_test.go` puts 32 packages on the frontier: `cmd/corvint` and 31
dependents. `cmd/corvint` is `package main` and has no importers. Every dependent comes from a
token edge, a literal naming `cmd/corvint` such as `go build ./cmd/corvint`, or from importers of
those packages. This is justified under the current rules. `go build` skips `_test.go`, but
`go test` and `go vet` of that package compile it, and the index cannot tell which command a
literal feeds. Those direct namers are also rule (c) readers of every path under `cmd/corvint`. A
throwaway variant stopped propagation through tokens that occur only in `_test.go` files, since
test files are never imported. It moved the single-path selection from 119 to 114. The 5 packages
it drops are `cmd/corvint-analyzer-python`, `cmd/corvint-docs-mcp`, `conformance/frontier-v0`,
`internal/dogfoodocm`, and `internal/frontiernextrepo`. It is not adopted here.

## 2026-09-18 EEP-V0 provider-to-impact workflow: synthetic fixture evaluation, and unsupported cases

`TestImpactProviderEvaluation` (`internal/extevidence/extevidence_test.go`) runs the
provider-to-`impact` composition path (`Section`) over the 5-relation mock fixture
`internal/extevidence/testdata/mock-provider.json`: 3 relations that must be admitted (one
`declared` `implements`, one `inferred` `mockdocs:enables`, one `observed` `verifies`) and 2 that
must be excluded (one `learned` relation, `EEP-V0-007`; one relation from a foreign provider
endpoint, `EEP-V0-006`). Results on commit `4a2c00f` (`Russells-Mac-Studio.local`, `go1.27.1`):
precision 1.000 (3 of 3 admitted rows expected), recall 1.000 (3 of 3 expected relations admitted),
zero false-positive relationships, zero `learned`-evidence admission, abstention accuracy 2 of 2
(the learned relation reports `excluded` and the foreign-provider relation reports `unresolved`,
both under `unknowns`, neither admitted), latency about 0.68 ms, and a 3022-byte `context.external`
section. `TestSelectionEvaluation` was rerun the same day over its existing corpora (63 cases:
see the two entries below) with the same results already on record. The fixture used here is one
record with five relations, not an independent corpus; the entries below already exercise a larger,
independent labelled set for the fail-closed selection profile. This shows the exclusion and
authority-assignment rules hold on the documented worked example; it is not an adopter outcome.

Unsupported in this slice, per `external-evidence-provider-v0.md` and `external-test-selection-v0.md`:
command, MCP, and remote provider transports (file transport only); multi-hop obligations beyond one
relation hop; and checkout worktree inspection for a V1 checkout binding (a checkout's canonical path
is echoed, never opened). None of these are measured above; none are estimated. The Change Frontier
sidecar for external obligations landed separately (EFO-V0, entry below) and is not measured here.

## 2026-09-18 EFO-V0 external obligations sidecar: reference-only join to the frontier

Decision 0313. `corvint obligations --cem FILE --impact FILE` writes `external-frontier-obligations/0`
(`docs/specs/external-frontier-obligations-v0.md`). The CEM and frontier wires are unchanged; the
sidecar cites the frontier through `binding.cem_sha256` (the `CF-V0-019` raw-copy digest) and CEM hunk
ids, and nothing reads it. Tested by `TestObligations*` in `internal/extevidence` and `cmd/corvint`;
no adopter receipt or labelled review sample exists yet, so promotion stays open.

## 2026-09-18 EEP-V2 path-to-path relations: synthetic conformance evaluation

Decision 0312. `TestSelectionEvaluation` now runs over both labelled corpora: the 23 ETS-V0 cases,
plus 40 cases in the independent two-repository fixture
`internal/extevidence/testdata/conformance-path/cases.json`, 63 in all. Ten of the path cases
cover directory scopes (`EEP-V2-012`, `EEP-V2-013`): a held path, a changed test inside the scope,
a scope that widens, and missing, dirty, other-repository, and slash-less cases that must not
narrow. Results: precision 1.000 (36 of 36 selected tests expected), unsafe narrowing 0 of 49 cases
that must not narrow, abstention accuracy 6 of 6, latency p50 about 98 ms and max about 181 ms per
case on one loaded development host, and a largest `test_selection` member of 5393 bytes. The V1 and no-provider CLI
tests keep their bytes. A V1 record carrying the same path relation stays an `unsupported`
unknown, which is why `TestAffectedSelectionPathRelation` gives `full` under V1 and `narrow` under
V2. The corpus is synthetic and was authored with the feature. It shows that the per-side and
worst-side rules hold. It is not an adopter outcome.

## 2026-09-18 ETS-V0 external test selection: synthetic conformance evaluation

Decision 0311. `TestSelectionEvaluation` over the 23 labelled cases in
`internal/extevidence/testdata/conformance-selection/cases.json`: precision 1.000 (16 of 16 selected
tests expected), unsafe-narrowing 0 of 20 cases that must not narrow, abstention accuracy 3 of 3,
latency p50 about 35 ms and max about 125 ms per case on one development host, and a largest
`test_selection` member of 4470 bytes. The corpus is synthetic and authored with the feature, so
these numbers show the fail-closed rules hold. They are not an adopter outcome, and promotion needs
a corpus drawn from a real change history.


## 2026-09-22 — OpenCode explicit MCP compatibility (V1-0022)

The installed OpenCode client sent the legacy initialize handshake and received method-not-found
from the modern-only server. A positive modern discover replay on the same binary/root isolated
protocol admission as the defect. The owner approved correcting the frozen check plan before
implementation; independent plan review resolved ordered initialization and legacy response framing.

The opt-in 2025-11-25 profile reuses the bounded shared stdio transport and all four existing native
tool registries. The first real client probe then exposed omitted tools/list params; normalizing
those at the legacy boundary produced successful real OpenCode discovery. Both failed exchanges
and the successful probe are retained in the local release evidence packet. ClientInfo versions
1.17.18 and 1.18.31 were observed separately with CLI 1.18.31. Discovery alone does not prove actual
host tool execution, sealed workflow usefulness, official conformance or formal FULL authority.
Existing docs/corpus workflows, test-validity vectors and INT/TERM descendant checks now exercise
both profiles. The original modern contract, companion distribution set and release gates remain.

The subsequent real OpenCode run executed `corvint.status` and received the READY read-only
repository receipt. The provider was an explicit deterministic localhost fixture with native
outbound-network restriction, not an inference or cost benchmark. The independent implementation
review found one completion-evidence defect: the new clauses initially followed a level-two
heading, outside OCM's Requirements parser. Moving that heading to level three preserves actual
clause enumeration; final OCM linkage and frozen checks remain required before local completion.


The dedicated OpenCode task retained that source and enrollment, then corrected the developer
preview installation: the example now uses an available local file URL instead of an unpublished
npm name. A separate native configuration enables core and test-validity MCP servers with the
explicit legacy selector. The packaged OpenCode skill routes existing tools on demand without
expanding the closed MCP registries. Independent review found no remaining protocol defect and
no blocking setup or skill issue; its forward cases preserved Unicode, non-Go and missing-test
boundaries. Executable requirement anchors now make MCPV0-021..023 directly linkable by OCM.

The 1.18.31 native probe loaded the plugin and skill and executed status, query, tracked-Go impact,
test-validity discovery, native context and caller-reported blocked-outcome tools. All repository
receipts matched the temporary fixture commit; absent retained tests stayed UNSUPPORTED. A first
probe inherited the parent PWD and therefore selected the wrong project despite subprocess cwd;
that failed fixture was retained and corrected by binding the child PWD to its actual directory.
The loopback-only deterministic provider supplies transport evidence, not model-quality evidence.
The expanded probe's INT/TERM cleanup is checked separately. Original and expanded probe receipts
remain under the private `corvint-v060-evidence/opencode-compat` and `corvint-opencode-evidence`
temporary directories. Full gates and exact final CEM/OCM closure remain separate observations.

Focused protocol/server regressions passed. Direct JavaScript invocation was refused because the
suite requires its Go-owned native fixture; the canonical host-adapter target remains the runner.
The documentation check exposed an unquoted requirement range parsed as a duplicate definition;
quoting that prose range preserves its meaning and the executable requirements. The original
failed checks remain in the private follow-up evidence.


### 2026-09-22 — patch companion qualification found an uninitialized planning branch

The exact `0.5.0a4` candidate `04e9d473` passed reproducible companion assembly and native
OpenCode's seven tool routes. Installed core qualification passed provider transitions and docs/MCP
stages, then failed planning seed with `INTENT_BRANCH_MISMATCH` against the unchanged Tasks
`e6b9d766` dependency. The planning helper created an empty `.git` directory while declaring
`intentBranch: main`; Tasks initialization correctly refused it. The same helper is present at the
public `v0.5.0a3` tag, whose published assets contain no companion/Tasks dependency. Initialize the
fixture's real Git repository on `main` before Tasks initialization, and exercise the seed with a
different caller default branch. Preserve the failed candidate and scratch as failure evidence;
rebuild and qualify the corrected exact candidate before publication. No MCP or Tasks authority
contract is weakened, and no partial installed-stage success is an overall qualification.

## 2026-09-22 AHI-002, AHI-003, AHI-010, AHI-024: Pi lifecycle and outcome repair

The owner requested a complete Pi integration audit. Independent review reproduced discarded
startup/compaction packets, hidden outcome-persistence degradation, malformed outcome JSON
silently normalized to empty input, and conflicting package/native adapter identities. Four
handler regressions failed against the original logic before repair. The extension now supplies
bounded startup/compaction recovery once through Pi's ephemeral context hook, including retries
without a new agent-start event, clears stale session/root state, rejects duplicate and mixed
identity command input, and exposes fallback persistence limits without storing automatic faults
in model history. Native invalid outcome input is distinguished from core unavailability. Package,
native translator, shim and compatibility metadata agree on 0.1.2; the host pin remains 0.85.1.

Actual Pi 0.85.1 on macOS arm64 passed an offline local-provider fixture for package install,
disable/update/re-enable/remove, two-turn ephemeral context and session non-persistence, explicit
outcome errors and startup SIGINT/SIGTERM descendant cleanup. The canonical host-adapter target now
runs the Pi JavaScript regressions. Full gate and immutable CEM/OCM completion evidence are retained
in the task's private dogfood reports, not inferred from these focused passes. No protected FULL,
interactive TUI/RPC, Linux/Windows, latency/recall or outcome-persistence claim is added.

Self-development routes used: initial query, pre-change dogfood coordination, dirty-diff affected
advice and native adapter/host tests; final CEM/OCM/frontier/finish use the enrolled plan. The first
query's measurement was not started in advance (NOT_OBSERVED); no savings claim is made. The
initial same-base coordination reported cem-prepare git-diff-failed, missing intent scope and
outcome input, retained under /tmp/corvint-pi-audit/start-dogfood.log. Non-Go path impact, provider
qualification, trace migration and mutation testing are inapplicable to this adapter repair.

### 2026-09-22 — OpenCode automatic file-change deadline

The user reported file-change `corvint-command-failed`/`timeout` on an unavailable work repository.
Against the exact `1ed0e673` binary, this repository reproduced a 500 ms default timeout while
the existing 2,000 ms override completed a receipt in 1,077 ms. These are correctness observations
under concurrent gates, not p95 results or a diagnosis of the remote repository. OpenCode now uses
the existing automatic ceiling by default and reports the actual deadline as a bound rather than
a diagnosed fault. A 750 ms valid-receipt regression fails under the old default; default hang,
explicit override, descendant cleanup, and successful FALLBACK preservation remain covered.
Independent review found no blocker. A subsequent dirty-worktree invocation still exceeded the
ceiling, so this repair makes no universal latency or absence-of-timeout claim. Query deadlines,
fault visibility and legitimate evidence degradations are unchanged.

### 2026-09-22 — installed roadmap proof followed obsolete table markup

Installed candidate `1ed0e673` passed the corrected planning seed, then failed its roadmap browser
inventory assertion. The retained console at the original fixture location renders eleven unique
ticket links and no forms, inside roadmap cards; the proof still searched for table links. Update
only the three selectors to the existing roadmap-ticket container, preserving count, detail,
Origin/Host/session refusal, state-preservation and cleanup assertions. A diagnostic copy of the
Tasks store correctly refused relocation and was discarded as qualification evidence. Browser
diagnostic attempts were interrupted and remain failed; HTTP inspection establishes this selector
defect but does not replace the required complete installed browser qualification.

## 2026-09-22 decision 0332: integrate reviewed source for the 0.6 candidate

The version slice passed its tuple, historical-identity, documentation and editor checks, then
strict completion at `055257f` and seal `7230485`. Its later sealed head is not relabelled as the
completion target. New enrollment there refused `worktree-prior-completion-stale`; a fresh isolated
checkout of the same sealed base was enrolled before source edits, preserving both generations.

The integration retains main's publication-record content (`63635b4`), reviewed MCP/OpenCode
source (`5f5b754`, `dbdda0d`), the declared-branch fixture repair (`39b61a0`), Pi fallback repair
(`b8b2ffc`), OpenCode automatic-event deadline/diagnostics (`4d1b9b7`) and the reviewed roadmap
selectors (`98d74ea`). Source-only commits preserve original references and receipts; importing
sealed sidecars would violate the new slice's CEM boundary. Build-log conflicts retain each added
record without copying unrelated alpha version changes. The requirement locator is regenerated
from the integrated specs. Later Pi protected/FULL work remains a separate lane.

The Pi input's full gate failed a query fixture restoration assertion; its focused repetitions do
not establish a fix or gate pass. The pending test backlog retains that failure and unknown cause.
Prechange query and tracked-Go impact were captured at the actual isolated root; query omissions
and the initial empty-diff coordinator failure remain in private evidence. Independent plan review
accepted the source/check scope and exact-target sequence. No new runtime behavior was designed
in this integration. Its CEM/OCMs, final gate and independent source review must bind the integrated
target; earlier input checks cannot qualify it. Optional browser/host evidence remains separately
qualified, and sealed correctness/cost and genuine dual-repository workflow evidence remain
required before owner acceptance of the exact 0.6 packet.

## 2026-09-22 MCPV0-011: literal lifecycle claim anchor

Candidate 794f93d passed the full gate, focused checks, four-archive qualification and direct
OpenCode tool/edit proofs. Native completion correctly refused an explicitly assessed MCPV0-011
claim gap: the existing stdout-close test names the requirement only in an excluded comment.
A literal named subtest now wraps the unchanged stdout-close lifecycle assertions. The wrapper
preserves all behavior, deadlines and cleanup; extraction and completion policy are unchanged.
The known gap is not relabeled unassessed. The new source invalidates target-bound qualification;
prior passes remain historical and mandatory checks must bind the replacement candidate. The
optional companion is omitted under decision 0331 because its separate installed browser stage
reached its three-minute timeout; that failure's root cause remains UNKNOWN.

## 2026-09-22 context-repository-anchors: TCP-V0-022 opt-in verbatim anchor field (V1-0084, decision 0333)

Contract: with `CORVINT_CONTEXT_ANCHORS=on`, five fixed anchor classes (quoted error string,
URL, `Scope::Value` enum, dotted config key, `file.ext:line` frame) are extracted from the task
and verified verbatim under a whole-anchor edge rule against the bounded bodies of the sources
whose `Words` postings share every word run of the anchor. Credit is the body-term BM25 form
inside the lexical slot; the reason carries the distinct prefix `anchor: `literal` xN verbatim; `;
the row keeps score 300 and authority `vocabulary`, so reserved authority rows always precede it.
Bounds: 4 to 256 bytes per literal, 16 anchors per task, 512 candidates per anchor (abstain
beyond). No index, snapshot or pack change; unset or other flag values keep the packet bytes
(`TestContextAnchorsDefaultBytes` against the recipe golden).

Decision: opt-in rather than default, the TCP-V0-019 shape, because the promotion evidence cannot
be produced on this host. Decision number: 0321 was assigned but already names the work-queue
adoption record on `main`; 0333 is used.

Evaluation: NOT_RUN. The frozen retrieval evaluation is `tools/retrieval-bench` over the external
`agent_retrieval_bench` releases (`benchmark/v2_*/…jsonl`, `corpus/v2_*`); neither directory is
present on the build host, so no flag-off/flag-on numbers exist. The bench also has no
anchor-bearing sample subset, so "anchor-bearing queries measured separately" needs a bench change
outside this ticket's ownership. The acceptance criterion is open, not met.

Gates: `go test ./internal/contextindex/ ./internal/specindex/`, `go vet`, gofmt, and the spec,
requirement, traceability, decision-number and line-citation checks; results in the ticket
report.

Audit review (IDX-SNAP-V0-017): the change adds `context_anchors.go` and edits `taskcontext.go`
and `context_terms.go` on the query side only; no extraction, fact or pack encoding changes, so
`analyzerSchemaID` stays `corvint-analyzer/73` and only the `TestAnalyzerSchemaInputs` source
digest is refreshed.
## 2026-09-22 tcq-environment-variants-and-flake-qualifier: shared flake rule and declared observation variants

Ticket `V1-0091`, decision 0339, requirements `TCQ-V0-048..050` in
`docs/specs/test-claim-qualification-v0.md`. A test observation may now declare a ResultDB-style
environment variant (`environment` object, key grammar `^[a-z][a-z0-9_]{0,63}$`, at most 32 pairs,
values at most 256 bytes); an undeclared variant is reported unknown and adds no wire member, so the
frozen `conformance/tcq-0` vectors are byte-identical. `tcq.Flaky` is the one divergence rule: more
than one distinct terminal status among `PASSED`/`FAILED`/`ERROR` across runs. `Request.PriorObservations`
(at most 16, dynamic tuple only, target-bound) lets TCQ pool row statuses per execution key across
byte-identical variants; a divergent key adds reason 18 `test-flaky` to claims that matched a row and
removes the relation while the current report state stands. The JS provider derives its Playwright
state and `flaky-retry` reason from the same rule over recorded attempts; a reporter label can only
add the qualification. Evidence: `TestObservationEnvironmentIsAdditive`,
`TestFlakyRuleNeedsDivergentTerminalOutcomes`, `TestSameRevisionDivergentOutcomesAreFlaky`,
`TestPriorObservationVariantMismatchIsNotFlaky`, `TestPriorObservationsRequireDynamicTupleAndTarget`
(`internal/tcq/flake_test.go`), `TestFlakyOutcomeIsSharedRule`
(`internal/jstestprovider/projection_test.go`). Not done here: the frontier CF-V0-016 closed
vocabulary and `docs/tcq-0.schema.json` do not yet list `test-flaky` or `observation.environment`;
the frontier shim supplies no priors, so neither can reach them today.
## 2026-09-22 V1-0009 read-only compiler qualification: case-fold collisions and a read-verb tripwire

Two hostile-path tests and one filesystem tripwire qualify the read-only Core evidence compiler
(ticket V1-0009, requirements GPK-V0-006 and GPK-V0-007; no requirement text changed). The
case-fold fixture commits `internal/token/token.go` and `internal/token/Token.go` through Git
plumbing (`hash-object`, `update-index --cacheinfo`, `write-tree`, `commit-tree`) so it exists on
case-insensitive macOS, where the worktree can hold only one file. Measured on APFS: `Build`
pins each path to its own committed blob (token.go f3c6b480, Token.go bc6664aa), never the other
case's bytes; `DirtyPaths` equals Git's ` M internal/token/Token.go`; `query` and path `impact`
report freshness `mixed-worktree` naming that path and `learning.local_trace_state`
`blocked-mixed-worktree`; `context` returns both rows under distinct blob hashes; range `impact`
refuses `unsupported-impact-worktree` (exit 2); two consecutive runs of every verb are
byte-identical. No verb silently picks one file, so no kernel change and no `t.Skip` was needed.
The case-sensitive branch of both tests (no dirty path, `fresh`, range impact accepted) is
written but not measured locally. `TestReadOnlyVerbsWriteNothing` snapshots the whole fixture
tree, `.git` and `.corvint` included, as path to kind, mode, sha256 and mtime before and after
`init`, `adopt`, `query` (with and without the `unplanned-reads.enabled` marker), path `impact`,
`docs draft`, `harness event` session-start and stop, and `lrf`; the only permitted delta is the
one self-observation row on session-start with `.corvint/` gitignored. A negative run over
`index` tripped the comparison on the index files, so the tripwire is live. No decision was
recorded: tests alone need none, so the reserved number 0342 stays unused. Gates run: gofmt, `go build ./...`, `go vet ./...`,
`go test ./cmd/corvint/ -run 'ReadOnlyVerbs|CaseFold'` (also `-count=5`),
`./internal/contextindex/...` and `./internal/specindex/` fully, and the spec-requirements,
requirement-definitions, traceability-tests, decision-numbers and line-citations checks. Open
observation for follow-up: the `context` receipt carries no freshness block, so the worktree
divergence reaches a `context` caller only through the blob hashes, not a named state.
## 2026-09-22 stable-operations: V1-0017 lifecycle check, hostile matrix, support window, command runbook

Decision 0341 and `docs/specs/stable-operations-v0.md` (`SOP-V0-001`..`012`). Two additive shell
checks, no Go change. `script/check-install-lifecycle.sh` ran the eight steps against a built
binary in 2.5 s and its wrapper (two stamped builds, archive path, tampered `SHA256SUMS`, usage)
in 9.3 s. `script/check-hostile-regressions.sh` ran 27 rows over 12 packages, all PASS, in 28.9 s
with `memory` and `case-folds-context-index` printed NOT_COVERED; its stub-driven wrapper passed in
4.5 s. Independent finding against the ticket wording: a truncated or byte-damaged snapshot does
not fail closed; the read verb exits 0 with byte-identical packet output and leaves the file
untouched, and `index --if-stale` rebuilds a same-size snapshot that differs in 460 bytes, so the
spec fixes packet identity as the recovery invariant and leaves snapshot byte identity to
`index-snapshot-v0.md`. The brief presumed case-fold and interruption cleanup uncovered; both have
tracked regressions and sit in the matrix. Evidence is darwin arm64 only; other hosts NOT_RUN. No
frozen evaluation applies to this operations slice. Proposed `make` targets
`install-lifecycle-test`, `hostile-regressions-check`, `hostile-regressions-test` are not added
here. `docs/SECURITY.md` still says the support window is a draft owner decision and is stale
against `SECURITY.md`.
## 2026-09-22 cem-0-3-structural-mechanical: Go structural mechanical reasons the verifier re-proves

Ticket V1-0087, decision 0338, spec `docs/specs/cem-0.3-structural-mechanical.md` (`CEM-SM-001..010`).
`cem/0.3` is `cem/0.2` plus `rename`, `move`, `import-reorder`, and `formatter-only`; 0.1 and 0.2
maps still reject that vocabulary, `prepare` still emits 0.2, and `mark` upgrades only when a
structural reason is used. Each reason is recomputed from the base blob and the patch with the Go
standard library and refused as `unproven-mechanical` on any parse or format failure. Fixture
pairs cover a true positive and a near-miss per class (a hidden `"hello"`/`"hi"` edit, a
`ToUpper`/`ToLower` swap, a shadowing rename, a `helper(21)`/`helper(22)` edit), plus selector,
exported, directive, `var`-order and `init`-order refusals. One admitted overlap: gofmt sorts
imports, so an import reorder is also formatter-only. No frozen evaluation exists for mechanical
classification precision; the reviewer-audit kill gate in `docs/CHANGE-EVIDENCE-MAP.md` (20%
false classification) is the only measured bar and was not exercised here. Not shipped: a
`cem-0.3.schema.json` and `protocol/cem-0.3` vectors (Apache-2.0 boundary), frontier and dashboard
profile acceptance, exported or type-aware renames, non-Go languages.
## 2026-09-22 issue 64 close-out: `deleted` verification, `affected --provider-command`, vocabulary mapping

Issue Beamfall/corvint#64 (generic revision-aware provider contract) was audited in 54fe6c3 as
mostly delivered by EEP-V0/V1/V2, ETS-V0/V1 and EFO-V0; this change closes the actionable
remainder. `EEP-V0-010` gains `deleted`: an untracked path endpoint whose path the record's
declared revision tracked, decided by one extra `git cat-file --batch-check` per view per
repository over the untracked paths only, and only when freshness is `repository-ahead`,
`provider-ahead`, or `unrelated-history` (so a `revision-unavailable` record can never claim
deletion). `deleted` is stale for `EEP-V1-008` and keeps the ETS code `missing-path-reference`,
so no selection wire changes (`TestReferenceVerificationDeleted`,
`TestTwoRepositoryDeletedTestPath`). `corvint affected` now takes `--provider-command ARGV_JSON`
under the same parser and bound as `--provider` (`EEP-TR-001`, `ETS-V0-001`,
`TestAffectedProviderCommandMatchesFile`). The issue's freshness vocabulary is mapped onto the
accepted wire names rather than renaming them (recorded in `docs/EXTERNAL-EVIDENCE-PROVIDERS.md`):
`provider-is-ancestor` is `repository-ahead`, `reference-missing` is `missing`/`deleted`,
`not-observed` is `not-verified`, and `reference-ambiguous` needs symbol identity (V1-0101).
Ticket V1-0104's premise was wrong: `--provider-mcp` never existed and the guide's MCP bullet was
accurate; the guide's stale bullets were the two ETS-V1 items (one-hop widening, no checkout
inspection), now corrected. V1-0101, V1-0102, V1-0107 and V1-0108 stay open. Port note: this entry was
reapplied from the old lineage onto the public history, where `impact --provider-mcp` does exist
(decision 0324); the guide's MCP bullet on this lineage is a pre-existing follow-up, not part of
this change.
## 2026-09-22 ocm-v0-conformance-vectors: freeze the OCM V0 proof wire in `conformance/ocm-v0/`

Ticket V1-0013, decision 0343, requirement `OCM-V0-014`. The suite freezes 25 structural vectors
(5 valid, 20 hostile) and 4 fixtures with 19 verifier cases for `ocm/0.1-experimental`. Valid
vectors are the byte output of the real `prepare`, `link`, and `mark` producers over a
deterministic seed repository with pinned Git identity and dates; `TestFrozenVectorsMatchTheRealProducer`
rebuilds all 5 universes on every run and requires byte equality. Every vector runs through the
real structural parser and every fixture case through the real `status` verifier; no double is
used. All 25 declared refusal codes matched the parser on the first run and no vector exposed a
defect, so `internal/lrfrepo` is unchanged. Two outcomes are frozen as observed: an extra top-level
member refuses with `unknown-field` (closed object), and a bound OID absent from the repository
refuses with `repository-object-unavailable`, ahead of `target-mismatch`. The package test runs
in about 24 s on a quiet host, dominated by Git subprocesses for the 9 universes it builds.
## 2026-09-22 batch-A cmd/corvint chores: V1-0030 V1-0047 V1-0048 V1-0051 V1-0052

Five queued `cmd/corvint` chores closed together, all read/write behavior only, no wire or
requirement-ID change. V1-0047: `taskman fixture`'s stdout-write failure now emits the same
`output-failed` error envelope as every other command instead of a bare stderr line
(`taskman_fixture.go`); the stale plain-text assertion in `taskman_fixture_test.go` and a new
case in `output_write_failure_test.go` cover it. V1-0048: the corpus relay's stdout-copy-failure
path (`corpus_integration.go`) no longer appends a second `output-failed` envelope after a failed
native command has already written its own; `docs_corpus_test.go` asserts the resulting stderr is
byte-identical to the native-only baseline. V1-0052: `docs_corpus_test.go` gained a pinned test for
`--root --corpus=FILE`, confirming `--corpus` is not consumed as `--root`'s value and the native
command instead refuses it with `--corpus is supported only on native evidence reads`. V1-0051:
documented the operator-set `CPUPROFILE` env knob (`context` and the harness-event path) in
`help.go`'s `context`/`harness event` help text and in `task-context-packet-v0.md`'s Non-goals and
authority section; no flag added, no requirement ID touched. V1-0030: the advertised
`docs draft`/`docs consume` example (`docs.go`, `source-documentation-draft-v0.md`) referenced
`internal/doccompiler`, which exceeds the SDD-V0-005 64-declaration bound and fails
`documentation-limit-exceeded` when actually run; replaced with `internal/docmaintain --task
Preview`, verified live against the built binary. `TestDocsHelpPrerequisitesAndWorkingExample` no
longer runs the parsed example against a synthetic fixture package that happened to export only
one declaration; it now runs `--root $(cd ../.. )` against this repository's own committed HEAD
source, so it would have caught the original bound violation.

Gates run: `gofmt -l` on the changed files, `go build ./...`, `go vet ./...`, the targeted tests
above plus `TestImpactAndHarnessHelpExposeActualLimitsAndUnsupportedProfiles` and
`TestSupportBoundaryDisclosesTheSelfObservationLedgerWrite`, `internal/specindex`'s
`TestIndexCoversSpecsAndHeaders`, and `make spec-requirements-check requirement-definitions-check
traceability-tests-check decision-numbers-check line-citations-check` — all pass. `REQUIREMENTS.tsv`
unchanged (prose-only spec edits, no requirement IDs touched). No decision record: no published
wire contract, spec bound, or refusal vocabulary changed; the envelope fixes bring two call sites
into line with the pattern already used elsewhere in the same files. Full `go test ./...` was not
run for this scoped batch (per `AGENTS.md`'s Verify section, exhaustive gate reserved for the
terminal boundary); only the targeted tests above and the listed `make` checks were executed.
## 2026-09-22 batch C: bounded lists, published bounds and redundant-parse cleanup

Six scoped fixes closed from the agent-memory backlog. V1-0043 (BBF-V0-010): `validateControl`
now caps `UnrelatedCriteria` and `RequiredSetup` at 1000 entries each (`maxControlListLength`,
decision 0336), so `acceptedReceiptBound` can no longer exceed the 32 MiB document bound; math and
rollback are in `docs/decisions/0336-behaviorfalsify-control-list-cap-2026-09-22.md`. V1-0046
published both previously-unstated bounds next to their spec rows with no new requirement IDs:
`externalMaxConfigInputs` (256) beside PWP-V0's `config-inputs-unobserved`, and the 10s
`cleanupReserve` beside BBF-V0-010; PWP-V0's row also notes the current code path actually refuses
an over-count as `report-output-overflow`, not `config-inputs-unobserved`, ahead of the ticket's
framing. V1-0045: `RunUnit`'s report read now uses its own `unitReportOutputLimit` (16 MiB, equal
to `defaultOutputLimit`) instead of the external provider's 4 MiB `externalOutputLimit`, cited in
`js-live-test-provider-v0.md`'s `report-not-written` row. V1-0056: `qualified-reporter.cjs` now
memoizes `--version` per executable path (`observedVersions`, mirroring `bundledBrowsers`); Go's
`sensitive_input_boundary.go` builds one `strings.Replacer` per receipt instead of one
`ReplaceAll` per sensitive value per field, preserving the existing longest-first prefix ordering
(`TestSensitiveInputPrefixOverlappingValuesRedactLongestFirst` still passes unmodified in intent,
call-site signature only). `application_attestation.go`'s "read the executable three times" item
does not apply to the current file: it has exactly two content reads, at prepare-time (stage and
digest) and inside `unchanged()` (later drift re-check), which are semantically required to happen
at different times and cannot be merged without breaking drift detection; left untouched. The "JS
emits paths only" companion item was skipped per the batch brief, since it is not a pure removal.
V1-0058: `internal/extevidence/mcp.go` dropped the two `mcpObject(...)`-then-`strictMCP(...)`
redundant pairs whose `mcpObject` result was already discarded (`mcpResponse`'s envelope decode,
and the initial handshake's `info` decode), since `strictMCP` alone already re-derives the same
duplicate-key and unknown-field checks; content/isError decoding, which uses its `mcpObject`
return value, is unchanged. V1-0057: `candidateAdjacency` (`internal/workqueue/proposal.go`) now
calls a new allocation-free `overlaps` helper instead of `len(intersection(...)) == 0`, dropping
the per-pair slice allocation and sort; `TestCandidateAdjacencyGroupOverlap` pins the adjacency
edges. All six changes keep existing test suites green; behaviorfalsify, jstestprovider,
extevidence and workqueue package tests all pass. UNKNOWN: whether the PWP-V0 `config-inputs-
unobserved` naming mismatch found while doing V1-0046 needs its own ticket, versus being purely a
documentation clarification — left as a note rather than filed separately.
## 2026-09-22 claude-code-compaction-pin-hooks: PreCompact/PostCompact pin verdict for V1-0094

Decision 0340 registers `PreCompact` and `PostCompact` on the Claude Code plugin (AHI-026 to
AHI-030). The hook names, payload fields and stdout routing were read from the installed Claude
Code 2.1.267 hook runner; no further compaction event exists there to register. `pre-compact`
prints a `corvint-compaction-pin/0` line (HEAD tree, dirty-path counts, at most 24 tracked dirty
paths) that the host joins into the compactor's instructions; `post-compact` re-validates the pin
the summary preserved, checks the tree and each path with one hermetic read-only `cat-file`, and
prints a `corvint-compaction-report/0` line naming every non-rehydratable path. The host shows that
report to the user only, so the model-facing rehydration remains `SessionStart(source=compact)`,
which now opens with a disclosure saying so; a host without the events ignores the registration
and that disclosure plus `compatibility.json` `compactionHooks` keep the gap visible. Evidence:
`TestAHI026`..`TestAHI029` in `cmd/corvint/host_adapter_compaction_test.go`, the AHI-030
assertion in `TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths`, and the extended
`TestAHI017AdapterHostKillMatchesDeclaredHooks`. Live compaction cycle NOT_RUN; black-box status
STATIC_ONLY; no frozen evaluation fits (the CEP §3 gate is an unrun 30-task three-cycle trial).
The plugin version stays 0.2.2 because `integrations/host-adapters.test.mjs` binds it to the
shared compatibility matrix this change does not own.
## 2026-09-22 opencode-mcp-verify: V1-0022 verified at HEAD with real OpenCode sessions

The ticket's repair, the explicit `--protocol-version 2025-11-25` profile (`MCPV0-021..023`), was
already on the public lineage before this batch; this entry records the verification of the
current tree against freshly built `corvint-mcp`, `corvint-docs-mcp` and
`corvint-test-validity-mcp`. Replaying the captured OpenCode `initialize` frame
(`protocolVersion` `2025-11-25`, `capabilities.roots` `{}`, `clientInfo` opencode) without the
selector still returns `{"code":-32601,"message":"Method not found"}`, the frame the owner report
reduced to; with the selector it returns `protocolVersion` `2025-11-25`, `serverInfo` and the
tools capability, then `tools/list` succeeds. Two installed clients, `/opt/homebrew/bin/opencode`
reporting 1.18.31 and `~/.opencode/bin/opencode` reporting 1.17.18, each ran an isolated
`opencode mcp list` (three servers `connected`) and a non-interactive `opencode run` session
against a deterministic loopback provider under a native outbound-network sandbox. Each session
completed `corvint.status`, `corvint.query`, `corvint.impact`, `corvint.test_validity`
(`discover: true`, evidence absent, `UNSUPPORTED`) and `corvint.docs_draft`; every repository
receipt pinned the fixture commit. Both clients send `notifications/cancelled` for every
`tools/call` after its response; the server ignores them, now pinned by
`TestMCPV0022LegacyCancelledAfterCompletionIsIgnored`. A first docs call refused a fixture whose
owner Markdown lacked an Agent digest (`unsupported-documentation-source`), a visible tool
refusal, not a transport failure. The provider was a transport fixture, not model evidence; the
owner's original failing machine and configuration remain UNKNOWN, and no FULL host authority is
claimed. Raw frames and receipts are retained under the session scratchpad `opencode-v1-0022/`.
## 2026-09-22 gate-affected floor and dogfood rg dependency: V1-0059, V1-0081, V1-0038, V1-0031

V1-0059: `link()` in `tools/gate-affected-select/readers.go` wired a string literal naming a
package directory as an importer edge whether or not the literal sat in a `_test.go` file. A test
file is never imported, so its holder's importers can never legitimately be reached through it.
`link()` now sources that componentRuns edge from a new `nonTestHolders` index (test-only literals
excluded) while the special root-module literal `"/"` case keeps using every holder, since
`componentRuns("/")` has no fallback and narrowing it silently drops the holder rather than just
its closure. New fixture `TestSelectPackagesStopsClosureAtTestOnlyTokenHolder` in `main_test.go`
pins the behavior. Measured at commit `3f30a02`: global `-unresolved` floor 130 → 125 packages (net
5 fewer: `cmd/corvint-analyzer-python`, `conformance/frontier-v0`, `internal/dogfoodocm`,
`internal/frontiernextrepo`, `benchmarks/selfuse-batch`, all previously unresolved only via a
test-file literal falsely propagating `cmd/corvint`'s unresolved status to its dependents).

V1-0081: with the V1-0059 fix applied, a one-package dirty-path selection
(`cmd/corvint/go_only_cutover_test.go`) narrowed from 143 to 139 of 218 total packages (65.6% →
63.8%), still far short of "well under half." Root cause: of the 125 packages left in the
`-unresolved` floor, 96 (44% of the whole module) self-locate directly — they call `os.Getwd` /
`runtime.Caller` or carry an escaping literal in their own non-test source — and are correctly
fail-closed under rule (d); only 29 are propagated through the importer/dependents graph, the only
part lever (3) can touch from inside `tools/gate-affected-select`. Lever (1) (a shared bounded
root-location helper) is out of ownership. Lever (2) (drop `-p 1` in `script/gate-affected.sh` when
the union is wide) was considered and rejected: `AGENTS.md` documents `cmd/corvint` panicking under
concurrent load at its current ~591s serial runtime, and `cmd/corvint` is selected in nearly every
plan, so removing serial package execution risks reintroducing that instability for a speed gain
that does not move the selection-count AC. Disposition: PARTIAL — lever (3) applied and measured
(130→125 unresolved, 143→139 one-package selection), AC unreachable within ownership because 96/218
packages self-locate directly in source outside `tools/gate-affected-select`.

V1-0038: `cmd/corvint/dogfood_record.go`'s `os.Getwd()` (line 40) is not the only reason `cmd/corvint`
is unresolved. `grep -rln 'os\.Getwd\|runtime\.Caller' cmd/corvint/*.go | grep -v _test.go` lists 16
files with real, independent calls (`dogfood_record.go`, `frontier.go`, `host_adapter.go`,
`local_completion.go`, `pi_adapter.go`, `eval.go`, `init_adopt.go`, `pi_tools.go`, `lrf.go`,
`migrate_traces.go`, `main.go`, `ocm.go`, `work.go`, `record.go`, `source_handoff.go`, `witness.go`).
Bounding only `dogfood_record.go`'s read cannot make `cmd/corvint` leave `-unresolved`; the other 15
files keep the package fail-closed regardless. Swapping `os.Getwd()` for the lexically-unflagged
`filepath.Abs("")` (the pattern `resolveExplicitRoot`/`normalizeRoot` already use in `main.go`) was
rejected as gaming the detector rather than genuinely bounding the read. Disposition: NOT DONE; the
stated AC needs a coordinated pass over all 16 files, out of this ticket's single-file scope.

V1-0031: `script/dogfood-check.sh`, `script/dogfood-change.sh`, `script/dogfood-bind-range_test.sh`,
and `script/dogfood-change_test.sh` (89 call sites total, not only the two files the ticket named)
call `rg` with no preflight; a host without it got a bare "command not found" partway through a run.
Each of the four now fails closed immediately after its `set` line with `REFUSE
unsupported-environment-missing-rg` when `rg` is absent from `PATH`, verified by running all four
with `rg` stripped from `PATH` (each refuses at exit 1 before any Git or build work starts) and
unchanged (`dogfood-bind-range_test.sh`, `dogfood-change_test.sh` both still exit 0) with `rg`
present. `docs/DOGFOOD.md` §4 now names `rg` as a prerequisite for `dogfood-change`/`dogfood-check`.
Disposition: DONE.

Gates run: `gofmt -l tools/gate-affected-select cmd/corvint/dogfood_record.go` (clean); `go vet
./tools/gate-affected-select/... ./cmd/corvint/...` (clean); `go test -count=1
./tools/gate-affected-select/...` (pass, includes the new fixture); `bash -n` on all four edited
scripts (clean); `shellcheck -S warning` on all four (clean); `script/dogfood-bind-range_test.sh`
and `script/dogfood-change_test.sh` full runs (both exit 0); `make spec-requirements-check
requirement-definitions-check traceability-tests-check decision-numbers-check line-citations-check`
(pass, no published-contract change). No `cmd/corvint/*.go` file was edited, so its own suite was
not rerun.
## 2026-09-22 flaky-batch-b: detached Git auto maintenance and ctime-tick witnesses

Five load-dependent `cmd/corvint` failures (V1-0032, V1-0034, V1-0035, V1-0061, V1-0071) share
one confirmed mechanism: on this host (git 2.54.0, no global config) every `git commit` spawns
`git maintenance run --auto --quiet --detach`, a grandchild that outlives the commit, holds
`.git/objects/maintenance.lock`, and can write `.tmp-<pid>-pack-*` files. Its writes race the
fixture byte digests, the materialization manifests, `t.TempDir` removal (`.git: directory not
empty`) and the work adapter runner's process-residue and 250ms pipe-drain checks. Setting
`maintenance.auto=false` and `gc.auto=0` removes the spawn entirely (GIT_TRACE=1: zero
maintenance processes across repeated commits). The specific pack write that V1-0061 recorded
was not reproduced here; that it comes from the same detached process is inferred, not observed.
Fixed in-scope: the affected fixture, the materialization fixture and the two committing
final-check scripts now disable both settings, and the runner keeps its last failure cause and
receipt in memory so the fatal message can name them (the command result and wire are unchanged).
Integration follow-up: `queryCLIRepository` in `cmd/corvint/query_test.go` now also disables
both settings (the same two `git config` lines as the affected fixture), which removes the
detached-maintenance cause behind V1-0061 and V1-0071; the drift restoration fatal now prints the
differing paths. `workDrainTimeout` (250ms) stays as the WQO-V0-034 pin. Repetitions: the five
named tests `-count=3` pass, the wider work/affected/query set `-count=1` passes.

Three ctime witnesses (V1-0033: `internal/trace`, `internal/authoritystore`,
`internal/cem/gitauth`) assumed the change time advances between adjacent syscalls; on a coarse
ctime clock the same-bytes restore lands in the tick of the original write. Each test now
re-applies its mutation until ctime differs, bounded at 2s, and skips otherwise. Product
limitation recorded: a restore that completes within one ctime tick is invisible to the witness.
`TestRunningFailedPassed` (V1-0036) waited 5s for each running event under whole-suite load; the
waits are now the 5-minute hang detector already used for completion (decision 0082). All four
tests pass `-count=5` with no skips on this host.
### 2026-09-22 batch D: contextindex/doccorpus fixes and decision 0337

Six V1 tickets landed in one change. `internal/contextindex`: V1-0049 rewords `citedNumbers`'s doc
comment to name its two malformed-range degradations explicitly and adds a table test pinning
`path:5-0`/`path:9-3`; V1-0053 hoists the two `regexp.MustCompile` calls in `eval_query.go` to
package vars; V1-0054 replaces `impact.go`'s per-changed-path linear scans over `index.Sources` with
a directory-to-paths index and a sorted path slice built once per call, output verified byte-
identical against the prior implementation via the package's existing tests; V1-0055 caches
`authority_trigger.go`'s per-target line splits, builds `range_impact.go`'s map before its linear
scan instead of after, and merges the two-spawn `cat-file -t`/`rev-parse base^{tree}` open of
`compileRangeImpact` into one `git rev-parse base^{commit} base^{tree}` call (not `--verify`, which
this Git version refuses with more than one revision argument) while leaving `verifyRangeBase`'s own
closing repeat of the same two-spawn pattern untouched, per the ticket. `TestAnalyzerSchemaInputs`'s
structural digest pin moved `e9058d1...` to `bca73e3...` (schema ID `corvint-analyzer/73` unchanged)
to reflect these edits.

`internal/doccorpus` (decision 0337, DCP-V1-032 amended): V1-0044 — `reconcileTest` (forward
emission) already retains a criterion or claim naming a variation absent from the normative set and
reports it as `undocumented-tested-behavior` (DCP-V1-031) rather than pruning it, but
`validatePreviousBehaviorAdapterResult` (the `--previous` strict check) fatally refused that same
retained shape, so a `run1.json` carrying exactly the finding DCP-V1-031 exists to report could not
be reused for the DCP-V1-032 delta. The validator now tolerates it on the same terms forward emission
does — the dangling reference must still be a criterion the test itself declares — and a new
`TestBehaviorAdapterPreviousToleratesDanglingCriterion` end-to-end test exercises run 1 (dangling
criterion, exits 0 with the `undocumented-tested-behavior` finding) then run 2 (`--previous` run1,
exits 0). V1-0050 — `lost_reverse_links` entries were joined with a literal NUL, which `textOK`
forbids in every other corpus text field; the six join sites now use a new `reverseLinkJoin` helper
(printable `|` separator, backslash-escaped where a field contains `|` or `\`, preserving the same
collision-freedom the NUL separator gave), and `validatePreviousBehaviorAdapterResult` now runs
`textOK` over every `Delta.LostReverseLinks` entry on read.

Owner question V1-0060 (should a prior bundle with dangling criteria disqualify a delta outright
instead of being tolerated) remains open; this batch implements the tolerant reading pending that
answer and does not close it.

Gates: `gofmt -l`, `GOTOOLCHAIN=local go build ./... && go vet ./...`, targeted
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/contextindex/... ./internal/doccorpus/...
./internal/specindex/`, and `make spec-requirements-check requirement-definitions-check
traceability-tests-check decision-numbers-check line-citations-check` all passed. Full `make gate`
was not run, per batch scope.

## 2026-09-22 patch-coverage-witness: `cem cover` records a per-hunk coverage witness from one local coverprofile and `cem report` downgrades unwitnessed test claims

Ticket V1-0085, decision 0347
(`docs/decisions/0347-patch-coverage-witness-from-a-local-coverprofile-2026-09-22.md`),
`TCQ-V0-051..054` in `docs/specs/test-claim-qualification-v0.md`. The TCQ YAGNI paragraph's
"coverage ingestion" exclusion is replaced by the four requirements, the acceptance matrix gains a
`coverage witness` row, and the rollback paragraph names how the slice comes out.

What changed: `internal/cem/wire/map.go` admits one optional closed-key hunk member `coverage`
(`{profileSha256, testRun, mode, state, covered}`) on `cem/0.3` only, with ranges required to be
ascending, non-adjacent, inside `newRange`, and consistent with `state`; `cem/0.1` and `cem/0.2`
keep rejecting it as `unknown-field`, so no fixture, conformance vector, `protocol/cem-0.2`
schema, or `interop/cem01-go` consumer changed. `internal/cem/workflow/cover.go` adds
`Session.Cover` and `internal/cem/cli/cli.go` the `cem cover --map --coverprofile --test-run
[--output]` action: one operator-named local profile, bounded at the gorunner coverage bound,
parsed by the gorunner parser now exported as `ParseCoverProfile`/`ParseCoverageBlockLine` (no
behaviour change to live verify), intersected with each hunk's added lines (diff-cover semantics),
and written on every hunk as `covered` or `uncovered`; the map is upgraded to `cem/0.3` as a
structural `mark` does. `internal/cem/workflow/read.go` adds a `## Test claims` section to
`cem report` listing each `test-claim` hunk as `tested` or downgraded with reason
`no-coverage-witness` / `coverage-witness-uncovered`; dispositions, counts, worklist and the
`status`/`verify` envelopes are unchanged.

Measured: four new tests (`TestSpec03CoverageWitness`, `TestParseCoverProfileExportsBlocks`,
`TestCoverRecordsCoverageWitnessAndReportDowngrades`, `TestCoverRefusesAmbiguousAndInvalidInputs`)
pass; package runs `internal/cem/wire` 1.7s, `internal/cem/workflow` 87.3s,
`internal/liveverify/gorunner` 24.0s, `internal/specindex` 0.9s, all `ok` at `-count=1`.

NOT MET / UNKNOWN: `corvint cem cover --help` and the cem help text do not list `cover` because
`cmd/corvint/help.go` (`cemHelpActions`) was outside this change's ownership; the command
dispatches. `docs/specs/cem-0.3-structural-mechanical.md` still describes the profile as adding
only the structural reasons and was not amended (not owned). No live `go test -coverprofile`
end-to-end run against a real repository was performed; the workflow test uses a synthetic
profile whose path suffix matches the hunk path. Reporter and labelled-corpus promotion gates of
the TCQ spec remain NOT_RUN.

Gates: `gofmt -l` (nothing), `GOTOOLCHAIN=local go build ./... && go vet ./...`, targeted
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/cem/... ./internal/liveverify/gorunner/
./internal/specindex/`, and `make spec-requirements-check requirement-definitions-check
traceability-tests-check decision-numbers-check` passed. `make line-citations-check` fails on 18
pre-existing citations in `docs/specs/falsifiable-packet-v0.md`, `docs/decisions/0082-*.md` and
`docs/specs/go-production-kernel-migration-v0.md` that cite `internal/contextindex` and
`cmd/corvint` lines this change does not touch; they were not repinned because those files are
outside this change's ownership. Full `make gate` was not run, per ticket scope.
## 2026-09-22 packet-trust-class: one `trust` class per cited row; tainted rows satisfy no basis

Ticket V1-0090, decision 0346 (proposed, experimental delivery), TCP-V0-023 and FPK-V0-032.

What changed: `internal/contextindex/trust.go` holds the closed five-class enum
(`project-authority`, `repository-content`, `repository-history`, `external-provider`,
`tool-output`), the one derivation table `trustByAuthority` keyed on the existing `authority`
label (an unlisted label is `tool-output`), and `TrustTainted`. `context` stamps `trust` on every
`results[].evidence[]` row, computes `governance` and `critical` over the non-tainted reserved rows
only, and adds the always-present `coverage.governance_refused` array naming each refused row.
`prove` (`cmd/corvint/prove_trust.go`) stamps `trust` on every `proof.rows[]` entry through the
same table; a tainted row keeps falsifier `none` (never `PASS`, never `proven_results`) and carries
a `refusal` naming the row. No new input is read; the `query`/`impact` wires, the `external`
section and the CEM ledger readers are untouched.

Measured: the recipe golden `internal/contextindex/testdata/context-recipe-default-golden.json`
re-captured with exactly 12 added `"trust"` members (11 `repository-content`, 1
`project-authority`) and one added `"governance_refused": []`; no other byte changed. The
`TestAnalyzerSchemaInputs` audit digest was repinned (consumer-only change, schema stays
`corvint-analyzer/73`, as the two prior repins did). Every row of the `prove` fixture proof is
untainted and unrefused (`TestProveRowsCarryOneTrustClassAndOldConsumersDecode`). Old-consumer
decoding covered for both wires by decoding the previous struct shapes and comparing canonical
JSON with the new members deleted.

NOT MET / follow-ups (text only, no tickets filed): the `query`/`impact` evidence rows carry no
`trust` (byte-exact under GPK-V0-002 and `conformance/cli-parity-v0`; needs its own amendment);
`internal/extevidence` external-section rows are not stamped (outside this change's ownership);
`prove checkpoint` claimed-authority rows are not classified; no external consumer has exercised
the new members.

Gates: `gofmt -l` (nothing), `GOTOOLCHAIN=local go build ./... && go vet ./...`,
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/contextindex/... ./internal/specindex/`
(ok, 134.5s and 0.2s), `cmd/corvint -run` over the two new prove tests plus the twenty context-,
answerability- and prove-row tests the wire change touches (ok), and `make spec-requirements-check
requirement-definitions-check traceability-tests-check decision-numbers-check` all passed.
`make line-citations-check` FAILS on 18 citations (decision 0082, `falsifiable-packet-v0.md`
rows citing `internal/contextindex/impact.go`, and `go-production-kernel-migration-v0.md` citing
`range_impact.go`); the same 18 fail with the index read from base 4519cad and none names a file
this change touched, so they are pre-existing from the batch D commit and were not repinned here.
Full `make gate` was not run, per ticket scope.

## 2026-09-22 gate-repair: DR-0039/DR-0040 `cem` choice-list extensions declared and the CEM trust citation repinned

The full `make gate` on the V1-0085 tree (base 1603d4a) reported two failures; repairing the first
exposed a third of the same shape. None touched a frozen expectation.

- `conformance/cli-parity-v0` `TestGPKV0002ManifestReplay`: `cem-mark-invalid-reason` failed
  `stderrSha256` (candidate `531926b2…`, manifest `b03c3c67…`) because decision 0338 added the four
  `cem/0.3` structural reasons to the `--reason` choice set (`CEM-SM-001`). Once declared, the next
  case `cem-invalid-subcommand` failed the same way (candidate `bab3d5ea…`, manifest `2a7db57b…`)
  because decision 0347 added `cem cover` to the action list (`TCQ-V0-051`). Both are recorded as
  intentional Go extensions in `conformance/divergence-register.md` (`DR-0039`, `DR-0040`) with the
  exact candidate and oracle bytes, declared as one-rewrite `stderrRewrites` `knownDivergence`
  entries in `manifest.json`, and pinned by `validMarkReasonDivergence` and
  `validCEMActionDivergence` in `manifest.go`. Measured: applying each single substitution to the
  candidate stderr reproduces the frozen digest byte-for-byte; the `SUMMARY` moves from
  `known-divergences=24` to `known-divergences=26`; the `cem` inventory row reports 19 byte-exact
  cases and names both declarations. The `flag provided but not defined: -candidate` text in the
  package output is emitted by the passing `TestCaptureCLIRejectsCandidateAuthority`, which expects
  that refusal; it is not a failure.
- `conformance/release-artifact-v0` `TestReleaseNotesCEMTrustCitationLandsOnTrustRoots`:
  `docs/RELEASE-NOTES-alpha.md` cited `docs/CHANGE-EVIDENCE-MAP.md:226-227@012d2dcc`, which
  commits 5ab91e3 and 1603d4a moved to lines 241-242. Repinned to `241-242`; the `@012d2dcc` anchor
  is unchanged because `script/check-line-citations.sh --hash docs/CHANGE-EVIDENCE-MAP.md:241-242`
  reproduces it. No other document cites `docs/CHANGE-EVIDENCE-MAP.md` by line. The two declarations
  shifted lines in `manifest.json` and `runner_test.go`, so `docs/decisions/0051-*.md:24` and
  `docs/specs/compat-replay-runner-v0.md:17` were repinned to `manifest.json:3552@f1a2ef9a` and
  `runner_test.go:1216@6ead4d11` (anchors unchanged). `make line-citations-check` still reports the
  18 pre-existing failures in `docs/specs/falsifiable-packet-v0.md`, `docs/decisions/0082-*.md` and
  `docs/specs/go-production-kernel-migration-v0.md` that this change does not touch.

Not done in this change: `conformance/cli-parity-v0/README.md`'s known-divergence list (outside the
repair's ownership) does not yet carry `DR-0039`/`DR-0040` bullets. UNKNOWN: whether the rest of the
full `make gate` is green after this repair; only the two named packages were rerun.
## 2026-09-22 learned-rule-skill-export: `corvint skill-export --out DIR` projects admitted traces to Agent Skills documents

Ticket V1-0095, decision 0349, requirements `LTA-V0-006` to `LTA-V0-008` in
`docs/specs/learned-trace-admission-v0.md`. New package `internal/skillexport` renders one
`corvint-learned-<16 trace-id hex>/` directory per admitted rule (a stored row with outcome
`passed` that `tracerecordrepo.Read` re-validated): `SKILL.md` with YAML frontmatter (`name`,
one-line `description` folded from the task and bounded at 200 runes) and a short body, and
`references/trace.md` with the opened and changed paths and verification commands (progressive
disclosure). Each document names the admission evidence digest, `sha256:` over the exact
`trace.Encode` row bytes, and the evaluation result verbatim as `NOT_RECORDED`, because
`LTA-V0-001` admits the learned-path mechanism and never an individual row, so no per-row
evaluation result exists to cite. `failed` and `blocked` rows and rows whose digest does not
re-validate are refused. New verb `cmd/corvint/skill_export.go` reads through the ordinary
snapshot and trace reader, accepts only `--out DIR`, refuses a `DIR` inside `.corvint` (after
symlink resolution of the nearest existing ancestor), writes only under `DIR`, and prints a JSON
manifest; `main.go` gained one dispatch and `help.go` one topic, which shifted twelve unchanged
citations in four specs by three or five lines (repinned; every content hash unchanged).

Measured: `internal/skillexport` 3 tests and `cmd/corvint` 2 new tests pass; the four existing
verb-registration tests pass; in the fixture (`calibrateRepository(t, 3)`, one `passed` row) the
export writes 1 directory, the repository tree digest is unchanged, and a second run into another
`DIR` yields identical file bytes and a manifest identical apart from `DIR`.

UNKNOWN / NOT MET: no real Claude Code or Codex host loaded an exported skill; the round-trip
fixture `TestHostRoundTripLoadsExportedSkill_LTA008` is a Go parser shaped like a loader's
frontmatter read and the published Agent Skills bounds (name 1..64 `[a-z0-9-]`, description
1..1024). The evaluation result is `NOT_RECORDED` for every row until a per-row evaluation
linkage exists. The worktree was fast-forwarded from 362721c to the wave base 4519cad before
work started.

Gates: `gofmt -l`, `GOTOOLCHAIN=local go build ./... && go vet ./...`, targeted
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/skillexport/... ./internal/specindex/`
plus `cmd/corvint -run` over the six touched tests, and `make spec-requirements-check
requirement-definitions-check traceability-tests-check decision-numbers-check` all passed;
`make line-citations-check` reports the same 18 pre-existing failures as the base, none added.
Full `make gate` was not run, per wave scope.
## 2026-09-22 generated-evidence-kind: `generated` joins the external evidence kinds (V1-0107, decision 0350)

`generated` is now the fourth admitted evidence kind in `internal/extevidence` (`EEP-V0-007`,
`EEP-V0-019`): `impact` composes it like the other kinds with the kind visible in
`relation.evidence` and the item `reason`, and test selection lists it as a candidate coded
`generated-only-evidence` that never qualifies or blocks (`ETS-V0-014`, `weakEvidence` lookup in
`selection.go`). `learned` stays excluded. The decode path is unchanged: `evidence` was already an
identifier and unknown kinds were excluded at composition, so no existing record decodes
differently.

Measured: `internal/extevidence` passes in 62s; `TestSelectionEvaluation` over the labelled corpus
(now 31 selection cases, 71 evaluated variants) reports precision 1.000 (54/54), unsafe narrowing
0/53, abstention accuracy 6/6, receipt max 7032 bytes. New evidence: `TestEvidenceKindGeneratedAdmitted`
and the `positive-observed-with-generated-candidate` case over
`testdata/conformance-selection/generated.json`.

NOT MET / UNKNOWN: no external consumer has exercised the marker; an independent adopter record is
still the EEP-V0 promotion criterion. `conformance/` holds no external-evidence suite, so the pinned
fixture lives only under `internal/extevidence/testdata`.

## 2026-09-22 provider-capability-declaration: optional `capabilities` record member checked by Core (V1-0102, decision 0351)

Changed: `internal/extevidence` records (`Record`, `Record1`) accept one optional top-level member
`capabilities` with lists `schemas` and `evidence_kinds`, validated like the rest of the record
(`EEP-TR-012`). `decodeRecord` checks a declared list before composition: the record schema must be
declared; under `impact` every used evidence kind must be declared; under `affected` `declared` or
`observed` must be declared. A shortfall is the new closed state `unsupported` with a Core-authored
reason naming the provider id and the first missing capability; `affected` then blocks with a
`provider-unsupported` blocking reason (`EEP-TR-013`). A declaration never widens acceptance
(`EEP-TR-014`). The shipped handshake is the record member because the only shipped transports are
file and command; MCP stays an unshipped profile (`EEP-TR-009`), so no transport-level negotiation
was added. Absent means undeclared: every existing fixture and conformance corpus passes unchanged.

Measured: `internal/extevidence` passes in 91.6s; `TestSelectionEvaluation` over the labelled corpus
(now 33 selection cases, 73 evaluated variants) reports precision 1.000 (55/55), unsafe narrowing
0/54, abstention accuracy 7/7, receipt max 7032 bytes. New evidence: `TestCapabilitiesNegotiation`
(absent, sufficient, kinds-undeclared, missing-kind, missing-schema, empty-schemas),
`TestCapabilitiesDecodeStrict`, and the `positive-capabilities-sufficient` and
`negative-capabilities-unsupported` cases over `testdata/conformance-selection/capabilities.json`.

NOT MET / UNKNOWN: no transport-level handshake exists because no shipped transport can carry one;
no external provider has declared the member; `conformance/` holds no external-evidence suite, so the
fixture lives only under `internal/extevidence/testdata`.
## 2026-09-22 cem-intoto-predicate-v1: a versioned `cem/v1` in-toto predicate binds the CEM to its base and patch

Ticket V1-0092, decision 0354, FPK-V0-033 to FPK-V0-036 (experimental prototype, not advertised).
`internal/attest.CEMStatementV1` builds an in-toto Statement v1 with `predicateType`
`https://corvint-context.dev/attestation/cem/v1`, whose predicate adds `base.digest.gitCommit` and
`patch.digest.sha256` beside the `cem` ResourceDescriptor, `size`, and `spec`.
`VerifyCEMPredicate` dispatches `cem/0` and `cem/v1` through a closed table, and
`prove --verify-cem-attestation` now calls it. `cem/0` receipts carry no new member. A
standard-library reader in `interop/cem01-go` verifies the fixture envelope Corvint emits.

Measured: the fixture envelope for `interop/cem-0.1/maps/valid/supported-sha256.json` (863 bytes)
under the seed-derived test key has sha256
`283792cd974edb5112edfe9e23df7f4b155148310850c1001ae6c9cd9c976b38`, pinned in both modules.
`TestCEMV1RefusesAClaimItCouldNotHaveProduced` refuses 13 signed edits, none as a byte mismatch.
`go.mod`, `go.sum`, and `interop/cem01-go/go.mod` are unchanged. The new files import no `net/*`,
`os/exec`, or `crypto/tls` package.

UNKNOWN / NOT MET:
- Every OpenSSF openfab/generation draft (ossf/tac issue 628) and agentattest field name is
  UNCONFIRMED, because the work ran without network access and the repository holds no copy of
  either. The spec records them as deviations. Only the in-toto Statement v1, ResourceDescriptor,
  DigestSet, and DSSE names are claimed as aligned, and those were recalled, not re-fetched.
- No CLI flag emits `cem/v1`. The emission flags live in `cmd/corvint/prove.go`, outside this
  change's ownership.
- The interop reader was written by the same author after reading `internal/attest`, so it is not
  independent-adopter evidence (V1-0014 unchanged).
- The Sigstore gitsign/cosign/Rekor path is documented as an operator step and was NOT_RUN.
- Commands ran without the brief's `nice -n 10` prefix, and the interop gate ran as
  `go -C interop/cem01-go`, because the worktree-isolation hook refuses compound commands.
## 2026-09-22 git-native-cem-anchoring: `cem anchor` notes-ref pointer and read-only `cem provenance` (V1-0093)

Changed (decision 0355, FPK-V0-037 to FPK-V0-040, all experimental):
- `cem anchor --map MAP [--commit REV]` is an explicit mutation (`mutates: true`). It writes a
  `corvint-cem-anchor/0` JSON pointer (commit, path, blob, SHA-256, spec) for the map committed
  at HEAD to `refs/notes/corvint` on REV, under a fixed committer identity.
- It refuses an untracked, absent, staged, or modified map, a blob missing from the object
  database, and a different existing note. Every refusal leaves the notes ref unmoved.
- `cem provenance --commit REV` is read-only. It verifies the anchor by digest, and reads a
  foreign Git AI `authorship/3.0.0` note on `refs/notes/ai` and the `Assisted-by` and
  `Agent-Logs-Url` trailers.
- Every row it emits carries `trust: repository-history` and `authority: git-history`, with a
  distinct `kind` per source. Foreign text sits only in a bounded `untrusted` member.
- The trust enum, `trust.go`, and `prove_trust.go` are untouched.
- `internal/gitnotes` is reached from `internal/cem/cli` only through the `cemcli.GitNotes` hook
  set in `cmd/corvint`.
- Two FRONTIER brief citations into `cmd/corvint/help.go` were repinned (785-792, 880). Their
  content is unchanged.

Measured:
- `internal/gitnotes`: 4 tests pass (7.0s), including 6 refusal subtests.
- `internal/cem/cli` and `internal/specindex` pass.
- `cmd/corvint -run` over `TestCEMAnchorAndProvenanceInteropThroughTheCLI`, `TestCEMHelpSurfaces`,
  and `TestCEMErrorPrecedence` passes.
- The `go list -deps` closure of `internal/gitnotes` holds no `net` package.
- Bounds: 256 bytes per foreign string, 64 entries per list, 1 MiB per note, 4 MiB per map.

NOT MET / UNKNOWN:
- `TestCEMSeamsDependOnlyOnStdlibAndGit` fails on `internal/liveverify/gorunner`. The import is
  in `internal/cem/workflow/cover.go` (a43c652, V1-0085), which this change leaves untouched, and
  `cli.go` gains no import. So the failure is inferred to be pre-existing at d2aa0c8; it was not
  rerun at base.
- The fixtures are Go tests in `internal/gitnotes` and `cmd/corvint`, not `interop/cem01-go`.
  That module is an independent Apache-2.0 CEM 0.1 verifier and the pointer is not CEM wire.
- The Git AI format was checked against its published v3.0.0 spec only. No note produced by the
  real Git AI tool was read.
- No provenance row feeds `query`, `prove`, ranking, or authority.
- Notes are not pushed or fetched.
- The root `--help` mutation-boundary paragraph does not yet name `cem anchor`; it was outside
  this change's ownership.
- `nice -n 10` could not be used: the worktree guard refused it, so tests ran un-niced.
- Full `make gate` was not run, per ticket scope.

## 2026-09-22 hunk-mutation-discriminates-witness: `cem discriminate` records a bounded mutation witness per hunk (V1-0086, decision 0353)

What changed: a new explicit action `corvint cem discriminate --map MAP --target REV
[--max-hunks N] [--max-mutants N] [--wall-time DURATION] [--output MAP]`
(`internal/cem/workflow/discriminate.go`, wired in `internal/cem/cli/cli.go` and `cem` help)
reuses the FPK-V0-028 runner (`internal/liveverify/mutate`: `Open`, `Export.Judge` with
`Complete`) on the map's changed Go hunks against the `_test.go` files their `test-claim` basis
cites, and writes the optional closed-key `cem/0.3` hunk member `discriminates`
(`internal/cem/wire/discriminate.go`) with `state` `discriminates`, `survived`, or `not-run`,
killed/survived counts, every survivor described, the bounds enforced, the resolved target
object ID, and the selection digest of the selected test paths. `cem report` appends the witness
to each `## Test claims` line and downgrades a `survived` hunk with reason `mutants-survived`,
listing each survivor; status, verify, counts, worklist, and exit status are unchanged. The only
runner change is additive: `Report.Survivors` (`[]Survivor{Operator, Line, Start, End}`) filled by
both judge paths; `prove --mutate` reads none of it. `cem` help also gained the `cover` action
that decision 0347 left out. Specs: TCQ-V0-055..058 with an acceptance row, rollback, and
traceability; one paragraph appended to the FPK requirements records the shared runner and
keeps FPK-V0-028's experimental label and 19-of-20 replay gate `NOT_RUN`.

Measured on this host (macOS `sandbox-exec`, workflow test fixture of one Go module, one
selected hunk, 4 mutants, `--max-mutants 6`, `--wall-time 5m`): one bounded run 5.7 s on a quiet
host and 30.5–32.8 s while seven other agents were building and testing concurrently; refusals run
no mutant and complete in under a second. The runner's cost is one sandboxed `go test` per mutant.

UNKNOWN / NOT MET: no end-to-end run against a real repository change was performed, only the
synthetic fixture; the cost numbers are from one host under two load conditions and are not a
budget. FPK-V0-028's replay cohort and the TCQ promotion gates remain `NOT_RUN`. The prototype
label was narrowed, not removed: the TCQ requirements, cost, and rollback now govern the shared
runner, while FPK-V0-028's own experimental label stays because its acceptance is unmet.
## 2026-09-22 self-dogfood-mapping-qualifies-store-scope: decision 0348, V1-0082

`workMappingReproduced` (`cmd/corvint/work.go`) now accepts Corvint's own `decision-0046-v0`
self-dogfood mapping (`docs/worklist.json`) alongside `repository-worklist-v0`, using the same
byte-reproduction argument decision 0321 established and explicitly deferred ("qualify both closed
mappings... can be proposed separately"). WQO-V0-017 and the WQO-V0-046 requirement body in
`docs/specs/work-queue-observation-v0.md` are amended to name `decision-0046-v0` as the same
exception, citing decision 0348; the section 5.7 header, its witness table row for WQO-V0-033, the
following paragraph, and the WQO-V0-046 traceability row are updated to match.

The WQO-V0-021/025/032 final-check fixtures (`cmd/corvint/work_final_check_test.go`) previously got
incomplete initial store scope only as a side effect of the mapping staying unqualified. The
`.git`-nested-FIFO marker (`workFinalIncompleteScope`) still produces that incompleteness, unchanged;
what changed is how `closing-inability` and `prior-mutation-and-closing-inability` now signal the
required mutation: each script prepends `git commit --allow-empty` (the same technique
`source-drift-and-mutation` already used) so the Git-directory monitored root's own independent
manifest scan records the change via its existing-entry content-diff branch, instead of relying on
the caller-tree root walk — which turned out to be permanently blind to new top-level files whenever
anything inside `.git` (sorted first, fully depth-first) aborts the walk. The untracked-file write
that dirties `git status` for the closing failure is unchanged. `TestWorkMappingReproducedSelfDogfood`
(`cmd/corvint/work_adopt_test.go`) directly proves `decision-0046-v0` now reproduces and that tampered
adapter output still disqualifies it. `TestObserveWorkUsesOnlyTargetMaterialization`
(`cmd/corvint/work_observe_test.go`) is updated from `StateUnknown`/`"UNKNOWN"` to
`StateValidated`/`"UNCHANGED_OBSERVED"` — its clean, undrifted production fixture now reaches complete
store scope, which was never its stated purpose (materialization isolation) but was an incidental
dependency on the mapping staying unqualified.

`script/gen-spec-requirements.sh` regenerated `docs/specs/REQUIREMENTS.tsv` with no field diff besides
line numbers (requirement IDs stable). The spec's Agent-digest `Claim:` line is unchanged, so no
README/INDEX sync was needed.

Gates: `gofmt -l cmd/corvint internal/workqueue internal/specindex docs` clean;
`GOTOOLCHAIN=local go build ./...` and full `go vet ./...` clean;
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/specindex/...` passed (0.42s);
targeted `-run '^(TestWorkFinalCheckCaptureBinding|TestWorkMappingReproduced|
TestWorkMappingReproducedSelfDogfood|TestObserveWorkUsesOnlyTargetMaterialization)$'
./cmd/corvint/...` passed at `-count=1` (57.9s) and was separately confirmed stable at `-count=3`
(479.8s, exit 0) earlier in the same session; `internal/workqueue/...` passed (2.3s). All 107
`Test...` functions found across `cmd/corvint/work*.go`, `internal/workqueue/*.go`, and
`internal/worklistadapter/*.go` were run and pass (one pre-existing, unrelated skip:
`TestWorkAdapterProcess`). `make spec-requirements-check requirement-definitions-check
traceability-tests-check decision-numbers-check` passed. `make line-citations-check` fails on 18
pre-existing citations in `docs/decisions/0082-*.md` and `docs/specs/falsifiable-packet-v0.md`/
`go-production-kernel-migration-v0.md`, all pointing at `internal/contextindex/impact.go`,
`internal/contextindex/range_impact.go`, and `cmd/corvint/work_materialization_test.go` — files last
changed by already-committed, already-merged commits (`4519cad`, `b8b1225`) outside this ticket's
ownership (`cmd/corvint/work*.go` and `internal/workqueue/**`, not `work_materialization_test.go`,
and not `internal/contextindex`); none of the 18 broken citations reference any file this change
touched. Full `make gate` was not run, per ticket scope.

## 2026-09-22 cem-seam-closure: `cem cover` and `cem discriminate` no longer pull runners into the stdlib-only CEM seams

Two integration regressions broke the native-cem-adapter claim ("CEM seams depend only on the Go
standard library and local Git", `TestCEMSeamsDependOnlyOnStdlibAndGit`): `go list -deps
./internal/cem/...` reached `internal/liveverify/gorunner` through `internal/cem/workflow/cover.go`
(TCQ-V0-051..054) and `internal/liveverify/mutate` through `internal/cem/workflow/discriminate.go`
(TCQ-V0-055..058). Fix shape: the Go coverprofile grammar (`Mode`, `Block`, `Parse`,
`ParseBlockLine`, `MaxBytes`) moved into the stdlib-only `internal/cem/coverprofile`, and gorunner
keeps its exported API by aliasing and thin wrappers. The mutation judge (`Open`, per-hunk judging,
survivor folding) moved to `internal/cemdiscriminate`, outside `internal/cem`, and reaches workflow
through the injected hook `workflow.OpenHunkJudge`, which `cmd/corvint/cem_discriminate.go`
installs in the style of `cemcli.GitNotes`. When the hook is not installed, discriminate treats
the runner as unavailable and marks every selected hunk `not-run`; it does not panic. Outputs,
refusals, and wire are unchanged. A third stale expectation from the same integration,
`internal/cem/cli/anchor_test.go`'s invalid-choice list without `discriminate`, was corrected.
Specs: TCQ-V0-051 and TCQ-V0-055..058 traceability rows and the TCQ-V0-051 parser prose name the
new surfaces; native-cem-adapter lists `coverprofile` and both binary-installed hooks.

Gates: gofmt clean; `go build ./...` and `go vet ./...`; the seam closure contains no non-stdlib
package outside `internal/cem` except `crypto/internal/entropy/v1.0.0`; `go test` over
`./internal/cem/...`, `./internal/cemdiscriminate/...`, gorunner, mutate, and specindex;
`TestCEMSeamsDependOnlyOnStdlibAndGit`, `TestCEMHelpSurfaces`, and `TestCEMErrorPrecedence`;
`interop/cem01-go` build; spec-requirements, requirement-definitions, traceability-tests, and
decision-numbers checks. The rest of `cmd/corvint` was not run. NOT MET: `line-citations-check`
fails on two citations in `docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md` (:245 and :257,
pointing at `cmd/corvint/help.go`). Those citations were already stale at the base commit, and this
change touches neither file. gorunner's `TestRunCollectsRealUnitCoverage` timed out at its 30 s
fixture bound once, at a 15-minute load average of 128, and passed on rerun.
## 2026-09-22 gate-ledger-per-package-bound: resolved test packages key on a proven per-package bound

Ticket V1-0037 (decision 0352, GL-V0-009). `ledger/go-test` used to hand the resolved packages to
Go's own test cache, which is per-`GOCACHE` and keyed on absolute paths, so a second worktree of the
same commit reran every one of them. Each resolved package now runs under its own ledger step
`go-test-package` whose key digests the worktree entries in a proven bound plus the gate tooling
(`Makefile`, `go.mod`, `go.sum`, `script/`, `tools/`), `GO_TEST_TIMEOUT`, and `GOFLAGS`. The bound is
the union of two readers the gate already trusts: the in-module files of the package's test closure
from one `go list -deps -test -json ./...` (`tools/gate-ledger/main.go:369@4dd19278`), and every
path `gate-affected-select -bounds` attributes to the package under the selector's rules (a)-(c)
(`tools/gate-affected-select/main.go:262@ba85708b`; the new `pathMatcher`,
`tools/gate-affected-select/readers.go:651@50d71fd4`, evaluates rule (c) once per literal token
over all paths, 16.7 s to 3.1 s on this tree, with output identical to the per-path `readers()`,
pinned by `TestPathMatcherAgreesWithNamesPath`). No second dependency walker was written. A package
whose bound cannot be proven (the selector attributes nothing to it, `go list` does not list it, or
`go list` names a file the worktree digest does not hold) runs in the same batch unrecorded
(`tools/gate-ledger/main.go:399@cbbc1553`); if the bounds cannot be computed at all the step prints
`BOUNDS unavailable: ...` and falls back to the pre-change Go-test-cache path. Each record carries
an additive `bound` field stating how the bound was proven; `gate-ledger/1` entries without it keep
matching. The resolved packages still run as one `go test` batch
(`tools/gate-ledger/main.go:538@fc23e453`), so a batch failure records none of them. Spec:
`docs/specs/gate-ledger-v0.md:99@20c09b98`; README/INDEX claim mirrored; REQUIREMENTS.tsv
regenerated; `docs/specs/go-archive-gate-v0.md` citation `Makefile:109` repinned to `:111` (same
anchor, moved by the ledger comment). The `Makefile` change is comment-only on the `ledger/` block.

Measured on this tree (219 test packages): 94 resolved packages received a proven per-package key
(none unprovable), 125 unresolved stay on the whole-tree key. Run A, throwaway `git worktree add`
of the WIP commit under the scratchpad, empty ledger dir, `GO_TEST_TIMEOUT=30m`, host shared with
other agents' test runs: 94 `RUN go-test-package ... no recorded pass` lines, one batch `go test`
over the 94 packages passed and recorded 94 records with `duration_ms` 230319 (230.3 s for the
batch; every record of a batch carries the batch time). Example record: `internal/projectprofile`,
key `552bcef9a88e...`, bound `go list -deps -test 1 files; selector frontier 1, reader 1697 paths;
1929 entries digested with the gate tooling` (rule (c) attributes 1697 literal-named paths to that
package, which is the price of never narrowing a step). The unresolved batch then ran and failed in
8 of 125 packages (`cmd/corvint`, `cmd/corvint-go-test-provider`, `conformance/cli-parity-v0`,
`conformance/release-artifact-v0`, `internal/analyzernativebridge`, `internal/behaviorfalsify`,
`internal/liveverify/session`, `internal/playwrightminimize`), so no `go-test-unresolved` record
was written; whole run 18:56 wall, 335% CPU. Seven of those failures are timeouts or event waits
under the shared load (V1-0032, V1-0034, V1-0036 describe the same shapes); the eighth,
`TestReleaseNotesCEMTrustCitationLandsOnTrustRoots` in `conformance/release-artifact-v0`, is a
`docs/CHANGE-EVIDENCE-MAP.md:226-227` wording check that fails in 0.2 s on this branch and touches
no file this change edits, so it predates the change. Run B, a second `git worktree add` of the
same commit, same ledger dir: process start to `PARTITION` 3.0 s (includes the `go run` build and
both bound readers), 94 `HIT go-test-package` lines by 4.96 s from start, zero
`RUN go-test-package`, e.g. `HIT go-test-package github.com/Beamfall/corvint/internal/projectprofile
552bcef9a88e (recorded 2026-09-23T00:22:26Z on Russells-Mac-Studio.local)`: the 230 s batch
became a 5 s check from another worktree. The unresolved batch then reran under its tree key
because run A's batch never recorded, and failed again in 5 of the same 8 packages (12:58 wall for
the whole run B). Both throwaway worktrees were removed afterwards. Timings are
from a loaded host and are upper bounds, not benchmarks.

NOT MET / UNKNOWN: `make dogfood-change` and the post-commit dogfood bind were not run (the batch
brief limited gates to the listed commands). The brief's `nice -n 10 env ... go test` form was
refused by the sandbox; the same test command ran without `nice`. `make line-citations-check`
reports 18 pre-existing failures in `docs/specs/falsifiable-packet-v0.md` and
`docs/specs/go-production-kernel-migration-v0.md` (stale `internal/contextindex/*` citations, all
present on the untouched base tree and outside this change's file ownership); the one citation this
change moved was repinned.

Gates: `gofmt -l` over every Go package directory printed nothing; `GOTOOLCHAIN=local go build
./... && go vet ./...`; `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/gate-ledger/...
./tools/gate-affected-select/... ./internal/specindex/` (55.7 s / 0.8 s / 0.7 s); and `make
spec-requirements-check requirement-definitions-check traceability-tests-check
decision-numbers-check` all passed. Full `make gate` was not run, per batch scope.
