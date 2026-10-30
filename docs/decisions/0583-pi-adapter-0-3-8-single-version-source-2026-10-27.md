# Decision 0583 — Pi adapter 0.3.8 and a single version source

Date: 2026-10-27. Status: experimental under the standing lane auto-accept (V1-2854). It applies
`AHI-020`, `AHI-024` and `AHI-071` and adds `AHI-240`.

## Context

Commit `8fc47b90f` bumped only `integrations/pi/package.json` (and, through `make
host-package-version-bump PKG=pi`, the compatibility manifests) to 0.3.7 for the empty-stdin
runner fix. The bump tool did not list `runtime.js` `PI_ADAPTER_VERSION`, the `workflow.js` tuple
and argv, or the native `piAdapterVersion`, so they stayed at 0.3.6: the same half bump decision
0558 repaired. The AHI-024 node test caught it, but only in the full gate, because CI's
`host-package-versions-check` step never compared locations.

## Decision

Every Pi location moves to 0.3.8 at once, for the reason decision 0558 gives: aligning the
emitters to 0.3.7 would change shipped content under an already-published version. 0.3.6 and the
never-emitted 0.3.7 refuse visibly. `package.json` `version` is the one source: `workflow.js`
imports `PI_ADAPTER_VERSION` from `runtime.js`, the node tests use that constant, and the bump
tool now owns the `runtime.js` and `pi_adapter.go` literals (`AHI-240`). `make
host-package-versions-check` runs `verify` for every package, so CI fails a manifest-only bump at
PR time. `runtime.js` does not import `package.json` at runtime, because the protected Pi build
bundles it and that import is unqualified on the Pi host.

## Rollback

Revert the change. Restoring 0.3.6 or 0.3.7 requires replacing the Pi package and the Corvint
binary as a pair.
