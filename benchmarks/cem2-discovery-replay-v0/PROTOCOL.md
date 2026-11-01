# CEM 2.0 discovery replay: pre-registered protocol (V1-2600)

Status: **pre-registered, not run.** Thresholds X = 0.15 and Y = 0.60 were set by the owner on 2026-11-01 (section 6).
Contract: [`docs/specs/cem2-discovery-replay-v0.md`](../../docs/specs/cem2-discovery-replay-v0.md)
(`VRP-V0-001..099`). Partition: **held-out** under the first-observation rule of
[`benchmarks/README.md`](../README.md#partitions-and-first-observation-rule). No replay output
existed when this file, the manifest and the harness were committed; nothing below was tuned to a
result.

## 1. Question and arms

Does a typed per-unit discrimination label (V1-2602 vocabulary) flag the unit a later defect fix
touched more often than a coverage-diff baseline, without an unacceptable false-alarm rate? Kill or
continue the CEM 2.0 umbrella on the answer. Nothing ships from this experiment.

- **Unit**: one non-test Go source hunk (`*.go`, not `*_test.go`) of the change's canonical CEM
  patch (`cem prepare`, cem/1.0, myers, three context lines). Other hunks are outside every metric.
- **Baseline arm**: coverage-diff over the oracle test set (section 4.1).
- **Typed arm**: a prototype label folded from the existing `cem verify --hunk-revert` and
  `cem discriminate` witnesses plus the same coverage profile (section 4.3).

## 2. Change set (`VRP-V0-010..014`)

**Rule (verbatim in the manifest's `selectionRule`)**: population = non-merge commits F on
`origin/main` at `b7ffc233396da54e2ac5a094ec314dc3b067f6fb`, excluding the root snapshot
`9477995d7`, whose subject matches `^fix(\(scope\))?!?:` and names at least one ticket or
requirement ID. F's touched lines are the old-side code lines (not blank, not a `//`, `/*` or `*`
line) of its `-U0` myers diff over non-test `.go` files, with inter-hunk context 0, no textconv,
no external diff and no renames, and no system or global Git configuration read (`VRP-V0-056`); a
pure insertion of code touches its two neighbouring old lines; a hunk whose removed and added lines
are equal line by line, in order, once every ID is masked only renumbers IDs and touches nothing
(`VRP-V0-060`). Each touched line is blamed at F^ (`git blame --porcelain --no-textconv`, no
`-M`/`-C`, no ignored revisions). The linked change P is the blamed commit sharing an
ID with F's subject that owns the most touched lines (ties: earlier committer date, then SHA); P
qualifies with 1..60 non-test Go source hunks. One row per P (the earliest F is the fix, later
ones are `laterFixes`). Rows are ordered by P's committer date; above 50, 40 are taken at
`floor(i*n/40)`.

**Result**: 255 `fix` commits carrying an ID, 52 qualifying changes, **40 selected**
(`VRP-C001..VRP-C040`), 429 U0 source hunks in total. Every selected P landed through a
`claude/*` lane branch (`changeLandedSubject`), so every change is an agent change.

**Manifest**: [`selection-manifest.json`](selection-manifest.json), produced by
`python3 -I select.py REPO b7ffc233396da54e2ac5a094ec314dc3b067f6fb --repository-label
https://github.com/beamfall/corvint`; two runs were byte-identical.

**sha256(selection-manifest.json) = `876c890b15b3403ceca42abce3a7837585360b47ee1970d9066eb46043383515`**
(also in [`selection-manifest.sha256`](selection-manifest.sha256); `harness.py check` refuses a mismatch).
The review repair of 2026-11-01 replaced `5a51e33a42dc6a329e9e3f90350653b395b6270f21953df0b90eee5b5cd29cda`:
only the `selectionRule` text changed; every row, count and the selected set are unchanged.

**Registration** (`VRP-V0-055`): [`registration.sha256`](registration.sha256) lists, in
`sha256sum` format and fixed order, this file, `thresholds.json`, the manifest, its digest file,
`select.py` and `harness.py`. Its own sha256 is the registration digest. `check`, `run` and `score`
exit 5 when any listed file differs; every capture's `run.json` records the digest, and `score`
refuses captures made under another one. The digest is recorded in the build log, not here, since
this file is one of its inputs.

Per change the manifest records `change` (P), `base` (P^), `fix` (F), both subjects, `sharedIds`,
`qualifies` (why), `touchedLinesBlamedToChange`, `touchedLines` ([path, line in P] — the ground
truth mapped onto CEM hunks at score time), `touchedHunks` (the U0 view), `laterFixes` (each with
its own touched lines), and the landing merge of P and F.

**Disclosed limits of the set**:

- **Escape stratum**: all 40 are `escape: same-merge` — the fix landed in the same first-parent
  merge as P (independent review, integration or CI repair before main). The public `origin/main`
  history starts 2026-10-29 and holds 39 first-parent merges, so no post-merge escape qualified.
  The post-merge stratum is **NOT_PRODUCED** from this history. These are still defects P's own
  tests let through; they are not field escapes.
- **External repository**: `beamfall/core` — **selection NOT_PRODUCED**: no clone exists on the
  preparing host (Unraid; `find /home/agent -maxdepth 3 -name core -type d` is empty). The AC
  requiring at least one external repository is unmet until the owner names a clone and
  `select.py` is run on it as a second, separately digested manifest committed before any run.
- Linkage is by blame plus a shared ID; a fix that rewrote lines another commit last touched is
  attributed to that commit or dropped. Touched lines in files F deleted are not counted.
- Some fixes are non-behavioural for a test oracle (help text, error wording). They stay in: the
  experiment measures exactly whether either arm can see such a unit.

## 3. Tool pin (`VRP-V0-023`)

The run uses one Corvint build for all changes, recorded per change in `run.json`
(`corvintVersion`, `goVersion`). Prepared with `Corvint 1.0.0-rc.3.dev (build 39, commit
b7ffc2333)`. Historical changes are judged by today's verifier and runners; each change's own
toolchain pin applies only through `go test` in its worktree.

## 4. Measurement

### 4.1 Oracle tests and coverage-diff baseline (`VRP-V0-021`, `VRP-V0-030`)

Oracle test set O(P) = every `_test.go` file at P in the directory of each changed source file.
The baseline is one `go test -count=1 -coverprofile=cover.out -coverpkg=<dirs> <dirs>` at P
(timeout 1800 s), recorded on the map by `cem cover` (help: `cmd/corvint/help.go:1443`,
`:1493-1503`; workflow `internal/cem/workflow/cover.go:36`). Per unit, an added line is
*executable* when a coverprofile block spans it and *executed* when such a block's count is > 0.

- **Primary baseline flag**: at least one executable added line is not executed (diff-coverage).
- **Secondary baseline flag** (reported, not decisive): executable added lines exist and none ran.
- No executable added line: not flagged. File absent from the profile: `NOT_OBSERVED(not-instrumented)`.

### 4.2 Citation and runs (`VRP-V0-022`, `VRP-V0-024..026`)

In a disposable detached worktree at P: `cem prepare --base P^ --target P` builds the map; each unit
cites every O(P) file **that exists at P^** in its directory as a whole-file `test-claim`
(`cem cite --lines 1:N`); a unit with none, and every non-unit hunk, is `cem mark --disposition
unknown`. O(P) files added by P are passed to discrimination as `--same-change-test` (co-authored
oracle, TCQ-V0-071) and cannot be cited at base. Maps stay repository-relative in the worktree
(CEM-V1-647, `cmd/corvint/help.go:1660-1663`) and are copied out after each step.

Capture safety (`VRP-V0-057`, `VRP-V0-058`, `VRP-V0-061`): the module path is read from `go.mod`
at P (`git cat-file`) before any step runs. `run` refuses (exit 4) a work or output directory that
overlaps the repository, its Git common directory or the other directory, after resolving symbolic
links. A change whose worktree path already exists is recorded `refused: worktree-path-exists` and
the existing path is left untouched; the harness removes only a worktree it created, in a `finally`
block, so a failure or interrupt never removes another tree. Each step runs in its own process
group; on exit, timeout or interrupt the whole group is killed and the harness waits, at most 10 s,
until the group is empty (`groupRetired` in `run.json`).

Existing machinery and its real bounds (no new mutation code is written for this experiment):

| Witness | Command | Operators / bound | Source |
|---|---|---|---|
| hunk revert | `cem verify --hunk-revert` | reverse-apply one test-cited Go hunk, rerun its named tests; results `held` / `survived` / `not-run` + closed reasons (`revert-breaks-build`, `timeout`, ...) | `internal/cem/verify/hunkrevert.go:25-47`; bounds 300 s per run, 1200 s total, `hunkrevert.go:68-69`; help `cmd/corvint/help.go:1640-1647`; flag `internal/cem/cli/cli.go:115` |
| mutation | `cem discriminate` | closed Go operator set `negate-condition`, `swap-binary` (`== != < >= > <= + - && \|\|` table), `replace-literal`, `delete-statement`, changed lines only, first-kill per test | `internal/liveverify/mutate/plan.go:14-33`, `:177-203`; runner `internal/cemdiscriminate/cemdiscriminate.go:94`; defaults 8 hunks / 8 mutants / 10 m `internal/cem/workflow/discriminate.go:36-38`; help `cmd/corvint/help.go:1444-1446`, `:1506-1520`; states `internal/cem/wire/map.go:127-129` |
| (reference) | `prove --base --mutate` | same Go mutator, at most 8 mutants per row, 10 m row and 30 m invocation budgets | `cmd/corvint/help.go:1164-1165`, `:1227-1238`, `:1268-1281`; `internal/liveverify/mutate/mutate.go:132`, `:140` |

**Budget (frozen)**: `--max-hunks` = the change's U0 source-hunk count + 64 (every unit is a
candidate), `--max-mutants 8` (the existing cap), `--wall-time 1800s` per change; hunk-revert at its
fixed 300 s / 1200 s. A unit not judged inside the budget is `NOT_OBSERVED`, never extrapolated.
`prove --mutate` is cited for operator provenance only; it is not run (it judges test rows, not
hunks). Upper bound on wall-clock: about 40 x (30 m coverage + 20 m revert + 30 m mutation) ≈ 53 h;
run it on WSL or the Windows benchmark box, not on a loaded lane host.

