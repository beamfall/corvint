# Delta enrollment deviation and adjudicated recovery

Issue: #389 / native V1-0536. Source at audit: `e26a144b9b4ef04147826fcde71af3652be4c548`.
Final base: `7bd7f5e03ad177e7496ce5dbd1563b8466f591a4`. Native attempt remains generation 88.
The sole added recovery path was admitted by same-claim widening receipt 1582; prior scope remains.

## Procedural failure

The coordinator required a fresh distinct checkout/enrollment preserving the original ACTIVE,
unsatisfied workflow. The builder instead cancelled key a0b303 in the same checkout, imported the
approved public base and began key 4da990 there. The builder interpreted preserved raw observations
and a distinct key as sufficient preservation, failing the distinct-checkout/ACTIVE-state boundary.
A supported CLI operation did not authorize that interpretation. Cancellation was non-success;
original ACTIVE state was lost and is not restored or represented as completed.

The original six selected-check observations, prior failure/unknowns, cancellation and receipts
remain retained. Four checks, reports and final check were unmet before cancellation. The original
cancelled status returns unmet=[] because evaluation exits early for cancellation, not because the
workflow passed. The final enrollment retains exactly the same ten check definitions, argv,
timeouts, reuse flags and intent paths; only the frozen base differs. No obligation was dropped.

## Independent audit and recovery authority

The same independent integration reviewer verified all 37 manifest artifact hashes and byte
lengths, base ancestry, intent blob pointers, clean source and retained observations. No receipt
deletion or produced-report loss was demonstrated; the original had no produced final report set.
The audit found current enrollment technically continuable, with coordinator adjudication required.
The coordinator independently verified those 37 artifacts and authorized continuing CURRENT
4da990 with unchanged ten-check plan and same native attempt, without retroactive approval of
cancellation. No new key, reset, history rewrite, acceptance reduction or third semantic repair is
permitted. V1-0581 tracks the concrete agent procedural safeguard gap, not a Core algorithm defect.

Exact evidence:

- `/tmp/corvint-parallel-dispatch/delta389-enrollment-deviation-handoff.json`
  sha256 `4ff66bdffcbae413ea1740089710f70458899720a6b435e9c71d670d4ff9050d`.
- `/tmp/delta389-independent-enrollment-recovery-review.json`
  sha256 `b58e118de349896a266311e826d1d40a129dea2d399c0d26115ce14394df947b`.
- `/tmp/corvint-parallel-dispatch/delta389-root-recovery-adjudication.json` records
  `CONTINUE_EXISTING_FINAL_ENROLLMENT_WITH_RECORDED_DEVIATION` at 2026-10-01 06:11:50 UTC.

## Observed binding, cleanup and remaining obligations

Neither enrollment contains enrollment-owned initial prechange receipts. Independent prechange
outputs remain retained with context revision `6776375514a49dc777c8ab8ff2369cd2d9c5d0a4`, distinct
from original public base 01f557; later coordination does not establish final-base prechange context.
Missing wrapper receipts remain NOT_OBSERVED/NOT_PRODUCED. No chronology is recreated.

At the hold, three fresh final-target checks were qualified; the running Git/external-evidence check
was interrupted and is unqualified. The owned runner, verifier, Go test and Git descendants were
retired by SIGTERM cleanup; all six recorded PIDs were independently absent at audit.
The first focused-check failure remains retained: fixture descendant never started, followed by a
same-input retry pass. The fixture polls 200 times at 10 ms; earlier one-second wording was
incorrect. V1-0582 tracks this suspected fixture robustness gap; cause and production implications
remain unknown. Interruption, cancellation and retry never substitute for a passing bound check.

This entry changes the target before verification. Every original ten-check obligation must have
binding-valid final observations; current completion is not claimed. Strict CEM/OCM, inspected
reports, the same independent final reviewer, seal, native gate and ready publication remain required.
Native docs-only remains blocked by the current strict-provider/selected-test contradiction;
V1-0579 and original four-class acceptance remain PARTIAL pending explicit owner disposition.
Preserve both enrollment histories and source worktrees throughout recovery and closeout.

## Documentation ownership qualification

At the recovered clean target e56825, the focused units, public CLI, all eight provider packages
and full Git/external-evidence packages passed. The docs gate then refused 25 newly emitted codes
without an owning spec. The DLT contract now documents those refusal/uncertainty meanings, including
the shared captured-record bound, with no compiler/profile change. This is a documentation repair
within the admitted paths; source semantics and the ten frozen check definitions remain unchanged.
Its new target requires binding-valid terminal checks. The new public main 901618 test-merge was
assessed in scratch: no conflict markers, all public INDEX entries/requirements preserved, plus
one DLT entry and its ten canonical requirement rows. No public import, base/enrollment change or
foreign-seal edit is necessary merely for that advancement; hosted test-merge validation is required.
The registry conformance check also required the INDEX claim and README first sentence to match
the short Agent digest claim exactly; that prose alignment is repaired without changing intent/status.
