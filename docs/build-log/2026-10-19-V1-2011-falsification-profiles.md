# 2026-10-19 V1-2011: shared-server isolation and runner-neutral falsification profiles

Ticket: V1-2011 (GitHub #1273 and #1274).

## Problem

Browser behavior falsification admitted only a disposable workspace with no persistent external
state (BBF-V0-004) and only a Playwright hook receipt. Adopters whose application runs as one
long-lived local server could not run controls at all (#1273). Adopters whose criteria are
checked by a non-browser suite had no way to rerun selected examples under a control (#1274).

## Decisions

`BBF-V0-019` to `BBF-V0-028` are added to `docs/specs/browser-behavior-falsification-v0.md` and
accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

- Isolation is explicit and opt-in. A request declares `isolation` (profile `shared-server`, a
  loopback URL, an application digest or Git revision, and a pinned reset hook) and
  `external_state: declared-shared-server`. Without it, `external_state` must stay `none` exactly
  as before. Loopback-only URLs keep the profile local, and the runner never contacts the server:
  the reset hook reports the identity it observed.
- The reset hook runs before every attempt, so it also runs between controls. The same hook runs
  again in phase `verify` after cleanup. Its output is bound to plan digest, phase, control and
  attempt, so a replayed observation is not admitted.
- Isolation failure is checked first in `classifyAttempt`. Reset failure, identity drift and a
  failed health check are `infrastructure_failed`, never `killed`. The receipt verifier
  recomputes through the same function, so a dropped or altered observation cannot verify as a
  kill.
- The runner-neutral profile reuses the existing control hook, approval, staging and containment.
  The hook prints a closed perturbation document naming only declared environment keys. The
  runner then runs the pinned Go toolchain (`go test -json`) and synthesizes the hook receipt from
  test2json events, so the unchanged BBF-V0-006 classifier yields the same six statuses.
- The Go toolchain runs in place after its digest check, because Go resolves GOROOT from its own
  executable path, so a staged copy would lose its standard library. The window between the
  digest check and exec is a recorded residual.
- The provider environment is closed (`GOPROXY=off`, `GOFLAGS=-mod=readonly`, `GOTOOLCHAIN=local`,
  `GOENV=off`, `GOWORK=off`, `CGO_ENABLED=0`), so module resolution cannot reach the network. A
  test repository that is dirty or moved before the run, or changed by it, is `invalid_control`.
- A panic, attributed or not, is a `panic` failure and never an assertion kill.
- `execute-receipt` refuses provider plans with the new owned code `receipt-profile-unsupported`
  until the event stream has its own retained-evidence binding.
- The new fields use `omitempty`, so legacy plans, reports and receipts stay byte-identical.
  `TestBBFV0023LegacyWireIsByteIdentical` pins plan and report digests captured at the base
  revision c27d54e9.

## Independent review (Codex, one round) and repairs

The Codex review of #1285 found three defects. Each repair has a test that fails without it.

- P1, rewritable isolation identity. The receipt kept the parsed identities but not the hook
  stdout they came from. A drifted-after receipt could have its `after` identity rewritten to the
  declared one and its digests recomputed, and verification accepted it as a kill. Each retained
  attempt now also retains the reset and verify phase stdout (`isolation_reset_bytes`,
  `isolation_verify_bytes`, `omitempty`). `verify-receipt` requires each to match the recorded
  stdout digest and to re-derive the recorded identity through the executor's own parser
  (`isolationIdentity`). Test: `BBF-V0-022 rewriting a drifted identity cannot forge a kill`.
  Process evidence itself is still unauthenticated (`independent-process-and-artifact-observation`
  stays in the receipt's unsupported list), so a forger who rewrites the bytes and the digest
  together is out of scope, as for hook stdout.
- P1, provider ran in the protected checkout. A test that wrote a tracked file modified the
  real test repository, detected only afterwards. The provider now runs in a disposable
  `git clone --no-hardlinks --no-checkout --template=` copy detached at the bound revision, inside
  the attempt's scratch directory and within the attempt timeout. Test: a `write` control whose
  example overwrites its own source is killed in the clone, and the protected repository stays
  clean at the same revision.
- P2, package patterns and cross-package events. `example...` passed the package validator, and
  outcomes were keyed by test name alone. Any `...` is now refused, outcomes are keyed by
  (package, test), and events from a second package are `provider-output-contradictory`. Tests:
  `BBF-V0-024 refuses embedded pattern` and `BBF-V0-027 other package`.

Fail-without-fix: with the three production files restored to the reviewed head 17e98b009 and the
new tests kept, exactly the four new subtests (and the existing repository-unchanged subtest)
failed; with the repairs, the focused tests and the package, command and `testacceptance` suites
pass.

## Measurements

The ten focused tests (`-run 'TestBBFV00(19|2[0-9])'`, 63 PASS lines, no skips), including four
controls run through real `go test` subprocesses in `TestBBFV0026GoProviderEndToEnd`, took 22 s
to 31 s per run on a shared host. The full package suite took 95 s.

## Evidence

Focused tests `TestBBFV0019`..`TestBBFV0028` pass. The fail-without-fix proof applied four
mutations in one run, then restored the files:

- removed the isolation check in `classifyAttempt`
- dropped `omitempty` on `Request.Isolation`
- removed the panic attribution
- removed the `execute-receipt` provider refusal

Under these mutations `TestBBFV0021`, `TestBBFV0022`, `TestBBFV0023`, `TestBBFV0026`,
`TestBBFV0027` and `TestBBFV0028` failed. The validation tests `TestBBFV0019` and `TestBBFV0024`
do not compile without the new fields. The existing package suite passes unchanged.

## NOT_RUN and residuals

- Ruby provider binding: depends on V1-2010 (example selectors), which is not on main.
- Application markers under the provider profile are refused, not supported.
- No evidence receipt (`execute-receipt`) exists for the provider profile.
- The disposable clone adds one local `git clone` and `checkout` per provider attempt.
- The Go toolchain is not staged (time-of-check to time-of-use window).
- No live adopter shared server or adopter Go suite was exercised (`NOT_OBSERVED`).

## Rollback

Revert the change. Requests without `isolation` or `provider` are unchanged and need no
migration. A plan that uses either profile is refused again by the prior validator.
