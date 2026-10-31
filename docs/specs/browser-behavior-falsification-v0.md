# Browser behavior falsification V0

- Owner: Russell Lewis
- Date: 2026-09-21
- Intent status: proposed; BBF-V0-015..018, BBF-V0-019..028, BBF-V0-029..033 and BBF-V0-214 accepted under the owner's standing auto-accept (2026-10-11) (V1-1366, V1-1367, V1-2011, V1-2027, V1-2029)
- Delivery status: experimental
- Authoritative inputs: owner requests [issue 54](https://github.com/beamfall/corvint/issues/54) and [issue 167](https://github.com/beamfall/corvint/issues/167),
`AGENTS.md`, `docs/specs/documentation-corpus-v1.md`, and
`docs/specs/js-live-test-provider-v0.md`.

## Agent digest
- Claim: An opt-in companion runs digest-approved caller hooks against a marked disposable workspace and emits fail-closed criterion-level falsification receipts.
- Status: proposed; BBF-V0-015..018, BBF-V0-019..028, BBF-V0-029..033 and BBF-V0-214 accepted under the owner's standing auto-accept (2026-10-11) (V1-1366, V1-1367, V1-2011, V1-2027, V1-2029)/experimental; no control result authenticates semantic adequacy or authorizes test-suite narrowing.
- Exists: the separate experimental planner, approval gate, bounded runner, closed classifier, synthetic conformance fixtures, and opt-in approval-bound evidence export with offline verification; optional control application markers classify inert controls as NOT-APPLIED, list them separately and flag repeats across revisions (BBF-V0-015..017); an attempt without an admitted hook receipt keeps its infrastructure or invalid classification with its stdout bound and still verifies (BBF-V0-018); an opt-in declared shared-server isolation profile resets and checks a caller-declared loopback server identity around every control (BBF-V0-019..023), and an opt-in runner-neutral profile reruns selected Go test examples under a pinned caller perturbation hook (BBF-V0-024..028), with an RSpec binding whose negative controls ran live through a docker-prefixed RSpec (BBF-V0-029..033).
- Blocked on: live adopter/browser evidence for each supported control kind and owner acceptance of the proposed contract.
- Read next: Requirements; Trust and mutation boundary; Acceptance matrix.

## Affected user and measurable job

An adopter with a revision-pinned browser behavior contract needs to learn whether one exact
criterion fails for the expected assertion identity under a deliberate perturbation, while unrelated
required behavior and cleanup remain valid. The output must distinguish a killed control from a
survivor, unsupported work, infrastructure loss and an invalid experiment without turning partial
coverage into a general adequacy claim.

The verified starting point is the experimental `corvint-corpus-behavior-provider/1` consumer in
`internal/doccorpus/behavior.go`, which validates declared negative-control events but does not
produce them. `internal/jstestprovider` retains Playwright test/project/browser/attempt and artifact
evidence. `internal/playwrightminimize` demonstrates digest-approved experimental planning and
bounded caller commands, but it diagnoses suite interactions rather than criterion falsification.

## Requirements

- `BBF-V0-001`: Planning and execution are separate opt-in operations. A plan is deterministic,
  closed-schema JSON with a content digest; execution requires explicit approval of that exact
  digest and refuses drift, unknown fields, trailing JSON and missing experimental admission.
- `BBF-V0-002`: Every plan and result binds the exact contract ID/digest, criterion and assertion
  identities, application/test/documentation Git revisions, test ID/path/line/title, Playwright
  project, runner/browser names and versions, configuration digest, perturbation definition and
  digest, attempt/retry identity, cleanup observation and retained artifact digests.
- `BBF-V0-003`: The closed control vocabulary is `wrong-locator`, `wrong-expected-value`,
  `omitted-assertion`, `omitted-event`, `reordered-event`, `wrong-project`,
  `suppressed-persistence`, `opposite-branch` and `changed-fixture-value`. A caller may select any
  supported subset; an absent hook or inapplicable control reports `not_supported` or `not_run`,
  never `killed`.
- `BBF-V0-004`: A control hook is caller-owned, content-digest pinned, argument-free and run only
  after plan approval under bounded process-group containment in a caller-marked disposable
  workspace. The runner stages the executable, supplies the perturbation as bounded JSON, permits
  only explicitly declared environment keys, rejects escaping artifact paths or symlinks, and
  verifies the workspace returns to its pre-control digest after the separately pinned cleanup
  hook. It never edits repository source or implicitly targets shared or persistent external state.
- `BBF-V0-005`: `killed` requires the target criterion to fail on its expected assertion identity
  during the first and only admitted Playwright attempt, every declared unrelated criterion and
  required setup observation to retain its expected outcome, no infrastructure failure, successful
  cleanup and complete identity/artifact binding. A crash, timeout, selector error, wrong assertion,
  unrelated failure, retry-only kill or cleanup failure cannot count.
- `BBF-V0-006`: Each control result is exactly one of `killed`, `survived`, `not_supported`,
  `not_run`, `infrastructure_failed` or `invalid_control`, with stable reason codes. Unknown or
  contradictory evidence fails closed to `invalid_control`; execution-boundary loss is
  `infrastructure_failed`.
- `BBF-V0-007`: Results preserve every requested ordinal and actual attempt without replacement.
  Identical approved inputs are independently reproducible; a later retry or rerun never hides the
  first outcome, and duplicate or missing ordinals invalidate the control set.
- `BBF-V0-008`: Every surviving or invalid control is an explicit criterion-level coverage gap.
  Any gap, unsupported/not-run control, stale binding or incomplete selected vocabulary preserves
  `full-relevant-suite` fallback.
- `BBF-V0-009`: The aggregate reports raw counts for all six statuses and mutation score as
  `killed/(killed+survived)` only when that denominator is nonzero. It reports supported, requested
  and executed denominators separately and never labels partial support universally adequate.
- `BBF-V0-010`: Plans are bounded by at most 64 controls, 16 attempts per control, a 24-hour total
  declared budget that must also exceed the executor's 10 s cleanup reserve, 32 MiB input/output and
  64 MiB per retained artifact, and at most 1000 entries each in a control's unrelated-criteria and
  required-setup lists, so the per-receipt output floor can never exceed the 32 MiB document bound.
  Cancellation, timeout and output overflow terminate owned processes; missing descendant-cleanup
  evidence is not success.
- `BBF-V0-011`: Synthetic conformance covers a tautological assertion, hidden duplicate element,
  wrong-value survival, expected kill, unrelated failure, timeout, cleanup failure, retry-hidden
  outcome and stale contract/revision/perturbation/artifact digests.
- `BBF-V0-012`: Receipts are observations, not authenticated caller honesty, browser-behavior
  parity, complete mutation coverage or narrowing authority. Generated control definitions remain
  proposals until the caller approves their exact plan and perturbation boundary.

- `BBF-V0-013`: The opt-in `execute-receipt` operation emits `corvint-browser-behavior-evidence/1` with the exact canonical plan digest preimage, an explicit approval observation bound to that digest, executing tool version/source revision/dirty state/executable SHA-256, and every actual control/attempt. It retains exact hook stdout and the native report declared as a digest-matching hook artifact before cleanup, with independently recomputable SHA-256 digests. Missing reports yield `invalid_control`; credential-shaped data refuses export with a value-free diagnostic. The complete encoded envelope, including base64, is bounded by BBF-V0-010; retained raw bytes additionally share a 16 MiB budget. Export rejects a base64 lower bound or complete encoding that exceeds the document limit before secret screening. If both limits and credential-shaped data fail, the value-free size diagnostic takes precedence; no rejected bytes are returned. Admitted envelopes still screen every raw field and the complete encoded envelope. `verify-receipt` requires caller-pinned plan and executable digests, verifies canonical bytes and approval/raw/report digests, exact inventories, source/runner/assertion identity, cleanup/workspace observations and recomputed outcomes/counts, and never reads a workspace or launches a command. Missing, tampered, stale, unsafe or unsupported evidence cannot verify. The existing `execute` report/0 is unchanged.

- `BBF-V0-014`: (accepted, decision 0507) In a sanitized acceptance
  worker, control staging and hooks preserve the validated per-run TMPDIR. This transport
  value is not a caller-declared environment key and cannot admit credentials or injection.
  Standalone non-acceptance execution retains its existing environment policy. Staged commands
  use `corvint-behavior-control-*`; workspace/native artifacts, imported runtimes, browser caches
  and hard-coded output paths remain caller-reserved transitive uncertainties. Joined execution
  removes staging; the acceptance parent also removes residues after worker interruption.

- `BBF-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1366) A
  control may declare an optional `application_marker` `{kind, type}`, where `kind` is
  `annotation` (a test annotation type) or `provider-event` (a provider event name) and `type` is
  an identifier; an invalid marker refuses the plan. The marker is part of the plan digest, not the
  perturbation digest. A hook receipt may report at most 64 `markers` observations
  `{kind, type, control_id}`; a malformed or oversized list is `invalid_control` with
  `marker-observation-invalid`. For a marked control, an attempt is applied only when an observation
  matches its kind, type and own control ID; repeats count once and another control's marker never
  applies. An admitted attempt is then classified `APPLIED-FAILED` (killed), `APPLIED-PASSED`
  (survived), `APPLIED-EARLY-FAIL` (applied, but a required setup observation failed first, which
  stays `invalid_control`) or `NOT-APPLIED` (a kill or survivor without the marker). `NOT-APPLIED`
  is `invalid_control` with `control-not-applied`, so it is never counted as a kill or a survivor
  and stays outside the mutation-score denominator. Other invalid or infrastructure outcomes carry
  no application value. A control's application aggregates its attempts: killed or survived
  carries its applied value; otherwise any `NOT-APPLIED` attempt makes the control `NOT-APPLIED`.
  The hook reports the markers; they are observations under BBF-V0-012, not independently
  authenticated native evidence.
- `BBF-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1366) The
  report lists `NOT-APPLIED` control IDs separately in `inert_controls`; they also remain coverage
  gaps. Compatibility rule for this profile: `application_marker`, `markers`, `application` and
  `inert_controls` are optional and omitted when empty, so a plan, report or evidence receipt with
  no marker is byte-identical to its prior form and still verifies; the six statuses are unchanged.
  `verify-receipt` recomputes every attempt and control application and the inert list. A control
  without a marker keeps its prior status, and its application is unobserved, never claimed as
  applied. The new-e2e acceptance consumer records verified inert controls as `not_applied` and
  unmarked ones as `application_unobserved`; an inert control adds reason
  `negative-control-not-applied` and its strength axis abstains as `NOT_MEASURED` with that reason,
  while an unmarked control leaves the assessment unchanged.
