# Ratchet selection verifier repair 1 — V1-4729

The host verifier at `0f0a9af34a1e4e38cdec92cae0b11060e21f5855`
failed only `use-case-receipts-check`: the UC-CHANGE-CONSEQUENCE implementation
receipt still pinned the previous bytes of `cmd/corvint/affected.go`.

Run the existing `script/repin-use-case-receipts.sh` to satisfy UCV0-004 and
UCV0-005. It updates only the affected subject digest, the receipt's immutable
repository revision, and the ledger's digest of that receipt. Receipt fields,
claims, goldens, assertions and implementation behavior remain unchanged.
Rollback restores the two receipt/ledger files and restores the stale-pin failure.

`make -s use-case-receipts-check use-case-receipts-test` passes: all 18 receipts
are current and all 11 shell regression cases pass. Raw output is retained at
`/home/russell/lanes/ratchetsel-evidence/verifier-repair1-check.log`, SHA-256
`6b7a6c36c5c2bc225d0f8044eec0edc9448cc8fc87347b157db9a0f3954da1bf`.
No Go source, test, spec or line citation changed in this repair. The previous
whole-package results retain their limits; the failed CLI qualification remains
failed. Broad gates and independent review belong to the host verifier. Native
task writes, publication and sealing remain prohibited by the lane charter.
