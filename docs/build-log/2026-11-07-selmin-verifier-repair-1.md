# Selmin verifier repair 1: listing scope is separate from binary execution

Date: 2026-11-07
Requirements: AFP-V0-9010, AFP-V0-2200 (V1-5029); precision guard AFP-V0-9003
Base: cb403735903b1e54a8469373489960c0b2b89d99
Repair input: 57bbbb40d1b772ecf468fefb85acc616e5b1cfee

The host's whole `internal/liveverify/affected` package failed its unchanged
`TestPrecisionReplay_AFPV09003`: a heavy.sh-only edit selected 87 language units,
violating the existing strict bound of fewer than 86. Directory-origin uncertainty
was also discarding proof that a confined command only lists packages. Its package
literal then incorrectly contributed a binary-execution edge.

## Decision and class boundary

Separate a command's read scope from whether it can execute the listed binary.
Only the existing closed proof of listing arguments, command confinement, literal
environment, isolated commands and nonescaping output admits a literal as a listing
argument. Unknown command uses, methods, mutations and escapes still fail that proof
and retain ordinary literal/binary edges. No new command form is certified.

The directory-origin allowlist is unchanged: empty/dot literals, a fresh direct
TempDir on a confined testing parameter, or a module root derived from Caller(0)
through Dir/Clean/Join and relative literals with unique confined definitions and
unchanged target resolution. Any unknown origin still sets whole-module import and
source observation and leaves its selector unproven for the source classifier.
All directory assignments must prove their scope; a later known value cannot erase
an earlier unknown one. A confined listing with unknown scope cannot run its target
binary, so its literal no longer creates that false edge. No package paths or
repository-specific exceptions are used in this rule.

A provisional attempt expanded safe root derivations and scoped declaration
uniqueness to the enclosing function. It passed the plugin tests but still failed
the parent precision guard at 86 language units. That approach was discarded in
full. The delivered fix keeps the original directory proof. A temporary diagnostic
confirmed that the console's remaining selection has an independent unbounded-read
witness, so changing that unrelated rule would not be an appropriate repair.

## Regression evidence

`TestUnknownDirectoryListingIsNotExecution_AFP_V0_9010` checks the reported computed
path and Git-root stdout shapes, an environment origin, an unknown function form,
and unknown-then-known assignments. Each stays a whole-module source/import reader,
adds no binary edge, and is selected on both import and body edits. Unknown command
methods and escaping the command retain the binary edge as well as source reads.
The original directory/symlink assertions and precision bound remain unchanged.

An earlier new fixture expected the import-reader witness while also introducing a
binary target under the provisional implementation. It failed because binary
execution was a competing witness. That failed run is retained externally and is
not passing evidence. The delivered regression deliberately uses a binary target
and asserts the corrected separation directly.

- Whole `internal/liveverify/affected/golang` package: PASS, 10.592s, including
  all existing tests. Log SHA-256
  `7db1ea30b8076b6b0337fdfca94d8847c3f4262d7f9f0090705242840454afed`.
- Unchanged `TestPrecisionReplay_AFPV09003`: PASS, 4.09s. Heavy-only selection is
  84 language units (previously 87), plus the same 187 Make targets and 32 CI jobs,
  303 total. Scope remains UNKNOWN; no frontier is erased. Test-only remains 75
  language units and declaration-spec remains 278. Log SHA-256
  `7aabd782e6ed07b9f6b256cfd1378f70489de08580053ea0898a88a42382e904`.
- `make -s spec-requirements-check line-citations-check`: PASS. Repin found no
  moved citation. Log SHA-256
  `23718c67395a68322def47be5ab345e7128e6ec617b4428ab0da2fbbef9b4728`.
- Whole `internal/liveverify/affected` package: PASS, 144.661s, including all
  existing tests and the unchanged precision guard. Log SHA-256
  `c507d27fb7fc75995494ad9e9808eff1ca6f1418bbb9aa7b6c7e07f7247e5de9`.
- Tests ran through heavy.sh one package per command, with TMPDIR set to the
  lane scratch directory and no identity exported. Go version: go1.27.3.

## Limits and rollback

Only V1-5029 is repaired in this round. V1-5026 / AFP-V0-9009 and V1-5030 /
AFP-V0-9011 are unchanged. Native ticket completion is not claimed: Tasks writes
are prohibited. Independent review and broader host gates belong to the orchestrator.
No wire shape, error/exit code, stderr text, golden or existing assertion is changed.
No new pre-change capture, publication or seal is run. Existing pre-change evidence
is reused; affected selection, focused package checks and final CEM binding/checking
are the applicable self-development routes. Other product routes are outside this
repair. Raw evidence is under `/home/russell/lanes/selmin-evidence/`.

Rollback: revert this repair's code, regression and requirement clarification as a
unit. That restores the conservative directory behavior but also the false binary
edges and the measured precision regression; it does not repair the verifier failure.
