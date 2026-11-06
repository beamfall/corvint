<!-- SPDX-License-Identifier: Apache-2.0 -->
# Unbounded lane merge-forward

The owner requested merging mainline `dba1f4d6c9a33098470ccf57794fb66f989f03c8`
into lane head `7249a21ad238ae6a24d3f9a6de3be4a10dea4146`, preserving both sides.
Merge `6da59eeab9901a220502feb2fd0c72a5383384f7` resolves exactly two conflicts:

- `internal/gateselect/readers.go` keeps mainline's typed lexical tokens and
  the lane's original-source offset tracking and comparison-only locator exemption.
- `docs/specs/README.md` keeps mainline's AFP-V0-2600 range/locality evidence and
  the lane's AZU-V0-9930/9931 range and bounded-reader claim.

The generated requirement table and spec index were taken from mainline, then
regenerated together by `make spec-requirements` in commit
`def495c26776699e6b3cb9878b446d9f27252501`. No lane requirement ID collides
with mainline; AZU-V0-9930 and AZU-V0-9931 retain their IDs. Existing seals and
all pre-round commits remain unchanged and reachable. No behaviour was discarded.

## Observed checks and review

`corvint affected --base dba1f4d6c9a33098470ccf57794fb66f989f03c8` returned 0.
The local toolchain is go1.27.3. The owner's focused package command,
`script/heavy.sh go test -count=1 ./internal/gateselect`, returned 0, as did
`make -s spec-requirements-check line-citations-check`. No Go file was regenerated;
only `internal/gateselect` holds a manually conflict-resolved Go file.

An independent static review of the combined merge diff, regenerated indexes,
complete merged scanner, locator proof and its tests reported no findings.
It verified the coexistence of typed tokens and comparison-only source offsets,
and preservation of both catalog updates. Review limits: supplied immutable
source only, no independent test execution, no second scanner/full-parent/spec-body
review, and CEM excluded. Runtime command access failed (read-only initialization,
then socket-directory mode refusal); the completed review consumed a retained
source packet without invoking tools. Review packet, report and original failures
are retained under `/home/russell/lanes/unbounded.tmp/merge-*`.

## Rebinding and limits

The new base is the merged mainline commit, never the old base. Initial preparation
refused stale ignored OCM artifacts; all three were preserved under the worktree's
private Git `corvint/stale-ocm/` directory before retrying. Preparation then verified
the existing sealed CEM and produced its map. The expected uncited preparation
reported `citation-plan-not-provided`, `not-ready` and `record-index-failed`.
Pre-change query/impact receipts remain STALE against the newer base; their original
pre-edit context is preserved. Guarding tests stay NOT_PRODUCED/guarding-tests-not-run;
no additional unconfined test selection was run. Query packet coverage was
NOT_PRODUCED/packet-coverage-unreadable, despite the query step being PRODUCED.

The owner explicitly instructed this round to stop after dogfood-check, without
sealing, although the supplied original head is itself a seal commit. No new seal,
push, PR, history rewrite, Tasks write, whole selection or repository gate is
part of this round. Host verification and batch CI remain external, unobserved work.
The handback records the final binding/check outcome and all return codes.
