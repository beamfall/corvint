# Corvint self-development

Use this guide when changing Corvint itself, or when a repository explicitly adopts it. It does not
change other repositories' workflows. Select applicable features for the task; do not run every
command. This is practical use of existing capabilities, not formal `FULL`, execution authority,
or a promise that a future host will automatically follow the guide. The owning contract is
[Corvint Self-Development V0](specs/corvint-self-development-v0.md).

## Start with the actual task

Follow [DOGFOOD](DOGFOOD.md) for measurement, immutable base, explicit intent/check enrollment and
pre-change evidence. Preserve the original query, omissions, failures and unsupported results.
Use [AGENT-ROUTES](AGENT-ROUTES.md) to expand original evidence; a locator is not governing intent.
The table selects existing public routes, not a new plan or receipt format. Capture complete output
outside tracked source with root, commit/tree, profile, exit and runtime identity. Missing observations
remain `NOT_OBSERVED`; recorded bytes are not billed tokens or proof of savings.

| Trigger / family | Existing route | Evidence and boundary |
|---|---|---|
| Orient or plan | `query --task TASK`; `context --task TASK --subject PATH`; `context defs IDENTIFIER`, `context refs IDENTIFIER`, `context grep TERM...` | Read governance and exact cited spec/test/source. Keep critical missing, withheld and omitted rows. Do not rewrite a refused original task or open sealed holdouts merely because a packet names them. |
| Plan an explicit feature ID | `feature FEATURE_ID --limit 1` | Darwin/Linux supports known ledger-only context even with non-Go sources. Known IDs with limit >1 require Go candidate ranking; non-Go ranking is a typed gap. Unknown IDs remain OUT_OF_SCOPE; explicit-ID inventory is unsupported. Use the separate experimental `features` for inferred candidates. Preserve omissions. |
| Assess affected paths | `impact PATH...`; explicit `.go` untracked mode; clean `impact --base FULL_SHA` | Select the matching profile. Range ends at captured HEAD and excludes the chosen immutable base. Keep mixed-tree/non-Go limits; never discard user work to satisfy admission. |
| Discover repository scope | `features`; `overview`; `review --base FULL_SHA` | Experimental clean-HEAD guidance with inferred immutable evidence, inert next calls and bounded local branch overlap hints. Index/source omissions remain unknown; never closes CEM/OCM/frontier/test obligations. |
| Ask independent questions together | `batch` with an admitted bounded request | Use only with a current snapshot and independent query/context/path-impact operations. Keep per-operation errors. One question stays standalone. |
| Prepare an index or adopt a repository | explicit supervised `index --if-stale`; `init` / `adopt` for first adoption | Index writes derived state; no detached refresh or service. Missing/stale snapshot may use an admitted standalone read fallback. |
| Select tests after a meaningful dirty diff | `affected` | Keep selected Go units, exclusions, unknowns and mandatory repository checks. Advice never permits skipping `make gate` or proves behavioral coverage. |
| Challenge a context/change/CEM claim | `prove --task TASK`, `prove PATH...`, `prove --base FULL_SHA`, or `prove --cem MAP` | Choose the relevant wrapper; retain embedded receipt and every PASS/FAIL/NOT_RUN. Do not repeat an identical query just to count another feature. |
| Challenge behavioral test coverage | explicit `prove PATH... --mutate` or range mutation | Only when the task needs this experimental execution and its sandbox/dependencies/budgets admit it. Retain baseline failures, survived mutants and unrun rows; never a hook or every-edit default. |
| Resume a caller-owned checkpoint | `prove --checkpoint FILE` | Revalidate actual existing `corvint-checkpoint/0` handles against current Git. No automatic writer, invented checkpoint, or claim of full conversation recovery. |
| Draft/consume source documentation | `docs draft`; `docs consume` using actual emitted stdin bytes | Producer `corvint-source-documentation-draft/0`, consumer `corvint-documentation-consumption/0`. Require admitted committed owner Agent digest and exported Go package declarations. Source rederivation does not accept generated intent or deliver HDC. |
| Expand source-view experiment evidence | opt-in `adapter source-view` / `adapter claude-source-handoff` | Only tasks owning the experiment use its exact packet/digest/selectors. Normal work reads original cited immutable source. No default host wrapper or usefulness claim. |
| Select backlog work or propose a wave | `script/corvint-work-queue`; `work observe` / `work propose-wave` | Validate current queue evidence, then inspect shadow proposals and conflicts. Existing coordinator owns claims and dispatch; observation authorizes neither. |
| Inspect native fixture planning | `work plan-fixture --executor ABSOLUTE_FILE --observations FILE` | Explicit native fixture only; fixed read commands and immutable source/snapshot bindings. Trusted-local reservation/history observations; no admission or production qualification. See [NTP](specs/native-taskman-planning-v0.md). |
| Bind and review a change | keyed `dogfood` workflow plus `cem` / every scoped `ocm` | Reuse the existing frozen plan, selected-check observations and exact report-set acknowledgment. Inspect citation meaning, unknowns and check adequacy. |
| Inspect unresolved review obligations | `frontier --cem MAP --ocm MAP --expected-base BASE --target TARGET --json`; `lrf` where relevant | Before acknowledgment inspect each relevant bound scope. Valid exit 1 means an open advisory queue; distinguish it from invalid input. `frontier/0` cannot close authority or make local policy satisfied. |
| Use a live Go provider | separate `corvint-go-test-provider --experimental --trusted-local --authority-bundle FILE` | Only an actual admitted parent-verifier attachment qualifies the route. Missing attachment is unavailable; ordinary selected tests still run. Protected runner/LPCV qualification is separate. |
| Record outcome or inspect learning | existing `dogfood finish`; supported trace-aware query; explicit `migrate-traces` | Do not duplicate finish's outcome. Migration apply requires explicit scope. Preserve trace refusal/learned-path counts; a mechanism or scoring change needs the registered two-arm LTA gate, not every record row. |
| Investigate repeated friction | `observations`; explicitly scoped `prove-observe` | Bounded private counts never rank evidence or confer authority. No transcript scan, invented usage or background monitor. |
| Change retrieval, learning or a profile | owning frozen `eval --goldens FILE` / registered benchmark engine arms | Pin engine, corpus, fixture and output identities; retain losing arms. No blind-v4 access or broad campaign for a prose-only change. Current eval supports an explicit trace-fixture arm, not unrestricted persisted trace replay. |
| Compare migration evidence | `migration-ratchet --profile FILE` | Bind exact baseline/candidate artifacts and preserve every delta, denominator, exception and comparability rule. Pass is incremental no-regression only; it proves neither adequacy nor completeness. |
| Inspect a console or release readiness | standalone `corvint-dashboard-snapshot`; explicit bounded `corvint-console`; `script/release-checklist` | Build standalone companions from their retained `cmd/corvint-*` source packages until the closed bundle migration lands. Start an optional loopback server only for useful requested inspection with cleanup. Checklist/archive gates do not sign, tag, publish or promote; keep U4, packet-5 and DR holds. |
| Change an optional analyzer/integration | owning public route and targeted tests from [spec index](specs/INDEX.json), including [Playwright External Provider V0](specs/playwright-external-provider-v0.md) for externally managed Playwright servers | Distinguish delivered internals, optional artifacts and unbuilt parent contracts. Do not separately launch every analyzer, MCP, Pulse or service; state the exact unavailable/inapplicable boundary. |
| Consume external evidence | `impact --provider FILE`, `--provider-command ARGV_JSON`, or `--provider-mcp ARGV_JSON` | The accepted bounded MCP profile calls one local tool. Remote HTTPS requires the separately built `corvint-remote-provider --allow-network --config FILE` command; Core never fetches network evidence. All routes retain the same strict EEP decode, reference checks and separated authority. |

