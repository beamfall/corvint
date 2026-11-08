# unbrd self-review repair (2026-11-07)

V1-4283, AFP-V0-3149, AFP-V0-2950 and AFP-V0-2956. All three reported P1 findings
exposed an uncertified input reaching a precision exemption. This repair narrows those proofs;
no JSON, wire, diagnostic, exit code, golden or core-freeze output changes are authorized.

## Decisions and closed allowlists

- Shell unrolling: retain plain identities and standalone literal defaults, and permit an unknown
  top-level prefix only when its plain self-reference ends the value. Any suffix is unproven and
  retains `shell:dynamic-source`; `constructed` does not resolve wildcard-prefixed extensionless
  targets. The reported `$PWD/$x/check` and an unknown suffix expansion both select the wrapper
  for configuration and target changes. Existing unsupported-transform regressions remain.
- Go root proof, identically in the planner and CI selector: certify every declaration and write,
  including the original initializer. The only allowed writes are an imported `os.Getwd` with
  two simple results, a single local zero-string declaration and a single local empty-string
  initializer. Another receiver's `Getwd`, arbitrary calls, parameters and unknown write forms
  remain root locators. The existing rebinding control now starts from the certified empty value;
  its original parameter initializer is retained as a negative regression in both proofs and the
  cold/warm parity test. No existing assertion was removed or weakened: arbitrary initialization
  now has explicit conservative assertions rather than masquerading as a bounded control.
- Listing output: arbitrary templates may emit executable programs, including buffered programs
  run by a named shell operand. Keep execution edges unless output is the default package path
  or an exact `{{.Dir}}` / `{{.ImportPath}}` template with the known options enumerated in
  AFP-V0-2956. New options and encodings fail closed. This avoids attempting to model temporary
  file identity or template execution. The reported template and an unknown printf template both
  retain the Makefile target on a runtime-input change; an unknown option also retains its edge.

This trades precision for soundness only where the proof is incomplete. Rollback must retain
uncertainty, not restore an unchecked exemption. No new capability, command or wire field is added.

## Evidence and limits

- `TestComposedSelfReferencesKeepSelection_AFPV03149`: reported extensionless composition plus
  unknown suffix form, configuration and target selection.
- `TestLocatorInitializerAllowlist_AFPV02950` in both proof packages: environment, unknown call,
  parameter, var initializer, unknown reassignment and unrelated receiver, with safe controls.
- `TestUnprovenLocatorInitializerSelection_AFPV02950`: the reported environment initializer and
  unknown initializer select the reader for `other/nested/config.go`.
- `TestUnprovenInitializerParity_AFPV02950`: both selectors and cold/warm memo agree for environment,
  unknown call and parameter initializers.
- `TestBufferedListingProgramSelection_AFPV02956` and `TestDriverListOutputAllowlist_AFPV02956`:
  end-to-end buffered program selection and closed format/option checks.

The whole shell, Go analyzer and CI selector packages passed. The parent affected package and
spec/citation checks are recorded with their final results in the lane handback. Original logs
and the selection receipt stay under `/home/russell/lanes/unbrd-evidence/repair9-*`.
The required affected route selected all four touched packages (495 selected units, 74 unknowns,
`scope=UNKNOWN`). The installed binary reported `self-binary-stale`; this selection is not claimed
as current-source equivalence. Pre-change capture is reused under the repair instruction.
Independent review and repository-wide gates belong to the host verifier. No Tasks writes,
publication, seal, provider, learning or external qualification route is authorized for this round.
