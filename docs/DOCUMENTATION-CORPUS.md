# Experimental documentation corpus

[Contract](specs/documentation-corpus-v1.md): proposed DCP-V1. Generated documentation retains
original evidence and explicit unknowns; it does not accept intent or prove behavioral adequacy.
The default binary needs no service, database or network. The MCP companion is separately built.

## Smallest native path

Use a full immutable commit and a literal tracked file or directory (not the repository root `.`).
The explicit timestamp is part of the reproducible input. Shell redirects below are operator writes;
Corvint prints results to stdout. Keep generated artifacts outside the inventoried source scope.

```sh
corvint docs corpus manifest --revision "$(git rev-parse HEAD)" --scope internal/doccompiler/draft.go --timestamp 2026-09-19T00:00:00Z > corpus-input.json
corvint docs corpus build --manifest corpus-input.json > corpus.json
corvint docs corpus search --artifact corpus.json --query DraftSources
corvint docs corpus coverage --artifact corpus.json
corvint docs corpus render --artifact corpus.json
corvint query --task 'DraftSources implementation' --corpus=corpus.json
```

The manifest declares every input path, commit, blob and SHA-256 in each scope before analyzer
admission. Compilation refuses missing/extra inputs, unknown providers, mismatched identities and
recursive generated sources. Unsupported analyzers remain gaps. Native extraction includes indexed
symbols/test declarations, literal Markdown paragraphs and anchored Go imports; it does not infer
runtime call graphs, API behavior or UI journeys. A declaration is never a test execution result.

Read operations share one rederiving reader: `info`, `validate`, `search --query TEXT`, `get`, `trace`,
`related`, `journey`, `stability` (each with `--id ID`), `locate --path PATH`, `coverage`,
`gaps [--id ID]`.
All take `--artifact FILE` and optional `--limit 1..256`. Queries report omissions, absence, valid
no-match, not-found and stale evidence separately. Coverage names its scope/denominator and never
claims universal behavioral coverage. Every read rechecks original immutable inputs; live source
and provider changes are reported separately from the artifact's recorded revision.

## Larger adoption imports

Use `corvint-corpus-input/2` with `corvint-corpus-adoption-provider/1` inputs for the
larger typed import profile. A provider declares either `record` or a sorted `shards` list;
every shard has the same provider ID, version and source repository, and its own pinned manifest
input. Commit shards after their original evidence, then name their exact commit, blob and SHA-256.
All shards are validated together; missing shards, duplicate IDs and dangling joins refuse the build.

The profile preserves typed claims, eight-section flows, stable paragraphs and retirement redirects,
test-link roles/confidence, explicit coverage membership, ticket history and intent comparisons.
See [typed records](../internal/doccorpus/adoption.go) and
[the contract](specs/documentation-corpus-v1.md) for the closed fields and bounds. Supplied-record
parity distinguishes admitted records and restricted drops; it does not infer repository completeness.
Restricted findings belong only in the separate `restricted_findings` input family. Share the
resulting severity summary, never the original local provider input or local detail report.

A shard is limited to 64 MiB, all pinned inputs together and the output to 128 MiB. Collections admit
100,000 subjects, claims and relations each, 4,096 journeys and 100,000 typed details. Each normalized
record plus its detail must fit 1 MiB so a bounded reader can return it. The older `/1`, behavior
adapter and retained receipt profiles keep their existing limits.

```sh
corvint docs corpus build --manifest adoption-input.json > corpus.json
corvint docs corpus inventory --artifact corpus.json --limit 256 --offset 0
corvint docs corpus get --artifact corpus.json --id provider:record-id
```

Use `next_offset` until absent to traverse the immutable inventory, including claims and relations.
The result's artifact digest binds each page; `omitted` counts all records outside that page. Other
`/2` read commands accept the same offset. Pages are at most 256 records and 4 MiB and may contain
fewer rows to meet the byte bound. Citations and typed details describe only that page. Capability
summaries retain counts and explicit omitted-ID totals; the inventory provides those IDs.
The corpus MCP exposes `corvint.docs_inventory` and offset arguments only for `/2`. Native evidence
attachments retain explicit projection omissions and mandatory check fallback.

## Native evidence consumers

Use the inline `--corpus=FILE` option on `query`, `context`, `impact`, `affected`, `test-validity`,
`work observe`, `work propose-wave`, or `cem status/verify/report`. It adds a separately attributed
`documentation` member and preserves native result members. Work evidence carries no task authority.
Impact follows exact source anchors and one explicitly recorded relation; incomplete selection always
requires the repository's full mandatory checks. The corpus never executes tests. Test-validity joins
only exact explicitly supplied `--receipt` bytes; discovery remains unlinked.

`docs corpus cem --artifact FILE --cem MAP [--id CLAIM]` emits original evidence citation arguments
and a sidecar binding the corpus and CEM digests. Only exact CEM-base Git spans qualify. Keep the
sidecar with the CEM for generated-documentation provenance; the frozen CEM wire cannot carry that
extra authority field. Applying citations remains an explicit existing `cem cite` operation.

