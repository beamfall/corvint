# Trusted PR test driver

`corvint-pr-tests` is an explicit CI executor, separate from the read-only planner. It calls a
trusted `corvint affected --base FULL_OID` and the existing `gate-affected-select`; advice
strings are never executed. Only root-module Go selection is admitted. Missing qualification,
invalid event topology, profile drift or planner/selector failure executes `./...` visibly.
The workflow currently has empty tool/artifact pins and therefore runs the full suite.

Build all three binaries from one reviewed clean immutable tool source outside the tested
checkout with Go 1.27.1 and `go build -trimpath -buildvcs=false`. Pass their absolute paths and
that source OID. The exact CLI is discoverable with `corvint-pr-tests -help`.

```sh
corvint-pr-tests --root /owned/tested-merge --base "$EVENT_BASE" --head "$EVENT_HEAD" \
  --target "$MERGE_OID" --source "$TOOL_SOURCE_OID" \
  --planner /trusted/bin/corvint --selector /trusted/bin/selector \
  --out /owned/pr-result --qualification /trusted/qualification.json \
  --qualification-sha256 "$REVIEWED_ARTIFACT_SHA256"
```

Use a fresh owned output directory outside every tested checkout. The driver owns its build
cache and an empty read-only module cache; external module dependencies are unsupported and
cannot qualify. `go test -json -p 1 -count=1 -race -timeout 50m` is fixed, under a 70-minute
command bound. No `go run`, `eval`, advice execution, PR secret, persisted credential or
`pull_request_target` is used. Child process groups are killed on return or interruption.

Freeze and measure before the full historical campaign:

```sh
corvint-pr-tests --mode freeze --root /owned/history --target "$CORPUS_END_OID" \
  --source "$TOOL_SOURCE_OID" --planner /trusted/bin/corvint \
  --selector /trusted/bin/selector --out /owned/shadow
corvint-pr-tests --mode shadow --rows 1 --root /owned/history \
  --source "$TOOL_SOURCE_OID" --planner /trusted/bin/corvint \
  --selector /trusted/bin/selector --corpus /owned/shadow/corpus.json --out /owned/shadow
# After measured resource/cleanup review, use the same invocation with --rows 200.
corvint-pr-tests --mode qualify --root /owned/history \
  --corpus /owned/shadow/corpus.json --out /owned/shadow
```

The runner uses one owned clone, serial rows and a shared owned build cache. Complete rows may
resume only after source/profile and all raw-file hashes revalidate. Invalid rows are retained
and fail the campaign; they are never replaced. One full invocation per row compares actual
failing packages with the previously captured selected set. This does not claim 200 selected
invocations. Missing package terminal events, infrastructure failure or build failure is invalid;
a complete assertion/race failure is a valid comparison, and any omitted failing package kills
promotion. A wholly green corpus provides limited counterfactual evidence.

Keep the corpus, each row's raw files and `row.json`, and `qualification.json` outside source.
After independent review, place the qualification JSON in a separate artifact commit at
`.github/qualifications/pr-go-race.json`; pin that commit and the file's SHA256 in the trusted
workflow, alongside the earlier tool-source commit. Review the complete raw archive before
admitting its digest. Only the exact matching Go binary, platform, OS release, compiler, driver,
planner and selector can narrow. The artifact pins evidence; it grants no repository authority.
Do not claim Linux or hosted qualification from the local macOS fixture.

