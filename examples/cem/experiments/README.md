# Experimental criterion experiments

This optional companion explores a possible CEM 0.4 direction. It is **not a CEM
0.4 wire release**. Existing CEM and Tasks schemas and default gates are unchanged.
The proposed contract is
[CEM criterion experiments V0](../../../docs/specs/cem-criterion-experiments-v0.md).

The packet connects every native task criterion to an exact named Go test, an
independently anchored and reviewer-attested oracle, canonical CEM hunks, and
registered incorrect implementations. `run` constructs immutable source snapshots
and executes a fixed, bounded Go command. `verify` checks historical artifacts
without running tests. `gate` additionally checks that the packet still applies
to the live submitted Tasks attempt and clean candidate tree.

Tests are trusted executable code. The clean offline environment is not an OS
security sandbox. Logs and source can contain private data; keep output local.
Prefer a fresh private output directory outside the repository; output selected
inside it can dirty the worktree and make the live gate refuse.
Execution remains `CALLER_REPORTED`, oracle relevance `REVIEWER_ATTESTED`, and
test adequacy and product improvement unproven.

## Run a disposable lifecycle

Build both companions from this checkout with Go 1.27.1. Use a fresh scratch
directory for binaries and qualification output; the demo refuses an existing
`--out` directory. Replace the paths below with absolute paths on your machine.

```sh
GOTOOLCHAIN=local go build -o /tmp/cem-demo-tasks ./cmd/corvint-tasks
GOTOOLCHAIN=local go build -o /tmp/cem-demo-experiments ./cmd/corvint-cem-experiments
GOTOOLCHAIN=local go test -json -count=1 -timeout 30m \
  -run '^TestCALV0019_' ./internal/tasks/store > /tmp/cem-demo-qualification.jsonl
```

Check the qualification command's exit status. Never substitute synthetic passing
events. Then run:

```sh
python3 examples/cem/experiments/demo.py \
  --source /absolute/path/to/corvint \
  --core /absolute/path/to/corvint-binary \
  --tasks /tmp/cem-demo-tasks \
  --experiments /tmp/cem-demo-experiments \
  --go /absolute/path/to/go \
  --qualification /tmp/cem-demo-qualification.jsonl \
  --out /tmp/cem-demo-new-directory
```

The script creates its own nonfixture queue, performs actual native initialization
and execution cutover, creates and claims a ticket, records eight test scenarios,
submits the candidate, runs the companion through a native `COMMAND` gate,
integrates locally in the disposable repository, completes the ticket, and reads
back its receipt audit. It then reopens and refines only that disposable ticket,
verifies historical evidence after completion and acceptance change, and confirms
that a terminal or changed attempt cannot reuse it as a live gate. The submitted
dirty-tree refusal is isolated while the attempt is live; the acceptance-change
case also has a terminal attempt, as recorded in the summary.
It merges only inside the new disposable repository.

`summary.json` retains the exact native observations and packet digests. Numbered
stdout/stderr files retain private command output. This demonstrates feasibility;
it does not qualify a production queue or establish superiority over strong tests.

## Supply another trusted experiment

`plan --repo REPO --request REQUEST.json --out NEW_DIRECTORY` validates and
captures canonical task, attempt and CEM inputs. The request's closed schema is
defined in `internal/criterionexperiment/schema.go`; the disposable demo writes
a complete `request.json` example. The initial profile permits only committed
standard-library-only Go modules, literal package directories and exact top-level
test names. An immutable oracle file must declare the selected test, and its
independent anchor must contain the exact acceptance criterion text.

Review the plan, oracle, registered controls and execution sources before approving
its returned canonical digest:

```text
run --repo REPO --plan PLAN/plan.json --approve EXACT_SHA256 \
  --experimental --trusted-local --out NEW_RUN_DIRECTORY
verify --repo REPO --plan PLAN/plan.json --receipt RUN/receipt.json
gate --repo REPO --plan PLAN/plan.json --receipt RUN/receipt.json
```

Verification can preserve an unresolved record, such as a surviving control; the
live gate requires all selected criterion obligations to be satisfied. A digest
does not establish semantic correctness. The proposed three-arm external outcome
study in the spec remains `NOT_RUN`; its cohort and thresholds need owner acceptance
before promotion.
