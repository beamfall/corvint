# spc-w47 independent-review repairs (V1-4331, V1-4332)

The owner's 2026-11-02 fix-round-2 request supplies three independent findings against
`7b838086a735a090d95df0f40389ee8dc3e4804d`: POSIX shell-function assignment leakage,
capacity/OCM counting drift, and a stale passing receipt surviving static failure. The frozen
lane base remains `b486d3978728731c6638b50fa2210dfbb6e365ea`.

Invalid margin and BASE fixture assignments now run in subshells. Capacity and its immutable-base
ID subtraction mirror `internal/lrfrepo/ocm.go`: exactly three trailing digits, the bounded prefix,
and definition lines inside fences count, while fenced headings cannot delimit the section.
Inspection confirmed generation historically admits a broader digit form and excludes fenced
lines from its budget. That admission behavior is preserved in generation modes; the subsequent
Go OCM bound gate remains unchanged. SRG-V0-740/744 and their trace rows expose the distinction.

A static failure removes both the precheck receipt and its `.lane` companion before returning,
including the explicit static-only route. Successful static checks retain the existing receipt-reuse
path. ASD-V0-2010 and its trace row describe the invalidation. No requirement IDs or wire bounds
change. Rollback is the three script repairs and their regression/spec amendments together;
rolling back receipt invalidation reintroduces stale success and is not a safe mitigation.

`TestSpecCapacityCombinedLanes` compares OCM and capacity for 256 ordinary IDs plus a four-digit
ID, and 255 ordinary IDs plus two fenced IDs, including a fenced heading. It checks base-ID
subtraction and preservation of generation's historical count. Batch case 1d seeds a passing HEAD
receipt and `.lane` file, verifies the receipt, triggers static failure, then proves both files are
absent and `--verify-receipt HEAD` refuses, for normal and static-only invocations.

Verification commands, exact final outcomes and log paths are retained in `spc-w47.fix2.md`.
The supplied independent findings are review evidence; no second independent review is claimed.
The prior handback's read-only real Git index and owner prohibitions remain: no commit, push,
Tasks or codex command, background job or sleep/poll loop. Staged gates use private Git index/object
storage. Post-commit dogfood binding, sealing, frozen qualification, real-index integration and
native ticket completion remain NOT_PRODUCED/NOT_RUN for the integrating operator. These repairs
do not establish landed or native ticket completion.
