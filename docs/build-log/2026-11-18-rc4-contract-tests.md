# RC4 docs contract tests

Date: 2026-11-18
Ticket: V1-5679, lane rc4-contract, task 3.1 only
Authority: RCF-V1-011 and owner decision RCF-D2 in the accepted rc4-candidate design.

## Decision

Extend `internal/mcp/corpusbridge/parity_test.go` and reuse its typed schema coverage, the
existing `/2` corpus builder fixture and the compact receipt fixture. Both docs servers use
this bridge. The advertised output schema describes only common fields, so pinning that
schema alone would leave most output fields unprotected.

Commit explicit field lists under `internal/mcp/corpusbridge/testdata/rcf-contract/` for:

- Each promoted tool's input and advertised output schemas, including bounds and required fields.
- Actual full, compact, paginated, missing-result and error responses, plus optional info fields.
- Every serialized field reachable from manifest, artifact and receipt types, including optional
  nested fields, JSON omission tags, input freshness and repository listing records.

Field lists require existing names and types but allow additive fields. Runtime lists union
array element types and replace result-ID map keys with a wildcard. They pin no IDs, values,
rank scores or result order. Type lists complement runtime fixtures where optional data is absent.
Prose descriptions are not pinned. Existing goldens, production code and wire shapes are unchanged.

The first `openspec-check` rejected checked task 3.1 and the already-checked task 4.1 after
pending task 2.3 (OSC-V1-005). Move those two completed prerequisites before the qualification
section, retaining their task numbers, scope and status. Qualification remains unchecked; no
baseline or gate is weakened. This order follows their actual dependency on accepted contracts,
not on the qualification result.

## Evidence and limits

`TestRCFV1011DocsToolSchemas`, `TestRCFV1011DocsToolOutputs` and
`TestRCFV1011CorpusFormat` cover RCF-V1-011's contract-test prerequisite. A temporary rename of
`Artifact.manifest_sha256` to `manifest_digest` failed `TestRCFV1011CorpusFormat`, naming the
missing field and expected `string json:manifest_sha256`. Separate renames of the search input
`query` to `query_renamed` and info output `built_at` to `compiled_at` failed
`TestRCFV1011DocsToolSchemas` and `TestRCFV1011DocsToolOutputs`. All mutations were reverted.
Mutation log SHA-256 values (raw logs live outside the repository):

- Artifact: `a18d73efa13ff9f982fe271a356075aafc68d82a09879e67845f5facbc89e0e5`
- Input: `3f9ff5a18079e948f90794fe3284e59fe76b238031715069f276b7642c444ec4`
- Output: `af0ffdc931068c510a9bff4e04e2b1c3aaefca9b92d0d587e54165d9b40d1a1d`

The whole `internal/mcp/corpusbridge` package passed via `script/heavy.sh` with no `-run`
filter (`go test -count=1 -timeout 30m`, 47.192 seconds), as did `go vet` on that package.
`make spec-requirements` produced no generated changes. Line-citation repinning moved nothing.
The affected plan selected this package directly but retained scope UNKNOWN and 57 unknowns;
no exhaustive coverage is claimed. The final
`make -s spec-requirements-check line-citations-check openspec-check` passed after the task-order
repair; OpenSpec retained 65 existing baseline findings. No baseline changed.

Task 3.2 is not delivered: qualification, stability promotion and delivery status remain pending.
Independent review is NOT_RUN in this lane; the orchestrator owns it. Darwin execution is NOT_RUN
on this Linux host. The retired dogfood/CEM workflow is not applicable (decision 0594).
