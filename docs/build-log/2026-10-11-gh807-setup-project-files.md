# GH-807 / V1-1242: name unbound setup-project files in `test-file-unbound`

## Finding

The owner qualified an external Playwright run with `corvint-js-test-provider e2e` against a config
that uses project dependencies (setup projects). Playwright reported the setup files' results beside
the selected spec. Only the spec was passed with `--test-file`, so the run refused
`report-identity-unknown` with the bare detail `test-file-unbound` although every result passed.
`--test-arg` selects what Playwright runs and `--test-file` binds identity; the two are independent,
and neither the help nor the refusal said which file was unbound.

## Decision

`PWP-V0-019` (accepted under the owner's standing auto-accept (2026-10-11)) keeps the refusal and
names each distinct unbound anchor file, sorted, with its cause (`not passed with --test-file`,
`not digested by the reporter`, `reporter digest differs from the --test-file digest`), at most 8 then
`+N more`. The `/2` profile names no file (its fixed untrusted-output detail may be further
redacted to `[REDACTED]`). The `e2e` help states the `--test-arg`/`--test-file` rule and the
setup-project consequence; owned-server runs do not apply this check.

No option binds dependency-project files automatically. Such a file would be observed only by the
reporter's single digest, while a `--test-file` is digested by the provider before and after the
run (`input-identity-changed`); auto-binding would widen binding beyond that observation. Missing
binding stays a refusal.

## Evidence

- `TestQualifiedReportNamesUnboundSetupProjectFiles` (fixture: a `setup` project anchored at
  `auth.setup.cjs` and a dependent `one` project): binding only the spec refuses with the setup file
  named and leaves it unbound; binding both admits the run with stable IDs; each cause and the
  8-file bound are checked against literal details (duplicate anchors, unsorted input); the `/2`
  detail names no file.
- `TestProviderHelpExplainsSetupProjectFileBinding` checks the help text.
- A live Playwright setup-project run is `NOT_RUN` (no network or npm downloads in this lane); the
  change touches only the refusal detail and help, not the passing predicate.

## Rollback

Revert the change: the detail returns to the bare `test-file-unbound` and the help to its earlier
text. Retained receipts with a named detail stay readable because the detail is free text.
