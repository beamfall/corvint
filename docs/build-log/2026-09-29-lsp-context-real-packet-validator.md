# Actual Core packet validation correction

The published tooling parent `1bd413b6adca0aa7d6a0cecb1b2fc0017f28317f` rejected both
actual context development reports from reviewed server candidate `af3052be`: lifecycle/wire and
owned process retirement passed, while strict context validation failed. Raw originals remain
private `/tmp/lsp-editor-{neovim,vscode}-context-af3052be.json`, unchanged. They are failed runs,
not retrospectively promoted successful executions.

Original `internal/mcp/bridge/bridge.go` hashes canonical JSON `index.DirtyPaths`; clean paths
encode as `[]`. The helper wrongly hashed empty bytes. Original
`internal/contextindex/taskcontext.go` criticalSelectors emits `{path,relation}` selectors,
not blob evidence. The helper wrongly treated these typed coverage selectors as blob claims.
This second bounded tool-code repair checks the canonical digest, admits only exact
known governing selector shapes at coverage.critical/critical_missing, requires matching bound
actual evidence, and rejects missing critical governance. Arbitrary unverified path dictionaries
and actual evidence without a blob remain failures.

A new isolated worktree and keyed enrollment use published parent as frozen base. The previous
sealed workflow remains untouched; attempted enrollment there refused prior-completion-stale.
Focused regressions cover wrong dirty digest, unbound actual evidence, forged/missing critical
selectors and arbitrary path-only claims, plus every prior false-pass case. Replaying unchanged
raw reports with their retained before/after Git manifests passes the corrected validator; this
is validator verification, not new client execution or a changed qualification claim.

Intent and delivery remain proposed experimental tooling / UNQUALIFIED. Actual fresh runs remain
NOT_RUN pending independent repair review, then root-selected immutable server bindings. Broader
conformance/promotion measurements remain NOT_RUN or NOT_OBSERVED. Rollback restores the parent
helper/spec without altering default native-Go Core or retained editor evidence. No nested agents,
source-server changes, native Tasks writes or publication occurred in this repair.

Independent review of `9bd0b58f` retained one P2: the typed critical-selector exemption accepted
scalar, bare-dictionary and nested-array containers. The owner explicitly approved one additional
bounded repair cycle after the normal two-code-cycle limit. Candidate `c96947f1` changes only
container validation and its negative fixtures: present critical/critical_missing fields must be
flat arrays with direct dictionary items, then retain exact selector shape/path/relation and
matching immutable governing authority evidence checks. No exemption skips arbitrary claims.

Both original real reports independently reproduce accepted malformed containers before this
repair and rejection after it. The unchanged raw reports pass corrected-validator replay, while
their original execution outcome remains FAILED. Focused harness checks, prior adversarial cases,
Python syntax and diff checks passed. Raw private results are retained under
`/tmp/lsp-context-flat-{repro-before,repro-after,real-replay,focused,selfcheck}.log`.
Fresh actual client execution remains NOT_RUN pending independently reviewed tooling and the
root-selected sealed server. All tuples remain UNQUALIFIED; broad qualification is held.

Independent bounded review passed exact `c96947f1`; the retained review report verified 60
malformed-container and forged-evidence cases reject and both genuine raw reports replay PASS.
