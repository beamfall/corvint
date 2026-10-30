# Dogfood Query Abstention V0

Owner: Russell Lewis
Date: 2026-09-30
Requirement prefix: `QAT-V0`
Intent status: accepted (decision 0450; V1-0523)
Delivery status: experimental
Authoritative inputs: owner request to merge the six experimental workflows (2026-09-30); `AGENTS.md` invariants 1, 2 and 4; `docs/DOGFOOD.md`; `daily-change-evidence-workflow-v0.md` DCW-V0-003/005/015/025/026; `go-production-kernel-migration-v0.md` GPK-V0-028.

## Agent digest
- Claim: An exact intentional authority-start trace refusal can be retained during structural dogfood closure without claiming query support.
- Status: accepted (decision 0450; V1-0523)/experimental; no release or query capability promotion.
- Exists: `internal/dogfoodflow/query_abstention.go` and focused hostile binding/replay tests.
- Blocked on: independent review and actual final original-task CLI qualification; general usefulness remains unqualified.
- Read next: Requirements; Evidence contract; Acceptance evidence and rollback.

## User and governing intent

The owner requested that six experimental workflows be merged. Their original unchanged task
selects authority-start on a clean tree containing a previously retained local outcome trace.
That intentional refusal is supported behavior under GPK-V0-028, while changing task wording or
removing the trace would conceal evidence. Existing DCW-V0-025 supplies a precedent for retaining
an unsupported discovery result during structural closure, but accepts only impact abstentions.
This separate contract (accepted by decision 0450) explicitly amends daily completion for one query refusal. It does
not retroactively claim that decision 0388 accepted a query exception.

## Requirements

- `QAT-V0-001`: The producer MUST qualify only exit 2, empty raw stdout and exactly the canonical single-line stderr envelope below. It MUST preserve the original task and earlier traces. Every other error, including unreachable or corrupt trace state with the same code, extra stderr, malformed envelopes, nonempty stdout and other exit statuses MUST remain blocking.
- `QAT-V0-002`: The producer MUST retain the exact NUL-terminated invocation, raw output files and a private canonical `corvint-dogfood-query-abstention/0` artifact binding argv/task SHA-256, immutable base/target, exit status and raw output SHA-256. Only a successfully written artifact may exempt the single exact query row. Every run MUST clear its previous query artifact and digest before success, a different failure or another abstention can replace it; failure to clear or write MUST block.
- `QAT-V0-003`: The report MUST keep the query row `NOT_PRODUCED` with reason `authority-start-trace-state-abstention` and optional `queryAbstentionEvidenceSha256`. The strict checker MUST reject absent, duplicate, extra, malformed, stale or tampered artifact members/bindings, duplicate report query rows/digest members, the reason on another step, missing artifacts and invalid argv/root/task/limit/NUL shape. An absent digest member in an older non-abstention report or a null member with no artifact MUST remain compatible.
- `QAT-V0-004`: The checker MUST reconstruct only `query --task ORIGINAL_TASK --limit 1` using independently selected base/current verifiers and any override, never the recorded executable or arbitrary argv. All selected verifiers MUST agree on exit 2 and the exact retained stdout/stderr bytes. Agreement on success or another failure MUST NOT qualify. Replay and receipt binding MUST both succeed.
- `QAT-V0-005`: This exception MUST NOT bypass clean-target validation, CEM/OCM policy, selected checks, independent review or local-outcome admission. Failed local-outcome/CEM/OCM rows MUST still block. Query discovery remains unavailable, packet coverage remains `NOT_PRODUCED`, and structural completion MUST NOT assert query support, authority, adequate coverage, correctness, FULL integration or release qualification. Existing impact profiles/reasons retain their meanings.

## Evidence contract

The only admitted stderr bytes are UTF-8 followed by one LF:

```json
{"code": "unsupported-query-trace-state", "error": "native Go authority-start query requires an absent clean-tree local trace store", "ok": false}
```

Files live at `<git-dir>/corvint/coordination-time-query.{argv,json,stderr}` and
`<git-dir>/corvint/coordination-time-query-abstention.json`. The argv file contains exactly eight
NUL-terminated fields: executable path, `--root`, repository root, `query`, `--task`, unchanged
nonempty task, `--limit`, `1`. Root aliases resolving to the same directory are admitted.
Executable identity is an inert observation; it is never replay authority. Task identity is
bound to these original retained bytes, not authenticated human authorship.

The artifact is canonical compact JSON with one trailing LF, in this key order:
`argvSha256`, `base`, `exitStatus`, `profile`, `reason`, `status`, `stderrSha256`, `stdoutSha256`,
`step`, `target`, `taskSha256`. Digests are `sha256:` plus lowercase hex; `exitStatus` is the string
`2`, `status` is `NOT_PRODUCED`, and `step` is `coordination-time-query`. The exact canonical bytes
are checked, including member order and newline. The optional additive report member is either
its SHA-256 digest or null; existing `corvint-dogfood-change/0` and impact-abstention meanings stay
unchanged. Both change and a successful check print a NOTE retaining the row and reason.

## Failure modes and non-goals

Evidence write/clear failure produces `query-abstention-evidence-failed`; fabricated reserved
reason input produces `query-abstention-invalid`. Binding or shape failure produces
`query-abstention-evidence-drift`; replay disagreement produces `verifier-disagreement`.
Ordinary query errors retain their existing blocking reason. This is no general unsupported-query
allowlist, no trace repair/migration, no task rewriting, no query compiler change, no new learning
input and no replacement for original pre-change evidence or the declared fallback route.

## Acceptance evidence and rollback

`TestQueryAbstentionReplayAndTransitions` covers exact replay and abstention-to-success/other-failure
cleanup. `TestQueryAbstentionRejectsNoncanonicalRefusals` covers the error boundary.
`TestQueryAbstentionRejectsEvidenceDrift` covers hostile receipts, argv, report rows and failed local
outcome/CEM/OCM rows. `TestQueryAbstentionVerifierDisagreement` covers base/current/override
mismatch and unanimous wrong results. `TestQueryAbstentionCompatibilityAndCompletion` retains
older reports and refuses an unbacked exemption. Required integration additionally uses the
unchanged original task on the final clean tree with preserved trace, independently selected
verifiers, CEM/OCM closure, actual selected checks and clean local outcome; root coordination
retains that evidence. Focused synthetic tests alone do not establish that live qualification.

Rollback removes the query producer/checker exception and additive report field: the original
query refusal blocks completion again. Preserve all earlier trace and failure evidence. No
migration, history rewrite, release promotion or query capability claim is authorized here.
