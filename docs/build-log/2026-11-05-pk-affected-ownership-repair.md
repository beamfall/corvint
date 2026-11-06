# Verifier repair 1: captured parity output ownership

The verifier at `a595e0e32f054601eb559f80b5d38ae5d172ba6d` found two new
unowned tracked paths in `TestOwnershipAuditRatchet_AZU_V0_002`:
`tools/str-parity/testdata/local-default.txt` and
`tools/str-parity/testdata/local-full.json`.

Declare exactly these captured local dry-run outputs in
`.corvint/non-source-paths.json` under AZU-V0-002, following the existing
captured help-snapshot declaration. They are output replayed by `TestMain`
for STR-V0-7313, never executable build inputs. The fixture bytes, frozen
contracts, ownership baseline and all existing test assertions are unchanged.
A worktree by-path plan still selects `tools/str-parity` for each fixture via
`ENCLOSING_PACKAGE`; the declaration does not remove its reader selection.
Rollback removes just these two declaration entries.

## Verification

With base `9db7a94169821d9b28701eaa88866aee2fec1083`, the checkout planner
selected both qualified packages; scope remains UNKNOWN with 83 unknowns.
The installed planner was also observed stale, so its selection is retained
separately and is not substituted for the checkout planner.

Both commands ran through `script/heavy.sh`, one complete package at a time,
with `GOTOOLCHAIN=local`, the private lane TMPDIR and no exported commit identity:

- `go test -count=1 -timeout 30m ./internal/liveverify/affected/`: PASS,
  117.079 package seconds, including the unchanged ownership ratchet.
  Log SHA256 `a3e64eae7af72e77fbba9bd6cc7108e39aaaffc5570b7bcb852a8821f8c61709`.
- `go test -count=1 -timeout 30m ./tools/str-parity/`: PASS,
  0.009 package seconds, including STR-V0-7313's captured-output consumer.
  Log SHA256 `ac37f08133f1473b34a7112a5869cdc08859889f669db7d3a0bf1722012d168a`.

Raw logs and plans are outside the repository in
`/home/russell/lanes/pk-affected-evidence/verifier-repair-1/`.
Darwin execution is NOT_RUN on this Linux host. Broader selection and
independent review remain with the host verifier. Reuse the existing pre-change
capture; the final binding/check outcome is recorded in the lane handback.
