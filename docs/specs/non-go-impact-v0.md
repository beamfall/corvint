# Non-Go impact V0

Owner: Russell Lewis
Date: 2026-09-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner-requested GitHub issue #390 (native V1-0537), and the
2026-09-30 instruction to implement it. This technical contract proposes concrete
semantics; issue intent does not accept generated implementation detail.

## Agent digest
- Claim: Ruby, JavaScript and TypeScript path/range impact binds syntax evidence and explicitly names unresolved dynamic dispatch.
- Status: proposed/experimental; local source and focused conformance implemented; human acceptance and integration pending.
- Exists: Ruby/JS/TS versioned positional, committed-range and MCP admission; bounded structured unknowns; existing JS/TS import rules.
- Blocked on: shared registry/CEM closeout, draft publication, integration and owner acceptance; no runtime-dispatch qualification.
- Read next: Requirements; Trust and failure boundaries; Acceptance and traceability.

## User and measurable job

A maintainer of Ruby/JS/TS product code needs immutable impact evidence for paths
or a complete committed range. The baseline CLI already admits these indexed
paths, but the range compiler requires a Go module and only qualifies Go hunks;
MCP admission requires .go. Syntax observations cannot establish runtime call or
test closure. The useful result is bounded evidence with an open semantic frontier.

## Requirements

- `NGI-V0-001`: Positional impact and MCP impact MUST admit tracked `.rb`, `.js`,
  `.jsx`, `.mjs`, `.cjs`, `.ts` and `.tsx` paths. MCP schema and runtime MUST agree
  on the same closed suffix set, retain path confinement, duplicates and bounds,
  and keep unchanged Go admission. CLI's broader indexed-path admission remains.
- `NGI-V0-002`: Default and expanded committed ranges MUST bind Ruby/JS/TS path
  membership and exact old/new textual hunk spans using the existing range
  identity, status, ancestry, path, source-size, deadline and drift checks.
  Go module/package/parser preconditions MUST apply only to changed Go members;
  root-level Ruby/JS/TS and repositories without Go modules MUST be admitted.
- `NGI-V0-003`: Every Ruby/JS/TS changed path MUST name a syntax-only dynamic
  dispatch limitation in structured unknowns independent of ranked-result limits.
  Specific detected computed dispatch, dynamic load, reflection or Rails autoload
  MUST add closed frontier codes. Detector absence MUST NOT establish closure.
  Positional CLI requests select `--language-profile non-go-syntax-v0`; the
  historical default path receipt remains byte-identical. MCP non-Go requests
  and non-Go range receipts select/name this experimental profile automatically.
  The selector is unavailable with `--base` or `--working-tree-untracked`.
  Ruby reverse-import resolution remains explicitly unavailable; JS/TS reuses
  existing relative/profile import rules without inferring runtime call semantics.
- `NGI-V0-004`: Added paths MUST carry the same receipt/evidence envelope and
  target Git blob binding as Go evidence. Range markers/ADR relations MUST remain
  exact-hunk qualified and caller-authored authority MUST remain withheld.
  Result omissions, critical misses and unsupported language members MUST remain
  explicit. Legacy Go-only receipts and canonical digest preimages MUST retain
  their exact bytes when no new language members occur. Existing refusal
  conditions and codes remain. Module validation keeps historical precedence in
  repositories without admitted Ruby/JS/TS source, including the no-module
  daily-workflow abstention. In a repository admitting those languages it depends
  on changed Go membership and follows snapshot/base validation; exact precedence
  for multiply-invalid mixed repositories is not a compatibility claim. Frozen
  oracle expectations remain immutable and are never regenerated from a candidate.
- `NGI-V0-005`: `impact --help` and MCP description MUST publish a per-language
  capability table identifying path/range support, syntax-only import rules and
  unresolved dispatch. Working-tree-untracked remains the separately bounded Go
  profile. Help MUST distinguish receipt production from semantic completeness.
- `NGI-V0-006`: Conformance MUST include each new suffix/language, negative
  dynamic cases and comment/string twins, mixed Go/web ranges, root-level inputs,
  no-Go-module repositories, limits, dirty/drift/excluded/binary refusals, and
  byte-identical repeat runs. Tests MUST establish schema/runtime agreement and
  unchanged Go-only and historical default path bytes. The CLI selector MUST be
  tested for invalid/repeated values and mutually exclusive profiles. No test execution or qualification is implied
  by an impact receipt.

## Trust and failure boundaries

Core performs bounded local immutable reads; it adds no network, process provider,
state write, execution, daemon or authority. Existing unsupported statuses/modes,
unsafe paths, binary content, excluded source and snapshot races remain refused.
A syntactic relation has syntax authority; only independently accepted intent has
project authority. Structured unknowns contain paths, counts and closed codes,
not excerpts or source-controlled diagnostic text. An output limit must never
remove the semantic frontier. Existing non-Go outside-profile omissions remain
for languages not admitted by this new range capability.

## Closed syntax unknown codes

| Code | Meaning |
|---|---|
| `dynamic-dispatch-unresolved` | Baseline: syntax does not establish runtime dispatch or test closure. |
| `reverse-import-rule-unavailable` | Ruby reverse-import semantics are unavailable. |
| `source-analysis-unavailable` | The requested source is missing, excluded or unavailable to syntax analysis. |
| `observed-reflective-dispatch` | A lexical reflection/metaprogramming candidate was observed. |
| `observed-computed-dispatch` | A computed-call candidate was observed. |
| `autoload-unresolved` | Ruby autoload was observed without resolved runtime semantics. |
| `dynamic-load-unresolved` | A load argument is dynamic, interpolated or concatenated. |

These codes are uncertainty, never execution evidence. The versioned receipt
names `language_profile: non-go-syntax-v0`; line 0 denotes a baseline frontier.

## Acceptance and traceability

| Requirement | Implementation | Executable evidence |
|---|---|---|
| `NGI-V0-001` | shared suffix helper; MCP descriptor/validator | `TestNonGoImpactMCPAdmission` |
| `NGI-V0-002` | shared range compiler language classification | `TestNonGoRangeImpact` |
| `NGI-V0-003` | shared lexical mask and structured frontier | `TestNonGoImpactDynamicUnknowns` |
| `NGI-V0-004` | existing range bindings/reducer | `TestNonGoRangeImpactLegacyGoBytes`, `TestNonGoRangeImpactGoModuleValidationPriority`, `TestNonGoImpactCLIProfile` |
| `NGI-V0-005` | impactHelp and MCP description | `TestNonGoImpactMCPAdmission`; actual CLI help check in the build-log |
| `NGI-V0-006` | committed fixture conformance | `TestNonGoRangeImpactMixed`, `TestNonGoRangeImpactRefusals`; tests above and existing range/impact regressions |

Names in this matrix identify evidence rather than implying passing execution.
Observed execution and independent findings are retained in
`../build-log/2026-09-30-non-go-impact.md`. No broad formal qualification or promotion is claimed.

## Non-goals, rollout and rollback

No Ruby runtime call-graph resolver, compiler/type-checker, complete dynamic
semantic impact, dynamic capacity increase, test executor, new provider transport
or working-tree non-Go admission. Existing wider positional suffix support is not
removed. The implementation stays experimental pending human acceptance and
integration. Revert task-owned code/spec changes to roll back; preserve original
receipts and the failed/unsupported outcomes. Kill or repair the extension if it
changes legacy Go bytes, invents semantic closure or weakens immutable binding.
