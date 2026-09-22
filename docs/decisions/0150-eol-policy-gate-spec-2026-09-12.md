# Decision 0150 — EPG-V0 owns and accepts the existing end-of-line policy gate

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation, recorded as owner
instruction on 2026-09-12.

## Context

Decision 0061 selected repository-wide `* -text` and left a `git ls-files --eol` drift check as
backlog. That check now exists as `script/check-eol-policy.sh`, is paired with a four-case fixture
test, and both targets run in `make gate`, but no numbered specification owns its behavior.
Decision 0105 accepted the sibling SRG-V0, GAG-V0, and OACS-V0 specifications after their named
defects were repaired and explicitly left EPG-V0 absent.

## Decision

1. Accept `docs/specs/eol-policy-gate-v0.md` as EPG-V0. Its intent is the script's current closed
   behavior: every tracked path reports exact `attr/-text`; only two exact interop fixture paths
   bypass rejection of indexed CRLF or mixed endings; all detected drift is reported before exit.
2. Record delivery as `implemented`. At `1c951c53`, the unchanged live check exits 0 over 3064
   tracked paths with 2 exempt fixtures, and `script/check-eol-policy_test.sh` exits 0 with four
   cases covering a clean tree, attribute drift, committed CRLF, and both exempt fixture paths.
3. Preserve every uncovered evidence boundary as `NO TEST` in the specification. Acceptance does
   not invent coverage for mixed-only non-exempt input, exempt-path attribute drift, multiple or
   simultaneous failures, malformed Git output, Git process failure, different-cwd invocation, or
   write observation.
4. Change no script, fixture, `.gitattributes` rule, Makefile target, or gate membership. A later
   behavioral change must amend EPG-V0 and carry its evidence in the same commit.

## Consequences

The final `make gate` script that lacked a numbered owner now has accepted intent, executable
implementation, and honest test boundaries. Decision 0061 remains the authority for the
repository-wide attribute choice; EPG-V0 owns only its mechanical enforcement.

## Rollback

Revert this decision, EPG-V0, and their index rows. That restores the documentation backlog without
changing the existing gate or the repository's line-ending policy.
