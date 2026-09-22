# Local Completion Policy V0

Owner: Russell Lewis
Date: 2026-09-06
Intent status: accepted direction (owner selected decision 0009 option 2 in the 2026-09-06 Codex dogfood repair task)
Delivery status: implemented
Authoritative inputs: `docs/DOGFOOD.md`, `docs/decisions/0009-harness-authority-boundary.md`, `docs/specs/agent-harness-integration-v0.md`

## Agent digest
- Claim: Explicitly enrolled changes require selected checks, bound evidence and inspected reports before local completion; no execution attestation.
- Status: accepted direction (owner selected decision 0009 option 2 in the 2026-09-06 Codex dogfood repair task)/implemented
- Exists: all 12 in-scope requirements have executable local evidence for the workflow, prompt compiler and native adapter.
- Blocked on: current-change canonical verification, report acknowledgment, strict outcome qualification and installed-hook validation, recorded separately; complete host-version matrix NOT_RUN.
- Read next: Requirements; Failure modes; Acceptance evidence and traceability.

## User and job

The owner asked Corvint to finish its own evidence workflow automatically, prevent premature completion,
restore clean-tree learning, and stop presenting incidental syntax matches as useful context for vague
follow-ups. The owner explicitly selected the local workflow gate rather than a new external authority
root. The agent still judges citation meaning, requirement coverage and test adequacy; the gate makes
those inputs and omissions explicit and automates their execution and validation.

The owner's 2026-09-08 cross-host setup request extends this same caller-owned workflow to Claude
Code. Independent review fixes distinct host namespaces, exact profile dispatch, trusted bounded
guidance and owned child lifetimes. This accepts no new external authority or FULL host claim.

## Verified starting point

At `6adc6e1ad82ca49d2daa28fbaecf596310028730`, the existing dogfood report and strict check pass for
the preceding repair. Native prompt delivery was repaired by refreshing a stale installed Codex
adapter. An active task's retired cache path then required a compatibility copy. The sole tracked
cache-version change is committed in `767c0a5`. The existing coordinator's `complete` means artifact
production, not review, actual test execution or successful workflow completion. Legacy harness
Stop always reports unavailable authority. Vague prompts can receive unrelated syntax under the
frozen broad query profile. None of those legacy profile meanings is changed here.

## Requirements

- `LCP-V0-001`: The local policy MUST remain caller-owned and non-authoritative. Its positive state
  means only that the explicitly selected local workflow conditions passed for the bound scope.
  It MUST NOT mint `HARNESS_ATTESTED`, close a Frontier, remove `frontier-authority-unavailable`,
  claim complete platform qualification, or reinterpret `corvint-harness-event/0` or `frontier/0`.
