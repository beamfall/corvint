<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4729: raw filename membership selects the ownership ratchet

Base: `654df5053cc3a5f36aa4ecb48af36c5eaea0a86c`.
Starting head: `9aa6ddb34e5f9c64831e772104b20b8838afb7fb`.
Intent: owner self-review repair; AZU-V0-9900 refines AZU-V0-002.

## Finding and decision

Git membership retains raw filename bytes, but dirty/range paths replace invalid UTF-8
with U+FFFD. Looking up the normalized dirty path in raw membership maps misses tracked
additions and deletions in hidden directories. Normalizing membership before comparison
would also miss a replacement whose two raw names have the same normalized form.

Compare raw base/index membership first, then convert the differences to the existing
dirty-path representation. Check tracked worktree existence with raw names and convert
committed names for the planner's existing membership annotation. Keep raw observations
for index drift checks. No wire, diagnostic, golden or existing assertion changes.
Rollback is a revert of this repair; it restores the documented invalid-name limitation.

## Evidence and limits

`TestOwnershipInvalidUTF8Membership_AZU_V0_9900` exercises hidden-path committed
additions/deletions, staged additions, worktree deletions, colliding-name replacements
and content-only edits. Running it against the starting implementation failed the five
membership-change cases; the content-only boundary passed. The final implementation
passes all six cases and checks the normalized triggering path.
`TestOwnershipInvalidUTF8IndexDrift_AZU_V0_9900` also passes: a raw index rename preserving
the displayed path still causes the existing drift refusal. Both witnesses are Unix-only;
Linux execution supplies no macOS or Windows runtime evidence. Whole CLI package results
and final dogfood qualification are retained in the external lane handback.

Raw evidence lives under `/home/russell/lanes/ratchetsel-evidence/`:

| Artifact | SHA-256 |
| --- | --- |
| `utf8-before-regression.log` | `3356c5f7f316cedbafd27b2695406d9420f5a5385f2d825ee0aa3eafe80ed52e` |
| `utf8-focused-final.log` | `0fab37f080fb98a7676c523d5a1c58834ad60f3d75a8968d49e95dcd50ee102f` |
| `utf8-affected.json` | `a19d260e847eb1149007f354f71dbb42c49f4138851b591860d7c8cf862764f4` |

The pre-fix reproduction accidentally omitted the explicit TMPDIR assignment; heavy.sh
used `/home/russell/.cache/corvint-tmp`. Final focused and whole-package commands use the
required `/home/russell/lanes/ratchetsel.tmp`. No lock-directory override was used.
Independent review and host qualification belong to the orchestrator; no completion
write, publication or seal is claimed.
