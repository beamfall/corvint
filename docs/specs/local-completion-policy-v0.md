# Local Completion Policy V0

Owner: Russell Lewis
Date: 2026-09-06
Intent status: accepted direction (owner selected decision 0009 option 2 in the 2026-09-06 Codex dogfood repair task); LCP-V0-008/009 Pi tuple accepted (decision 0452; V1-0506)
Delivery status: implemented
Authoritative inputs: `docs/DOGFOOD.md`, `docs/decisions/0009-harness-authority-boundary.md`, `docs/specs/agent-harness-integration-v0.md`

## Agent digest
- Claim: Explicitly enrolled changes require selected checks, bound evidence and inspected reports before local completion; no execution attestation.
- Status: accepted direction (owner selected decision 0009 option 2 in the 2026-09-06 Codex dogfood repair task); LCP-V0-008/009 Pi tuple accepted (decision 0452; V1-0506)/implemented
- Exists: all 15 in-scope requirements have executable local evidence for the workflow, prompt compiler and native adapter; `LCP-V0-016` (one repository bracket per dogfood event) is accepted by decision 0439 (V1-0881). `LCP-V0-017` (envelope `ok` mirrors the exit status) is accepted by decision 0465 (V1-1012). `LCP-V0-018` (plan intent refusals name the rule) is accepted by decision 0472 (V1-1043). `LCP-V0-019` proposes reuse of immutable facts inside one enrolled event while re-reading private inputs and cleanliness (V1-1124; accepted, decision 0508). `LCP-V0-033` (snapshot read overlapping the opening status read) is accepted under the owner's lane auto-acceptance (V1-1524, V1-2144). `LCP-V0-034` (named prompt-bounds refusal code) is accepted under the same auto-acceptance (V1-1976). `LCP-V0-036` (base-ancestry probes refuse a grafted or shallow checkout with `ancestry-rewritten`) is accepted under the same auto-acceptance (V1-1757). `LCP-V0-037` (begin refuses an intent without numbered requirements) is accepted under the owner's standing auto-accept (V1-0680). `LCP-V0-038` (cancel of an active enrollment requires `--reason`) is accepted under the owner's standing auto-accept (2026-10-11) (V1-2177). `LCP-V0-040` adds strict sealed continuation with distinct current/evidence targets; aggregate and context-abstention continuation remain unsupported. `LCP-V0-041` (sealed continuation re-pins only the lane closeout's own proved outcome record) and `LCP-V0-042` (drift refusal names the artifact and digests) are accepted under the owner's standing lane auto-accept (V1-3194). `LCP-V0-043` (a signal during the final strict check reports `finish-interrupted`, not `final-check-failed`) is accepted under the same auto-accept (V1-3192). `LCP-V0-060` (a real signal through the `dogfood finish` CLI keeps the active policy on stdout beside the stderr refusal, and the next finish reuses the bound observations) is accepted under the same auto-accept (V1-3192). `LCP-V0-880`..`LCP-V0-882` (coordinator preservation hold before a cancel) are experimental under the standing lane auto-accept (V1-0581). `LCP-V0-883`..`LCP-V0-885` (enrollment export and import across a worktree archive and restore) are experimental under the same auto-accept (V1-0580).
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
  Only an existing final lock path (`EEXIST`) reports `operation-in-progress`. Permission denial
  while creating the lock or its parent reports `operation-lock-permission-denied`; other creation
  failures report `operation-lock-create-failed`. These public codes MUST remain bounded and
  path-free while preserving the filesystem cause for `errors.Is` and `errors.As`. A failure MUST
  NOT remove an existing lock or modify enrollment/owner state (owner bug-fix request 2026-09-28).
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
  not satisfied. When this key has no enrollment and the worktree owner is another key whose
  enrollment is active, the inactive evaluation adds that key as `owner` (V1-0295); the member grants
  nothing and never takes over the enrollment. Persistent lifecycle is only active, satisfied or explicitly cancelled; cancellation
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
- `LCP-V0-005`: `finish` MUST reuse the public CEM/OCM and daily dogfood workflow (`LCP-V0-014`). It MUST NOT
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
  pre-change receipts are archived as bounded enrollment-owned records at enrollment; the
  coordinator never writes them and labels its own receipts coordination-time (`DCW-V0-026`). Existing
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
  A separately authorized, digest-bound `coordination-time-impact` context abstention is not a missing
  completion proof: it remains visibly `NOT_PRODUCED`, while CEM/OCM, checks, clean-target and
  verifier requirements above remain unchanged. Its malformed, missing, stale or untyped form fails.
- `LCP-V0-008`: Codex and Claude Code Stop, and the exact ordinary Pi final-settlement boundary, MAY request one bounded remediation continuation for an explicitly enrolled
  incomplete change. If `stop_hook_active` is true, the adapter MUST release with a visible fixed
  unresolved-policy notice instead of looping. A blocking Stop reason MUST name the closed unmet
  categories of `policy.unmet` and the `dogfood status` argv for the session key, and never a
  caller check ID (amended 2026-09-27, V1-0298). Inactive, satisfied and cancelled states are distinct.
  A Stop whose evaluation names another session's active `owner` releases with
  `local-policy-other-session-active`, and its completion carries that `owner`. The Codex and Claude
  Code adapters then show a fixed notice that this session is not gated, followed by the keyed
  `dogfood status` argv for that owner, so a session that lost its key (for example after `/clear`)
  does not pass Stop silently (V1-0295). The qualified lifecycle's closed completion keeps only the
  decision and reason.
  Timeouts, malformed input and unsupported hosts fail open with explicit failure, not a satisfied
  claim. An automatic event whose deadline expires reports the fixed `dogfood-event-deadline` code
  (or `dogfood-event-index-snapshot-stale` once the read fell back to the in-memory build of a
  snapshot miss; accepted 2026-09-25 with `AHI-031`, decision 0400), never the generic `dogfood-event-unavailable`, even when the expiry first surfaces as a failed
  repository or policy read. The Git repository probe's own fixed bound expiring first
  (`repository-probe-timeout`) is the same time bound and reports the same code (V1-0396), as does
  an event input read from stdin that ends with the deadline passed, never
  `dogfood-event-input-unavailable` (amended 2026-10-15, V1-1391). That code
  MUST be returned once the deadline passes, without waiting
  for a read stage that does not observe cancellation (the in-memory index compile of a snapshot
  miss); the abandoned read writes nothing. `dogfood-event-policy-drift` is reserved for an observed
  change: the policy target moved or both policy reads succeeded and disagree. A failed policy re-read
  observed no drift and reports as a read failure (`dogfood-event-unavailable`, or
  `dogfood-event-deadline` on expiry). A read refused because the worktree moved under it
  (`repository-state-unstable` from the probe, `dogfood-event-repository-drift` or
  `dogfood-event-context-drift`) reports the fixed `dogfood-event-worktree-churn` code, and
  `dogfood-event-policy-drift` keeps its own code; a prompt the bounded prompt-context contract
  refuses reports `dogfood-event-prompt-bounds` (`LCP-V0-034`); every other read failure stays
  `dogfood-event-unavailable` (amended 2026-10-16, decision 0533, V1-0880; amended 2026-10-21, V1-1976). A supplied `stop_hook_active` MUST be boolean. Expensive
  verification and mutations never run in Stop. User interruption remains effective.
  The owner’s 2026-09-29 Pi build request additionally admits Pi 0.99.1 / extension
  0.3.8 (superseding 0.3.7, 0.3.6, 0.3.5, 0.3.4, 0.3.3 and 0.3.2, decisions 0550, 0556, 0558 and 0583, V1-2150, V1-2854). Its `agent_before_settle` handler evaluates only a trusted completed activity
  without an existing continuation or pending message. A validated incomplete policy
  may append one fixed remediation custom message and request one continuation; the
  same activity then supplies `stopHookActive: true` and cannot loop. Errors, aborts,
  queued work, stale generations and failed reads never request continuation. A new
  explicit operator activity may reset that bound. Automatic recovery is ephemeral.
  Pi hashes `corvint-local-completion-session/pi/0` plus NUL plus the host session ID;
  forks and tree navigation do not adopt another session’s enrollment. Slash commands
  are operator assertions, not authenticated human provenance, because RPC can invoke
  the same command surface. This remains local caller policy with FALLBACK support; the Pi tuple amendment is accepted by decision 0452 (V1-0506).
- `LCP-V0-009`: `corvint dogfood event` MUST use a separate `corvint-dogfood-event/0` envelope and
  result-digest domain. Legacy query, task-context and harness command behavior stays unchanged.
  The result binds its full normalized public content, exact repository snapshot, event and local
  policy state. Native adapters validate that envelope before using its completion decision. The
  automatic command shares one bounded deadline and a complete 8000-byte response ceiling, except
  Claude Code session-end, whose declared host kill is tighter than the other admitted event/host
  pairs and which therefore uses its own shorter deadline strictly below that kill.
  The admitted plugin tuple is host `codex` with host version `unknown` or host `claude-code` with
  host version `unreported-by-hook-api` (`AHI-023`), adapter version `0.1.0`, for session-start/user-prompt/stop/session-end only. Any other tuple refuses with
  `unsupported-dogfood-event-host`. The separate ordinary Pi tuple is host `pi`, host
  version `0.99.1`, surface `extension`, adapter version `0.3.8` (superseding `0.3.7`, `0.3.6`, `0.3.5`, `0.3.4`, `0.3.3` and `0.3.2`, decisions 0550, 0556, 0558 and 0583), for the same four
  events only. It shares the sealed profile and bounds but never the protected Pi
  profile or its authority. Any other event refuses with `unsupported-dogfood-event`
  (`cmd/corvint/local_completion_event.go`, `dogfoodEventInput`). Claude post-tool and
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
  Without an explicit path, requirement ID, source-backed identifier or task mention (`LCP-V0-013`)
  anchor, automatic context retains bounded governance/scope and reports
  `explicit-task-anchor-required`; it withholds incidental lexical rows. Unknown/ambiguous anchors and stale scope remain explicit. General natural-language
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
  (`internal/contextindex/local_completion_context.go`, `failBounds` and `packet`); the native event reports either as
  the fixed `dogfood-event-prompt-bounds` (`cmd/corvint/local_completion_event.go`, `dogfoodReadFailureCode`; `LCP-V0-034`, V1-1976; it was `dogfood-event-unavailable`).
  Amendment (2026-10-07, accepted by decision 0441; V1-0939, V1-0942): the receipt rules
  above are unchanged, but the Claude Code and Codex adapters inject its `corvint-hook-context/0`
  projection (`AHI-045`); an anchorless prompt whose receipt holds only governance, current scope
  and `explicit-task-anchor-required` injects nothing (`AHI-046`); and the trusted workflow guidance
  appears only on a main-thread SessionStart (`AHI-047`).
- `LCP-V0-012`: Private plans, observations and report sets MUST have explicit byte/count limits,
  strict schema/duplicate-field/path validation, atomic publication and bounded ownership. No daemon,
  network, new database, transcript scanner or arbitrary background executor is added. Every spawned
  process uses cancellation and descendant cleanup with a regression. A concurrent enrollment cannot
  silently overwrite an active owner. Two-session contention and same-owner interleaving must
  prove shared legacy artifacts cannot be acknowledged or published under the wrong owner. Errors preserve prior valid state.
