# V1-2766 boundary-mutant replay: preregistration

Status: preregistered 2026-10-27. Ticket V1-2766. Nothing in this design has been run yet: no
mutant, selector or test. This file and the cohort draw are committed together. Their sha256 values
are recorded in the pilot receipt before the first mutant is generated. Any departure goes in a
hashed amendment written before the affected mutants run. Results are reported unrepaired, null
and negative ones included.

## Question

Does `corvint affected` (A) catch regressions that changed-packages-only selection (B) misses? It
asks this for representative changes and, separately, for changes at caller-visible boundaries.
The interim replay (prereg sha256 `172496b5c36f0f71e3e8d063942e02b97f7509faa43d35f19bf2de5122156507`,
amendment 1) cannot answer it. Its mutants are mostly caught inside the changed package, its
ground truth stops at A, its stages stop at the first kill, and its 12/12 bounds recall only
above about 0.76.

## Cohorts (frozen)

- **Universe.** Corvint public history at origin/main `98b7808110ca913640de97ff5581641d42e52120`.
  - Take the second parent of every first-parent merge (774 merges, the whole public history).
  - Drop the 15 commits drawn by prereg 172496b5.
  - Keep a commit only if it changes a non-test `.go` file under `internal/` or `cmd/`. That
    leaves 150 eligible commits.
- **Draw.** `perm = random.Random(20261027).sample(sorted(eligible), 150)`.
  - **Cohort R** (representative) is `perm[:75]`.
  - **Cohort C2** (boundary challenge) is `perm[75:]`.
  - Both lists, in draw order, are in `v2766-boundary-mutant-draw.json` (sha256
    `ef70d698a7ceba80b9414b96ede7478ea19b1a781d66714b73c36188fc2dd40f`).
  - The draw used read-only git and no selector or test output.
- **Mutants per commit.** One, so each commit is one independent cluster. The base is the commit's
  first parent, and the tree under test is the commit plus the mutant.
- **Changed functions.** A changed function is a function or method in a changed non-test `.go`
  file under `internal/` or `cmd/` whose body overlaps a diff hunk against the first parent.
- **Cohort R operators.** The four first-order operators of prereg 172496b5, applied to changed
  functions:
  - negate an `if` condition;
  - `==` to `!=`, or `<` to `<=`;
  - return the zero value instead of the first non-error result;
  - delete one non-declaration statement.
- **Cohort C2 operators.** Caller-visible boundary operators, applied only to changed code:
  - **BR (exported return semantics).** In a changed exported function or method, replace one
    non-error result of one `return` with its zero value.
  - **BS (serialization).** In a struct type the commit changed that has a `json` or `yaml` tag,
    either toggle `omitempty` on one tag or rename one tag key by appending `_x`. Alternatively, in
    a changed function, replace one literal default assigned to a struct field with its zero value.
  - **BI (interface behaviour).** In a changed method whose name is a method of an interface type
    declared in the module or the standard library, replace one result with its zero value or
    delete one non-declaration statement.
  - **BE (error propagation).** In a changed exported function, do one of two things:
    - inside `if err != nil { return ..., err }`, return `nil` in place of `err`;
    - replace `fmt.Errorf(... %w ..., err)` with `errors.New("v2766 mutant")`.
  - **BX (caller-visible side effect).** In a changed exported function, delete one statement of
    any of these kinds:
    - a statement that assigns through a pointer parameter or receiver;
    - a statement that writes an element of a map or slice parameter;
    - a statement that calls `Write`, `WriteString`, `Encode`, `Set`, `Add`, `Store` or `Close` on
      a parameter.
- **Choosing the mutant.**
  - Sort the candidates by (file, byte offset, operator).
  - Shuffle them with `random.Random("<sha>:R")` or `random.Random("<sha>:C2")`.
  - Take the first candidate whose package and test binary compile
    (`go test -count=1 -run '^$' <pkg>`).
  - A commit with no compiling candidate is recorded as NO_CANDIDATE and skipped. Its count is
    reported, because the prevalence of boundary candidates is itself a result.
- **Stage 0, before any test runs.**
  - Extend the mutant generator with the C2 operators, each covered by fixture tests.
  - Generate both cohorts' candidate lists and chosen mutants.
  - Hash them into the receipt.
- **Cohort H (context only).** Cohort H is the 30 real non-flaky Go failures from the 2026-10-26
  shadow-traffic study: B selected 19 of 30, while B plus the reader rules and the CI shadow
  selection selected 30 of 30.
  - Those outcomes were already seen, so cohort H is not blind and enters no verdict.

## Selectors

Both selectors are computed at the commit against its first parent, without the mutant. The
mutant lies inside changed files, so it cannot change either set.

- **A.** The Go packages selected by `corvint affected --base <first parent>`, using one installed
  build for the whole pilot. Its `corvint --version` is recorded, and it must contain the pin.
