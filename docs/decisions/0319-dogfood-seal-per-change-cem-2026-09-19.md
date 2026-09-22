# Decision 0319 — Seal each change's CEM out of the shared tracked path

Date: 2026-09-19. Status: accepted (delegated call on the owner's request to stop the CEM file
conflicting between concurrent changes).

## Context

Every dogfood change commits its CEM at `.corvint/change.cem.json`, one tracked path. Any two open
branches therefore conflict on it, and each merge forces the other branch to rebase and rebind.
The path is also the frozen `cem/0.2` self-exclusion (`wire.ExcludedCEMPath`, CF-V0-006): `cem
prepare` refuses any other map path, and the OCM, LRF, local-completion, and frontier contracts
read it.

## Decision

The dogfood loop gains a last step, `make dogfood-seal BASE=<sha>` (`DOGFOOD-013`). It runs
`dogfood-check` and, on PASS, adds one commit that only renames the CEM to
`.corvint/changes/<bind-commit>.cem.json`. Binding and checking still happen at the fixed path, so
no wire contract changes. After the seal, main never carries the fixed path, and each branch adds
a file no other branch can name.

`dogfood-check` treats a seal commit as covered by its bind commit, and when BASE has no CEM it
takes the previous binding from the bind commit under the newest seal reachable from BASE
(`DOGFOOD-014`). A sealed HEAD is refused (`sealed-head`), and `dogfood-change` refuses a change
that adds a sealed CEM (`sealed-cem-in-change`); reworking a sealed branch starts by reverting or
dropping the seal commit.

## Alternatives weighed

- *A per-change map path in `cem/0.2`*: removes the conflict at the source, but changes a frozen
  wire literal that adopters, the frontier vectors, and five verifiers depend on. The conflict is a
  dogfood-workflow cost, so it does not justify a protocol version.
- *A `.gitattributes` merge driver*: GitHub ignores custom drivers, and `merge=union` corrupts JSON.
- *Binding after merge in CI*: loses the author-side review of the evidence before merge.
- *Git notes or an out-of-tree store*: the evidence stops travelling with the commit it binds.

## Consequences

- One transitional conflict remains for a branch opened before this decision whose bind edits the
  fixed path while main has deleted it; resolve by keeping the deletion and sealing.
- Local-completion's sidecar-only check reuse (`AllowCemSidecarOnlyReuse`) does not recognise a
  seal commit, so its checks rerun once after sealing.
- Squash or rebase merges drop the bind commit a sealed file is named after; merge commits keep it.

## Rollback

Remove `script/dogfood-seal.sh`, its Makefile target, `is_seal_commit` and `previous_binding` in
`script/dogfood-check.sh`, the `sealed-cem-in-change` guard in `script/dogfood-change.sh`, their
tests, `DOGFOOD-013` and `DOGFOOD-014`, and this record. Sealed files already on main stay as
inert history.
