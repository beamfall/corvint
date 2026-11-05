# AZU verifier repair 1

The host verifier at `a4f4516547def247314123796d6437065ccb4f45` found stale
use-case subject pins, inconsistent AZU spec statuses, an unclassified test-owner
declaration, newly reached unbounded readers and a failed historical selection
baseline. This repair is limited to those findings. The owner prohibits frozen
output or assertion changes, publication, ticket writes and sealing.

## Decisions and requirements

- SRG-V0-011/970: keep both AZU header statuses `experimental` and make the
  Agent digest agree. Preserve the owner's goal and delegation in the current-state
  prose, including the explicit NOT_QUALIFIED limit. Regenerate the spec index.
- AFP-V0-710 and AZU-V0-002/8900: classify `.corvint/test-selection.json` as a
  closed planner declaration at the top level, each owner and each must-run
  entry. `TestTestSelectionClosedPositions_AFPV0710` checks additive known-shape
  entries and rejects unknown members at all three positions against a Git pin.
- AZU-V0-004/8901/8902: the shell root-query recognizer compares an option without
  running Git. Use the same split-token convention as `golang.rootLocatorToken`
  so the conservative literal scanner does not treat this recognizer, its language
  registry and the unbounded-reader checker as repository readers. Existing shell
  package tests still cover root queries and refusal of queries for another cwd.
- AFP-V0-023/025 and TSP-V0-950: retain the seven actual newly reached unbounded
  units, recording their invocation chains and why read scopes cannot bound their
  computed sources. Do not cut caller edges or claim a new confinement proof.
- UCV0-004/005: repin the changed `cmd/corvint/affected.go` subject and its ledger
  receipt digest with the maintained script. Preserve the receipt shape and claims.
- AZU-V0-001: historical `pr-6` actually exits 3 because a
  `NO_SELECTABLE_TEST driver:...` unknown is misclassified as a language needing an
  adapter. The freshness notice only occupies the baseline runner's bounded error
  excerpt; it does not cause the exit. Extend the existing selected-driver exception
  to this unknown-unit path. Added cases in `TestSTRV0003BuildVerdicts` preserve
  existing Go selections and keep an undeclared Python unit incomplete.

The replay diagnostic is retained outside the repository at
`/home/russell/lanes/azu-new-evidence/repair-1/pr6-dry-run.json` and
`pr6-dry-run.stderr`. The raw manifest and logs remain there too. No freshness
guard, diagnostic, exit code, golden, wire shape or existing assertion is changed.

Rollback is to revert this repair commit, preserving the original verifier failure
and experimental qualification limits. These repairs do not supply historical
full-suite/mutation evidence, independent review or sibling integration, and do not
promote AZU-V0. Pre-change capture is reused as directed by the owner; native affected
planning is used to select repair checks. Other self-development routes are not
applicable to this integration repair, and model review remains orchestrator-owned.

## Verification evidence

All Go tests below ran through `script/heavy.sh`, one whole package per command,
with `GOTOOLCHAIN=local go test -count=1 -timeout 30m` and no `-run` filter.
The native affected plans against the fixed owner base are retained as
`repair-1/affected-before.json` and `repair-1/affected-repair.json` under the
external evidence root. The exact toolchain check passed for go1.27.3.

| Package | Result | Package test time | Log SHA-256 |
| --- | --- | ---: | --- |
| `internal/sessiontest` | PASS | 11.428 s | `47a972d85c3a2f26fff138e13565f37710b4c0cfde146968f60d76cc7088f4ee` |
| `internal/liveverify/affected/shell` | PASS | 0.789 s | `f6818709e4a66faaaf002293d806497399c4d2c4588e046a5668df43fc542da1` |
| `tools/declaration-compat-check` | PASS | 0.287 s | `0971464cd71257e94b61c3444279e924ac85ab20d86f69984322170b0cc14187` |
| `internal/specindex` | PASS | 1.347 s | `d434f9c794a607f03c8f310686043791f0c0152085dda46254172cabaa0cef3d` |
| `conformance/use-cases-v0` | PASS | 0.318 s | `40890363d44c3e76e7f3e61eabebc43ed838cc41abc88a1cf55264fd27f8193d` |

The spec generator, receipt repinner, line citation repinner and line citation
check passed. The line repinner found no unchanged content that moved.
The pre-repair historical diagnostic JSON SHA-256 is
`f26ae3ae483006e1766b39dd36733b55129a6544d8e7f51475bba896e9e2149d`.