- `LCP-V0-013`: Prompt context MUST accept a task mention as an explicit anchor. A mention is one
  whitespace-delimited field, after trimming leading quotes, backticks, asterisks and brackets and
  any run of trailing punctuation or symbols other than `_`, whose path part is path-shaped (it
  contains `/`, has an extension, or is tracked). So `localhost:8080` and `issue#12` stay ordinary
  text, while a dotted host such as `example.com:443` is path-shaped and is `anchor-not-found`. A
  matched field is removed whole from the text left for path and identifier anchors, and mentions
  are deduplicated by their parsed form. Each form has one resolution rule: (1) `path:line`,
  `path:line-line` and `path:line:column` (which names the line) resolve the path exactly, or a bare
  file name by base name, and require `1 <= start <= end <=` the bound source's line count. The row
  names the start line with relation `explicit-line` and authority `task-text`; the row schema is
  unchanged. A line outside the source is `anchor-not-found`, more than one in-range candidate is
  `ambiguous-anchor`, and an unreadable source is `anchor-evidence-unavailable`. (2) `path#name`
  matches declarations named exactly `name` within the resolved path only. None is
  `anchor-not-found` (a `#L12` fragment is such a miss, not a line form), more than one is
  `ambiguous-anchor`, and a dotted name that is not itself a symbol falls back to its terminal part
  as `qualification-unverified`. (3) A commit is 7 to 40, or exactly 64, lowercase hex characters
  with at least one digit and one letter. Only a prefix of the bound commit resolves, as a revision
  anchor with no path row; any other commit is `anchor-evidence-unavailable`, because this profile
  reads no history (`LCP-V0-011`). (4) A requirement ID keeps its existing definition rule. Every
  row is pinned to its bound blob, a dirty path stays `anchor-worktree-changed`, and a dirty
  candidate with no match in its bound blob (a line or declaration added in the worktree) is also
  `anchor-worktree-changed` with no row. Such a candidate still counts, so beside another match the
  anchor is `ambiguous-anchor`. A zero or inverted line range is `anchor-not-found` even on a dirty
  path. Mentions count toward the 32-anchor bound, and no mention is resolved lexically, by pronoun,
  or from a transcript.
- `LCP-V0-014`: `finish` MUST run the daily change (coordination) and the final strict check in the
  running binary, through the Go implementation behind `dogfood change` and `dogfood check`
  (`DCW-V0-020`), with that binary as every verifier role. Its base and tree verifier digests are
  therefore equal, and their agreement shows only that one binary is self-consistent, not that the
  change left the checker's verdict unchanged; that independent base build stays with
  `make dogfood-check` (`DCW-V0-022`). It MUST NOT run a repository script,
  read `VERSION`, build `cmd/corvint`, resolve `corvint` on `PATH`, or read `CORVINT_BIN` or any
  `DOGFOOD_*` variable; its coordinator inputs are the fixed values in `LCP-V0-005`. So it works in
  any Git repository whose worktree ignores the daily path's private outputs. Each run keeps the
  former bounds: a 10-minute deadline and `maxLogBytes` per stream, where overflow, cancellation
  or a nonzero exit fails the run. The deadline kills a running Git or step child process and is
  checked between steps; an in-process step is not pre-empted, and descendants of a killed child
  are not reaped as a group. Logs are secret-screened before they are retained.
- `LCP-V0-015`: A selected check MUST NOT run the final check itself. Besides the `dogfood-check`
  names, an argv containing the adjacent pair `dogfood check` or `dogfood seal` is refused as
  `final-check-not-prerequisite`.
- `LCP-V0-016`: (accepted by decision 0439; V1-0881; no GitHub issue)
  A dogfood event MUST take exactly one `GPK-V0-007` bracket: one complete identity-and-status
  observation before its reads; the policy evaluation, envelope and context packet read against
  that observation, where the index loader hits the snapshot the observed tree names and applies
  the observed dirty set, or builds in memory on a miss; and one complete observation after. The
  event MUST NOT probe the repository a second time, MUST NOT let the index loader take an
  identity or status read of its own, and MUST NOT issue the `ls-tree` profile read, which no
  event envelope emits. A closing observation whose identity or dirty set differs from the
  opening one is refused as `dogfood-event-repository-drift` and never retried; the native
  surface reports it as its fixed `dogfood-event-worktree-churn` refusal with no envelope
  (amended 2026-10-16, decision 0533, V1-0880; it was `dogfood-event-unavailable`).
  Every other refusal, the envelope bytes and the context packet are unchanged. Measured in the
  fixture: four Git processes per event where `stop` took ten and an index-backed event
  thirteen. Rollback: revert the change; the two-probe path keeps no persistent state.
- `LCP-V0-017`: (accepted, decision 0465; V1-1012) The top-level `ok` of a `corvint-local-completion/0`
  envelope from `begin`, `status`, `verify`, `finish`, `review` or `cancel` MUST equal whether the
  command exits 0. `verify` exits 1 with `ok:false` when its recorded observation is unqualified
  (nonzero exit, timeout, overflow, dirty run or screened log; a refusal still exits 2), and
  `finish`, including the transport-adapted recovery route, exits 1 with `ok:false` while the
  policy is not satisfied. The emitted `policy` (its `checks[].qualified` and `exit`, `satisfied`,
  `unmet` and `nextActions`) is unchanged and remains the detail; `ok` alone never reads a failed
  check or an unsatisfied finish as success. `status` stays read-only with exit 0 and `ok:true`
  whatever the policy state. Before this, a selected check that exited 1 produced exit 1 but
  `ok:true`. Rollback: revert the change; no persisted state or other envelope member changes.
- `LCP-V0-018`: (accepted, decision 0472; V1-1043) A `dogfood begin` plan refusal of `invalid-intent-scope` or
  `plan-bound-exceeded` MUST keep its stable code in the top-level `code` and nested `error.code`,
  and its nested `error.message` MUST be the code followed by `: ` and a fixed remediation naming
  the rule: the plan `intents` array lists 1-16 repository-relative spec paths, sorted in byte
  order with no duplicates, the same rule `DOGFOOD_INTENTS_FILE` follows for `dogfood change`
  (whose `missing-intent-scope` fix line names it). The text is fixed, so the message still carries
  no path or command text; every other refusal keeps `message` equal to its code. Before this, an
  unsorted or duplicated intents array printed only the bare code. Rollback: revert the change; no
  code, exit class or persisted state changes.

- `LCP-V0-019`: (accepted, decision 0508) An enrolled dogfood event
  MUST pin repository identity once for policy evaluation. Its re-read of every private policy
  state and artifact input MUST reuse the first evaluation's immutable target, tree, content
  digest only when the closing observation matches that target and tree.
  Base ancestry depends on mutable shallow/graft metadata and MUST be re-proved on every read.
  The re-read MUST still prove cleanliness with localcompletion's own Git status semantics
  before and after its reads, re-resolve HEAD after the closing status, and retain the policy-drift comparison and existing refusals.
  Otherwise it MUST fall back to a full evaluation, never skip the private-input re-read.
  Drift-only snapshots MUST omit content listing; snapshots using content MUST retain it.
  Acceptance: the clean enrolled Stop trace falls from 28 to 22 Git processes, existing Stop
  results remain unchanged, and same-host CPU/wall measurements are retained in the build log.
  Rollback: revert the change; no persisted state or wire format changes.
- `LCP-V0-020`: (accepted, decision 0510) Every `dogfood status`
  and `dogfood finish` evaluation MUST carry nonempty `nextActions` or a stable
  `noActionReason`, including when Frontier authority is `UNAVAILABLE` with
  `frontier-authority-unavailable`. Active incomplete evaluations retain the eligible
  keyed actions from LCP-V0-003 and omit `noActionReason`. Inactive, cancelled and
  satisfied evaluations offer no continuation and name `enrollment-inactive`,
  `enrollment-cancelled` and `workflow-satisfied`, respectively. A partially evaluated
  CLI result with an error and no actions names `evaluation-unavailable`; refusals
  without an evaluation retain their existing named error code. The additive optional
  JSON field MUST surface in the CLI `policy` consistently. Frontier unavailability
  MUST NOT become local satisfaction or imply authority. Rollback: remove the additive
  field and its assignments; persisted state and existing actions are unchanged.
- `LCP-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1014) When a `verify` check log
  matches the secret screen, the observation MUST stay `secretScreened` and unqualified, and each
  retained log MUST begin with the line `log-secret-screened` followed by the captured output with
  every secret-shaped span replaced by `[REDACTED]`, so test names and FAIL lines stay readable. If
  the redacted text still matches the screen, would exceed the 256 KiB log bound, or the captured
  output contains a PEM or PGP private-key block header (complete block or not), the log MUST hold
  only the marker line. The `verify` result for an unqualified run MUST name, in additive
  `checks[].failedTests`, the distinct test names of `--- FAIL: NAME` lines in its retained logs,
  at most 16 names of at most 256 bytes each, none matching the screen; status and finish do not
  re-read logs for it. Rollback: restore the marker-only log and drop the additive field.
- `LCP-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1011) Each plan check takes 1-64
  argv elements and a `timeoutSeconds` of 1-3600. The `invalid-check-bound` refusal from `dogfood
  begin` MUST keep that stable code and its message MUST name both bounds, the offending check id
  and the requested argv count and timeout, so a caller can split an over-long run into several
  checks. The message carries no path or command text, and an id that matches the secret screen is
  shown as `[REDACTED]`. Rollback: drop the message detail.
