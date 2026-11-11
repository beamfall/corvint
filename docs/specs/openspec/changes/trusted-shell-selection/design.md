# Design: `trusted-shell-selection`

Line numbers are those of `b55635262788687c3eea9a08a94378e4ab8856a4`.

## Which steps run candidate code today

On a `pull_request` or `merge_group` run every job checks out the tested merge commit and runs its
own `ci.yml`. Steps that execute code from that checkout:

| Job and step | Candidate code it runs | Does it choose what runs? |
|---|---|---|
| `doc-gates`: `make -s doc-gates` (`ci.yml:180`) | `Makefile` (`DOC_GATE_TARGETS`, `doc-gates`, `doc-gate/%`), `script/doc-gates-summary.sh`, every gate script, `go run ./conformance/use-cases-v0` | yes: the target list |
| `doc-gates`: `make -s ai-coauthor-trailers-check` | `Makefile`, `script/check-ai-coauthor-trailers.sh` | no (one fixed check) |
| `shell-test-selected` (`ci.yml:221`) | `Makefile` `shell-test-affected`, candidate-built `cmd/corvint` and `tools/shell-test-select`, the selected step recipes | yes: planner, selector and the `shell-test:` list |
| `go-static`, `go-product-shard` Go tests, `web-flows`, `go-live`, `artifact-integrity` | the candidate's code under test | no: each runs a fixed, workflow-defined set (the shard set is chosen by trusted tools, CIN-V0-011) |
| `go-product-shard` `flaky.sh`, `outcomes.sh` | `.github/forward-rows` scripts | they judge, not choose; they sit under `.github/`, so the `ci-control-plane` waiver holds any edit (CIN-V0-8720 reads its authorizer from the base too) |
| `docs-plan`, `shard-plan`, narrowing gate | trusted tools from `corvint-pr-bin` (`ci.yml:614-633`, `1063-1084`) and `.github` scripts | yes, but already trusted |
| `cadence-plan` | the event base's `.github/ci-cadence` (sparse checkout of the base) | yes, already trusted |

So the two candidate-chosen selections are the shell-test selection and the doc-gates target list.
Everything under `.github/` is already held by the waiver, which a pull request cannot bypass.

## Decisions

### D1: the shell-test planner and selector are built from the event base (CIN-V0-8724)

The `shell-test-selected` step stops calling `make shell-test-affected`. It:

1. sets `B` to the event base
   (`github.event.pull_request.base.sha || github.event.merge_group.base_sha`) and
   `base=$(git merge-base HEAD "$B")`, as today;
2. adds a detached worktree of `B` under `$RUNNER_TEMP/trusted-select` and builds
   `go build -trimpath -buildvcs=true -o "$RUNNER_TEMP/trusted-select-bin/" ./cmd/corvint
   ./tools/shell-test-select` there (the checkout is full depth, so `B` is present);
3. plans the tested tree with the base-built planner:
   `corvint --root "$PWD" affected --accept-stale-binary --base "$base"`, and admits the plan only
   with no degradation or exactly `self-binary-stale` (the CIN-V0-8722 rule: the planner's commit is
   `B`, not the tested tree, which is the point); any other degradation is FULL;
4. runs the base-built `shell-test-select --plan plan.json --makefile M` twice: once with
   `M = git show "$B:Makefile"` and once with the candidate `Makefile` (D2).

