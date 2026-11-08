# Python terminal results and write-only open dependencies

Scope: the two findings in the rspt11py self-review repair, against
`8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`. Owning intent remains
`docs/specs/runner-selection-plugins-v0.md`: RSP-V0-003, RSP-V0-007,
RSP-V0-021 and RSP-V0-022. No requirement, wire shape, error code, golden,
existing assertion, registry activation or qualification status changes.

## Decisions and regression coverage

Successful opens observe prior existence and access even when the caller never
reads file contents. The Linux collector now retains that dependency for every
supported non-exclusive open, including write-only, append, truncate and create
without exclusivity. Existing proven own-creation suppression remains governed
by RSP-V0-007. The class is all successful opens, not a list of problematic
access modes. The existing closed `supportedOpenFlags` mask and combination
checks remain the only admitted forms: ordinary access modes with known flag
bits, and the separately handled O_PATH form. Unknown bits or unsupported
combinations mark components incomplete and retain the dependency; only a
supported exclusive creation establishes creation provenance.

`TestRSPV0021WriteOnlyOpenClassification` checks dependency retention, exclusive
creation, and an unknown high flag bit directly. All six cases pass.
`TestRSPV0021WriteOnlyOpenDependency` executes the native fixture without a read
or write syscall, checks successful open dependencies, then checks that deletion
changes non-creating opens to a missing lookup. Its unknown-bit case requires
incomplete evidence. This live test remains blocked by the sandbox below.

Unittest callbacks aggregate to exactly one parent row. The callback allowlist
is the active test object and the exact native `unittest.case._SubTest` type
whose `test_case` is that active object. Display-name parsing and duck-typed
parent attributes confer no identity. Unknown forms withhold terminal rows so
the existing reconciliation fails closed, including after FULL retry. Failure
outranks skip, skip outranks pass, and all reported skip reasons survive in
callback order, including when the parent failed. No terminal pass is invented
at `stopTest`.

`TestRSPV0003UnittestAdapterSubtests` executes the embedded adapter via its
actual command, inventories a temporary module, and feeds captured stdout to
the existing selection session. Seven cases cover the reported skip, nested
subtests, mixed pass/skip with multiple reasons, failure before and after skip,
successful subtests, and an unknown object impersonating a parent. It verifies
native process outcomes, exact rows, and the unknown form's failed FULL retry.

## Observed verification and limits

The required affected plan selects both touched packages. Its installed binary
is stale relative to inherited source changes: `self-binary-stale`. Scope is
UNKNOWN, selected=537, excluded=396, unknown=99 (77 LANGUAGE_FRONTIER,
2 NO_SELECTABLE_TEST, 20 UNOWNED_DIRTY_PATH). Only the repair's two packages
were run, each as a whole package through `script/heavy.sh` without a filter.

- Registry: PASS, 0.885 seconds, including existing tests and adapter execution.
- Proctrace: FAIL, 3.897 seconds. JSON terminal events: 40 pass, 44 fail,
  including subtests and the package result. The collector regression passes;
  live tracing fails with unresolved descriptor/dirfd observations. A separate
  reproduction starts `/bin/cat` with its input pipe open: `/proc/<child>/cwd`,
  `/proc/<child>/fd/0`, and `/proc/<child>/fdinfo/0` each return ENOENT while the
  child is running. This sandbox cannot supply the process metadata the Linux
  backend requires. Host execution is required; no assertion was weakened.
- `script/check-line-citations.sh --repin`: PASS; no moved citations found.
- `make -s spec-requirements-check line-citations-check`: PASS; existing 180
  skipped basename citations remain visible. Its existing recipes also ran
  specindex and lrfrepo checks through heavy.sh.

Raw evidence stays in `/home/agent/lanes/rspt11py-evidence/`. SHA-256 digests:

| File | Bytes | SHA-256 |
| --- | ---: | --- |
| registry-repair.log | 506 | `4d66a10c2775e8e676bcd7c15b8416862b23f140429e2f9b704d43303b6181bc` |
| proctrace-repair.log | 129181 | `fefb3f2edc4b5e35d2dfb32e6e5359bcd15cc683accb9238b12f4667b3ef5f51` |
| repin-repair.log | 190 | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
| citations-check-repair.log | 1525 | `4808341be53d4db93ea2c87174bc7d977f80f4d6be2e13ba638122c78a1c0804` |

Independent review, host live qualification and repository-wide gates remain
owned by the orchestrator. No Tasks writes, publication or seal is authorized.
The earlier V1-5113 legacy activation blocker remains unchanged. Final binding
and check outcomes belong in the external lane handback after the repair commit.
Rollback is a revert of this repair's code and new fixtures; it restores the
known dependency/reconciliation gaps and does not establish qualification.