- `LCP-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1524, V1-2144) A dogfood
  event that reads a context packet (`user-prompt`, `session-start`) MUST start its snapshot read
  from the opening observation's identity (object format, `HEAD`, tree) as soon as that identity
  read returns, while the same observation's status read is still running, and MUST then apply the
  complete opening observation (root, dirty set, status digest, observed `HEAD`) to the hit exactly
  as `LCP-V0-016`'s loader does. The overlap MUST NOT add a Git process, change the bracket, read a
  snapshot for any tree other than the opening one, or change the envelope or packet bytes; a
  prefetch the bracket did not start, or one for another object format or tree, is not used and
  the event reads the snapshot sequentially. Measured on a 708-tracked/759-untracked dirty
  worktree at load 31-40, the `pre-compact` adapter median fell from 0.879 s to 0.678 s (100
  interleaved runs each, no run over the 1.6 s event deadline; build log
  `2026-10-21-hooks-1021-snapshot-prefetch`). Rollback: call `gokernel.ProbeRepositoryAround`
  again and drop the prefetch key.
- `LCP-V0-034`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1976) A dogfood event whose
  prompt context is refused by the bounded prompt-context contract (`*contextindex.Error` with code
  `unsupported-dogfood-context-bounds`, `unsupported-dogfood-context-budget` or
  `invalid-dogfood-context`) MUST report the one fixed `dogfood-event-prompt-bounds` code, and the
  host adapter's degradation reason MUST carry it as
  `corvint-event-rejected:dogfood-event-prompt-bounds`, which `SOL-V0-010` admits into the
  self-observation ledger. Only the typed code is read, never message text; any other
  `contextindex` refusal or untyped failure stays `dogfood-event-unavailable`. The code names a
  cause, not a remedy: the bounds themselves are unchanged (an over-bound anchor set now degrades to a
  partial packet under `LCP-V0-035`). Rollback: drop the `contextindex.Error` branch in
  `dogfoodReadFailureCode` and the registry entry; the refusal returns to
  `dogfood-event-unavailable`.
- `LCP-V0-035`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2276) A prompt
  whose derived explicit anchors (requirement IDs, mentions, paths, identifiers) exceed the
  32-anchor profile MUST NOT be refused: the compiler keeps the first 32 in the fixed order
  requirement IDs, mentions, paths, identifiers, each sorted, and resolves only those. The packet's
  `resolution` MUST carry, only then, `dropped_anchors` (the count) and `dropped_anchor_kinds` (count per kind
  that lost anchors); the dropped anchors are raw task terms, so `LCP-V0-011` keeps them out of
  the packet and they are named by kind and count only. A prompt that otherwise resolves reports
  `unresolved`/`anchor-bound-exceeded`; an earlier reason (`anchor-not-found` and the rest) keeps
  precedence. This amends `LCP-V0-011`: `unsupported-dogfood-context-bounds` remains only for a
  requirement ID with more than 128 candidate definitions, and the byte bound stays
  `invalid-dogfood-context`. Non-goals: raising the 32-anchor profile or ranking anchors by
  relevance. Failure modes: a long prompt whose most useful anchors sort late loses them; the
  counts say so. Rollback: restore the `failBounds` call in `addAnchors` and drop the two fields.
- `LCP-V0-036`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1757) Every base-ancestry
  probe in `LCP-V0-002` (`begin`, evaluate, `finish` and the aggregate finish paths) MUST first apply
  the shared ancestry guard (`EEP-V0-028`, `internal/gitancestry`): when the checkout's Git common
  directory holds `info/grafts`, the checkout is shallow, or that metadata cannot be read, the probe
  refuses with `ancestry-rewritten` instead of asking `git merge-base --is-ancestor`, whose answer
  such a checkout rewrites. The guard reads those files from the already-resolved common directory
  (`gitancestry.RewrittenAt`, which names the same reasons as `gitancestry.Rewritten`) rather than
  spawning Git, so the `LCP-V0-019` Git-process bound on the Stop path is unchanged. The refusal is a hard error at every site, before any local write, so it
  consumes no enrollment generation and never reaches `satisfied`; it is not reported as the
  `base-not-ancestor-of-target` unmet condition, because the checkout made no claim about history.
  Rollback: drop the guard call in `requireBaseAncestor`.
- `LCP-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0680)
  `dogfood begin` MUST read each intent blob at the enrollment target and refuse it with the
  code finish's default-form `ocm prepare` would report for its numbered Requirements section
  (`invalid-intent`, `invalid-requirements-section`, `missing-requirements`,
  `duplicate-requirement` or `too-many-obligations`), and refuse a non-blob or over-4 MiB intent
  with `repository-object-unavailable`, before `plan.json` creates a generation directory, so a
  refused enrollment consumes no generation and leaves the session inactive. The check reuses
  the `ocm prepare` parser (`lrfrepo.IntentRequirementsRefusal`), so begin and finish cannot
  disagree on the same bytes. Non-goals: declared intent forms (finish prepares only the default
  form) and re-checking an intent edited after enrollment, which finish still refuses. Failure
  modes: a plan that enrolled a decision document or instructions file now stops at begin
  instead of after selected checks. Rollback: drop the requirements read in `Begin`.
- `LCP-V0-038`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2177)
  `dogfood cancel` of an `active` enrollment MUST be refused with `cancel-reason-required` and
  leave the saved state unchanged unless `--reason TEXT` is nonempty; the message MUST name the
  number of selected checks and saved observations the cancel would close as non-success and the
  route for a moved base (leave the enrollment ACTIVE and begin a new enrollment in a fresh
  worktree or clone). With a reason the cancel proceeds as before and the result echoes the
  reason as `cancelReason`; the reason is not persisted, so the closed state member set and older
  binaries' reads are unchanged. Cancelling a satisfied or cancelled enrollment needs no reason.
  Non-goals: recording the reason in state, a force flag, refusing cancel by an owner. Failure
  modes: a script that cancelled an active enrollment without a reason now exits 2 and must pass
  `--reason`. Rollback: drop the reason check in `Cancel` and the `--reason` flag.
- `LCP-V0-880`: (experimental under the standing lane auto-accept; V1-0581; refines `LCP-V0-038`)
  `corvint dogfood hold --session-key K --reason TEXT` MUST record the coordinator's preservation
  hold on K's `active` enrollment, which K owns in this worktree, as
  `<git-dir>/corvint/local-completion/holds/K.json` (profile `corvint-local-completion-hold/0`,
  the session, plan digest, BASE and reason), outside the session directory so it never counts as a
  generation; it changes no enrollment state. While the hold exists, `dogfood cancel` of that ACTIVE
  enrollment MUST refuse `enrollment-preservation-hold` (exit 2) with the state byte-identical, its
  detail quoting the hold's reason, naming the distinct-checkout route (leave it ACTIVE, begin a
  new enrollment in `git worktree add` or a fresh clone) and the release command. A hold on a
  non-active enrollment refuses `preservation-hold-requires-active`; a second hold refuses
  `preservation-hold-exists`; a hold file that does not decode as its profile and session refuses
  `invalid-preservation-hold`. The hold is the agent-facing check that compares a proposed cancel
  with the coordinator's governing instruction before any lifecycle mutation; it is separate from
  Core's technical capability, which `LCP-V0-038` already admits with a reason.
- `LCP-V0-810`: (experimental under the standing lane auto-accept, 2026-11-01; V1-3735; refines
  `LCP-V0-002`) Without `--session-key`, a manual `corvint dogfood` command run inside Claude Code
  (`CLAUDECODE=1` with a nonempty `CLAUDE_CODE_SESSION_ID` of at most 4096 bytes) MUST derive its
  default key exactly as the Claude native hook does (lowercase SHA-256 of
  `corvint-local-completion-session/claude-code/0`, one NUL byte, and the identifier), so it
  resumes the enrollment that hook reports. When a `CODEX_THREAD_ID` or `CODEX_SESSION_ID` is also
  set the host is ambiguous and the command MUST refuse `session-identity-ambiguous` rather than
  pick one; with neither host identity it refuses `session-identity-required`. Both refusals MUST
  name `--session-key KEY` as the fix. Codex's hash domain and precedence are otherwise unchanged.
  Evidence: `TestLCPV0810SessionKeyAdmitsClaudeCode` (`internal/localcompletion`),
  `TestLCPV0810SessionRefusalsNameSessionKey` (`cmd/corvint`). Rollback: drop the Claude branch,
  the ambiguity refusal and the two remedies.
- `LCP-V0-881`: (experimental under the standing lane auto-accept; V1-0581)
  `dogfood hold --session-key K --release-authorization TEXT` MUST remove the hold only after
  writing it, with the authorization, as the next `holds/K.releases/release-NNN.json`; release
  records are never removed or rewritten, so a later cancel is traceable to the authorization that
  admitted it. A release with no hold refuses `preservation-hold-absent`. Exactly one of `--reason`
  and `--release-authorization` MUST be given (else `invalid-local-completion-option`). Each text is
  one line of at most 512 bytes, refused `invalid-local-completion-option-value` otherwise, and
  refused `hold-text-secret-screened` when the secret screen matches. The cancel that follows keeps
  every raw observation, as `LCP-V0-038` does, and records non-success.
- `LCP-V0-882`: (experimental under the standing lane auto-accept; V1-0581) Qualification: the
  held case refuses the cancel and keeps the ACTIVE state; the distinct-checkout case enrolls a
  second worktree while the held enrollment stays ACTIVE; the authorized case releases with a
  recorded authorization and cancels, keeping the observations. Non-goals: a global cancellation
  ban, coordinator identity or signatures (the authorization is caller text), retroactive approval
  of a crossed boundary, rewriting historical state. Failure modes: an agent can still release the
  hold itself; the release record then names the authorization it claimed, which the coordinator
  adjudicates. Evidence: `TestLCPV0880PreservationHoldGuardsCancel`. Rollback: delete
  `internal/localcompletion/hold.go`, its `Cancel` check and the `hold` subverb.
- `LCP-V0-883`: (experimental under the standing lane auto-accept; V1-0580)
  `corvint dogfood export --session-key K --output FILE` MUST write, to a new FILE (refusing
  `export-output-exists`), a bundle of profile `corvint-dogfood-enrollment-export/0` naming K, its
  lifecycle, plan digest and BASE, HEAD's commit and tree, the source worktree root and Git
  directory, and every regular file of K's session directory (all generations) plus every worktree
  or Git-directory artifact the state names (report-set bindings and reports, terminal artifacts,
  coordination), and K's preservation hold and release records (`holds/K.json` and
  `holds/K.releases/*`, `LCP-V0-880`, `LCP-V0-881`) as `gitdir` files, each by anchor (`session`,
  `gitdir`, `root`), relative path, SHA-256 and bytes, at
  most 512 files of 4 MiB and 64 MiB in all (`export-bound-exceeded`), with the SHA-256 of the
  bundle. A path outside its anchor refuses `export-path-invalid` and an output it cannot create
  `export-output-unwritable`. It changes no repository or enrollment state (`"mutates": false`). An absent enrollment
  refuses `enrollment-absent`; one with an aggregate outcome or sealed continuation refuses
  `enrollment-export-unsupported`, since their ledgers name paths the bundle does not relocate.
- `LCP-V0-884`: (experimental under the standing lane auto-accept; V1-0580)
  `corvint dogfood import --session-key K --input FILE` MUST read FILE through its resolved parent
  directory, so a symlinked ancestor such as macOS `/tmp` or `/var` is admitted while a FILE that is
  itself a symlink is not, and MUST refuse, writing nothing: an unreadable
  or over-bound bundle (`import-bundle-unreadable`); a bundle whose digest, profile or any file digest fails (`import-bundle-invalid`); another key
  (`import-session-mismatch`); a worktree where K already has state (`import-would-replace`), so an
  import never resets or replaces an enrollment; another owner (`worktree-already-enrolled`); a HEAD
  whose commit or tree differs from the bundle's (`import-target-mismatch`); an unresolvable BASE
  (`base-unavailable`); a destination file with other bytes (`import-artifact-conflict`); and an
  absent worktree artifact that Git does not ignore (`import-artifact-not-ignored`); and, when this
  worktree's root or Git directory differs from the source's, a frozen plan any string of which names
  the source root or Git directory (`import-plan-not-relocatable`), since relocating it would break
  the plan digest that binds it. Otherwise it
  writes the session and Git-directory files, including the hold and release records, so a held
  ACTIVE enrollment stays held after the restore, the absent ignored worktree artifacts and the owner,
  and prints the restored policy.
- `LCP-V0-885`: (experimental under the standing lane auto-accept; V1-0580) Import MUST relocate
  every saved string path under the source Git directory or root to this worktree's, except the
  frozen plan, which is carried byte for byte so its plan digest still holds, then
  recompute the report-set digest only when it held at the source, carrying the review and
  terminal acknowledgments only when they named exactly that report set; nothing else changes, so
  the restored enrollment evaluates as its source did and never more satisfied: a satisfied one
  stays satisfied only when every bound artifact is current, an ACTIVE one stays ACTIVE with its
  observations and unmet items, and a stale report set stays stale. A worktree with no state and
  no bundle stays `inactive`: whether it ever held an enrollment is `NOT_OBSERVED`, so the
  distinction needs the exported bundle, which `docs/DOGFOOD.md` tells the operator to keep outside
  the Git directory before an archive. Non-goals: discovering an archived Git directory,
  aggregate-outcome and sealed-continuation export, a check executable under the source root
  (its relocated path changes the check digest, so that check reads unqualified). Evidence:
  `TestLCPV0883ExportImportRestoresActiveEnrollment` (refusals; the ACTIVE policy restored with
  relocated logs; the owner verifies again; a bundle under a symlinked ancestor imports),
  `TestLCPV0884ImportRefusesPlanNamingSourceRoot`, `TestLCPV0883RestoredHoldStillGuardsCancel`,
  `TestLCPV0885RelocationNeverLaundersAStaleReportSet`,
  and the `LCP-V0-883 restore satisfied` case of `TestLocalCompletionRealEvidenceWorkflow` (a real
  satisfied enrollment imported into a `git worktree add` restore evaluates satisfied there with
  the same target, check and reports). Rollback: delete `internal/localcompletion/export.go` and the
  `export` and `import` subverbs.
