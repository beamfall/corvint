# Decision 0371: Local LSP evidence provider for one- and two-hop context expansion

Date: 2026-09-23. Status: proposed (experimental delivery; ticket V1-0099).

This decision:

- adds `EEP-V0-023..026` and `TCP-V0-043..046`;
- adds the package `internal/lspprovider`, the file `cmd/corvint/context_lsp.go` and
  `extevidence.InlineSection`;
- changes no default wire and no `protocol/**` wire, and adds no root verb, flag or help text.

## Context

The packet's syntax slots follow imports and identifiers the index extracts itself. They do not
follow the type-checked definition and reference edges a language server resolves: a method call
through an interface, a selector on an imported package, or an unexported caller in another file.
Graph-guided localization systems build this kind of neighbourhood: LocAgent walks a code graph
over several hops, and the ticket cites RPG-Encoder (arXiv 2602.02084) for repository graph
encoding. Corvint's invariants rule out a daemon, a network dependency or a new authority, and
they require that missing evidence stays visible (invariants 2, 3, 7).

## Decision

1. **The provider is an ordinary external-evidence provider that Core owns and runs in
   process.** gopls answers `textDocument/references` and `textDocument/definition`. Core turns
   the answers into one `external-evidence-provider/2` record of path-to-path relations and
   decodes it with the same code as every file, command or MCP record (`InlineSection`). The
   relations therefore carry `external-provider` authority and trust, Git-ancestry freshness and
   per-endpoint blob verification. They never become project authority.
2. **It is off by default, behind one environment knob**, `CORVINT_CONTEXT_LSP=gopls`, following
   the `CORVINT_CONTEXT_ANCHORS=on` precedent. Any other value prints the golden bytes.
3. **The output is a separate `external` member, not a ranking input.** `results` and every
   other member are unchanged, as `EEP-V0-015` and `EEP-V2-003` require for provider items.
   Changing ranking from these relations needs its own requirement and the paired ladder of
   decision 0070.
4. **One owned process per invocation.**
   - gopls runs as `gopls serve` inside `internal/procgroup`, with a 20 s hard wall time and a
     15 s soft query deadline.
   - Output is bounded per message and in total.
   - The environment disables module downloads, the checksum database and toolchain switching.
   - gopls gets a private cache directory, which is removed afterwards.
   - A record is trusted only after a clean exit with the process group proven cleaned up.
5. **Bounded, labelled expansion.**
   - Bounds: at most three seeds, two hops, 64 queries, 32 relations and 64 KiB.
   - Every relation names its query, its hop, its seed and, at hop two, the file it came
     through.
   - Every relation also opens with a SHA-256 query digest over the provider, commit, seeds and
     bounds.
   - The provider revision is gopls's own module version.
6. **Pinned or omitted.** gopls reads the working tree, so a file whose bytes differ from the
   index is omitted and counted, never related.
7. **Failure is one visible row.** Absence, inapplicability, a timeout or a session failure
   gives an `unavailable` provider row with its reason and no partial relations.

## Alternatives set aside

- **A long-lived gopls daemon or `gopls -remote`.** Faster on warm repositories, but it is the
  permanent daemon invariant 7 excludes.
- **Merging relations into `results`.** Measured recall could move, but external evidence would
  then outrank or displace project evidence (invariant 3). The ranking question is left to its
  own measured slice.
- **A generic LSP host for every language server.** No second server is qualified. The record
  shape is language-neutral, so another server would add only its own launch profile.
- **Symbol-level entities.** The provider contract has no symbol identity (open ticket
  `V1-0101`), so relations stay path-to-path.

## Consequences and rollback

- `context` with the flag set costs at least one gopls workspace load. On Corvint itself the
  15 s soft deadline stopped hop one after 8 queries, so large repositories get a truncated but
  labelled expansion (`query.stopped`).
- gopls shares the user's Go build cache and follows the user's Go telemetry mode, which Corvint
  neither reads nor changes. Both are disclosed in `EEP-V0-*` trust text.
- Rollback removes the following and unsets the knob; the default wire never changed:
  - `internal/lspprovider`
  - `cmd/corvint/context_lsp.go` and its test
  - the one `attachLSPEvidence` call
  - `InlineSection` and its `sectionOf` split
  - the fixture `lsp-gopls.json`
