# V1-1043: dogfood begin plan intent refusals name the rule (2026-10-08)

## Intent

A lifecycle plan whose `intents` array listed valid spec paths out of byte order made
`corvint dogfood begin --plan P` print only the bare code, so the cause stayed hidden behind later
`review`/`finish` refusals. `make dogfood-change` already names the rule for the same mistake in
`DOGFOOD_INTENTS_FILE` (`missing-intent-scope` fix line). The fix keeps the stable code and adds the
remediation to the message.

## Reproduction (before)

Fresh one-commit repository, plan `{"base":BASE,"intents":["docs/b.md","docs/a.md"],"checks":[...]}`,
installed Corvint 1.0.0-rc.2 (build 360):

    {"code":"invalid-intent-scope","error":{"code":"invalid-intent-scope","message":"invalid-intent-scope"},"ok":false}
    exit=2

## Change

- `LCP-V0-018` (proposed; appended): for `invalid-intent-scope` and `plan-bound-exceeded`, the
  nested `error.message` is the code, `: `, and a fixed remediation naming the sorted,
  de-duplicated 1-16 repository-relative spec path rule (and the 1-16 checks bound for
  `plan-bound-exceeded`). Codes, exit class, stdout and every other refusal's `message` are
  unchanged; the text is fixed, so the "no path or command text" property of
  `emitLocalCompletionFailure` holds. The refusal-code table rows cite the requirement.
- `cmd/corvint/local_completion.go`: `localCompletionRemedies` map consulted by
  `emitLocalCompletionFailure`.
- CCF-V1-004 freezes the dogfood envelope shape `{"code","error":{"code","message"},"ok":false}` and
  code names, not message text; no conformance case pins these two messages (grep of
  `conformance/` and tests found no occurrence).

## Verification

After the change, the same reproduction with the lane binary:

    {"code":"invalid-intent-scope","error":{"code":"invalid-intent-scope","message":"invalid-intent-scope: the plan intents array must list 1-16 repository-relative spec paths, each clean, sorted in byte order with no duplicates (the rule DOGFOOD_INTENTS_FILE follows for dogfood change); sort and de-duplicate it, for example with LC_ALL=C sort -u"},"ok":false}
    exit=2

- `TestDogfoodBeginPlanIntentRefusalNamesTheRule` (unsorted, duplicated, empty, 17 intents) passes;
  at the base the message equals the code, so the `code: ` prefix and rule assertions fail.
- `TestLocalCompletionCLIRejectsUnknownAndSecretInputs`, `TestLocalCompletionVerifyOKMirrorsCheckResult`
  pass; `go build ./...`, `go vet ./cmd/corvint` pass.
- Doc gates pass: spec-requirements-check, requirement-definitions-check, traceability-tests-check,
  decision-numbers-check, line-citations-check, error-code-ownership-check, unbounded-readers-check,
  use-case-receipts-check (after `script/repin-use-case-receipts.sh` repinned
  `UC-EVIDENCE-CARRYING-COMPLETION`), diagnostic-coverage-check.

## Limits

- The full `cmd/corvint` package and `make gate` were not run (focused tests only).
- A saved enrollment that fails `validatePlan` on reload also emits `invalid-intent-scope`; it now
  carries the same remediation text, which describes the rule but not that the saved state drifted.
- `LCP-V0-018` is proposed, not accepted.

## Independent review

Codex (gpt-6-astra, read-only) reviewed a4acc6f6..3b761f4c: no concrete defects; VERDICT: PASS.
