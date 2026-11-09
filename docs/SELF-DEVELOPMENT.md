# Corvint self-development

Use this guide when changing Corvint itself, or when a repository explicitly adopts it. It does not
change other repositories' workflows. Select applicable features for the task; do not run every
command. This is practical use of existing capabilities, not formal `FULL`, execution authority,
or a promise that a future host will automatically follow the guide. The owning contract is
[Corvint Self-Development V0](specs/corvint-self-development-v0.md).

## Start with the actual task

Freeze the base and collect pre-change evidence with the routes below. Preserve the original query, omissions, failures and unsupported results.
Use [AGENT-ROUTES](AGENT-ROUTES.md) to expand original evidence; a locator is not governing intent.
When several agent lanes share one host, follow [LANE-ORCHESTRATION](LANE-ORCHESTRATION.md)
(`ASD-V0-013..018`).
The table selects existing public routes, not a new plan or receipt format. Capture complete output
outside tracked source with root, commit/tree, profile, exit and runtime identity. Missing observations
remain `NOT_OBSERVED`; recorded bytes are not billed tokens or proof of savings.

| Trigger / family | Existing route | Evidence and boundary |
|---|---|---|
| Orient or plan | `query --task TASK`; `context --task TASK --subject PATH`; `context defs IDENTIFIER`, `context refs IDENTIFIER`, `context grep TERM...` | Read governance and exact cited spec/test/source. Gitlinks bind only superproject path/commit identities; nested contents remain outside coverage (EAF-V0-007). Keep critical missing, withheld and omitted rows. Do not rewrite a refused original task or open sealed holdouts merely because a packet names them. |
| Plan an explicit feature ID | `feature FEATURE_ID --limit 1` | Darwin/Linux supports known ledger-only context even with non-Go sources. Known IDs with limit >1 require Go candidate ranking; non-Go ranking is a typed gap. Unknown IDs remain OUT_OF_SCOPE; explicit-ID inventory is unsupported. Use the separate experimental `features` for inferred candidates. Preserve omissions. |
| Assess affected paths | `impact PATH...`; explicit `.go` untracked mode; clean `impact --base FULL_SHA` | Select the matching profile. Range ends at captured HEAD and excludes the chosen immutable base. Keep mixed-tree/non-Go limits; never discard user work to satisfy admission. |
| Discover repository scope | `features`; `overview`; `review --base FULL_SHA` | Experimental clean-HEAD guidance with inferred immutable evidence, inert next calls and bounded local branch overlap hints. Index/source omissions remain unknown; never closes CEM/OCM/frontier/test obligations. |
| Ask independent questions together | `batch` with an admitted bounded request | Use only with a current snapshot and independent query/context/path-impact operations. Keep per-operation errors. One question stays standalone. |
| Prepare an index or adopt a repository | explicit supervised `index --if-stale`; `init` / `adopt` for first adoption | Index writes derived state; no detached refresh or service. Missing/stale snapshot may use an admitted standalone read fallback. |
| Select tests after a meaningful dirty diff | `affected`; explicit immutable receipt via `affected --snapshot FILE` (AFP-V0-019) | Keep selected Go units, exclusions, unknowns and mandatory repository checks. Advice never permits skipping `make gate` or proves behavioral coverage. |
| Exercise main runner execution | separate experimental `corvint-test-runner plan` / approved `run` | 57 concrete experimental profiles, separately qualified by runtime tuple; listed unavailable IDs still refuse. Independently pinned tools, source/configuration and fresh reports retain native outcomes. SQL CASE/SUITE_ONLY and executed/skipped counts preserve their actual granularity. SourceFile is caller-bound; complete inventory does not accept a criterion. See [companion](../cmd/corvint-test-runner/README.md) and [execution contract](specs/test-runner-execution-v0.md). |
| Check native criterion capture | separately built `corvint-tasks criterion-binding capture` / offline `criterion-binding verify` | Native Tasks owns complete record/coherence validation; Core uses only wire projections through independently pinned bounded read calls. Historical validity is separate from current applicability. See [boundary contract](specs/tasks-criterion-binding-v0.md); installed versions lacking these experimental verbs remain unsupported. |
| Verify a CEM1 reference candidate | separately built standalone Go consumer `verify-candidate --repository DIR --map FILE --expected-base FULL_OID --target FULL_OID --artifacts DIR` | Exact `cem/1.0-experimental.1` only; independent immutable Git derivation and opaque artifact-byte integrity. All eight authority/source/discrimination/external limits remain NOT_OBSERVED. Go >=1.24 source-build floor; no default producer or legacy dispatch migration. See [public candidate algorithms](../protocol/cem-1.0/ALGORITHMS.md). |
| Draft/consume source documentation | `docs draft`; `docs consume` using actual emitted stdin bytes | Producer `corvint-source-documentation-draft/0`, consumer `corvint-documentation-consumption/0`. Require admitted committed owner Agent digest and exported Go package declarations. Source rederivation does not accept generated intent or deliver HDC. |
| Expand source-view experiment evidence | opt-in `adapter source-view` / `adapter claude-source-handoff` | Only tasks owning the experiment use its exact packet/digest/selectors. Normal work reads original cited immutable source. No default host wrapper or usefulness claim. |
| Read test dependency evidence | `test-evidence PATH [--lines START:END] --revision COMMIT`, `test-evidence --test TEST`, or `test-evidence --explain TEST --base COMMIT`; `flows test-evidence --flows DIR` joins AFU links; MCP group `test-evidence` exposes `corvint.test_evidence` | RSP-V0-051; reads existing records only. UNKNOWN names unavailable inventory, run context or freshness; absent admitted qualification is FULL unqualified. No collection or authority promotion. |
| Use a live Go provider | separate `corvint-go-test-provider --experimental --trusted-local --authority-bundle FILE` | Only an actual admitted parent-verifier attachment qualifies the route. Missing attachment is unavailable; ordinary selected tests still run. Protected runner/LPCV qualification is separate. |
| Inspect web flows and test gaps | `flows --manifest FILE [--evidence FILE]`; separate opt-in `corvint-web-flows` | Experimental declared-flow reports, literal assertion candidates and guided DOM observations. Read commands never launch the provider; unknown inventory, unexplored controls, stale evidence and backend contradictions remain visible. See [application flows](specs/application-flow-understanding-v0.md) and [provider usage](../tools/web-flows/README.md). |
| Change retrieval, learning or a profile | owning frozen `eval --goldens FILE` / registered benchmark engine arms | Pin engine, corpus, fixture and output identities; retain losing arms. No blind-v4 access or broad campaign for a prose-only change. Current eval supports an explicit trace-fixture arm, not unrestricted persisted trace replay. |
| Inspect a console or release readiness | explicit bounded `corvint-console`; `script/release-checklist` | Build standalone companions from their retained `cmd/corvint-*` source packages until the closed bundle migration lands; `corvint-companion-release -tasks-only` builds the separately bounded Tasks archive without qualifying the full bundle. Start an optional loopback server only for useful requested inspection with cleanup. Checklist/archive gates do not sign, tag, publish or promote; keep U4, packet-5 and DR holds. |
| Inspect Go semantic dependencies | `context --task TEXT --subject PATH --lsp gopls`; explicit MCP `task-review-lsp` profile with `lsp: "gopls"` | Optional installed local gopls, immutable endpoint checks, separated external evidence; no ranking change. Use [LSP setup and qualification](LSP.md); absence stays unavailable and synthetic parity does not establish agent benefit. |
| Change an optional analyzer/integration | owning public route and targeted tests from [spec index](specs/INDEX.json), including [Playwright External Provider V0](specs/playwright-external-provider-v0.md) for externally managed Playwright servers | Distinguish delivered internals, optional artifacts and unbuilt parent contracts. Do not separately launch every analyzer, MCP, Pulse or service; state the exact unavailable/inapplicable boundary. |
| Consume external evidence | `impact --provider FILE`, `--provider-command ARGV_JSON`, or `--provider-mcp ARGV_JSON` | The accepted bounded MCP profile calls one local tool. Remote HTTPS requires the separately built `corvint-remote-provider --allow-network --config FILE` command; Core never fetches network evidence. All routes retain the same strict EEP decode, reference checks and separated authority. |

