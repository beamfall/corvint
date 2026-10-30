# CEM Stable Bundle Command V0

Owner: Russell Lewis
Date: 2026-10-25
Requirement prefix: `CSB-V0`
Intent status: accepted (owner's standing CEM-V1 auto-accept, 2026-10-11 and 2026-10-20)
Delivery status: implemented
Authoritative inputs: GitHub issue #1767 and ticket V1-2672, `docs/specs/cem-stable-v1.md`
(`CEM-V1-240` plain assembly, `CEM-V1-341` ok envelope, `CEM-V1-350` nextActions,
`CEM-V1-573..576` multi-runner requests), `internal/cemcandidate` (`AssemblePlain`) and `AGENTS.md`
invariant 4.

## Agent digest
- Claim: `corvint cem bundle --request FILE --out DIR` assembles a cem/1.0 stable bundle from a plain assembly request outside the repository, writing only DIR.
- Status: accepted (owner's standing CEM-V1 auto-accept, 2026-10-11 and 2026-10-20)/implemented
- Exists: the command in `internal/cemcandidate/bundle.go`, routed by `cmd/corvint/cem_gate_verdict.go`, its help in `cmd/corvint/help.go`, and the read-only disposable-clone end-to-end test in `internal/cemcandidate/bundle_test.go`.
- Blocked on: nothing; the reply on issue #1767 naming the route belongs to the orchestrator.
- Read next: Requirements; Failure modes; Acceptance evidence.

## Intent and scope

`cem-stable-plain-assembly/1` (`CEM-V1-240`) is the out-of-tree route to a `cem/1.0` stable bundle,
but until now it ran only through the unsupported `corvint-cem-candidate assemble-stable` binary, so
a reviewer with a base..target and no Tasks store had no supported public command (issue #1767).
This slice exposes that same assembly as `corvint cem bundle`. It adds no wire member, no verifier
rule and no new request profile; `CEM-V1` holds more than 256 requirements, so the command's rules
live in this companion spec.

## Requirements

- `CSB-V0-001`: (V1-2672; accepted under the owner's standing auto-accept (2026-10-11)) Command. `corvint cem bundle --request FILE --out DIR` takes exactly `--request` and
  `--out`, each once, each an absolute clean path, and no global option: the request names the
  repository, so `--root` or any other token before `cem` is refused `invalid-arguments` (exit 2),
  as are an unknown, repeated or value-less option and a relative or unclean path. The command is
  dispatched before root resolution, so it runs from any working directory. `corvint help cem` and
  `corvint cem bundle --help` name it.
- `CSB-V0-002`: (V1-2672; accepted under the owner's standing auto-accept (2026-10-11)) Request profile. FILE must be a `cem-stable-plain-assembly/1` request, in its
  single-runner or multi-runner form (`CEM-V1-240`, `CEM-V1-573..576`). Any other `profile`,
  including the Tasks-bound assembly, is refused `bundle-profile-unsupported`; bytes that are not a
  JSON object or do not decode as the request are refused `bundle-request-invalid` (exit 1).
- `CSB-V0-003`: (V1-2672; accepted under the owner's standing auto-accept (2026-10-11)) Output and isolation. The command runs `AssemblePlain` unchanged: it creates DIR
  (which must not exist and must lie outside the repository), writes `DIR/artifacts/` and
  `DIR/stable.json`, verifies the bundle with the stable verifier before writing the map, and never
  writes the repository, its object store, refs or notes, nor commits the map. Success prints one
  `{"ok":true,...}` line holding the `cem-stable-plain-assembly-result/1` result (`CEM-V1-341`) and a
  `nextActions` member naming `corvint cem verify-stable` over the bundle with the request's
  repository and revisions (`CEM-V1-350`); that next action accepts the bundle unchanged and yields
  the same `verdictSha256` for the same inputs.
- `CSB-V0-004`: (V1-2672; accepted under the owner's standing auto-accept (2026-10-11)) Path rules. Before assembly the command resolves the ancestor directories of FILE,
  DIR and every absolute clean request pin (source map, runner plans and receipts) to their real
  paths, and the request's repository path whole, so a macOS `$TMPDIR` path under `/var` is read as
  `/private/var`; the result and next action name the resolved paths. A final path element that is a
  symlink is still refused `candidate-input` by the no-follow reader. An ancestor that cannot be
  resolved is refused `bundle-path-unresolved`, naming the path as written. `corvint help cem` states
  these rules.
- `CSB-V0-005`: (V1-2672; accepted under the owner's standing auto-accept (2026-10-11)) Refusals. Every refusal writes one closed stderr line
  `{"code": C, "error": M, "ok": false}`, prints nothing on stdout, and exits 2 for an argument
  refusal and 1 otherwise. C is the typed code of the refusal (`invalid-arguments`,
  `bundle-path-unresolved`, `bundle-profile-unsupported`, `bundle-request-invalid`,
  `candidate-input`, or another assembly code). When the assembler's own stable verification does
  not accept the bundle, C is the verifier's refusal code, or `bundle-verification-rejected` for a
  `REJECT`, whose message names the verifier's issue codes. Any other untyped assembly error carries
  `bundle-refused`.

## Non-goals and simpler baseline

No anchor, verdict-ledger, note or commit step: the map stays out of tree and the caller decides
whether to commit it. No Tasks-bound assembly, no new request or result profile, no wire or frozen
verifier change, and no change to `corvint-cem-candidate`, which keeps its own route. The simpler
baseline, documenting `corvint-cem-candidate assemble-stable`, was rejected because that binary is
unsupported and absent from release help.

## Failure modes

- Late refusal leaves a partial DIR: a verifier or pin re-check refusal after the artifacts are
  written leaves DIR with `artifacts/` and no `stable.json`, as `CEM-V1-240` does; a caller reruns
  with a fresh DIR.
- An existing DIR, or one inside the repository, is refused by the assembler and nothing is written.
- A path that changes between resolution and reading is caught by the assembler's pin digests and
  re-check, never accepted on stale bytes.
- A symlinked final element anywhere is refused rather than followed.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CSB-V0-001 | `RunBundleInvocation` (`internal/cemcandidate/bundle.go`), dispatch in `cmd/corvint/cem_gate_verdict.go`, help in `cmd/corvint/help.go` and `cem_action_help.go` | `TestCSBV0001CEMBundleRoutesBeforeRootResolution`; `TestCSBV0002And005BundleRefusals`; `TestCEMV1340CEMActionHelpNamesOnlyThatAction` |
| CSB-V0-002 | `bundle` profile check | `TestCSBV0002And005BundleRefusals` (tasks profile, not JSON) |
| CSB-V0-003 | `bundle`, `AssemblePlain`, `PlainNextActions`, `EncodeOK` (`result.go`) | `TestCSBV0001To003BundleFromDisposableCloneVerifiesOutOfTree` (read-only clone, two bundles, one verdict digest, tamper refused, repository unchanged) |
| CSB-V0-004 | `resolveAncestors`, `PlainRequest.resolvePaths` | `TestCSBV0004BundlePathRules`; the end-to-end test reads every input through a symlinked ancestor |
| CSB-V0-005 | `refuseBundle`, `verifierRefusal` | `TestCSBV0002And005BundleRefusals`; `TestCSBV0005VerifierRefusalKeepsItsCode`; `TestCSBV0004BundlePathRules` (a swapped base and target is refused `bundle-verification-rejected` naming `base-revision-mismatch`); `TestCSBV0001CEMBundleRoutesBeforeRootResolution` |

## Rollout, rollback, compatibility

The command is additive. Rollback removes the `cem bundle` dispatch, `internal/cemcandidate/bundle.go`
and its help lines and regenerates `internal/capcatalog/catalog.json`; bundles already written stay
ordinary `cem/1.0` bundles that `cem verify-stable` reads unchanged.