## Providers and retained observations

External tools can emit `corvint-corpus-provider/1` JSON using the generic subject/relation vocabulary
in [the types](../internal/doccorpus/types.go). Declare provider implementation version and artifact
commit separately from the earlier source commit. Commit retained observations before the provider
record so their digests and revisions can be named without self-reference. Add each artifact's exact
scope and manifest input, with purpose `provider`, `observation` or `evidence`. At most eight revisions
and sixteen providers are accepted. Core never invokes a provider or imports a network URL.

Each normalized record has a provider-qualified ID and anchored evidence or an explicit unknown.
Anchors include repository/commit/path/blob/content hash, inclusive line span/span hash, authority,
kind and inclusion reason. Closed schemas reject unknown/duplicate fields and record IDs. Provider
claims remain declarations: source identity checks do not authenticate provider honesty. Reported
review remains attributed with effective generated trust; providers cannot self-declare verification.

Native observations use actual Go preview-session or JS unit/E2E receipt inputs, decoded and projected
by `testvaliditydoc`. Exact run digest, package/test and source bindings are required. Original absolute
source keys may be mapped explicitly using `source_paths`; no suffix guessing occurs. Opaque Go session
and E2E app-build identities remain unknown. Preview evidence remains non-promotable; failed, skipped,
flaky, incomplete and stale results retain their native axes.

Recorded journey verification additionally requires a separate complete ordered step artifact with
schema `corvint-corpus-journey-observations/1`. It binds the exact run digest, source revision and test,
cleanup and every action/operation/expected/observed result in order. Every step joins the same run.
A passed test name alone cannot qualify a journey. The positive conformance fixture is synthetic
non-UI evidence; no real browser verification is claimed. See the independent
[flow adapter](../examples/documentation-corpus/README.md) for importing declared product flows
without adding application vocabulary to Core.

### Behavior-provider production and reconciliation

`corvint docs corpus behavior-adapter --input REQUEST.json [--previous RESULT.json]` maps
caller-owned reviewed flow/variation, source-candidate and test inventories into the experimental
`corvint-corpus-behavior-provider/1` profile. The request uses
`corvint-behavior-adapter-request/1`: each mapping names an input ID, an RFC-6901 record-list pointer
and relative field pointers. Each globally stable variation ID carries explicit preconditions,
ordered actions, observable facts, expected outcomes, allowed projects and exact tests. Every test
names the same variation in a closed semantic claim. Outer scalar and list names are configurable;
anchors, assertions, ordered events, behavior runs and observation links retain their closed issue-40
shapes. Each input carries its exact JSON text as a string plus a full-file Git anchor whose SHA-256
matches those bytes.

Add `--check` to validate a request. The command neither emits nor writes a result or artifact; it
prints one `corvint-behavior-adapter-check/1` report. It lists every build stage in build order, each as
`passed`, `refused`, `not-evaluated` or `not-applicable`, and lists every refusal it can determine in
that same order. Inputs, mappings, observations, mapped records and discovery executions are items,
and each item is checked independently of the others. Within one item, checking stops at the item's
first refusal, and one `not-evaluated` entry says `remaining checks for <item> not evaluated after
<refusal>`. A stage or item that depends on a refusal, such as the record checks of a mapping whose
input was refused, is listed as `not-evaluated` with `blocked_by`. Repairing a refusal can therefore
reveal more. The report is accepted exactly when the build would succeed, and its first refusal is
the error the build would return. The exit status is 0 when accepted, 1 when refused,
and 2 when the request or previous file cannot be read.

Input anchor placement: every input anchor, including the migration, discovery, inventory, runtime
and receipt inputs, must name the provider repository. Its `repository` must equal the request
`source.id`. `source` must equal the provider (end-to-end) entry of `revisions`. An input retained
in the application or documentation repository is refused with
`supply immutable Git identities from the provider repository`. Application and documentation
commits appear only as identities inside `revisions` and inside the mapped evidence anchors.

The migration input is a minimal schema-2 migration identity record. It holds exactly five members
and must equal the request: `schema` is `2`, and `contract_id`, `source_revision`,
`documentation_revision` and `revisions` repeat the request values. All six repository identities
and revisions are full Git object IDs. In the example below, `PROVIDER_MEMBER` stands for the
provider-repository member name declared on `BehaviorRevisions.E2E` in
`internal/doccorpus/behavior.go`. That /1 name is frozen, and the adapter refuses any other spelling.

