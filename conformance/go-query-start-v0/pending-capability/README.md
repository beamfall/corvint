# Pending-capability oracle vectors

These are **historical Python oracle captures made before the Go candidate implemented the named
query profiles**. They remain standalone development vectors rather than replayable CLI parity
evidence; `conformance/cli-parity-v0` now carries the oracle-captured GPK-V0-043 command evidence.

## Why capture them before Go implements the capability

The migration plan requires that a parity corpus's expected bytes never come from candidate code.
Capturing the oracle's output *before* the Go path exists makes candidate self-certification
structurally impossible: when Go later gains the capability, it is compared against bytes that
predate it.

## Vectors

### `oracle-budgeted-omission.json`

- Command: `corvint --root . query --task "bound json parsing and index" --limit 8 --budget-bytes 1500`
- Captured against this repository at commit `06a6a0988f5178a8c1ae902bdad6df71c74c858c`
  (receipt `revision` records the *tree* identity `19835316e5c3d93483f493ab33c354289cdff576`,
  not the commit).
- Oracle result: `state` `BUDGETED`, `requested_results` 4, `included_results` 1,
  `omitted_results` 3.
- **Captured blocker:** `corvint query` rejected the option outright —
  `"--budget-bytes is not implemented for native Go authority-start query"`. W3 uses this capture
  as generic packet-compiler evidence; exact command replay remains pending W4's general-query port.
- **Blocker superseded 2026-08-29 (W3 landed).** `corvint query` accepts `--budget-bytes` and
  compiles the packet natively; `TestPacketBudgetMatchesFrozenPythonOracleVector` replays this
  capture byte-for-byte. **Exact standalone repository-query budget parity landed 2026-08-31 under
  GPK-V0-043** as `query-repository-budget-selection`; GPK-V0-028 remains the unchanged explicit
  limit-1 authority-start profile.
- **What it proved:** `omitted_results` is produced by packet-budget selection, not by `--limit`
  truncation. `internal/contextindex/receipt.go` emitted `"omitted_results": 0` and
  `"critical_missing": []` as constants — consistent with a Go kernel that has no budget selection,
  and live defects the moment budget selection landed. `setCoverage` now derives both. This vector
  was the demonstrated-red for that transition.

### `oracle-budget-boundary-sweep.json`

- Compiled by the Python oracle from `internal/contextindex/testdata/oracle-budgeted-omission-input.json`,
  the pre-budget receipt the vector above was compiled from. Every expectation in it is oracle
  output; none of it is produced by the Go packet compiler it certifies.
- `vectors` holds one compiled receipt per budget at which the oracle's selection changes, each
  paired with the byte below it. `projectFiles` records the source inventory the sweep was compiled
  against — behaviourally complete for this input, because the capture revision's tree carries no
  record ledgers and no root `package.json`.
- `envelope_floor` brackets the smallest budget that can hold the envelope at all: one byte below it
  the oracle raises, and the admitted budget carries the receipt it returns instead.
- `absent_inventory` is the same input compiled with **no** inventory. The oracle reads an absent
  inventory as an unknown project rather than a signal-free one
  (`src/context_corvint_learning.py:196`), so it closes the plan with a different gate and the packet
  is measured against a different size. Go elected the signal-free fallback there and diverged live
  on `corvint feature` in a repository with no indexed sources; `verificationProfile` in
  `internal/contextindex/receipt.go` now matches the oracle.
- **Weaker than the vector above as demonstrated-red**: it was captured after the Go budget path
  existed. It is still non-circular — Python authored every byte — and it did fail the Go candidate
  when first run, which is how the gate-election divergence was found.

## Trace advisory gap closed

`GPK-V0-044` made the local trace store explicit fixture data for the standalone repository-intent
profile. `conformance/cli-parity-v0` now carries pinned Python-oracle vectors with matching advisory
results, nonmatching and non-passed rows, absent and mixed-worktree states, and budget compaction.
The authority-start profile still refuses a present clean-tree trace store; the repository-intent
profile now consumes it.
