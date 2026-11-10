# Design: `agent-packages`

## Decisions

- End-of-turn delivery: a blocking `Stop` decision with the report as its reason, at most one nudge
  per turn, guarded by `stop_hook_active` and by the working-tree digest (owner decision R5,
  2026-11-09; review fix F1). This replaces the non-goal "No shim returns a block decision".
- After compaction: `SessionStart` with source `compact`, not `PostCompact` (owner decision R5;
  review fix F18). The Codex equivalents of both are NOT_VERIFIED and are checked in task 3.
- The session's changes are the session-start commit diffed against the working tree; after-edit
  records only add paths (review fix F19).
- The receipt flag reads the PROD-TEST-020 run result record and says "no receipt" (owner decision
  R7; review fix F20).
- Unknown `CORVINT_SELECTION` value: the hooks report "UNKNOWN, run all" with a reason and exit 0;
  the CLI and CI exit 2 (owner decision R13).
- Docs review is not used by any hook; if later added it is advisory unless the repository opts in
  (owner decision R4). The end-of-turn report uses `affected` plus receipts (contradiction 9).
- MCP: `corvint mcp` for testing tools and `corvint-docs mcp` for docs tools, names and schemas from
  change `mcp-tool-table` (owner decision R30). No Tasks MCP server; the tasks skill uses the
  `corvint-tasks` CLI (owner decision R17).
- Licence of `beamfall/corvint-agents`: Apache-2.0 (owner decision R25; review fix F12).
- Qualification: one pinned model per host, model ID and agent version recorded (owner decision
  R27; review fix F21); runs use the owner's logged-in subscription CLIs (Claude Code, Codex CLI) on
  the Mac, WSL or Unraid hosts or a self-hosted runner there, with no API keys, no spend cap, never
  on GitHub-hosted runners and never on fork `pull_request` or `pull_request_target`; one evidence
  file per release candidate plus a weekly run against the newest agent versions (owner decisions
  R39 and R40, 2026-11-09, amending R26). AGT-V1-029 is rewritten accordingly.
- End-of-turn budget: the p95 latency on the seven pinned repositories is recorded and the 3 s
  budget is set from it (review fix F23, AGT-V1-030).

## Failure modes

- Corvint binary missing, or outside the package's supported range: the shim prints one notice
  naming the installed and supported versions and exits 0; doctor reports it (AGT-V1-006).
- `corvint agent hook` times out, crashes or returns invalid output: the shim prints "UNKNOWN, run
  all" at end of turn or nothing for after-edit, and exits 0. The edit is never blocked (AGT-V1-015).
- The plan is UNKNOWN or FULL: the report says "UNKNOWN, run all" with the reason code.
- The agent version is outside the package's qualified list: doctor warns; hooks still run
  (AGT-V1-025).
- A workflow that sends qualification jobs to the owner's self-hosted runner gains a fork or
  `pull_request_target` trigger, targets a GitHub-hosted runner, or reads an agent API key: the
  workflow lint fails (AGT-V1-029). (Earlier wording named a "key-bearing" workflow; superseded by
  R39.)
- A subscription login has expired or its usage limit is reached during qualification: the
  scenarios not run are NOT_RUN in the release candidate's evidence file and the gate for that host
  waits for a rerun (AGT-V1-029).
- The end-of-turn hook would nudge twice: the loop guard (`stop_hook_active`, same tree digest)
  returns no block decision and the turn ends (AGT-V1-013).
- No PROD-TEST-020 record exists yet: every selected test shows "no receipt" and AGT-V1-014 cannot
  be qualified.
- Measured end-of-turn p95 exceeds the budget: the budget is raised with evidence or the hook is
  changed (AGT-V1-030).
- The host has not trusted the project or the hooks (Codex `/hooks`, Pi project trust): doctor
  reports the hooks as not active and names the host step to enable them.
