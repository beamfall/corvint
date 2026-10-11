# V1-0281: navigation exposes supporting link review

Intent: AFU-V1-025 amendment proposed, pending owner acceptance (V1-0281).
The baseline acceptance fixture marked `profile/save-name` verified even though its source
link was stale and its flow incomplete. Navigation now carries an additive optional
`review_state` beside the existing test-evidence `verification`, including packet recovery
steps. Supporting links cover the step, expected outcomes, and containing variations and
outcomes. Stale dominates unreviewed; inferred links cannot confer reviewed authority.
No new requirement ID, schema identity, effect grant, or test-evidence meaning is introduced.
Rollback is removal of this field and its projection, reverting the spec amendment and fixtures.

The implementation and focused unit/CLI assertions cover aggregation, step isolation,
unreviewed links, recovery, omitted-field compatibility, and the acceptance fixture's stale
profile step. Both navigation CLI goldens include the new field. Bounded searches found no
standalone navigation packet/map JSON schema; historical evidence receipts are unchanged.
`origin/main`'s owning spec and all 11 open PR diffs (749–760, excluding 753) were inspected;
no new requirement ID was allocated. The existing AFU-V1-025 traceability row was amended.

Evidence and limits:

- Baseline: `32bfc85a44fd82b5793c7e30b2231200e16c025a`; initially clean worktree.
- Corvint query used: `/private/tmp/v1-0281-query.json`, returned the owning spec with
  explicit unparsed-source omissions; its reported revision differs from the worktree baseline,
  so original source was read directly. Corvint affected used before tests and after the
  source diff: `/private/tmp/v1-0281-affected-baseline.json` and
  `/private/tmp/v1-0281-affected.json`; retained language-frontier and unowned-path unknowns.
  Broad suggested tests are not claimed run; owner explicitly excluded `make gate` and `./...`.
- Baseline and modified focused appflows tests passed (AFU-V1-025 through 028).
  Final CLI, document gates and independent review results are recorded below.
- `sh script/repin-use-case-receipts.sh --check`: all 18 receipts current; main.go unchanged.
- The real Git index is outside the writable sandbox. Staging there was refused. Requirement
  generation uses a copied scratch index and scratch object directory with read-only alternates,
  staging the amended spec there before `make -s spec-requirements`; the real index is unchanged.
  A scratch mktemp wrapper supplies `-p /private/tmp/claude-501/gotmp`, since the host mktemp's
  default location is protected despite TMPDIR. No repository tooling is changed for this.
- Initial `make dogfood-change BASE=...` failed writing the external worktree Git directory.
  CEM/OCM sealing is NOT_PRODUCED: sandbox restriction and explicit no-commit task scope.
  No release, learning, live browser, mutation, or external qualification is claimed.
- Installed Core: 1.0.0-rc.3 build 407; Tasks: 0.0.0-tcp01-unverified+build.407.
  Official update checks for both failed their version smoke with
  `process-after-start-failed: descendant snapshot unavailable`; available versions/freshness
  remain UNKNOWN. No verification bypass or installation attempted. Account usage telemetry
  was not available through the exposed tools (NOT_OBSERVED).
- No task-store writes, commits, pushes, decision files or plugin manifest changes. Native ticket
  completion and integration remain outside this patch-only task. Tooling-blocker filing is
  deferred because the owner explicitly prohibited task-store writes.

Independent review found no correctness findings. Its requested variation-only outcome case
was added and passed; saved packets' exact-execution comparison and regeneration requirement
are now documented in the wire contract. No concurrency behavior changed.

Verification (all Go commands used `GOTOOLCHAIN=local`,
`TMPDIR=/private/tmp/claude-501/gotmp`, `GOMAXPROCS=3`, `-count=1`, `-timeout 30m`):

- `go test ./internal/appflows -run 'TestAFUV102[5-8]|TestNavigationExecutionAdmission'`: PASS.
- `go test ./cmd/corvint -run 'TestAFUV10(2[5-8]|39)'`: PASS, including navigation goldens,
  scripted navigation goals, and the acceptance fixture's map and packet stale-link checks.
- `make -s spec-requirements`: generated from the scratch-staged owning spec.
- Document gates: PASS for `spec-requirements-check`, `requirement-definitions-check`,
  `traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
  `error-code-ownership-check`, `unbounded-readers-check`, `use-case-receipts-check`, and
  `use-case-receipts-test` (all 5 cases). The three temp-path failures passed on retry.
  Earlier environment failures are retained in
  `/private/tmp/v1-0281-doc-gates.log` and `/private/tmp/v1-0281-doc-gates-final.log`; the scratch
  mktemp shim was corrected to preserve explicit template paths. Git scratch-index variables
  are scoped to this worktree's commands, so fixture repositories use their own indexes.
- `git diff --check`: PASS. No exhaustive gate or full package sweep ran.

Final local outcome: patch and requested focused verification complete; independent review clean.
Owner acceptance of the proposed amendment, real-index staging and post-commit dogfood sealing
remain outstanding under the stated task/sandbox limits. Final check output is retained in
`/private/tmp/v1-0281-unit-final.log`, `/private/tmp/v1-0281-cli.log`,
`/private/tmp/v1-0281-doc-gates-final.log`, `/private/tmp/v1-0281-doc-gates-retry.log`, and
`/private/tmp/v1-0281-receipts-test-final.log`.
