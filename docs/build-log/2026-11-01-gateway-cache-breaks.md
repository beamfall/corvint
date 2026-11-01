# 2026-11-01 gateway cache-break report (experimental)

Base `claude/local-model-gateway` 8dca05f1307f405d050c485b4f4980bdbd422306 (stacked on PR #44).
Tickets V1-4035 (cache-break detection) and V1-4033 acceptance criterion 1 (tool-list byte
share). Spec `docs/specs/local-model-gateway-v0.md`, LMG-V0-011 and LMG-V0-012.

## Decision

Measure before rewriting. Cache-mark rewriting and tool-schema slimming stay non-goals; this slice
adds the evidence that would justify them. Each ledger line now carries the byte length and a
short SHA-256 key of the request's `system` and `tools` values (never their text). The key lets the
report group requests into cache threads without storing the prefix itself.

The prompt-cache prefix is tools, then system, then messages, so a stable thread reads back what
its previous request read plus what it wrote. A shortfall of at least 1024 tokens (the minimum
cacheable prefix) that also writes at least 1024 is a break. Times are request starts (completion
less duration), because the cache entry is written near a request's start; a request that started
before its predecessor ended is not compared with it, so parallel cold requests are not breaks.
The cause is classified from the ledger alone:
- a thread's first request after another thread in the session: the tool list or system prompt
  changed, or a subagent started. Subagents share the session key but carry their own system
  prompt and tools, so they open their own thread instead of looking like a break of the main one.
  A new thread that reads the earlier prefix in full is not a break;
- over 5 minutes between starts: expiry under the default TTL, or changed content. The ledger
  cannot see a client's 1-hour TTL request, so this stays a labelled ambiguity;
- a new prefix at least 1024 tokens shorter: compaction, or another conversation with the same
  system prompt and tools;
- otherwise: earlier content changed (an edited message or hook output).

Ledger lines written before this change have no keys and group by session and model only.

## Review

Codex (one round) returned six findings, all addressed with test scenarios in
`TestLMGV0012CacheBreaks`:
- breaks were timed from completion, so a long stream hid an expiry;
- parallel cold requests were read as sequential;
- a new thread that fully hit the cache still counted as a break;
- HTTP 200 lines carrying a stream error were counted;
- a shrunk prefix from another conversation with the same keys was named an edit;
- the token sum was labelled "written again" although it includes new content. It is now the
  cache-creation total on the flagged requests.

## Evidence

- `TestLMGV0011LedgerPrefixKeys` and `TestLMGV0012CacheBreaks` pass with the existing package
  tests (`internal/modelgateway`, `cmd/corvint-gateway`, WSL host, go1.27.3).
- The owner's live ledger from the LMG qualification (13 lines: one print-mode run, two upstream
  400s, and a 10-request interactive session) reports 0 breaks. The interactive session read
  51,709 to 55,959 cached tokens per turn, each equal to the previous read plus creation, so the
  proxied prefix stayed byte-stable. Those lines predate LMG-V0-011, so the tool-byte share is not
  yet observed on real traffic.
