<!-- SPDX-License-Identifier: Apache-2.0 -->
# Post-merge CI host adapter contract (experimental)

This directory is reference material for operators who run the Corvint post-merge workflow on
their own CI host. Corvint itself stays a local binary: it ships no hosted service, daemon, account
or network dependency. Intent is proposed and delivery is experimental under PCH-V0
(`docs/specs/postmerge-ci-host-v0.md`). Nothing here has run on a hosted runner yet.

| File | Purpose |
|---|---|
| `workflow-graph.json` | Host-neutral workflow graph: steps, order, credential classes and documented commands |
| `github-actions/postmerge.yml` | Reference pipeline for one merged change (role `pipeline`) |
| `github-actions/source-trigger.yml` | Post-merge source trigger that only dispatches (role `source-trigger`) |
| `github-actions/reconcile.yml` | Nightly reconciliation, replay set and metrics (role `reconcile`) |
| `install-pinned.sh` | Builds Corvint companions from a pinned commit and refuses unpinned digests |

## Workflow graph

`workflow-graph.json` (profile `corvint-postmerge-host-graph/0`) is the contract a host adapter
implements. Its nine steps run in this order: trigger, intake, delta, follow-up-item, authoring,
trusted-validation, draft-change-requests, findings, metrics. `after` lists the steps that must
finish first. `credentials` lists the only credential classes the step may hold. `commands` lists
the only Corvint commands it may run. `pendingCommands` names a command that is not published yet.
The delta step is `NOT_PRODUCED` until `corvint delta` exists.

Only the authoring step runs an author with write access to a worktree. It carries attestation
`required`, which means `corvint step snapshot`, `env-check` and `verify` (ASS-V0) bracket it. It
never holds an outward-write credential class.

## Host adapter obligations

A host adapter is a set of CI workflows plus five operator hooks. It must:

1. Mark each job with `env.CORVINT_PM_STEP`, a comma-separated list of the graph steps it performs.
   An authoring job performs only `authoring`.
2. Mark each workflow with `env.CORVINT_PM_WORKFLOW`: `source-trigger`, `pipeline` or `reconcile`.
3. Name every secret with its credential-class prefix, for example `CORVINT_PM_FORGE_READ_TOKEN`.
   Expose a secret only at the job or step that needs it, never in workflow-level `env`.
4. Run post-merge only. The source trigger fires on a push to the merged branch, never on a change
   request, continues on error and is time-bounded. It cannot block or fail the merged change.
5. Accept a change by its full merge commit id, so any change can be replayed. Run one pipeline at a
   time per change without cancelling a run in progress.
6. Pin every third-party action to a full commit, and check out with `persist-credentials: false`.
   Build Corvint binaries from a pinned commit and verify their digests before the first use.
7. Pass untrusted values to scripts through `env`, never by expanding `${{ }}` into the script text.
   Validate a dispatched value as one whole string with `case` patterns and a length check before
   writing it to `$GITHUB_OUTPUT` or `$GITHUB_ENV`. A line-oriented `grep` accepts a value when any
   one line matches, so a newline can inject a second output.
8. Reconcile on a schedule, so a lost dispatch is retried. Connector upserts are idempotent
   (PMC-V0-005 to PMC-V0-007), so running a change twice is safe.

## Operator hooks

The hooks live in the operator's repository under `postmerge/hooks/`. Corvint does not ship them.

| Hook | Inputs | Outputs | Credential classes |
|---|---|---|---|
| `export-context.sh CHANGE DIR` | full change id | `fixture.json` and `input.json` for `corvint-postmerge-connect` (PMC-V0) | forge-read, tracker-read |
| `reader.sh RAW CANDIDATE` | raw intake file | candidate JSON for `corvint-intake validate` | agent-model |
| `step-host.sh WORKTREE HOST` | author worktree | ASS-V0 host record | none |
| `author.sh INPUT WORKTREE OUTPUT` | admitted author input | edits in the worktree and a connector input | agent-model |
| `validate.sh BEFORE AUTHOR-OUT WORKTREE` | uploaded before-state and author output, both untrusted | exit status; operator acceptance checks, which may be a no-op | none |

Raw tracker and forge text stays outside the author's worktree. Only the admitted typed record from
`corvint-intake validate` reaches the authoring job. The trusted job takes the binding and source
item from its own export, not from author output.

## Porting to another host

1. Copy `workflow-graph.json` and keep one job (or a stricter split) per marked step set.
2. Map each credential class to the host's secret store, scoped per job. A host that cannot scope
   secrets per job must not run the authoring step on that host.
3. Keep authoring on its own runner, separate from any job that later holds write credentials.
4. Map the source trigger to the host's post-merge event, and keep it non-blocking and bounded.
5. Map concurrency to a per-change lock, replay to a manual run by change id, and reconciliation to
   a scheduled run.
6. Keep modes to `dry-run` and `recording`. Remote live writes are not supported by the connector.
   When a live adapter is qualified, split `draft-change-requests` and `findings` into separate jobs.
   Each holds only its own `forge-write` or `tracker-write` credential.
7. Audit the ported workflows against these rules. The audit is the Go function `Audit` in
   Corvint's `internal/postmergehost` package. Its tests run it on these reference templates. It
   is not yet a published command. For GitHub Actions syntax, run it from a Corvint checkout.
   For any other host, review the same rules by hand and record that the audit is `NOT_RUN`.

## Limits

- The audit reads only the declared workflow text. It does not see organisation, runner or
  environment secrets, or repository settings.
- It does not see the implicit runtime token that artifact actions use. Inference, not observed:
  author code on a hosted runner may obtain `ACTIONS_RUNTIME_TOKEN`. With it, the author could
  delete and re-upload the `before-state` artifact, although it is uploaded before the author runs,
  and could write Actions cache entries scoped to the default branch that other workflows may
  restore. The templates bind no before-state digest from a job the author cannot influence, so
  `validate.sh` and the trusted job must treat `before-state` as untrusted, like author output.
- Coverage is lexical and partial. `LINE_ORIENTED_VALIDATION` matches only the `printf`/`echo`
  pipe and here-string forms. Expressions are checked in `run` and `with.script`, not in other
  action inputs that evaluate code. A custom step `shell:` is refused rather than audited. The
  audit cannot see the repository default branch, so check that the source trigger names it.
- Attestation on the same virtual machine is not confinement.
- No template has run on a hosted runner, and no replay set has run in dry-run mode.