- `CORVINT_HOOKS=off`: every shim exits 0 immediately with no output; doctor reports the switch.
- Session state is missing, corrupt or expired: treated as no recorded edits; the end-of-turn report
  falls back to the diff against the session-start commit.
- The host changes its payload shape: the binary rejects the payload with a reason code, prints
  "UNKNOWN, run all", and the weekly qualification fails and files a report (AGT-V1-025).

## Superseding decision

Decision 0594 item 1 (`docs/decisions/0594-refocus-on-test-selection-2026-11-09.md:42-43`) cut
context work and names "the OpenCode qualifier, the native hook observer"; commit `841a58cbd` removed
the hook adapters, MCP plugin wiring, the host lifecycle qualifier and the Gemini, OpenCode and Pi
packages under it. PRS-V1-006
(`docs/specs/corvint-1.0-product-and-release-v1.md:345-347`, evidence row at `:532`) limits Core
hosts to the plain CLI plus Codex and Claude Code at FALLBACK and says FULL support for any host
must not block Core. It is also cited by `host-lifecycle-qualification-v1.md:12,26,83` and
`public-release-v0.md:93`. This proposal reverses part of the first and replaces the second.

The superseding decision MUST state:

1. Which parts of 0594 item 1 and commit `841a58cbd` are reversed: after-edit, end-of-turn,
   session-start and compaction hooks, MCP registration in packages, and OpenCode and Pi packages
   return as a rebuild under this proposal. The text of 0594 does not name hook adapters or packages
   explicitly; the decision states the reversal against the commit.
2. Which parts stay cut: context injection, retrieved-context MCP tools, LSP, the evaluation runner,
   the shared `corvint-harness-event/0` profile and Gemini CLI.
3. PRS-V1-006 is replaced: Claude Code and Codex at the full bar of this proposal block 1.0,
   replacing "at FALLBACK" and "Formal FULL ... MUST NOT block Core"; OpenCode and Pi are preview and
   never block; Pi is opt-in. The FALLBACK and FULL host vocabulary and the nine HLQ cases are
   replaced by the G21 definition (AGT-V1-023).
4. `agent-harness-integration-v0.md` and `host-lifecycle-qualification-v1.md` stay retired as
   history, and the citations of PRS-V1-006 in them and in `public-release-v0.md:93` are annotated.
5. The new home `beamfall/corvint-agents` is licensed Apache-2.0 (owner decision R25, 2026-11-09),
   the same terms as `beamfall/corvint` since decision 0586 (`LICENSING.md` lines 5-10, integ
   9ec3a29d0), so no licence boundary is crossed and decision 0002 needs no amendment either way.
   Earlier wording, superseded (review fix F12): "its permissive licence sit outside the decision
   0002 licence boundary of `beamfall/corvint` ... if the owner instead keeps packages in
   `integrations/`, 0002 must be amended."
6. The removal of `integrations/` (AGT-V1-027) and the condition for it.
7. The kill switch `CORVINT_HOOKS=off`, the no-injection rule (AGT-V1-019), and the location of
   session state under invariant 4 (AGT-V1-022).
8. Qualification uses the owner's logged-in subscription CLIs on the owner's machines, with no API
   keys and no spend cap (owner decisions R39 and R40, 2026-11-09), and what happens to the 1.0
   gate if those runs have not passed by the freeze. Earlier wording, superseded: "Who provides the
   agent API keys and CI spend for qualification (G21), and what happens to the 1.0 gate if they
   are not provided by the freeze."
9. That the Pi ordering note at `docs/EXTENSION-ECOSYSTEM.md:125` is superseded.
10. That it is part of the single decision of answer 18 that supersedes the pre-cut 1.0 contract,
    not a separate decision, unless the owner chooses otherwise.

## Acceptance evidence

- Unit tests in `beamfall/corvint` for `corvint agent hook` per host and event, using the payload
  fixtures recovered from `841a58cbd^`, including timeout, crash, invalid payload, kill switch and
  out-of-range binary.
