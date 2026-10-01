# Experimental historical replay

Build the optional companion and invoke it with separate historical and host-policy files:

```sh
GOTOOLCHAIN=local go build -o /tmp/corvint-postmerge-workflow ./cmd/corvint-postmerge-workflow
/tmp/corvint-postmerge-workflow replay --change CHANGE-ID --dry-run \
  --fixture /absolute/historical.json --policy /absolute/host-policy.json --root /absolute/product-repo
```

Exit 0 means MATCH, 1 means MISMATCH and 2 means BLOCKED or invalid invocation. The JSON report
contains canonical `recording_jsonl`, its digest, input/policy bindings, `human_verified_mismatches`
and `generated_mismatches` as separate lists, `deferred_stages`, reasons and qualification limits. MATCH never implies whole-workflow acceptance: qualification is
always `NOT_OBSERVED` in this experimental slice. Redirect the report to a trusted private location
outside input and source paths if it should be retained. The CLI does not choose an output file.

The Go wire definitions are `internal/postmergeworkflow/types.go`; the executable contract is
`docs/specs/postmerge-replay-v0.md`. Fixture `connector` is the existing `postmerge-connector/0`
historical forge/tracker fixture. `source_item` identifies its linked tracker item. Expectations
contain affected flow IDs, a follow-up boolean, gap IDs, typed findings, and exactly four label
entries (`affected_flows`, `followup`, `test_gaps`, `defects`). Every label carries a basis and
`evidence_sha256`; human labels also carry human/approval IDs matching the full forge/repository/change/base/merge binding in the separately pinned registry.

Host policy contains the connector policy and independently pins an absolute adapter executable,
its SHA-256, bounded arguments and `runtime_sha256`. It optionally pins an absolute registry and
its SHA-256. Registry value digests hash the exact compact JSON representation of the expected
value, before semantic set sorting. The registry is an operator-owned approval record; it does not
authenticate a person or establish historical correctness. Do not put credentials in policy arguments.

The adapter receives only a `postmerge-replay/0` request with binding, complete fixture SHA-256
and runtime SHA-256 on stdin. It must resolve immutable inputs through its trusted runtime contract,
execute actual supported stages, and return the closed `Result` profile. It never receives raw
title/body from the harness. Required stage applicability is fixed by harness code. Reported
documentation targets, gaps, findings and counts must be consistent with connector input. Missing
stages block. Detected documentation targets and test gaps are compared and counted in the follow-up
request, but their author/scope/validation stages and draft requests must be reported as `deferred`
because the actual scope/validation verifier is not integrated: an `observed` claim for them, or
any connector draft, blocks. Never fake receipts to make that path pass.

`affected_flows` and `test_gaps` are the delta stage outputs. Once `corvint delta` (issue #389) is
available, an adapter derives them from that record's documentation-drift and uncovered-behaviour
sections and reports the record's SHA-256 as the `delta` stage digest. Replay itself never parses
the record, so a later record schema change touches only adapters.

The same adapter bytes execute twice from fresh private directories; typed outcomes, stage artifact
digests and actual connector recording bytes must match. A minimal environment excludes inherited
credentials. This is not a filesystem or network sandbox, so every CI host must supply its own
containment and runtime/input bindings before any positive whole-workflow qualification.

The focused conformance tests execute a synthetic adapter against real disposable Git content and
the real local recording connector. They are harness evidence only. Actual delta/intake/author/
scope/validation/metrics integrations, historical expectations with generated or human-verified basis,
local and CI host qualification, independent review, final change evidence, landing and native
completion remain required for the whole issue. Human labels are optional; human-verified coverage
remains `NOT_OBSERVED`.
