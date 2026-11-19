<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4679: restore source-only frontier proof

Owner: Russell Lewis. Date: 2026-11-18. Repair input: `811ceae0ac9fb3a55a97c995909976622f7324c7`.
Requirements: AFP-V0-511, AZU-V1-960, AZU-V1-961. CIN-V1-941 remains unchanged.

## Finding and decision

The foreign-read repair made every source-bound entry call `entryNamesAny`. That helper
conservatively returns true without located sites. Python dynamic imports, .NET unparsed
source and Swift conditional compilation report bare reasons, so unrelated unowned files
incorrectly acquired their source-bound unknowns. These plugins' resolution is confined to
owned source; their proof does not depend on site records.

Preserve both contracts with optional `SourceOnlyFrontiers`, a static plugin declaration.
Only a bare entry whose reason is also in the validated source-bound declaration uses this
proof. Python, .NET and Swift declare it for their existing source-bound reasons. Located,
overflow, unowned-site and bounded-token entries still take the foreign-read proof. Dirty
resolution inputs still retain every reason. An undeclared reason cannot use the new proof.
The graph encoding and digest inputs, JSON shapes, diagnostics and frozen goldens are unchanged.

This distinction avoids making incomplete foreign-read observations look complete. The existing
AZU-V1-960 bare-entry test still retains uncertainty when the stricter declaration is absent.
The owning spec explains this distinction under its existing requirement; no new requirement
or broader narrowing claim is introduced. Revert this repair to roll back the declaration;
that restores the three reported AFP-V0-511 regressions while leaving foreign-read handling.

## Evidence

`TestSourceOnlyUnownedFrontiers_AFPV0511` adds eight cases: bare source-only proof, absent
source-bound classification, dirty resolution input, located read, unowned site, unknown site
form, overflow and bounded tokens. Existing assertions remain intact.

Whole-package tests through `script/heavy.sh`, Go 1.27.4, `-count=1 -timeout 30m`:
selector PASS (10.329 s), Python PASS (0.191 s), .NET PASS (0.022 s), Swift PASS (0.055 s),
TypeScript PASS (5.503 s). The selector package includes AZU-V1-960/961 and CIN-V1-941 tests.
Raw logs remain in `/home/russell/lanes/batch-m8-evidence/`.

| Log | SHA-256 |
|---|---|
| `test-affected.log` | `78cf039bb94310521f967a3cffc03e903bdcd70c31789449d013eae78d363095` |
| `test-python.log` | `37aee0c0c21169389d9ca08d898e5cad0e52127b99f17d5d6ee24600da0eacdc` |
| `test-dotnet.log` | `d40c2ebf00d2a48369ce53d0b1efe418c7353901e1578c8fd5710f86113eeec1` |
| `test-swift.log` | `635031644f16ce88afa38897230428f267020e4b7312d62cb6809e4aa96922d1` |
| `test-typescript.log` | `c8521488c65ad50187c18e3c1820043e07919dd9a82069c411aeae3b8a4c1990` |

All 17 packages enumerated by `go list ./internal/liveverify/affected/...` completed with
exit 0 after the repository declaration repairs recorded in the companion entries; two have
no test files. Whole-package existing tests were run without a name filter. The original
live-repository failure is retained there. Touched-package vet, Windows vet, spec requirements,
line citations, OpenSpec and long-lines checks pass. OpenSpec retains its 65 baselined findings;
line citations retain 184 skipped untracked basenames. No new ignored finding is introduced.

The affected plan retains UNKNOWN scope; it is no whole-repository qualification. Independent
review and hosted verification remain with the orchestrator. Dogfood is retired by decision 0594.