### 4.3 Typed label fold (`VRP-V0-031..033`)

First match wins, per unit:

1. hunk-revert `held` → `discriminated(hunk-revert)`
2. discriminate `discriminates` → `discriminated(mutant-kill)` (a kill by a same-change test is
   recorded as such in the witness detail and reported as a separate count)
3. hunk-revert reason `revert-breaks-build` → `type-discriminated`
4. baseline secondary flag (no executable added line ran) → `uncovered(no-executed-line)`
5. no executable added line → `uncovered(no-executable-line)`; not in profile → `uncovered(not-instrumented)`
6. hunk-revert `survived`, or discriminate `survived` with ≥ 1 mutant → `weak-test`
7. otherwise → `NOT_OBSERVED(<reasons>)`

**Typed flag** = `weak-test` or `uncovered(no-executed-line)`. `likely-equivalent` is
**NOT_PRODUCED**: no equivalence screen (TCE or other) exists in `internal/liveverify`,
`internal/cemdiscriminate` or `internal/cem` (searched 2026-11-01); surviving mutants that are
equivalent therefore inflate `weak-test`, which biases the typed arm *toward* false alarms. The
V1-2602 rule "a co-authored test discriminates only if it fails at base" is also not implementable
with today's machinery; same-change kills are counted separately so the secondary analysis can
exclude them.