For issue392 connector work, use the separate experimental `corvint-postmerge-connect` CLI and
its [local conformance kit](../cmd/corvint-postmerge-connect/README.md). Recording/dry-run retain
fixed canonical requests; live references update private local files only. Vendor transport,
host credential isolation and independent adopter qualification remain unobserved.

## Read, implement, challenge

The [six experimental workflows](V1-WORKFLOWS.md) add bounded, explicit routes to this table:
`breakage` for pinned declared repositories and API relationships; the explicit `context`
instruction flags for conditional project-file selection; and `script/proof-tour.sh` for the
disposable independent-review tour. Use only the route relevant to the task. Actual session
loading, cross-provider continuity, behavioral breakage, adequate tests and third-party
interoperability remain unqualified; none of these routes accepts intent or closes authority.

Use `corvint help COMMAND` and the owning spec for exact flags and admission before invoking a
route. Begin with a handful of applicable rows. After the first meaningful source diff, inspect
`affected` when it can advise the changed paths; keep the actual gate mandatory. A guidance-only change may not need dirty-Go advice, batch,
mutation, draft generation or checkpoint handling. Do not manufacture source changes or toy tasks
to make those rows appear used.

Keep route dispositions in the task evidence/build-log entry prose: **used**, **unavailable**,
**deferred**, or **not applicable**, with an original output pointer or concrete reason. “Used” means
a command actually ran for this task at the identified root/revision; an example, unit test or green
gate does not establish host adoption. This is caller assessment, not a new ledger, universal
planner, strict LCP field, authenticated record or feature-use percentage.