## Read, implement, challenge

Use `corvint help COMMAND` and the owning spec for exact flags and admission before invoking a
route. Begin with a handful of applicable rows. After the first meaningful source diff, inspect
`affected` when it can advise the changed paths; keep the actual gate mandatory. Challenge relevant
claims with nonmutating `prove`. A guidance-only change may not need dirty-Go advice, batch,
mutation, draft generation or checkpoint handling. Do not manufacture source changes or toy tasks
to make those rows appear used.

Keep route dispositions in the existing task evidence/BUILD-LOG prose: **used**, **unavailable**,
**deferred**, or **not applicable**, with an original output pointer or concrete reason. “Used” means
a command actually ran for this task at the identified root/revision; an example, unit test or green
gate does not establish host adoption. This is caller assessment, not a new ledger, universal
planner, strict LCP field, authenticated record or feature-use percentage.

For the optional [documentation corpus route](DOCUMENTATION-CORPUS.md), inventory a committed
slice, build a corpus and retain an actual search/trace receipt. Use explicit corpus evidence on native
reads only where applicable; unsupported providers and absent journeys remain visible. The separate
MCP server and generated-block writes require their own explicit invocation.
The experimental behavior-provider profile additionally retains exact flow/test/project joins and
ordered runtime witnesses; missing consumer fixtures or runtime evidence preserve gaps and full-suite
fallback. See the behavior-contract section of `docs/specs/documentation-corpus-v1.md`.
The separate stability profile retains independent repetitions, nested Playwright retries and manual
reruns with raw denominators, cleanup and exact declared-versus-observed execution topology. It never
upgrades behavior coverage, adequacy or parity.

For the optional documentation route, retain producer output and feed those exact bytes to the
consumer; its [source contract](specs/source-documentation-draft-v0.md) gives the admitted example.
For mutation, evaluation, migration, signing, provider and service routes, obey the existing scope,
permission, frozen-input and cleanup boundaries. Optional unavailability never licenses fake input,
weakened policy or erasing user work.

## Finish through the existing gate

Follow [DOGFOOD §4](DOGFOOD.md#4-produce-the-cem) to commit source, bind and commit the CEM, and prepare
every owning OCM on the same final target. Run frozen selected checks through the existing keyed
`dogfood verify`. Before acknowledging its report set, read the CEM/OCM reports and relevant
`frontier/0` queues together. Keep structural links, actual test results, local satisfaction and
Frontier authority separate. An open advisory queue is not a command failure or a new Stop rule.

Independent review checks whether a material applicable family was missed and whether each claimed
use has real evidence. Repair the omission or explain its boundary before acknowledgment. Reuse
strict finish's single explicit outcome; no duplicate learning record. Close measurement after
review, keeping prior planning gaps and unknown tokens/cost visible. If no paired baseline exists,
make no savings claim. Native host use in another checkout stays `NOT_OBSERVED` for this task.

Update this guide's affected route/status and its relevant evidence in the same future feature
change. Rollback removes these routing additions without weakening the existing completion policy,
removing old traces, or reclassifying a failed or unqualified experiment.
