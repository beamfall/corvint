<!-- SPDX-License-Identifier: Apache-2.0 -->
# Bind child-build proof independently of delayed Wait

Review input: `bb0804c5764017e6d7f75bf5afb2e515ab5f2ddf`.
Lane base: `cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`.
Scope: the remaining independent-review P1 for V1-4380,
LPCV-V0-7601 and LPCV-V0-954. The three accepted fixes stay unchanged.

## Decision and failure boundary

A successful first Wait could hash a partially instrumented replacement rather
than the completed build's output. Consuming proof once only prevented replay.
The collector now freezes build arguments and environment at Start and, after
successful Wait, builds a private reference executable with those settings.
Only that private artifact's digest receives module-wide instrumentation proof;
the caller's output path is never hashed to establish proof. Launch auditing
continues comparing the actual launched bytes to the certified digest.

This deliberately pays for one additional build, normally using Go's cache,
instead of changing when the original build publishes its output or introducing
a process wrapper. The private command bypasses recursive proof generation and
discards its diagnostics. Failure to produce the reference artifact withholds
proof; no original command result or public diagnostic is changed. The private
scratch directory is removed after hashing. Rebuilding changed sources can
conservatively withhold proof for an otherwise covered original executable.
No performance or production qualification is claimed.

## Acceptance evidence

`TestUninstrumentedChild/partial-replace-first-wait` observes stderr EOF from
the full build without calling Cmd.Wait, completes a main-only replacement,
then calls the original command's first Wait and launches the replacement.
The existing incomplete-record assertion rejects this sequence. The companion
`covered-delayed-wait` accepts a completed full build whose first Wait is also
delayed. Existing repeated-Wait and partial-replacement assertions remain.

The whole internal/linecov package, through script/heavy.sh with Go 1.27.3,
failed on the old implementation only at the new replacement case (25.864 s),
then passed with the fix (43.092 s). These are different implementations of
the same expanded suite, not an isolated performance comparison. Raw outputs
remain outside Git under `/home/russell/lanes/bugfix-evidence/`, prefixed
`delayed-wait-`. SHA-256 of `before.log`:
`39ce1e90a5672d9dc5503cdc585d30067102759c9862a3177a73a5b3777f96bc`;
of `after.log`:
`26ef22cd818624657ceb3cff9c45963ce4eb3e8b5f7296b234c70ee7fc23bf2e`.

## Self-use, limits and rollback

Used the existing pre-change capture and native affected selection against the
lane base. The installed binary reports self-binary-stale; its UNKNOWN scope
does not establish exhaustive coverage. Selected internal/linecov was tested
as a whole package. New pre-change capture, broad gates, Tasks writes,
publication, seal and model-backed review are excluded by the repair charter;
the orchestrator owns independent review and wider host checks. Retrieval,
learning, provider qualification and command catalogs are not changed.
Final CEM rebind/check uses the existing capture; its exact outcome and all
NOT_PRODUCED rows belong to the external lane handback.

Rollback reverts this repair and recollects coverage records. It restores the
unsafe delayed-Wait behavior, so rollback cannot retain this proof guarantee.
