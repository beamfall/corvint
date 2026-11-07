# Heavycap independent review repair

Date: 2026-11-07. Base: `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`.
Review input: CHANGES_REQUIRED, environment-unsetting bypass (P1) and all-matching
run-pattern refusal (P2), against `e73f4b9e967d948754fcfc265d7ffb0a51e83d8a`.

Both findings refine V1-5007 / ASD-V0-2478. A source edit authorizes running
existing tests, including the test it breaks. Whole-unit manifest admission has
the same obligation, while an all-matching run pattern still runs the package.

Decision: automatic and manifest admission accept environment assignments, including
those affecting Go's default environment-file location, but refuse unsupported
`env` options instead of ignoring them. In particular, `env -u GOFLAGS` cannot
hide an exclusion stored in GOENV behind an inherited `GOFLAGS=-p=1`. Manual
reasons retain their existing policy. No new diagnostic, error or exit code is added.

The run-pattern proof is deliberately bounded: empty, `.`, `.*` and `.+`, with
an optional leading `^`, match every top-level test and example. Selective subtest
patterns are not used as whole-package authority. The last repeated direct run
pattern is effective. Restrictive or quoted inherited patterns remain conservative
refusals; no general regular-expression equivalence inference is introduced.

The existing failing-source fixture gains the exact exported-GOFLAGS/GOENV/unset
reproduction, including a child-resolution check. It also exercises all-matching
patterns through small, new and changed-source admission and through a whole-unit
manifest with source-change admission disabled. Existing assertions and frozen
outputs remain unchanged; exclusion negative controls remain in place.

Rollback: revert this repair's guard, regression additions and spec clarification.
That restores both reported defects. The previously absent external pre-commit
entry point still leaves that separate V1-5007 integration PARTIAL. No new
pre-change capture, native ticket write, publication, model-backed review or seal
belongs to this repair. The existing pre-change evidence is reused at its original
identity; merged-base staleness remains visible. Host-wide verification and the
independent follow-up review belong to the orchestrator. Performance savings,
external interoperability and unavailable telemetry remain NOT_OBSERVED.
