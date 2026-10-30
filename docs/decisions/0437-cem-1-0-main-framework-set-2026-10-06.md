# Decision 0437 — Closed main-framework set for CEM 1.0 runner coverage

Date: 2026-10-06. Status: accepted by the owner on 2026-10-06 ("accept", in reply to the orchestrator's
proposal). Answers V1-0642. Scopes V1-0591 and its language parents V1-0592, V1-0593 and V1-0594.

## Context

CEM 1.0 (V1-0591) needs task-linked evidence for "all main supported test runners", but "main" was
never closed. `docs/specs/test-runner-execution-v0.md` § Runner inventory lists 57 concrete
experimental profiles: 21 dynamic, 17 native, 16 platform, two SQL and one Appium Android. None is
stable-qualified. The registry also lists six IDs that have no profile: `appium`, `pgtap`,
`sqllogictest`, `shader-behavior`, `html-behavior` and `structured-data-behavior`. A scratch audit at
a5326ba6 proposed further additions that have no profile or ticket. That audit acquired and ran no
framework software and was not independently reviewed (V1-0642 body).

Without a closed set, V1-0591 cannot finish: each new runner needs native acquisition, a fixed
profile and live qualification, and an open "main" list makes every omission look like a gap.

## Decision

The closed main-framework set for CEM 1.0 is the following. The rule for admitting a framework is a
widely used test runner in its own right that produces a machine-readable native report. A layer that
executes inside a runner already in the set is covered at that runner's level.

1. **Accepted: the 57 existing profiles.** These are the profiles of the runner inventory as of this
   decision, including Ginkgo v2 (`TRE-V0-018..020`) and CMocka (`TRE-V0-016..017`). Each is
   qualified under its language parent: V1-0592 (JavaScript/TypeScript, Python, Ruby), V1-0593 (Go,
   Rust, .NET, C/C++) or V1-0594 (JVM, Apple, Android and the remaining domains).
2. **Accepted additions:**
   - Jasmine, under V1-0592 (its own ticket).
   - Boost.Test, under V1-0593 (its own ticket).
3. **Covered by an existing runner, not a separate profile:**
   - Python doctest counts as covered only where the project's own pytest configuration collects it
     and the pytest profile's native report carries it. The profile adds no doctest flag. Qualifying
     this path belongs to V1-0592.
   - Cucumber-JVM counts as covered where it runs on the JUnit Platform and appears in a JUnit,
     Gradle or Maven profile's report. Qualifying this path belongs to V1-0594.
   - Quick counts as covered at XCTest test-method level. Qualifying this path belongs to V1-0594.

   Scenario- or spec-level identity beyond what the host runner reports is not claimed. Where a
   qualification fails, the framework becomes an exclusion under item 4. It does not become implied
   coverage.
4. **Excluded from CEM 1.0 (explicit, not implied coverage):**
   - behave, Cucumber-JS and Cucumber-Ruby.
   - The C++ doctest framework and Unity (C).
   - Compose UI assertion qualification beyond the Android profiles' test level.
   - A JNI behavioural bridge.
   - The named HTML/CSS, shader (GLSL/Metal) and structured-data (JSON/YAML/TOML) application assertion
     tuples, together with the registry IDs `shader-behavior`, `html-behavior` and
     `structured-data-behavior`.
   - The unqualified registry IDs `appium`, `pgtap` and `sqllogictest`. The explicit profiles
     `appium-uiautomator2-wdio`, `sql-pgtap` and `sql-sqllogictest-sqlite` remain in the set.

   A CEM 1.0 receipt for a project using an excluded framework reports the runner as unsupported
   (uncertainty), never as covered. Each exclusion may be revisited after 1.0 by a new decision.

This decision closes scope only. It does not qualify any profile, change a wire format or requirement,
or alter the release gates.

## Rollback

Supersede this decision with a new one. Reopen V1-0642 if the set must be reconsidered before 1.0. The
Jasmine and Boost.Test tickets can be archived, and their dependency edges removed from V1-0592 and
V1-0593. No code, profile, receipt or wire format changes.
