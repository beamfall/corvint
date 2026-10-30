# Go archive containment — V1-3179

This lane works the eight CodeQL archive-extraction alerts in V1-3179: #15 and #16
(`go/unsafe-unzip-symlink`) and #17–#22 (`go/zipslip`). One of them is a confirmed
defect, and that defect is repaired here. Every other site already refused traversal
on POSIX, but used a sanitizer the analyzer does not recognize. Those sites now share
one platform-aware containment check. Closing the alerts still needs the CodeQL run on
the delivered head.

## Decision

- **Shared lexical check.** `internal/archivepath.Join(root, name)` converts a
  slash-separated member name to native form. It refuses any name that
  `filepath.IsLocal` rejects for the running platform: empty, absolute, an escaping
  `..`, and on Windows a volume, rooted or reserved device name. Otherwise it returns
  `filepath.Join(root, name)`. It does not resolve links.
- **Sites routed through `archivepath.Join`.** Each site's own validator stays in place:
  - `internal/update` `extract` (UPD-V0-003)
  - `internal/changee2e` `untar` (CCE-V0-008)
  - `internal/companionrelease` `VerifiedRetainedBundle.Extract` and `writeTree` (PUB-V0-015)
  - `tools/compat-trial` `materialize` (CTR-V0-001)
- **`receipt unretire` (JRT-V0-011).** It now writes each restored receipt under the
  canonical name that `snapshot.ReceiptName` regenerates from the parsed sequence. It no
  longer writes the archive member's own name. The base code already required that
  exact canonical name, so no file name changes.
- **`conformance/cli-parity-v0` `extractSafeTar` (#15–#17, confirmed defect).** It
  validated each link target only lexically. A chain like `d/a -> ../x`, then
  `b -> d/a/../../outside`, then `b/payload` wrote outside the root. The function is
  unused, but it can still be called. The fix:
  - Each member's parent, and each new link's target, is now resolved through the links
    already extracted. A userland resolver does this with Lstat and Readlink, a 40-hop
    limit, and the real root from `Abs` plus `EvalSymlinks`.
  - Any step above the root is refused.
  - Regular files are created with `O_EXCL`.
  - After the last member, every link is resolved again. A link that a later link made
    escape is removed and the archive is refused.
  - No requirement owns this harness function, so this entry is its record.

## Evidence

On Linux/amd64 with Go 1.27.3, run through heavy.sh, all of these pass:
- `TestV13179ExtractConfinesMembersToDest`
- `TestV13179UntarConfinesMembersToTree`
- `TestV13179ExtractConfinesEntriesToTarget`
- `TestV13179WriteTreeConfinesEntriesToDir`
- `TestV13179MaterializeConfinesEntriesToRoot`
- `TestV13179RetiredReceiptReturnsCanonicalName`
- `TestV13179Join*`
- the four `TestV13179ExtractSafeTar*` tests
- the existing `TestUPDV0003`, `TestCCEV0008`, `TestJRTV0006` and `TestJRTV0011` tests

Each new test has a positive control and escape attempts. On Windows the escape
attempts also include reserved device names.

For #15–#17 there is a before/after witness. The fixed tests were run against the
baseline `snapshot.go` at `21fea70cb1fec853eecd55ebaadc3a61a1e11534`. There,
`RefusesLinkChainEscape` and all three `RefusesLateEscape` subtests fail with "escape
was extracted". With the fix, they pass.

## Limits

- **Analyzer result not observed.** No local CodeQL is installed. That the change
  satisfies the analyzer is inferred from the 2.27.3 query library: the `IsLocal`
  guard, the `EvalSymlinks` suppression, and arithmetic not being a taint step. It stays
  NOT_OBSERVED until the delivered-head scan runs.
- **Fixture links.** The `cli-parity-v0` fixture links in `materializeEntries` still
  validate link targets lexically. Those fixtures are repository-owned input, so they
  are out of scope here.
- **Not run.** `make gate` was not run.

Rollback: revert this change. No wire, schema, ID or persisted-data change.
