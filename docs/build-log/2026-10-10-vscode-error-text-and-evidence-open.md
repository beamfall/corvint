# VS Code error text and evidence document opening

Owner-requested bounded implementation of V1-0439 and V1-0440 against base
`2145af619f68698dbb1c097369e29d589e623f7d`. VSC-V0-077 and VSC-V0-078 remain
proposed, pending owner acceptance; the extension's deferred/FALLBACK status is unchanged.

Process failures preserve the stable code prefix and display the first non-empty stderr line
through nonfatal UTF-8 decoding and the shared 200-scalar display sanitizer. Empty/blank stderr
keeps the prior fallback. Diagnostics retain containment, identity, severity, caps and generation
checks while constructing whole-line ranges directly, leaving document-bound clamping to VS Code.
They no longer open evidence documents or reject evidence solely because it exceeds file length.
No CLI protocol, process execution policy or installed-host qualification changes are claimed.
Rollback reverts these adapter/spec/test edits; there is no migration or new persistent state.

## Evidence

- Baseline: all 85 extension tests passed.
- Focused regression coverage: `extensions/vscode/test/error-presentation.test.ts` exercises first
  non-empty line selection, control/ANSI/bidi sanitization, malformed UTF-8 replacement, Unicode
  scalar bounds, stable prefixes and empty fallback. `extensions/vscode/test/presentation.test.ts`
  asserts diagnostics for unopened files and beyond-file lines, invalid-line rejection, range
  coordinates, severity/identity and zero document-open calls.
- Independent native reviewer inspected the implementation, spec and test diff and found no
  defects. Its pending generated-index concern was resolved by regenerating both new rows.
- Final extension lint, typecheck and all 89 tests passed. All requested documentation gates passed:
  `spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
  `decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
  `unbounded-readers-check`, `use-case-receipts-check`, and `use-case-receipts-test`.
  The separate `sh script/repin-use-case-receipts.sh --check` passed (all 18 receipts current);
  receipt fixture tests also passed independently with the normal Git environment (5 cases).
  Logs: `/tmp/vscode-final-tests.log`, `/tmp/vscode-doc-gates.log`, `/tmp/vscode-repin.log`,
  `/tmp/vscode-receipts-test.log`. Real Electron/UI clamping and performance qualification
  remain NOT_RUN.

## Corvint use and limits

Installed Core: 1.0.0-rc.3 build 407; Tasks: 0.0.0-tcp01-unverified+build.407.
Official updater checks for both components failed with descendant snapshot unavailable;
available versions/freshness remain UNKNOWN, and no binaries were changed. Account usage
telemetry was unavailable; no cost or savings claim is made.

Used the pre-change authority query and affected plan (raw local outputs:
`/tmp/vscode-context.json`, `/tmp/vscode-affected.json`). The query omitted four ranked results
and returned an unrelated capability spec; the owner's exact paths supplied the fallback.
The affected plan retains language-frontier unknowns and is advisory, not behavioral proof.
The requested extension/doc checks are the scoped verification; repository-wide `make gate`
was NOT_RUN under the explicit scoped-work policy. Go ranking and engine frozen evaluations
are not applicable to these TypeScript presentation changes.

Index refresh and change-start dogfood failed because the shared Git metadata is outside the
sandbox's writable roots (`/tmp/vscode-dogfood.log`). Post-commit CEM/OCM/seal evidence is
NOT_PRODUCED under the owner's no-commit instruction. No ticket store writes or publication
were performed. The requirements generator reads the Git index; a disposable index/object
store in `/private/tmp/vscode-gate-git` supplies the edited spec without altering the real
index. A temporary mktemp wrapper directs gate scratch files to writable `/private/tmp`.
