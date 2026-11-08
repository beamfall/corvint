# Decision 0592: Runner selection plugins in two forms, with a language-neutral sound selector

- Date: 2026-11-07
- Status:
  - §1 and §2: accepted (owner, 2026-11-07, "Yes, both forms").
  - §3 to §7: orchestrator-made, owner may override (the owner delegated in-task forks).
- Authority:
  - Owner chat, 2026-11-07: "get the system to always be only running the required minimum tests
    every single time ... it should be perfect"; "it would need to support go, but also other
    langauges and e2e"; scope "beamfall support basically"; "each langauge and runner should be a
    plugin so other langauges and runners can be easily added"; "Yes, both forms".
  - `docs/specs/runner-selection-plugins-v0.md` (`RSP-V0-001..047`).
- Amends: `docs/specs/language-plugins-v0.md` (`LGP-V0-002` closed piece set; the run-time loading
  non-goal now points to the runner protocol).
- Keeps: decision 0590 and `LPCV-V0-950..979` (records, freshness, intersection, misses); decision
  0591 and `CMS-V0-001..011` (invariant 7); `ACC-V0-002`/`-005` (no `PATH`, digest-pinned
  executables); every existing static plan only while this spec is shadow-only, before its rollout
  enables a first runner (`RSP-V0-046`).

## Context

Per-test line records exist for Go, node runners and browser E2E, but only as developer commands,
and they observe executed lines only: a changed data file, an added fixture a glob would find, or a
script a test executes sends the change to the static plan (`LPCV-V0-953`). Language plugins
(`LGP-V0`) are compiled-in and their piece set has no notion of what a runner would run, what a test
touched, or whether narrowing a runner was ever proven safe. The owner asked for minimum test runs
across every Beamfall runner, each language and runner a plugin.

## Decision

1. **Two plugin forms (owner).** Runner pieces are compiled-in or out-of-process. The out-of-process
   form is one `corvint-runner-<name>` executable speaking versioned line-delimited JSON over stdio
   (`corvint-runner/1`), local only, one process per run, no daemon or network, discovered only
   through digest pins in the tracked `.corvint/runner-plugins.json` and resolved from the release
   manifest or a per-user store, never from `PATH` or the repository. Go, shell and make, and the
   JavaScript and TypeScript runners stay compiled-in (`RSP-V0-025..032`).
2. **Scope (owner).** Go, TypeScript and JavaScript with Playwright E2E, Swift (XCTest, XCUITest,
   Swift Testing), Kotlin and Java (JUnit, Espresso), C# (`dotnet test`), Rust (`cargo test`),
   Python (pytest), shaders, and shell and make targets. Beamfall repositories may ship their own
   plugins (`RSP-V0-041`).
3. **"Perfect" means sound within a qualified execution model.** Selected ⊇ affected for every runner
   whose `inventory`, `test-evidence` and `qualification` pieces are present and qualified for the
   change's boundary classes; FULL for every other runner, including every runner never qualified
   under this spec; every project-mandated test (`AFP-V0-2484`, `AFP-V0-2485`) always run; an
   auditable reason per excluded test. Selection is the union of every observed dependency, and
   anything missing or uncertain selects or widens to FULL. In CI only trusted-base inputs and
   protected-ref evidence can narrow (`AFP-V0-013`/`-014`). Exact minimality is not claimed
   (`RSP-V0-012`, `RSP-V0-017`, `RSP-V0-042`, `RSP-V0-043`).
4. **Measured, not asserted.** Narrowing is enabled per runner and class only by a retained passing
   qualification record (replay plus fault injection per boundary class), stating the one-sided 95%
   upper bound on the miss rate, never "zero misses". A trial is one independently sampled mutant
   that FULL kills, and a miss is any failing test left unselected; the bound covers only that
   mutant population. The initial floor is 300 trials per class (an upper bound of 0.995%). Sampled
   FULL runs on the default branch measure misses against the retained plan's exact base, head and
   excluded set. No runner narrows until the sampling rate is nonzero; its value and CI cost bound
   are open numbers, set from the first measured runs and not invented here (`RSP-V0-034..037`).
5. **Tracing is a core service.** File reads, failed lookups, directory listings, writes, execs and
   outside-process contact are observed by one compiled-in backend per OS, Linux first (`ptrace` with
   seccomp-BPF, unprivileged), used only by test execution, never by read commands. A plugin's
   self-report cannot be evidence of what its process did, and one backend serves every runner. It
   adds no daemon, privilege, kernel component or network, so it stays inside invariant 7 as amended
   by decision 0591; its size and start cost are measured under `CMS-V0-006`/`-011`
   (`RSP-V0-020..024`).
6. **Soundness beats frozen narrowing expectations.** A selection-baseline or narrowing assertion
   that conflicts with a soundness fix is updated in that change and recorded in the build log
   (`RSP-V0-039`).
7. **Local may stay broader than CI** until a runner's runtime evidence qualifies on the local OS;
   it is never narrower (`RSP-V0-038`). A runner not qualified under this spec is FULL. Today's
   static plans govern only while the spec is shadow-only (rollout steps 1 and 2); from the first
   enabled runner, no static narrowing survives for an unqualified runner (`RSP-V0-046`).

## Rollback

- Per runner: delete or supersede its qualification record, or drop its entry from
  `.corvint/runner-plugins.json`; it returns to FULL immediately.
- Whole capability: revert each slice, restoring the plans in force before this spec; records are
  derived state and need no migration.
- Reversing §1 or §2 is a new owner decision superseding this one; §3 to §7 may be overridden by
  the owner in chat and recorded as an amendment.
