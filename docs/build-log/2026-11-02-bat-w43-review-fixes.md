# bat-w43 independent-review repairs

Date: 2026-11-02. The owner supplied six findings against
`8e84e2e5ba7b321ec184edacce512dab0ab416e4..fce793663668d20dbe4fa005a64cb2a7052ae1b2`
and requested an uncommitted repair handback without .git writes, codex, Tasks writes or make gate.

Scratch Git now uses batch-check's environment allowlist (ASD-V0-785). Interrupted repinning
captures the missing after snapshot during cleanup and publishes normal snapshots atomically
(ASD-V0-787). Unaudited screening recognizes valid qualified references to supplied raw IDs,
resolves only Git location, and enters the pure-read registry so writer preparation cannot read
the queue hint (TCD-V0-773). Lane scans retain captured commit pins and first-head attribution
(TCD-V0-774). The interrupted-lane and CI-policy fixtures canonicalize temporary path aliases
(ASD-V0-788, TCD-V0-770). No requirement IDs or persistent formats were added; rollback is to
revert these local repairs and their regression witnesses.

Focused Go tests and assembly/interrupted-lane shell tests pass. The native precheck suite
retains an unrelated Pi stale-tuple diagnostic failure under Node 22.22.1's isolated reporter;
the same suite passes with a temporary external wrapper executing that node:test module
directly in a fresh foreground process. No assertions are suppressed. Spec-generation and
requirement-definition checks pass on a temporary staged fixture; the real index is unchanged.

Per-finding failing controls, passing commands, the Node workaround and friction are retained
in [the requested handback](../../bat-w43.fix-handback.md). Darwin qualification and a new
independent post-fix review are NOT_OBSERVED/NOT_RUN. CEM binding/sealing, native ticket completion,
exhaustive gates and publication remain NOT_PRODUCED/NOT_RUN under the owner's repair scope.