- **B.** Packages with a changed non-test `.go` file. Each runs whole.
- **Secondary, descriptive only: BR+.** B plus the A units whose witness is `DECLARED_READ_SCOPE`,
  `PATH_LITERAL_READER` or `UNBOUNDED_READER`. It shows whether A's rescues come from the dependency
  closure or from the reader rules.

## Ground truth

- **Setup.** All runs happen on the WSL runner through `heavy.sh`, with `GOTOOLCHAIN=local`,
  `go test -json -count=1 -timeout 30m`, and the same host for the whole pilot.
- **Baseline.** The unmutated commit, over A ∪ B. Tests that fail at baseline are excluded.
- **Mutated tree.**
  - Every B package runs whole, with no stop at the first kill.
  - The A \ B packages then run, restricted to those that can observe the mutant under the
    amendment-1 rule of prereg 172496b5: the test binary transitively imports the mutated package,
    or its tests invoke the go toolchain on that directory or `./...`.
- **Confirmation.**
  - A failure is confirmed when it fails at the mutant, fails again on one `-run '^Name$'` rerun,
    and passed at baseline.
  - A failure that passes on the rerun is FLAKY and excluded.
  - The ARTIFACT exclusion of prereg 172496b5 carries over (`TestSelectionOnTheLiveDirtyWorktree`).
- **Isolated full-suite sample.**
  - It covers the first 10 executed mutants of each cohort, in draw order.
  - Their baseline and mutated trees run every package listed by `go list` at that commit, passed
    as an explicit list.
  - This exposes two kinds of omission:
    - **A omissions:** kills visible only outside A.
    - **Pruning omissions:** kills in A packages that the observability rule skipped.

## Measures (per cohort, never pooled)

- **killed:** the mutant has at least one confirmed failure in the observed ground truth.
- **B-escape:** a killed mutant where B has no confirmed failure.
- **A-rescue:** a B-escape where A has a confirmed failure.
- **Rates:**
  - B-escape rate = B-escapes / killed.
  - A-rescue rate = A-rescues / B-escapes, NOT_DEFINED when there are no escapes.
  - Value rate = A-rescues / killed.
- **Intervals.**
  - Wilson 95% two-sided intervals throughout.
  - For a zero count, also the exact one-sided 95% upper bound 1 - 0.05^(1/n). For example,
    0/59 gives 0.050.
  - With one mutant per commit, the commit is the cluster.
- **Cost.** From the baseline run, the per-mutant ratio of B's summed package elapsed time to A's,
  reported as the median and the pooled sum ratio. Deployment savings come from cohort R only.
- **Enriched cohort.** C2 is enriched by construction. Its rates describe boundary changes and are
  never weighted into savings: the prevalence of such changes is NOT_OBSERVED except through C2's
  candidate count.

## Decision rule (frozen)

**Cohort R, the primary question:**

- **INCONCLUSIVE** if killed_R < 40 (underpowered).
- **ADDS_VALUE** if A-rescues_R ≥ 3.
- **NO_MEASURABLE_VALUE** if A-rescues_R = 0 and the one-sided 95% upper bound on the value rate
  is ≤ 0.06. That needs killed_R ≥ 49.
- **INCONCLUSIVE** otherwise, reported with the point estimate and bounds.

**Cohort C2, reported separately:**

- **BOUNDARY_RESCUES** if A-rescues_C2 ≥ 3.
- **NONE_OBSERVED** with its bound if A-rescues_C2 = 0.
- **INCONCLUSIVE** otherwise.

**Ground-truth check.** If the isolated sample shows a pruning omission on a B-escape, a
NO_MEASURABLE_VALUE verdict becomes INCONCLUSIVE. Any A omission is reported as a limit of A, not
as a rescue.

**Final sentence.** Whether `corvint affected` adds catch value over changed packages follows from
cohort R. C2 can add "on boundary changes" but cannot overturn R.

## Budget and stop rule

- **Cap.** A hard cap of 40 runner-hours of wall time, baselines included.
- **Order.** Cohort R runs in draw order until all 75 commits are done or 26 runner-hours are spent.
  Cohort C2 then runs in draw order until all 75 are done or the cap is reached.
- **Stopping.** At the cap, the current mutant finishes and the run stops. The receipt reports the
  number executed per cohort.
- **No reruns.** No cohort is extended or rerun after its results are seen.
- **Power.** At the interim replay's kill rate of 12 of 18, 75 R commits give about 50 killed. Zero
  rescues in 50 bound the value rate at 0.058.

## Limits

- One repository and one language (Go). The mutants are synthetic. Cohort R is representative of
  Corvint's lane history, not of other projects.
- A failure that no test in the module can observe is invisible to every selector.
- The observability pruning outside the isolated sample can hide A-rescues. That bias works
  against ADDS_VALUE, and the isolated sample measures it.
