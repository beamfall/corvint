# Affected hidden-package integration: verifier repair 1

The host verifier rejected the AZU-V0-005 integration on two documentation gates:
the zero-unknown spec's intent status included multiline provenance, and newly indexed
hidden packages appeared as unrecorded unbounded readers. This repair changes only
spec metadata, project-owned test read declarations and fixture annotations; planner behavior, wire shapes,
goldens, error text and test assertions remain as previously delivered.

## Decisions and evidence

- SRG-V0-011 / SRG-V0-970: keep both header statuses and the digest status exactly
  experimental. Preserve the owner's goal and delegated design provenance in a separate
  header field, and put the local-evidence qualification in the spec body. Regenerate
  the spec index and README from the staged spec; this does not accept or promote it.
- AZU-V0-005 / AFP-V0-023 / AFP-V0-051: declare empty external read scopes for
  `.github/cishards` and `.github/testconfine/cmd`, rather than enlarging the permanent
  unbounded-reader floor. The former's `./...` strings are partition input data;
  its isolated helper is built from embedded copies of its own directory's files.
  The latter's existing test clears CORVINT_CONFINE_ROOT and checks the early refusal,
  before the production root locator or scope loader executes. Neither test package
  needs another tracked directory. The scope checker also sees package names in the
  frozen wall-time replay table as paths; existing fixture-path markers identify
  that in-memory table without changing a single string value or assertion. Any
  new external read must extend the declaration and pass checks and confinement.
- Existing whole-package tests passed with the actual Landlock wrapper and these
  declarations: `.github/cishards` (8.503s after annotations) and `.github/testconfine/cmd` (0.011s).
  Each command used `script/heavy.sh`, `-count=1 -timeout 30m`, no `-run` filter,
  and `-exec` pointing to the existing wrapper built into external evidence scratch.
  `internal/specindex` also passed whole (0.876s) through spec-requirements-check;
  the existing hidden `.github/testconfine` library passed whole (0.059s).
  Package selection used `corvint affected --base
  8e2c13aa926fcaf1731fae71b27c41c41c8f09af` before tests. The installed binary is
  stale; a current-tree binary's separate plan is retained alongside that observation.

The first focused rerun correctly refused stale generated metadata after the spec edit.
Staging the spec and running `make spec-requirements` regenerated the two affected
metadata rows; no requirement definitions changed. The next scope-lint failure
identified the in-memory replay literals; annotation retained the exact table text
(after stripping only markers), SHA-256
`f916106c06cdb05931a1ac61bae5ed4a9a490009b34c8141b20f8058e1f98089`.
Both read-scope and unbounded-reader checks then passed. Final documentation-gate results,
citation repins, CEM rebind and dogfood-check evidence are retained in the repair handback.

The host's selected test run returned informational rc=3, with whole-package fallback:
no declared-consumer adapter and a `go list ./...` universe that omits four hidden
packages. That incomplete receipt is not a passing suite. This repair does not change
runner adapters, execution policy, error codes or stderr to conceal the limitation.
The prior whole CLI failure, independent review, browser qualification, broad safety
qualification and native ticket completion retain their recorded limits. The
orchestrator owns independent review and host verification; no model reviewer ran.

Rollback is removal of the two declarations and restoration of the spec metadata;
that restores the verifier failures without changing any runtime wire. Raw receipts
and logs stay under `/home/russell/lanes/azu-small-evidence/repair1/`; the handback
records their results and digests. Existing pre-change receipts are reused. No fresh
pre-change capture, push, PR, Tasks write or seal is authorized in this repair.

## Retained evidence digests

- Final read-scope and unbounded-reader checks, exit 0:
  `scopes-final.log`, SHA-256 `11e23d626c1b2845b598de98b8e45be0536dbe2f2b0e8d3283618536abff6421`.
- Final whole confined shard package, exit 0:
  `test-cishards-confined-final.log`, SHA-256 `fc8af2e4f7a5884e7b5911ef5ac4ed28b6ba5895417dce7520f5e71b9d8e80a0`.
- Whole confined wrapper command, exit 0:
  `test-testconfine-cmd-confined.log`, SHA-256 `c069cfbcdef768c402e8bf27760d6934a0842d003115900000a745778b5c6aa4`.
- Retained losing static-literal check (311675 bytes), exit 2:
  `focused-doc-gates-final.log`, SHA-256 `a6a1252d8c9b14903449e3c98ca5bbf4cd0d471b33fe2b4fd2a8ad99bee14bd2`.
