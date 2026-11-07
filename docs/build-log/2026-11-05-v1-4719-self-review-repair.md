# V1-4719: generic Git fixture runner repair

Self-review found that the generic program runner in `cmd/corvint/record_fresh_test.go`
passed an unfiltered `testEnvironment` to Git. The direct-command source guard cannot
recognize that wrapper, so exported identity still overrides repository configuration
and an invalid exported date prevents its initial commit. This is a coverage gap in
CCF-V1-9800, not a change to the accepted fixture or product contracts.

Decision: filter the runner's Git environment with the existing shared identity
filter, preserving its configuration isolation and the environment for CLI invocations.
Exercise the actual wrapper through the existing fresh-repository test in both main
and linked worktrees, with foreign names/emails and invalid author/committer dates.
Assert both recorded identities against the existing repository configuration. This
runtime regression covers the wrapper without claiming interprocedural source analysis.
All existing assertions and frozen artifacts remain unchanged.

The existing pre-change capture is reused. Independent review belongs to the
orchestrator; no model-backed reviewer, publication, Tasks write or seal is authorized.
Raw repair evidence is retained outside the repository in
`/home/russell/lanes/fixtureident-evidence/`.

The regression failed before the runner fix in both repository variants with Git's
`invalid date format: invalid-fixture-date` (package 7.983s, exit 1). After the fix,
the focused wrapper/fixture/context/LSP/core-profile run passed (package 5.487s,
exit 0). The source guard and SHA/golden regression were included in that run.
Formatting, diff whitespace and line-citation checks passed without a repin change.

Affected selection (installed binary): 183 selected units, 461 excluded, scope
UNKNOWN for Go build-constraint variants and shell foreign invocations. The binary
reported self-binary-stale; the selected `cmd/corvint` unit covers this repair.

| Repair evidence | Bytes | SHA-256 |
| --- | ---: | --- |
| `repair-affected.json` | 97377 | `ac865c83e6704ffc2abf0314d8b7e5b05fa01dec2fc24b76fdbd9ca025548416` |
| `repair-red.log` | 1849 | `be20bd14998d4ded8ad1c2f3342783718b2a9bb138f2967d2a63cce1aa4f4a8a` |
| `repair-focused.log` | 1188 | `b08e1f6bbe4370d86655b8356842fc6f60d40e73196f1eaca3068070228e9ea3` |
| `repair-whole.log` | 66824 | `18e8ffc91c3e8376083e912d694a98de85c72c57ce464518aebeed07b471fe10` |

The required whole `cmd/corvint` suite ran through `script/heavy.sh`, without a
`-run` filter, and returned exit 1 after 510.590s with 46 top-level failures.
The repaired fresh-repository regression was not among them. The run again includes
`TestConvertedRefusalDiagnostics` and `TestWorkMaterializationNoCallerGitWrites`,
whose temp-ancestry/source-qualification blockers were reproduced at the unchanged
base in the original lane evidence. The mandated TMPDIR remains under a Git
repository with a group-writable ancestor. Other failures remain retained unknowns;
this repair does not qualify them or alter any frozen assertion to suppress them.
V1-4719 remains PARTIAL pending host-side whole-package qualification. Local outcome
remains blocked. The focused repair itself is complete.

Friction: the pre-fix run queued 120s for host load plus 196.6s for a shared slot;
the focused passing run queued 120s plus 18.0s; the whole run queued 52.1s for a
slot. The wrapper warned that /tmp had 2 GiB free against a 3 GiB floor. No
semaphore override was used. Line-citation checks retained their existing notice
about 180 basename-only citations skipped at repository root.

Windows vet passed for both modules through `script/heavy.sh make windows-vet`
(exit 0). `repair-windows-vet.log`: 1339 bytes, SHA-256
`3a97cbc0fb72e28a52445e0f7d3147b32de1cd1767caa966a7e6b8d2b40a4e2b`.