- `LCP-V0-960`: (experimental under the standing lane auto-accept, 2026-10-31; V1-3634; refines
  `LCP-V0-884`) A bundle file anchored at the Git directory MUST name a path under `corvint/`,
  the only Git-directory subtree an export carries (the local-completion state, holds and the
  `corvint/` terminal artifacts). Any other `gitdir` path, such as `hooks/pre-commit`,
  `config.worktree` or `info/exclude`, MUST refuse the whole import with
  `import-gitdir-path-refused` before anything is written, even when every digest holds, so a
  crafted bundle can never plant code or configuration Git reads. Evidence:
  `TestLCPV0960ImportRefusesGitDirPathOutsideCorvint` (fails on BASE: the hook was written).
  Rollback: drop the `gitDirExportPrefix` check in `readBundle`.
- `LCP-V0-039`: (experimental under the standing lane auto-accept; V1-2762) Amending
  `LCP-V0-010` and `LCP-V0-013`, prompt context MUST count a token as an anchor only when it can
  name something in this repository's tree, so prose is neither resolved nor missing:
  (1) a URL (`scheme://...`) is removed before any selector is read; (2) an absolute path under
  the index root, or that path without its leading `/` as an over-bound derived query writes it
  (`AHI-016`), is rebased to its repository path, and any other absolute path, a path with an
  empty, `.` or `..` segment, a glob, and a tracked directory are not anchors; (3) a relative
  path with a `/` is an anchor only when its first segment is a tracked top-level entry, so
  `origin/main`, `pass/fail`, `linux/amd64` and `28/28` are prose while `pkg/missing.go` stays
  `anchor-not-found`; (4) a bare token without `/` is a file name only when it is tracked or has
  a non-empty stem and a one-word lower- or upper-case extension that starts with a letter and
  is not a one-letter dotted abbreviation, so `1.64`, `go1.27.3`, `rc.2`, `7.7%` and `e.g` are
  prose, and a tracked base name such as `main.go` stays a file even when a declaration shares
  its extension; (5) a backticked identifier stays explicit and is `anchor-not-found` when
  absent, while an unbackticked qualified name or code-shaped word (`NOT_RUN`, `GitHub`) is an
  anchor only when a declaration carries its name (for a qualified name, its terminal part), and
  this filter runs before the 32-anchor bound of `LCP-V0-035` so prose never spends it;
  (6) an ambiguous path or identifier anchor whose candidates exceed the 128-candidate bound
  beside earlier rows, or a bare base name shared by more than 128 tracked files, counts as
  `ambiguous-anchor` with no rows instead of refusing the whole prompt. A single declaration
  name with more than 128 declarations keeps the `LCP-V0-012` refusal. No packet member is
  added or renamed. Non-goals: suffix or partial-path resolution (`contextindex/x.go`), treating
  a bare domain such as `example.com` as prose (it stays an explicit file name, frozen in
  `mention-cases.json`), resolving another checkout's absolute path against this tree, and
  relaxing the all-anchors-resolve rule of `LCP-V0-010`: a prompt with one ambiguous, unread or
  unqualified anchor stays unresolved. Failure modes: a prompt that relied on an unbackticked
  absent identifier or a foreign absolute path to report `anchor-not-found` now abstains with
  `explicit-task-anchor-required` or resolves on its remaining anchors; a backticked name keeps
  the explicit outcome. Rollback: restore `pathShaped` and the identifier gate in `addAnchors`,
  drop `selector`, `anchorIdentifiers` and `crowded` in
  `internal/contextindex/local_completion_context.go`.


- `LCP-V0-040`: (owner-requested V1-1894 repair, 2026-10-30) A legacy enrolled change
  MAY complete after the required seal without restoring the shared CEM or moving HEAD. This
  amends `LCP-V0-002`, `LCP-V0-003`, `LCP-V0-020` and decision 0534 disposition 2 only for
  the strictly proved seal continuation below; other later commits retain stale completion. The
  continuation MUST derive exactly one parent from the current immutable commit, prove that its
  entire change is deletion of the parent's regular CEM and addition of the identical blob and
  mode at `.corvint/changes/<parent>.cem.json`, and reject merges, descendants, another file or
  mode change, rewritten ancestry, and a present shared working-tree map. The completion
  `target` remains the actual seal; additive `evidenceTarget` names the parent. Public CEM/OCM
  verification MUST retain that parent target and the original base, using the archive bytes.
  The original reviewed report set, its review digest, retained successful observations and
  logs, OCM/intent/map bindings, and parent coordinator/outcome MUST remain verifiable. The
  preserved observations MUST each match the enrollment's retained observation and qualify at
  the parent under the original frozen check policy. They MUST NOT substitute for the latest
  selected checks: those must independently qualify at the actual seal, including after a
  later failed, cancelled, timed-out or screened attempt. A continuation MUST produce and require
  review of a new exact report set naming both commits, the archive and original reviewed set,
  while retaining the original report set verbatim. Recovery MUST execute the real strict
  checker without a cached-pass shortcut; it verifies the parent evidence while rechecking the
  same clean seal before and after. Its comparison is recorded separately as
  `corvint-sealed-check/0`, never by rewriting the parent's coordinator report. The terminal
  binds this comparison plus actual checker exit/logs and the new report set. Status and host
  events validate these inputs read-only; repeated successful finish writes no outcome or state.
  Non-goals: aggregate-profile continuation, automatic reuse of parent checks at a seal, generic
  historical-target execution, missing evidence repair, or new harness authority. The initial
  continuation supports exactly one PRODUCED/none query and impact step and refuses context
  abstention/deadline replay that would otherwise execute against a different HEAD; existing
  stray or malformed abstention evidence still refuses. Missing or drifted prior evidence,
  checks, logs, reports, owner, plan, archive, outcome or current tree cannot publish success.
  The optional private `sealedContinuation` state is additive for this runtime; older runtimes
  cannot read it and must not be used to finish that enrollment. Rollback: revert the repair and
  retain the affected enrollment and sealed tree for a compatible runtime; never delete its
  evidence or reverse the seal to manufacture success.

- `LCP-V0-041`: (V1-3194, 2026-10-30) The supported order is: enrolled strict `finish` satisfied
  at the bind commit, then the one-command lane closeout (`make dogfood-closeout`, which runs
  `dogfood finish-lane` and seals), then the keyed `verify` actions at the seal, `finish`, review
  of the new report set, and `finish` again. The closeout's own `dogfood record` rewrites the
  coordinated `local-outcome.json`, so sealed `finish` MAY replace that one coordination pin only
  when every one of these holds: the `change.ocm-status.json` pin is unchanged; the new outcome
  parses as a `dogfood-record` `recorded` outcome whose `ok`, base, target, tree, candidate set
  and both path-admission digests reproduce from Git at the plan base and the evidence target;
  its trace names the evidence target with outcome `passed`, the same `trace_id` and exactly the
  admitted paths; the target's trace store still holds the enrollment's own row with that same
  admitted set and its newest row is a different-task row byte-equal (as JSON) to the outcome's
  trace; and the coordinator report is complete for the original base and evidence target and
  binds the new outcome digest. The replacement happens only inside `finish` (status and host
  events stay read-only), forces a new continuation report set naming the replaced and
  replacing digests, and requires its review before completion. Every other prior binding,
  report, observation and pin keeps the `LCP-V0-040` rules. Non-goals: accepting a change to
  any other pinned artifact, a hand-written or replayed outcome, or a new state field. Failure
  modes: an OCM status change refuses `foreign-change`; an unparseable, failed or inconsistent
  outcome refuses `replacement-tampered`; another base, target or tree refuses
  `replacement-retargeted`; a missing enrollment or closeout trace row refuses
  `replacement-unrecorded`; a coordinator report that does not bind the new digest refuses
  `replacement-unreferenced`; each refusal writes no state. Rollback: revert the acceptance;
  an enrollment that already re-pinned keeps valid state bytes, and one that did not returns to
  the `LCP-V0-040` refusal with its evidence retained.

- `LCP-V0-042`: (V1-3194, 2026-10-30) Every `sealed-evidence-drift` refusal MUST keep that code
  and carry a bounded detail `artifact=<label> reason=<reason>`, plus `pinned=sha256:<hex>
  current=<sha256:<hex>|absent|unreadable>` when a pinned artifact's bytes changed. Labels are
  fixed names (`local-outcome.json`, `change.ocm-status.json`, `change.cem.json`, `state`,
  `worktree`, `seal-binding`, `prior-report-set`, `prior-binding-NN`, `prior-report-NN`,
  `prior-observation-NN`, `coordination`), never file contents or absolute paths. Reasons are
  likewise fixed: `shared-map-present`, `sealed-state-invalid`, `worktree-dirty`,
  `seal-binding-changed`, `prior-report-set-invalid`, `prior-report-set-digest`,
  `binding-path-changed`, `report-path-changed`, `artifact-changed`, `prior-observation-missing`,
  `prior-observation-unqualified`, `coordination-pins-invalid`, and the `LCP-V0-041` reasons
  `foreign-change`, `replacement-unreadable`, `replacement-tampered`, `replacement-retargeted`,
  `replacement-unrecorded` and `replacement-unreferenced`. The Go
  surface is `SealedDriftError`; the CLI envelope's `detail` field carries the same text and
  the remedy names the supported order. Non-goals: content diffs or automatic repair. Rollback:
  revert to the bare code; no stored state depends on the detail.

- `LCP-V0-043`: (V1-3192, 2026-10-30) When the caller's context ends (a supervised SIGINT or
  SIGTERM, or a caller cancellation) while `finish` runs its final strict check, the result MUST
  be the `finish-interrupted` refusal, not `final-check-failed`. Its envelope detail names the
  step (`final-check` or `aggregate-final-check`), the retained check logs as
  `$GIT_DIR/corvint/local-completion/<key>/<generation>/final-check.{stdout,stderr}`, and the
  rerun command `corvint dogfood finish --session-key <key>`. A genuine checker failure, including
  the step's own deadline, MUST keep `final-check-failed` and name the same retained logs. Neither
  result writes an outcome, terminal or state, so the next `finish` reuses the unchanged bound
  results and must regain success through a real check. Non-goals: resuming the interrupted
  check, a new exit status, or classifying an interruption in another step. Rollback: revert to
  the bare `final-check-failed` code; no stored state depends on the classification.

- `LCP-V0-060`: (V1-3192, 2026-10-30) A real SIGINT or SIGTERM delivered to the
  `corvint dogfood finish` process during its final strict check MUST reach the classification of
  `LCP-V0-043` through the CLI's own signal context: exit 2, exactly one parseable
  `finish-interrupted` refusal on stderr, and on stdout the `ok:false` `dogfood-finish` envelope
  carrying the unsatisfied `active` policy and its next actions, never empty stdout. The strict
  and sealed final checks MUST both return that pre-check policy with the refusal; the aggregate
  final check keeps its stderr-only refusal. The following real `finish` MUST reuse the bound check
  observations unchanged (no check is rerun). Non-goals: a new exit status or a combined
  single-stream envelope. Failure modes: a signal that lands before or after the final check is
  outside this requirement. Rollback: return the zero evaluation again; stderr carries the same
  refusal and no stored state depends on the stdout envelope.
