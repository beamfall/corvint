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

A `corvint-corpus-flow-relations/1` provider declares flow-level relations: `depends-on` another
flow, `produces` or `consumes` a named event or job, and `external` calls to a named system, each
with an optional source anchor and a `reviewed` or `unreviewed` state. With such a provider,
`impact` and `affected` add a `flow_relations` trace that walks reviewed joined relations
downstream from the changed flows, to `--corpus-depth=N` hops (1 to 16, default 3). Each reached
flow keeps its path, such as `A -> job:J -> B`, and its relation IDs. Unreviewed or unjoined
relations are listed in `withheld` and are never traversed. `trace` on a flow, including the
corpus MCP tool `corvint.docs_trace` with its `depth` argument, walks the same relations. The trace
is project-declared corpus data below project authority and never narrows required checks.

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
input was refused, is listed as `not-evaluated` with `blocked_by`. A duplicate or invalid identity
is charged to the first such item that was not already refused. If the number of inputs, mappings,
observations or discovery executions exceeds its bound, those items are not checked one by one.
The report keeps at most 1024 entries, ending with `further entries omitted after <N>` when it drops
any, and caps each message at 4 KiB with a ` … [truncated <N> bytes]` marker. Repairing a refusal
can therefore reveal more. The report is accepted exactly when the build would succeed, and its first refusal is the error
the build would return, under the same 4 KiB cap. The exit status is 0 when accepted, 1 when refused,
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
New behavior declarations should use the `/2` provider (see Multi-repository behavior records), which names repositories only through the neutral `source` and `repositories` members.

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

### Playwright behavior witness walkthrough

This walkthrough connects four steps (DCP-V1-080): the Playwright receipt producer, `discover-playwright`,
committing the evidence, and `witness-playwright`. The result is a set of behavior runs that the
behavior adapter can count in `runtime_witnessed_contracts`. Each command's own help repeats its part:
`corvint docs corpus witness-playwright --help` prints the receipt guidance and the annotation contract.

1. **Publish behavior annotations from the tests.** `witness-playwright` reads only the
   annotations that a test attempt published (see the contract below). The shipped producer,
   `internal/doccorpus/behavior-events.cjs` (DCP-V1-081), publishes them from a Playwright fixture,
   so specs do not spell events by hand:

   ```js
   const { test: base } = require('@playwright/test');
   const test = require('CORVINT/internal/doccorpus/behavior-events.cjs').extend(base);
   test.use({ behaviorFixtures: ['seeded-items'] });
   test('lists items', async ({ page, behavior }) => {
     behavior.cleanup(async () => { /* remove the seeded items */ });
     await page.goto('/items');
     await behavior.expect({ id: 'items-visible', behavior: 'list-items', criterion: 'items-visible' })(page.locator('#count')).toHaveText('2');
     await behavior.negativeControl('empty-hidden')(page.locator('#empty')).toBeHidden();
     await page.locator('#done').click();
   });
   ```

2. **Produce the qualified receipt and the report.** Run
   `corvint-js-test-provider e2e --external-server ... --retain > evidence/receipt.json` exactly as the
   receipt guidance in `corvint docs corpus discover-playwright --help` shows. That run replaces the
   project's reporters, so it writes no report. Produce REPORT.json in a second, caller-run
   Playwright run of the same config file and runner version, against the same running app:
   `npx --no-install playwright test --config playwright.config.ts --reporter=json > evidence/report.json`.
   The behavior annotations are read from this report run, not from the receipt run. The importer
   checks each test's report outcome against the receipt, and credits a test only when both show
   a single first attempt (retry 0) that passed. The report must name the same `configFile` as the
   receipt, otherwise every test is `report-foreign-config`. The provider's `--keep-reporters` mode
   runs Playwright through a generated config, so it is not a qualified way to produce this report.
3. **Discover.** `corvint docs corpus discover-playwright --migration evidence/migration.json --config playwright.config.ts --playwright-list evidence/list.json --receipt evidence/receipt.json > evidence/discovery.json`
   reads the receipt file unchanged and binds each listed test to its receipt identity.
