# Heavycap package-list boundary repair

Date: 2026-11-07. Scope: V1-5007 / ASD-V0-2478, self-review P1 and P2.
Repair starts at 76dc3ea5a942bcba9a3f444dcee401bf44622925; lane base remains
c3b29cbb54d31c8526d3cce4ad74356b6fb9b943.

## Findings and decision

P1: `go test ./p -run=^TestOther$ ./p -run=.` could receive whole-source
admission while Go passed the second positional group to the test binary,
leaving the selective first pattern effective. The installed Go testflag.go
confirms that the package list ends at a following flag; later positional
arguments start the binary tail. P2: package collectors had independent,
incomplete value-flag lists, so `-outputdir DIR` could be mistaken for a
package even though `-outputdir=DIR` was understood.

Close the class under ASD-V0-2478: both selection scanners and all four
admission package collectors track the single contiguous package list.
After a subsequent flag, an unconsumed positional argument cannot prove
whole or narrow selection. This includes package-looking and unknown words,
without a denylist of their contents. Known flag operands are consumed before
boundary handling. All package collectors use the existing closed
`go_test_flag_arity` table and syntax validation; unknown flags and missing
operands cannot certify collection. The understood grammar is known flags,
one contiguous package list, then known flags, with inline or separate values
according to the table. Existing explicit `-args`, aliases and terminator
policies remain in force. No new flag is admitted by this repair.

This amends the existing requirement's parsing detail under the owner's
heavycap repair charter. No wire field, diagnostic text, exit code, golden,
core-freeze output or existing test assertion changes. Rollback is a revert
of this repair; it restores the two known admission defects.

## Acceptance evidence

`TestASDV02478SourceEditRunsExistingTests` now requires refusal for the exact
P1 command and an unknown positional tail, in both narrow-to-whole and
whole-to-narrow orders. It checks both scanner results and wrapper refusal.
A contiguous package list with trailing known flags remains admitted.
For P2, `-outputdir`, `-pgo`, `-modfile` and `-coverprofile` each execute the
broken existing test with inline and separate operands, before and after
the package list (16 execution cases). Existing assertions remain intact.

- `corvint affected --base c3b29cbb54d31c8526d3cce4ad74356b6fb9b943` selected
  tools/test-commands-check; scope UNKNOWN, 365 selected, 526 excluded,
  69 LANGUAGE_FRONTIER unknowns. Stale installed binary notice retained.
- Go toolchain check: go1.27.3. `script/heavy.sh env GOTOOLCHAIN=local go test
  -count=1 -timeout 30m ./tools/test-commands-check` with an explicit charter
  whole-package reason: PASS, 64.750 seconds, no -run filter.
- `script/gen-spec-requirements.sh --write`: PASS, no generated change.
- `script/check-line-citations.sh --repin` after staging: no moved citations.
- `make -s spec-requirements-check line-citations-check`: PASS. Its package
  checks use heavy.sh, one package per command. Heavy slot wait: 19 seconds.
- `bash -n script/heavy.sh` and `git diff --check`: PASS.

Raw evidence is outside the repository in heavycap-evidence. SHA-256:

- boundary-package.log (whole-package PASS, 64.750 seconds):
  `8defd0b4dfb194f94bcb92fc8666768006d28b9c03838c081a6eeddd636787b1`.
- boundary-affected.json (selection counts above):
  `f0f479f531ec3736e6e532c13cff413dba2cf002d5250386655013ce766784f7`.

## Limits and route disposition

Used: existing pre-change capture, affected selection, requirement-to-test
traceability and focused package verification. Final CEM binding and
post-commit dogfood-check are recorded in the external lane handback.
Independent review is orchestrator-owned and NOT_RUN here. Broad selection,
doc-gates, full gates, publication, native Tasks writes and seal are excluded
by the repair charter. No fresh pre-change capture was run. The previously
recorded external pre-commit integration blocker remains; native ticket
completion is NOT_PRODUCED. This repair does not claim external qualification.

Friction: configured runner reports /tmp free=1G below its 3G floor but
proceeds; the required private TMPDIR is used, without a lock override.
Initial repin noted unstaged inputs; staging and rerunning resolved it.