- `LCP-V0-920`: (experimental under the standing lane auto-accept, 2026-11-01;
  ci-keyed-finish-seal-compat; amends the `LCP-V0-040` non-goal on automatic reuse and the keyed
  `verify` step of the `LCP-V0-041` order, for declared checks only) A check whose frozen plan
  declares `allowSealArchiveReuse` (omitted when false, so existing plan digests hold) MUST qualify
  at the actual seal from its latest observation at the evidence target, with no rerun, only when
  the recorded sealed continuation, re-resolved at that commit as the exact one-parent
  unchanged-blob and mode move of the map (`LCP-V0-040`), equals the recorded binding, names the
  observed commit as its evidence target, and that commit still has the observed tree; the usual
  clean, passed, retained-log rules apply to that latest observation, so a later failed attempt or a
  missing log still reruns it. The observation keeps its tested commit, distinct from the seal; the
  new report set, its review and the real final strict check at the seal are unchanged. The
  declaration is the plan author's statement that the check does not read the archive layout; a
  check that does must not declare it. Every other case (no declaration, no recorded continuation,
  another evidence target, a source, map, mode or lineage difference) reruns the check at the seal.
  Non-goals: reinterpreting historical success, reusing an undeclared check, claiming the check ran
  at the seal, a runtime measurement. Evidence: `TestLCPV0920SealArchiveReuseNeedsTheExactSeal`,
  `TestLCPV0920UndeclaredCheckReruns`; one avoided check run per declared check, measured as the
  unchanged observation count. Rollback: drop the field; a plan that declares it then fails to begin
  under strict decoding, and enrollments rerun their checks at the seal.
- `LCP-V0-955`: (accepted under the owner's standing auto-accept (2026-11-01); V1-3542) A prompt or
  session context packet that does not cite its governing instruction file MUST say why in the
  `governance` entry of `coverage.unavailable_selectors`: `reason` `downgraded-authority` with
  `warnings`, the row's `TCP-V0-055`..`057` screen codes in screen order (so a governing file the
  working tree modifies names `self-modified-authority`), or `reason` `unread` when no governing
  candidate could be read. The hook projection carries the entry unchanged. A symlinked governing
  file keeps its `TCP-V0-272`/`TCP-V0-274` `reason` (batch 1101a, 2026-11-01). Rollback: drop the two
  members.

- `LCP-V0-800`: (proposed, not accepted; V1-0320, 2026-11-01) When a prompt carries no
  `LCP-V0-010` anchor, automatic prompt context MAY fall back to the task-context compiler
  (`context --task`) with the prompt text as its subject, and a bare ticket ID (for example
  `V1-0320`) MAY resolve to task evidence through the project's own roadmap rows. The fallback
  stays bounded by `LCP-V0-012`, keeps `explicit-task-anchor-required` visible beside any
  fallback rows, never counts a roadmap row as authority, and abstains when the ID has no row.
  Non-goals: pronoun resolution, transcript reconstruction, a hosted ticket lookup. Failure
  modes: an unpromoted `context --task` (the fallback stays off), an ambiguous ticket ID (named
  `ambiguous`), a roadmap read that fails (named, never silent). Acceptance requires reversing
  the accepted `LCP-V0-010` withholding, so it needs an owner decision and the `context --task`
  promotion first; until then no code implements it. Rollback: delete this entry.
- `LCP-V0-780`: (experimental under the standing lane auto-accept (2026-11-01); V1-4053) The
  strict storage decoder MUST admit an absent member exactly when its Go field carries the struct
  tag `localcompletion:"optional"`, replacing the hand-kept allowlist; every `omitempty` member
  of `Plan` and the completion state MUST carry the tag, so a new optional member cannot be
  refused as missing on a file an older binary wrote. Rollback: restore the allowlist; the
  tags are inert without the decoder check.

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
questions; while another session's enrollment of the worktree is active, that release shows a notice
naming the owner key. A continuation limit releases with an unresolved notice rather than pretending success.
Original pre-change receipts and failed test runs are retained; later coordination receipts cannot
retroactively establish pre-change chronology.
Unassessed rows require reviewer judgment: changed obligations must receive adequate links before
acknowledgment; the local policy cannot infer that semantic distinction from raw unknown counts.
In a repository that does not ignore the daily path's private outputs or its local trace store
(`docs/DOGFOOD.md`, Daily adopter path), the in-process coordination or final check fails and
finish stays unsatisfied (`LCP-V0-014`). An unresolvable running executable is a failed run, not a
fallback to `PATH`.

### Refusal codes

The workflow, its CLI and the automatic event surface refuse with the kebab-case codes below
(decision 0100). Each row cites the first emitting site and states only the condition checked there;
a code emitted at more than one site keeps the meaning of its first. Codes this spec already names
elsewhere are not repeated.

