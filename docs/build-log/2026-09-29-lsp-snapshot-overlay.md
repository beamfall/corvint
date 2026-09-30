# Experimental LSP snapshot foundation (V1-0480)

The owner requested both agent and optional editor LSP work, initially Go/gopls with VS Code and
Neovim, and authorized parallel bounded implementation. This slice creates only an unreferenced
native-Go snapshot component. Technical intent is proposed in `docs/specs/lsp-snapshot-overlay-v0.md`;
no editor, adapter, semantic join, verified Git receipt or support claim is delivered. Rollback removes
this package/spec; there is no persistent state. The native ticket remains open and dependency-blocked.

Base: `75f06f9020a38af8a775a35479b4b16cc69b8e3c` from `origin/main`; isolated branch
`codex/lsp-snapshot-overlay`. The primary checkout and native ticket were not modified.
Inherited medium effort is justified by identity, version and concurrency boundaries. Weekly usage
was 87% at admission and 89% at the verification checkpoint; no reset credits were redeemed.

The API stores immutable captures of client text separately from syntactic Git identities. It bounds
live documents and content bytes, rejects stale/duplicate edits, and invalidates repeated versions on
close/reopen or explicit reset. All filesystem identity, Git object verification, save/rename event
detection, result-publication serialization and caller-retained snapshot budgets remain caller-owned.
URI identity is lexical only; symlink/case alias uncertainty remains explicit. Reset conservatively
clears all live overlays. No Core consumer, ranking, learning or packet bytes are changed.

## Local evidence and limitations

Raw receipts/check logs are retained at `/tmp/lsp-snapshot-evidence/` and pre-change receipts copied
to the worktree's private Git `corvint` directory. Pre-change `query` returned one authority result,
three omitted ranked results and 16 withheld test candidates; path `impact` returned four authority
results with no listed uncertainty. These are context leads, not behavioral proof. Neither sealed
held-outs nor original excluded fixture bodies were opened. No adapter, live server or editor route
was applicable to this isolated package; independent client/profile qualification remains NOT_RUN.

`corvint affected --base` was captured before baseline and after changes. It retained language-frontier
unknowns (including Go build variants/nested modules and other-language dynamic/configuration paths)
and unowned documentation paths. Its static full-gate advice was not treated as owner authorization
to run the exhaustive gate; scoped selection is not equivalent coverage. Go runtime was 1.27.1.
Focused baseline
`go test -count=1 -timeout 30m ./internal/specindex` passed. Post-change
`go test -race -count=1 -timeout 30m ./internal/lspsnapshot ./internal/specindex` and
`go vet ./internal/lspsnapshot` passed. `spec-requirements-check`,
`requirement-definitions-check`, and `traceability-tests-check` passed (three existing planned test
rows retained). These are local observations, not full-gate or integration claims. `make gate` was
NOT_RUN under the owner's scoped-work preference.

Initial dogfood enrollment returned `plan-bound-exceeded`: no genuine base-present owning intent
was declared, and the enrollment implementation requires at least one intent. Enrollment is
NOT_PRODUCED; no substitute intent or session key was invented. Initial `make dogfood-change`
returned incomplete with no baseline diff, missing intent/outcome and pre-change receipts not yet at
its expected private-Git location. Final daily-workflow binding uses documented `#no-intent-declared`
unless the parent review establishes a genuine base-present scope. CEM binding, inspected reviewer
report, seal and final completion are pending independent review; no enrollment is cancelled to hide
that state. Billed tokens, cache use, full-session bytes and end-to-end savings are NOT_OBSERVED.
