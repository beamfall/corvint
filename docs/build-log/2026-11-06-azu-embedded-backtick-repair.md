<!-- SPDX-License-Identifier: Apache-2.0 -->
# AZU embedded-backtick caller repair

Independent review of `0e2b21ad82fa4fd4fbd10e35eb339723e9a66f73` identified
an AZU-V0-004 / AZU-V0-8901 regression: excluding backticks inside the
formatted-directory matcher hid a quoted shell command substitution. The Go
caller ``exec.Command("sh", "-c", "run/`printf %s run`.sh").Run()`` executes
`run/run.sh` without containing that basename literally. With only the known
literal caller declared, the integrated plan incorrectly became BOUNDED with
no unknowns and excluded the undeclared caller's tests.

The repair permits embedded backticks within the existing conservative directory
pattern while retaining raw-string opening delimiters. It preserves existing
shell-expansion behavior and declaration admission, without inferring a precise
edge for the computed caller. The integrated
`TestConstructedDirectoryRetainsUnknown_AZU_V0_8901` now covers the reported
`sh -c` form and requires foreign-invocation UNKNOWN plus selection of the known
caller. `TestConstructedForeignCallers_AZU_V0_8901` also checks that listing this
caller admits it and omitting it rejects completeness. All prior assertions,
frozen contracts and goldens remain unchanged. Rollback must preserve both
uncertainty and verified caller selections; restoring the omission is unsafe.

Before-fix whole shell-package testing failed both new cases; after the repair,
the whole package passed (0.459 s). All Go tests use `script/heavy.sh`, one
package per command, without filters or exported Git identity. Raw evidence is
outside the repository at
`/home/russell/lanes/azu-new-evidence/independent-backticks/`:

| Log | SHA-256 |
|---|---|
| `shell-before.log` (rc 1) | `eac3265fc9ad39d2f305aa82043490e3f0b10d19a5f71c2cb3c09d80565ac831` |
| `shell.log` (rc 0) | `8004f8753617ee0f615a0fc5cdd26d3657b5f03d1b1927e9ff1fd4115111f773` |

Self-development route: `corvint affected --base` at the requested immutable
base selected the shell and affected integration packages (UNKNOWN, 186 selected,
86 unknowns). Installed-binary staleness remains explicit degradation. The
existing pre-change capture is reused; replacement capture, broad gates,
model-backed review and seal are excluded by the repair instructions. The host
orchestrator owns independent review. Retrieval/learning and historical replay
qualification are outside this matcher repair; aggregate AZU-V0 qualification
remains NOT_QUALIFIED. Final focused integration results, CEM rebind/check and
retained assurance limits are recorded in the lane handback.
