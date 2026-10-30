# Test selection friction: narrow, correct and explained — lane K=2 (w5)

Fifteen agent-memory friction tickets in the `corvint affected` planner, `internal/gateselect`, the
review snapshot, `corvint test` and the merge-queue narrowing scripts, worked from base
`c5d5f0829`. Requirements: AFP-V0-928 to AFP-V0-931 (`docs/specs/affected-plan-v0.md`) and
STR-V0-173, STR-V0-174 (`docs/specs/session-test-runner-v0.md`). The merge-queue and
pull-request narrowing scripts under `.github/` (V1-3109, V1-3159, V1-3160) are the branch's tip
commit, for their own pull request and the owner's `ci-control-plane` review; that commit adds
their decisions here.

## Already on main

V1-3123, V1-3129 and V1-3138 are ADN-V0-001 to ADN-V0-011 (`additive-narrowing-v0.md`) and
V1-3133 is IDX-SNAP-V0-115, each with the traceability tests named there; the tickets were only
left open.

## Decisions

- V1-3146 is confirmed, not only suspected. For a sealed CEM on this repository `Select` printed 39
  `reader` packages and the planner selected none of them. The `/` anchor (the module-path
  literal) beside a lone `.corvint` token, which gateselect lets name a directory component, made
  six of them readers; those six are now dropped (AFP-V0-931). The other 33 hold a parent-only
  token such as `".."`, which can climb from the package directory to the root. The planner's
  V1-0290 rule does not compose those, but `Select`, the CI safety net, still does. The first
  version of the fix used the planner's rule outright, which broke ADN-V0-011's climbing case and
  was narrowed back to the bare `/` anchor.
- V1-3147 is a live miss, not only duplication. The snapshot review's tail lacked the AFP-V0-050
  prior read scopes, so editing one key of the read-scope declaration selected every declared
  package by declaration. The new test fails on base with exactly that. Both paths now go
  through `planAffectedGraph` (AFP-V0-930).
- V1-3152 needs an explanation, not a narrowing. On bb943b524 (24 paths) the three unowned paths
  (`Makefile`, `.github/workflows/ci.yml`, `.corvint/test-read-scope-literals.json`) make every
  non-participating plugin's frontier bear through AFP-V0-511. That is 87 entries, 75 of them
  TypeScript, which reads any `.yml` as a resolution input. Narrowing those rules needs a
  per-plugin soundness argument, so this lane does not attempt it. Instead each such entry now
  names the unowned path (AFP-V0-929), and `--format text` prints it. Even a one-Go-file change
  stays UNKNOWN through `go:build-constraint-variants`, `go:nested-module-frontier` and
  `shell:foreign-invocation-unmodelled`, which is also why V1-3160's merge-queue delta reuse
  never applies.
- V1-3149: the declared `-p 1` wins over heavy.sh's `GOFLAGS=-p=N`. `corvint test
  --go-parallel N` (STR-V0-173, local only) is recorded through `declaredDigest`.
- V1-3155: `corvint test` and `make go-test` ignored `CORVINT_GOCACHE` and used the declared
  `/tmp` cache, while the other Makefile targets and heavy.sh's remote used the per-lane one. They
  now honour it at launch (STR-V0-174). The reported divergence in results is not reproduced.
- V1-3148: `--format json|text` (AFP-V0-928, amending AFP-V0-201).

## Narrowing scripts (tip commit, AFP-V0-933 to AFP-V0-935)

- V1-3160: AFP-V0-816 required a BOUNDED delta and record receipt, which this repository never
  produces, so delta reuse could never apply. A receipt whose unknowns are all LANGUAGE_FRONTIER
  entries is now accepted (AFP-V0-935). The pinned selector FALLBACKs on the module-level ones and
  attributes the rest, exactly as it does for the queue-range selection the entry runs. Any other
  unknown still refuses.
- V1-3159: GitHub names a stacked entry's `base_sha` as the entry ahead of it, which is not on
  main. The scripts now come from the newest main commit that base descends from. The selection
  is measured from that commit too, a superset of the entry's range, so narrow.sh's `.github/` and
  shadow guards cover the earlier entries as well (AFP-V0-934).
- V1-3109: narrow.sh's AFP-V0-098 guard now uses the exit status of `git diff --quiet`, as
  mq-narrow.sh already did (AFP-V0-933).

## Dogfood intent scope

The CEM's intent scope is `docs/specs/session-test-runner-v0.md` alone.
`docs/specs/affected-plan-v0.md`, the owning spec of AFP-V0-928 to AFP-V0-931 and AFP-V0-933 to
AFP-V0-935, holds more than 256 requirements, so `ocm prepare` refuses it
(`too-many-obligations`; DOGFOOD.md §4, OCM-V0-023 (3)). Its base lines that the hunks implement or
amend are cited as `specification` instead: AFP-V0-050, 098, 152, 201, 494, 511, 731, 813 and 816,
plus ADN-V0-009 and ADN-V0-011. Omitting the spec is not a claim that its requirements hold. They
are carried by the traceability rows and the tests above. Restructuring that spec is an owner
decision.

## Not done

V1-3243 is PARTIAL. The fix, `git update-index --chmod=+x` on `script/release-env-compat_test.sh`
and `script/runtime-environment_test.sh` plus a repository guard test, is ready. But CEM-V1-300
refuses any patch holding a mode-only group (`metadata-only-change-unsupported`), even beside text
hunks, so a dogfood seal cannot bind it. It is left outside this sealed branch for the
orchestrator.

V1-3158 is PARTIAL. A narrowed queue entry writing no tested tree is AFP-V0-733's stated design:
the main push is the full-suite safety net. Running only the complement on the push removes that
net, so it needs an owner decision, and hosted queue runs to measure it, before any change.
