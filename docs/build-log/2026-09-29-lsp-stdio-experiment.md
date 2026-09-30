# Experimental capability-free LSP stdio boundary

V1-0481 candidate branch `codex/lsp-stdio-editor` starts from public origin/main
`75f06f9020a38af8a775a35479b4b16cc69b8e3c`. Owner requested both agent enrichment
and an optional editor companion; this slice remains a proposed/experimental
technical contract, with no semantic, text-sync, snapshot or upstream integration.
The standalone command requires `--experimental` and owned pipe/socket stdio.
Rollback stops/disables the companion without changing Core defaults.

Focused race tests passed for `internal/lspstdio` and `cmd/corvint-lsp`, including
framing, lifecycle ordering, encodings/roots, unknown cancellation, blocked
input/output cancellation and real subprocess interruption after initialize.
The initial real-process interruption test FAILED: closing inherited os.Stdin did
not retire a blocked read. The repair checks the stream kind, registers owned
pipe/socket descriptors with the Go poller and proves interruption with a received
initialize response followed by a partial input frame. Interactive terminal and
regular-file descriptors are rejected before setting nonblocking flags. There are
no subprocess descendants or private provider caches in this profile.

Focused vet, `internal/specindex`, spec-requirements-check,
requirement-definitions-check and traceability-tests-check passed. Initial spec
checks failed on README claim mismatch and unstaged requirement generation; both
were repaired. Traceability tooling retains three planned rows, not qualified
client support. `make gate` and repository-wide tests are NOT_RUN under scoped
owner policy. Real VS Code/Neovim qualification and independent review are pending
and owned by the coordinating task; transcript tests establish no supported tuple.

Corvint query was used at the baseline; it returned a generic documentation
compiler authority lead plus explicit omitted results, not LSP contract evidence.
Direct routes and the owner's external LQP proposal supplied the bounded context.
`affected --base 75f06f9020a38af8a775a35479b4b16cc69b8e3c` was used before tests;
its broad advisory checks do not override the owner's explicit focused-check
scope. Query, affected and check outputs are retained privately under
`.corvint/lsp-stdio-evidence/` in the isolated worktree.

`make dogfood-change` was run at change start and FAILED with cem-prepare
`git-diff-failed`, missing intent scope, missing outcome/verification inputs and
subsequent map-not-produced reasons. Enrolled begin was attempted twice with the
new experimental owning spec and returned `intent-path-not-found` because that
spec did not exist in committed content. There is no active enrollment/session key;
no unrelated accepted intent was substituted. Final CEM/OCM binding, independent
review and local outcome remain NOT_PRODUCED for integration. No seal, publication,
merge or native ticket completion is claimed by this branch.

Official LSP 3.18 base/initialize source was read after Context7 returned mixed
3.18/3.19 snippets; mixed snippets were not treated as complete protocol proof.
Parent processId monitoring is explicitly deferred: EOF/deadline bounds are not
a parent-crash qualification claim. Live overlays, Unicode document positions,
agent/editor parity, outcomes and semantic freshness remain NOT_RUN. Retrieval,
learning, mutation, provider, console, navigation and flow feature routes are not
applicable to this separate capability-free transport. Token/cost savings are
NOT_OBSERVED. Native runtime stayed inherited medium for lifecycle reasoning;
account usage was 87% at start and 90% at the final checkpoint, below the 99% stop.