## 5. Results per change and NOT_OBSERVED (`VRP-V0-040..042`)

`harness.py score` writes one row per change: units, touched units, baseline flag and secondary
flag per unit, typed label per unit, whether the fix touched a flagged unit (per arm), flagged
units, false alarms, and wall-clock (`replaySeconds` = prepare through discriminate, monotonic
clock). Touched units = units whose new range holds a manifest `touchedLines` entry.

- A unit witness that is missing, timed out or not run is `NOT_OBSERVED(reason)`, never estimated.
- Capture per arm is `true` when a touched unit is flagged, `false` only when every touched unit
  is observed and unflagged, else `NOT_OBSERVED`.
- A **false alarm** is a flagged unit touched by neither the fix nor a `laterFixes` entry.
- The CEM 1.0 dogfood cost comparator is `benchmarks/dogfood-measure` (`wallNs`,
  `corvint-dogfood-measure/1`) on the same host and changes; unless that is run, `dogfoodSeconds`
  stays `NOT_OBSERVED` and no cost ratio is claimed.

## 6. Decision rule (`VRP-V0-050..054`)

Over **paired** changes (both captures observed):

- `b` = typed captures and baseline does not; `c` = the reverse; `delta = (b - c) / paired`.
- `FA_typed` = Σ typed false alarms / Σ typed-flagged units (pooled over observed changes).

