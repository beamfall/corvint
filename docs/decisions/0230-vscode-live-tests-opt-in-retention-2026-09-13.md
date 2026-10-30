# Decision 0230 — the VS Code extension passes `--retain` only behind an explicit setting

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Decision 0218 gave `corvint-js-test-provider unit|e2e` and `corvint-go-test-provider session` an opt-in
`--retain` flag (`LPCV-V0-055`) and left open that the VS Code extension does not pass it. The
extension spawns `corvint.liveTests.command` verbatim (`VSC-V0-058`). The VS Code specification is
silent on retention. `LPCV-V0-055` requires retention to be opt-in, and decision 0218 rejected
retention on by default.

The call (`VSC-V0-070`):

- A new resource-scoped boolean setting, `corvint.liveTests.retainEvidence`, defaults to `false`. With
  it `false` the spawned argv is byte-for-byte `corvint.liveTests.command`, as before.
- With it `true`, the extension inserts `--retain` directly after the subcommand token, and only
  for a command whose program basename is `corvint-js-test-provider` with subcommand `unit` or `e2e`,
  or `corvint-go-test-provider` with subcommand `session`. Both providers parse the arguments after
  the subcommand with Go's `flag` package, which stops at the first non-flag argument, so the token
  directly after the subcommand is always parsed as a flag.
- With it `true`, any other command is refused before spawning. So is a command that already names
  a `retain` flag (`--retain`, `-retain`, `--retain=...`). Both are reported through the
  `VSC-V0-066` cause-naming unavailability path. A refused command is never run without the
  retention the setting asked for, and a user-written `--retain=false` cannot silently override it.
- The extension itself still writes nothing. The provider process writes into
  `.corvint/test-evidence` (gitignored in this repository), which `corvint test-validity --discover`
  reads.
- Like a changed `corvint.liveTests.command`, a changed setting applies the next time the provider
  starts. A provider that is already running is not restarted.

Rejected alternatives:

- Always pass `--retain`. That makes an editor session a repository writer without the operator
  asking, and it contradicts the opt-in rule of decision 0218.
- Run an unrecognized command without the flag when the setting is `true`. The operator would see
  live results and believe retention happened when it did not (invariant 2).
- Append `--retain` to the end of argv. A trailing positional argument would stop Go flag parsing
  first.

What stays open:

- `NOT_RUN`: a real-provider `@vscode/test-electron` run in which the extension spawns a retaining
  provider and `test-validity --discover` selects the result.
- `NOT_RUN`: a qualified Go session matrix.
- Go results stay preview (`GLTP-V0-048`).

`PUB-V0-006` therefore remains not met at release tier.

Consequences: `VSC-V0-070` is added. The `VSC-V0-058..070` traceability row, the `LPCV-V0-055` and
`PUB-V0-006` traceability rows, the reach-gap plan, and the release-readiness `PUB-V0-006` row are
amended.

Rollback: revert the commit. That removes the setting and the argv insertion. No provider, schema or
wire contract changes. Files already retained under `.corvint/test-evidence` stay readable by
discovery, or can be deleted.
