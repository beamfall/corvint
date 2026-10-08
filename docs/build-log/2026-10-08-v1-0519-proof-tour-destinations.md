# 2026-10-08: Proof tour refuses existing ACK and identity destinations (V1-0519)

## Intent

Ticket V1-0519 records an independent review of candidate `eaad98d6`: resume wrote the
retained ACK copy and tool-identity receipt without collision checks, so a pre-created
`resume-1-ack.txt` symlink had its target overwritten. The change adds proposed `PT-V0-008` to
`docs/specs/proof-tour-v0.md`. Owning feature V1-0504; root integration V1-0516.

## Findings on the base

Base `ecfff8e0` already contains the round-wide preflight from `d2425f7f`
(`script/proof-tour.sh`, the `for auxiliary in ack.txt tool-identity.txt` loop). With a paused
real tour, the base focused tests pass, including the symlink collisions for `ack.txt`,
`tool-identity.txt` and `ready.stderr`. The base gaps were:
regular-file collisions were not tested, no test showed that a collision-free copy still passes
the preflight, and the two writes (`cp`, `printf >`) followed a symlink that appeared after the
preflight.

## Decisions

- **Defense in depth, not a second policy.** The ACK copy is now `cat` under `set -C` in a
  subshell, and the identity receipt is written under `set -C`, refusing with
  `retain-ack-failed` or the new `retain-identity-failed`. Bash noclobber opens absent paths with
  `O_EXCL`, so a dangling symlink is refused, and an existing regular file is refused. The
  preflight remains the primary refusal (`resume-output-collision`).
- **No fabricated accepted ACK in the maintained tests.** The tests keep their existing rule:
  only rejected/stale MOCK_TEST_ONLY ACKs. Clean-resume completion was exercised only in scratch
  with a mock ACCEPT. That run is mechanical evidence, not a review.

## Evidence

- Failing before: the new tests run against a mutant with the auxiliary preflight and noclobber
  removed fail at `collision-symlink-ack.txt exit=3 expected=2`.
- Passing after: `PROOF_TOUR_TEST_REVIEW_OUT=<paused tour> script/proof-tour-test.sh`, 31 PASS
  lines with exit 0. These cover symlink and regular-file collisions for `ack.txt`,
  `tool-identity.txt` and `ready.stderr`. In each case the sentinel or existing receipt stays
  byte-identical, the symlink is not replaced and no partial patch receipt is written. A
  `clean-resume-admitted` case reaches ACK admission.
- Scratch, mock ACCEPT ACK (`reviewer=MOCK_SCRATCH_ONLY`): a clean resume exits 0
  (`outcome=complete`) with a byte-identical retained ACK. With only the preflight removed,
  pre-existing symlink and regular `resume-1-ack.txt` destinations both refuse with
  `retain-ack-failed` and keep their bytes.

## Independent review

Codex (`gpt-6-astra`, read-only) reported no P0/P1. Its P2 was that the maintained tests never
reach the new no-clobber writes, because collisions stop at the preflight and the clean case uses
a rejected ACK. Disposition: accepted as a coverage limit, not fixed. Reaching either write needs
an accepted ACK, which the maintained tests deliberately never fabricate, or a race-injection
hook in the production script. The scratch mutant that keeps no-clobber and removes only the
preflight (above) isolates that layer. It is unmaintained mechanical evidence, and the spec
witness row says so.

## Non-goals and failure modes

This change is not a hostile-filesystem sandbox. A concurrent writer that swaps a destination for
a non-regular target between the preflight and the write (for example a symlink to a device) is
outside this developer-script guarantee. Bash noclobber permits non-regular targets. Step
receipts written by `run_step` still rely on the preflight alone. A collision leaves the round
blocked until the operator removes the destination, and nothing is deleted.

## Rollback

Revert this commit. The base preflight from `d2425f7f` still refuses collisions.

## NOT_RUN

- An actual independent ACCEPT resume of a real tour (semantic review) is NOT_RUN in this lane.
- `make gate` and repository-wide tests are NOT_RUN (scoped work).
- Root integration (V1-0516) and the native ticket completion are left to the orchestrator.
