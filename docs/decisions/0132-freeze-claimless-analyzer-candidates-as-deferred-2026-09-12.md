# Decision 0132 — Claimless analyzer candidates are frozen as delivery status `deferred`

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

The 2026-09-01 audit (`docs/reviews/FABLE-5.1-AUDIT-2026-09-01.md` F14, §7) proposed freezing
16 analyzer-candidate binaries and about 5,600 spec lines that carry no claim. This decision
re-measured that list against the tree at `f3e6163e`.

## What "frozen" means

Frozen is the delivery status `deferred`, now defined in `docs/SPEC-DRIVEN-DEVELOPMENT.md`
(Status model). Seven specs already used that status without a definition. It is not a new word
`frozen`, because `Frozen: <date>` headers already mean a spec's text-freeze date. A `deferred`
spec and its code:

1. stay in the tree unchanged, and their existing tests keep running;
2. get no new work;
3. are not advertised and not built into a release archive or install;
4. carry a `Disposition:` header naming the return condition.

Intent status is unchanged. Reopening needs a recorded decision.

## Re-measurement

- **Binaries.** The 16 `cmd/corvint-analyzer-*` commands are still isolated. Each
  `internal/analyzer<family>` package is imported only by its own command. Only
  `internal/analyzercap` imports `internal/analyzerexec` outside those commands. No release path
  builds them: `script/go-archive-gate` builds only `corvint` archives, and
  `script/corvint-companion-release-gate` builds `corvint-console`, `corvint-dashboard-snapshot` and
  `atm`. Point 3 already holds for them, so no script changes.
- **Already `deferred`.** Kotlin/Android, native/JVM bridge, shader, structured-data, Swift/Apple,
  `verification-planner-observer-v0`, and `session-context-dividend-v0`.
- **Deferred now.** Three specs had no selection, launch, install, or support claim and were still
  `experimental`: `analyzer-candidate-profiles.md` (which also owns the Go, JS/TS, .NET, Ruby,
  HTML/CSS, shell, and SQLite commands), `analyzer-python-native-candidate.md`, and
  `rust-analyzer-candidate-v0.md` (intent stays accepted under decision 0007 D6). With these, every
  analyzer-candidate command sits under a `deferred` spec.
- **Not frozen: the tree contradicts the audit list.**
  - The six affected-test adapter specs (.NET, Kotlin/JVM, Ruby, Rust, Swift, TS/JS) are imported
    by `cmd/corvint/affected.go:18-25@6671b48a`. They are part of the shipped `corvint affected` surface.
  - `work-queue-observation-v0` is accepted (decision 0046) and reachable through
    `corvint work`.
  - `local-observability-dashboard-v0` ships `corvint-dashboard-snapshot` in the companion bundle
    that `public-release-v0` requires.
  - The two zero-ID programs are research catalogues that live specs depend on.
    `applied-intelligence-breakthroughs-v0` is accepted and not started, so deferring it changes
    nothing. `context-evolution-program-v0` cites core `internal/gokernel`.
  - `human-documentation-compiler-v0` is accepted owner intent and was not in the backlog entry.
  - `sql-native-ratchet-gate-v0` is an owner-instructed opt-in gate. It stays as it is.

## Separate finding

The audit's F25 still holds at `f3e6163e`. One untracked file makes range `impact` refuse with
`unsupported-impact-worktree`, makes learned traces `blocked-mixed-worktree`, and fails the archive
gate. Each is correct by its contract, so fixing it is a contract change, not a small repair. It is
filed in `docs/agent-memory/bugs.md` with a repro.

Rollback: revert this decision's commit. That restores the three `experimental` statuses and
removes the `deferred` definition. No code, script, or test changes.
