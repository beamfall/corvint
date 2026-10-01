# Experimental historical replay

Build the optional companion and invoke it with separate historical and host-policy files:

```sh
GOTOOLCHAIN=local go build -o /tmp/post-merge-workflow ./tools/post-merge-workflow
/tmp/post-merge-workflow replay --change CHANGE-ID --dry-run \
  --fixture /absolute/historical.json --policy /absolute/host-policy.json --root /absolute/product-repo
```

Exit 0 means MATCH, 1 means MISMATCH and 2 means BLOCKED or invalid invocation. The JSON report
contains canonical `recording_jsonl`, its digest, input/policy bindings, basis-labelled mismatches,
reasons and qualification limits. MATCH never implies whole-workflow acceptance: qualification is
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
stages block. Document/test authoring and all draft requests currently block because the actual
scope/validation verifier is not integrated. Never fake receipts to make that path pass.

The same adapter bytes execute twice from fresh private directories; typed outcomes, stage artifact
digests and actual connector recording bytes must match. A minimal environment excludes inherited
credentials. This is not a filesystem or network sandbox, so every CI host must supply its own
containment and runtime/input bindings before any positive whole-workflow qualification.

The focused conformance tests execute a synthetic adapter against real disposable Git content and
the real local recording connector. They are harness evidence only. Actual delta/intake/author/
scope/validation/metrics integrations, historical labels, local and CI host qualification, independent
review, final change evidence, landing and native completion remain required for the whole issue.