**Validity**: `paired ≥ 0.8 × selected` (32 of 40). Below it the run is `INVALID_RUN`: one rerun is
allowed after an infrastructure-only repair (no change to the rule, units, labels or budget);
a second `INVALID_RUN` is recorded as **KILL** (the machinery cannot label the population).

**Verdict**: **CONTINUE** iff `delta ≥ X` **and** `FA_typed ≤ Y` **and** `b > c`; otherwise **KILL**.
Secondary measures (secondary baseline, per-change flagged units, same-change-kill exclusion,
cost against dogfood) are reported and never change the verdict.

| Threshold | Value | Status |
|---|---|---|
| **X** — minimum escaped-defect capture gain of typed over baseline (`delta`, fraction of paired changes) | **0.15** | set by the owner 2026-11-01 (the proposed default) |
| **Y** — maximum typed false-alarm rate (`FA_typed`) | **0.60** | set by the owner 2026-11-01 (the proposed default) |

The owner sets both in [`thresholds.json`](thresholds.json) and in this table in one commit before
any `run`; `harness.py run` and `score` exit 3 while either is not a number, exit 5 when a
registered file differs from `registration.sha256`, and exit 4 when the executing harness is not the
one inside `--repo` or a registered file is untracked or differs from that repository's HEAD.

Proposed-default reasoning (for the owner; not binding): with 40 paired changes, `X = 0.15` means
at least 6 more changes caught by typed than by baseline net of the reverse; with `c = 2`, `b = 8`
gives a one-sided sign-test p ≈ 0.055, so a smaller X cannot be told from noise at this N. For Y,
the 40 changes hold 429 U0 source hunks of which 88 are touched (10.7 and 2.2 per change), so a flagger choosing units
at random has a false-alarm rate near 0.8; `Y = 0.60` requires typed flags to be clearly better
than random while tolerating the known `weak-test` inflation from the missing equivalence screen.

## 7. Runbook (`VRP-V0-034`)

```sh
H=benchmarks/cem2-discovery-replay-v0
python3 -I $H/harness.py check                               # digest + threshold status
python3 -I $H/harness.py selftest                            # synthetic fold/rule checks only
python3 -I $H/harness.py plan --repo "$PWD" --work "$HOME/vrp/work" --out "$HOME/vrp/out" --corvint "$(command -v corvint)"
# after the owner commits numeric X and Y:
python3 -I $H/harness.py run   --repo "$PWD" --work "$HOME/vrp/work" --out "$HOME/vrp/out" --corvint "$(command -v corvint)"
python3 -I $H/harness.py score --repo "$PWD" --out "$HOME/vrp/out" > "$HOME/vrp/results.json"
```

`run` keeps the first observation: a change whose output directory exists is skipped, never
overwritten. Work and output directories must be outside the repository and its Git directory,
and must not overlap each other. `score` binds every artifact to its change before folding labels
(`VRP-V0-059`): `run.json` must name the change, its commit, its base and the registration digest;
the artifact files and their sha256 must equal `run.json`; every map's `baseRevision` and
`patchSha256`, the hunk-revert `expectedBase`, `targetRevision` and `patchSha256`, discrimination's
`treeRevision`, and coverage's `profileSha256` and `testRun` must match the change and its capture;
hunk ids must agree across maps; and all captures must come from one Corvint build and Go version.
Any mismatch exits 6. The verdict and every
per-change row go into a build-log entry (`docs/build-log/YYYY-MM-DD-v1-2600-replay.md`).
