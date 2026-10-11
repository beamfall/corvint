# Batch w3 integration (PRs 749, 750, 751, 752, 757, 759, 763)

Date: 2026-10-10

Batch w3 carries seven owner-accepted pull requests onto main 97c3a4bd. Each one was conflicting
against main only in `docs/decisions/README.md`:

- #749: no process-group signal after the leader is reaped (`PGO-V0-009`, decisions 0492 and
  0501; it also carries the records for decisions 0477 and 0479).
- #750: the top-level tasks help no longer implies that `ticket create` takes `--target`
  (V1-0628, `CAL-V0-212`, decision 0502).
- #751: `examples/**` relicensed to AGPL-3.0-or-later, the verify-pr SC1007 fix, and the recipes
  test in the gate (V1-0135, V1-0128, V1-0139, decision 0504).
- #752: a sanitized stderr line in the VS Code failure text, and no document opening for evidence
  diagnostics (V1-0439, V1-0440, `VSC-V0-077`/`078`, decision 0503).
- #757: the lost decision 0500 record (`AFP-V0-041` eight-shard default).
- #759: the spec index prose false positive and the generated spec docs (V1-0524, V1-1109,
  V1-0178, V1-0163, `SRG-V0-002`/`003`, `DCG-V0-001`, decision 0506).
- #763: the `AFP-V0-028`..`030` amendments (decision 0509).

Resolution: every conflict was a pair of row insertions at the head of the decisions index. Both
sides were kept, with main's rows after the batch rows, in merge order. `REQUIREMENTS.tsv`
regenerated without change. Decision numbers and requirement IDs are unique across the batch and
main; the one duplicate decision number, 0016, predates the batch.

Evidence:

- Each constituent was reviewed, bound and sealed on its own branch; its sealed CEM is carried
  under `.corvint/changes/`.
- On the batch head, the doc gates and `script/repin-use-case-receipts.sh --check` pass. The Go
  tests of every package the batch changes pass, under a host load average near 54:
  `internal/groupreap` and `internal/tasks/intent` each failed one timing test on the first run
  and passed on rerun, and `cmd/corvint` hit its 30-minute timeout, so only its failed and
  timed-out tests (`TestDogfoodEventSpawnsOneBracket`, `TestWorkFinalCheck*`,
  `TestAHI044HookAdaptersFailOpen`) were rerun, and passed.
- Merging #749 with main broke the build of `internal/tasks/store` tests: #749 removed
  `killGateGroup`, which `pool_output_bound_test.go` (CAL-V0-033, from main) still called in its
  cleanup. The Unix-only fixture now sends the same `SIGKILL` to its own group directly, as the
  other test cleanups do.
- Change evidence is not bound on this branch: the owner moved the rebind to the w4 batch, which
  lands this branch together with the other open PRs.

Limits: `make gate`, the full `./...` suite, a complete `cmd/corvint` run, and the rest of the 235 packages that
`corvint affected` selected are NOT_RUN. Merge-commit conflict resolutions are not bound by a CEM.