Protected workflow/ruleset status: **VERIFIED** (2026-09-19). The repository workflow and literal
pins alone do not protect their own control plane. Decision 0320 replaces the required-workflow
policy the free plan lacks: `ci-control-plane.yml` (AFP-V0-016) fails any PR that changes
`.github/`, and the `main` ruleset requires it. Decision 0390 removes the admin bypass: such a PR
merges only after an admin posts a `ci-control-plane` `success` status on its exact reviewed head
SHA, and `doc-gates` joins the required checks. As recorded on 2026-09-19, before decision 0390,
ruleset 23699808 was active on the default branch: a pull request with 0 approvals, merge commits
only (a squash drops the bound change's sha), `require_extra_approval_for_unattributed_changes`
false (the GitHub default of true blocks a solo repository), required checks `go-product` and
`ci-control-plane`, and the admin role as the only bypass actor, in pull-request mode. The check
posted `success` on PR #26 (run 35444060752) and PR #24 (run 35446378936); its `failure` path has
not yet run on a real PR. Keep pins empty until `pr-tests-qualification.yml` (AFP-V0-017)
produces a PASS.
The runtime environment is an allowlist with exact recorded bytes, a fixed absolute Go PATH,
`/usr/bin/cc`, `GOENV=off`, `LANG=C`, `LC_ALL=C`, `TZ=UTC`, and exclusively owned HOME/TMP/cache
under `/tmp/corvint-pr-tests-runtime`. An existing runtime path is refused; owned runtime state is
removed on return; cleanup failure is reported with a nonzero exit. Output/runtime paths are
resolved through their nearest existing ancestors before creation and rechecked as real external
directories, so a symlinked parent cannot create inside the tested tree. Historical rows use that same fixed layout and reset HOME/TMP between rows.
Stdout is capped at 128 MiB and stderr at 8 MiB; overflow immediately cancels the process group
and invalidates execution. Hosted log preview is capped at 1 MiB/256 KiB. Planner/selector drift
at launch falls back to full; Go/compiler/environment drift fails safely. Freeze ignores grafts;
row reuse binds the execution source/environment and raw plan/audit hashes as well as outcomes.

## Shared Linux container preparation

`--mode container` is an optional native Windows/Linux launcher. It never starts Docker,
pulls an image, switches context, publishes artifacts, or launches the corpus automatically.
The exact official image is:

```
docker.io/library/golang@sha256:eef6a67266eeed3c86dd47fd01b32faa8bf0229eb83eb3d4d466e80391bd3820
```

This retained `corvint-pr-container/1` profile and its evidence reader remain exact Go 1.27.0.
The current native driver requires Go 1.27.1 under decision 0293 and is not qualified to execute
inside this image. A current-source container migration needs a separately reviewed image and
new measurements; no image or qualification is supplied here.

The historical path requires a dedicated full Git clone, the frozen corpus, and three separately
supplied Linux binaries named `corvint`, `gate-affected-select`, `corvint-pr-tests` from one reviewed
immutable source compatible with the retained image. The launcher hashes those files, mounts them
read-only, and invokes `/trusted/corvint-pr-tests` with that source identity; it does not build them.
Historical preparation used Go 1.27.0 with `-trimpath -buildvcs=false`. This is not an instruction to
build the current source with Go 1.27.0, nor qualification of a current launcher/legacy tools pairing.
Trusted execution source and tested target remain separate.

First historical job (arguments shown as placeholders, supplied directly without shell eval):

```
corvint-pr-tests --mode container --container-mode shadow --docker-context desktop-linux --trusted TRUSTED_BIN_DIR --root DEDICATED_CLONE --source FULL_TRUSTED_SHA --corpus CORPUS_JSON --row 1 --out NEW_EVIDENCE_DIR
```

Use an existing explicit Docker context on Linux instead of `desktop-linux`. Output must be a
new directory outside the input, preferably D/E on Windows. Each call executes exactly one
unchanged corpus index, retains `row-NNN`, `inspect.json`, `profile/profile.json`, `export.json`
and `cleanup.json`, then removes only its container. An interrupted run fails and may lack raw
evidence; no complete-row claim is made. Failed cleanup is nonzero with a retained witness.
Optional missing plan/selector files have explicit export errors. Raw stdout/stderr retain the
existing 128 MiB/8 MiB bounds; archive export refuses symlinks and extra/path-traversal entries.

`--container-mode freeze --target FULL_SHA` produces the frozen 200-pair corpus. `--container-mode
run --base BASE --head HEAD --target MERGE` clones into owned storage and runs full unless the
separately supplied `--qualification` and `--qualification-sha256` match. `--container-mode
qualify --corpus CORPUS_JSON --evidence RETAINED_ROWS` reads the row directory read-only and
writes qualification separately. That mode executes no tested code. Non-container shadow keeps
its existing `--rows` interface; `--row N` chooses exactly one index and is exclusive with --rows.

The 14 GiB tmpfs is memory-backed and counts against the same 8 GiB container memory limit;
2 CPU/GOMAXPROCS=2 and cold GOCACHE apply to both campaign and PR execution. Actual Windows
Docker mount-path spelling, image/tool runtime verification, selected/fallback fixture,
interrupted-container cleanup, memory/filesystem measurements and the first real historical
row remain NOT_RUN. If this bounded profile is too small, re-review storage rather than changing
limits or substituting an unbounded volume. Do not run 200 rows until the first-row evidence and
candidate freeze are accepted. Hosted execution remains NOT_RUN and workflow pins remain empty.

Container control operations have a two-minute deadline within an 80-minute launcher ceiling;
the test operation keeps its 75-minute bound. Docker daemon logging is disabled and inspected,
so PID1 file descriptors cannot bypass the retained-output bound. Both historical and PR
container checkouts use `/work/checkout`; process capture cannot bypass limits via io.Copy.
Run export requires only its actual selection/execution/Go outputs, not historical row files.
