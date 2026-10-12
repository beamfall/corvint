# V1-0632 C13..C17: stable doc corpus, witness, console, capsule and dashboard routes

Date: 2026-10-12. Base: `85668d6c3d786ff46365f3b1642ae2de06304f8a`. Branch:
`claude/v1-0632-projections`. Scope: migration rows C13 to C17 of
`protocol/cem-1.0/stable/MIGRATION.json` under decision 0525, with requirements
`CEM-V1-160` to `CEM-V1-168` in their own section of `docs/specs/cem-stable-v1.md`
(`CEM-V1-140..159` belong to the parallel V1-1276 lane). No ticket-store writes or PRs.

## Decisions

- Each command exposes the existing library-level stable projection only through an explicit
  stable input. No default changes and no 0.x document is relabelled:
  - C13: `corvint docs corpus cem-stable` requires `--artifact`, `--cem`, `--expected-base`,
    `--target` and `--stable-artifact-root`, and emits `corvint-corpus-cem-stable/1` only on a
    verifier `ACCEPT`. Every other outcome exits 2 with `corpus-stable-refused` and no stdout.
    The 0.x `cem` operation is unchanged and still refuses a cem/1.0 map.
  - C14: `corvint witness --stable-artifact-root DIR` passes the root to `witness.Compile`.
    The command then admits only a `BOUND` source under `corvint-witness-stable/0`.
  - C16: `corvint-console -stable-artifact-root DIR` turns on verified chains. A stable change
    is drawn only when the URL names `base=<full commit>` and `verify.Stable` returns `ACCEPT`
    for that base, the change as target, and the root. The verifier's own envelope, sidecar,
    `changeIntegrity` and `referenceIntegrity` results then replace the display-only gaps.
- Fail-closed fork (C14): a committed cem/1.0 map without the flag, the flag over a non-stable or
  absent map, and any non-`BOUND` stable source exit 2 with `stable-witness-refused`. Before this
  change, a stable map without the flag gave an exit-0 `UNSUPPORTED` report.
- Fail-closed fork (C14): collapse stays library-level. No collapse command exists, and
  `correlated-witness-collapse-x0.md` lists a CLI as a non-goal. No witness, corpus or console
  MCP tools exist either, so none are added.
- Fail-closed fork (C16):
  - verified mode refuses a change whose URL carries no `base=`, including a click from the
    listing, which carries none;
  - one artifact root serves every change;
  - a verifier timeout under `-timeout` refuses.
- Fail-closed fork (C15): the capsule consumer holds a capsule, not a repository, so it cannot
  run the repository verifier. A cem/1.0 enrollment keeps refusing at the capsule CEM base
  check, and a test now pins that refusal. Plan:
  1. an enrollment format carrying the stable artifact roles;
  2. a capsule-native stable verifier over capsule objects;
  3. a protected-local-execution spec amendment.
- Fail-closed fork (C17): the dashboard registry is hash-pinned and the bundle adapter is
  `NOT_STARTED` (it reads no bytes). cem/1.0 profiles are refused as
  `DASHBOARD_INVALID_ARGUMENT`, and a test pins that. The `VerifierCEMV01/V02` constants in
  `internal/dashboard/source/types.go` stay until V1-1278. Plan:
  1. a new registry version with a `cem-ocm-bundle-stable-v0` registration;
  2. verification over the C05 stable universe;
  3. snapshot fields for axes, bindings and input hash;
  4. new conformance vectors.

## Evidence

- `TestCEMV1160WitnessStableRoute`, `TestCEMV1162CorpusStableOperation` and
  `TestCEMV1164ConsoleVerifiedStableChain` (C13, C14, C16) build real fixtures with
  `internal/cem/stablefixture`. Each shows acceptance, and refusal of each of these:
  - a wrong base;
  - edited map bytes;
  - a tampered artifact;
  - a missing artifact root.
- The console test also covers:
  - a missing and a malformed base;
  - a verifier timeout;
  - following a rendered hunk link, which keeps the accepted base;
  - reference rows that show the verifier's `referenceIntegrity` value.
- C15 and C17 deliver refusals only, so they have no acceptance case:
  - `TestCEMV1167CapsuleRefusesStableEnrollment` shows that an exact stablefixture cem/1.0 map
    is refused at the capsule CEM base check.
  - `TestCEMV1168DashboardRefusesStableBundle` shows that minimal arguments naming cem/1.0
    profiles are refused, and that no registered adapter profile names cem/1.0.
- The 0.x routes and their suites are unchanged.
- Corvint use: `corvint affected --base 85668d6c3` (Corvint 1.0.0-rc.3 build 456) selected 80
  units with scope `UNKNOWN`, whose unknowns are `LANGUAGE_FRONTIER` items.

## Review

- Codex (`gpt-6.1-sol`, read-only) round 1: FAIL with four P2 findings and one P3:
  - Verified hunk links dropped the base. Fixed: `Chain.VerifiedBase` is set on `ACCEPT`, and
    hunk links carry it.
  - The missing- or malformed-base refusal names no verifier outcome. Fixed by amending
    `CEM-V1-165`: it is an argument refusal raised before the verifier runs.
  - The timeout was untested. Fixed: an explicit deadline check refuses, and a test covers it.
  - This log overclaimed C15/C17 acceptance. Fixed: C15 and C17 are now described separately.
  - P3: accepted reference rows kept "not checked" text. Fixed: they show the verifier's
    `referenceIntegrity` value.
- Round 2: PASS, with one P3 (the spec rollback said four tests, not five), which is fixed.
- `CEM-V1-160..168` are accepted under the owner's standing auto-accept (2026-10-11).

## Limits and NOT_RUN

- Collapse has no command route. C15 and C17 deliver refusals and plans, not stable consumers.
- The console listing does not yet carry `base=`, so verified chains are reachable only by URL.
- Not run:
  - Linux, Windows and other runtime tuples;
  - affected units outside the focused set;
  - `make gate`.

## Rollback

- Delete the five new tests and `VerifyStableChain`.
- Restore:
  - `cmd/corvint/witness.go`, `docs_corpus.go`, `docs_corpus_help.go` and the witness help text;
  - `internal/console/server.go`;
  - `cmd/corvint-console/main.go`.
- Remove the `CEM-V1-160..168` section.

No 0.x profile, default, packet or stored evidence changes.
