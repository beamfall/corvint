# Flow variation coverage

`corvint flows coverage` reports the complete declared documentation inventory against accepted
AFU variation matrices and retained original Playwright `/3` receipts. It does not execute tests,
accept generated intent, or discover runs that were never retained.

```sh
corvint flows coverage --denominator coverage/denominator.json --receipts coverage/runs.json
corvint flows coverage --denominator coverage/denominator.json --receipts coverage/runs.json --offset 0 --limit 20
corvint flows coverage --denominator coverage/denominator.json --receipts coverage/runs.json --write-back /existing/parent/fresh-docs
corvint flows coverage --denominator coverage/denominator.json --receipts coverage/runs.json --check /existing/parent/fresh-docs
```

Both input paths are repository-relative committed files read at captured HEAD, or `--revision FULL_SHA`.
Exit0 means every row is PROVEN, API_PROVEN or validly EXCLUDED; exit1 means a gap or documentation
byte drift; exit2 means invalid/incomplete input or unsafe output. Paging never narrows the evaluated
inventory: every page carries global gap count, verdict, total and full-report digest. Default20,
maximum100 rows per page; reduce the limit if the300KiB page bound is exceeded.

The opt-in `corvint-mcp --tool-profile flows` adds read-only `corvint.flows.coverage` with
`denominator`, `receipts`, `offset`, `limit`. The default profile is unchanged. The same page request
returns the same compiler document as the CLI, inside the MCP evidence envelope. Write-back is CLI-only.

## Inputs and authority

`application-flow-coverage-denominator/1` has `source:{id,revision}` (the existing corpus repository
identity), `intents:{directory,revision}`, `documented_inventory:{kind,path,revision,sha256}`,
`flows:[{documented_id,intent_flow_ids}]` and `variations`.

Inventory kind `generation` strictly rederives a338 `generation.json`; `corpus` strictly Opens the
complete corpus and selects every typed flow subject. Kind `intents` uses the complete loaded AFU
intent set: path equals the intent directory and sha256 hashes deterministic JSON of the sorted
`[]FlowIntent` array (`flowcoverage.Encode(set.Flows)`), without a trailing newline. It does not
require a behavior adapter. Generation input retains256MiB and corpus128MiB ceilings.

Every documented ID occurs exactly once in `flows`; every loaded AFU flow is mapped; every accepted
variation occurs exactly once in `variations`. An empty mapping, empty matrix or proposed intent
stays a gap. Current committed and dirty inventory additions/changes cannot leave an old subset green.
Actor, preconditions, actions, facts and observable outcome records come from the accepted AFU source.
A variation entry is the first selected accepted navigation locator; an optional entry supplement
requires a valid locator and original human review anchor.

Each variation contains `intent_flow_id`, `variation_id`, optional `entry`, `citations`, `tests`
and optional `exclusion`. Citations have exactly `docs`, `claims`, `source_branches`, `legacy_tests`,
`downstream_flows`. Each category has `anchors`, `ids`, `unavailable`: evidence or an explicit missing
reason, never both. Claim IDs resolve through the selected corpus/generator; downstream IDs resolve
through the complete AFU set. Other categories use existing `doccorpus.Anchor` values. Span hashes
retain exact line-ending bytes. Cited source, test, configuration, build, documentation and acceptance
changes produce STALE; source declarations are never runtime witnesses.

An exact test has `file`, `full_title`, `project` (required; empty string is valid), `anchor` and
`assertions`. An asserting contract additionally has `mode` (`browser` for accepted ui, `api` for
accepted api), `outcomes` (accepted IDs), `controls` (different exact tests), `fixtures`, `page_objects`,
and an `acceptance` anchor with authority `source-document`, evidence_kind `review`. These anchors
are declared project-owned review references, not authenticated signatures or machine-understood
proof of the prose. All required outcomes need asserting tests; navigating links cannot satisfy them.

An exclusion is an immutable `{path,revision,sha256}` reference to a closed
`application-flow-coverage-exclusion/1` document: `flow_id`, `variation_id`, `matrix_sha256`
(deterministic JSON of the complete accepted FlowIntent), `source_revision`, `author`, `rationale`,
`generated:false`, `accepted:true`. Exact matrix/source bindings and freshness are mandatory.
A sign-off cannot turn a proposed/empty accepted matrix into green coverage.

## Original receipt binding

`application-flow-coverage-runs/1` has `directory`, `revision`, and `runs`, listing every JSON file in
that committed directory. Keep the inventory outside that directory. Each run carries its immutable
`path`, `revision`, `sha256`, `kind` (`planned` or `manual`), `source_revision`, `test_revision`, exact
reporter-label-to-Git-path `paths` map, ordered `package_paths` labels (package and lock), `build_scope`,
`application_identity`, and declared `environment` values.

All reporter config inputs must be retained, including the provider-generated temporary override and
reporter source. Preserve override bytes from `receipt.external.configOverride` and the exact reporter
bytes; commit them and map their original absolute labels to those immutable paths. Never map by
basename or open a path taken from a receipt. Recompute package identity with original labels/order
and recompute the full served-build directory from immutable regular Git entries. Symlinks/gitlinks
cannot be admitted build entries. Dirty/untracked build additions also invalidate freshness.

PROVEN/API_PROVEN requires exact test identity, original canonical `/3` projections, one observed
attempt and one schedule start at retry0, **configured retries0**, qualified lifecycle/browser tuple,
unchanged test/config/package/build bytes and a passing subject. Its accepted negative control must
be observed failing with `assertion-or-test`, under the same source/test/build/app/config/package/
environment/browser binding group. A run with an expected failing control may have failed aggregate
execution. Infrastructure, cancellation, missing schedules and nonzero configured retry budgets cannot
prove coverage. Comparable byte-group retry/conflicting manual history is FLAKY. Source/test commit IDs and caller app labels remain provenance, but metadata-only commits or relabeling cannot reset identical-byte failure history. Subject/control proof pairing still requires matching declared app identity. Historical groups stay in the
report but do not permanently poison a separately requalified current group.

The bounded result proves byte binding and caller-declared application identity. `/3` does **not**
attest deployed lineage, observe a clean execution checkout, or localize the failing assertion. A
control result means only the accepted declared control was observed failing; its independent
adequacy remains unknown. Missing committed served-build binding stays MISSING_TEST. External server
cleanup remains caller-owned; only provider runner cleanup is observed. These results grant no ETS
exclusion authority and do not prove migration parity.

## Documentation and bounds

Write-back exclusively creates a fresh directory beneath nonsymlink parents. For generation input,
it retains all eight pages per flow and appends escaped E2E coverage to functional-overview.md. The
original manifest becomes `source-generation.json`; `coverage-manifest.json` binds actual changed
page hashes, original generation, denominator, receipt inventory and complete `coverage.json`.
Corpus/intents input writes one coverage page per documented flow. `--check` rederives and compares
all bytes read-only. Human files are never overwritten. Missing-test rows include a skeleton request
with entry/outcomes and revalidated fixture/page-object anchors, with missing discovery explicit.

Limits:512 documented flows,8,192 expanded rows,128 receipts of4MiB each and64MiB aggregate,
64MiB full report,300KiB page,320MiB output package. Generation retains256 flows/2,048 pages;
coverage allows four metadata files. Counts or bytes above limits refuse rather than truncate.