- `BBF-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1366) An
  inert-control history reads only caller-supplied retained evidence receipts, each with its
  caller-pinned plan and executable digests, at most 64 per call; no store is read, created or
  written. Every receipt must pass `verify-receipt`, otherwise the history is refused with
  `history-receipt-invalid`; an empty or oversized history is refused with
  `history-bound-exceeded`. The same control is the exact contract, criterion, assertion, test,
  Playwright project, control ID, control kind and declared marker; hook and perturbation digests
  are excluded. A revision is the plan's application and test revision pair. A control that is
  `NOT-APPLIED` at two or more distinct revisions is a control defect with reason
  `repeated-not-applied` and its sorted revisions; repeats at one revision are not a defect. The
  result is deterministic and independent of receipt order.

- `BBF-V0-018`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1367) An
  attempt whose hook stdout yields no admitted hook receipt (the hook timed out, crashed,
  overflowed or otherwise failed its process boundary, or exited cleanly with undecodable output)
  keeps its executor classification, `infrastructure_failed` or `invalid_control`, in
  `execute-receipt`. Its exact stdout is retained and bound to `stdout_sha256` without a native
  report, so no `native-report-unavailable` omission applies; an attempt whose hook never started
  observes no stdout and retains no raw entry. The BBF-V0-013 missing-report rule applies only to
  attempts with an admitted hook receipt. `verify-receipt` accepts an attempt without a receipt
  only when its retained bytes match the raw and attempt digests, it carries no native report or
  retained artifacts, its workspace baseline is the plan's, its retained bytes do not decode as a
  hook receipt whatever its recorded process outcome, and its status and reasons recompute from
  its process and cleanup observations. An attempt with no observed stdout verifies only when
  its hook never started, it carries no receipt, retained artifacts or application, and its
  status is `infrastructure_failed`, `invalid_control` or `not_run`. Neither kind ever verifies as
  `killed`, as `survived`, or with a status outside the vocabulary. A retained-byte-bound omission
  still refuses verification.

- `BBF-V0-019`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2011) A request
  may declare `isolation` with profile `shared-server`, a credential-free `http`/`https` loopback
  `server_url` (no query or fragment), an `application_identity` that is a `sha256:` digest or Git
  object ID, and a pinned `reset` command, and must then declare `external_state`
  `declared-shared-server`. Without `isolation`, `external_state` must stay `none` and the
  BBF-V0-004 disposable-workspace boundary is unchanged. The wall-clock budget must exceed the
  cleanup reserve plus the isolation reserve. The isolation is part of the approved plan digest.
- `BBF-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2011) Before
  every attempt of every run control, the runner invokes the staged reset hook with role
  `isolation-reset` and a bounded `corvint-browser-behavior-isolation-check/0` request (plan
  digest, phase, control, attempt, declared URL and identity). After the control's cleanup it
  invokes the same hook with phase `verify`. The hook prints one closed
  `corvint-browser-behavior-isolation-observation/0` document bound to the same plan digest,
  phase, control and attempt that reports the server identity it observed. The runner itself never
  contacts the server.
