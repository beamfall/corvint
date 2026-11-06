# pk-affected: self-review driver flag repair

Owner intent: the two P1 self-review findings supplied for lane
`claude/pk-affected`, base `9db7a94169821d9b28701eaa88866aee2fec1083`.
Starting head: `3f2991df50047bd6c5a3be7995cd1e834cce0374`.
The owning ticket remains V1-4684; these repairs implement the existing
AZU-V0-001 directory-equivalence and unsupported-inline-program clauses,
with computed-site closure under AZU-V0-003. They do not claim native ticket
completion or broader zero-unknown qualification.

## Decisions and regression evidence

1. Apply `-C=DIR` and `-C DIR` to the working directory in the flag loop after
   the Go subcommand, using the same directory resolution as the prefix loop.
   `TestDriverGoEqualsDirectory_AZU_V0_001` now exercises both placements and
   forms, source and test-only package changes, a direct workflow job, transitive
   make/CI callers and exclusion of every such caller for an unrelated package.
2. A non-shell interpreter option is outside the shell scanner's supported
   grammar: it can embed a program or consume the next word. Retain its invocation
   site as computed instead of guessing which word names a script. This covers
   long, equals and attached execution flags without an incomplete interpreter
   flag allowlist. The deliberate cost is conservative selection for other
   non-shell interpreter options. Literal script invocations without options and
   existing shell-program scanning keep their existing treatment.
   `TestDriverInlineInterpreterComputed_AZU_V0_001` adds Node eval/print variants,
   attached Python/Ruby/Perl programs, an option before Python's program flag and
   PowerShell's command flag. Constructed package paths avoid incidental literal
   path matches. Both source/test-only changes retain the make target, its
   transitive make/CI callers and a direct job; both sites store the computed
   frontier. An unrelated make target stays excluded.

The expanded regressions fail against the starting parser using a Go source
overlay (rc=1, 0.063 package seconds), and pass against the repairs (rc=0,
0.068 package seconds). The failing overlay log SHA256 is
`9a1f5282105f4f99926953a1e4f33e37be65ac0688d9e575ff174a804346caf4`;
the passing focused log SHA256 is
`dd8e81f1403909da647482ad58780012c29d303cdd60400c4a77ba539a8c4c27`.
Existing assertions were strengthened, not removed;
committed goldens and all frozen wire/error/exit/stderr contracts are unchanged.
No requirement definition changed: the current AZU-V0-001 clause and test matrix
already govern these cases. Rollback reverts this repair, restoring the two
missing-driver risks without a wire migration.

## Verification and limits

The installed `corvint affected --base` at the stated base selects
`internal/liveverify/affected`; its plan is UNKNOWN, 176 selected / 86 unknown.
It reports a stale installed binary, which is retained as uncertainty rather
than represented as final-source selection proof. The existing pre-change
capture is reused; no new `make dogfood-change` capture is run.

The final whole-package run passes every existing affected-package test without
a `-run` filter: 90.724 package seconds / 91.1 heavy runtime seconds. Log SHA256:
`7fc70aced6cb9ebb9b2fa1af4399a0e41dcd8b7671a5b688bc1549b286e70304`.
Windows static qualification (`script/heavy.sh make windows-vet`) passes.
Log SHA256:
`90ea85a89e16d00883e4e2fa3d5b1d322dea096f320987a7b09533d030ae7c94`.
The line-citation repinner found no unchanged-content citation movement;
`make -s spec-requirements-check line-citations-check` passes with the inherited
five capacity warnings and 180 skipped basename citations.

All Go tests run through `script/heavy.sh`, one package per command, with
GOTOOLCHAIN=local, Go 1.27.3, private TMPDIR
`/home/russell/lanes/pk-affected.tmp` (0700), the configured shared lock directory
and no exported commit identity. Raw logs, plans, overlay inputs and report
snapshots stay outside the repository under
`/home/russell/lanes/pk-affected-evidence/self-review-driver-flags/`.
The final binding/check result, command digests and numbered friction repros
are retained in `/home/russell/lanes/pk-affected.handback.md`.
Independent review, broader gates, external/runtime platform qualification,
native Tasks writes, publication and seal are NOT_RUN by instruction.
