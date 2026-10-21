# V1-1463 and V1-1927: batch-check names the merged-forward BASE and the verifier reason code

Date: 2026-10-20. Lane: claude/batch-dogfood-flow (base 5cf75daf0). Requirements: `DCW-V0-110`,
`DCW-V0-111` (owner lane auto-acceptance 2026-10-11).

## Decision (V1-1463)

The ticket offered two routes: admit main-side commits as "main-admitted", or seal Tasks policy
carries. Both were declined as the delegated conservative choice:

- Admitting a commit because a ref such as `origin/main` contains it would make a mutable local ref
  an authority over the binding (product invariant 3), and batch-check has no other evidence that a
  commit passed the mainline's own check.
- Sealing policy carries changes a convention owned outside this lane.

Instead the verdict is unchanged and the failure names the route that already passes: when every
unbound commit is reachable from the second parent of a merge in the range, a second `fix:` line
names that merge and its merged parent as the `BASE` for a merged-forward lane. From that BASE every
lane commit is still checked (the observed `13e1b26d8` case passed from `f81114713` the same way).
The line states the mainline condition and the check command, because a merged lane branch can hold
unbound commits too. This is a deviation from the ticket's first acceptance criterion; the second
(DOGFOOD.md names the BASE a merged-forward lane uses) is met.

## V1-1927

`cem status` already reports `stableVerification.code` (for example `unsupported-object-alternates`
with `state=unsupported`). batch-check now repeats a code shaped as a reason code on the
`CONSTITUENT` line and failure detail, and a code whose cause lies outside the sealed change prints
its own `fix:` line. Verified with the built binary and the real verifier on a fixture whose
`.git/objects/info/alternates` exists.
