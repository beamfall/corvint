# Decision 0555: spf13/viper is the untouched public repository for V1-0019

Date: 2026-10-23. Status: accepted owner delegation, conditional on the provenance check below
finding no prior Corvint use. Ticket: V1-0019 (PRS-V1-008). Build-log entry:
`docs/build-log/2026-10-23-v1-0019-viper-selection.md`.

## Owner instruction

On 2026-10-23 the owner wrote: "Pick a suitable mid-size public Go repository yourself for
V1-0019 and record the choice before freezing cases." The orchestrator made the pick under that
delegation. This record is written, and committed on its own, before any clone of the repository
and before any corpus derivation.

This supersedes, for this selection only, the method in decision 0539 item 1 (a preregistered
pool of candidate repositories and a committed coin). The owner's later instruction delegates the
choice directly. The rest of 0539 item 1 still holds: urfave/cli does not count as held out,
because the V1-0859 and V1-0431 fixes were derived from its misses.

## Selection

1. Repository `https://github.com/spf13/viper.git`, pinned at
   `528f7416c4b56a4948673984b190bf8713f0c3c4`. That is the public `HEAD` (`refs/heads/master`)
   that `git ls-remote --symref` advertised before any corpus was derived. The pin stays fixed
   even if `master` moves.
2. Reasons. viper is a mid-size Go library for configuration loading. It is not an HTTP router
   like go-chi/chi, and it is in a different domain from the earlier candidates (go-chi/chi for
   rc.1 run-001, spf13/cobra prepared and never run, urfave/cli for rc.2 with orientation FAIL
   under decision 0427). A bounded `git grep` of this repository at `origin/main` `368b80d4`
   found no earlier mention of spf13/viper.
3. Admission is conditional. If the bounded declared and historical corpus identity check that
   decision 0432 defined finds prior Corvint use of viper, admission stops. The owner is then
   asked for direction. No other repository is substituted under this decision.

## Unchanged protocol

The protocol is the one decision 0432 items 2 to 5 set for urfave/cli. Only the repository URL,
the pin, the module and the repository-specific loop descriptions change:

- the existing `benchmarks/untouched-repository-v1/harness.py`, used without modification;
- the 20 most recent first-parent commits ending at the pin, with no pathspec and no new path
  exclusions;
- the C1/C2/C3 thresholds, controls, measurement count, exclusions, failure treatment and
  no-tuning rules;
- the preregistration is sealed with its `.sha256` file before any Corvint invocation against
  the cases.

Execution is not part of this decision. The cases run later, on the frozen candidate.

## Rollback

Withdraw the selection before execution. Keep this record and any admission or preregistration
files it produced. A withdrawal does not make urfave/cli or cobra held out again.
