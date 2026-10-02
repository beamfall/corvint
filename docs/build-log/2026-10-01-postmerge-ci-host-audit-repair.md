# Post-merge CI host audit: local action and runner-command repair

Date: 2026-10-02. This entry corrects the audit coverage described in
[the original CI host entry](2026-10-01-postmerge-ci-host-templates.md), following independent
review of [PR 458](https://github.com/beamfall/corvint/pull/458). The filename was reserved with
the earlier client date. PCH-V0 remains proposed and experimental; ticket V1-0545 remains open.

## Decision

PCH-V0-006 requires every action to be pinned to a full commit. The old owner grammar admitted
`.` and therefore accepted `uses: ./config/evil@<40-hex>`. GitHub's runner treats that value as a
local action path, including the apparent commit suffix. The owner must now begin with an ASCII
letter or digit. The rest of the reference grammar is unchanged.

PCH-V0-007's environment-command check now covers both `::` and legacy `##[` command prefixes,
with command names matched without regard to case. The runner registers `set-env` and `add-path`
case-insensitively and parses both forms. This closes the observed uppercase and legacy-command
misses within the existing environment-change restriction; it does not interpret shell programs.
The spec and operator README state the exact new coverage.

Official source checked on 2026-10-02:

- [PipelineTemplateConverter](https://github.com/actions/runner/blob/main/src/Sdk/DTPipelines/Pipelines/ObjectTemplating/PipelineTemplateConverter.cs)
  distinguishes local references before remote action references.
- [ActionCommandManager](https://github.com/actions/runner/blob/main/src/Runner.Worker/ActionCommandManager.cs)
  uses a case-insensitive command registry and tries both parsers.
- [ActionCommand](https://github.com/actions/runner/blob/main/src/Runner.Common/ActionCommand.cs)
  defines the two command syntaxes.

## Evidence

The unchanged package baseline passed. With the new tests and old guards, seven new cases failed
because the audit returned no finding: a local action with a commit suffix, uppercase modern
`SET-ENV` and `ADD-PATH`, and both commands in lowercase and uppercase legacy forms. The additional
lowercase modern `add-path` case already passed and now protects that previously untested alternative.
After the repair, the focused package passed, including the shipped templates' zero-finding check.

Six independent mutation checks each failed the intended named subtest with its missing finding:

| Removed protection | Failing subtest of `TestAuditRefusesUnsafeTemplates` |
|---|---|
| First owner character restriction | `local_action_with_commit_suffix` |
| Command case folding | `script_prints_::SET-ENV` |
| Modern command prefix | `script_prints_::set-env` |
| Legacy command prefix | `script_prints_legacy_set-env` |
| `set-env` alternative | `script_prints_::set-env` |
| `add-path` alternative | `script_prints_::add-path` |

The mutations used separate scratch Go overlays. None changed the tested source file, and each
failed for the expected missing `UNPINNED_ACTION` or `RUNNER_ENV_FILE` finding, not a build failure.

Focused tests and vet passed for `internal/postmergehost` and `internal/specindex`. The required
`spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
`decision-numbers-check` and `line-citations-check` targets passed, along with Go formatting and
line-ending checks. Final enrolled checks bind these results to the committed change.

## Qualification and rollback

The audit sees literal workflow text. It cannot see indirect environment-file access, command
strings constructed at run time, code inside pinned actions, or runner-host settings. It does not
prove runtime credential isolation. Hosted replay remains `NOT_RUN`; the delta step remains
`NOT_PRODUCED`. The source test and declared-YAML results do not promote those missing outcomes.

The prior seal was reverted by an ordinary commit. Evidence for this repair is rebound against
the original base `ff3da727e95a1999cbc4a58a471cdd498693f902`; earlier source-evidence limitations
remain visible. Full repository validation is `NOT_RUN` under the owner's scoped issue policy.
Rollback is an ordinary revert of the repair's source and documentation changes, followed by a new
evidence binding. Do not restore an obsolete seal or reuse checks after source changes.