| Code | First emitting site | Condition at the cited site |
|---|---|---|
| `sealed-binding-required` | `internal/dogfoodflow/sealed_check.go` (`ResolveSealBinding`) | The immutable tip is not the exact supported one-parent CEM move, or its proof is unavailable. |
| `sealed-binding-drift` | `internal/dogfoodflow/check.go` (`run`) | The clean seal identity changed during strict checking. |
| `sealed-aggregate-unsupported` | `internal/dogfoodflow/check.go` (`run`); `internal/localcompletion/sealed.go` (`finishSealed`) | Aggregate-profile completion cannot use the legacy sealed continuation. |
| `sealed-context-replay-unsupported` | `internal/dogfoodflow/check.go` (`run`) | Parent query/impact context is not exactly the supported produced-step shape or requires HEAD-dependent replay. |
| `sealed-evidence-drift` | `internal/localcompletion/sealed.go` (`validateSealed`, `acceptCloseoutOutcome`) | The seal, original reviewed evidence, retained checks/logs, or recorded coordinator identity no longer matches, other than the lane closeout's own proved outcome record (`LCP-V0-041`); the detail names the artifact, reason and pinned/current digests (`LCP-V0-042`). |
| `sealed-reviewed-evidence-required` | `internal/localcompletion/sealed.go` (`finishSealed`) | No prior reviewed bind report set is available for continuation. |
| `base-not-ancestor-of-target` | `internal/localcompletion/storage.go` (`requireBaseAncestor`) | Git reports the plan base is not an ancestor of the enrollment-time `HEAD` commit (exit 1, empty stderr) |
| `ancestry-rewritten` | `internal/localcompletion/storage.go` (`requireBaseAncestor`) | the checkout's Git common directory holds `info/grafts`, the checkout is shallow, or that metadata is unreadable, so the base-ancestry probe is not asked (`LCP-V0-036`) |
| `base-unavailable` | `internal/localcompletion/lifecycle.go` (`Begin`) | the plan base does not resolve, or resolves to a different object |
| `cancel-reason-required` | `internal/localcompletion/lifecycle.go` (`Cancel`) | an `active` enrollment is cancelled without a nonempty `--reason`; the message names the selected checks and saved observations it would close (`LCP-V0-038`, V1-2177) |
| `enrollment-preservation-hold` | `internal/localcompletion/lifecycle.go` (`Cancel`), `internal/localcompletion/hold.go` | a coordinator's preservation hold keeps the ACTIVE enrollment; the detail names the distinct-checkout route and the authorized release (`LCP-V0-880`, V1-0581) |
| `import-would-replace`, `import-target-mismatch`, `import-artifact-conflict`, `import-plan-not-relocatable` | `internal/localcompletion/export.go` (`Import`) | a restore never replaces an enrollment, is bound to the exported commit and tree, never overwrites other bytes, and never rewrites a frozen plan that names the source root or Git directory (`LCP-V0-884`, V1-0580) |
| `import-gitdir-path-refused` | `internal/localcompletion/export.go` (`readBundle`) | a bundle names a Git-directory file outside `corvint/`, such as `hooks/pre-commit` (`LCP-V0-960`, V1-3634) |
| `cem-bindings-required` | `internal/localcompletion/finish.go` (`prepareBindings`) | the public `cem status` run against the plan base and current target returned an error |
| `check-executable-unavailable` | `internal/localcompletion/storage.go` (`resolveExecutable`) | `exec.LookPath` cannot resolve a check's executable |
| `completion-evidence-drift` | `internal/localcompletion/finish.go` (`revalidate`) | after finishing, the tree is not clean, the target or tree differs from the pre-finish snapshot, or the saved report is no longer current |
| `dogfood-coordination-failed` | `internal/localcompletion/finish.go` (`coordinate`) | the in-process `dogfood change` coordination run did not pass (`LCP-V0-014`) |
| `dogfood-event-context-drift` | `cmd/corvint/local_completion_event.go` (`localEventContext`) | the loaded index commit or tree revision, or the dirty-path digest, differs from the probed repository context |
| `dogfood-event-deadline` | `cmd/corvint/local_completion_event.go` (`dogfoodExpiryCode`) | the event's context deadline expired or was cancelled, or the Git repository probe's own fixed bound expired first (V1-0396), including during the stdin input read (V1-1391) |
| `dogfood-event-index-snapshot-stale` | `cmd/corvint/local_completion_event.go` (`dogfoodExpiryCode`) | the event's deadline expired after the read found no matching index snapshot and fell back to its in-memory build (`AHI-031`, decision 0400); proposed 2026-09-26, not accepted: also reported before that build when the recorded `index` build cost does not fit the time left (`IDX-SNAP-V0-012`) |
| `dogfood-event-input-unavailable` | `cmd/corvint/local_completion_event.go` (`dogfoodInputFailureCode`) | reading the event input from stdin failed before the event's deadline passed (V1-1391) |
| `dogfood-event-native-budget` | `cmd/corvint/local_completion_event.go` (`localEventContext`) | eight prompt-context attempts, each shrinking the budget, never fit the natively escaped response within the byte budget |
| `dogfood-event-output-too-large` | `cmd/corvint/local_completion_event.go` (`dogfoodEventBytes`) | the canonical response plus a final LF exceeds the byte budget |
| `dogfood-event-output-unavailable` | `cmd/corvint/local_completion_event.go` (`runLocalCompletionEvent`) | writing the encoded response to stdout failed |
| `dogfood-event-policy-drift` | `cmd/corvint/local_completion_event.go` (`localEventRead`) | the evaluation's target is set and differs from the commit probed before the event |
| `dogfood-event-prompt-bounds` | `cmd/corvint/local_completion_event.go` (`dogfoodReadFailureCode`) | the prompt-context compiler refused the event's prompt as `unsupported-dogfood-context-bounds`, `unsupported-dogfood-context-budget` or `invalid-dogfood-context` (`LCP-V0-034`, V1-1976) |
| `dogfood-event-repository-drift` | `cmd/corvint/local_completion_event.go` (`localEventRead`) | the closing observation of the event's one bracket differs from the opening one (`gokernel.ErrRepositoryDrift`), or the commit differs from the expected target |
| `dogfood-event-worktree-churn` | `cmd/corvint/local_completion_event.go` (`dogfoodReadFailureCode`) | the event's repository read was refused as `repository-state-unstable`, `dogfood-event-repository-drift` or `dogfood-event-context-drift`: the worktree moved under the read (`LCP-V0-008`, decision 0533, V1-0880) |
| `dogfood-report-drift` | `internal/localcompletion/finish.go` (`checkCoordinatorContext`) | the dogfood report does not parse, is not complete, or names a base or target other than the plan base and current target |
| `duplicate-local-completion-option` | `cmd/corvint/local_completion.go` (`localCompletionFlags`) | a `local-completion` option is given twice |
| `enrollment-bound-exceeded` | `internal/localcompletion/storage.go` (`load`) | saved state has more than 64 observations, or its intent-pointer or executable count does not match the plan |
| `enrollment-cancelled` | `internal/localcompletion/finish.go` (`finishWithProfile`) | the saved enrollment's lifecycle is `cancelled` |
| `enrollment-drift` | `internal/localcompletion/storage.go` (`load`) | saved state names another session, or its plan digest does not match its plan |
| `enrollment-generation-bound-exceeded` | `internal/localcompletion/lifecycle.go` (`archiveGeneration`) | 16 enrollment generations are already preserved for the session |
| `enrollment-plan-conflict` | `internal/localcompletion/lifecycle.go` (`Begin`) | the session already has an active enrollment for a different plan digest |
| `final-check-failed` | `internal/localcompletion/finish.go` (`finishWithProfile`, `finalCheckError`) | the final in-process `dogfood check` did not pass (`LCP-V0-014`); the detail names the step and its retained logs (`LCP-V0-043`) |
| `finish-interrupted` | `internal/localcompletion/finish.go` (`finalCheckError`) | the caller's context ended during the final strict check; the detail names the step, the retained logs and the rerun command, and no state is written (`LCP-V0-043`) |
| `final-check-not-prerequisite` | `internal/localcompletion/storage.go` (`validatePlan`) | a check argv element is `dogfood-check` or ends in `/dogfood-check.sh`, or the argv contains `dogfood check` or `dogfood seal` (`LCP-V0-015`) |
| `immutable-base-required` | `internal/localcompletion/storage.go` (`validatePlan`) | the plan base is not a Git object id |
| `initial-receipt-secret-screened` | `internal/localcompletion/lifecycle.go` (`Begin`) | a preserved pre-change query or impact receipt matches the secret screen; recorded as the `NOT_PRODUCED` reason in its refusal metadata |
| `input-bound-exceeded` | `internal/localcompletion/storage.go` (`strictJSON`) | strict JSON input is empty or larger than its bound |
| `intent-path-not-found` | `internal/localcompletion/lifecycle.go` (`Begin`) | an intent path is absent from the tree of the current target commit |
| `invalid-intent`, `invalid-requirements-section`, `missing-requirements`, `duplicate-requirement`, `too-many-obligations` | `internal/localcompletion/lifecycle.go` (`Begin`, via `lrfrepo.IntentRequirementsRefusal`) | an intent's numbered Requirements section would be refused by finish's `ocm prepare` (`LCP-V0-037`, V1-0680) |
| `repository-object-unavailable` | `internal/localcompletion/lifecycle.go` (`Begin`) | an intent path names a tree or a blob over 4 MiB (`LCP-V0-037`) |
| `invalid-check-argv` | `internal/localcompletion/storage.go` (`validatePlan`) | a check's first argv element is empty, or any element exceeds 4096 bytes or contains NUL |
| `invalid-check-bound` | `internal/localcompletion/storage.go` (`validatePlan`) | a check has fewer than 1 or more than 64 argv elements, or a timeout outside 1..3600 seconds; the message names the bounds and the requested values (`LCP-V0-032`) |
| `invalid-check-id` | `internal/localcompletion/storage.go` (`validatePlan`) | a check id does not match `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$` or repeats |
| `invalid-dogfood-context` | `internal/contextindex/local_completion_context.go` (`validateLocalPrompt`) | prompt context was requested without an index at an immutable revision |
| `invalid-dogfood-event-arguments` | `cmd/corvint/local_completion_event.go` (`runLocalCompletionEvent`) | an event option is repeated |
| `invalid-dogfood-event-budget` | `cmd/corvint/local_completion_event.go` (`runLocalCompletionEvent`) | the requested byte budget exceeds the event maximum |
| `invalid-dogfood-event-input` | `cmd/corvint/local_completion_event.go` (`dogfoodEventInput`) | the event input exceeds the kernel input bound or is not valid UTF-8 |
| `invalid-enrollment-generation` | `internal/localcompletion/storage.go` (`load`) | the saved generation is not 68 bytes prefixed by the plan digest and `-` |
| `invalid-enrollment-pointer` | `internal/localcompletion/storage.go` (`load`) | a saved intent pointer names another path than its plan intent, or a revision or blob hash that is not a Git object id |
| `invalid-execution-path` | `internal/localcompletion/storage.go` (`load`) | a saved executable path is not absolute, not clean, or longer than 4096 bytes |
| `invalid-intent-scope` | `internal/localcompletion/storage.go` (`validatePlan`) | a plan intent is not a valid path or is not strictly after the previous intent; `error.message` names the sorted, de-duplicated 1-16 path rule (`LCP-V0-018`) |
| `invalid-lifecycle` | `internal/localcompletion/storage.go` (`load`) | the saved lifecycle is not `active`, `satisfied` or `cancelled` |
| `invalid-local-completion-action` | `cmd/corvint/local_completion.go` (`localCompletionFlags`) | the action is not `begin`, `verify`, `review`, `status`, `finish`, `cancel` or `handoff` |
| `invalid-local-completion-json` | `internal/localcompletion/storage.go` (`strictJSON`) | strict JSON input does not parse |
| `invalid-local-completion-option` | `cmd/corvint/local_completion.go` (`localCompletionFlags`) | an option is not allowed for the action, or the mutually exclusive `--anchors` and `--receipt` are both given |
| `invalid-local-completion-option-value` | `cmd/corvint/local_completion.go` (`localCompletionFlags`) | an option value is empty or longer than 4096 bytes |
| `invalid-local-completion-schema` | `internal/localcompletion/storage.go` (`strictJSON`) | strict JSON input parsed and passed the JSON type check, but decoding into the target type with unknown fields disallowed failed; the JSON type check emits the same code at `internal/localcompletion/storage.go` (`validateJSONTypes`), and a required-field read of input that is not an object at `internal/localcompletion/storage.go` (`requireFields`) |
| `invalid-local-state-directory` | `internal/localcompletion/lifecycle.go` (`generationCount`) | the session's generation path exists and is not a directory |
| `invalid-public-evidence-result` | `internal/localcompletion/finish.go` (`public`) | a public evidence command exited zero but its stdout is not JSON with `ok: true` |
| `invalid-report-set-digest` | `internal/localcompletion/lifecycle.go` (`Review`) | the review's report-set digest is not 64 lowercase hex |
| `invalid-review-digest` | `internal/localcompletion/storage.go` (`load`) | a saved review digest is present and not 64 lowercase hex |
| `invalid-session-key` | `internal/localcompletion/storage.go` (`open`) | the session key is not 64 lowercase hex |
| `invalid-tree-listing` | `internal/localcompletion/storage.go` (`snapshotWithContent`) | a tree-listing row has no tab separator |
| `invalid-verification-exit` | `internal/localcompletion/storage.go` (`load`) | a saved exit is not the canonical decimal of an integer in -1..255 |
| `invalid-verification-observation` | `internal/localcompletion/storage.go` (`load`) | a saved observation's log paths are not the check's numbered logs, or its target, tree, check digest or content digest is malformed |
| `invalid-worktree-owner` | `internal/localcompletion/storage.go` (`owner`) | the worktree owner file does not hold a 64-hex key |
| `local-completion-action-required` | `cmd/corvint/local_completion.go` (`runLocalCompletion`) | `local-completion` is given no action argument |
| `local-completion-failed` | `cmd/corvint/local_completion.go` (`emitLocalCompletionFailure`) | the failure code to emit contains a character other than `a-z` or `-`, is empty, or is longer than 96 bytes, so it is replaced |
| `local-completion-option-required` | `cmd/corvint/local_completion.go` (`localCompletionFlags`) | the action's required option is missing |
| `local-completion-option-value-required` | `cmd/corvint/local_completion.go` (`localCompletionFlags`) | a non-inline option is the last argument, or its next token is option-like (`GPK-V0-064`, decision 0196) |
| `local-outcome-evidence-drift` | `internal/localcompletion/finish.go` (`checkCoordinatorContext`) | the local outcome artifact is unreadable or its digest differs from the report's |
| `local-state-bound-exceeded` | `internal/localcompletion/storage.go` (`readFile`) | a local state file is larger than its read bound |
| `local-state-not-regular` | `internal/localcompletion/storage.go` (`readFile`) | a local state file is not a regular file |
| `local-state-symlink` | `internal/localcompletion/storage.go` (`checkParents`) | a local state path or one of its parents is a symlink |
| `log-secret-screened` | `internal/localcompletion/finish.go` (`saveProcess`) | a process's stdout or stderr matches the secret screen |
| `missing-local-completion-field` | `internal/localcompletion/storage.go` (`requireFields`) | a required field is absent from the JSON input |
| `ocm-bindings-required` | `internal/localcompletion/finish.go` (`prepareBindings`) | the dogfood OCM aggregate status is not OK |
| `operation-in-progress` | `internal/localcompletion/storage.go` (`lock`) | the final operation lock path already exists |
| `operation-lock-permission-denied` | `internal/localcompletion/storage.go` (`operationLockFailure`) | permission denied creating the lock or its parent; underlying cause retained |
| `operation-lock-create-failed` | `internal/localcompletion/storage.go` (`operationLockFailure`) | another lock creation failure; underlying cause retained |
| `output-bound-exceeded` | `internal/localcompletion/finish.go` (`Write`) | a bounded output buffer would exceed the artifact byte bound |
| `plan-bound-exceeded` | `internal/localcompletion/storage.go` (`validatePlan`) | the plan has fewer than 1 or more than 16 intents or checks; `error.message` names both bounds (`LCP-V0-018`) |
| `plan-unavailable` | `internal/localcompletion/storage.go` (`ReadPlan`) | the plan file's parent directory does not resolve |
| `prior-completion-stale` | `internal/localcompletion/lifecycle.go` (`Begin`) | the session's satisfied enrollment for a different plan no longer evaluates as satisfied; the CLI message names the staged-binding route of a fresh worktree or clone (V1-0953) |
| `public-command-output-bound` | `internal/localcompletion/finish.go` (`public`) | a public evidence command overflowed its stdout or stderr bound |
| `public-evidence-command-failed` | `internal/localcompletion/finish.go` (`public`) | a public evidence command exited non-zero without a valid code on stderr |
| `public-report-path-invalid` | `internal/localcompletion/finish.go` (`generateReports`) | the report path is not an allowed path |
| `public-report-path-missing` | `internal/localcompletion/finish.go` (`generateReports`) | the result has no string `report` field |
| `report-set-stale` | `internal/localcompletion/lifecycle.go` (`Review`) | at review, the tree is not clean, the saved report is not current, or the report-set digest differs |
| `repository-identity-changed` | `internal/localcompletion/storage.go` (`refreshAuthority`) | the repository root, Git directory or common directory changed since open |
| `repository-snapshot-drift` | `internal/localcompletion/lifecycle.go` (`evaluateObserved`) | the target, tree or cleanliness changed during evaluation |
| `repository-unavailable` | `internal/localcompletion/storage.go` (`open`) | the Git authority for the repository root cannot be opened; the session key was already checked |
| `secret-shaped-plan` | `internal/localcompletion/storage.go` (`validatePlan`) | a plan intent matches the secret screen |
| `selected-check-unverified` | `internal/localcompletion/finish.go` (`revalidate`) | a plan check has no qualifying observation for the current snapshot |
| `session-identity-ambiguous` | `internal/localcompletion/types.go` (`SessionKey`) | no explicit session key, and both a Codex identity variable and a Claude Code session identity are set (`LCP-V0-810`) |
| `session-identity-required` | `internal/localcompletion/types.go` (`SessionKey`) | no explicit session key, neither `CODEX_THREAD_ID` nor `CODEX_SESSION_ID` is set, and no Claude Code session identity (`CLAUDECODE=1` with `CLAUDE_CODE_SESSION_ID`) is set (`LCP-V0-810`) |
| `uncommitted-work` | `internal/localcompletion/lifecycle.go` (`Verify`) | the tree is not clean before verification |
| `unknown-selected-check` | `internal/localcompletion/lifecycle.go` (`Verify`) | the selected check id is not in the plan |
| `verification-attempt-bound-exceeded` | `internal/localcompletion/lifecycle.go` | Verify refuses at 64 saved observations; evaluation also exposes this unmet reason when any selected check remains unqualified |
| `verification-cancelled` | `internal/localcompletion/lifecycle.go` (`Verify`) | the context was cancelled after the observation was saved |
| `verifier-disagreement` | `internal/localcompletion/finish.go` (`checkCoordinatorContext`) | on the final read, the report's dogfood check outputs do not agree |
| `worktree-already-enrolled` | `internal/localcompletion/lifecycle.go` (`Begin`) | another session holds an active enrollment for the worktree |
| `worktree-owner-mismatch` | `internal/localcompletion/storage.go` (`requireOwner`) | the worktree owner is not this session |
| `worktree-prior-completion-stale` | `internal/localcompletion/lifecycle.go` (`Begin`) | another session's satisfied enrollment of the worktree no longer evaluates as satisfied |