- A purity test that hook calls leave `git status` and `.corvint/` unchanged (AGT-V1-022).
- Drift check passing in `beamfall/corvint-agents` CI, and an npm pack audit for the OpenCode and Pi
  packages (AGT-V1-005, AGT-V1-007).
- Retained qualification transcripts for Claude Code and Codex at the pinned versions on Linux and
  macOS covering every AGT-V1-023 scenario, with retries logged.
- A workflow lint result showing logged-in qualification jobs run only on the owner's machines and
  only on the AGT-V1-029 triggers.
- One evidence file per release candidate with the AGT-V1-023 tuple, including model ID, for every
  scenario (AGT-V1-029).
- p50 and p95 end-of-turn latency on the seven pinned repositories, with the budget derived from it
  (AGT-V1-030).
- One weekly run record showing the newest-release check and, if it failed, the untested marking and
  the filed report.
- OpenCode and Pi: a qualification run on the owner's machines when a logged-in client is available
  there, otherwise retained as NOT_RUN with the preview label. (Earlier wording: "if keys allow".)

## Rollback

- Users: `CORVINT_HOOKS=off` (immediate), then uninstall the package; MCP and skills can stay.
- Packages: publish a fixed package version; never delete a published version. npm deprecation is
  used for a bad OpenCode or Pi version.
- Binary: the agent commands are read commands; reverting them leaves shims printing the range
  notice.
- `integrations/` removal: revert the removal commit; the skill-only packages return from history.
- If the change is withdrawn, the 30 IDs are retired, not reused, and PRS-V1-006 stays as accepted.

## Contradictions and open questions for the owner

1. Pi has a built-in MCP client per its extensions page (read 2026-11-09); the panel assumed it did
   not. Settled without the owner: Pi uses its built-in MCP client once confirmed; the AGT-V1-010
   CLI-backed fallback stays generic for hosts without one.
2. Codex has no confirmed non-interactive plugin install, and project hooks need trust through
   `/hooks`. "One install command" may not be achievable for Codex without a manual step.
3. Command names: G19 says `corvint agents doctor` (plural); G20 says `corvint agent hook` and
   `corvint agent manifest` (singular). Proposed: keep both as written, or choose one verb.
4. Product invariant 4 says read commands do not mutate repository or trace state. Hook session state
   is placed outside the repository (AGT-V1-022); the owner confirms this is allowed or amends the
   invariant.
5. Commit `841a58cbd` is 209 files and 27,208 deleted lines, not 133 and 16,800 as the panel brief
   says.
6. Where the weekly run files its report: proposed as an issue in `beamfall/corvint-agents`, because
   CI does not write to a Tasks store. Settled without the owner: the weekly run files a GitHub
   issue in `beamfall/corvint-agents`, and a person mirrors it to the Tasks store.
7. Settled by owner decisions R39 and R40 (2026-11-09): qualification uses the owner's subscription
   logins on the owner's machines, with no API keys and no spend cap. Earlier text: "Agent API keys
   and CI spend for qualification are not arranged (G21). Without them the 1.0 gate for Claude Code
   and Codex cannot pass."
8. Settled by owner decision R25 (2026-11-09): Apache-2.0.
9. AGT-V1-013 and AGT-V1-014 overlap with the preview `review` evidence check (G25 item 2); the
   end-of-turn report blocks 1.0 while `review` is preview. Proposed: the end-of-turn report uses
   `affected` plus test receipts directly and does not depend on `review`. Settled without the
   owner: adopted as proposed.
10. How many passes out of how many runs make a qualifying pass (NEW-AGT-2). Open.
11. Whether the Codex `Stop` block reason reaches the model, and whether Codex `SessionStart`
    carries source `compact`. NOT_VERIFIED; checked before AGT-V1-013 and AGT-V1-018 are built for
    Codex.
