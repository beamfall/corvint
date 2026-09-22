# Decision 0122 — CEM and OCM next actions keep the contract command name `corvint`

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

`cem prepare` and `ocm prepare` return one `nextActions` argv whose first element is `corvint`
(`internal/cem/workflow/commands.go`, `internal/lrfrepo/ocm_write.go`). Since decision 0088 the
only product binary is `corvint`, so a user who copies that argv on a clean host gets "command not
found" (first-user quickstart walkthrough, `docs/BUILD-LOG.md`, 2026-09-12).

The argv is frozen evidence. `conformance/cli-parity-v0/manifest.json` pins the exact stdout
bytes of `cem-prepare-create`, `cem-prepare-resume` and `ocm-prepare-resume`. A probe build that
emitted `corvint` failed all three with `stdoutSha256` mismatches under
`replay --only <case>`; the unmodified build passed all five `cem-prepare*`/`ocm-prepare*` cases.
`GOC-V0-002` forbids re-capturing those expectations.

Call: keep `corvint` as the literal contract command name in both actions. `CEM-CB-016` and the OCM V0
`nextActions` paragraph now say so, and that a caller running the `corvint` binary substitutes its
own executable for that element. `README.md` tells users to do that or to build the binary under
the name `corvint`, after checking that no older Python `corvint` is earlier on `PATH`.

Alternatives set aside:

- Emit `corvint`. Moves 3 frozen parity cases, which would need either a re-capture (forbidden)
  or three new known-divergence declarations for a change that is not a Python defect.
- Emit the invoked program name (`os.Args[0]` basename). Moves the same 3 cases and makes the
  bytes depend on how the binary was invoked, which no frozen fixture can pin.

Consequences:

- Newer Go-only surfaces (`dogfood verify`/`review`/`finish` in `internal/localcompletion`) already
  emit `corvint`; the two frozen CEM/OCM actions remain the exception until a later accepted
  exact-parity contract re-freezes them.
- `examples/cem/verify-pr.sh` is unaffected: it runs `CORVINT_BIN` (default `corvint`) and never
  executes a returned action.

Rollback: revert this decision's commit. That removes the two spec sentences, the README paragraph,
the source comments and the unit assertion, and restores the backlog entry; emitted bytes do not
change either way.
