# 2026-10-21 lane misc-1021m: local-completion base-ancestry guard

Tickets: V1-1757 (delivered in part; the ticket stays open for the remaining residuals).
`LCP-V0-036` is accepted under the owner's standing auto-accept of lane requirements (2026-10-11).
LCP-V0-034 and LCP-V0-035 are taken by open PRs #1453/#1460/#1464, so this lane uses 036.

## Problem

`corvint dogfood begin`, status/evaluate, `finish` and the aggregate finish paths decide whether
the plan base is an ancestor of `HEAD` with `git merge-base --is-ancestor` (`LCP-V0-002`). Git
honours `info/grafts` even with replace objects disabled, and a shallow boundary cuts parents, so
in such a checkout the probe's answer is rewritten history (residual of the EEP-V0-028 audit in
`2026-10-21-affected-1021d-ancestry-guard`).

## Decisions

- `requireBaseAncestor` refuses with the new code `ancestry-rewritten` before the probe when the
  common Git directory holds `info/grafts` or `shallow`, or cannot be resolved. It is a hard error
  at every call site (before any local write), not the `base-not-ancestor-of-target` unmet
  condition: the checkout made no claim about history.
- The first version called `gitancestry.Rewritten`, which spawns two `git rev-parse` processes;
  `TestEvaluateAgainstChangedBase` (LCP-V0-019 Git-process bound) caught 6 calls instead of 4. The
  delivered guard is `gitancestry.RewrittenAt(commonDir)`, a file-only twin that reads the same two
  paths from the already-resolved `gitauth` common directory. `repo.git` passes an explicit env
  allowlist, so `GIT_GRAFT_FILE`/`GIT_SHALLOW_FILE` cannot redirect Git away from those paths.
- CI's `fetch-depth: 2` job (ci.yml) runs static/vet checks only, not `dogfood begin`, so the
  shallow refusal does not affect it. A shallow adopter checkout now gets a named refusal instead
  of a silently rewritten answer.
- witness `resolveBaseTree` is not guarded: `corvint-witness-v0` already requires that a graft
  neither invent nor hide an ancestor, with a fixture test, so a blanket refusal would contradict it.

## Not delivered (lane triage)

- V1-2156, V1-2151: in flight in open PR #1457 (inside batch #1464).
- V1-2290: every gob encoder is in `internal/contextindex` (lane exclusion).
- V1-0313: fix lies in `internal/contextindex/history.go` and needs the unaccepted GPK-V0-072.
- V1-1478: fix lies in `internal/contextindex/registry_dialect.go` and needs an NGI-V0-008 decision.
- V1-2107: needs a sealed-byte schema rewrite of the CEM stable chain (const to enum for the runner
  receipt profile). That amends CEM-V1-091's closed re-emission set; decisions 0530 (approved) and
  0532 (declined) make it an owner fork. Proposed: a `cem-stable-regen --widen` mode limited to
  const-to-enum or enum-superset edits, status `SCHEMA_VALUE_SET_WIDENED`, re-binding via
  `sealChain`, new CEM-V1-37x requirements and a verify-output-in-schema test.
- V1-2291: putting git in the producer's process group conflicts with VPO-V0-010's own-group
  interruption sweep; Linux `Pdeathsig` covers one platform; VPO-V0-026 records crash cleanup as
  unqualified. Needs a containment decision.

## Evidence

- `TestAncestryRewrittenCheckoutRefusesBaseProbe` (grafts and shallow; begin, status, finish)
  fails with the guard disabled (`begin: <nil>, want ancestry-rewritten`) and passes with it.
- `TestRewrittenAtMatchesRewrittenWithoutGit_LCPV0036`: same reason as the Git-asking guard.
- Full `./internal/localcompletion`, `./internal/gitancestry`, `./internal/appflows`,
  `./internal/extevidence` pass; go vet and gofmt clean.

## NOT_RUN

`./cmd/corvint` (affected tier 1, not changed directly) and the 80 tier-2 units: batch CI.

## Rollback

Revert this change: drop the guard call in `requireBaseAncestor`, `RewrittenAt`, its test, and
the LCP-V0-036 rows.