Why the event base and not `main` or the narrowing pin: the event base is what the pull request is
judged against everywhere else (cadence, CIN-V0-8720), it is always in the full-depth checkout, and
a queue entry's base is the commit that is already on `main`. The planner is stamped
(`-buildvcs=true`) so its staleness is visible, not hidden (CIN-V0-8722's reasoning).

### D2: the steps run are a union, and a removed step forces FULL (CIN-V0-8725)

`selected = select(plan, base Makefile) ∪ select(plan, candidate Makefile)`. A step the candidate
adds can only be found in the candidate list; a step the candidate drops from `shell-test:` is still
found in the base list. Every selected target that the candidate `Makefile` still defines runs
(`make --keep-going TARGETS` in the candidate tree, since a shell test checks candidate scripts).
A base-selected target the candidate no longer defines cannot run; the step prints
`::warning::CIN-V0-8725: shell-test step NAME of the base is gone in this change; running every
shell-test step` and the job runs FULL: every step of both lists that the candidate defines. The
summary line names the removed steps, so the owner reviews the removal. The job does not fail on a
removal: a legitimate removal must be able to merge.

A pull request that legitimately changes the selector or the planner gets its new code tested by
the candidate's Go tests (`tools/shell-test-select`, `cmd/corvint`) in the shards; its own
shell-test job still selects with the base's selector, and the summary says so
(`planned by the event-base selector at B`). The new selector takes effect for the next pull
request after it merges. A change to the `shell-test:` list is the D2 union; a change to
`Makefile` already makes the selector print every step (AFP-V0-2401 "names the Makefile as dirty"),
now judged by the base-built selector.

### D3: when the base cannot build its selector (CIN-V0-8724)

If the worktree cannot be added or either build fails, the step never uses the candidate's
selector. It runs FULL over the union of both lists' steps that the candidate defines, with
`::warning::CIN-V0-8724: the event base B does not build its shell-test selector; running every
shell-test step`. Listing the steps then needs no Go: the `shell-test:` prerequisites of both
`Makefile`s are read by a short `awk` in `ci.yml` itself (held by the waiver), which the
CIN-V0-8727 test checks against `ReadSteps` on the real `Makefile`. A base that does not build is
rare (it passed its own run, or it is `main`); the cost is latency, never coverage.

### D4: doc-gates runs the base target list too (CIN-V0-8726)

`make -s doc-gates` stays as it is (the candidate list, so an added gate runs). A following step
reads `DOC_GATE_TARGETS` from `git show "$B:Makefile"` with the same `awk` reader (the variable is a
`:=` list of one word per continuation line) and from the candidate `Makefile`, and runs
`make --keep-going` on every base gate that is missing from the candidate list but still defined by
the candidate, each through its own target name (not `doc-gate/%`, which the candidate may
redefine). A base gate the candidate no longer defines is printed as
`::warning::CIN-V0-8726: doc gate NAME of the base is gone in this change` in the log and summary.
The step runs only when `B` is set (pull request and queue); on a push the lists are equal by
construction. The candidate's `doc-gates` recipe, `doc-gates-summary.sh` and the conformance run
are unchanged; the extra step only closes "drop it from the list".

### D5: testing criterion 3 without a hosted run (CIN-V0-8727)

A Python extract-and-run test under `.github/forward-rows/`, run in `go-static` beside the
CIN-V0-8720 tests, takes the two steps' scripts from `ci.yml` and runs them in a fixture repository:

- the fixture module has a tiny `cmd/corvint` (prints a fixed affected-plan/1 naming one shell test
  unit) and `tools/shell-test-select` (prints the step that runs it), which build offline in about a
  second each, and a `Makefile` whose steps write marker files;
- base commit `B` holds those; the candidate commit edits `tools/shell-test-select` to print
  nothing. The extracted step must still run the base-selected step (its marker exists);
- further candidates: drop the step from `shell-test:` (still runs); delete the step's target (FULL
  with the CIN-V0-8725 reason); make `B` fail to build (FULL with the CIN-V0-8724 reason);
- doc-gates: a candidate that drops `gate-b` from `DOC_GATE_TARGETS` still runs `gate-b`; one that
  deletes `gate-c` prints the CIN-V0-8726 reason;
- control: the same step script with the old `make shell-test-affected` body selects nothing for
  the edited selector, so the test fails on the base workflow.

The `awk` reader is also run on the repository's own `Makefile` and compared with
`ReadSteps` / `DOC_GATE_TARGETS` as `make` expands it, so the reader cannot drift silently.

## Landing

The change touches `.github/workflows/ci.yml` and adds a test under `.github/forward-rows/`, so it
is one `.github`-only pull request (ASD-V0-706), held by the `ci-control-plane` waiver. The spec
requirements land first in a separate code pull request. `github-guard.sh` treats the new test file
as an unmapped, so global, input (as for the CIN-V0-8720 tests); `tools/ci-forward-row`'s
`guardUnmappedInputs` names it in the code pull request.

## Open questions for the owner

1. Should a removed shell step or doc gate fail the job instead of FULL plus a warning? This design
   chooses not to block, so a legitimate removal can merge; the owner may prefer a required waiver.
2. Should `doc-gates` also refuse a candidate that redefines a base gate's recipe? This design
   leaves gate implementations to the candidate (non-goal).
