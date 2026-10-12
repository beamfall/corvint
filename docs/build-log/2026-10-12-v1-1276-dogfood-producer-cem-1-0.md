# 2026-10-12 Core cem/1.0 producer and dogfood loop cutover (V1-1276)

Branch `claude/v1-1276-dogfood-producer` from `origin/main` ddd559754 (includes V1-1281, #831).
Decision 0525 step 5. Requirements `CEM-V1-140..152` in `docs/specs/cem-stable-v1.md`, accepted;
amends `CEM-V1-017`, `CEM-V1-018`, `CEM-V1-030..033`, `CEM-V1-058` and `DCW-V0-039`/`DCW-V0-049` in place.

## Intent

The RC3 loop must produce only `cem/1.0` (decision 0525). Before this change Core prepared
`cem/0.2`, the bind loop refused a stable map (`cem-map-stable-unsupported`, `CEM-V1-032`),
`batch-check` admitted only canonical `cem/0.2`/`cem/0.3` constituents
(`constituent-cem-stable-unsupported`), and OCM prepare refused `cem/1.0` (`CEM-V1-058`).

## Decisions

1. `cem prepare` writes `cem/1.0` by default. The `cem/0.x` producer stays behind the explicit
   `--spec cem/0.2` until V1-1278 removes the 0.x readers; any other value (`cem/0.1`, `cem/0.3`,
   the experimental candidate) is refused `invalid-arguments` before anything is written. This is
   the fail-closed 0.x producer flag the brief left to the lane.
2. `cem begin` keeps `cem/0.1` by default because the frozen cli-parity case `cem-begin-map` pins
   it; `--spec cem/1.0` opts in. The library zero value keeps the legacy producers, so MCP,
   frontier, conformance and other library callers are unchanged.
3. Stable writers (prepare `--replace`, begin, cite, mark, cover, discriminate) mutate only a
   producer map: no criteria, receipts, links, artifacts or provenance; a `role` is kept. A bound
   stable map is still refused with `unsupported-spec`. Cover and discriminate no longer upgrade a
   stable map to `cem/0.3`.
4. The producer never proves a structural claim; `cem status` runs the stable verifier, which
   ACCEPTs a correct `import-reorder` and REJECTs `unproven-mechanical` otherwise.
5. `cem status` over a stable map admits `--max-unknown`/`--max-mechanical` as policy ceilings
   after an ACCEPT (exit 1 on an exceeded ceiling); the embedded `stableVerification` is
   unchanged. `cem verify` still refuses ceilings.
6. The stable verifier needs an explicit artifact root. The prepare `nextActions`, the loop's
   `cem status`, `dogfood check` and `batch-check` pass `--artifacts <absolute repository root>`
   for a stable map only; a producer map names no artifact, so the root is never opened.
7. A `cem/0.x` sidecar left by an earlier run is refused as a different spec; the loop treats that
   exact line as an outdated map and reruns prepare with `--replace`.
8. `batch-check` selects the proof form from the map's declared spec, so a 0.x `ready-for-ci`
   line never proves a stable map and a stable ACCEPT line never proves a 0.x map.
9. OCM prepare, read and link admit a stable map only through a stable ACCEPT of the expected base
   and target with the repository root as artifact root and the matching canonical patch digest;
   the sidecar need not be committed because the first pass runs before the CEM commit.
10. The `#831` chore class is live: `markStableChoreMap` now writes `bind-chore`/`seal-chore` on
    the prepared map; `TestChangeDCWV0049MarksPreparedStableChoreMap` is the positive chore case.

11. `dogfood finish` (local completion) reads the default `cem/1.0` change map: its `cem status`
    adds `--artifacts <absolute root>` and keeps the ceilings, and its `cem report` adds the root
    and drops the ceilings, which a stable report refuses and the preceding status enforced
    (`CEM-V1-151`). Sidecar-only reuse stays `cem/0.x` only (`CEM-V1-150`).
12. The stable verifier takes only full 40-hex object IDs, never `HEAD`. The `review-change.sh`
    recipe resolves the target to its full ID, adds `--artifacts` for a `cem/1.0` map and passes
    no ceilings to its report; `docs/DOGFOOD.md` section 6 runs the stable `cem status` with the
    ceilings and then `cem report` without them, on full IDs (Codex round 2, P2).
13. A stable prepare over sidecar bytes that are neither a `cem/0.x` nor a `cem/1.0` map keeps the
    "not a valid CEM document" refusal; only a readable `cem/0.x` map is "a different spec".
14. Tests that exercise `cem/0.x`-only surfaces (0.x status goldens, `verify --patch`,
    `report --ocm`, `cem export`, the v1 attestation, the `ci-verify.sh` unsupported-profile case,
    the explicit sidecar reuse, the `verify-pr.sh` recipe test, which reads `cem/0.1`/`cem/0.2`
    only) now prepare with `--spec cem/0.2`; the dogfood flow tests expect the stable
    `cem=accepted` / `state=accepted` summary in place of `ready-for-ci`.
15. `script/dogfood-bind-range.sh` reads a `cem/1.0` range map with `--artifacts <absolute
    worktree root>`; a `cem/0.x` map keeps the old argv. Its own test drives a fake binary. Its
    citation-plan comparison parses the compact stable map's hunk order (Codex round 1, P1).
16. `DCW-V0-047` is extended to the `dogfood-ocm status` comparison: the base verifier refuses an
    OCM read over a `cem/1.0` map (`unsupported-spec`), so under an admitted declaration the tree
    verifier runs in both roles, as it already did for `cem status`; undeclared, the comparison is
    still byte-identical and fails `verifier-disagreement`. Found by the live qualification; this
    is a fork decision of the lane (`TestCheckDCWV0047DeclaredOCMStatus`).
17. The aggregate verifier worker grammar admits `--artifacts` on the strict `cem status` read only
    for the worker's own root (`CEM-V1-152`); without it the aggregate finish over a default map
    failed `final-check-failed`.
18. `review-change.sh` resolves `CORVINT_ROOT` to an absolute path before it adds `--artifacts`,
    since the stable read refuses a relative root (Codex round 1, P2). It selects the stable read
    from prepare's own next action, which names `--artifacts` only for a `cem/1.0` map, not by
    grepping the map's bytes, so a resumed map with other JSON whitespace still gets it (Codex
    round 4, P2; `script/cem-recipes_test.sh` case `r-spaced`, which fails on the old grep). The
    match is the action's exact argv prefix through `--target <full ID>`, `--artifacts`, so a
    worklist path cannot supply it (Codex round 5, P2; case `r-legacy-dash`, a `cem/0.2` prepare
    of a file named `--artifacts`, which fails on a whole-output match). Round 6 reviewed this fix.
