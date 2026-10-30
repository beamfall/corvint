# Decision 0097 — over-bound prompts get a disclosed anchor query, not a stored pointer

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

Claude Code and Codex sessions routinely submit prompts over the 2,000-character / 16,384-byte
task bound: orchestration prompts, pasted transcripts, compaction summaries. The native wrapper
refused every one with `prompt-over-query-bound`, and the user saw
`Corvint FALLBACK degraded: prompt-over-query-bound; coding continues` as the normal case.
`docs/specs/agent-harness-integration-v0.md` forbids truncation, correctly: a truncated prompt is an
invented query (AGENTS.md invariant 2).

## Options weighed

1. **Truncate.** Rejected; unchanged from the spec.
2. **Store the prompt under `.corvint/` behind a sha256 pointer that a follow-up command resolves**
   (the Headroom shape in the former `docs/agent-memory/ideas.md` entry). Rejected:
   - `user-prompt` runs on a read path. Invariant 4 permits one write there, the self-observation
     ledger, and a prompt store is a second.
   - `AHI-005` and the lifecycle contract forbid persisting or returning prompt text.
   - The secret screen can only refuse a write. It cannot make a pasted secret safe to keep.
   - The host already holds the full prompt in its own context, so the pointer resolves to
     nothing the model lacks.
3. **Derive a query from the prompt's explicit anchors and disclose the derivation.** Accepted. The
   query is the complete, distinct, verbatim set of backtick spans, path-like tokens, identifiers
   and requirement IDs. It needs no state and no wire change. The disclosure states the counts and
   that the rest of the prompt was not queried, so the packet makes no claim about elided text.

## Calls

1. New requirement `AHI-016` and the "Over-bound prompts" section of
   `docs/specs/agent-harness-integration-v0.md` own the closed anchor rule, its non-goals, failure
   modes, acceptance evidence and rollback. Implementation: `cmd/corvint/prompt_bound.go`, wired
   into the Claude and Codex runners in `cmd/corvint/host_adapter.go`.
2. No anchor subset is ever chosen. An anchor-free prompt, or one whose anchor set exceeds either
   bound, keeps the named `prompt-over-query-bound` refusal and its visible degradation (`AHI-009`).
   This answers the 2026-09-12 `questions.md` entry "harness adapter: is a routinely-exceeded task
   bound the right design?". The routine case is now served, and the residual refusal stays visible.
   It is not reclassified as a silent skip, so the "degrade visibly" rule is unchanged.
3. The kernel task bound, the MCP `task` schema and the Gemini CLI and OpenCode integrations are
   unchanged. Those adapters keep refusing until they implement the same rule. The anchor-free
   cross-host boundary case in `conformance/harness-event-v0/common-logical-interaction.json`
   remains a refusal for every host.

Rollback: restore the unconditional refusal in `normalizeAdapterInput` and delete
`cmd/corvint/prompt_bound.go` with its tests; nothing stored or migrated.

## Amendment 2026-09-12 (same day, accepted): the JavaScript adapters implement the rule

Call 3 is stale. It says the Gemini CLI and OpenCode integrations are unchanged and keep refusing
over-bound prompts, but `fbceef49` shipped the same closed rule in both: the Gemini CLI hook and the
OpenCode `corvint_context` tool serve an over-bound prompt through
`integrations/gemini-cli/hooks/prompt-bound.mjs` and its byte-identical twin
`integrations/opencode/src/prompt-bound.js`, and three anchor-bearing boundary cases in
`conformance/harness-event-v0/common-logical-interaction.json` pin task and disclosure bytes for all
four hosts. The rest of call 3 stands: the kernel task bound and the MCP `task` schema are
unchanged, and the anchor-free boundary case remains a refusal for every host. Call 1's
"Implementation" line likewise extends to the two JavaScript twins, as `AHI-016` already records.

The port first trimmed with `String.prototype.trim`, which differs from Go `strings.TrimSpace` on
U+FEFF (JavaScript trims, Go keeps) and U+0085 (Go trims, JavaScript keeps), so one prompt could be
bounded or disclosed differently per host. The twins now export `trimSpace` over Go's
`unicode.IsSpace` set, and two boundary cases pin both characters for all four hosts.

Rollback: for the JavaScript adapters, follow the `AHI-016` rollback in
`docs/specs/agent-harness-integration-v0.md` (restore the refusal and delete both twins, the
anchor-bearing boundary cases and their tests); call 3's original text then holds again.