4. **Commit the evidence and write REQUEST.json.** Commit the receipt bytes unchanged at a tracked
   path, beside the migration, discovery and reviewed inventories. Then add the receipt to
   REQUEST.json as one more `inputs[]` entry: an `observed` input with `external-provider`
   authority and a full-file anchor in the provider repository. `document` holds the exact
   receipt bytes as one JSON string, and both `sha256` and `span_sha256` are the SHA-256 of those
   bytes. `start_line` is 1 and `end_line` is the file's last line; the receipt is one line.
   The reviewed flow, variation, candidate and test inputs and their mappings are those of
   the behavior-adapter request above, so they are left out of this sample:

<!-- DCP-V1-080 witness request example -->
```json
{
  "schema": "corvint-behavior-adapter-request/1",
  "provider_id": "behavior",
  "provider_version": "1",
  "contract_id": "items-contract",
  "source": {"id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "revision": "2222222222222222222222222222222222222222"},
  "revisions": {
    "app": {"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "revision": "1111111111111111111111111111111111111111"},
    "PROVIDER_MEMBER": {"id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "revision": "2222222222222222222222222222222222222222"},
    "docs_corpus": {"id": "cccccccccccccccccccccccccccccccccccccccc", "revision": "3333333333333333333333333333333333333333"}
  },
  "source_revision": "2222222222222222222222222222222222222222",
  "documentation_revision": "3333333333333333333333333333333333333333",
  "migration_input": "migration",
  "discovery_input": "discovery",
  "inputs": [
    {
      "id": "receipt",
      "anchor": {
        "repository": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "revision": "4444444444444444444444444444444444444444",
        "path": "evidence/receipt.json",
        "blob": "GIT_BLOB_OF_EVIDENCE_RECEIPT_JSON",
        "sha256": "SHA256_OF_RECEIPT_BYTES",
        "start_line": 1,
        "end_line": 1,
        "span_sha256": "SHA256_OF_RECEIPT_BYTES",
        "symbol": "",
        "region": "",
        "authority": "external-provider",
        "evidence_kind": "observed",
        "reason": "retained qualified Playwright receipt"
      },
      "document": "RECEIPT_BYTES"
    }
  ],
  "mappings": [],
  "observations": []
}
```

   The `migration` and `discovery` inputs have the same shape: the migration is `declared`, and the
   discovery is `observed`. `4444...` is the later commit that retained the inputs; it is not
   `source_revision` (see the self-referencing revision pitfall above).
5. **Witness.** `corvint docs corpus witness-playwright --input evidence/request.json --receipt-input receipt --report evidence/report.json`
   prints one `corvint-behavior-witness-import/1` bundle. `--receipt-input` names the request
   input, not a file. Each entry in `witnesses` carries a behavior run `document` and its
   `observation` link. To credit it, commit each run document unchanged as an `observed` input,
   add the observation link to `observations`, and set the test's `runtime` to
   `{"evidence": <that input's anchor>, "observation": <the link's id>}`. Then run `behavior-adapter` again.

The annotation contract. The importer reads the passing attempt's `annotations` in published order.
Unknown annotation types are ignored.

| Annotation type | Description | Rule |
| --- | --- | --- |
| `corvint-behavior-event` | One closed JSON event | Keys `browser_context`, `page`, `frame`, `navigation`, `parent_page`, `behavior`, `criterion`, `matcher`, `locator`, `value`, `passed`, `sequence`, `kind`, `id`, and no others. `sequence` is 1, 2, 3, ... in publication order. `passed` is true. `browser_context`, `page`, `frame` and `id` are non-empty. `kind:id` is unique. At most 4096 events. |
| `corvint-behavior-cleanup` | `passed` | Exactly one, published after cleanup ran. If none is published, the reason is `cleanup-unobserved`. If the value is `failed`, or more than one is published, the reason is `cleanup-failed`. |
| `corvint-behavior-fixture` | A fixture name | One per registered fixture, equal to the test's `fixtures` in order. |
| `corvint-behavior-role` | A role name | One per registered role, equal to the test's `roles` in order. |

- `kind` is one of `page`, `assertion` or `negative-control`.
- A `page` event names its `navigation`: one of `main-frame`, `frame`, `redirect`, `popup` or
  `setup`. A `popup` also names its `parent_page`.
