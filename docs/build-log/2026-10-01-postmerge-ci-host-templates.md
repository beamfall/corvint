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
templates audit with zero findings, and 32 single mutations each produce their specific finding.

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
