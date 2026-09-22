# Decision 0014 — project-operations instruction results carry their classifying words

Date: 2026-09-01. Status: accepted. Authority: repository owner, verbatim instruction "amend"
(2026-09-01), given on the proposed amendment to `GPK-V0-039` in
`docs/specs/go-production-kernel-migration-v0.md`.

## Scope

The instruction accepts exactly one amendment to `GPK-V0-039`: an `instructions` result admitted
under `project-operations` intent rests, in addition to the words its text matched, on the query
words that classified the task (the project-operations terms the shared `queryIntent` rule
matched), and the relevance floor counts them as that result's support.

Why: the harness `user-prompt` event routes every prompt through `EvalQuery`, whose floor withdrew
"orient me on the contributor workflow and roadmap gates" on this repository because `AGENTS.md`
shares only the word `gate` with it, while the oracle and the standalone authority-start profile
answer with `AGENTS.md`. The intent classifier had already found five project-operations words in
the task; the instructions file is the project's answer to those words by design (product
invariant 3), so the withdrawal was a one-word accounting of a five-word match.

## Explicit exclusions

This decision does not change the floor for `repository` or `agent-tooling` results, the
two-word bar itself, the divergence `DR-0008`, the harness routing pinned by `GPK-V0-043`, or any
promotion, cutover, merging, tagging, signing, publication, or release gate.

## Consequences

`evalRankDocuments` adds the intent's matched terms to an instructions result's support under
project-operations intent. Conformance is unaffected: the floor is withdraw-only, the
`harness-user-prompt` parity case already clears it, and `harness-user-prompt-out-of-scope` is a
repository-intent task.
