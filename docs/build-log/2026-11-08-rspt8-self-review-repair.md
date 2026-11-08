# Shell results and process dependency self-review repair

Date: 2026-11-08
Scope: V1-5110 / rspt8 self-review's three P1 findings, against
`8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`; repair-start HEAD
`751d4b75e144c6a09ae87fa099dbf9b9c36ac094`.
Authority: `docs/specs/runner-selection-plugins-v0.md`, RSP-V0-003, RSP-V0-007,
RSP-V0-022. No new accepted requirement, wire, error code, stderr, golden,
qualification or narrowing claim.

## Findings and decisions

1. RSP-V0-003: the existing platform observation boundary overwrites native terminal
   states when the report is incomplete. Add a separate internal shell-result entry
   point to the same bounded parser; it preserves parsed terminal states alongside
   the original error and incomplete status. Existing `platform.Parse` retains its
   frozen semantics. The entry point admits only Bats and ShellSpec; existing native
   parsers admit their explicit TAP/formatter grammars, and the adapter admits only
   passed/failed/skipped-with-reason. Unknown runners/records remain errors; earlier
   failures still stand. A passing FULL retry cannot erase the first attempt.
2. RSP-V0-007/022: access mode cannot prove independence from prior existence.
   Successful ordinary opens retain a dependency regardless of access mode unless
   the existing creation proof applies. The existing closed supported-open-flags
   predicate remains the authority: O_PATH is metadata access, unknown flags are
   incomplete, and only understood exclusive creation establishes new ownership.
   No additional list of exceptional write-only flags was introduced.
3. RSP-V0-007/022: outside classification controls only executable accounting;
   it cannot certify implicit kernel accesses. Inspect every executable's ELF
   headers, including count-only outside tools. Only understood ELF images and
   bounded, absolute, resolvable PT_INTERP paths can retain complete evidence;
   unknown executable formats remain incomplete. Record repository interpreters
   through the existing path/read machinery. No outside executable digest is added.

Rollback: revert this scoped repair; retain the findings and keep affected runners
FULL/unqualified. Independent review remains orchestrator-owned.

## Acceptance evidence and limits

- `TestRSPV0003ShellDivergence` retains the partial success assertion unchanged.
  `TestRSPV0003ShellRetryPreservesPartialFailure` covers truncation and an unknown
  record after a failure, with a passing FULL retry and preserved missing identity.
  `TestRSPV0003ShellPartialResultsBoundary` covers both shell profiles, unknown
  runners/records, and unchanged legacy UNKNOWN observations.
- `TestRSPV0007WriteOnlyOpenExistence` deletes/renames data after a successful
  write-only open and close, with no read. `TestRSPV0007OpenExistenceProvenance`
  passes for ordinary/nonexclusive/exclusive and unknown create forms.
  Existing `TestRSPV0007OpenProvenance` assertions were preserved.
- `TestRSPV0022OutsideExecutableInterpreter` compiles an outside executable that
  names a repository interpreter. Its direct collector subtest passes and captures
  the repository dependency while leaving outside exec entries count-only.
  `TestRSPV0022UnknownOutsideExecutableFormat` passes the fail-closed control.
- Whole packages ran through `script/heavy.sh`, one per command, without identity
  exports or `-run`: platform PASS (8.210 s); proctrace FAIL (20 top-level tests,
  3.777 s), including existing tests, with unresolved-descriptor/dirfd failures in
  this sandbox; registry FAIL (0.501 s), only `TestRSPV0040ShellTraceRefusals`.
  The live proctrace assertions are unchanged and require the host rerun. Their
  failure does not qualify complete collection, deletion/rename or loader mutation.
- The registry failure is outside the three requested repairs: shellOutput embeds
  bytes.Buffer, so io.WriteString in the existing fixture bypasses its bounded Write.
  A focused rerun with the original shell_plugin.go overlaid reproduces the same
  failure (0.008 s). Neither the unrelated implementation nor assertion was changed.
- `make -s spec-requirements-check line-citations-check` PASS, including its two
  one-package heavy-wrapped checks. Repin found no moved citations after staging;
  180 pre-existing basename citations remain skipped. Toolchain check PASS, go1.27.3.
- NOT_RUN: repository gates, whole-repository test selection, live qualification,
  Windows vet, publication, Tasks writes and independent review, per repair charter.
  No model-backed reviewer or pre-change capture rerun. No seal is authorized.
  Final CEM bind/check outcome is retained in the external handback, not preclaimed here.

## Self-use and original receipts

Used: existing pre-change evidence (not recollected), affected planning and scoped
package verification; final CEM work follows after this commit. Affected selected all
three touched packages and both spec-check packages, with scope UNKNOWN, 545 selected,
388 excluded and 98 unknowns (77 language frontiers, 1 no-selectable-test, 20 unowned
paths). The installed binary warns it is stale (built at the supplied base); this is
its own advisory result, not current-source qualification. The base includes earlier
integrated work and its selection is wider than this repair; only the authorized
packages ran. Frozen retrieval/learning evaluations and provider routes are not
applicable: no retrieval, learning or external provider behavior changed.

Raw logs stay under `/home/agent/lanes/rspt8-evidence/`. Digests bind the exact observed
results, including failures; they do not turn those results into passes.

| File | Bytes | SHA-256 |
|---|---:|---|
| `repair-affected-final.json` | 246378 | `ef7e751d653a9045554a87e14d493959ba81f8b702fd8f4af5cfec2e6eac90b7` |
| `repair-affected-final.stderr` | 498 | `b817bf3fe1a33502037062d87af9d744b2637a17a49cafd79871e783f876320e` |
| `repair-proctrace-final.log` | 52651 | `6a232fab2b380fcab0032adca89cef736b4bd0e78ced48b30ce16b95130ae734` |
| `repair-platform-final.log` | 695 | `8ed55660064939f7aed9b2a51b509287a30a4600e408ae701183649f58fbdd44` |
| `repair-registry.log` | 664 | `fb8e2f5282a3bc879b8b5dea422224e20283ab071b87532113a3aa7da288b3da` |
| `repair-registry-baseline.log` | 664 | `dd58ea46982e781c142437ebb4d04bb8067aa32204282d524735859bf3445a30` |
| `repair-spec-check.log` | 1525 | `cc18163a6c8305c14f1422d16d17ec325d17de58f49fcb991e8a4fa423fd1c2b` |