19. A `cem/1.0` prepare skips the 0.x sidecar refusal but still probes every recognized
    interrupted-publication backup before recovery or resume cleanup: a producer or `cem/0.x`
    backup is restored or removed, a bound stable or unreadable one is refused and kept
    (`CEM-V1-141`, `TestCEMV1141StablePrepareGuardsBackups`; Codex round 3, P1). `docs/DOGFOOD.md`
    section 4 now shows the stable status read with a full target ID and `--artifacts` (round 3, P2).
20. `CEM-V1-017` is amended in place to name the `CEM-V1-145` status-ceiling exception: after an
    `ACCEPT`, `cem status` with ceilings adds the counts and `policyIssues` and exits 1 when a
    ceiling fails, while `stableVerification` stays the unmodified verifier result. It previously
    still prohibited those fields and required the verifier's own exit code (Codex round 6, P3).
    Text only; behavior and tests are unchanged.

## Review and acceptance

Independent Codex review: rounds 1 to 5 returned FAIL, and each finding was fixed in this change
(the Codex round citations under Decisions). Round 6 returned FINAL-VERDICT PASS with one P3, fixed by decision 20.
`CEM-V1-140..152` are accepted under the owner's standing auto-accept (2026-10-11) after the
round 6 PASS.

## Output change declaration

This change alters the `cem status` output the loop records (a stable envelope replaces the
`ready-for-ci` envelope and the argv gains `--artifacts`), so the base verifier cannot agree. A
dogfood run of it MUST declare `DOGFOOD_OUTPUT_CHANGE=DCW-V0-047` (V1-1279) for `dogfood-check`
and `dogfood-seal`; `docs/specs/daily-change-evidence-workflow-v0.md` is modified in
`BASE..HEAD`, so the declaration is admitted.

