# Build log

Append-only record of material design decisions, independent findings, failed evaluations, and
promotion evidence, newest entry first. Each entry carries a date heading and the requirement or
decision IDs it concerns, so `rg -n '^## ' docs/BUILD-LOG.md` is the index.

The public tree starts this log at the 0.4.0a4 alpha. Entries written before publication are internal
working records and are referenced from decisions and specifications as historical context only.

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