- `BBF-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2011) A failed or
  unbound reset yields `infrastructure_failed` with `isolation-reset-failed` and the control hook
  is not run. A reported identity different from the declared one before or after the control
  yields `infrastructure_failed` with `isolation-identity-drift`. A failed or unbound verify phase
  yields `infrastructure_failed` with `isolation-health-failed`. These checks precede every other
  classification, so an isolation failure is never `killed` or `survived`. An attempt that records
  an isolation observation under a plan without isolation is `infrastructure_failed` with
  `isolation-observation-unexpected`.
- `BBF-V0-022`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2011) The report
  records the plan's declared isolation, each attempt records its reset and verify process
  evidence and observed identities, and the limitations name the shared server as caller-declared
  and not independently sandboxed. `execute-receipt` retains them, and `verify-receipt` refuses a
  report whose isolation differs from the plan's. For each attempt with retained hook stdout it
  also retains the reset and verify phases' stdout (`isolation_reset_bytes`,
  `isolation_verify_bytes`), and `verify-receipt` requires each to match its recorded process
  stdout digest and to re-derive exactly the recorded identity under the attempt's plan digest,
  phase, control and attempt. It recomputes each admitted attempt's status from its recorded
  isolation observation, so an isolation observation cannot be dropped or its identity rewritten
  to forge a kill.
- `BBF-V0-023`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2011) A plan,
  report or receipt that uses neither profile is byte-identical to its form before BBF-V0-019. The
  new fields are omitted when absent.
- `BBF-V0-024`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2011) A request
  may declare `provider` with name `go-test`, a pinned Go `toolchain` command, one
  repository-relative `package` (or `.`; any `...`, embedded or trailing, is refused) equal to
  the target project,
  1..256 unique top-level `Test*` `examples` that include the target assertion, and 1..64
  `perturbation_env_keys`. Its runner identity is `go-test` with browser and browser version
  `none` and config file `go.mod`. Each control's unrelated criteria and required setup must be
  other selected examples, and controls may not declare application markers. Perturbation keys
  may not overlap the declared, reserved or provider-forced environment. The test repository must
  have no uncommitted or untracked changes.
- `BBF-V0-025`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2011) Under the
  provider profile, each control's pinned hook runs under the same approval, staging, containment,
  timeout and cleanup rules and prints one closed `corvint-behavior-control-perturbation/0`
  document bound to the plan digest, the control's perturbation digest and the attempt. Its
  environment may name only declared perturbation keys, with values of at most 4096 bytes and no
  NUL. Any other output is `invalid_control` with `perturbation-output-invalid`.
- `BBF-V0-026`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2011) The runner
  then runs the pinned toolchain's verified bytes (BBF-V0-214) as
  `go test -json -count=1 -run '^(examples)$' ./package`, under process containment, in a
  disposable clone of the test repository detached at the bound revision (copied objects, no
  hardlinks, no template hooks), so a test that writes files never modifies the protected test
  repository; a clone that cannot be staged is `provider-staging-unavailable`. The environment is
  a scratch HOME/GOPATH/GOCACHE, the declared keys (which may supply a cache), the perturbation
  and the forced `CGO_ENABLED=0`, `GOENV=off`, `GOFLAGS=-mod=readonly`, `GOPROXY=off`,
  `GOSUMDB=off`, `GOTELEMETRY=off`, `GOTOOLCHAIN=local` and `GOWORK=off`, so module resolution
  never reaches the network. A protected test repository that is not at the bound revision or not
  clean before the run is `test-repository-drift`, and one changed during the run is
  `test-repository-mutated`; both are `invalid_control`.
- `BBF-V0-027`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2011) The runner
  synthesizes the hook receipt from the provider's `test2json` events: the target and each
  unrelated and setup example take their own top-level terminal action (`pass`, `fail` or `skip`),
  a missing example is `not_run`, and the native receipt digest is the event stream's digest.
  Outcomes are keyed by (package, test); events naming more than one package are
  `invalid_control` with `provider-output-contradictory`, so a same-named test in another package
  is never attributed to an example. A
  failure whose output reports a panic, attributed to that example or to no example, is a `panic`
  failure, never an assertion. The admitted receipt is classified by the unchanged BBF-V0-006
  rules into the same six statuses. A provider exit other than 0 or 1, timeout, overflow or other
  boundary loss, or a `build-fail` event, is `infrastructure_failed` with
  `provider-process-failed`. A non-JSON line is `invalid_control` with `provider-output-invalid`,
  and two terminal actions for one example are `invalid_control` with
  `provider-output-contradictory`.
- `BBF-V0-028`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2011)
  `execute-receipt` refuses a provider-profile plan with `receipt-profile-unsupported`, and
  `verify-receipt` refuses any receipt whose plan declares a provider or whose report names one,
  until the provider's native event stream has its own retained-evidence binding.
- `BBF-V0-029`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2027) A request
  may instead declare `provider` with name `rspec`, a pinned RSpec `toolchain` command (for example
  a wrapper that runs RSpec through a container), one repository-relative project directory as
  `package` equal to the target project, and 1..256 unique `examples`, each one RSpec example id: a
  project-relative spec file whose path components do not start with a dot, ending in `.rb`, and
  one scoped id such as `spec/checkout_spec.rb[1:2]`, at most 1024 bytes, including the target
  assertion. Its runner identity is `rspec` with browser and browser version `none` and config file
  `.rspec` or `Gemfile`. Each control's unrelated criteria and required setup may be RSpec example
  ids and must be other selected examples. The other BBF-V0-024 rules (perturbation keys,
  application markers, clean test repository) are unchanged, and a mixed identity (an `rspec`
  provider with a `go-test` runner or `go.mod`, or the reverse) is refused.
- `BBF-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2027) The runner
  runs the pinned RSpec command after its digest check, in the declared project directory of the
  same disposable clone BBF-V0-026 stages, as `<toolchain> --format json --out
  .corvint-behavior-rspec.json <examples>`, so a container wrapper that shares the directory writes
  the report where the runner reads it. A pre-existing entry at the report path is refused before
  the run. The environment is a scratch HOME and TMPDIR, a fixed PATH, the declared keys, the
  perturbation and the forced `BUNDLE_FROZEN=true` and `SPEC_OPTS=` (empty), which neither the
  declared nor the perturbation environment may name. The report is read through a root-confined,
  regular-file, no-symlink open of at most 16 MiB; its bytes are the native receipt. The
  BBF-V0-026 drift and mutation rules apply unchanged.
