<!-- SPDX-License-Identifier: Apache-2.0 -->
# AZU constructed caller and Git path repair

Self-review of the azu-new lane at `6e15556c1f34b04081a7609467fc7c583763483b`
found two gaps in the existing AZU-V0-004/8901 and AZU-V0-009/8903 contracts.
No declaration, receipt, diagnostic, golden or exit contract changes are needed.

The directory completeness guard now includes Go raw-string formats and a
directory literal concatenated separately from its separator. The integrated
`TestConstructedDirectoryRetainsUnknown_AZU_V0_8901` demonstrated that both
``fmt.Sprintf(`run/%s.sh`, name)`` and `"run" + "/" + name + ".sh"` previously
produced BOUNDED plans with the undeclared caller excluded. With the repair,
both retain foreign-invocation uncertainty and the known caller's tests.
The existing completeness table also verifies admission when each caller is listed.

Replay now decodes Git's C-quoted old/new file headers and treats a literal tab
as a header delimiter, preserving spaces within paths. `/dev/null` supplies no
coordinates. `TestChangedLineGitPaths_AZU_V0_8903` creates actual Git diffs for
Unicode, space and tab filenames, each modified and deleted, checks exact
coordinates and admits the correctly bound mutation receipt. Header-like content
inside hunks cannot replace a filename. All six cases failed before the repair.
The tab fixture is unavailable on Windows, where that filename is invalid.

Whole shell and replay package tests passed through `script/heavy.sh`, without
test filters or exported Git identity. Logs remain outside the repository under
`/home/russell/lanes/azu-new-evidence/`, with prefix `self-review-repair-2-`:

| Log | SHA-256 |
|---|---|
| `shell-before.log` (rc 1) | `6ad96f9f2c9702a7368b6f5403e090cb61cc37a8e04f0b1a6a0c9cd18794093b` |
| `replay-before.log` (rc 1) | `61ccfafb2af450735133ea914f45a85cd23a2b60cd9ebcb73f2df1f8ccbfb484` |
| `shell.log` (rc 0, 0.469 s) | `41177e1e4bea9d8837ee6f1a8e84c7f857668571c13be142461f31a80feb3922` |
| `replay.log` (rc 0, 0.286 s) | `973df93f273778576df03adec7ee513762fc588d52323a8502fd8b36670eddd5` |

The existing whole affected integration package also passed (rc 0, 101.336 s);
`affected-tests.log` SHA-256 is
`a2d027cbfec4080ff69b36e3bd16aec27ac3d6774bf11a8865baa7dbbf7de08a`.

Self-development routes used: `affected --base` selected both repaired packages
and the affected integration package (UNKNOWN scope, 186 selected, 86 unknowns).
The installed binary's staleness remains a degradation. The existing pre-change
capture is reused; no replacement capture is claimed. Final integration tests,+CEM rebind/check and their explicit local outcome are retained in the handback.
Retrieval/learning evaluations and provider qualification are not applicable to
these two parser/guard repairs. Broad qualification and independent review belong
to the host orchestrator. Historical full-suite/mutation qualification remains
NOT_PRODUCED; synthetic receipt fixtures do not qualify AZU-V0-009.

Rollback must retain UNKNOWN for unsupported caller constructions and leave
replay unqualified when path coordinates cannot be established. Reverting to the
unsafe caller omission or changing frozen outputs is not an acceptable repair.
