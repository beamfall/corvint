<!-- SPDX-License-Identifier: Apache-2.0 -->
# Self-review repair: literal source resolution and runtime frontier bearers

Owner: Russell Lewis. Date: 2026-11-18. Tickets: V1-4679 and V1-5087.
Requirements: AZU-V1-960, AFP-V0-511, CIN-V1-941/942 and TSP-V0-942.

The review found that literal CommonJS data candidates did not account for extensionless
source files or directory source indexes. A stripped source alternative could therefore
certify an incomplete read set. The soundness class is incomplete loader resolution being
mistaken for a complete negative proof, not a particular delimiter or filename suffix.

The literal CommonJS proof now has a closed allowlist: an ordinary repository-relative path
accepted by the existing path validator, with the exact lowercase `.json` extension, whose
literal target exists as a regular file. Everything else retains the existing path-token-bound
frontier and UNKNOWN scope. This includes missing files, extensionless source loads,
directories, manifest redirects, unknown extensions and unsupported path syntax. This decision
uses the review's permitted uncertainty outcome instead of claiming complete Node resolution.
The existing stripped source alternative and ESM interpretation remain unchanged. Non-goals
are a new module resolver, a new frontier reason or changes to wire shapes and diagnostics.

`TestCommonJSLiteralSourceAlternatives_AZU_V1_960` covers both `#` and `?`, extensionless `.js`
loads, directory `index.js` loads and a regular `.future` file. Each fixture also resolves
`settings.cjs` through the stripped specifier. Changes to the literal target and an unknown
input retain the exact frontier and UNKNOWN scope. Existing exact-JSON, ESM and unsupported
syntax tests remain intact. The initial version of this new test incorrectly required
immediate selection as well as uncertainty; the first whole-package run exposed that mismatch.
The final new assertion tests the permitted uncertainty outcome. No pre-existing assertion
was weakened, removed or changed.

Runtime degradation previously appended a LANGUAGE_FRONTIER after bearer computation. It now
joins frontier construction with its reached test holders, their actual witness kind and dirty
path. The existing bearer sorter, deduplicator and 16-unit cap apply. The runtime-widening test
requires correspondence in both directions and checks the holder and witness on each runtime
bearer. The existing text renderer consumes these ordinary bearers without a format change.
No frozen output, golden, error code or stderr string changes.

The installed planner refused with `self-binary-stale`. A fresh checkout build selected both
touched packages and specindex against base `9f4d5d89ab0648425db40e617f3d890598f9ff79`:
429 selections, 61 unknowns, scope UNKNOWN. This is selection evidence, not full-suite truth.
Raw evidence and final results are retained under `/home/russell/lanes/batch-m8-evidence/`;
the handback records commands, digests and limits. The orchestrator owns independent review
and repository-wide validation. Previously recorded live-budget and CLI environment failures
are not cleared by this focused repair.

Rollback is to revert this repair: that restores the two reviewed gaps. It requires no state
migration. The requested CEM bind/check remains unavailable under decision 0594, as documented
in `docs/DOGFOOD.md`; no replacement capture, seal or task-store write is introduced.

Final focused verification: whole affected PASS (11.245 s), whole TypeScript PASS (6.656 s),
and spec requirements plus line citations PASS, including whole specindex (1.034 s). Tests
ran through heavy.sh with go1.27.4, no identity exports and one package per command. The
TypeScript and spec checks queued for 235.2 s and 192.2 s respectively; no semaphore override
was used. The repinner moved one citation and its automatic second pass converged. Its 184
pre-existing untracked-basename skips remain visible. No broader validation was run.

Raw evidence SHA-256 digests:

- `round3-affected.json`:
  `cd717061e63e68bd81a409b7da8fc33bacddf605baf7a77c08f2ac990a787bfa`
- `round3-typescript-test.log`:
  `17333453c15fbadd43a54341e7393558445b38982dd4e854a1b84ccd4e6cda4d`
- `round3-typescript-test-final.log`:
  `4547ed8c5408e6e106e2dcfda4c3ad912f600488c681d865c5e09b7da441fef8`
- `round3-affected-test.log`:
  `280374fed6f50db5344d2f3ab1a4e26030b75b0f192c4ce1e110e10389d51970`
- `round3-spec-citations.log`:
  `5f83ae3cfc7ead63cedcdf5708c446dc4501932c3b26809790776984a68d33d0`
- `round3-repin-final.log`:
  `4a6e8563701007f48f4a6d4b993a43c0342b4cecb38a252c080317aa623897d1`
