## 2026-09-27 V1-0430: gate scripts failed on a stock GNU/Linux host

The release-gates run 36344236053 on 7075454e stopped the ubuntu-24.04 `full-gate` at
`go-archive-gate-test` with `FAIL: a run that recorded its own witness did not pass`. The macos-15
full gate passed. Only the owner's macOS gate had ever reached the steps after `go-test`, so every
later gate step was then run in `golang:1.27.1`. Five steps failed there, from three defects.

- **Mode reads (hosted ubuntu fails).** `script/go-archive-gate_test.sh` and
  `script/gate-receipt_test.sh` read a file mode with `stat -f '%Lp' X 2>/dev/null || stat -c '%a' X`.
  GNU `stat -f` means `--file-system` and takes no format. It prints file-system status for `X` and
  exits 1, so the fallback's mode is appended and never equals 700 or 600. Both now try the GNU form
  first. BSD `stat` rejects `-c` without writing to stdout.
- **Alternation order (Debian `mawk`).** `script/check-error-code-ownership.sh` built its
  per-directory call patterns with `a[k] = (k in a) ? a[k] "|" x : x`. POSIX leaves the order
  unspecified, and `mawk` creates `a[k]` before it evaluates the right-hand side. Every pattern then
  began with an empty alternative (`(|name…)\(`), which matches any call, so 363 strings that are not
  codes were reported unowned. `gawk` and BSD awk evaluate the test first. The membership test is now
  a separate branch.
- **Missing `jq`.** `script/check-host-package-versions.sh` read every version through `jq`. Without
  `jq`, each read fell back to "absent", and every package was reported as a stale bump. The script
  now refuses with `jq is required`. Hosted runners and the owner host both have `jq`.

Evidence:
- **golang:1.27.1, stock `mawk` plus `jq`:** all 25 gate steps after `go-test` passed with the fix.
- **Before the fix:** the two mode tests failed, and the ownership check failed under `mawk` but
  passed under `gawk`. The `mawk` run's `directories` intermediate began each pattern with `|(|`.
- **Without `jq`:** `host-package-versions-check` now prints the new message.
- **Owner's macOS host:** the six changed targets passed.
- **Remaining check:** the hosted ubuntu full gate on the next candidate confirms the rest of `make
  gate` on Linux.

Rollback: revert the change. The hosted ubuntu full gate then fails again at
`go-archive-gate-test`.
