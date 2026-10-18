# V1-1681: CCE-V0-021 environment block for `corvint-test e2e prove` (serve argv part)

Spec: `docs/specs/change-compiled-e2e-v0.md` (CCE-V0-003 `environment`, CCE-V0-021). Authority: owner
direction of 2026-10-17 ("Build the CCE-V0-021 environment block (V1-1681 serve argv part) first").
Delivery stays experimental. CCE-V0-022 (env allowlist values, non-loopback abort, serve receipts) is
not in this slice.

## Decisions

- `prove` reads `environment` at the head revision and serves each side's staged tree with it; the
  proof records the environment digest, or `static` when the member is absent.
- A bare `serve[0]` is resolved once on Prove's PATH and handed to the BBF hook with that PATH
  (`serve_bin`, `serve_path`), because the hook runs with BBF's fixed PATH. A relative `serve[0]`
  names an executable inside the side's tree and refuses `serve-outside-tree` through a symlink out.
- `ready` needs a 2xx answer, the same rule as the provider's external-server check, so the two
  cannot disagree about readiness.
- The serve leader is waited on with `waitid(WNOWAIT)` and reaped only after its process group is
  killed, so the group ID cannot be reused while it is signalled (the `internal/procgroup` pattern).
- A serve that exits before or after `ready`, or during the run, reads `serve-exited`; every
  `environment-unavailable:*` reason reads INCONCLUSIVE.
- Declared names may not be `PATH`, `HOME`, the prove toolchain names or `CORVINT_BEHAVIOR_*`; only
  names that are set reach the control request, because BBF passes an unset key as an empty value.

## Independent review

Codex (`codex exec -s read-only`, one round) on 533e17dae returned eight findings, all confirmed
against source and fixed in the follow-up commit: group-ID reuse after reaping, a stray server
answering after serve exits, the provider's 2xx ready rule, a relative `serve[0]` escaping the tree,
reserved env names, the hook's fixed PATH changing which `serve[0]` runs, unset names entering the
control keys, and the port still held after stop (now a bounded wait for refusal).

The live run then caught a ninth defect that the unit tests missed: BBF refuses an empty definition
value, so `serve_bin` is omitted for a path `serve[0]`.

Recorded as a proof limitation, not detected: a serve that binds another port than `base_url` (for
example a dev server's fallback port) and a detached child that outlives the serve process group.

## Evidence

- Focused tests (`heavy.sh`, one package per command): `./internal/changee2e` ok, `./cmd/corvint-test`
  ok; `go vet` and `gofmt -l` clean.
- Live (Playwright 1.63.0, Node v22.23.2): `TestCCEV0021LiveEnvironmentDiscriminates` PASS (both sides
  and both BBF controls served through the declared argv; DISCRIMINATES with the environment digest),
  `TestCCEV0009LiveDiscriminates` and `TestCCEV0009LivePreserves` PASS with `environment: static`.

## Rollback

Revert the two commits. Without the member, `prove` behaves as before apart from the recorded
`environment: static` field.