<!-- DCP-V1-045 migration example -->
```json
{
  "schema": 2,
  "contract_id": "checkout-behavior",
  "source_revision": "2222222222222222222222222222222222222222",
  "documentation_revision": "3333333333333333333333333333333333333333",
  "revisions": {
    "app": {"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "revision": "1111111111111111111111111111111111111111"},
    "PROVIDER_MEMBER": {"id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "revision": "2222222222222222222222222222222222222222"},
    "docs_corpus": {"id": "cccccccccccccccccccccccccccccccccccccccc", "revision": "3333333333333333333333333333333333333333"}
  }
}
```

Avoid the self-referencing revision pitfall. `source_revision` and the provider entry of `revisions`
name the provider commit the record describes, and that commit cannot contain the record. Commit the
record, and the other inputs, in a later provider commit. Their input anchors name that later commit,
while `source_revision` keeps naming the described one.

The `corvint-behavior-adapter-result/1` response contains the provider, its normalized normative
variation projection, canonical test semantic claims, retained migration, discovery, runtime and
qualified-receipt inputs, separate coverage rows, a reconciliation frontier and an optional
prior-result delta. Prior results from earlier revisions of the same repository identities are accepted
only after stable lineage, self-consistent historical revisions, artifact and contract digests,
uniqueness and semantic-link validation. Publish the referenced
sidecars before the provider and declare all of them in the corpus manifest. The adapter compares
semantic claims, assertions, pages/events/controls, projects and structurally qualified witnesses in
both directions; the normal corpus build remains responsible for Git rebinding and final receipt
qualification. Source/test proposals never enter or rewrite the documented denominator. Empty,
unreviewed, stale, contradictory or partially witnessed input remains unknown or unreviewed, and
every result retains `full-relevant-suite` fallback with no narrowing authority.

The optional `corvint-corpus-behavior-stability-provider/1` profile adds digest-bound repeated
Playwright evidence without changing behavior-contract coverage. `stability --id ID` returns the
selected one-spec, feature-batch or suite policy identity, separate raw outcome/cleanup/retry counts,
and every immutable contributing receipt and attempt. Planned repetitions, retries and manual reruns
remain distinct. Missing, duplicate, stale, cross-revision or contradictory contributors refuse;
failed cleanup or any threshold miss yields `not-stable`. The repository policy and each observed run
also bind exact CI node/shard, Playwright worker, database mode, project-set, split-version and resource
topology; any declared-versus-observed mismatch refuses qualification before counting. A stability
report is repeated-run evidence, not test adequacy, behavior parity or selection authority.

## Rendering and maintenance

Set manifest `profile.format` to `markdown` or `json`, and optional `profile.groups` to generic subject
kinds (`all` is the default). `render` emits a deterministic filename-to-content plan; it writes no
output directory. Profiles are data, never executable templates. Markdown includes generated markers,
states, claims, directed relations, journey/observation evidence, source citations and gaps.

`docs corpus maintain --artifact FILE --page PAGE.md` previews one Markdown block. Add `--apply` to
write that freshly rederived block explicitly. It preserves human bytes and file permissions, and
refuses stale evidence, accepted intent, malformed/duplicate markers, tampered blocks and unsafe
paths. The page must already exist. Serialized previews cannot authorize a later write.
Apply retains the original inode at the returned `recovery_path`, including late writes from an
editor that held it open. Keep that file until other editors have closed it; removing recovery files
is a separate operator action. Publication uses a pinned directory and never overwrites a competing
new page. There is a brief absent-path window; this is not a crash-atomic filesystem transaction.

## Separate MCP profile

Build `./cmd/corvint-corpus-mcp` explicitly and start it with `--root /absolute/repository --artifact
corpus.json`. It serves stdio only. Validated capabilities gate ten possible read tools: `docs_info`,
`docs_search`, `docs_get`, `docs_locate`, `docs_find_related`, `docs_coverage`, `docs_gaps`,
`docs_get_journey`, `docs_get_stability`, `docs_trace`, each prefixed `corvint.`. Present-zero capabilities are callable;
absent capabilities are not advertised. Calls revalidate source/artifact identity. Changing the
configured artifact requires restarting the server. Model-facing text uses the repository-data
envelope; terminator collisions refuse. Structured receipts match the CLI reader.

Corpora contain repository text and paths. Keep them local according to repository policy. There is
no automatic collection, telemetry, publication, database, daemon, test execution or authority upgrade.
The small author-labelled evaluation measures only its frozen synthetic fixture and reports raw
latency/bytes; it provides no external usefulness, savings or HDC qualification claim.

## Multi-repository behavior records

Use `corvint docs corpus behavior-provider --input REQUEST.json` for the bounded `/2` declaration
profile. Supply the closed request described in [the corpus spec](specs/documentation-corpus-v1.md#multi-repository-behavior-declarations-issue-330),
commit the emitted provider, and list it as a `records` input in the corpus manifest. Set
`behavior_repositories` to the independently expected repository root commits and revisions.
The corpus reports stale/missing members and unavailable external evidence as gaps. It preserves
full-suite fallback and does not qualify runtime claims. The `/1` behavior adapter remains unchanged.
