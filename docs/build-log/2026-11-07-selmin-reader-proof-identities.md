# Selmin self-review repair: builtin identity and Caller filename provenance

V1-5029; AFP-V0-2200 and AFP-V0-9010. Input revision:
`e89830941d0fc81884ba42f70025d5d73f1fe868`; lane base remains
`c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`.

## Findings and decision

The environment proof accepted the spelling `append` without its builtin
identity. A same-package helper could inject a byte-computed environment key
and make a supposedly import-only reader observe test filenames. The shared
precondition now certifies predeclared identity only when every source/test
file parses and no declaration or import shadows a predeclared name anywhere
in the unit. Dot imports and unreadable files fail closed. This covers the
environment, argument-slice and output escape proofs together; declaration
scope deliberately over-approximates fields and separate test packages.
The existing expression allowlists still reject unknown forms. No helper body
is accepted as a substitute identity proof.

The Caller directory proof substituted a physical filename despite compiler
line directives. Its closed origin allowlist remains unchanged-directory
literals, confined fresh testing TempDir calls, and confined module-root
derivations from direct `runtime.Caller(0)` through `filepath.Dir/Clean/Join`.
The last origin now additionally requires absence of both line-comment and
block-comment line directives anywhere in the parsed file. Unknown filename
transforms remain unproven. An unproven directory retains whole-module import
and source reads without inventing a binary execution edge.

No wire shape, error/exit code, stderr contract, golden or existing assertion
changed. The owning spec records the preconditions and traces the regressions.
Rollback is to retain these readers conservatively; removing the preconditions
would reopen the reported false exclusions.

## Evidence

The current-source affected plan selected all three focused packages at tier 0:
414 selected, 478 excluded, 68 unknowns (66 LANGUAGE_FRONTIER and
2 UNSELECTED_BINARY_CONSUMER). Unknowns remain explicit.

Whole-package Go tests used one separate `script/heavy.sh env GOTOOLCHAIN=local
go test -count=1 -timeout 30m PACKAGE` invocation each, with no identity
environment and no lock override:

- `./internal/liveverify/affected/golang/`: PASS, 10.272s. This includes
  `TestReaderBuiltinIdentity_AFP_V0_9010`: package/local append, variadic
  argument append, shadowed conversion, unknown environment, builtin control.
  Each checks actual selection on a duplicate-import test-file addition.
  `TestCallerFilenameProvenance_AFP_V0_9010` checks line, block and column
  directives plus an unknown filename transform, and the physical-filename
  control, against an import edit in the remapped target.
- `./internal/liveverify/affected/languages/`: PASS, 0.046s.
- `./internal/liveverify/affected/`: PASS, 115.294s.

`make -s spec-requirements-check line-citations-check`: PASS on the staged
change. Citation repin found no unchanged citation content that moved.
The required Make target also ran its own focused specindex/lrfrepo checks
through heavy.sh; those passed. Go toolchain: go1.27.3.

Raw logs are outside the repository in `~/lanes/selmin-evidence/`.
SHA-256:
- `self-review-golang.log`: `68417d2e2f4a8c56fbc94b6e315b8e3083ea82c7e0591163bd3413df4de62dc9`
- `self-review-languages.log`: `5805c763910e9ba3ae4a1c7332f2456e895995486b1c10305cd0a93b70dcf96e`
- `self-review-spec-checks-final.log`: `a3ebd7e48a1f47f13d2a15cbb0868df8307ad8bfa7e8bb8ce8de8d52435b13f9`

- `self-review-affected-tests.log`: `f20603ce074496c56f4003d1e2688eb6daed6ad83d7f44ed1d1c42e70e184538`

## Limits

Independent review and host-wide gates belong to the orchestrator. No new
pre-change capture, broad gate, golden regeneration, seal, publication or Tasks
write was run. Linux runs do not qualify Darwin-only files. No performance
improvement is claimed. The final binding report and any NOT_PRODUCED rows are
retained in the external handback; structural acceptance is not semantic review.
