<!-- SPDX-License-Identifier: Apache-2.0 -->
# ghci merge-forward evidence

The owner requested rebinding the unsealed ghci lane against mainline
`6339f980e13e36c8ff51a593568da3d935ffae77`, preserving the host's merge
`860d50c5088448193f0e98218112626a5b716755` and regeneration
`da9ea9f762ee9b91c65ae9ee5e3de92de4ec1ace`.

Conflict resolution: `docs/specs/README.md` retains the lane's CI Narrowing row
and the mainline's Runner Selection Plugins entry; its final placement follows
the host regeneration. `docs/specs/REQUIREMENTS.tsv` was regenerated to retain
both sets of requirements, including mainline RSP rows and lane CIN-V0-8720 and
CIN-V0-8721. The spec index has no remaining diff requiring regeneration.
No requirement ID was renumbered, and no Go file was conflict-resolved or
regenerated. `make -s spec-requirements-check` passed on the host's head.

The CEM is rebound over the whole lane range from the merged mainline. Existing
citation evidence is refreshed and reviewed against that base; unsupported seed
policy evidence remains explicitly unknown. This lane was not sealed, so this
round ends after a clean post-bind dogfood check, without a seal.

The prior whole `tools/ci-forward-row` test failure in
`2026-11-08-ghci-forgiveness-lifecycle-repair.md` remains a failed local outcome;
this round does not rerun or repair it. Its original verification commands remain
in the retained outcome input. Only the requested spec and line-citation checks
run in this round; broader selection and hosted qualification remain with the
host verifier. Prechange agent receipts are NOT_OBSERVED for this rebind. The
initial preparation reported citation-plan-not-provided and uncommitted-sidecar
OCM/status/outcome refusals; those are retained in the private prepare log.