- `BBF-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2027) The runner
  synthesizes the hook receipt from the report: each selected example takes the status of the
  report example whose id, without its leading `./`, equals it (`passed`, `pending` as `skipped`,
  `failed`); a missing example is `not_run`. A failure is an `assertion` only when its exception
  class is `RSpec::Expectations::ExpectationNotMetError`,
  `RSpec::Expectations::MultipleExpectationsNotMetError` or `RSpec::Mocks::MockExpectationError`;
  any other failure, or none stated, is a `panic` failure, never an assertion. A missing,
  unreadable or non-JSON report, a report without `examples`, `summary.example_count` or
  `summary.errors_outside_of_examples_count`, or another status is `provider-output-invalid`; a
  nonzero `errors_outside_of_examples_count` (a load or syntax error) is `infrastructure_failed`
  with `provider-build-failed`; an example count that disagrees with the list, a repeated example
  or an example that was not selected is `provider-output-contradictory`. An RSpec exit other than
  0 or 1, timeout, overflow or other boundary loss is `infrastructure_failed` with
  `provider-process-failed`. The admitted receipt is classified by the unchanged BBF-V0-006 rules.
- `BBF-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2027)
  `execute-receipt` refuses an RSpec provider plan with `receipt-profile-unsupported` and
  `verify-receipt` refuses any receipt whose plan or report names the RSpec provider, as BBF-V0-028
  does for every provider, until its report has its own retained-evidence binding.
