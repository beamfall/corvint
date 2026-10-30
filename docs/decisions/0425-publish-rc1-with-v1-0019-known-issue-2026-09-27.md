# Decision 0425: publish 1.0.0-rc.1 with the V1-0019 failure as a known issue

Date: 2026-09-27. Status: accepted (owner answer 2026-09-27: "Publish rc.1 now" and
"spf13/cobra").
Tickets: V1-0019, V1-0020, V1-0431, V1-0432, V1-0433.

## Context

V1-0019 run-001 ran the frozen untouched-repository cases (`corvint-1.0-product-and-release-v1.md`,
Frozen cases) on the `1.0.0-rc.1` candidate `b967f6bbe33c5eba367a07d7eeb2bd1e372e9162` (build 163).
It failed:

- go-chi/chi: orientation FAIL, with 3 treatment-only critical misses in 20 scored cases.
  Consequence PASS (0 misses in 19 scored cases) and completion PASS (12 informative cases, no
  false-complete verdict).
  The result is `benchmarks/untouched-repository-v1/runs/run-001.json`, sha256
  `3a518374754dc757a0e140ac859b9b29d7bd0a6b6ad6473b87081ffac515843a`. The missed critical paths are
  test files, which a task context without a subject does not rank (V1-0431).
- beamfall/core: orientation FAIL, with 1 treatment-only critical miss in 20. Consequence PASS
  (0 in 20) and completion PASS (12 informative, none false-complete). The private result has sha256
  `43fbcdfd0800724b8944caaca3ed6a65dd5cf3d2433ac7706efa0e8c751b3f8b`; its corpus, run and receipts
  stay in the owner's local release evidence. The miss is inferred to be the V1-0431 class and was
  not separately diagnosed.
- Corvint: aborted before scoring. The loop target's range adds a sealed CEM, so `dogfood change`
  refused (V1-0433), and the harness raised instead of recording `HARNESS-FAILURE` (V1-0432). Only
  the start marker `benchmarks/untouched-repository-v1/corvint/runs/run-001.started.json` exists.

The agent asked the owner two questions. The first was whether to publish `1.0.0-rc.1` now, with
these results as known issues, or hold it for the fixes. The second was which public repository
replaces go-chi/chi as the held-out repository, since go-chi/chi's cases were read to diagnose
V1-0431. The owner answered "Publish rc.1 now" and "spf13/cobra".

## Decision

1. `1.0.0-rc.1` is the notes commit on `b967f6bb`, tagged `v1.0.0-rc.1` and published as a GitHub
   prerelease. Its release notes list the run-001 results above as known issues.
2. V1-0019 stays open and gates 1.0 final instead of `1.0.0-rc.1`. In the task store, V1-0019 and
   V1-0020 (which depends on V1-0019) move from release `v0-9` to `v1-0`, where V1-0020 covers the
   next candidate. The `v0-9` acceptance criterion becomes: "One immutable release candidate passes
   every in-scope gate and exercised rollback before owner-approved publication, and its
   untouched-repository result (V1-0019) is published with it and gates 1.0 final (decision 0425)."
3. The fixes for V1-0431, V1-0432 and V1-0433 land before `1.0.0-rc.2`. V1-0019 runs again on that
   candidate.
4. spf13/cobra replaces go-chi/chi as the held-out untouched repository. Its pin, cases and
   preregistration digest are frozen and published before that run, as decision 0422 required for
   go-chi/chi. The go-chi/chi run-001 stays recorded as evidence and is not rerun as the held-out
   case.

## Non-goals

- This does not waive V1-0019 for 1.0 final or change any orientation, consequence or completion
  threshold.
- It does not choose how V1-0431, V1-0432 or V1-0433 are fixed.
- It does not accept, approve or consent to anything on the owner's behalf beyond the two answers
  recorded above.

## Rollback

A published tag is never moved. Withdrawing `1.0.0-rc.1` means adding a `## 1.0.0-rc.1 withdrawn`
entry to `docs/RELEASE-NOTES.md` and marking the GitHub prerelease withdrawn. Until `v0-9` is
promoted, the task-store change is undone by a `release update` that returns V1-0019 and V1-0020 to
`v0-9` with its earlier acceptance criterion. Item 4 is undone by reverting this record, which
makes go-chi/chi the held-out repository again.
