<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4679: relative foreign readers and regular-file cache probes

Date: 2026-11-18
Input revision: `80b50bfdcfcedd9087ed0f3b35f661fd0407cdde`
Lane base: `55decc6a041a1df3f8ae43e95ba21cea669e7d80`
Status: both supplied self-review findings repaired; aggregate qualification remains PARTIAL.

## Findings and decision

P1, AZU-V1-960 / AFP-V0-511: a successful source lookup is not a complete runtime
read-set proof. Every local relative import now preserves foreign candidates before resolved
specifier pruning, independently of the source match. A Node test requiring `./settings`
retains `settings.json` even when source resolution finds `settings.cjs`.

The class is any source-resolution success suppressing foreign read alternatives or uncertainty.
The closed allowlist is unchanged ordinary repository-relative literal path syntax: ASCII
letters, digits, slash, space, dot, underscore, hyphen, at-sign and hash; suffixes do not limit
filenames. Only an absent candidate, a regular file, or a directory with an absent package
manifest certifies the exact, JSON-suffix and directory-index candidates. Unsupported path
syntax, other file kinds, failed metadata reads and any manifest form retain an unproven read
set. The new unknown-form regression resolves `settings%future.cjs` yet requires a bounded
read set and the exact `typescript:path-token-bound` frontier for an unrelated JSON path.
No new bad-form blacklist or manifest interpretation was added.

P2, AFP-V0-391/392 and AZU-V1-960: classify the candidate before probing its child manifest.
A regular file returns immediately, avoiding the `ENOTDIR` observation that permanently marks
cache recording uncacheable. Only a directory needs a manifest probe; other unproven kinds
remain uncertain. `TestForeignRegularFileGraphCache_AZU_V1_960` covers exact relative JSON
and alias-to-JSON imports, requiring cold storage, a warm hit, identical graph digest and
selection, and selection of the JSON reader. Temporary private cache directories isolate tests.

`TestResolvedRelativeForeignReaders_AZU_V1_960` covers P1 and its fail-closed unknown form.
The owning spec's traceability row names both regressions. No frozen wire fields, diagnostics,
exit codes, goldens or pre-existing test assertions changed. Rollback is to revert this repair
commit; that restores both reported defects and must retain the ticket's unresolved status.

## Verification and limits

Go version: `go1.27.4`. Tests used `script/heavy.sh`, configured locks without overrides, one
package per command, lane TMPDIR and no exported commit identity. Fresh-binary affected planning
against the full lane base selected TypeScript, the selector and specindex before testing.
The previous lane binary refused with `self-binary-stale`; rebuilding resolved that refusal.

The new regressions failed against the input implementation in all four cases: missing JSON
reader, missing unknown-form read bound, and exact/alias cache storage refused for
`unstampable path settings.json/package.json`. The first whole-package run after the fix failed
only the new unknown-form test's demand for a selected reader: the implementation correctly
retained the required path-token-bound frontier. The new test was corrected to require that
specific frontier; no pre-existing assertion was changed. Both failed evaluations are retained.

Final whole TypeScript package: PASS 7.867s. Whole selector package: PASS 13.121s.
`make -s spec-requirements-check line-citations-check`: PASS, with specindex through heavy.sh,
whole package PASS 1.721s. Citation repin found no moved citations and skipped 184 citations
whose basenames are not tracked at repository root. No second repin was needed.

Final-code affected planning: PASS; UNKNOWN, 365 selected (198 tests, 167 drivers), four
unknowns: Go build constraints, shell foreign invocation and two binary consumers. The receipt
predates only this evidence entry. It is not independent truth or accepted historical replay.

AZU-V0-009 final canonical replay, full-suite truth and changed-line mutation qualification
remain NOT_PRODUCED, delegated to the host. Independent review is orchestrator-owned.
Whole-repository gates and prohibited selection commands were NOT_RUN. Windows vet was
NOT_RUN (no push); Darwin qualification was NOT_RUN (Linux host). No zero-missed-tests claim.
CEM binding/checking remains unavailable: `docs/DOGFOOD.md` records removal under decision
0594. The exact final attempts and head are retained in the external handback; no replacement
workflow, seal, push, PR or task-store write is authorized in this round.

Raw logs and receipts live in `/home/russell/lanes/m8-frontier-evidence/`. Their SHA-256 values:

| Artifact | SHA-256 |
|---|---|
| `self-review-regressions-before.log` | `655c40924a317c3f6785a35f4f63c66e3b0f2842c56c06845154e94e815d9c12` |
| `self-review-typescript.log` | `d173123ac2660b1a4427781e7dbdbbfba8d5a4d16d56f73f436544e9c32793dc` |
| `self-review-typescript-final.log` | `52477af1270e99cea065ae35b1f671bda7bbedfa5bb2c4427d0f11fcdcc55b34` |
| `self-review-affected-package.log` | `c738699d32585f364a363b81b7c937d97dfe39730b5aee428bfbe43786e88f70` |
| `self-review-spec-citations.log` | `7a159e0ac35b16d82f149a45bd72072b33c54186b348e9868a04f18d4439e139` |
| `self-review-affected-final.json` | `30a82662271b8372ed5a6501b963b34382d5935bb8f43869c976fac09187cce2` |

Final implementation/test blob identities (SHA-256):

- `internal/liveverify/affected/typescript/typescript.go`: `d4ba61320819cd6be3ddfb585b22c3178a958cc82306ecbe3b5f8d3532d28c0b`.
- `internal/liveverify/affected/typescript/foreign_module_candidates.go`: `8169aa4d008c67624770413d473bd05daecc045eca2ffcb6bf8e8eefa639b3ce`.
- `internal/liveverify/affected/typescript/frontier_foreign_reads_test.go`: `c2ea4777b0d56c8ec95507da6876c833a6394be9bc7b52f27cecd078e222d785`.
