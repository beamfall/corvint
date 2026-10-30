# Decision 0382: Core store promotions do not require the `public-release` gate

Date: 2026-09-24. Status: accepted (owner answer 2026-09-24: "Make it optional"). The owner
authorized the v0-5, v0-7 and v0-8 promotions on 2026-09-24 ("you can do the promotes").
Tickets: V1-0020, V1-0021. Applies `PRS-V1-005` to the task-store release policy. Decision 0381
critical-path step 6.

## Finding

Policy version 2 marks all six gates `required: true`, so `release readiness` needs a passing
attestation of each one for every release. The `public-release` gate runs `make
public-release-check`, which qualifies an installed companion bundle only, in `editor` or `core`
mode (companion profile `/2`, `PUB-V0-020`). Both modes need a companion bundle directory, a Node
and npm cache, Playwright browsers, Python and, for `core`, a Go authority bundle. A Core-only
release has none of these inputs. The v0-5 attestation of 2026-09-23 recorded it as `FAIL` (exit 2,
bundle directory unset, inputs unrecoverable), and no Core release could be promoted.

`PRS-V1-005` says a Core release candidate MUST NOT require companion input, and that companions
keep their own qualification and version. A store gate that requires companion qualification before
any Core release can be promoted contradicts that.

A second blocker surfaced while collecting evidence at the 0.8.1 release commit `1281e26`: `make
companion-release-gate` failed there. Its installed smoke still required five `corvint-mcp` tools,
but decision 0374 (`caf742b`) had returned the default profile to `corvint.query`, `corvint.impact`
and `corvint.status`. This change corrects the smoke to the three default tools; the gate passes at
the fix commit `222b51d`.

## Decision

1. Policy version 3 sets the `public-release` gate to `required: false`. Its command is unchanged.
   A release that lists `public-release` in its own `requiredGates` still requires it
   (`release.Assess` requires every gate that the policy marks required or the release names). No
   release names it today, so a companion release that needs the gate must add it to its own
   `requiredGates`.
2. The other five gates stay required: `focused-docs`, `full-gate`, `interop-gate`,
   `companion-release` and `release-checklist` (`script/release-checklist --pre-promotion`).
3. v0-5, v0-7 and v0-8 are captured, attested and promoted in order at one primary-checkout `HEAD`,
   the merge commit of the change that records this decision and the smoke fix. Candidate,
   readiness and promotion compare `HEAD` and the source digest outside `.taskman/` (corvint-tasks
   `internal/release/release.go` `Assess`), so the chain runs with nothing outside `.taskman/`
   changed. One run of each gate at that commit is attested to each candidate, and
   `release-checklist` runs before the `v0.8.1` tag exists, since at a later `HEAD` a present tag is
   not a child of `HEAD` and the tag row fails. The gate logs and their sha256 are kept under
   `~/projects/corvint-release-evidence/`, and the build log records the promoted head.
4. Every acceptance criterion of a release is attested by its `full-gate` run, whose `make gate`
   runs the tests that back the release's COMPLETED tickets. The other gates attest criterion 0.
5. v0-6 is not promoted here. It still waits for V1-0011, which waits for the owner's V1-0184 run.

## Non-goals

This does not qualify any companion, change `make public-release-check`, or change the 1.0 release
definition. It does not promote the published 0.7.0 or 0.8.0 archives: a store promotion binds the
captured head (decision 0381 item 10).

## Rollback

`corvint-tasks policy update` to version 4 with `public-release` required again. Promotions are not
reversible in the store; a wrong promotion is corrected by a later release, never by rewriting the
journal.
