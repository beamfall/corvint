# V1-0276: bounded navigation traffic and linear attribution

Intent: AFU-V1-057 and AFU-V1-058 are **accepted, decision 0512
(V1-0276)** in `docs/specs/application-flow-understanding-v1.md`. Current remote
`origin/main` and every open `beamfall/corvint` PR diff were checked before allocating
057/058; no collisions were found. Baseline: `29e1f653486fca13216606683d3a5c2b05cacd36`.

The existing navigation refusal `navigation-bound-exceeded` now covers more than
256 traffic filenames before any traffic file is opened. Duplicate filenames and
empty files count. The shared 8,192-record bound is unchanged. One map-local index
uses an exact flow/step pair and retains input order within each bucket; steps look
up their own bucket. Traffic attribution takes O(R + S) work and O(R) auxiliary
space, eliminating per-step full-list cloning/filtering. Wire construction, effect
classification, grants and verification remain unchanged. Navigation help belongs
to `cmd/corvint/flows_navigate.go`, so `cmd/corvint/flows.go` needs no change.

Acceptance evidence: `TestAFUV1057TrafficFileBound` exercises zero, 255, 256 and
257 filenames, empty/repeated files and refusal before an invalid first file;
`TestAFUV1057TrafficRecordBound` covers exactly 8,192 and excess records across
files. `TestAFUV1058TrafficIndexParity` compares transitions against the previous
filter, verifies pair isolation and bucket order, and retains caller input.
`TestAFUV1057TrafficFileBoundCLI` checks named refusal with empty stdout and exact
map/packet byte parity with 256 empty files. Existing map and packet goldens retain
mixed observed-traffic cases. A read-only independent reviewer found no blocking
findings in the index, refusal, compatibility or proposed requirements.

Corvint self-use: query, tracked-path impact and affected were **used**; receipts
are retained in `/private/tmp/v1-0276-{query,impact,affected}.json`. Query omitted
four ranked results and withheld test symbols; impact omitted 36 ranked results
including direct callers. Original task-named source/tests were read as fallback.
Affected scope remains UNKNOWN, retaining language-frontier unknowns and the spec as
an unowned dirty path. Affected advice cannot override the explicit owner exclusion of `make gate` and
repository-wide Go tests. The host reported
`corvint-event-rejected:dogfood-event-deadline`; coding continued under the declared
fallback. Commit-bound CEM/OCM, frozen post-commit checks and seal are
**NOT_PRODUCED** because this task explicitly prohibits commits. Browser execution,
provider qualification, learning and mutation experiments are **not applicable**.

Runtime: Core `1.0.0-rc.3 (build 407)`; Tasks
`0.0.0-tcp01-unverified+build.407`. Both official update checks failed with
`process-after-start-failed: descendant snapshot unavailable`; freshness and
available release versions remain UNKNOWN. No update or Tasks mutation was made.
Account usage telemetry was unavailable in the callable tool inventory.

Verification: all completed successfully with `GOTOOLCHAIN=local`,
`TMPDIR=/private/tmp/claude-501/gotmp`, and `GOMAXPROCS=3`:

- Pre-change: `go test -count=1 -timeout 30m ./internal/appflows -run 'TestAFUV102[5-8]'`.
- Post-change: `go test -count=1 -timeout 30m ./internal/appflows -run 'TestAFUV10(2[5-8]|5[78])'`.
- Race: the same post-change appflows command with `-race`.
- CLI: `go test -count=1 -timeout 30m ./cmd/corvint -run 'TestAFUV10(2[5-8]|57)'`;
  existing navigation map and checkout-packet golden bytes passed unchanged.
- `make -s spec-requirements-check requirement-definitions-check traceability-tests-check
  decision-numbers-check line-citations-check error-code-ownership-check
  unbounded-readers-check use-case-receipts-check` passed using the temporary staged
  index. The Go test command cleared temporary Git variables for fixture isolation.
  The traceability gate retained its existing three planned entries.
- `make -s use-case-receipts-test` passed all five cases separately with normal Git
  fixture isolation. An initial scratch mktemp wrapper mishandled absolute templates;
  it was corrected without repository changes before the successful rerun.
- `sh script/repin-use-case-receipts.sh --check` passed: all 18 receipts current.
  `cmd/corvint/main.go` was unchanged; no repin was needed.
- `git diff --check` passed. `make gate` and repository-wide Go tests were NOT_RUN
  by explicit owner instruction.

The final check log is `/private/tmp/v1-0276-checks.log`. No owned test or check
process remains running.

The sandbox denies writes to the worktree Git index outside the writable roots.
The spec and changed sources were staged in a temporary alternate index, with new
blobs in a temporary object directory; `make -s spec-requirements` regenerated the
tracked TSV from that index. Normal staging remains outstanding. A scratch-only
`mktemp` wrapper uses the authorized TMPDIR for invocations lacking a template;
repository scripts are unchanged. No commits, pushes, decision files, ticket-store
writes or plugin manifest changes are part of this task.

Rollback: revert the new file refusal and index in `internal/appflows/navigate.go`
and its help/tests/proposed clauses, then regenerate REQUIREMENTS.tsv. No persistent
product state or wire migration is involved. Owner acceptance is decision 0512. Normal staging and
any later commit-bound dogfood/integration remain outside this uncommitted result.