- An `assertion` event counts only when it equals a registered assertion of the test on `id`,
  `behavior`, `criterion`, `matcher`, `locator` and `value`.
- A run counts as witnessed only if three things hold. Its events must equal the
  `ordered_events` of each flow the test registers. Every registered assertion must have an equal
  event. Every required page and negative control must have been seen.

Reading an unwitnessed test. Each test without a witness names exactly one `reason`. For an
annotation-stage reason, `missing_annotations` lists the needed annotation types that the attempt
never published (DCP-V1-082). Those reasons are `events-unobserved`, `event-malformed`,
`cleanup-unobserved`, `cleanup-failed` and `fixture-role-mismatch`. The types are listed in the
order `events`, `cleanup`, `fixture`, `role`. `fixture` and `role` are listed only when the test
registers fixtures or roles. A malformed or mismatched annotation that was published is not
missing, so `missing_annotations` can be absent even when the reason is an annotation-stage reason.
For example, a test that published nothing at all, with one registered fixture, reports
`events-unobserved` with `missing_annotations` `["events", "cleanup", "fixture"]`.

What the producer records:

- It records an `assertion` only after Playwright's own `expect` matcher passes, and only when the
  attempt gained no error while the matcher ran. The producer resolves `expect` from the project's
  `playwright/test`; a caller cannot supply it. `matcher` is the method called, `locator` is the
  Locator's selector, and `value` is the literal expected argument. The supported matchers are
  `toHaveText`, `toContainText`, `toHaveValue`, `toHaveClass`, `toHaveId`, `toHaveAccessibleName`,
  `toHaveAccessibleDescription`, `toHaveRole` (each with a non-empty string) and `toHaveCount`
  (with a non-negative integer). The only matcher option it accepts is `timeout`.
- The valueless state matchers `toBeVisible`, `toBeHidden`, `toBeEnabled` and `toBeChecked` are
  recorded with the state name as `value`: `visible`, `hidden`, `enabled` and `checked`. Call them
  with no argument or with `{timeout}` only. A state option such as `{visible: false}` or
  `{checked: false}` runs the matcher but records nothing, and `behavior.expect` has no `.not`: use
  `toBeHidden` for an element that must not be visible, or `negativeControl` for a negated check.
  The reviewed outcome declares the same pair, for example matcher `toBeVisible` with value
  `visible`, and `behavior-adapter` compares it byte for byte (`DCP-V1-083`, `DCP-V1-084`).
- In any other case the matcher still runs, but nothing is recorded. That covers a RegExp or array
  expectation, other options, a locator inside a frame, a locator on a page that was not observed,
  and anything that is not a Locator. The producer warns once on stderr.
- `negativeControl(id)` records a `negative-control` event after any `to...` matcher passes,
  including `.not`.
- It records a `page` event for each main-frame commit of an http(s) URL in a page of the test's
  `context` fixture. The `id` is the URL path, so the query and fragment never leave the browser.
  Commits to the same path in a row are recorded once. `navigation` is `popup` for the first commit
  of a page opened by an observed page, `redirect` when the committed request was redirected, and
  `main-frame` otherwise. It never produces `frame` or `setup` navigations.
- `browser_context` is `context-1`, and pages are `page-1`, `page-2`, and so on, in the order
  they were observed. `frame` is `main`.
- At teardown it runs the `behavior.cleanup` callbacks, last registered first. It then publishes
  the fixtures, the roles, the events and one cleanup outcome. The outcome is `passed` only if at
  least one callback was registered and every callback returned. With no callbacks registered it
  publishes no cleanup outcome, so the test stays `cleanup-unobserved`.
- If `corvint-behavior-*` annotations were published outside the producer, or there are more than
  4096 events, it publishes one non-JSON event instead, and the test reports `event-malformed`.

Limits. The producer reads private Playwright fields: Locator `_frame` and `_selector`, and Page
`_opener`. These are qualified on Playwright 1.61.1. When one of them is absent, the producer
records nothing for that page or locator. The importer cannot tell producer annotations from
annotations a test pushed by hand. A path segment that carries a secret is published as part of
the page `id`. Pages in browser contexts other than the `context` fixture are not observed.

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
