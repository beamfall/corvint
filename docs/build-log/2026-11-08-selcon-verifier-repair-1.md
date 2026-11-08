# V1-5094: verifier repair 1 — CI replay arguments

Requirement: AFP-V0-9501. Base: `9761ea1f459c0d3b3c963953e24a76eaeda16370`.
The host's selection-contract-check stopped at pr-2 because the replay supplied
`--profile ci` without its required absolute `--confine` argument.

The replay now supplies an absolute planning-only wrapper path beneath its checkout.
`dryRunManifest` always supplies `--dry-run`; the CLI's dry-run branch constructs
invocation arguments without opening or executing the wrapper. No wrapper binary is
needed to compare selected packages. Local-profile replay arguments stay unchanged.
The CLI validation, wire formats, errors, exits, assertions and goldens are unchanged.
Rollback removes this caller fix and its new regression; CI corpus replay would again
refuse its missing argument, rather than falsely qualify selection parity.

This finding is a caller argument omission, not an analysis soundness gap. No source-form
allowlist changes. The replay's existing success allowlist remains exit 0, or exit 3
with a regular manifest; every other exit remains an error. The new
`TestSelectionContractDryRunArguments_AFPV09501` exercises both profiles, exact argv
including dry-run and the CI absolute path, a path containing spaces, the reported
argument-refusal exit 2, and unknown exit 4 even with a manifest. Existing incomplete
dry-run tests retain the exit-3 controls.

The whole `tools/selection-baseline` package passed through `script/heavy.sh`, one
package with no test filter (2.859 package-test seconds). The lane binary's
`corvint affected --base` selected that package before the run. Repin found no moved
citations. `make -s spec-requirements-check line-citations-check` passed. Raw evidence
is outside the repository in `/home/russell/lanes/selcon-evidence/`:

| File | SHA-256 |
| --- | --- |
| `repair1-test-baseline-final.log` | `d2de5ed02dd06e5ec340de7f3bef874cf14e523c7af493db8891f7991bf998c9` |
| `repair1-affected-current.json` | `95f8811606aee98668551349701f6f302463b47b07391a36d2d9019c7c70d0cd` |
| `repair1-spec-line-check.log` | `f4ac47235b260f4c1d32ea05aee138e5d2aa8c8dd3eb9a9d9c194c5fd7597c14` |

Self-use: affected used; final CEM rebind/check required. Pre-change capture reused by
explicit instruction. Corpus replay and whole-repository gates remain NOT_RUN here:
the owner reserves them for the host verifier. Independent review is NOT_PRODUCED in
this lane and belongs to the orchestrator. No selection parity or performance gain is
claimed from argument fixtures. Prior failed cmd/corvint evidence and separate CI
control-plane integration remain open as recorded in the original lane entry.

Friction: the installed binary warned self-binary-stale, so the existing lane-built
binary produced the final affected receipt without that warning. Heavy admission
initially refused a whole-package test without HEAVY_WHOLE_REASON; the successful
run supplied the owner's whole-touched-package requirement and affected selection
as its reason. No lock directory override was used.
