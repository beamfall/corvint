# LSP editor harness independent-review repair, 2026-09-29

Independent review of `0d2dc4d26254cfc940376d4b059fcc0b0e0363e2` found outer interruption/exception
cleanup could be bypassed, and success did not require complete wire evidence. The exact review is
retained in `/tmp/editor-harness-independent-review.md`. This is the one bounded repair cycle;
semantic development mode is deferred until the connected server and a separate bounded change.

Before repair, disposable fixture reproduction returned exit 0 with no transcript; interrupting
the outer harness yielded no report and a live owned fixture editor. Those observations are retained
in `/tmp/editor-repair-repros-before.json`. Each reproduction's known fixture process group was
retired in its own `finally`. After repair, the same missing-transcript case exits 1; interruption
exits 1 with a report and no live owned editor (`/tmp/editor-repair-repros-after.json`). An initial
repair used InterruptedError, which subprocess internally retries; the final dedicated
HarnessInterrupted exception reaches the lifetime finally instead. No failed reproduction is
qualification evidence.

The outer lifetime now handles INT/TERM and exceptions, retires its direct client group plus exact
private-directory seeds and witnessed descendants, and preserves malformed/missing evidence as
failure reasons. Cleanup/inventory gaps retain the original directory and report as UNKNOWN.
A report destination failure preserves `harness-report.json` in the original private directory.
Successful probes require complete untruncated ordered initialize/result/initialized/shutdown/result/
exit, the requested root, negotiated encoding, proxy-witnessed configured server executable digest,
zero server exit and retired owned group. The server digest is also checked after execution.

Focused checks cover outer INT/TERM, malformed JSON, missing transcript false success, fragmented
100 KiB transport, EOF and interruption of a signal-resistant server and child. These fixture checks
do not qualify actual editor interruption or semantic behavior. Exact real-client refresh follows
on the committed repaired harness against `/tmp/corvint-lsp-ad605e73-clean`; its clean immutable
build receipt is `/tmp/corvint-lsp-ad605e73-clean-build.json`. The old dirty-source binary reports
remain separate and are not retroactively rebound. Caller-provided source commit metadata is
explicitly NOT_PROVEN by the harness; source-to-binary proof belongs to the external build receipt.
All tuples remain UNQUALIFIED, and unrun conformance/resource/outcome cases remain NOT_RUN.

## Actual-client refresh on the committed repair

Both real clients passed on clean harness commit `c50799f65bd95b2cf67f212180df465cdcc9ab71`:
Neovim 0.12.5 negotiated UTF-8 and VS Code 1.137.0 negotiated UTF-16. Each report has exit 0,
LIFECYCLE_OBSERVED, `wireEvidence.valid=true`, eight transcript records (including configured
server digest provenance), OWNED_EDITOR_PROCESSES_RETIRED and no remaining owned processes.
Each records `harnessWorktreeDirty=false`, the exact harness commit, main script digest and all
three client/proxy asset digests. The server executable digest is
`e2f0d510c33b3e5c60ec3a20d96345f4238a64b1f842a540b81e7aed2170621d`; the separately supplied clean
build receipt binds source `ad605e73b764b798e90a5fef414255aeeddb3dd9`, Go 1.27.1 and
`vcs.modified=false`. The harness itself still labels its caller source claim NOT_PROVEN.

- `/tmp/lsp-editor-neovim-reviewed-clean.json`: SHA256 `ea2e89c436d94427014b3da0f1893b27ee38bcb645c8d0df783bbf89aafe1fdd`.
- `/tmp/lsp-editor-vscode-reviewed-clean.json`: SHA256 `0310da3bcd0ff22801bc06af1db7537af253a7b835569aebf295714ae2c741eb`.

This addendum changes documentation only and does not relabel the earlier dirty-server reports.
At the refresh checkpoint, weekly usage was 95%; optional semantic-mode implementation is deferred
to preserve review/integration budget. Those real semantic/Unicode buffer trials remain NOT_RUN.
