# 0307 — Competition-relative alpha bar for retrieval; blind-v6 attempted once and scored FAIL on forbidden hits (2026-09-17)

Status: accepted (the bar) and consumed (the attempt). Owner instructions 2026-09-17, verbatim:
"get us to release asap"; "come up with an efficiant and solid plan to get to release. no more
messing about"; "you can change the bar if its acceptible vs the competition"; "go" (on
`release-scope-20260917/FAST-RELEASE-PLAN-20260917.md` in the release-next packet). The third
instruction supersedes, for the `0.4.0a4` alpha only, the reservation of acceptance-criteria
changes in the 2026-09-16 handoff. `thresholds()`, `learned_trace_gate`, `GPK-V0-073` FULL
qualification and `GPK-V0-074` promotion are unchanged and not claimed.

## The bar

The only shared measurement between Corvint and a competitor is the runner's own exact-search
baseline (`exactBaseline`, `benchmarks/runner/main.go`): term overlap over source text, top-5 file
hit rate and abstention on the same sealed cases. Published retrieval benchmarks (ContextBench,
CORE-Bench) score file and block recall with no shared cases, so they frame the bar but cannot be
scored against. The alpha bar is therefore relative to that baseline on the held-out partition:

| metric | alpha bar | FULL gate (unchanged) |
|---|---|---|
| top-5 task success | ≥ 1.5 × exact-search baseline and ≥ 0.50 | `thresholds()` |
| abstention accuracy | ≥ 0.5 and above baseline | `thresholds()` |
| must-exclude hits | 0 | 0 |
| critical recall | reported, no floor | 1.0, zero misses |
| latency | runner mean per case ≤ 10 s; host adapter 30 s budget never exceeded | 500 ms |
| precision | development floor 0.8 and empty `v4_release.blocked_by` as preconditions | held-out 0.8 |

PASS iff every gated row holds on one run of the sealed manifest at the committed configuration.

## The attempt

Configuration `c31d928e` (decision 0304), engine source identical to branch head `1895b414` for
`internal/`, `benchmarks/` and `cmd/`. The runner was built from `c31d928e` plus
`blind-v6-fixture-extension.patch` (decision 0301; fixture digest `e56d71d9…`, the digest the sealed
`--validate-only` was taken against) in a scratch clone; the patch stays unapplied in this checkout
and touches only the learned-trace fixture and the runner's digest constant. Preregistration
(`v6/SEALED-V6-QUALIFICATION/PREREGISTRATION.json`, sha256 `b3f9ec12…`) froze the bar, the
prerequisites on observed-v5 and the development corpus, the manifest and case digests, the pins,
the runner digest `c5241426…` and the one command; `ATTEMPTED.json` was written exclusively before
the runner started; `RESULT.json` binds the output digest `f0dd005c…`.

| metric | observed-v5 (`c31d928e`) | blind-v6 held-out | baseline (v6) | bar |
|---|---|---|---|---|
| top-5 task success | 0.514 (1.64×) | 0.571 (1.67×) | 0.343 | met |
| abstention accuracy | 0.5 | 0.5 (5/10) | 0.0 | met |
| must-exclude hits | 0/36 | **7/36** | — | **not met** |
| latency mean per case | 6.85 s | 7.19 s | — | met |
| critical misses | 12/25 | 23/39 | — | reported |
| recall | 0.333 | 0.429 | — | reported |
| byte-weighted precision | 0.067 | 0.099 | — | reported |

Per repository top-5 (Corvint / baseline): beamfall 0.429 / 0.286, flask 0.571 / 0.571,
cobra 0.714 / 0.429, zod 0.714 / 0.286, execa 0.429 / 0.143. Forbidden hits: flask 4, cobra 3,
none elsewhere. Three of the seven are in expected-`OUT_OF_SCOPE` cases the engine answered
`READY` (`v6-flask-dispatch-requests-typo-oos`, `v6-cobra-json-docs-oos` ×2); four are sibling
declarations admitted beside the target in positive cases (`scaffold.py:post`,
`views.py:as_view`, `app.py:do_teardown_appcontext`, `active_help.go:AppendActiveHelp`).

Verdict: **FAIL** on the must-exclude row. The attempt is consumed. The FULL `v4_release` list
blocks on eleven checks including `zero_forbidden_hits`, as expected.

## Decision

1. The alpha bar above is the retrieval acceptance rule for `0.4.0a4` only. It is not a FULL
   qualification and does not amend `thresholds()` or `GPK-V0-073`.
2. Blind-v6 is consumed with a FAIL. No sealed partition remains (v4 retired, v5 and v6
   consumed). Any further retrieval qualification needs a new commission under `GPK-V0-073`.
3. `0.4.0a4` ships with retrieval labelled experimental and unqualified, as the plan's FAIL branch
   states. The release notes carry the table above.
4. No engine change. Symbol-recall work stays stopped (decision 0306). The forbidden-hit class
   (sibling declarations and scope answers on out-of-scope tasks) is the first target of any
   future cycle, because it also fails the FULL `zero_forbidden_hits` gate.

## Evidence

Recorded first on the retrieval cycle line (`codex/cycle11-tooling-20260917`, docs commit
`1ab35bd4`, CEM sidecar `4fed2b5c`); this copy carries the same decision into the `0.4.0a4`
integration leaf, whose engine is the integrated core line and not `c31d928e`.

Release-next packet: `v6/preregister.py`, `v6/attempt.sh`, `v6/SEALED-V6-QUALIFICATION/
{PREREGISTRATION.json,PREREGISTRATION.sha256,ATTEMPTED.json,RESULT.json,validate-only-attempt-runner.json,
run/heldout-v6.json,run/case-summary.txt}`; the planning-store amendment
`owner-amendments/20260917-experimental-alpha-fallback/` (FINAL-RELEASE revision 25, dependencies
PI-ADAPTER and SELF-DOGFOOD only). Not done: no push, tag or publication; no protected authority;
no sealed case content read before the attempt; the corvint worktree, benchmark cache and cycle 8/9
checkouts untouched.
