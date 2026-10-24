# 2026-10-24 — Site knowledge stage 8: offline acceptance pipeline (V1-1905)

## Decision

Stage 8 ships as `internal/skhproof`, reached only through `corvint-test e2e how-accept`
(`SKH-V0-163`..`176`). It applies the one `remove-guard-line` transform to a `COMPLETE` stage 7
draft, writes the candidate only by exclusive creation of a caller-named path, and assesses
recorded live-run evidence (`site-knowledge-proof-evidence/0`) and an accepted oracle
(`site-knowledge-accepted-oracle/0`) into one receipt (`site-knowledge-acceptance/0`). The six
conditions are checked in the `SKH-V0-081` order and the first failing one names one closed code.

## New encodings

- `site-knowledge-proof-evidence/0`: candidate SHA-256, application revision, infrastructure
  failure, repeats with retries, the TSA-V0 validity result, the NEA-V0 axes, step, selector and
  method bindings with witnesses, controls, and the CCE-V0-009 verdict with mutants. Decoded
  strictly (closed fields, 1 MiB, 256 entries per list).
- `site-knowledge-accepted-oracle/0`: origin, accepting commit, screen, locator, matcher, expected.
- `site-knowledge-acceptance/0`: status `PROVEN`, `EXECUTION_VERIFIED` or `BLOCKED`, the first
  failing code, the oracle code, the transform, conditions, blocked axes, unverified elements,
  verdict, mutants and controls. Canonical bytes with a final newline.

## Limits retained

- Live run NEEDS-HOST: the pinned tuple (Playwright 1.64.0, Node v24.11.1, headless shell 1248)
  is absent on the lane host (Node v22.23.3, no Playwright 1.64.0 in the worktree) and on its WSL2
  runner (Node v22.22.1, shell 1228). No evidence file is recorded; every shipped receipt reads
  `infrastructure` with `live-run-not-recorded`.
- Selector and method bindings read `binding-unverified` until the `SKH-V0-089` PWP-V0, RVN-V0 and
  AMSP-V0 witness amendments land.
- `proverAccepted` is false until the V1-1733 prover contract is accepted, so nothing reads
  `PROVEN`; tests flip it only to exercise the ladder.
- `SKH-V0-084` adopter qualification is NOT_RUN.

## Rollback

Delete `internal/skhproof`, `cmd/corvint-test/howaccept.go` and its test, and the `how-accept`
dispatch and usage line in `cmd/corvint-test/main.go`. No core path, store or index reads these
encodings.
