# Lane friction: finish-lane names the dirty paths it refuses

The sec-res-w2 lane (V1-3197..3199) followed its brief and built `corvint` into `./bin/` inside the
worktree. `bin/` is not ignored, so `make dogfood-inputs-check` refused `dirty-worktree` with
"remove every other uncommitted path" but named no path. The stopped run's
`.corvint/change.cem.json` sidecar looked like the cause, but finish-lane already admits it. Only a
separate `git status` showed the binary.

- `DCW-V0-390`: the `dirty-worktree` refusal from finish-lane and inputs-check now prints an
  `uncommitted:` line between the `REFUSE` line and the `fix:` line. The line names at most five
  sorted, Go-quoted paths from Core's hardened probe and counts the rest. `gokernel.Repository`
  carries the sample as a string, which keeps the struct comparable for its `!=` callers. The sample
  is not on the wire.
- `docs/LANE-ORCHESTRATION.md` §3 tells lanes to build their own `corvint` under the git dir,
  beside `lane-inputs/`, not into `./bin/`. The orchestrator's lane brief should say the same.

- While verifying this change, I found that `TestCheckDCWV0160ComputesOCMStatusOnce` left an
  untracked `internal/dogfoodflow/corvint-dogfood-report.json.tmp` in the source tree. The test
  built its check with no Git directory, so the report was staged in the working directory. That
  is the same dirty-worktree trap. The test now sets the Git directory to its temporary root.

Other friction from that lane needs no product change:

- `dogfood-change` was run after the edits instead of at change start, and returned rc=2. That was
  a process slip.
- The intents file was refused as unsorted. The refusal already names `LC_ALL=C sort -u`.
- A hand-edited citation cited a span its own hunk rewrites. The prepare precheck refused it
  (`cite-span-not-stable`) and named the nearest stable spans.

The field went into `gokernel.Repository` as `DirtySample`, one line at the end of the struct, and
its helper went into `repository_sample.go`, so the rest of `repository.go` shifts by only one line.
The moved citations were repinned with `script/check-line-citations.sh --repin`. The four unpinned
citations in `agent-harness-integration-v0.md` and the frontier brief now carry content hashes, and
their inventory rows were deleted.