For the optional [documentation corpus route](DOCUMENTATION-CORPUS.md), inventory a committed
slice, build a corpus and retain an actual search/trace receipt. Use explicit corpus evidence on native
reads only where applicable; unsupported providers and absent journeys remain visible. The separate
MCP server and generated-block writes require their own explicit invocation.
The experimental behavior-provider profile additionally retains exact flow/test/project joins and
ordered runtime witnesses; `docs corpus behavior-adapter` can produce and reconcile that profile
from caller-mapped inventories without changing its legacy wire member or authority. The separate
For issue 339, use the explicit adoption `/2` corpus and `docs corpus inventory --offset` to inspect
large supplied-record imports and restricted severity-only summaries; this does not accept imported intent.
The `docs corpus behavior-provider` route emits bounded `/2` multi-repository declarations for corpus
ingestion; unavailable external bytes and runtime qualification stay explicit gaps. Its reviewed
documentation projection is normative: source/test proposals cannot rewrite stable variation IDs or
their preconditions, actions, facts and outcomes. Missing consumer fixtures, semantic joins or runtime
evidence preserve gaps and unconditional full-suite fallback. See the
behavior-contract and behavior-provider adapter sections of `docs/specs/documentation-corpus-v1.md`.
The separate stability profile retains independent repetitions, nested Playwright retries and manual
reruns with raw denominators, cleanup and exact declared-versus-observed execution topology. It never
upgrades behavior coverage, adequacy or parity.

For the optional documentation route, retain producer output and feed those exact bytes to the
consumer; its [source contract](specs/source-documentation-draft-v0.md) gives the admitted example.
For mutation, evaluation, migration, signing, provider and service routes, obey the existing scope,
permission, frozen-input and cleanup boundaries. Optional unavailability never licenses fake input,
weakened policy or erasing user work.

For the experimental [corpus republish companion](specs/corpus-republish-v0.md), retain separate approved documentation/source pins, deterministic producer bytes, complete previous-inventory parity and an actual indexed inventory receipt. The issue #397 local command and pending-state tests exercise admission, replacement, retry, corrupt-state preservation and destination protection. Incremental reuse is limited to explicitly eligible ordinary adoption contributions; global validation and indexed production still run. Host approval authentication, credential isolation, remote publishing and independent external adoption remain unqualified.

## Finish through the existing gate

Commit the source, run the selected checks and the gate that [AGENTS.md](../AGENTS.md) names, and
keep actual test results separate from structural or advisory output. The CEM bind, check and seal
loop was retired by decision 0594.

Independent review checks whether a material applicable family was missed and whether each claimed
use has real evidence. Repair the omission or explain its boundary before acknowledgment. Close measurement after
review, keeping prior planning gaps and unknown tokens/cost visible. If no paired baseline exists,
make no savings claim. Native host use in another checkout stays `NOT_OBSERVED` for this task.

Update this guide's affected route/status and its relevant evidence in the same future feature
change. Rollback removes these routing additions without weakening the existing completion policy,
removing old traces, or reclassifying a failed or unqualified experiment.

For owner-trusted MkDocs navigation work, use the separate explicit `docs nav --trusted-project`
profile and retain the real pinned loader/reload artifact. See
[Trusted Project Navigation V0](specs/trusted-project-navigation-v0.md). This does not authorize
execution through ordinary docs reads or qualify the HDC capsule, site build or offline behavior.

For issue338 source-derived documentation, use `docs flows generate`, retain the emitted generation
manifest, then use `docs flows check` for read-only per-item span drift and byte regeneration checks.
Commit provider bytes only at the caller's explicit boundary and use `docs flows finalize` before
corpus build/Open. This route produces generated lexical observations; existing accepted-outcome
AFU docs and original Playwright witnesses keep their separate meaning. See [flow generation](FLOW-DOCUMENTATION.md).

For a complete accepted flow-variation denominator and original `/3` receipts, use the bounded
[flow coverage route](FLOW-COVERAGE.md): `flows coverage --denominator FILE --receipts FILE`.
Its global verdict is fail-closed even when paging; byte-bound PROVEN does not attest deployed lineage.
Only explicit `--write-back FRESH_DIR` writes documentation, while `--check DIR` remains read-only.


Pi integration changes use `docs/specs/pi-workflow-v0.md`, AHI-024/025 and LCP-V0-008/009.
Retain closed native read/receipt evidence and run the focused Pi units, actual pinned
Pi host modes and native Tasks temporary-store fixture. These do not qualify protected
Pi authority, other operating systems, supervisor participation or comparative superiority.

For operator-requested machine updates, use the separate `corvint-update check|apply|rollback`
companion under [Operator update V0](specs/operator-update-v0.md). It is not a Core read command;
network access and executable activation are explicit, with checksum and qualification limits.
