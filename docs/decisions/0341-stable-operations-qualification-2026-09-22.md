# Decision 0341 — Stable operations qualification as two local checks and a command runbook

Date: 2026-09-22. Status: accepted (ticket V1-0017, release v0-8). Adds `docs/specs/stable-operations-v0.md`
(`SOP-V0-001` to `SOP-V0-012`); amends nothing.

## Context

V1-0017 asks that install, upgrade, rollback, uninstall, backup and corrupted-state recovery, the
hostile-input regressions, and the vulnerability/support/changelog/runbook set close before the 0.9
candidate. The tree already held installer fixture tests (`PUB-V0-025`/`026`), a prose runbook, and
hostile tests spread over twelve packages, but no command ran a released executable through its own
index and read path, and no single command named which hostile tests are release-blocking or which
categories have none.

Measured on 2026-09-22 (darwin arm64): a truncated or byte-damaged `.corvint/index` snapshot leaves
the read verb at exit 0 with byte-identical packet output and the file untouched; `index --if-stale`
rebuilds it (`mutates: true`) at the same size but not byte-identically (460 differing bytes). The
ticket's wording "fails closed with a named reason" and "rebuilds byte-identically" does not match
the shipped snapshot contract, which treats damage as a miss.

## Decision

1. `script/check-install-lifecycle.sh` is the lifecycle qualification: one archive or binary, eight
   ordered steps in a private temporary directory, packet-byte identity as the recovery invariant,
   and read non-mutation of a damaged snapshot as the invariant-4 check. `SOP-V0-006` encodes the
   measured contract, not the ticket's assumed one; snapshot byte identity is an open question for
   `index-snapshot-v0.md`.
2. `script/check-hostile-regressions.sh` is the release-blocking hostile matrix: exact `-run`
   regexes per package, NOT_RUN for any test without a `--- PASS` line, and NOT_COVERED rows for
   `memory` and `case-folds-context-index` that are printed, never faked.
3. `SECURITY.md` states the support window: 0.x, latest published release only; the 1.0 window is
   set by the owner at V1-0021. `docs/RELEASE-RUNBOOK.md` becomes exact ordered commands through tag
   and rollback, citing only existing scripts.
4. No Go change, no service, no daemon. The `make` targets `install-lifecycle-test`,
   `hostile-regressions-check` and `hostile-regressions-test` are proposed for the Makefile owner.
   Hosts other than darwin arm64 are NOT_RUN in this change.
