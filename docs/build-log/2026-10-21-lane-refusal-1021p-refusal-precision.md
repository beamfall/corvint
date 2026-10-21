# 2026-10-21 lane refusal-1021p: refusal precision (V1-1412, V1-2002, V1-1216)

## Problem

- V1-1412 (suspected): `context --expand` refused with an unrelated code when its 10-second
  source-read bound expired mid-expansion. Reproduced before any fix with
  `TestContextExpandDeadlineExpiryRefusesAsDeadlineExceeded`, which stalls each of the seven Git
  steps past a one-second bound for an `all` and an over-long range. 13 of 14 cases refused with
  `repository-root`, `invalid-handle`, `missing-handle` or `git-failed`. The 14th, an over-long
  range with a stalled recheck, correctly refused `invalid-handle` before the bound expired.
- V1-2002: `doccorpus.AdmitIndex` returned only the capacity refusal. The derivation's other
  `corpus-refused` refusals were swallowed: the record family bound, the key bound, a missing or
  duplicate identity, and the path fanout. So `docs corpus build`, `docs flows` and republish
  could write an artifact whose indexed reads refuse. Reproduced with a claim that reuses a subject
  identity: admission returned nil, and the query-time derivation refused `corpus-refused`.
- V1-1216: an index file's `index` mention token matched every loop `index`, `row.index` or
  `{ index: 0 }` in any script, so most index-file changes came out `unresolved-reference`.

## Decisions

- `ESV-V0-013`: any `--expand` refusal returned after the bound expired, or after the context was
  cancelled, is `deadline-exceeded`. That code is already the corpus MCP's per-call deadline
  vocabulary. A refusal decided before expiry keeps its code. `adapter source-view` is unchanged,
  because `TestSourceViewCancellationTerminatesGitProcessGroup` pins its `repository-root` on
  cancellation. Aligning it is a residual.
- `DCP-V1-244`: `AdmitIndex` returns every typed derivation refusal with the same code and
  message. The derivation reads only the artifact, so no refusal is left that it cannot pre-check.
- `ANL-V0-047`: only the index file's `index` token is skipped, and only in code position (left
  unblanked by the lexer) in a confidently lexed script. An identifier cannot be a module specifier,
  and `index` is never an import binding name. A directory-name identifier still refuses, because
  it can signal an aliased import. Comments, strings, templates, markup and doubtful lexes still
  refuse. The change can widen placement but never narrow it on an unprovable mention.
- No analyzer schema or contextindex snapshot change.

Requirements are accepted under the owner's standing auto-accept (2026-10-11).

## Evidence

- Each new test fails without its fix and passes with it:
  `TestContextExpandDeadlineExpiryRefusesAsDeadlineExceeded` (13 failures),
  `TestDCPV1244AdmitIndexRefusesEveryDerivationRefusal` (admission returned nil), and
  `TestANLV0047IndexIdentifiersDoNotHideTheIndexFileReference` (unresolved before the fix).
- Focused, affected-package and doc-gate results are in the PR body.

## NOT_RUN

- Tier-2 affected units and `make gate` are left for batch CI (AFP-V0-162).

## Rollback

Revert the lane commit. No stored format, wire schema or index encoding changes.
