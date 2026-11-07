<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selfloor self-review repair: preserve unmodeled reads

V1-5000, FRONTIER_SITE slice; base `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`,
incoming head `4bdad492aa1a7313920c650d587fa18d7abd084b`. The owner requested
only the four P1 self-review repairs, with no publication, task writes or seal.
The existing pre-change capture is reused. This entry records the repair
choices under AFP-V0-3121, AFP-V0-3122 and AFP-V0-3128; no new public contract
or requirement ID is introduced.

## Findings and decisions

1. `strip` passed its expanded argument through unchanged. A space generated
   by `subst` was consequently truthy, losing the actual local package edge.
   Evaluate GNU make's ASCII whitespace trimming and collapsing before the
   surrounding condition runs. Unknown inputs retain uncertainty. Mark `strip`
   as evaluated so its read boundary is checked as other evaluated functions
   are. `TestFrontierSiteStripCondition_AFP_V0_3121` checks space and tab
   conditions and selection of the actual package's target and callers.
2. Attached long option values bypass the file-operand scanner. Choose the
   explicitly permitted conservative refusal instead of inferring additional
   option semantics: value-taking long options with `=` remain uncertified,
   as joined short values already do. The extensionless `--file=LICENSE`,
   quoted filename and changed-directory regressions require target and caller
   selection for both the pattern file and an unrelated sealed CEM.
   Witness: `TestFrontierSiteAttachedFileOptionKeepsClaim_AFP_V0_3128`.
3. Literal make arguments are insufficient evidence of a bounded invocation.
   Use an exact allowlist for modeled scheduling, reporting, directory and
   ordinary mode flags. Evaluation, loading, file/search-path options,
   unrecognized forms and the unmodeled option terminator retain claims.
   `TestFrontierSiteMakeOptionAllowlist_AFP_V0_3128` checks conservative target
   and caller selection, including FRONTIER_SITE witnesses, and bounded
   controls that retain certification.
4. Preserve command-local `GOFLAGS` while peeling assignment and `env`
   prefixes. Nonempty or unresolved values retain an uncertified read
   boundary, including through shell wrappers, since executable flags can run
   arbitrary file readers. `TestFrontierSiteGoEnvironmentKeepsClaim_AFP_V0_3128`
   checks direct, env, quoted, make-unknown, shell-unknown and shell-wrapper
   forms, retaining target and caller selection on an unrelated CEM.

The exact-option and GOFLAGS decisions deliberately trade precision for safety;
no replay improvement is claimed. The owning experimental spec records these
bounds and test witnesses. No existing assertion, committed golden, JSON/wire
shape, field, error code, exit code, stderr or core-freeze artifact changes.
Rollback must retain conservative claims for unsupported forms; reverting to
permissive certification restores the missing-edge risks.

## Evidence and limits

The new regression groups failed before implementation in all four finding
classes (0.184 s package test time). The first repair replay passed the four
reported reproducers, then caught a new test fixture's supposed missing `-C`
argument being supplied by the following goal. The fixture was corrected to
an empty attached directory value; no inherited assertion was changed.
A source inspection additionally retained claims for the option terminator,
whose goal parsing the make invocation scanner does not model.

The selected whole-package tests, focused spec/citation checks and final
binding/check results are retained in the lane handback. Raw receipts and logs
stay outside the repository at
`/home/russell/lanes/selfloor-evidence/self-review/`.

Self-development routes: affected selection is used (406 units, scope UNKNOWN,
70 retained unknowns; installed executable reports self-binary-stale). Existing
pre-change receipts are reused by repair instruction. Final CEM/OCM reporting
and dogfood-check provide structural binding evidence, not semantic acceptance.
Independent review and host-wide verification are deferred to the orchestrator
by explicit instruction. Historical replay, mutation, external interoperability,
Windows and Darwin qualification are NOT_RUN in this focused repair. No task
completion write or delivery beyond the local lane is claimed.

## Focused verification results

All Go tests ran through `script/heavy.sh`, one package per command, with
`GOTOOLCHAIN=local`, lane TMPDIR, no identity variables and no lock override.
The local toolchain is go1.27.3. Whole-package tests on final code passed:

- `go test -count=1 -timeout 30m ./internal/liveverify/affected`: 107.208 s,
  including every inherited test and the four new regression groups. An earlier
  whole-package run passed in 116.544 s before the option-terminator repair.
- `go test -count=1 -timeout 30m ./internal/gateselect`: 2.004 s; selected
  driver parity package, unchanged in this repair.
- `script/check-line-citations.sh --repin`: no moved citation.
- `make -s spec-requirements-check line-citations-check`: PASS after final
  spec edits (includes heavy-wrapped specindex and focused lrfrepo checks).

Low `/tmp` space was warned despite explicit lane TMPDIR; admission succeeded.
The existing binary's stale-source notice and Linux-only qualification remain
visible. Final binding/check outcomes belong in the external handback because
that outcome occurs after this implementation commit.

| External artifact | Bytes | SHA-256 |
|---|---:|---|
| `affected.json` | 162498 | `4d3a56e29718db8d8b9349e1505fb2b2cd0d5a2bf27e9dc6a946056f5effa2e7` |
| `regression-before.log` | 6069 | `8f1dfc39eef500e8931b449ca59bcd987447bb3b0414dd09e5616352b65151e2` |
| `regression-after.log` | 1318 | `e938cfd46c9026b87854f1c723bf012efeab18815842324fc3178df0d5d6b678` |
| `affected-test.log` | 1049 | `025b3b22b5d6baf6722c92aa973a163d92e7ee088de19da29954a0b5b59101f2` |
| `gateselect-test.log` | 843 | `7eaf1d627f6e9290c1adbe07dabae6ef18823dda8707ccea6885ee74996823ec` |
| `spec-citations.log` | 2217 | `13d67c061772a14a6ad18995b3f6eee516c824ebdcd0f751abe7a305a451101e` |
| `repin.log` | 190 | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
