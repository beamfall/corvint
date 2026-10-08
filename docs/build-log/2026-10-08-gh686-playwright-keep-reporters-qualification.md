# 2026-10-08: Playwright keep-reporters qualification command (V1-1028, GitHub #686)

## Intent

[Issue 686](https://github.com/beamfall/corvint/issues/686): a keep-reporters receipt (`PWP-V0-010`
to `PWP-V0-013`) never projects passing, so an adopter who must keep the project's reporters gets no
passing evidence. The issue asks for a command that records a live keep-reporters qualification.
Requirements `PWP-V0-014` to `PWP-V0-018` are proposed, pending owner acceptance.

## Decisions

- **Provider first.** The answer proposed to the `PWP-V0-013` ordering question is that the provider
  reporter runs first, followed by the kept entries. Playwright calls reporters in list order, so the
  provider copies each callback's values before a kept reporter sees them. Retained provider-last
  receipts stay readable and never pass.
- **Differential qualification, not tuple admission.** `qualify-keep-reporters` runs the same
  external configuration replace-only and then with kept reporters. It qualifies only when both runs
  are complete and fully qualified on their own terms, the identity and every control config input
  are equal, all kept entries are known, and the outcomes match one to one. Test IDs are not
  compared, because they bind config inputs and the kept reporter modules are extra inputs. The record
  never adds a Node or Playwright tuple, so it stays inside decision 0448. Whether that matches 0448's
  intent is an owner question.
- **Exact match to project.** `e2e --keep-reporters --keep-reporters-qualification FILE` carries the
  decoded record in `projectReporters.qualification`. The record binds every config input of the keep
  run as `configInputs`. A receipt passes only when its profile, runner and Node versions, entries and
  config inputs exactly equal the record's; a changed, added or removed input abstains. A run with more
  than 256 inputs or an input that cannot be bound (not absolute, not a 64-hex digest) is incomplete.
- **Experimental.** While `PWP-V0-014` to `PWP-V0-018` are proposed, the command, the flag and the
  passing projection are labelled experimental and are not promoted or advertised as delivered.
- **Missing evidence is `not-run`.** Missing Playwright and other control-run failures, a refused or
  failing kept reporter, unobserved entries and cancelled or partial runs give named `not-run`
  reasons. The keep run is not started after an incomplete control run. The command exits nonzero
  for anything other than `qualified`.
- **Closed canonical record.** Profile `corvint-playwright-keep-reporters-qualification/0`, a fixed
  member order, closed reasons, a 256 KiB bound, a secret screen, and a strict decode whose only
  refusal is the value-free `keep-reporters-qualification-invalid`. With `--retain`, both receipts are
  retained and the record binds their SHA-256.

## Limits

- Provider-first order protects only values already copied in the same callback. Mutation that
  persists across callbacks is caught only by the differential comparison.
- Suites that are not repeatable fail closed. ESM, `node_modules` or other reporters whose module is
  `unknown` cannot qualify. Kept reporters' effects stay `unknown`.
- The record is local, unsigned evidence with the same trust as a retained receipt.

## Independent review

Codex (`gpt-6-astra`, read-only) reviewed `origin/main..HEAD`:

- P1, accepted: drift in a file loaded by both the config and a kept reporter, or a newly loaded input,
  did not invalidate a record. Fixed by binding all keep-run config inputs and requiring exact equality.
- P3, accepted: a kept reporter module also imported by the config produced a `qualified` record that
  failed its own encoder. Fixed by the same change, with a regression test.
- P1, not accepted as written: the passing projection is enabled before owner acceptance and a live run.
  The record is itself the per-host live qualification the issue asks for, and projection needs an
  exact match. The surface is labelled experimental and acceptance stays an owner question.

## Evidence

Tests (`TMPDIR` lane-local):

- `GOTOOLCHAIN=local GOMAXPROCS=3 go test -p 1 -count=1 -timeout 30m ./internal/jstestprovider ./cmd/corvint-js-test-provider ./internal/testvaliditydoc`
  passed, and so did the same command with `-race` and `go vet` on those packages.
- `node --test internal/jstestprovider/qualified-reporter_test.cjs` passed, 14 of 14.

Live Playwright qualification of the command is `NOT_RUN`. No live run was attempted in this lane, and no
browser or process that the lane did not start was touched.

The Corvint dogfood step was degraded: the adapter reported `FALLBACK`
(`corvint-event-rejected:dogfood-event-deadline`), and coding continued without Corvint context
collection.
