# Non-Go impact: preserve historical path parity during CI repair

Date: 2026-09-30. Issue #390 / native V1-0537; draft PR407. This is a source-level
repair record, not integration, human contract acceptance or ticket completion.

## Observed failure and decision

PR407 CI at sealed d20fbdc2 found missing ownership for seven emitted syntax
unknown codes, stale default/task-review MCP tool-list fixtures, analyzer/98
input-audit drift, historical `impact-web-component` stdout drift, and changed
no-module refusal precedence in the portable daily-workflow fixture. Earlier
focused passes and the original satisfied enrollment remain tied to 386f92ee;
they are not renamed as validation of this repair.

The immutable CLI manifest is historical oracle evidence under GOC-V0-002 and
GPK-V0-033/034. Its expectations are unchanged. Default path Impact/EvalImpact
retain their historical envelope and bytes; explicit
`--language-profile non-go-syntax-v0` selects the new structured non-Go frontier.
MCP non-Go paths and newly admitted non-Go ranges name that experimental profile
automatically. Syntax evidence does not qualify dispatch, test closure or runtime
coverage. NGI remains a proposed technical contract; no accepted divergence or
oracle expectation is generated from candidate output.

Repositories without admitted Ruby/JS/TS source retain the historical no-module
refusal before dirty-worktree validation, including the first-pass CEM workflow.
Mixed repositories still validate module eligibility against changed Go membership
after snapshot/base checks, with multiply-invalid precedence explicitly limited.
Analyzer extraction/receipt inputs were reviewed and the maintenance schema was
bumped to analyzer/99; the reviewed audit digest is
383cb833841c4bbaed61ddc5862fb26966a03c0d380e370b8b7932c1cf5acd65.
The two MCP goldens change only impact's description and suffix schema; all other
response bytes were independently compared unchanged. Closed codes are now named
in the owning NGI spec.

## Focused execution and independent finding

- Immutable `TestGPKV0002ManifestReplay`: PASS, 146.862s, original manifest unchanged.
- CLI explicit/default profile, hostile option admission and portable no-module/module-root workflow: PASS, 18.576s.
- Selected non-Go/Go impact/range conformance plus analyzer audit: PASS, 47.300s.
- Exact MCP default/task-review tool-list fixture test: PASS, 0.385s.
- Native MCP non-Go admission/frontier test: PASS, 1.136s.
- Git-index-based error-code ownership check: PASS after staging the actual owning spec.
- Independent bounded source/contract/fixture review: no P1/P2; reviewer independently executed the analyzer audit and checked both MCP golden deltas.

These runs observed the candidate source before its repair commit. No repository-wide
gate or release qualification was rerun. Local temporary evidence is retained under
`/tmp/impact-ci-*`; the original failed CI logs are retained by the coordinator.

## Remaining closeout at this source boundary

The staged repair correctly invalidates line citations and the generated
REQUIREMENTS.tsv. Their surgical relocation/regeneration, fresh CEM/enrolled
binding/check/seal, draft update, integration and native completion remain pending.
The shared table/CEM paths are owned by another active lane; this source claim will
be committed cleanly and released before the next atomic closeout. No passing
publication or native completion is claimed from this intermediate commit.

Rollback: revert the task-owned repair commit and keep the old sealed CEM,
immutable manifest, failed CI evidence and prior source-bound checks.
