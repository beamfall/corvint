# LSP public harness: combined relation states (V1-2536)

## Defect

A WSL2 r20 run of `script/measure-lsp-quality.py` (gopls v0.23.0) reported `go-attach-expand`
combined 0/20 while upstream and Core passed 20/20: 20 path relations, provider
`omitted_rows` 64, 64/64 queries, Core packet parity true, and the gold
`internal/lspevidence/context.go -> internal/lspprovider/provider.go gopls:uses-definition`
missing.

## Root cause (reproduced)

Reproduced on Linux with go1.27.2/go1.27.3 and gopls v0.23.0 with `--root` a clean
`sourceBase` (`cf93522e`) worktree. The CLI built from that `sourceBase` tree gives combined
0/N with the same 20 relations, 64 omitted provider rows and 64 queries. The CLI built from
origin/main `025120e1` gives N/N. Both binaries put the gold relation in the 32-row provider
record. At `sourceBase`, `lspevidence.Attach` calls `extevidence.InlineSection`, which sorts
`external.path_relations` only by the lexical relation key. In that order the gold row is index
21 of 32: `cmd/corvint/context_lsp.go` and eleven `internal/contextindex/index.go` rows sort
first. `--limit 20` then cuts it. Origin/main calls `InlineTaskSection`, whose TCP-V0-054
subject-first selection puts the gold row at index 4. The provider rows cap and the
core/gopls merge are not the cause. The README told operators to build the CLI in the
`sourceBase` checkout, and that measures the pre-TCP-V0-054 projection. The harness failed
correctly, but its boolean check could not tell a broken projection from a legitimate
bound miss.

## Decision

LQP-V0-022: classify each combined relation as `present`, `projection-not-subject-first`,
`omitted-by-packet-bound`, `omitted-by-provider-bound` or `absent`. Only `present` passes. A bound
miss is reported as uncertainty (`combinedRelationUncertain`) and still fails. The README now
builds the CLI from the candidate tree and measures against a separate `sourceBase` worktree.
The frozen qualification record's command text is unchanged. No Go code changed. `--repeat`
stays limited to 1..20.

## Rollback

Revert this change to restore the boolean path-level relation check and the old README procedure.
