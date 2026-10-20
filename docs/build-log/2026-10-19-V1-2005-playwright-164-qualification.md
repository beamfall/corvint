# V1-2005: Playwright 1.64.0 qualification and typed e2e version refusal

Date: 2026-10-19
Ticket: V1-2005 (GitHub issue beamfall/corvint#1263).
Requirements: PWP-V0-032..034 (`docs/specs/playwright-external-provider-v0.md`, section "Playwright
1.64.0 and typed version refusal"), accepted under the owner's standing auto-accept of lane
requirements (2026-10-11).

## Problem

A consumer pinned to `@playwright/test@1.64.0` could not use external mode: `e2e` refused
`external-playwright-version-unqualified`, exited 1 and left stdout empty, so the consumer had no
document to read and no list of versions to pin to.

## Decisions

1. **Qualify 1.64.0 rather than only document it.** The 1.64.0 reporter internals the provider reads
   were checked in the installed sources and are unchanged; the live matrix passed (below).
2. **Bundled tuple keyed by exact runner version.** The single bundled-browser constant set became a
   table (`qualifiedBundledBrowsers`, Go and reporter). 1.63.0 keeps revision 1243 byte-for-byte;
   1.64.0 gets revision 1248 only. No tuple is inherited across versions, so 1.64.0 cannot bind on
   1.63.0 browser identity and the reverse.
3. **Narrow profile scope.** 1.64.0 is a `ReceiptRuntimeTuple` candidate for `/0` and `/3` on Node
   v22.23.2 only, the profiles the live matrix covered. `/1`, `/2`, Node v24.11.1 and the system-Chrome
   tuple stay 1.63.0-only. The freshness profile is unchanged (1.63.0, Node v22.23.3).
4. **Typed refusal as a document, not a new exit status.** Pre-run `e2e` refusals became a typed
   `E2ERefusal`; the one-shot command writes `corvint-playwright-e2e-refusal/0` to stdout. Exit 1 and
   the stderr line are unchanged so existing callers keep working. Existing refusal codes are reused;
   no new error code was added. Typing the refusal made `error-code-ownership-check` see the
   long-emitted `external-unsupported-option` literal, which no spec named; it is now listed in the
   spec's refusal vocabulary. Removing the unused `errors` import shifted `runner.go` by one line, so
   the error-code citations in `js-live-test-provider-v0.md` and this spec were repinned (same
   content, new line numbers).

## Install evidence

`/private/tmp/cem10-build/toolchains-2026-10-17/playwright-1.64/` (user-space, owner-approved
toolchain root of 2026-10-17): `@playwright/test`, `playwright` and `playwright-core` 1.64.0 each
matched their lockfile sha512 integrity; the package-selected browser install exited 0.
Observed tuple: Darwin arm64, revision 1248, manifest 156.0.8078.4, observed
`Google Chrome for Testing 156.0.8078.4`, SHA-256
`77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634`.

## Measurements and evidence

- Live 1.64.0 (Node v22.23.2, control v22.23.3): `TestQualifiedPlaywrightLive` 30.06 s,
  `TestQualifiedPlaywrightAttemptsLive` 25.88 s, both devices-spread tests ~1.1 s; package
  `ok 58.565s`. System Chrome 155.0.8059.85 stayed unqualified; the control Node abstained with
  `runtime-tuple-unqualified`.
- Live 1.63.0 regression with the same four tests: `ok 63.010s`.
- Unit: `internal/jstestprovider` and `cmd/corvint-js-test-provider` packages pass in full; guards
  `tools/fixture-maintenance-check`, `internal/testvaliditydoc`, `internal/appflows` and
  `cmd/corvint-go-test-provider` pass; `go vet` clean on touched packages.
- Fail without fix (each mutation reverted after): removing the 1.64 runtime-tuple case or the 1.64
  bundled entry fails `TestPWPV0032...` and `TestPWPV0033...`; an untyped version refusal fails both
  `TestPWPV0034...` tests; a CLI that does not write the document fails
  `TestPWPV0034E2EVersionRefusalWritesTypedDocument`; not admitting 1.64.0 fails `TestPWPV0032...`
  and `TestPWPV0034PreRunRefusalsAreTyped`.

## Identity

Receipts that built before are byte-identical: no receipt struct, member or encoding changed, the
1.63.0 tuple values are unchanged (asserted by `TestPWPV0032...` "no 1.63 regression" and the live
1.63.0 run). The embedded reporter bytes changed but are not part of receipt identity.

## NOT_RUN

- `make gate` (owner preference for scoped work; `corvint affected` scope UNKNOWN, dependency-path
  units outside the selected guards not run).
- `/1` and `/2` live on 1.64.0; Node 24 on 1.64.0; system Chrome on 1.64.0; Linux amd64.
- The reporting consumer's checkout and CI: `NOT_OBSERVED`.

## Rollback

Revert the change: 1.64.0 leaves the version list, the bundled table and the runtime-tuple case, and
the refusal writer goes away. Receipts written on 1.64.0 then read `unqualified` and are neither
deleted nor rewritten.