- `BBF-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2027) A live test
  MUST run the RSpec negative controls with real RSpec through a docker-prefixed toolchain and
  classify them: a perturbation that breaks the target expectation is `killed`, an error raised
  outside an expectation is `invalid_control` with a `panic` failure, an unused perturbation is
  `survived`, a hook naming an undeclared key is `invalid_control` with
  `perturbation-output-invalid`, and a spec that writes a tracked file is classified while the
  protected test repository stays at its revision. The test is skipped without docker or the
  owner-approved qualification image `corvint-rspec-qual:2026-10-20`.
- `BBF-V0-214`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2029) The go-test
  provider MUST read the pinned Go driver once, check those bytes against its digest and run
  exactly them from a private copy, with `GOROOT` set to the directory two levels above the pinned
  path, the root the in-place driver resolves. A swap of the original after the check MUST NOT
  run; a swap before staging is `command executable drift` or `command changed before staging`; a
  root without `pkg/tool` is `provider toolchain GOROOT unavailable`; a declared or perturbation
  `GOROOT` key is refused at plan time. Limit: only the driver is digest-pinned. The compiler,
  linker and standard library under `GOROOT` and the RSpec toolchain, which may be a wrapper that
  resolves paths from its own location, still run in place; `TestBBFV0214LimitToolsRunInPlace`
  recorded both swaps running (V1-2215); `BBF-V0-820` closes both.
- `BBF-V0-820`: (experimental under the standing lane auto-accept; V1-2215; amends the
  `BBF-V0-214` limit) The go-test provider MUST, right after the driver's digest check, digest the
  GOROOT members a `go test` run reads (`VERSION`, `go.env`, `bin`, `lib`, `pkg/include`,
  `pkg/tool` and `src`, without any `testdata` directory), then copy them into a private GOROOT
  under the run's scratch directory, digesting the bytes it copies, put the verified driver bytes
  at its `bin/<driver>`, and run that driver with `GOROOT` set to the private copy, in place of the
  `BBF-V0-214` root two levels above the pinned path. A difference
  between the two digests is `provider toolchain GOROOT changed before staging`; a member entry that
  is not a regular file or directory is `provider toolchain GOROOT holds an unsupported entry`;
  either refuses before anything runs. The digest and the copy run inside the control's command
  budget and stop when the run is cancelled or the budget is spent (`command budget exhausted`),
  and the driver then gets only what remains of that budget, so staging never extends the run into
  the time reserved for cleanup (Codex review of linux-w2). The RSpec provider MUST run its toolchain from a verified
  private copy, as `runPinned` runs every pinned command. A swap of a GOROOT tool, the standard
  library or the RSpec toolchain after its check therefore never runs. Non-goals: pinning GOROOT
  digests in the plan (the check binds the run's own reads, not a plan-time value), and tools
  outside GOROOT. Failure modes: each go-test provider run copies about 220 MB, which adds that
  copy's time to every control; an RSpec toolchain that resolves paths from its own location no
  longer finds them and fails its run, so such a toolchain must be pinned as a self-contained
  wrapper. Falsifier: a GOROOT tool, a standard library file or the RSpec toolchain swapped between
  its check and the run whose swapped bytes run. Evidence: `TestBBFV0820ToolsRunFromVerifiedCopies`, `TestBBFV0820StagingHonorsTheCommandBudget`
  (replaces `TestBBFV0214LimitToolsRunInPlace`, which recorded the swapped runs). Rollback: run the
  driver with the original GOROOT and the RSpec toolchain from its path again.
- `BBF-V0-215`: (proposed; V1-2524) A plan's `contract_file`, `test_file`, `config_file` and Go
  provider `package` MUST be judged as canonical slash Git paths on every host: nonempty, not
  rooted at `/`, unchanged by slash path cleaning, not `..` or below `../`, and free of `\` and
  `:`. The check MUST NOT use the host's native path cleaning, which on Windows rewrites
  `tests/x.spec.ts` to `tests\x.spec.ts` and refused every plan there with `contract and test paths
  must be repository-relative`. Failure mode: a Windows spelling (`tests\x.ts`, `C:\repo\x.ts`,
  `C:/repo/x.ts`, `\\server\share\x.ts`) is refused with the same diagnostic on every host,
  including Linux, where a backslash was previously an ordinary file-name byte; plans that used
  such a name no longer build. Evidence: `TestBBFV0215RepositoryRelativeUsesSlashSyntaxOnEveryHost` runs
  the Windows spellings on Linux; the native Windows rerun is NEEDS-HOST. Rollback: restore the
  native `filepath` check in `repositoryRelative`.
- `BBF-V0-216`: (proposed; V1-2030, decision 0552) Every `ServerIdentity` an isolation observation
  records carries `source`, `observed` or `hook-reported`. The runner never contacts the server
  (BBF-V0-020), so every identity it derives from a reset/health hook report is `hook-reported`;
  `observed` is reserved for an independent server probe, which no profile performs. A report
  whose declared isolation rests on any identity that is not `observed`, or on none, MUST append
  the report-level coverage gap `isolation/hook-reported-identity` after its criterion-level gaps
  (BBF-V0-008); the `/` keeps it disjoint from every control ID. A forged hook that reports the
  declared identity therefore still yields a kill whose report has a gap, so no consumer that
  requires an empty gap list reads it as complete. `verify-receipt` recomputes the gap and
  re-derives each identity with its source from retained bytes, so a receipt that drops the gap or
  relabels an identity `observed` does not verify. Identity drift compares only `server_url` and
  `application_identity`. Plans, reports and receipts without isolation are unchanged
  (BBF-V0-023). Failure mode: an isolation receipt exported before this row records identities
  without `source` and no longer verifies; re-run `execute-receipt`. Evidence:
  `TestBBFV0216HookReportedIdentityCapsVerdict`. Rollback: drop `source` and the gap from
  `isolationIdentity`, `hookReportedIsolation`, `Execute` and `recountReport`.
- `BBF-V0-217`: (proposed, standing lane auto-accept 2026-10-11; V1-2688, GH #1789) A request
  that declares the BBF-V0-019 shared-server isolation MAY also declare `authentication` with
  profile `authenticated-disposable` and one to four `storage_state` files, each `{path, sha256}`.
  The record is part of the approved plan digest. Each file MUST be a clean absolute regular file
  of at most 1 MiB with no symlink component, outside the disposable root and every repository
  root, and a JSON object whose keys are a subset of `cookies` and `origins`. BBF never starts,
  signals or stops the server; the caller's reset hook owns it. A request without isolation is
  refused with `authentication-requires-isolation`; a wrong profile, count or file shape with
  `authentication-input-invalid`; bytes that differ from a pin with `authentication-input-drift`.
  Under the profile, a declared environment key with a credential segment (the NEA-V0-013 list),
  or a declared value that matches the shared secret screen, is refused with
  `authentication-environment-secret-shaped`.
- `BBF-V0-218`: (proposed; V1-2688) Execution rebuilds the plan, which rechecks every storage-state
  digest before any hook runs, after each shared-server reset and again after the last control.
  Drift after a reset adds the attempt reason `authentication-input-drift` and skips that control;
  drift before execution or after the last control returns `authentication-input-drift` and
  publishes no report. The report copies only the
  declared paths and digests into `authentication` and adds the limitation that the disposable
  account is caller-declared and storage-state contents are never retained. Before a report or
  evidence receipt is returned, every cookie value and origin storage value of at least 8 bytes is
  searched for in its serialized bytes and in each retained raw payload (plan preimage, hook,
  native and isolation bytes) before base64 encoding; a match refuses it with
  `authentication-material-in-report`.
  `verify-receipt` refuses a report whose `authentication` differs from the plan's.
- `BBF-V0-219`: (proposed; V1-2688) Plans, reports and receipts without `authentication` are
  byte-identical to their form before BBF-V0-217. Failure mode: a caller who moves or refreshes
  the storage-state file must rebuild and re-approve the plan. Rollback: remove the field and its
  checks; no stored plan without it changes.

### Approval-bound evidence wire

The closed envelope fields are `schema`, `plan_preimage`, `approval`, `tool`, `raw_attempts`,
`report`, `unsupported`, and `digest`, in that order. Byte members use JSON base64. Hashes are
lowercase SHA-256 with the `sha256:` prefix. `plan_preimage` is the existing canonical plan JSON
with its `digest` member set to the empty string, without a trailing newline; hashing the decoded
bytes reproduces the approved plan digest. No workspace is needed to recompute it.

The approval fields, in canonical order, are `schema` (`corvint-browser-behavior-approval/1`),
`plan_digest`, `method` (`explicit-approve-plan`), and `digest`. Its hash covers compact UTF-8 JSON
with `digest` empty and HTML escaping disabled. The envelope hash similarly empties only its own
`digest`; embedded report digests retain the existing report/0 convention. The explicit approval
flag is observed after the executor verifies it against the rebuilt plan; this is not a signature
or an authenticated operator identity.

Each raw row names `control_id`, `ordinal`, `attempt`, `hook_bytes`, `hook_sha256`, `native_bytes`,
`native_sha256`, and `omissions`. A native report must appear in the hook's `artifacts` with the
same digest as `native_receipt_sha256`. Both byte sets are read before cleanup. The hook digest
also equals its process stdout digest; native bytes must match the parsed hook and retained
artifact observation. Setup, target and unrelated assertion observations, retry identity, process
bounds, cleanup and workspace digests remain in the bound report/0. The verifier recomputes
classification instead of trusting a claimed kill. Explicitly unsupported claims are authenticated
operator identity, authenticated hook semantics and independent process/artifact observation.
`verified-observation` describes internally consistent retained evidence, not authenticated execution
or general test adequacy. A non-passing control remains non-passing in the verification output.

Rollback removes the two new operations and evidence profile; prior plan/0, hook/0 and report/0
consumers continue to read the unchanged legacy output.

## Simpler baseline and non-goals

The baseline is a caller manually running one deliberately broken browser test and retaining its
native receipt. V0 standardizes bounded orchestration, identity and classification; it does not
generate product-specific perturbation code, edit Playwright tests, discover criteria, provision a
browser, authenticate hook semantics, sandbox arbitrary caller code, mutate a production service or
replace the full suite. Issue 53's provider-record generation and reconciliation frontier are
independent.

## Trust and mutation boundary

The contract and control definitions are caller declarations. Exact Git revisions and digests make
drift visible but do not prove that a hook implements its label honestly. Approval authorizes only
the digest-bound plan. The execution process receives a fresh staged copy of each pinned hook and a
bounded JSON request; it runs in the explicitly marked disposable workspace with a minimal declared
environment. The runner reads and hashes the workspace, hook output and artifacts, then invokes the
separate cleanup hook and requires the original digest. A hook capable of reaching undeclared
external state remains outside the trust boundary; the plan must declare no persistent external
state, or declare the BBF-V0-019 shared-server isolation, and the result retains that limitation.
Under that isolation the server identity is reported by the caller's pinned reset hook, not
observed by the runner.

Malformed identities, stale bytes, unknown statuses, missing expected observations and boundary
violations fail closed. Infrastructure failures remain distinct from failed assertions. Cleanup is
always attempted with its own bounded context even after execution failure or cancellation.

### Owned emitted error codes

| Code | Meaning |
| --- | --- |
| `operator-authorization-required` | Execution lacks approval of the exact plan digest. |
| `approved-plan-drift` | Rebuilding current immutable inputs does not reproduce the approved plan. |
| `receipt-invalid-control` | Offline verification cannot reproduce the retained receipt's shape, pinned identities, digests, inventories, observations or outcome. |
| `receipt-native-path-invalid` | A declared native-report path is absolute, noncanonical, parent-relative or longer than the path bound. |
| `receipt-native-report-invalid` | Reading the native report fails, exceeds the retained-byte bound or does not match its declared digest. |
| `receipt-plan-unsafe` | The canonical plan preimage cannot be encoded or fails the secret screen. |
| `receipt-secret-shaped-data` | The envelope, raw hook output or native report contains secret-shaped data; export refuses without echoing the value. |
| `receipt-tool-identity-invalid` | The executing companion's name, version, revision or executable digest is missing or malformed. |
| `history-receipt-invalid` | An inert-control history receipt fails offline verification against its pinned digests. |
| `history-bound-exceeded` | An inert-control history is empty or holds more than 64 receipts. |
| `authentication-requires-isolation` | `authentication` is declared without the shared-server isolation (BBF-V0-217). |
| `authentication-input-invalid` | The authentication profile, file count, path, location, size or JSON shape is invalid (BBF-V0-217). |
| `authentication-input-drift` | A storage-state file's bytes differ from its pin at planning, before execution, after a shared-server reset or after the last control (BBF-V0-217, BBF-V0-218). |
| `authentication-environment-secret-shaped` | Under authentication, a declared environment key is credential-named or its value is secret-shaped (BBF-V0-217). |
| `authentication-material-in-report` | A storage-state value would appear in a report or receipt; nothing is published (BBF-V0-218). |
| `receipt-profile-unsupported` | `execute-receipt` was asked to retain evidence for a runner-neutral provider plan, which has no retained-evidence binding yet (BBF-V0-028, BBF-V0-032). |

Attempt reasons added by the opt-in profiles (BBF-V0-021, BBF-V0-025..027, BBF-V0-030..031): `isolation-reset-failed`,
`isolation-identity-drift`, `isolation-health-failed`, `isolation-observation-unexpected`,
`perturbation-output-invalid`, `test-repository-drift`, `test-repository-mutated`,
`provider-staging-unavailable`, `provider-process-failed`, `provider-build-failed`,
`provider-output-invalid` and `provider-output-contradictory`.

## Acceptance matrix

| Behavior | Required evidence |
| --- | --- |
| deterministic plan and approval | round-trip, unknown-field, trailing-JSON, digest-drift and no-approval tests |
| expected criterion kill | exact assertion failure plus unrelated-pass and cleanup witnesses |
| survivor and false positives | tautology, hidden duplicate, wrong value, unrelated failure and wrong assertion fixtures |
| execution failures | timeout, crash, overflow, cancellation and descendant-cleanup fixtures |
| immutable binding | stale revision/config/hook/artifact and retry/ordinal adversarial fixtures |
| aggregate | six raw status counts, nonzero raw mutation denominator and partial-support fallback |
| application markers | `TestBBFV0015ApplicationClassification`, `TestBBFV0015UnmarkedControlKeepsLegacyClassification`, `TestBBFV0015AggregateApplication`, `TestBBFV0015PlanMarkerValidation` |
| inert listing and compatibility | `TestBBFV0016InertControlsListedSeparately`, `TestBBFV0016LegacyWireUnchanged`, `TestBBFV0016InertControlAbstainsOnStrength`, `TestBBFV0016ControlApplicationSeparatesInertAndUnobserved` |
| cross-revision inert defect | `TestBBFV0017RepeatedNotAppliedAcrossRevisions` |
| approval-bound retained evidence | `TestBBFV0013OfflineReceipt`, `TestBBFV0013ReceiptTampering`, `TestBBFV0013ReceiptFailuresNeverKill`, `TestBBFV0013SecretOutputRefused`, `TestBBFV0013ReceiptApprovalAndBounds`; actual CLI export/verify smoke |
| attempt without an admitted hook receipt | `TestBBFV0018InfrastructureFailedAttemptVerifies`, `TestBBFV0018ForgedNilReceiptAttempts`, `TestBBFV0018ForgedUnobservedAndHiddenReceipts`, `TestBBFV0018PreHookAttemptShapesVerify` |
| declared shared-server isolation | `TestBBFV0019IsolationProfileIsExplicitAndBounded`, `TestBBFV0021IsolationFailureIsNeverKilled`, `TestBBFV0022IsolationEvidenceReceipt`, `TestBBFV0023LegacyWireIsByteIdentical` |
| runner-neutral Go provider | `TestBBFV0024ProviderPlanIsBounded`, `TestBBFV0026GoProviderEndToEnd`, `TestBBFV0027ProviderEventsAreClosed`, `TestBBFV0028ProviderEvidenceReceiptIsRefused` |
| RSpec provider binding | `TestBBFV0029RSpecProviderPlanIsBounded`, `TestBBFV0030RSpecProviderEndToEnd`, `TestBBFV0031RSpecReportIsClosed`, `TestBBFV0032RSpecEvidenceReceiptIsRefused`; live `TestBBFV0033LiveRSpecNegativeControls` (real RSpec 3.13.6 in `corvint-rspec-qual:2026-10-20`) |
| authenticated-disposable inputs | `TestBBFV0217AuthenticationPlanIsBounded`, `TestBBFV0218AuthenticationDriftAndMaterial`, `TestBBFV0218AuthenticationReceiptBinding`, `TestBBFV0219LegacyWireUnchanged` |
| host-independent plan paths | `TestBBFV0215RepositoryRelativeUsesSlashSyntaxOnEveryHost` (Windows spellings on every host); native Windows rerun NEEDS-HOST |

Focused package and command tests run first. Repository completion additionally requires the
canonical Go test/vet and interoperability gates, independent review, and the dogfood CEM/OCM
completion loop. Synthetic fixtures prove classifier behavior only; live adopter/browser utility is
`NOT_OBSERVED` until a caller supplies and approves real safe hooks.

## Rollout, rollback and maintenance

The capability ships as a separately built experimental companion and internal package. It is not
part of the default `corvint` command, install path or policy input. Schema or status-vocabulary
changes require a new profile or explicit compatibility rule. Rollback deletes the companion,
package and this opt-in spec; existing corpus behavior records, native Playwright receipts and
default commands remain unchanged.

## Traceability

| Requirements | Implementation boundary | Evidence |
| --- | --- | --- |
| BBF-V0-001..003 | `internal/behaviorfalsify` planner/types; `cmd/corvint-behavior-falsify` | plan/decode/vocabulary tests |
| BBF-V0-004, BBF-V0-010 | live hook boundary and `internal/procgroup` integration | disposable-root, staging, cleanup and interruption tests |
| BBF-V0-005..009 | receipt validator, classifier and aggregate | synthetic falsifier matrix and adversarial receipt tests |
| BBF-V0-014 | acceptance-safe TMPDIR transport | `TestBBFV0014TempRootTransport`, `TestBBFV0014TempRootInterruptedStaging` |
| BBF-V0-013 | `internal/behaviorfalsify/receipt.go`; companion `execute-receipt` and `verify-receipt` | offline, tampering, missing evidence, secret and bound tests |
| BBF-V0-011..012 | conformance fixtures and report limitations | end-to-end synthetic command tests and independent review |
| BBF-V0-015..016 | `internal/behaviorfalsify` classifier, receipt verifier; `internal/testacceptance` consumer | `TestBBFV0015ApplicationClassification`, `TestBBFV0015PlanMarkerValidation`, `TestBBFV0016InertControlsListedSeparately`, `TestBBFV0016LegacyWireUnchanged`, `TestBBFV0016InertControlAbstainsOnStrength` |
| BBF-V0-017 | `internal/behaviorfalsify/history.go` | `TestBBFV0017RepeatedNotAppliedAcrossRevisions` |
| BBF-V0-018 | `internal/behaviorfalsify/receipt.go` capture, `unobservedAttemptVerifies` and `unadmittedAttemptVerifies` | `TestBBFV0018InfrastructureFailedAttemptVerifies`, `TestBBFV0018ForgedNilReceiptAttempts`, `TestBBFV0018ForgedUnobservedAndHiddenReceipts`, `TestBBFV0018PreHookAttemptShapesVerify` |
| BBF-V0-019..028 | `internal/behaviorfalsify/profiles.go`; `executeAttempt`, `classifyAttempt`, `ExecuteReceipt` and `VerifyReceipt` | `TestBBFV0019IsolationProfileIsExplicitAndBounded`, `TestBBFV0021IsolationFailureIsNeverKilled`, `TestBBFV0022IsolationEvidenceReceipt`, `TestBBFV0023LegacyWireIsByteIdentical`, `TestBBFV0024ProviderPlanIsBounded`, `TestBBFV0026GoProviderEndToEnd`, `TestBBFV0027ProviderEventsAreClosed`, `TestBBFV0028ProviderEvidenceReceiptIsRefused` |
| BBF-V0-029..033 | `internal/behaviorfalsify/profiles.go` (`validateProvider`, `runRSpecProvider`, `rspecReceipt`) and `plan.go` (`observationIDValid`) | `TestBBFV0029RSpecProviderPlanIsBounded`, `TestBBFV0030RSpecProviderEndToEnd`, `TestBBFV0031RSpecReportIsClosed`, `TestBBFV0032RSpecEvidenceReceiptIsRefused`, `TestBBFV0033LiveRSpecNegativeControls` |
| BBF-V0-820 | `internal/behaviorfalsify/goroot_stage.go` (`stageGOROOT`), `profiles.go` (`runProvider`, `runRSpecProvider`) | `TestBBFV0820ToolsRunFromVerifiedCopies` |
| BBF-V0-214 | `internal/behaviorfalsify/execute.go` (`stageVerified`), `profiles.go` (`runProvider`, `validateProvider`) | `TestBBFV0214ToolchainRunsVerifiedBytes`; limit superseded by BBF-V0-820 |
| BBF-V0-215 | `internal/behaviorfalsify/plan.go` (`repositoryRelative`) | `TestBBFV0215RepositoryRelativeUsesSlashSyntaxOnEveryHost`; native Windows rerun NEEDS-HOST |
| BBF-V0-217..219 | `internal/behaviorfalsify/authentication.go` (`validateAuthentication`, `AuthenticationMaterial`), `execute.go` (`execute`), `receipt.go` (`ExecuteReceipt`, `VerifyReceipt`) | `TestBBFV0217AuthenticationPlanIsBounded`, `TestBBFV0218AuthenticationDriftAndMaterial`, `TestBBFV0218AuthenticationReceiptBinding`, `TestBBFV0219LegacyWireUnchanged` |
| BBF-V0-216 | `internal/behaviorfalsify/profiles.go` (`isolationIdentity`, `hookReportedIsolation`, `declaredServer`), `execute.go` (`Execute`), `receipt.go` (`recountReport`) | `TestBBFV0216HookReportedIdentityCapsVerdict` |

## Unresolved decisions and promotion criteria

The exact adopter hook protocol may need a future version after live use. Promotion beyond
experimental requires at least one caller-owned disposable Playwright fixture exercising every
supported control kind, all deterministic repository gates passing, no source/shared-state mutation,
and owner review of the resulting plan and raw receipts. Browser portability, hook authenticity and
universal adequacy remain explicitly unclaimed.
