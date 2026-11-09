# Carry Greptile 5/5 across unchanged PR diffs

Owner instruction (2026-11-08): "figure out why this is so difficult, and fix it".
Requirement: GRG-V0-002 in `docs/specs/greptile-gate-v0.md` (previously unused).
Base: `19c5a0111d667a11fd3a82ebcc8920f8a848260c` (`origin/main`).

## Root cause and rule

The owner supplied PR 153's timeline: Greptile reviewed `106c1c982` at 23:16, a main
merge at 23:40 reset the gate to pending, and the manual re-review request came only at
00:00. The workflow compared commit identities before considering whether the
PR-authored change was the same. Base merges and subsequent dogfood bind/seal commits
therefore forced another manual review cycle without a new PR-authored code change.

GRG-V0-002 admits a stale 5/5 review only when GitHub's `<base_ref>...R` and
`<base_ref>...head` comparisons have identical sorted `(status, filename,
previous_filename, patch)` tuples with every patch byte (hunk coordinates included, after review found that dropping them let an edit move between identical contexts) and excluding
only the named dogfood evidence files. Hunk section text and all other patch bytes
remain significant. Renames from source into an evidence path remain in the comparison.
An independent `R...head` comparison must report `ahead` or `identical`.
Failed reads, missing patches, pagination, missing file lists, truncation, and the
300-file limit refuse carry. File limits apply before excluding evidence. All existing
waiver, pending-first, thread, serialization, and merge-group behavior remains in force.
The same PR-head decision is posted to the merge-group commit.

The implementation reads metadata only; no checkout, PR code execution, dependency
installation, or new workflow permissions. Branch refs are URI-encoded. JSON is parsed
by jq, never evaluated as shell code. Rollback removes the carry predicate and restores
exact-head-only review. Official API and CLI references checked for comparison limits
and paged response handling:
[GitHub comparisons](https://docs.github.com/en/rest/commits/commits#compare-two-commits)
and [gh api](https://cli.github.com/manual/gh_api).

## Evidence and limits

`TestGRGV0002CarryOnlyUnchangedPRDiff` exercises 29 scenarios on each of
`pull_request_target` and `merge_group`. Before changing the workflow, its
`identical_diff` subtests failed on both paths: expected pending then success, observed
pending then pending with `ask @greptileai to re-review`. The same tests pass with the
carry predicate. Existing GRG-V0-001 and V1-5096 tests are retained.

Validation commands (heavy commands forced local because the owner prohibited network
writes and work outside this worktree):

- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'GRGV000|V15096' ./tools/ci-reuse-plan/`: PASS.
- `GOTOOLCHAIN=local go vet ./tools/ci-reuse-plan/`: PASS.
- `actionlint .github/workflows/greptile-gate.yml`: PASS. Baseline SC2016/SC2018/SC2019
  findings were reproduced on the unmodified workflow; scoped directives document
  intentional jq/GraphQL literals and already-validated ASCII logins without changing behavior.
- `make spec-requirements` and `make spec-requirements-check`: PASS with the spec staged
  in the normal worktree index. The generator also verified the shortened digest Claim.
- `make changed-checks BASE=19c5a0111d667a11fd3a82ebcc8920f8a848260c`: PASS after marking
  test metadata paths as fixtures. The existing stale `internal/tasks/cli` reader entry
  remains a warning; its registry is outside this change's allowed scope.
- `git diff --cached --check`: PASS.

Corvint context was used at the base above and identified the workflow, spec and test;
its bounded packet retained omissions and no existing GRG-V0-002 definition. `corvint
affected --base 19c5a0111d667a11fd3a82ebcc8920f8a848260c` was used before tests and after
the change; it reports UNKNOWN with LANGUAGE_FRONTIER exclusions, not complete
coverage. Installed Core is `1.0.0-rc.3.dev`, build 136 at that base. The optional update
check returned freshness UNKNOWN because process-group cleanup could not be proved
in this sandbox; no update was attempted. Tasks commands, queue writes, subagents,
network writes, push, and CEM bind/seal were excluded by the owner's explicit scope.
Hosted carry remains NOT_OBSERVED. This is a local implementation and adversarial
fixture result, not proof of hosted scheduling or production API availability.

An initial native staging attempt could not create `index.lock`. A worktree-local
alternate index allowed spec preparation to continue. Native staging subsequently
succeeded on retry; final spec generation and checks used the normal index.
