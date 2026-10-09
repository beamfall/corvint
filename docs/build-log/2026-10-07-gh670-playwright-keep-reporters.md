# 2026-10-07: Playwright external provider keeps project reporters on request (V1-0986, GitHub #670)

## Intent

[Issue 670](https://github.com/beamfall/corvint/issues/670): the external provider's controlled
config replaces the project's `reporter` list. Evidence that a project reporter would have produced
is therefore lost whenever the provider runs. The owner chose an opt-in option that keeps those
reporters. The receipt binds what the provider can observe about them and records the rest as
unknown. The default stays replace-only with byte-identical receipts. Requirements `PWP-V0-010` to
`PWP-V0-013` are proposed, pending owner acceptance.

## Decisions

- **Append, provider last.** `--keep-reporters` builds `[...kept, provider]`. The provider finds its
  own entry by its private report path. The original value is normalized in the controlled config.
  Module names and the built-in reporters' output paths resolve from the original config
  directory, because the controlled config lives in a scratch directory.
- **Bind identity, not effects.** Each kept entry records its resolved name. Its module is `bound`
  only when that exact path is already a reporter-observed config input, so the existing drift
  recheck covers it. Its options are bound by a SHA-256 over sorted-key canonical JSON, and option
  values are never retained. `effects` is always `unknown`.
- **Fail closed in both directions.** A binding without the keep-reporters config, a keep-reporters
  config without a binding, an open value, or a module digest that differs from the config input
  all refuse `project-reporters-invalid` and cannot project passing. A run whose reporter did not
  report the entries is the infrastructure failure `project-reporters-unobserved`.
- **Same profiles.** The member is opt-in on `/0` to `/3` rather than a new profile revision. Readers
  that predate it fail closed through the canonical re-encoding check. This is an owner question.
- Owned-server and freshness runs refuse `keep-reporters-unsupported-mode`. This includes the direct
  `RunFreshE2E` entry point, which refuses before any process starts.
- **No passing projection yet.** Kept reporters see each result before the provider does, and the
  mode has no live qualification. Like `/2`, a keep-reporters receipt is retained and decodes but
  projects non-passing until a live keep-reporters qualification is recorded.
- **`configDir` for file-writing built-ins.** `blob`, `html`, `json` and `junit` receive the original
  config directory as `configDir`, unless the project set one. Without it, their default and
  `PLAYWRIGHT_*_OUTPUT_NAME` outputs would resolve against the scratch directory, which is deleted
  after the run. Whether Playwright lets this option override its own value is unverified offline.

## Independent review

Codex (`gpt-6-astra`, read-only) reported three findings. All three were accepted and fixed:

- P1: unqualified keep-reporters receipts could project passing. They now abstain.
- P2: built-in default and environment-named outputs would land in the scratch directory. The
  built-ins now receive `configDir`.
- P2: `RunFreshE2E` bypassed the unsupported-mode guard. It now has its own guard and test.

## Evidence

Tests (`TMPDIR` lane-local, `-p 1 -count=1`):

- `go test ./internal/jstestprovider` passed. It covers the default-config golden, the kept list
  resolution evaluated by Node, a stand-in runner showing that the project reporter writes its
  evidence only in keep mode, binding shape mutations and the unsupported modes.
- The Node case `PWP-V0-011` in `qualified-reporter_test.cjs` covers absent, bound and unknown
  options, key-order independence and the entry-position checks.
- `go test ./internal/testvaliditydoc` passed. A keep-reporters receipt decodes, keeps its binding
  and abstains. The same receipt without keep-reporters passes.
- `go test ./cmd/corvint-js-test-provider` passed.

Live Playwright qualification of the option is `NOT_RUN`: the lane host had no complete Playwright
installation. It is only a unit-level assumption that Playwright passes the normalized list to
`onBegin` as `config.reporter`. If the shape differs, the run fails closed as
`project-reporters-unobserved`.

## Known limits

- A kept custom reporter that reads `config.configFile` sees the controlled config path.
- ESM, `node_modules` or symlinked reporter modules record `module: unknown`.
- Kept-reporter output counts toward the 4 MiB output bound.
- Outputs of kept reporters are not ingested; this is a non-goal.
