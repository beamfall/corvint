# Decision 0104 — sql-native ratchets pin the toolchain by content and abstain on a lost causal parent

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

`script/check-sql-native-ratchets.sh` hardcoded `/opt/homebrew/Cellar/go/1.27.0/libexec`, and
`SNR-V0-003`/`SNR-V0-004` required that fixed path. An earlier attempt to discover the root
(`4db71bee`, not on this lineage) was withdrawn as a spec conflict. Separately, its pinned
`causal_parent` `a7db6200685c1ea7d4afc1798fb376ccc724acd1` is not an object in this repository, and
the script archived it before any assertion, so no run could complete.

The owner calls:

1. **Pin by content, discover the root.** `SNR-V0-003` now discovers `GOTOOLCHAIN=local go env
   GOROOT` and keeps every content pin: the `bin/go` SHA-256, `GOVERSION=go1.27.0`, the receipt
   tool's own SHA-256, and the whole-root receipt (entry count plus combined SHA-256, over paths
   relative to the root, `internal/toolchainreceipt/receipt.go:78@9e22ea70`). The path was never the identity;
   the digests are. A version string alone still admits nothing. The gain is location independence
   only: the digests are platform-specific, so the gate still runs only on a byte-identical
   darwin/arm64 Homebrew `go1.27.0`. A separate `VERSION`-file digest is not added, because the
   root receipt already covers that file.
2. **Do not replace `causal_parent`.** The commit that added the script (`01f66571`, "rebuild shadow
   analyzer slice") records in its BUILD-LOG entry that the analyzer was provenance-copied from
   rejected checkpoint `929d2ea`. `git cat-file` finds neither object, and no reachable commit before
   `01f66571` contains `experimental/analyzers/sqlnative`. Any re-pin would be a guess, the invented
   certainty invariant 2 forbids. The pin stays as provenance.
3. **Abstain on that half only.** `SNR-V0-013` keeps the current tree's 400 allocs/op causal ceiling
   mandatory. When `causal_parent` is absent, the gate skips the parent export, fixture copy, and
   parent samples, and reports `causal_parent_min_b_op=NOT_RUN causal_parent_min_allocs_op=NOT_RUN
   causal=NOT_RUN-causal-parent-unavailable`, the same form as the existing `latency=NOT_RUN`. The
   strict-separation claim is never printed without its measurement. Every other check still runs.

Verified on this host 2026-09-12: `go env GOROOT` resolves to the formerly hardcoded path; the go
binary, receipt-tool, and root-receipt pins all still match, so no pinned value changes. The script
prefix through the first `check_toolchain` completed with `causal_available=false`. The 30-sample
benchmark was not run, so no `SNR-V0-014` success line has been observed.

Known residual: `core_parent` `718dfc7d` resolves only through `refs/codex/snapshots/*` refs, so a
fresh clone fails `SNR-V0-007` at its `git archive`; the spec's drift section records this.

Rollback: revert this decision's commit. That restores the fixed path, the unconditional causal
archive, the prior SNR-V0 text, and the `fixes.md` entry.