### Resolution and Stop decision reasons

Prompt-context resolution and the Stop decision carry the kebab-case reasons below (decision 0100).
Each row cites the first emitting site and states only the condition checked there.

| Code | First emitting site | Condition at the cited site |
|---|---|---|
| `ambiguous-anchor` | `internal/contextindex/local_completion_context.go` (`resolution`) | the resolution `reason` when no earlier case applies and an anchor matched more than one candidate |
| `anchor-evidence-unavailable` | `internal/contextindex/local_completion_context.go` (`resolution`) | the resolution `reason` when anchors exist and a task-evidence candidate was unreadable or requirement definitions were capped |
| `anchor-not-found` | `internal/contextindex/local_completion_context.go` (`resolution`) | the resolution `reason` when no earlier case applies and an anchor matched no candidate |
| `anchor-worktree-changed` | `internal/contextindex/local_completion_context.go` (`resolution`) | the resolution `reason` when no earlier case applies and a task-evidence path is among the index's dirty paths |
| `local-policy-continuation-limit` | `cmd/corvint/local_completion_event.go` (`dogfoodCompletion`) | a `stop` event that would block has `stopHookActive` true; decision `release` |
| `local-policy-incomplete` | `cmd/corvint/local_completion_event.go` (`dogfoodCompletion`) | a `stop` event whose lifecycle is `active`, or `satisfied` without the evaluation satisfied; decision `block` |
| `local-policy-other-session-active` | `cmd/corvint/local_completion_event.go` (`withOtherSessionOwner`) | a `stop` event whose inactive evaluation names another session's active enrollment as `owner`; decision `release`, and the completion carries `owner` |

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
| LCP-V0-002 | `TestOperationLockFilesystemErrors`; `TestOperationLockFailureCauses`; `TestEnrollmentAndReadOnlyPolicy`; `TestLocalStateBoundsAndContention`; `TestDogfoodEventGoPythonWireAndSession`; `TestEnrollmentRefusesIntentAbsentFromBase`; `TestRefusedEnrollmentConsumesNoGeneration`; `TestEnrollmentPinsSameChangeIntentAtHead`; `TestEnrollmentRefusesHeadNotDescendedFromBase`; `TestFinishRefusesHeadMovedToUnrelatedHistory` |
| LCP-V0-003 | `TestEnrollmentAndReadOnlyPolicy`; `TestInactiveKeyNamesActiveWorktreeOwner`; `TestCompletionNextActions`; `TestFinishRefusesHeadMovedToUnrelatedHistory`; `TestLocalCompletionRealEvidenceWorkflow`; `TestDogfoodEventReadOnlyEnrolledStopAndPrompt` |
| LCP-V0-004 | `TestActualVerificationAndSecretRefusal`; `TestVerificationCancellationCleansDescendant`; `TestCEMV1150StableSidecarGrantsNoReuse` (the 0.x canonical-map sidecar reuse test is retired with the 0.x map under `CEM-V1-210`) |
| LCP-V0-005 | `TestLocalCompletionRealEvidenceWorkflow`; actual CEM/OCM/coordinator/checker fixture |
| LCP-V0-006 | `TestLocalCompletionRealEvidenceWorkflow`; generated-but-unread/stale report refusals |
| LCP-V0-007 | `TestLocalCompletionRealEvidenceWorkflow`; strict-check and repeated-finish assertions |
| LCP-V0-008 | `TestDogfoodEventStopLifecycle`; `TestDogfoodEventReadOnlyEnrolledStopAndPrompt` other-session release and notice; `TestQualifiedLifecycleStopComposition` closed completion; `TestDogfoodEventStrictInputAndDeadline` deadline code, uncancellable-read expiry and input-read expiry (V1-1391); `TestV10880WorktreeChurnHasNamedCode` and `TestV10880AdapterReasonNamesChurnAndKeepsUnavailable` (`cmd/corvint/worktree_churn_test.go`) churn, policy-drift and unlisted-read codes (decision 0533); native first/recursive Stop regressions, with the Git probe bound lifted beside the event deadline (`gokernel.WithProbeBound`, V1-1391) |
| LCP-V0-009 | `TestDogfoodEventGoPythonWireAndSession`; `TestDogfoodEventStrictInputAndDeadline`; `TestAdapterRejectedReasonSurfacesEngineErrorCode`; `TestRejectedEventReasonAppendsEngineCode`; `TestAdapterErrorTailIsBounded` |
| LCP-V0-010 | `TestDogfoodPromptFrozenAnchors`; `TestDogfoodPromptScopeDoesNotResolveAndPreservesStaleness`; frozen follow-up and explicit/unknown/ambiguous anchors |
| LCP-V0-011 | `TestDogfoodPromptPrivacyNoHistoryAndNonmutation`; `TestDogfoodPromptCriticalBudgetAndImpossibleEnvelope`; accepted adapter amendment (decision 0441): `TestClaudeNativeDogfoodLifecycle` (AHI-046 and AHI-047 subtests), `TestAHI047GuidanceIsMainThreadSessionStartOnly` |
| LCP-V0-012 | `TestLocalStateBoundsAndContention`; `TestVerificationCancellationCleansDescendant`; `TestDogfoodPromptBoundsAndCancellation` |
| LCP-V0-013 | `TestDogfoodPromptMentionAnchors` (frozen `mention-cases.json`); `TestDogfoodPromptMentionIdentityAndRefusals`; `TestUseCaseHostileTaskOrientation` prompt-mention cases |
| LCP-V0-014 | `TestDogfoodFinishRunsFromBinaryInForeignRepository` (built binary, non-Corvint repository with no `script/`, `VERSION` or `cmd/corvint`, poisoned `DOGFOOD_*`, `CORVINT_BIN` and `PATH`); `TestLocalCompletionRealEvidenceWorkflow` (in-tree) |
| LCP-V0-015 | `storage.go` `runsDogfoodCheck` guard; `final-check-not-prerequisite` refusal row; `TestValidatePlanRefusesFinalCheckInAnyForm` (script name, make target, `dogfood check` and `dogfood seal` refused; `dogfood change` and a non-adjacent pair admitted) |
| LCP-V0-016 | `localEventRead` in `cmd/corvint/local_completion_event.go` running its reads inside `gokernel.ProbeRepositoryAround` (`GPK-V0-076`), and `localEventContext` loading the snapshot through `contextindex.LoadSnapshotObserved` against the opening observation | `TestDogfoodEventSpawnsOneBracket` (`cmd/corvint/dogfood_event_bracket_test.go`), which counts four Git spawns through a PATH shim for `stop`, `user-prompt` and both `session-start` sources over clean and dirty snapshot-present fixtures; `TestDogfoodEventRefusesRepositoryDriftInsideTheBracket`, which writes a file during the miss build and pins the `dogfood-event-repository-drift` error and the native surface's `dogfood-event-worktree-churn` refusal (decision 0533); `TestProbeAroundSpawnsFourGitProcessesAndRefusesDrift` (`internal/gokernel/repository_around_test.go`); the before/after hook timings in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-06-v1-0881-single-event-bracket.md` |
| LCP-V0-017 | `localCompletionExit` and the shared envelope in `cmd/corvint/local_completion.go`; `TestLocalCompletionVerifyOKMirrorsCheckResult` (a selected check exiting 1 yields exit 1, `ok:false`, `qualified:false`, `exit:1`; a passing check yields exit 0 and `ok:true`); `TestDogfoodFinishRunsFromBinaryInForeignRepository` (pre-review finish exit 1 with `ok:false`, satisfied finish `ok:true`); the before/after envelopes in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-08-v1-1012-verify-ok-mirrors-exit.md` |
| LCP-V0-020 | `TestCompletionNoActionReasons`; `TestLocalCompletionFrontierUnavailableNextActionOrReason`; `TestLocalCompletionUnavailableEvaluationReason`; `TestLocalCompletionRealEvidenceWorkflow` (satisfied status and finish JSON) |
| LCP-V0-018 | `localCompletionRemedies` in `cmd/corvint/local_completion.go`; `TestDogfoodBeginPlanIntentRefusalNamesTheRule` (`cmd/corvint/local_completion_test.go`) over unsorted, duplicated, empty and 17-path intents arrays |
| LCP-V0-019 | `EvaluateObserved`, `EvaluateAgainst` and drift-only snapshots in `internal/localcompletion/lifecycle.go` and `internal/localcompletion/storage.go`; matched closing observation in `localEventRead` | `TestEnrolledStopSpawnsBoundedGit`, `TestDogfoodEventObservedMismatchFallsBack`, `TestEvaluateAgainstMatchesFullEvaluation`, `TestEvaluateAgainstReReadsPrivateInputs`, `TestEvaluateAgainstDirtyAndDrift`, `TestEvaluateAgainstChangedBase`; trace and same-host measurement record in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-10-v1-1124-stop-git-calls.md` |
| LCP-V0-031 | `screenedLog` and `failedTestNames` in `internal/localcompletion/screened_log.go`, applied in `Verify`; `TestScreenedCheckLogKeepsFailureLines` (screened FAIL line kept with the token redacted and named in `failedTests`; an unscreened failure names its subtests; a passing check names none; truncated and complete PEM and PGP private-key blocks keep only the marker; the over-bound and name bounds); `TestActualVerificationAndSecretRefusal` (no secret persists) |
| LCP-V0-032 | `CheckBoundError` in `internal/localcompletion/types.go`, raised by `validatePlan`; `emitLocalCompletionFailureDetail` in `cmd/corvint/local_completion.go`; `TestDogfoodBeginCheckBoundNamesTheLimit` (`cmd/corvint/local_completion_test.go`: a 4000 s timeout and 65 argv elements; a secret-shaped check id is shown as `[REDACTED]`) |
| LCP-V0-033 | `snapshotPrefetch` and `dogfoodEvent` in `cmd/corvint/local_completion_event.go`, consumed by `localEventContext`; `gokernel.ProbeRepositoryAroundPrefetching` (`internal/gokernel/repository.go`); `adoptObservation` (cmd, so the analyzer-schema inputs are unchanged) | `TestDogfoodEventPrefetchesTheSnapshotFromTheOpeningIdentity`, `TestSnapshotPrefetchServesOnlyItsOwnTree` and `TestAdoptObservationMatchesTheObservedLoader` (`cmd/corvint/dogfood_event_bracket_test.go`: one identity-only load, answer bytes equal to the sequential read, other-tree and unstarted fallback, adoption equal to `contextindex.LoadSnapshotObserved`'s); `TestProbeAroundPrefetchingOverlapsTheOpeningStatusRead` (`internal/gokernel/repository_around_test.go`: the status reply blocks until prefetch receives the identity, four Git calls, no closing prefetch, none after a failed identity read); `TestDogfoodEventSpawnsOneBracket` still counts four spawns; the A/B timings in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-hooks-1021-snapshot-prefetch.md` |
| LCP-V0-034 | `dogfoodReadFailureCode` in `cmd/corvint/local_completion_event.go`; `admittedAdapterRejections` in `internal/observations/observations.go` | `TestV11976PromptContractRefusalHasNamedCode` (`cmd/corvint/worktree_churn_test.go`: bounds, wrapped budget and invalid-context refusals report `dogfood-event-prompt-bounds`, another `contextindex` code stays `dogfood-event-unavailable`); `TestAdapterDegradationAdmitsPromptBounds` (`internal/observations/observations_test.go`); both fail without the change |
| LCP-V0-035 | `keepAnchors` and `resolution` in `internal/contextindex/local_completion_context.go` | `TestDogfoodPromptLCPV0035KeepsFirstAnchorsInOrder` (`internal/contextindex/local_completion_context_test.go`: 2 paths and 35 identifiers keep 32 anchors and drop 5 identifiers, the packet holds no dropped term, a reversed prompt gives the same packet, 33 paths drop one path); `TestDogfoodPromptEarlyResourceBounds` keeps the 128-candidate refusal |
| LCP-V0-036 | `requireBaseAncestor` in `internal/localcompletion/storage.go` calling `gitancestry.RewrittenAt`; `TestRewrittenAtMatchesRewrittenWithoutGit_LCPV0036` (`internal/gitancestry`: same reason as the Git-asking guard for plain, shallow and grafted checkouts); `TestEvaluateAgainstChangedBase` still counts four Git calls; `TestAncestryRewrittenCheckoutRefusesBaseProbe` (`internal/localcompletion/lifecycle_test.go`: `info/grafts` and `.git/shallow` fixtures; `begin` refuses with no `state.json`, and after a valid enrollment status and finish refuse with the state bytes unchanged) |
| LCP-V0-037 | `Begin` in `internal/localcompletion/lifecycle.go`; `IntentRequirementsRefusal` in `internal/lrfrepo/ocm.go` | `TestEnrollmentRefusesIntentWithoutNumberedRequirements_LCPV0037` (`internal/localcompletion/lifecycle_test.go`: no section, no IDs and a duplicate ID are refused with the `ocm prepare` code 16 times without state.json, then a valid plan enrolls; fails without the change) |
| LCP-V0-038 | `Cancel` in `internal/localcompletion/lifecycle.go`; `CancelReasonError` in `internal/localcompletion/types.go`; `--reason` in `cmd/corvint/local_completion.go` | `TestCancelActiveEnrollmentRequiresReason` (`internal/localcompletion/lifecycle_test.go`: a bare cancel of an active enrollment refuses with the check and observation counts and unchanged state, a reasoned cancel closes it, a repeat bare cancel succeeds; fails without the check); `TestDogfoodCancelActiveEnrollmentRequiresReason` (`cmd/corvint/local_completion_test.go`: exit 2 envelope names `--reason TEXT` and the fresh-worktree route, status stays active, `cancel --reason` echoes `cancelReason`); V1-0953 staged route: `TestStagedBeginAfterSatisfiedStageRefusesAndKeepsIt` (`internal/localcompletion/lifecycle_test.go`: a stage-2 begin after a satisfied stage refuses `prior-completion-stale` and keeps the stage-1 state bytes) and `TestDogfoodPriorCompletionStaleNamesStagedRoute` |
| LCP-V0-810 | `SessionKey` in `internal/localcompletion/types.go`; the `session-identity-*` remedies in `cmd/corvint/local_completion.go` | `TestLCPV0810SessionKeyAdmitsClaudeCode`, `TestLCPV0810SessionRefusalsNameSessionKey` |
| LCP-V0-880..882 | `Hold`, `HoldError` in `internal/localcompletion/hold.go`; the hold check in `Cancel`; the `hold` subverb in `cmd/corvint/local_completion.go` | `TestLCPV0880PreservationHoldGuardsCancel` (`internal/localcompletion/hold_test.go`) |
| LCP-V0-883..885 | `Export`, `Import`, `relocateState` in `internal/localcompletion/export.go`; the `export` and `import` subverbs in `cmd/corvint/local_completion.go` | `TestLCPV0883ExportImportRestoresActiveEnrollment`, `TestLCPV0884ImportRefusesPlanNamingSourceRoot`, `TestLCPV0883RestoredHoldStillGuardsCancel`, `TestLCPV0885RelocationNeverLaundersAStaleReportSet` (`internal/localcompletion/export_test.go`); `TestLocalCompletionRealEvidenceWorkflow/LCP-V0-883_restore_satisfied` (`cmd/corvint/local_completion_test.go`) |
| LCP-V0-039 | `selector`, `explicitPaths`, `anchorIdentifiers`, `crowded` and `pathCandidates` in `internal/contextindex/local_completion_context.go` | `TestDogfoodPromptLCPV0039ProseIsNotAnAnchor` (`internal/contextindex/local_completion_context_test.go`: a path among URL, number, version, slash-prose, markup and status-word noise resolves alone; absolute and slash-stripped root paths rebase; prose-only prompts abstain; backticked, missing-path and qualified cases keep their outcome; 129 `main.go` files and two crowded base names count ambiguous without refusing); frozen `TestDogfoodPromptFrozenAnchors`, `TestDogfoodPromptMentionAnchors` and `TestDogfoodPromptLCPV0035KeepsFirstAnchorsInOrder` unchanged; end to end through the Claude Code hook, `TestV12762CorvintShapedPromptEnvelopesResolveOrAbstain` (`cmd/corvint/host_adapter_prompt_resolution_test.go`: a path or requirement ID among URL, version, percentage and slash-prose noise yields its one row with no `resolution` member, and anchorless prompts are silent) |

| LCP-V0-040 | `TestLocalCompletionRealEvidenceWorkflow` sealed completion and positive-first drift/checker controls; `TestLocalCompletionSealedBinding` exact rename, source/map/name/mode/symlink/multi-parent/descendant controls; `TestLocalCompletionSealedReportCancellation` caller cancellation reaches canonical-fixture seal validation |
| LCP-V0-041 | `TestLocalCompletionSealedCloseoutRepin` (`cmd/corvint/local_completion_test.go`: enrolled strict finish, the real `dogfood finish-lane` closeout, verify, then sealed finish produces a report set naming both outcome digests, review, and satisfied at the seal with the bind as evidence target and no manual edits; positive-first controls refuse tampered, retargeted, unreferenced and unrecorded replacements and a foreign OCM status change with unchanged state, and the restored evidence completes) |
| LCP-V0-042 | `TestLocalCompletionSealedCloseoutRepin` (each control asserts `SealedDriftError` artifact, reason and pinned/current digests; the CLI `dogfood finish` envelope carries the code and detail); `TestLocalCompletionRealEvidenceWorkflow` existing drift controls keep `sealed-evidence-drift` |
| LCP-V0-043 | `TestLocalCompletionRealEvidenceWorkflow` (`cmd/corvint/local_completion_test.go`: a caller cancellation during the final strict check returns `finish-interrupted` with step `final-check`, both retained logs on disk, the rerun command and byte-identical state; a genuine checker failure returns `final-check-failed` naming the logs; the following real finish is satisfied without new verification); `TestTransportAdaptedRecoveryNative` keeps `final-check-failed` |
| LCP-V0-060 | `TestLocalCompletionRealEvidenceWorkflow` via `assertFinishSignalCLI` (`cmd/corvint/local_completion_signal_unix_test.go`: the test binary re-executes the real `--root R dogfood finish --session-key K` argv through `runProfiledContext` under `signal.NotifyContext(terminationSignals())`, holds only the final strict check open, delivers a real SIGINT and then SIGTERM, and parses exit 2, the stderr `finish-interrupted` refusal naming step, both retained log paths on disk and the rerun, the stdout `ok:false` active policy envelope, and byte-identical state); the following real finish is satisfied with the check observations unchanged |
| LCP-V0-920 | `TestLCPV0920SealArchiveReuseNeedsTheExactSeal` (`internal/localcompletion/sealed_archive_reuse_test.go`: a declared check qualifies at its exact seal from its one bind observation, the observation count and tested commit unchanged; positive-first controls refuse an undeclared check, a missing continuation, another evidence target, a later failed observation, a missing log (restored, it qualifies again) and a source commit after the seal; a false declaration is absent from the encoded plan); `TestLCPV0920UndeclaredCheckReruns` |
| LCP-V0-955 | `TestLocalPromptDowngradedGovernanceNamesItsReason` (`internal/contextindex/local_completion_context_test.go`: a modified `AGENTS.md` yields `downgraded-authority` with `self-modified-authority`) |
| LCP-V0-800 | none (proposed; implementation waits for acceptance and the `context --task` promotion) | NOT_PRODUCED: a test that a natural-language prompt and a ticket ID each get task evidence follows acceptance |
| LCP-V0-780 | `TestLCPV0780OptionalTagAdmitsAnAbsentMember`, `TestLCPV0780OmitemptyFieldsAreTaggedOptional` (`internal/localcompletion/optional_tag_test.go`: a tagged absent member decodes; every `omitempty` field of `Plan` and the state is tagged) |

## Rollout, rollback and remaining gates

Record decision 0009 option 2; independently review the plan; implement and test in an isolated
worktree. Run the canonical gate and relevant frozen regressions, inspect the new workflow's reports,
then complete the existing post-commit dogfood loop against the original base. Rebuild the native
binary and refresh the Codex plugin. Preserve verified compatibility files for active tasks whose
cached path an install removes. Verify current hook discovery/trust and real installed events.
Rollback disables/cancels the explicit enrollment or restores the legacy adapter; legacy evidence
formats and commands remain usable. No claim of fully qualified native-platform support follows.
Rolling back `LCP-V0-014` restores the `runScript` path of `internal/localcompletion/finish.go`
at `e667812`, which runs `script/dogfood-change.sh` and `script/dogfood-check.sh` through `bash`
and therefore works only inside Corvint's own tree; enrollment state and report formats are
unchanged either way.

Cross-host executable witnesses: `TestClaudeNativeDogfoodLifecycle` invokes the actual Python
adapter and production Go CLI for identity/handoff, first/recursive Stop, governed context,
startup sources, read-only state and closed host admission. `tests/test_local_completion_claude.py`
and the existing adapter suites cover exact golden domains, all six stdio routes, resealed
provenance negatives, complete-frame bounds and interruption. These are adapter/core fixtures,
not observations of a real Claude/Codex model-host lifecycle.


## Proposed aggregate outcome extension

[Aggregate Local Outcome V0](aggregate-local-outcome-v0.md) owns the experimental, explicit
aggregate selector, nonlearning receipt, `/1` report and additive pending state discriminator.
Its source candidate is **NOT_QUALIFIED**. The default `/0` workflow and 200-path learning limit
remain governed by this existing contract. Aggregate acceptance requires the extension's native
qualification, independent review and frozen terminal gates; a source patch or focused test pass
does not satisfy those requirements. The held recovery entrypoint is separately pending.
