## 2026-10-21 V1-1701: read-ahead review fixes (CAL-V0-380..381)

Lane pool-checkpoint, follow-up to `2026-10-20-v1-1701-walk-decode-prefetch.md`. An independent Codex
review of the read-ahead found three defects. Each is fixed, with a maintained witness that fails on
the unfixed code (checked by reintroducing each defect).

- **Speculative close errors could replace the refusal (P2).** A read ahead recorded its close error
  in the per-audit `nativeRead` at read time. When the walk refused at an earlier receipt (for example
  a forked `prev`), the audit's cleanup then turned that refusal into `UNSUPPORTED_FILESYSTEM`.
  `nativeRead.Read` is now `readHeld` (no bookkeeping) plus `settle` (close error, physical-read
  witness, read hook). Read-ahead holds the unsettled result, and step settles it on reaching the
  receipt. A failed read-ahead is discarded and read again in sequence. Witness:
  `TestV11701_ReadAheadCloseFailureSettlesInSequence`.
- **The byte budget was checked after the read (P2).** A window could hold about 1.4 MiB, and the spec
  did not say that decoded state is retained too. Each read-ahead is now bounded by the budget left
  before it buffers, so held receipt bytes never exceed 512 KiB. A receipt that does not fit ends the
  window, and one larger than the whole budget is read by step. CAL-V0-381 now states that decoded
  receipts and afterimages are in addition to the budget, in proportion to it. Witness:
  `TestV11701_ReadAheadHoldsAtMostTheBudget`, with the budget narrowed to 4 KiB.
- **Mixed inline and blob posts were validated twice (P3).** `prepare` validated the inline posts and
  then dropped them at the blob-backed one, so step validated them again. `prepare` now checks for any
  blob-backed post before validating any post. Witness: `TestV11701_MixedInlineAndBlobPostsValidateOnce`
  compares `validateRecord` call counts.

Cost: a window that ends on a receipt that does not fit opens that receipt once more, about once per
512 KiB of receipts. The 47,000-receipt measurement was not repeated, because the per-receipt work is
unchanged.
