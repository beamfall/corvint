# Build log

Append-only record of material design decisions, independent findings, failed evaluations, and
promotion evidence, newest entry first. Each entry carries a date heading and the requirement or
decision IDs it concerns, so `rg -n '^## ' docs/BUILD-LOG.md` is the index.

The public tree starts this log at the 0.4.0a4 alpha. Entries written before publication are internal
working records and are referenced from decisions and specifications as historical context only.

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
