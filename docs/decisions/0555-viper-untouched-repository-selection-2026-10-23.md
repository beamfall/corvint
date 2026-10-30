# Decision 0555: spf13/viper is the untouched public repository for V1-0019

Date: 2026-10-23. Status: accepted owner delegation. The provenance check below found no prior
Corvint use, so the repository is admitted. The cases are frozen and have not been executed.
Ticket: V1-0019 (PRS-V1-008). Build-log entry: `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-v1-0019-viper-selection.md`.

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

## Admission finding

The selection above was committed on its own (`a4ce7bec`) before viper was cloned. The check
that decision 0432 defined then ran, with the same scope. Its path identities, ref inventory
digest and limits are in `benchmarks/untouched-repository-v1/viper/admission.json`.

- No viper identity in 21 current declared metadata files: the development and blind manifests,
  every existing preregistration, the urfave admission packet, the performance and query-start
  manifests, the cw-trial held-out manifest, the blind-v7 ticket payload and the two bench
  READMEs.
- `git grep -i viper` over every tracked path at `368b80d4` found nothing. That covers
  `benchmarks/` (including the 61 retained result files and the snapshot fixtures), `docs/`,
  `conformance/` and the frozen ARB v2 fold map, which lists 25 repositories and not viper.
- `git log --all -i -G viper` over `benchmarks`, `docs`, `conformance`, `tools/retrieval-bench`,
  `.corvint` and `README.md`, across 1790 local refs, matched only this record's commit.
- No match in the primary checkout's `.corvint/` ledgers (`self-observations.jsonl`, the
  evidence and scope files, `changes/`, `history-backups/`, `test-evidence/`). There is no
  `unplanned-reads.jsonl`. No match in the 676 untracked or ignored files under `benchmarks/`,
  `docs/` and `.corvint/`. No local viper or ARB dataset directory was found.

Limits: agent transcripts, sealed case bodies, unreachable Git objects, the Windows benchmark
host and unrecorded private experiments were not searched. This is a bounded absence finding.
Evidence of actual prior use found later stops admission.

## Frozen packet

- Clone: complete, not shallow. Tree `fffffa7740143511f55b4dadf559f8cbcb6ea6f8`, module
  `github.com/spf13/viper`, PIN^ `bdc880d2f5195885636621f8d5e966246258b8a4`.
- The unchanged harness (`b67cc95a...`) derives 20 cases. All 20 are orientation-eligible and 13
  are consequence-eligible. Nothing is excluded under E1-E3. Corpus SHA-256
  `3d39b9338c5a8b00f64062892a12d6786a116936ad2ce4b1ff440c80444b8f65`.
- Preregistration SHA-256 `e2d6a26a77a0d3a1488dccc4d9f897e11fdef738c4409957b605092769e2be5e`.
  `targetVersion` is `1.0.0-rc.3`, the version on `main`. Under decision 0491 item 3, an rc.3
  result is early evidence and does not close V1-0019.
- An independent read-only Codex review of these exact digests returned APPROVE with no blocking
  finding, and only then was `preregistration.sha256` written. The review re-derived the corpus
  byte for byte and checked the loop's structural facts.
- Two open harness tickets are not in these bytes: V1-2281 (distinct expected refusals) and the
  V1-0777 bounded `agent_cost` reader. If either lands before execution, the run needs a
  numbered deviation and a reseal first, because the harness refuses a harness digest that does
  not match.

## Rollback

Withdraw the selection before execution. Keep this record and any admission or preregistration
files it produced. A withdrawal does not make urfave/cli or cobra held out again.
