# Decision 0355 — Git-native CEM anchoring through a notes ref, with foreign notes and trailers read as history

Date: 2026-09-22. Status: proposed (experimental delivery; ticket V1-0093). Amends
`docs/specs/falsifiable-packet-v0.md` revision 20 (FPK-V0-037 to FPK-V0-040); it extends no
trust class and changes no existing wire.

## Context

A committed CEM (`.corvint/change.cem.json`) is bound to its change only by its path. Git already
carries per-commit metadata that survives rebases of unrelated history: notes refs and commit
trailers. Other AI-coding tools use both. Git AI stores an `authorship/3.0.0` note on
`refs/notes/ai` (per-file attestation lines, a `---` divider, JSON metadata naming agents, models,
and `messages_url`). Commits made with an assistant can carry `Assisted-by` and `Agent-Logs-Url`
trailers. None of that text is project-owned, and a URL in it must not become a network
dependency (invariant 7).

## Decision

1. `cem anchor --map MAP [--commit REV]` is an explicit mutation. It is the only writer of
   `refs/notes/corvint`, and its receipt reports `mutates: true`. It writes a JSON pointer
   (`corvint-cem-anchor/0`: commit, path, blob, SHA-256, spec) to the map committed at HEAD. It
   refuses an untracked, uncommitted, or dirty map, and a blob that is not in the object
   database. It never replaces a different note. The notes commit uses a fixed identity so the
   write is reproducible.
2. `cem provenance --commit REV` is read-only. It reads that anchor and verifies it by digest. It
   reads a foreign `refs/notes/ai` note and the two trailers as `repository-history` rows
   (authority label `git-history`). Each source has its own `kind`: `cem-anchor-note`,
   `git-ai-authorship-note`, `assisted-by-trailer`, `agent-logs-url-trailer`. Foreign text is
   carried only inside a bounded `untrusted` member. No URL is fetched.
3. The trust enum and `trust.go`/`prove_trust.go` are unchanged. `repository-history` never
   satisfies project authority (invariant 3).
4. The new actions reach `internal/gitnotes` through a `cemcli.GitNotes` hook installed by the
   binary. This keeps the CEM seams' stdlib-and-CEM dependency closure.

## Consequences

- An anchor is a pointer, not a signature. Anyone with push access to the notes ref can write
  one, so `verified` means only "these committed bytes hash to this digest".
- Notes are not pushed or fetched by default. Sharing anchors is an operator Git choice outside
  this decision.
- The round-trip and interop fixtures live in `internal/gitnotes` and `cmd/corvint` tests, not
  in `interop/cem01-go`. That module is an independent Apache-2.0 CEM 0.1 verifier that cannot
  import the AGPL internals, and the pointer is not part of the CEM wire.

## Rollback

Delete `internal/gitnotes/`, `cmd/corvint/cem_anchor.go` and its test, and
`internal/cem/cli/anchor_test.go`. Remove the two `cemActions` entries, the hook and its dispatch
case, the help lines, and FPK-V0-037 to FPK-V0-040. Any written anchors can be removed with
`git update-ref -d refs/notes/corvint`.
