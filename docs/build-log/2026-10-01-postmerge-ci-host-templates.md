# Post-merge CI host templates and adapter contract

[Issue 398](https://github.com/beamfall/corvint/issues/398) (part of #388) asks for reference CI
templates and a host adapter contract for the post-merge workflow. This entry proposes PCH-V0
(`docs/specs/postmerge-ci-host-v0.md`) as an experimental contract. Native ticket V1-0545 stays open.

## Decisions

**Location and license.** The templates live under `protocol/postmerge-host/` (Apache-2.0, per
`LICENSING.md`), because operators copy them into their own repositories. The graph validator and
audit live under `internal/postmergehost` (AGPL). No Core command, no installed binary and no
network path are added, so product invariant 7 is unchanged.

**Graph before templates.** A host-neutral `workflow-graph.json` is the contract. It defines nine
ordered steps, six credential classes with disjoint secret prefixes, and the documented commands for
each step. The GitHub Actions files are one implementation of that graph.

**Credential split.**
- Only the authoring job runs an author with worktree write access. It is a separate job holding
  only the `agent-model` class and a read-only token.
- The trusted job re-exports binding and source item instead of trusting author output.
- The reference pipeline holds no outward-write secret at all, because remote live writes are
  unsupported (PMC-V0-006). Writer jobs are split out only once a live adapter is qualified.

**Post-merge trigger and recovery.**
- The source trigger is a push-only, `continue-on-error`, five-minute dispatch, so it cannot block
  the merged change request.
- Nightly reconciliation retries lost dispatches.
- Replay is a manual run by full change id or by a committed replay-set file.

**Audit scope.** go.mod has no dependencies, so the audit parses a restricted YAML subset. It refuses
anchors, aliases, tags, flow mappings, folded scalars, duplicate keys, tabs and unmodelled job keys,
rather than guessing. The audit flags:
- write classes, write permissions or `github.token` in authoring;
- unclassified or dynamic secrets;
- workflow-level secrets;
- unpinned actions;
- persisted checkout credentials;
- expressions in scripts;
- a Corvint binary run before the pinned install;
- commands outside the step's documented set;
- order violations against `needs`.

## Evidence and limits

`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/postmergehost` passes. All three
templates audit with zero findings, and 55 single mutations (after review repairs) each produce
their specific finding.

The following remain open or `NOT_RUN`:
- Issue acceptance item (a), a hosted dry-run of the #395 replay set: not run, because #395 has not
  landed and no hosted runner was used.
- The delta step: `NOT_PRODUCED` until `corvint delta` (#389) exists.

The audit cannot see organisation, runner or environment secrets, or the implicit runtime token that
artifact actions use. Same-VM attestation is not confinement.

## Repair r1 (independent review FAIL)

- **Newline injection in the change id.** `grep -Eqx` passed a multi-line dispatched value when any
  one line matched, so `<40hex>\nchange=refs/pull/1/head` wrote a second `change=` output that
  reached `actions/checkout ref:` and the hooks (reproduced against the original template under bash
  and sh). The pipeline, replay loop and installer now validate whole strings with `case` and a
  length check. The audit adds `LINE_ORIENTED_VALIDATION`; a mutation reintroducing the grep form
  refuses, and `TestResolveRefusesInjectedChange` and `TestReplayRefusesMalformedChange` run the
  shipped step scripts against injected values.
- **Before-state overclaim.** The upload comment no longer claims later edits are detectable. The
  spec and README record, as an unobserved inference, that author code may obtain
  `ACTIONS_RUNTIME_TOKEN` to replace the artifact or poison default-branch caches; no
  author-independent digest is bound, so before-state is untrusted input.
- **Installer glob and audit gaps.** Companion names are checked before any fetch and admit only
  lowercase letters and hyphens. Step `shell:` is refused as unmodelled, `with.script` expressions
  are flagged, and the source trigger's push filter must be a literal `branches` list
  (`SOURCE_TRIGGER_BRANCHES`). Remaining lexical limits are recorded in the spec and README.

The focused package now has 39 mutations, each producing its specific finding code.

## Repair r3 (independent review FAIL)

- **Case-folded `with.script`.** Action input names are case-insensitive, so `Script:` reached the
  action as `INPUT_SCRIPT` and audited clean. The screen now matches `script` in any case and
  refuses any non-lowercase input name (`NON_LOWERCASE_INPUT`).
- **Startup variables and the runner env file.** A workflow, job or step `env` key such as
  `BASH_ENV`, `ENV`, `LD_*` or `DYLD_*` made a shell or the loader run an unaudited file. Such keys
  are refused (`STARTUP_ENV`, a denylist), a non-mapping `env` is unmodelled, and a run script that
  names `GITHUB_ENV` or `GITHUB_PATH` is refused (`RUNNER_ENV_FILE`). The lexical residue (other
  variables, indirect file access, pinned actions writing the file) is recorded in the spec failure
  modes and README limits.
- **`working-directory`.** A step or `defaults.run` value must be a literal scalar
  (`WORKING_DIRECTORY`).
- **Unkilled `defaults.run` refusal.** A new mutation adds an unknown `defaults.run` key.

The shipped templates still audit clean. Twelve new mutations bring the total to 55; deleting each
new check, one at a time, fails at least one of them.

## Repair r4 (independent review FAIL)

- **Non-mapping `with`.** A step `with` that was a scalar expression or a sequence skipped the
  input screen and audited clean. It is now `UNMODELLED_KEY`, as a non-mapping `env` already was.
- **ASCII input names.** `with` input names must match `[a-z0-9_-]` (`NON_LOWERCASE_INPUT`). A
  dotless-i (U+0131) `script` stayed lowercase, missed the case-folded `script` match and still
  reached the action as `INPUT_SCRIPT`.
- **Case-folded `env` names.** The fold is kept, because Windows runners read environment names
  without regard to case, and a lowercase `ld_preload` mutation now pins it.
- **More code-executing variables.** `HOME`, `CC`, `GOTOOLCHAIN`, `JAVA_TOOL_OPTIONS` and the
  `GIT_CONFIG` prefix join the `STARTUP_ENV` denylist (`NODE_OPTIONS`, `PYTHONPATH`,
  `PYTHONSTARTUP`, `PERL5OPT` and `RUBYOPT` were already on it). The shipped templates still audit
  clean.

Nine new mutations bring the total to 64. Deleting each new check, one at a time, fails a named
subtest.