- `LCP-V0-002`: Enrollment MUST be explicit through `corvint dogfood begin`. One bounded plan pins
  the full immutable base, sorted unique intent paths, and a nonempty selected check set whose IDs,
  argv and timeouts are fixed before execution. Each intent path MUST resolve to a blob at the
  enrollment-time `HEAD` commit, and its pointer records that commit OID and blob OID (decision
  0170). An intent absent from the base but present at that commit is a same-change bootstrap intent
  (`OCM-V0-009`) that `finish` counts against the base. `begin` refuses an intent that commit does
  not contain rather than recording an empty blob hash, since an empty hash pins nothing. `begin`
  also refuses with `base-not-ancestor-of-target` when the base is not an ancestor of that commit
  (`git merge-base --is-ancestor`): `finish` and the public CEM/OCM commands diff the base and target
  as two trees, and the coordinator's ancestry check applies only when an anchor ref is configured,
  so an unrelated history (for example another branch checked out, or an orphan commit with the
  change's tree) otherwise reached a satisfied completion. A failed probe stays an error, never
  that code. These refusals precede every local write, so they consume no enrollment generation.
  `HEAD` can still move to unrelated history after `begin` enrolls, so evaluate and `finish` re-run
  the identical probe against the current target before any local write and report the same
  `base-not-ancestor-of-target` code as an unmet condition rather than a hard error, so a moved
  `HEAD` never reaches `satisfied`. The host session identity is hashed locally; the
  private worktree Git directory and session hash identify enrollments. Because the legacy
  coordinator writes worktree-global artifacts, at most one active worktree-wide owner is allowed;
  another session refuses enrollment until that owner satisfies or explicitly cancels. A separate
  bounded operation lock serializes same-owner mutations; stale locks never authorize takeover.
  No new raw host prompt, transcript, credential or inferred task meaning is captured.
  The sole provenance exception is a preexisting caller-owned explicit query/impact receipt: it may
  already contain its caller-entered task and is archived under LCP-V0-005, never used for ranking.
  A matching active enrollment resumes; a different
  active plan refuses replacement. Explicit enrollment after cancellation or current satisfaction
  creates a bounded new generation and preserves prior evidence; repeated successful enrollment
  of the identical satisfied plan is idempotent. Default session identity may come only from `CODEX_THREAD_ID`
  or `CODEX_SESSION_ID`; an explicit 64-hex session key is also admitted. Claude native hooks
  derive that explicit key as lowercase SHA-256 of UTF-8
  `corvint-local-completion-session/claude-code/0`, one NUL byte, and the unmodified native
  `session_id` (nonempty string, at most 4096 UTF-8 bytes). Missing/empty identity degrades as
  `missing-session-identity`; nonstring/oversized identity as `invalid-session-identity`.
  Claude never reads inherited CODEX identity variables. Codex's existing hash domain is unchanged.
  A cross-host handoff retains the original explicit key and worktree for manual commands;
  the receiving native hook evaluates only its own natural key and cannot take over an enrollment.
- `LCP-V0-003`: `status` and automatic event evaluation MUST be read-only. They derive unmet
  conditions from the enrolled plan and current repository/artifacts. Missing enrollment is inactive,
  not satisfied. Persistent lifecycle is only active, satisfied or explicitly cancelled; cancellation
  never becomes successful completion. Invalid, stale, missing, oversized or cross-session evidence
  cannot satisfy the policy. Dirty or untracked work is incomplete and is never discarded.
  `nextActions` MUST name only the next eligible workflow phase: unqualified selected checks,
  then current report inspection, then finish. A dirty worktree, owner mismatch, unrelated target,
  or exhausted verification attempts when a check remains unqualified yields only keyed read-only
  `status`; all unmet conditions remain visible. That action preserves the original handoff handle
  for a later explicit recheck, not an instruction to poll or repair ownership/history automatically.
  Satisfied, cancelled and inactive states offer no continuation. Reaching 64 observations while
  verification is still needed adds `verification-attempt-bound-exceeded` once; already-qualified
  checks can still proceed to review/finish at that bound.
- `LCP-V0-004`: `verify` MUST execute only an explicitly selected plan check, as argv without shell
  interpretation, through the existing bounded process-group runner. It captures the tested commit,
  repository state, exact argv, observed exit/timeout/cancellation and bounded stdout/stderr digests.
  Only a clean, successful observation with retained matching logs qualifies. Exact commit matching
  is the default. A check may explicitly declare `allowCemSidecarOnlyReuse` in its frozen plan; only
  that check may reuse an observation when the complete Git content excluding the validated CEM
  sidecar is identical. Reports preserve the tested commit and reused target as separate fields;
  source, spec, plan or other artifact changes invalidate reuse. Selected checks are not a claim
  that every necessary check was selected; the reviewer must assess adequacy.
- `LCP-V0-005`: `finish` MUST reuse the public CEM/OCM and repository dogfood workflow. It MUST NOT
  infer citations, link requirements from shared paths, waive unknowns, run commands found in saved
  free text, commit files, or expand scope. Missing bindings yield an actionable worklist using the
  existing cite/link/mark commands. Validated bootstrap exceptions and honestly unassessed unchanged
  obligations retain their existing meanings and counts. The local policy refuses caller-assessed
  unknowns (reasons other than `unassessed`) using the complete verified worklist. Unassessed rows
  remain visible for report review; the workflow does not infer whether a requirement was untouched.
  The legacy aggregate's structural acceptance alone does not establish semantic scope coverage.
  Coordinator inputs are deterministic:
  `DOGFOOD_TASK` is a fixed non-prompt label plus plan digest; `DOGFOOD_VERIFY` is
  `local-completion-checks-sha256:<digest>`, a display-only reference to the canonical frozen argv
  array. Full argv and observations remain readable in the bound report set; the reference is not
  an executed command. This preserves the legacy recorder's 512-character verification grammar; `DOGFOOD_OUTCOME=passed` is supplied only after actual selected
  checks qualify. Intent manifests come from the frozen plan. An empty citation TSV is permitted
  only after strict CEM status validates already-bound citations; otherwise finish refuses with a
  worklist. The final strict check is outside the selected prerequisite check set. Original
  pre-change receipts are archived as bounded enrollment-owned records before the coordinator
  overwrites its worktree-global files; subsequent receipts are labelled coordination-time. Existing
  explicit-query task text may remain only in these provenance copies. Secret-shaped receipts are
  not duplicated: retain their original digest and a fixed archival-refusal reason; preserve the
  original file untouched. This exception adds no host-prompt or transcript capture.
- `LCP-V0-006`: Once maps are ready, `finish` MUST generate the CEM reviewer report and every scoped
  OCM report, together with the selected verification observations, as one bounded report set. An
  explicit `review --report-set DIGEST` acknowledgment MUST bind that exact set, base, target,
  enrolled plan, CEM/patch and OCM/intent digests. Generation alone is not inspection. The acknowledgment
  is caller/model inspection, not proof of independence, semantic correctness or execution authority.
  Any bound-byte change invalidates it. The set binds raw CEM/OCM reports, plan and check
  observations/logs; the coordinator report's mutable `dogfoodCheck` append is outside that set.
  Status and finish revalidate current bindings, including before terminal publication.
- `LCP-V0-007`: Terminal success MUST require all selected checks to pass, current report inspection,
  a clean final target, the existing complete dogfood artifact set and successful independent
  base/current verifier agreement. Outcome materialization precedes the existing strict check because
  that check validates its digest; policy satisfaction follows the check. Failed or blocked outcome
  recording, or a successful record followed by check failure, MUST NOT satisfy the workflow.
  `outputsAgree:true` alone is insufficient: the strict checker must actually exit zero. Scope,
  owner, target and artifact identities are rechecked across generation, checking and publication.
  Repeating an identical successful finish MUST not append duplicate task outcomes.
  A separately authorized, digest-bound `prechange-impact` context abstention is not a missing
  completion proof: it remains visibly `NOT_PRODUCED`, while CEM/OCM, checks, clean-target and
  verifier requirements above remain unchanged. Its malformed, missing, stale or untyped form fails.
- `LCP-V0-008`: Codex and Claude Code Stop MAY request one bounded remediation continuation for an explicitly enrolled
  incomplete change. If `stop_hook_active` is true, the adapter MUST release with a visible fixed
  unresolved-policy notice instead of looping. Inactive, satisfied and cancelled states are distinct.
  Timeouts, malformed input and unsupported hosts fail open with explicit failure, not a satisfied
  claim. An automatic event whose deadline expires reports the fixed `dogfood-event-deadline` code,
  never the generic `dogfood-event-unavailable`, even when the expiry first surfaces as a failed
  repository or policy read. That code MUST be returned once the deadline passes, without waiting
  for a read stage that does not observe cancellation (the in-memory index compile of a snapshot
  miss); the abandoned read writes nothing. `dogfood-event-policy-drift` is reserved for an observed
  change: the policy target moved or both policy reads succeeded and disagree. A failed policy re-read
  observed no drift and reports as a read failure (`dogfood-event-unavailable`, or
  `dogfood-event-deadline` on expiry). A supplied `stop_hook_active` MUST be boolean. Expensive
  verification and mutations never run in Stop. User interruption remains effective.
- `LCP-V0-009`: `corvint dogfood event` MUST use a separate `corvint-dogfood-event/0` envelope and
  result-digest domain. Legacy query, task-context and harness command behavior stays unchanged.
  The result binds its full normalized public content, exact repository snapshot, event and local
  policy state. Native adapters validate that envelope before using its completion decision. The
  automatic command shares one bounded deadline and a complete 8000-byte response ceiling, except
  Claude Code session-end, whose declared host kill is tighter than the other admitted event/host
  pairs and which therefore uses its own shorter deadline strictly below that kill.
  The admitted plugin tuple is host `codex` with host version `unknown` or host `claude-code` with
  host version `unreported-by-hook-api` (`AHI-023`), adapter version `0.1.0`, for session-start/user-prompt/stop/session-end only. Any other tuple refuses with
  `unsupported-dogfood-event-host` and any other event with `unsupported-dogfood-event`
  (`cmd/corvint/local_completion_event.go:73,148`). Claude post-tool and
  file-change retain the legacy harness profile, normalization and plain session hash; neither
  profile accepts an envelope from the other. A compact session start over a dirty worktree adds
  the `AHI-003` compaction block under the context packet's `compaction` key and its `compaction-*`
  codes to `degradations`; every other event and start source keeps the envelope unchanged.
  Host/request/result/repository/policy/Frontier
  provenance remains validated before any completion decision. A non-zero native event or legacy
  harness result degrades as `corvint-event-rejected`; when the final 4096 bytes of stderr are one
  JSON object whose top-level `code` is a string matching `[a-z0-9-]{1,96}`, the degradation is
  `corvint-event-rejected:<code>`. Missing, malformed, non-string, or nonmatching codes retain the
  unsuffixed degradation so no other stderr or caller-derived data is reflected.
- `LCP-V0-010`: New-profile prompt context MUST separate project governance, declared enrollment scope
  and current-task evidence. It may reuse the task-context compiler with an empty subject. Root
  instructions remain available without lexical relevance, but never establish task answerability.
  Without an explicit path, requirement ID or source-backed identifier anchor, automatic context
  retains bounded governance/scope and reports `explicit-task-anchor-required`; it withholds incidental
  lexical rows. Unknown/ambiguous anchors and stale scope remain explicit. General natural-language
  discovery stays available through the unchanged direct query command. No pronoun resolution or
  transcript reconstruction is claimed. Empty startup context stays unresolved. A qualified name
  whose terminal symbol alone matches remains `qualification-unverified`; an otherwise resolved
  task whose evidence is omitted for budget becomes `task-evidence-omitted`.
- `LCP-V0-011`: Prompt context MUST project only allowlisted, Git-reextractable evidence and fixed
  diagnostic fields. Raw task terms, nearest-claim explanations and unverified prompt fragments are
  excluded. Mandatory identity, resolution and omissions survive byte budgeting; missing critical
  selectors are disclosed. Every repository-authored field is enclosed as untrusted data. Scope
  pointers are caller declarations, not proof that a proposed spec is accepted. Claude adds fixed
  trusted workflow guidance outside the unchanged untrusted-data envelope as JSON argv arrays
  derived only from validated native root/key. Reserve its encoded overhead before the one core
  request, retaining core reserves; bound complete UTF-8 host JSON plus LF at 8000 bytes. Oversize
  output becomes one fixed degradation, never a truncated path, key, packet or resealed receipt.
  A budget that cannot retain the mandatory fields fails `unsupported-dogfood-context-budget`, and
  explicit-anchor context over its bounded candidate profile fails `unsupported-dogfood-context-bounds`
  (`internal/contextindex/local_completion_context.go:239,430`); the native event reports either as
  the fixed `dogfood-event-unavailable` (`cmd/corvint/local_completion_event.go:135`).
- `LCP-V0-012`: Private plans, observations and report sets MUST have explicit byte/count limits,
  strict schema/duplicate-field/path validation, atomic publication and bounded ownership. No daemon,
  network, new database, transcript scanner or arbitrary background executor is added. Every spawned
  process uses cancellation and descendant cleanup with a regression. A concurrent enrollment cannot
  silently overwrite an active owner. Two-session contention and same-owner interleaving must
  prove shared legacy artifacts cannot be acknowledged or published under the wrong owner. Errors preserve prior valid state.

## Non-goals and simpler baseline

The baseline is the documented agent-driven DOGFOOD loop. This slice removes orchestration gaps and
adds a local stop policy; it does not automate semantic truth, make same-UID files tamper-proof,
qualify `frontier/1`, change broad query ranking, invent a general session-memory system, auto-commit
user work, or promote cost/utility claims. The required native integrations are the admitted Codex and Claude Code plugin tuples;
other hosts retain their legacy paths. Enrollment is worktree-local: a native event in another
checkout does not discover it merely because the repositories share a Git common directory.
Complete host qualification remains separately unproven.

## Failure modes

Missing citations, caller-assessed unknowns, an uncommitted sidecar, failed selected checks,
unread reports, stale targets, changed plan/intent/report/log bytes, malformed state, cancellation,
timeout and unsupported cleanup remain distinct unmet conditions. Missing enrollment releases ordinary
questions. A continuation limit releases with an unresolved notice rather than pretending success.
Original pre-change receipts and failed test runs are retained; later coordination receipts cannot
retroactively establish pre-change chronology.
Unassessed rows require reviewer judgment: changed obligations must receive adequate links before
acknowledgment; the local policy cannot infer that semantic distinction from raw unknown counts.

### Refusal codes

The workflow, its CLI and the automatic event surface refuse with the kebab-case codes below
(decision 0100). Each row cites the first emitting site and states only the condition checked there;
a code emitted at more than one site keeps the meaning of its first. Codes this spec already names
elsewhere are not repeated.

| Code | First emitting site | Condition at the cited site |
|---|---|---|
| `base-not-ancestor-of-target` | `internal/localcompletion/storage.go:447` | Git reports the plan base is not an ancestor of the enrollment-time `HEAD` commit (exit 1, empty stderr) |
| `base-unavailable` | `internal/localcompletion/lifecycle.go:39` | the plan base does not resolve, or resolves to a different object |
| `cem-bindings-required` | `internal/localcompletion/finish.go:173` | the public `cem status` run against the plan base and current target returned an error |
| `check-executable-unavailable` | `internal/localcompletion/storage.go:515` | `exec.LookPath` cannot resolve a check's executable |
| `completion-evidence-drift` | `internal/localcompletion/finish.go:334` | after finishing, the tree is not clean, the target or tree differs from the pre-finish snapshot, or the saved report is no longer current |
| `dogfood-coordination-failed` | `internal/localcompletion/finish.go:365` | the `dogfood-change.sh` coordination run did not pass |
| `dogfood-event-context-drift` | `cmd/corvint/local_completion_event.go:375` | the loaded index commit or tree revision, or the dirty-path digest, differs from the probed repository context |
| `dogfood-event-deadline` | `cmd/corvint/local_completion_event.go:129` | the event's context deadline expired or was cancelled |
| `dogfood-event-input-unavailable` | `cmd/corvint/local_completion_event.go:117` | reading the event input from stdin failed |
| `dogfood-event-native-budget` | `cmd/corvint/local_completion_event.go:431` | eight prompt-context attempts, each shrinking the budget, never fit the natively escaped response within the byte budget |
| `dogfood-event-output-too-large` | `cmd/corvint/local_completion_event.go:456` | the canonical response plus a final LF exceeds the byte budget |
| `dogfood-event-output-unavailable` | `cmd/corvint/local_completion_event.go:148` | writing the encoded response to stdout failed |
| `dogfood-event-policy-drift` | `cmd/corvint/local_completion_event.go:279` | the evaluation's target is set and differs from the commit probed before the event |
| `dogfood-event-repository-drift` | `cmd/corvint/local_completion_event.go:276` | the repository context probed after the event differs from the one before, or the commit differs from the expected target |
| `dogfood-report-drift` | `internal/localcompletion/finish.go:472` | the dogfood report does not parse, is not complete, or names a base or target other than the plan base and current target |
| `duplicate-local-completion-option` | `cmd/corvint/local_completion.go:129` | a `local-completion` option is given twice |
| `enrollment-bound-exceeded` | `internal/localcompletion/storage.go:312` | saved state has more than 64 observations, or its intent-pointer or executable count does not match the plan |
| `enrollment-cancelled` | `internal/localcompletion/finish.go:46` | the saved enrollment's lifecycle is `cancelled` |
| `enrollment-drift` | `internal/localcompletion/storage.go:294` | saved state names another session, or its plan digest does not match its plan |
| `enrollment-generation-bound-exceeded` | `internal/localcompletion/lifecycle.go:514` | 16 enrollment generations are already preserved for the session |
| `enrollment-plan-conflict` | `internal/localcompletion/lifecycle.go:69` | the session already has an active enrollment for a different plan digest |
| `final-check-failed` | `internal/localcompletion/finish.go:98` | the final `dogfood-check.sh` run did not pass |
| `final-check-not-prerequisite` | `internal/localcompletion/storage.go:176` | a check argv element is `dogfood-check` or ends in `/dogfood-check.sh` |
| `immutable-base-required` | `internal/localcompletion/storage.go:137` | the plan base is not a Git object id |
| `initial-receipt-secret-screened` | `internal/localcompletion/lifecycle.go:147` | a preserved pre-change query or impact receipt matches the secret screen; recorded as the `NOT_PRODUCED` reason in its refusal metadata |
| `input-bound-exceeded` | `internal/localcompletion/storage.go:45` | strict JSON input is empty or larger than its bound |
| `intent-path-not-found` | `internal/localcompletion/lifecycle.go:127` | an intent path is absent from the tree of the current target commit |
| `invalid-check-argv` | `internal/localcompletion/storage.go:163` | a check's first argv element is empty, or any element exceeds 4096 bytes or contains NUL |
| `invalid-check-bound` | `internal/localcompletion/storage.go:159` | a check has fewer than 1 or more than 64 argv elements, or a timeout outside 1..3600 seconds |
| `invalid-check-id` | `internal/localcompletion/storage.go:155` | a check id does not match `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$` or repeats |
| `invalid-dogfood-context` | `internal/contextindex/local_completion_context.go:59` | prompt context was requested without an index at an immutable revision |
| `invalid-dogfood-event-arguments` | `cmd/corvint/local_completion_event.go:97` | an event option is repeated |
| `invalid-dogfood-event-budget` | `cmd/corvint/local_completion_event.go:110` | the requested byte budget exceeds the event maximum |
| `invalid-dogfood-event-input` | `cmd/corvint/local_completion_event.go:171` | the event input exceeds the kernel input bound or is not valid UTF-8 |
| `invalid-enrollment-generation` | `internal/localcompletion/storage.go:297` | the saved generation is not 68 bytes prefixed by the plan digest and `-` |
| `invalid-enrollment-pointer` | `internal/localcompletion/storage.go:321` | a saved intent pointer names another path than its plan intent, or a revision or blob hash that is not a Git object id |
| `invalid-execution-path` | `internal/localcompletion/storage.go:316` | a saved executable path is not absolute, not clean, or longer than 4096 bytes |
| `invalid-intent-scope` | `internal/localcompletion/storage.go:145` | a plan intent is not a valid path or is not strictly after the previous intent |
| `invalid-lifecycle` | `internal/localcompletion/storage.go:309` | the saved lifecycle is not `active`, `satisfied` or `cancelled` |
| `invalid-local-completion-action` | `cmd/corvint/local_completion.go:117` | the action is not `begin`, `verify`, `review`, `status`, `finish` or `cancel` |
| `invalid-local-completion-json` | `internal/localcompletion/storage.go:50` | strict JSON input does not parse |
| `invalid-local-completion-option` | `cmd/corvint/local_completion.go:126` | an option is not allowed for the action |
| `invalid-local-completion-option-value` | `cmd/corvint/local_completion.go:139` | an option value is empty or longer than 4096 bytes |
| `invalid-local-completion-schema` | `internal/localcompletion/storage.go:58` | strict JSON input parsed and passed the JSON type check, but decoding into the target type with unknown fields disallowed failed; the JSON type check emits the same code at `internal/localcompletion/storage.go:67`, and a required-field read of input that is not an object at `internal/localcompletion/storage.go:342` |
| `invalid-local-state-directory` | `internal/localcompletion/lifecycle.go:547` | the session's generation path exists and is not a directory |
| `invalid-public-evidence-result` | `internal/localcompletion/finish.go:197` | a public evidence command exited zero but its stdout is not JSON with `ok: true` |
| `invalid-report-set-digest` | `internal/localcompletion/lifecycle.go:435` | the review's report-set digest is not 64 lowercase hex |
| `invalid-review-digest` | `internal/localcompletion/storage.go:334` | a saved review digest is present and not 64 lowercase hex |
| `invalid-session-key` | `internal/localcompletion/storage.go:34` | the session key is not 64 lowercase hex |
| `invalid-tree-listing` | `internal/localcompletion/storage.go:486` | a tree-listing row has no tab separator |
| `invalid-verification-exit` | `internal/localcompletion/storage.go:327` | a saved exit is not the canonical decimal of an integer in -1..255 |
| `invalid-verification-observation` | `internal/localcompletion/storage.go:330` | a saved observation's log paths are not the check's numbered logs, or its target, tree, check digest or content digest is malformed |
| `invalid-worktree-owner` | `internal/localcompletion/storage.go:400` | the worktree owner file does not hold a 64-hex key |
| `local-completion-action-required` | `cmd/corvint/local_completion.go:53@df0e82dd` | `local-completion` is given no action argument |
| `local-completion-failed` | `cmd/corvint/local_completion.go:167@e27e19ee` | the failure code to emit contains a character other than `a-z` or `-`, is empty, or is longer than 96 bytes, so it is replaced |
| `local-completion-option-required` | `cmd/corvint/local_completion.go:144` | the action's required option is missing |
| `local-completion-option-value-required` | `cmd/corvint/local_completion.go:134` | a non-inline option is the last argument, or its next token is option-like (`GPK-V0-064`, decision 0196) |
| `local-outcome-evidence-drift` | `internal/localcompletion/finish.go:476` | the local outcome artifact is unreadable or its digest differs from the report's |
| `local-state-bound-exceeded` | `internal/localcompletion/storage.go:226` | a local state file is larger than its read bound |
| `local-state-not-regular` | `internal/localcompletion/storage.go:210` | a local state file is not a regular file |
| `local-state-symlink` | `internal/localcompletion/storage.go:193` | a local state path or one of its parents is a symlink |
| `log-secret-screened` | `internal/localcompletion/finish.go:408` | a process's stdout or stderr matches the secret screen |
| `missing-local-completion-field` | `internal/localcompletion/storage.go:346` | a required field is absent from the JSON input |
| `ocm-bindings-required` | `internal/localcompletion/finish.go:145` | the dogfood OCM aggregate status is not OK |
| `operation-in-progress` | `internal/localcompletion/storage.go:414` | the operation lock directory cannot be created |
| `output-bound-exceeded` | `internal/localcompletion/finish.go:240` | a bounded output buffer would exceed the artifact byte bound |
| `plan-bound-exceeded` | `internal/localcompletion/storage.go:140` | the plan has fewer than 1 or more than 16 intents or checks |
| `plan-unavailable` | `internal/localcompletion/storage.go:236` | the plan file's parent directory does not resolve |
| `prior-completion-stale` | `internal/localcompletion/lifecycle.go:77` | the session's satisfied enrollment for a different plan no longer evaluates as satisfied |
| `public-command-output-bound` | `internal/localcompletion/finish.go:182` | a public evidence command overflowed its stdout or stderr bound |
| `public-evidence-command-failed` | `internal/localcompletion/finish.go:194` | a public evidence command exited non-zero without a valid code on stderr |
| `public-report-path-invalid` | `internal/localcompletion/finish.go:283` | the report path is not an allowed path |
| `public-report-path-missing` | `internal/localcompletion/finish.go:278` | the result has no string `report` field |
| `report-set-stale` | `internal/localcompletion/lifecycle.go:461` | at review, the tree is not clean, the saved report is not current, or the report-set digest differs |
| `repository-identity-changed` | `internal/localcompletion/storage.go:506` | the repository root, Git directory or common directory changed since open |
| `repository-snapshot-drift` | `internal/localcompletion/lifecycle.go:253` | the target, tree or cleanliness changed during evaluation |
| `repository-unavailable` | `internal/localcompletion/storage.go:38` | the Git authority for the repository root cannot be opened; the session key was already checked |
| `secret-shaped-plan` | `internal/localcompletion/storage.go:148` | a plan intent matches the secret screen |
| `selected-check-unverified` | `internal/localcompletion/finish.go:338` | a plan check has no qualifying observation for the current snapshot |
| `session-identity-required` | `internal/localcompletion/types.go:154` | no explicit session key and neither `CODEX_THREAD_ID` nor `CODEX_SESSION_ID` is set |
| `uncommitted-work` | `internal/localcompletion/lifecycle.go:393` | the tree is not clean before verification |
| `unknown-selected-check` | `internal/localcompletion/lifecycle.go:383` | the selected check id is not in the plan |
| `verification-attempt-bound-exceeded` | `internal/localcompletion/lifecycle.go` | Verify refuses at 64 saved observations; evaluation also exposes this unmet reason when any selected check remains unqualified |
| `verification-cancelled` | `internal/localcompletion/lifecycle.go:428` | the context was cancelled after the observation was saved |
| `verifier-disagreement` | `internal/localcompletion/finish.go:479` | on the final read, the report's dogfood check outputs do not agree |
| `worktree-already-enrolled` | `internal/localcompletion/lifecycle.go:53` | another session holds an active enrollment for the worktree |
| `worktree-owner-mismatch` | `internal/localcompletion/storage.go:425` | the worktree owner is not this session |
| `worktree-prior-completion-stale` | `internal/localcompletion/lifecycle.go:61` | another session's satisfied enrollment of the worktree no longer evaluates as satisfied |

### Resolution and Stop decision reasons

Prompt-context resolution and the Stop decision carry the kebab-case reasons below (decision 0100).
Each row cites the first emitting site and states only the condition checked there.

| Code | First emitting site | Condition at the cited site |
|---|---|---|
| `ambiguous-anchor` | `internal/contextindex/local_completion_context.go:387@0e96865e` | the resolution `reason` when no earlier case applies and an anchor matched more than one candidate |
| `anchor-evidence-unavailable` | `internal/contextindex/local_completion_context.go:383@b7524c52` | the resolution `reason` when anchors exist and a task-evidence candidate was unreadable or requirement definitions were capped |
| `anchor-not-found` | `internal/contextindex/local_completion_context.go:385@07c6c8fe` | the resolution `reason` when no earlier case applies and an anchor matched no candidate |
| `anchor-worktree-changed` | `internal/contextindex/local_completion_context.go:389@37e9b097` | the resolution `reason` when no earlier case applies and a task-evidence path is among the index's dirty paths |
| `local-policy-continuation-limit` | `cmd/corvint/local_completion_event.go:340@50f727f3` | a `stop` event that would block has `stopHookActive` true; decision `release` |
| `local-policy-incomplete` | `cmd/corvint/local_completion_event.go:338@3862af35` | a `stop` event whose lifecycle is `active`, or `satisfied` without the evaluation satisfied; decision `block` |

## Resource and trust boundaries

The plan is at most 64 KiB with at most 16 intent scopes, 16 checks and 64 argv elements per check.
Each check has a finite timeout (at most 3600 seconds); stdout and stderr are separately bounded.
At most 16 preserved enrollment generations and 64 verification attempts per generation are admitted.
Report and state read limits are implementation constants covered by boundary tests. The native event
has the existing adapter event deadlines (1.6 seconds, 0.6 seconds for Claude Code session-end) and the 8000-byte
complete-response ceiling. Both are further bounded by the `AHI-017` deadline: the declared host kill (2 seconds, 1 second for
Claude Code session-end) less the 0.4-second process reserve, measured from process start, with a watchdog that degrades an
event whose cancellation has not returned. No automatic check is allowed to extend that deadline. User-selected local commands and
review acknowledgments remain caller-owned observations even when their bytes are digest-bound.

## Acceptance evidence and traceability

| Requirement | Implementation/evidence to produce |
|---|---|
| LCP-V0-001 | `TestDogfoodEventReadOnlyEnrolledStopAndPrompt`; legacy CLI/harness frozen parity |
| LCP-V0-002 | `TestEnrollmentAndReadOnlyPolicy`; `TestLocalStateBoundsAndContention`; `TestDogfoodEventGoPythonWireAndSession`; `TestEnrollmentRefusesIntentAbsentFromBase`; `TestRefusedEnrollmentConsumesNoGeneration`; `TestEnrollmentPinsSameChangeIntentAtHead`; `TestEnrollmentRefusesHeadNotDescendedFromBase`; `TestFinishRefusesHeadMovedToUnrelatedHistory` |
| LCP-V0-003 | `TestEnrollmentAndReadOnlyPolicy`; `TestCompletionNextActions`; `TestFinishRefusesHeadMovedToUnrelatedHistory`; `TestLocalCompletionRealEvidenceWorkflow`; `TestDogfoodEventReadOnlyEnrolledStopAndPrompt` |
| LCP-V0-004 | `TestActualVerificationAndSecretRefusal`; `TestVerificationCancellationCleansDescendant`; `TestExplicitSidecarReuseRequiresCanonicalMap` |
| LCP-V0-005 | `TestLocalCompletionRealEvidenceWorkflow`; actual CEM/OCM/coordinator/checker fixture |
| LCP-V0-006 | `TestLocalCompletionRealEvidenceWorkflow`; generated-but-unread/stale report refusals |
| LCP-V0-007 | `TestLocalCompletionRealEvidenceWorkflow`; strict-check and repeated-finish assertions |
| LCP-V0-008 | `TestDogfoodEventStopLifecycle`; `TestDogfoodEventStrictInputAndDeadline` deadline code and uncancellable-read expiry; native first/recursive Stop regressions |
| LCP-V0-009 | `TestDogfoodEventGoPythonWireAndSession`; `TestDogfoodEventStrictInputAndDeadline`; `TestAdapterRejectedReasonSurfacesEngineErrorCode`; `TestRejectedEventReasonAppendsEngineCode`; `TestAdapterErrorTailIsBounded` |
| LCP-V0-010 | `TestDogfoodPromptFrozenAnchors`; `TestDogfoodPromptScopeDoesNotResolveAndPreservesStaleness`; frozen follow-up and explicit/unknown/ambiguous anchors |
| LCP-V0-011 | `TestDogfoodPromptPrivacyNoHistoryAndNonmutation`; `TestDogfoodPromptCriticalBudgetAndImpossibleEnvelope` |
| LCP-V0-012 | `TestLocalStateBoundsAndContention`; `TestVerificationCancellationCleansDescendant`; `TestDogfoodPromptBoundsAndCancellation` |

## Rollout, rollback and remaining gates

Record decision 0009 option 2; independently review the plan; implement and test in an isolated
worktree. Run the canonical gate and relevant frozen regressions, inspect the new workflow's reports,
then complete the existing post-commit dogfood loop against the original base. Rebuild the native
binary and refresh the Codex plugin. Preserve verified compatibility files for active tasks whose
cached path an install removes. Verify current hook discovery/trust and real installed events.
Rollback disables/cancels the explicit enrollment or restores the legacy adapter; legacy evidence
formats and commands remain usable. No claim of fully qualified native-platform support follows.

Cross-host executable witnesses: `TestClaudeNativeDogfoodLifecycle` invokes the actual Python
adapter and production Go CLI for identity/handoff, first/recursive Stop, governed context,
startup sources, read-only state and closed host admission. `tests/test_local_completion_claude.py`
and the existing adapter suites cover exact golden domains, all six stdio routes, resealed
provenance negatives, complete-frame bounds and interruption. These are adapter/core fixtures,
not observations of a real Claude/Codex model-host lifecycle.
