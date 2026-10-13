# 2026-10-13 Prospective registration: citation support over the next 30 change maps (V1-1337)

This entry registers, before any of its maps exist, the scorer, the set, the metrics and the
decision rules of the prospective citation run that V1-1337 asks for and that decision 0527's
criterion K2b reads. It is committed by lane V1-1336 (`docs/build-log/2026-10-13-v1-1336-citation-arms.md`),
which adds the scorer's arms; the run itself happens later and is recorded in its own entry. Every
choice the analysis needs is fixed below. A change to any of them is a new registration with its
own date, made before the maps it scores are read; it never amends this one.

## Scorer

- Source: `tools/cem-citation-baseline/main.go` with `git hash-object` blob OID
  `40b37c5e520881baf390e7547c7d505d9d487654`, built with `GOTOOLCHAIN=local go build ./tools/cem-citation-baseline` from the
  tree of the lane's merge commit M (the first commit on `origin/main` whose parents include this
  lane's seal commit), so the verifier tier rules it calls (`internal/cem/verify`) are M's. Before
  the run, `git rev-parse M:tools/cem-citation-baseline/main.go` MUST print the OID above; if it
  does not, the run is void and needs a new registration.
- Arms, null and members: `DCW-V0-051..056` as accepted at M. Null: N = 200 derangements per map,
  seeded from the map's `patchSha256` (`DCW-V0-053`). Pooling: the ratio estimator over maps with
  a null and its 95% normal-approximation interval (`DCW-V0-055`).

## Set

- Scope: the first 30 maps, in order, that (a) are added under `.corvint/changes/` by a commit on
  the first-parent chain of `origin/main` after M, (b) declare `cem/1.0`, (c) have status
  `citationClass` `change`, and (d) are scored with arms (valid status, empty `armsIssues`). Order
  is the first-parent position of the adding commit, then the bind SHA in byte order. A map that
  meets (a) to (c) but not (d) is skipped, does not count toward 30, and is listed with its issue
  code in the run's entry; more than 5 skipped maps is itself reported as a finding beside the
  result.
- Targets: each map is verified at its own bind commit (no `--targets`); a native post-M map's
  bind sidecar already holds its bytes.
- Run: in a scratch clone at C, the first-parent commit that adds the 30th map, copy the 30 maps
  into `.corvint/prospective/` and run
  `cem-citation-baseline --root <abs clone> --maps .corvint/prospective --summary summary.json > maps.jsonl`.
  Retain both files under `docs/build-log/evidence/v1-1337/`.

## Metrics

All read from `summary.json` member `change` (identical to `all` for this set).

- Primary: `arms.authored.anchoredExcessExclAdjacent.pooled` (E) and its `ci95` lower bound (L):
  pooled excess anchored hunks over the derangement null, diff-adjacent bases excluded, per
  supported hunk.
- Comparators: `arms.grepbot.anchoredExcessExclAdjacent.pooled` (G) and
  `arms.preimage.anchoredExcessExclAdjacent.pooled` (P).
- Self-citation: `selfCitation.diffAdjacentBaseShare` (D), diff-adjacent authored bases over
  authored bases of supported hunks.
- Secondary: held share H = `tiers.held` / (`hunks` − `tiers.mechanical-proved`), all paths.

## Baselines

- Panel (the 2026-10-13 CEM 1.0 confidence panel, evaluation-methodology lens; its scratch
  scripts are not retained in the repository): 2.1% [1.7%, 2.5%] pooled excess over a
  derangement null, diff-adjacent bases excluded, on the committed corpus.
- This scorer on the committed corpus (V1-1336, `docs/build-log/evidence/v1-1336/`, 615 maps at
  their `CEM-V1-104` T' targets, 15 at their bind commits; 629 scored, f0fda542 refused with
  `unsupported-resource-limit`): E = 2.04% [1.64%, 2.44%] over the 615 maps with a null
  (22,639 supported hunks); G = 67.0%; P = 0.00%; D = 3.2%; H = 0; the grep bot anchors at least
  the authored count in 629 of 629 scored maps. Under the rules below that corpus is already a
  kill (E ≤ 0.021), so the prospective set must move the number, not merely reproduce it.

## Decision rules (evaluated in this order; the first that holds is the outcome)

1. Kill: E ≤ 0.021, or D > 0.25. Outcome recorded as "anchored is lexical coincidence, not
   support" (decision 0527 K2b failed).
2. Improve: L > 0.025 and L > G, and E > G and E > P. Outcome recorded as "authored maps beat
   both mechanical producers" (decision 0527 K2b passed).
3. Otherwise: "inconclusive"; decision 0527 K2b is not met, so the anchored tier stays out of the
   reviewer-facing summary as K2b's failure branch requires until a new registration passes.

Secondary, reported beside the outcome and never changing it: if H < 0.25 the decision 0523 K1
held falsifier is recorded as firing on this set; the current verifier never produces `held`, so
H = 0 unless the P2 witness producer lands before M. 0523 K1's own Go-code restriction is that
decision's procedure, not this registration's.

## Rollback

Revert this file with the lane. A void run (scorer OID mismatch at M) or a new registration
supersedes it; the numbers it records are not edited after the fact.
