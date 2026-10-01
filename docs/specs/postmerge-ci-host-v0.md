# Post-merge CI Host V0

Owner: Russell Lewis
Date: 2026-10-01
Intent status: proposed
Delivery status: experimental
Authoritative inputs: human request https://github.com/beamfall/corvint/issues/398 (part of #388);
`docs/specs/postmerge-connectors-v0.md`; `docs/specs/authoring-step-scope-v0.md`;
`docs/SPEC-DRIVEN-DEVELOPMENT.md`; `AGENTS.md`.

## Agent digest
- Claim: Reference GitHub Actions templates and a host adapter contract run the post-merge workflow with per-step credentials and no write credential in authoring.
- Status: proposed/experimental; declared-YAML audit only. No template has run on a hosted runner.
- Exists: `protocol/postmerge-host` (graph, templates, installer, adapter contract); `internal/postmergehost` (graph validator, YAML-subset parser, audit) and its tests.
- Blocked on: the merge replay set (#395), a hosted dry-run of that set, `corvint delta` (#389), owner acceptance and native completion.
- Read next: Requirements; Trust boundary; Acceptance evidence.

## User and measurable job

A CI maintainer adopts the post-merge workflow on their own host without designing the credential
split themselves. The measurable job has three parts. The maintainer copies one reference pipeline
and supplies five hooks. An audit shows that no authoring environment can reach an outward-write
credential. Any merged change can be replayed by its full id. The simpler baseline is handwritten
workflow YAML that is reviewed once.

At base `ff3da727`, Corvint has the post-merge connector (PMC-V0), intake (`corvint-intake`), step
attestation (ASS-V0) and metrics (PMM-V0) companions, but no CI host template or host contract.
Corvint stays a local binary. The templates are operator reference material, not a hosted service
(product invariant 7).

## Requirements

- `PCH-V0-001`: Publish a host-neutral workflow graph, `protocol/postmerge-host/workflow-graph.json`,
  with profile `corvint-postmerge-host-graph/0`. It declares nine ordered steps: trigger, intake,
  delta, follow-up-item, authoring, trusted-validation, draft-change-requests, findings, metrics.
  Each step lists the earlier steps it follows, its allowed credential classes and its documented
  Corvint commands. Credential classes have disjoint secret-name prefixes and an outward-write flag.
  Authoring requires attestation and holds no outward-write class. A command that does not exist
  yet is listed as pending. Unknown fields, unsorted lists and forward dependencies refuse.
- `PCH-V0-002`: Document the host adapter contract in `protocol/postmerge-host/README.md`:
  - step and workflow markers (`CORVINT_PM_STEP`, `CORVINT_PM_WORKFLOW`);
  - the secret naming convention;
  - the hooks (export-context, reader, step-host, author and validate), with each hook's
    inputs, outputs and credential classes.
  Raw tracker and forge text stays outside the author worktree. Only `corvint-intake validate`
  output reaches authoring.
- `PCH-V0-003`: Ship a reference pipeline that processes one merged change by full commit id:
  - It accepts only `dry-run` and `recording` modes. Remote live writes are unsupported (PMC-V0-006).
  - It serializes runs per change without cancelling a run in progress.
  - Authoring runs in its own job on a fresh runner, bracketed by `corvint step snapshot`,
    `env-check` and `verify`.
  - Trusted validation runs on another fresh runner. It takes the binding and source item from its
    own export, not from author output.
  - The delta step stays `NOT_PRODUCED` until `corvint delta` exists.
- `PCH-V0-004`: Ship a source trigger. It runs on push to the merged branch only, never on a change
  request. It continues on error, is bounded to ten minutes, and only dispatches the pipeline.
  Because of this, it cannot block or fail the merged change request.
- `PCH-V0-005`: Ship scheduled reconciliation that re-dispatches recent first-parent merges, so a
  lost dispatch is retried. A manual replay input re-dispatches a committed list of full change ids
  in dry-run mode by default. Repeated runs rely on connector idempotency (PMC-V0-005 to PMC-V0-007).
- `PCH-V0-006`: Pin every action to a full commit and check out with `persist-credentials: false`.
  Build Corvint companions from a pinned 40-hex source commit, and refuse any binary whose SHA-256 is
  not in the operator's pin file before its first use.
  Untrusted values reach scripts through `env`, never by expanding expressions into script text.
- `PCH-V0-007`: Provide an audit, `internal/postmergehost.Audit`, over a restricted YAML subset. It
  refuses unsupported constructs: anchors, aliases, tags, folded scalars, flow mappings, multiple
  documents, tabs, duplicate keys, and unmodelled job keys such as reusable workflows, containers
  and services.

  It reports each of these, with the finding code in brackets:
  - change-request triggers [CHANGE_REQUEST_TRIGGER];
  - workflow-level write permissions or secrets [WORKFLOW_WRITE_PERMISSION, WORKFLOW_LEVEL_SECRET];
  - secrets without a class prefix [UNCLASSIFIED_SECRET];
  - a credential not allowed for the job's marked steps [CREDENTIAL_NOT_ALLOWED];
  - any outward-write class, write permission or `github.token` in an authoring job
    [AUTHORING_WRITE_CREDENTIAL];
  - an authoring job that performs other steps [AUTHORING_NOT_ISOLATED];
  - unpinned actions [UNPINNED_ACTION] and persisted checkout credentials
    [CHECKOUT_PERSISTS_CREDENTIALS];
  - expressions in run scripts [RUN_EXPRESSION_INTERPOLATION];
  - a Corvint binary run before the pinned install [UNPINNED_BINARY];
  - a Corvint command not documented for the job's steps [UNDOCUMENTED_COMMAND];
  - job dependencies that violate graph order [ORDER_VIOLATION];
  - source-trigger, concurrency and mode-input rule violations.

  Every shipped template audits with zero findings.
- `PCH-V0-008`: The README gives a porting guide that maps markers, credential classes, isolation,
  post-merge triggering, per-change locking, replay and reconciliation to another host. It states
  the qualification limits:
  - declared YAML only;
  - no view of organisation, runner or environment secrets, or the implicit runtime token;
  - same-VM attestation is not confinement;
  - no hosted run.

## Non-goals

- A hosted service, daemon, GitHub App, or network dependency in the default local product.
- Remote live writes, vendor API adapters, or running an agent.
- A published audit command.
- A general YAML parser.
- Templates for hosts other than GitHub Actions.
- Authenticating the forge or tracker.
- Proving runtime credential isolation.

## Failure modes

- A secret is configured outside the workflow text, for example at organisation, environment or
  runner level, and is reachable by authoring. The audit cannot see it, so it reports a PASS for the
  declared text only.
- Artifact actions use an implicit runtime token. An author could tamper with the uploaded patch or
  connector input. The trusted job re-derives the binding and source item from its own export and
  checks the patch with `git apply --check`. It does not authenticate the author output.
- A dispatch fails on push. Nightly reconciliation re-dispatches the change.
- A ported template uses an unmodelled construct. The audit refuses it rather than guessing.

## Trust boundary

The audit reads declared workflow text and grants no authority. Hooks are operator code. Corvint
does not ship them, and the audit observes only the hook names it invokes. Step attestation is
ASS-V0 local observation, not confinement. The pin file and the source commit are trusted operator
configuration, and rebuilding the pins happens on a trusted machine with the same toolchain.

## Acceptance evidence

Run:

`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/postmergehost`

It checks that:
- the graph validates, and seven malformed graphs refuse;
- all three templates audit clean;
- the authoring job references no write-class secret, write permission or token;
- 32 single mutations each produce their specific finding code;
- the YAML subset refuses unsupported syntax;
- `install-pinned.sh` passes `sh -n` and refuses a short commit;
- the README documents every hook the templates call.

Issue #398's first acceptance item is still open. It requires the replay set (#395) to run in
dry-run mode on a host, and that is `NOT_RUN`.

## Traceability

| Requirements | Implementation | Evidence |
|---|---|---|
| PCH-V0-001 | `protocol/postmerge-host/workflow-graph.json`, `graph.go` | `TestGraphContract` |
| PCH-V0-002, PCH-V0-008 | `protocol/postmerge-host/README.md` | `TestReadmeDocumentsTemplateHooks` |
| PCH-V0-003, PCH-V0-004, PCH-V0-005 | `protocol/postmerge-host/github-actions/*.yml` | `TestReferenceTemplatesAuditClean`, `TestAuditRefusesUnsafeTemplates` |
| PCH-V0-006 | `protocol/postmerge-host/install-pinned.sh`, `audit.go` | `TestInstallPinnedSyntax`, `TestAuditRefusesUnsafeTemplates` |
| PCH-V0-007 | `yaml.go`, `audit.go` | `TestAuthoringEnvironmentHasNoWriteCredential`, `TestAuditRefusesUnsafeTemplates`, `TestParseYAMLSubset`, `TestShellCommands` |

## Qualification and rollback

The material is experimental reference material, audited only against declared text. Promotion
requires all of the following:
- owner acceptance;
- a hosted dry-run of the #395 replay set;
- `corvint delta` (#389) replacing the delta placeholder;
- a review of each operator's host-level secret scoping.

Rollback: delete the copied workflows from the operator repository, then revert the
`protocol/postmerge-host` and `internal/postmergehost` commits. This adds no Core command, no
installed binary and no store migration.