## Evidence

Focused tests: see the traceability table under "Core cem/1.0 producer and dogfood loop" in
`docs/specs/cem-stable-v1.md`.

Live AC1 qualification (throwaway `git clone --no-local` of `117f29a81`, removed afterwards; not
the final bind). Inputs: four intent specs, a 152-row citation plan, a verify file, outcome
`passed`, full BASE `ddd559754614e7b7bd4e7527eb7fbb9500bfaf71`, `DOGFOOD_OUTPUT_CHANGE=DCW-V0-047`.

- First `make dogfood-change` pass: every step PRODUCED; `local-outcome` `record-index-failed`
  (the uncommitted sidecar, as `docs/DOGFOOD.md` step 5 expects). The map is `"spec":"cem/1.0"`.
- After the CEM commit, the second pass exits 0.
- `make dogfood-check`: `PASS`, `cem=accepted hunks=152 supported=152 unknown=0 mechanical=0`,
  `ocm=ready-for-review`, `base-independence=NOT_PROVED declared=DCW-V0-047`. The base digest is
  that of empty output, because the base verifier refuses the stable read.
- `make dogfood-seal`: `PASS`; the sealed `.corvint/changes/<bind>.cem.json` is `cem/1.0`.

An earlier clone of `ebf8a2731` failed the check with `verifier-disagreement` on the OCM
comparison, which led to decision 16.

## Limits and NOT_RUN

- `internal/localcompletion` sidecar-only reuse binds only canonical `cem/0.x` CEMs; a `cem/1.0`
  sidecar grants no reuse and the check reruns (`CEM-V1-150`, fail closed). Stable reuse is a
  follow-up; `TestExplicitSidecarReuseRequiresCanonicalMap` now prepares with `--spec cem/0.2`.
- Frontier admits only `cem/0.2` and refuses a `cem/1.0` map with `unsupported-frontier-context`
  (observed with the branch binary); `internal/frontier` belongs to a parallel lane, so the
  reviewer's frontier view is NOT_PRODUCED for a stable map and `conformance/host-lifecycle-v1`
  pins its frontier case's prepare to `--spec cem/0.2`. Frontier stable admission is a follow-up.
- `tools/cem-citation-baseline` runs `cem status --citation` without `--artifacts`, so it reports
  a sealed `cem/1.0` map as an `artifacts-required` error row; its test prepares `cem/0.2`. The
  step 6 migration rerun owns its stable read.
- `docs corpus cem` projects `cem/0.x` only and reports a `cem/1.0` map as `internal-error`
  wrapping `unsupported-spec`; `cem report --ocm` reads `cem/0.x` only. Their tests prepare
  `cem/0.2` (or remove the default map first). Stable projection is a follow-up.
- The hostile-evidence use cases assert `cem/0.x` status codes and resolve the target as `HEAD`,
  which the stable verifier refuses; they prepare `--spec cem/0.2`. A stable read also refuses a
  relative `--root` (pre-existing parity with `cem verify-stable`).
- `internal/observations` keeps `cem-map-stable-unsupported` in its closed reason list so ledgers
  written before V1-1276 still replay; nothing writes it now.
- OCM fails closed for a bound stable map whose artifacts lie outside the repository root.
- The class-split citation baseline rerun stays NOT_PRODUCED until committed maps carry a role.
- `script/dogfood-bind-range.sh` against the real binary NOT_RUN; its test drives a fake binary.
- `make gate` NOT_RUN (owner preference for scoped issue work); the final bind and seal are the
  orchestrator's and are NOT_RUN here.

## Rollback

Revert the change. `cem prepare` again writes `cem/0.2` and refuses a stable sidecar; the loop,
`batch-check` and OCM again refuse `cem/1.0`. A `cem/1.0` map sealed meanwhile stays valid for
`cem status`/`cem verify-stable`, but a reverted `batch-check` refuses it, so a batch over it needs
this change or a rebind under `--spec cem/0.2`. No wire, schema, packet or frozen case changes.
