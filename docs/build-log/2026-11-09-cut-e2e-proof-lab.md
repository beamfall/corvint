# Decision 0593: the E2E proof lab is cut

Owner decision 2026-11-09 ("Cut it"); base `b1a117eb129687f9555fa0c826fadfa0627afe3b`, branch
`claude/cut-e2e-proof-lab`.

## What left

- Binaries: `corvint-test`, `corvint-tests-accept`, `corvint-behavior-falsify`,
  `corvint-playwright-minimize`, with their catalog trees and `llms.txt` entries.
- Packages: `internal/behaviorfalsify` (+`cli`), `internal/changee2e`, `internal/extmutate`,
  `internal/playwrightminimize` (+`cli`), `internal/skhproof`, `internal/testacceptance` (+`cli`).
- Guides: `docs/E2E-ADOPTION.md` (replaced by `docs/APPLICATION-MAP.md`, which covers only the
  manifest and `corvint flows appmap build`) and `docs/EXTERNAL-APP-ACCEPTANCE.md`.
- Specs retired with bodies kept: `CCE-V0`, `NEA-V0`, `PSM-V0`, `BBF-V0`, `SKH-V0`; `CTF-V0` keeps
  only `CTF-V0-010`. `AFP-V0-490..493` retired and `AFP-V0-447` amended: `corvint affected audit`
  has no `e2e` selector, and an `e2e` input member is refused (`unsupported-affected-audit`).
- `protocol/postmerge-host/workflow-graph.json`: the trusted-validation step no longer lists the
  two `corvint-tests-accept` commands; no workflow invoked them.

## What stayed

`corvint-test-runner` is a standalone binary again (authored tree, no `worker` entry). The
packages other routes import stay: `internal/swiftimpact`, `internal/playwrightruns`,
`internal/extevidence`, `internal/archivepath`. `internal/testvaliditydoc` now keeps its two
recorded external-projection fixtures under its own `testdata/recorded/`, since they used to live
in `internal/changee2e`. The `corvint-test:` stderr prefix of core `corvint test` is unrelated and
unchanged.

## Deferred to a `.github`-only change

`github-split-check` (`ASD-V0-706`) refuses a range mixing `.github` with code, so these follow
separately: the `ci.yml` `cce-live` job, the `.github/cishards` lab entries in
`walltime_test.go` and `package-costs.json`, `tools/ci-reuse-plan/release_live_workflow_test.go`,
then the `make cce-live` no-op stub. The `.github/cishards` replay literals for the deleted
packages left `.corvint/test-read-scope-literals.json` here, because `test-read-scopes-check`
refuses a literal for a package that no longer exists.

The amended live specs are `PTF-V0` (its acceptance consumer lived in `internal/testacceptance`),
`PMR-V2` (the planned `PMR-V2-002` emitter home), `LGP-V0` (TypeScript declares Mutation absent),
`CMS-V0` and the 1.0 product inventory; `docs/specs/INDEX.json` drops every deleted implementation
path. `conformance/postmerge-runtime-v2` still pins `internal/behaviorfalsify` and
`internal/testacceptance` sources by path; nothing reads them, and they resolve at `b1a117eb`.

## Open tickets

The CCE, NEA, PSM, BBF and SKH tickets in `.taskman/` are not closed by this change; they are
dispositioned against decision 0593 after it merges.
