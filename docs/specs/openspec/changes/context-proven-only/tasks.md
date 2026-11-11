# Tasks: `context-proven-only`

## Tasks

Build tasks in order, one commit each. Each names its requirement IDs and its proof.

- [x] 1 Strip the removed surfaces and their strings: `context --task`, `--worktree-overlay`,
      `feature`, `features`, `overview`, help text, integration skills and commands, process docs,
      and the CCF-V1 `context --task` row and goldens (CCF-V1-002, TCP-V0 retired). Proof: build,
      vet, focused `cmd/corvint` freeze tests.
- [x] 2 Re-home the instruction doctor as `corvint instructions` with the `context --instruction-*`
      alias, and reduce the path brief to three sections (IID-V0-001, TBC-V1-009 to TBC-V1-011).
      Proof: focused `cmd/corvint` and `internal/pathbrief` tests.
- [ ] 3 Delete the context bench layer and its inventory rows (RBD-V0, CWT-V0, HNE-V0, CEP-V0,
      TRJ-V0 retired). Proof: build, vet, line-citation inventories consistent.
- [ ] 4 Delete the ranking code and the trace packages that no KEEP user reaches (CRK-V0, LTA-V0,
      URE-V0 retired). Proof: build, vet, focused `internal/contextindex` tests.
- [ ] 5 Mark the retired specs, regenerate the spec index, amend the modified specs and write the
      build-log entry. Proof: `make spec-requirements` with no diff after, the line-citations
      check, the invariant check, `openspec validate --all`.
- [ ] 6 On the separate branch `claude/ctxcut-cishards`, remove the CI shard table rows of the
      deleted tests and packages. Proof: the JSON files parse and name no deleted package.
