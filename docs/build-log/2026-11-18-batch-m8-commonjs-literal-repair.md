<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4679: preserve literal CommonJS query and fragment filenames

Input revision: `ef925da6cf43063937581cbc65ce0dff71fc6531`.
Lane base: `9f4d5d89ab0648425db40e617f3d890598f9ff79`.
Scope: round 2 independent-review P1 only, AZU-V1-960 and AFP-V0-511.

## Finding and decision

Relative CommonJS reads lost literal filenames when a URL-style source alternative resolved.
`require('./settings#one.json')` could resolve `settings.cjs` speculatively and then lose the
original token. Query characters were also stripped earlier by the import scanner.

Keep the parsed loader kind and preserve relative CommonJS query/fragment literals before
normalization. Merge their foreign candidates after ordinary specifier pruning, alongside the
existing URL interpretation. The ordinary candidate alphabet now includes `?`, a literal Node
filename character. Unsupported syntax, failed probes and directory package manifests retain
the existing bounded-read frontier. No new interpretation of unsupported forms is accepted.
ESM continues through its existing resolution path.

`TestCommonJSLiteralPaths_AZU_V1_960` covers `#` and `?` with a resolving source alternative:
editing the literal JSON selects its CommonJS reader, ESM does not gain that literal read,
and editing the source alternative continues to select either reader. The question-mark rows
skip Windows because its filenames cannot contain that character. The existing template-depth
scanner assertion now also checks the new CommonJS kind; its previous expectations remain.
`TestCommonJSUnknownLiteralPath_AZU_V1_960` requires the exact path-token-bound frontier for an
unsupported literal form despite a resolving source alternative.

Rollback: revert this repair commit. That restores the reported false exclusion; it does not
justify marking the broader zero-missed-tests qualification complete. No wire fields, error
codes, diagnostics, exit codes, goldens or frozen outputs changed. The two supplied P2 findings
remain recorded and untouched: runtime-degradation bearer correspondence and the missing
trailer on `811ceae0a`.

## Verification

Go `go1.27.4`; Linux; lane TMPDIR mode 700; configured heavy locks without overrides.
Every test invocation used `script/heavy.sh`, one whole package, `-count=1 -timeout 30m`,
without exported commit identity. All 17 packages from `go list ./internal/liveverify/affected/...`
ran: 14 passed, two have no test files, and liverepo failed its existing latency gate.
The whole TypeScript package passed in 9.338 s, including existing tests and the new regressions.

`TestIncrementalSelectionMeetsTheLiveBudget` failed after all three built-in rounds: the
selected round had p95 176.964532 ms against 100 ms, calibrated p95 150.152918 ms and a
7.51-probe ratio against 5.00. Its graph has 452 units and only Go/Python plugins. This repair
changes neither its selector nor its measurement code. Host contention was observed, but the
cause of the budget miss is NOT_PROVEN. The failure remains a host qualification blocker;
no threshold, assertion or fixture was weakened and no passing performance claim is made.
The whole liverepo package took 279.915 s. No other test failure was reported.

TypeScript vet, Windows vet for both modules, changed-checks, and the requested spec,
citation, OpenSpec, long-lines and unbounded-readers checks passed. Specindex ran whole
through heavy.sh and passed in 1.781 s. No record-entry source digest became stale;
read sets remain unchanged. Citation repinning found no moved citations; 184 existing
untracked-basename citations were skipped. OpenSpec reported 65 baselined findings in 29
subjects; its commit rules OSC-V1-003/004 were NOT_RUN without a supplied OpenSpec base.
All added lines are at most 100 characters; gofmt and staged whitespace checks passed.

The installed planner refused with `self-binary-stale`. A fresh checkout binary built after
126.5 s of semaphore waiting and successfully planned against the full lane base before tests:
UNKNOWN, 429 selections (240 tests, 189 drivers), 61 unknowns (57 language frontiers, two
no-selectable-test and two unselected-binary-consumer entries). TypeScript and specindex
were selected. This is a planning receipt, not independent truth or a zero-missed-tests claim.

Aggregate verification is PARTIAL because the existing latency gate failed. Independent review,
full-suite truth and mutation qualification remain host-owned and NOT_PRODUCED here.
Whole-repository gates, new dogfood capture, publication, task writes and sealing were NOT_RUN.
The requested CEM bind/check workflow is retired under decision 0594 in `docs/DOGFOOD.md`;
no replacement workflow is introduced. The final closeout attempt and head are in the handback.
The supplied P2 findings remain open and unchanged.

Raw logs stay under `/home/russell/lanes/batch-m8-evidence/`. Stable evidence SHA-256 values:


- `round2-affected.json`:
  `060c8a2a990dcfe27a514b7a2217f5dabb43d45f76fadf000a3e629ba6775fd4`.

- `round2-test-typescript.log`:
  `e4f49bd7b49a52ecfe65d0f0e9b7cf9d92f5e96c39e91a82b1a10d8fad1c49ba`.

- `round2-test-liverepo.log`:
  `051e099351f3f4b7c1efa6daf77664a7ab9c4f189cdbf124bc4fc37b633c3dac`.

- `round2-required-checks.log`:
  `120168ec0d0538c429e6cd5165af53e59f8da93ef3c746b4a6d4d0b96a164a58`.

- `round2-windows-vet.log`:
  `6374c75c072249be7592105b4555dd19b3e325af0af8f426a400955ab03bb65a`.

- `round2-changed-checks.log`:
  `fba96926b6ff726d13fbe6087c0e7a70a69f0732909c82046d9010d9c0057594`.

Verified implementation/test SHA-256 values (under `internal/liveverify/affected/typescript/`):

- `typescript.go`:
  `1ded62cdb8b41085de899b3cd9b98d7c85fe9031440fec431c5636b3e41376ef`.

- `foreign_module_candidates.go`:
  `e0a4a2843aaa46bbea71f77b9c94bc4e9178bc2cef5f827702fc4d1c0ebd3944`.

- `commonjs_literal_path_test.go`:
  `ef97b8730a77278016e5be0032bbcdcd6e77a340d77ebbacd19574cdd632407d`.

- `typescript_test.go`:
  `7bcc483a6a6331cb892a79bfe4bbf8fa345d912e329c8ce7d1096999f5f1b271`.
