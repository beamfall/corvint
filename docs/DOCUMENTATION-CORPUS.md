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

`manifest` takes repeated `--scope PATH` values and emits one manifest with one scope per path in
byte order; a duplicate or nested scope is refused. The default `corvint-corpus-input/1` manifest
holds at most 4,096 files; `--schema 2` emits `corvint-corpus-input/2`, bounded by its 100,000
records. A refused inventory names the bound, the scopes and the file count.

Read operations share one verified reader: `info`, `validate`, `search --query TEXT`, `get`, `trace`,
`related`, `journey`, `stability` (each with `--id ID`), `locate --path PATH`, `coverage`,
`gaps [--id ID]`.
`search --include claims` (MCP `include: ["claims"]`) adds claim hits to the subject hits, ranked
together and labelled in `result_kinds`; it does not restrict the result to claims. For claims only,
use the `claims` operation (`corvint.docs_claims`).
All take `--artifact FILE` and optional `--limit 1..256`. Queries report omissions, absence, valid
no-match, not-found and stale evidence separately. Coverage names its scope/denominator and never
claims universal behavioral coverage. A flow's `missing-asserting-e2e-tests` gap names the failed
condition of its closest test link; a link that fails only because its review is provider-reported
(attributed, not authenticated) reports `asserting-e2e-tests-unauthenticated-review` instead
(`DCP-V1-150`..`152`). A read verifies the artifact's canonical bytes, content digest, builder,
manifest and profile digests and pinned revisions, then decodes it without rederiving it
(`DCR-V1-410`); `info` reports `validation: artifact-verified`. A development binary whose builder
label is dirty or unrecorded proves no rebuild equivalence, so it always rederives. An artifact built by
another attested Corvint revision stays readable while the builder schema, derivation version and Go
toolchain in its builder engine match the running binary's; when the derivation version changed, the
refusal names both versions and the corpus must be rebuilt (`DCX-V1-471`). Rebuild it from the manifest
it embeds, without re-supplying inputs: `corvint docs corpus build --artifact corpus.json > corpus.new.json`
then `mv corpus.new.json corpus.json` (redirecting onto the file being read truncates it first). The
rebuild checks the artifact's content and manifest digests and every pinned input's blob and SHA-256
in Git; a read never rebuilds, and a running corpus MCP server asks to be restarted on the new file
(`DCX-V1-495`..`498`). `validate`, and `--verify-rederive`
on any read, rederive every record from the original pinned inputs (`source-rederived`,
`DCR-V1-413`). Live source and provider changes are observed from Git status and the tree listing
and reported separately from the artifact's recorded revision (`DCR-V1-411`). `--timings` writes one
phase summary (parse, verify, freshness, index, query, encode) to stderr (`DCR-V1-415`).
The CLI streams the artifact file through that check instead of holding it whole (`DCR-V1-431`).
A native read joined with `--corpus=FILE` takes the same verified read (`DCR-V1-432`), as does
the current side of `delta` unless `delta --verify-rederive` rederives it (`DCR-V1-460`).
Producers that write records onward (`docs flows generate`, flow coverage,
the indexed companion build, republish and `carry-forward`) still rederive.

## Larger adoption imports

Use `corvint-corpus-input/2` with `corvint-corpus-adoption-provider/1` inputs for the
larger typed import profile. A provider declares either `record` or a sorted `shards` list;
every shard has the same provider ID, version and source repository, and its own pinned manifest
input. Commit shards after their original evidence, then name their exact commit, blob and SHA-256.
All shards are validated together; missing shards, duplicate IDs and dangling joins refuse the build.

A record or shard with many anchors may use the experimental compact
`corvint-corpus-adoption-provider/2` encoding instead: a `files` table states each file's
`path`, `blob` and `sha256` once (`repository` and `revision` default to the record's
`source`), each anchor names its entry by zero-based `file` index with only its span, span hash
and optional `symbol` or `region`, and optional `anchor_defaults` supply a shared `authority`,
`evidence_kind` and `reason`. The build expands it to the `/1` record and admits that unchanged;
on #1664's shape the anchors cost about half the bytes (`DCX-V1-487..489`).

The profile preserves typed claims, eight-section flows, stable paragraphs and retirement redirects,
test-link roles/confidence, explicit coverage membership, ticket history and intent comparisons.
It also gives supplied records a typed home: `gaps` (producer gap claims, returned by `docs_gaps`
with `origin: supplied`; a gap may target a subject or a claim, such as a run's `test_run`
claim), a `status` claim detail (`blocked`, `ready`, `proven`, `partial`, `not-demonstrable`,
`disputed`, `obsolete`, `not-established` or `unknown`, with an optional `provisional` or
`classification-only` qualifier, counted by `docs_coverage` under `STATE` or `STATE:QUALIFIER`, so
a qualified `proven` never counts as plain `proven`), and `manual_case` subjects linked by
`manual_case_for` relations, which satisfy the `missing-manual-cases` gap. A `test_run` claim
detail carries a supplied test-run receipt (`outcome`, `run`, `origin: supplied`, and optional
`retries`, `flake_verdict`, `cleanup`, `control_outcome` and `validity` axes), returned by
`docs_get` and counted per outcome, flake verdict and cleanup by `docs_coverage`. A
`migration_mapping` relation joins two `inventory` units of a code-migration ledger with a closed
`outcome` and `basis`; `docs_get` on a unit returns its mapping chain. A `legacy_case` detail on a `manual_case` subject names its external case ID
(`C123`), so `docs_get C123` returns that subject and its links, or a `legacy-case-unlinked` gap
when no subject declares the case. The `gaps` capability and the query index list supplied gaps.
Supplied records stay attributed and never raise trust.
See [typed records](../internal/doccorpus/adoption.go) and
[the contract](specs/documentation-corpus-v1.md) for the closed fields and bounds. Supplied-record
parity distinguishes admitted records and restricted drops; it does not infer repository completeness.
Restricted findings belong only in the separate `restricted_findings` input family. Share the
resulting severity summary, never the original local provider input or local detail report.

A shard is limited to 64 MiB, the provider record shards together to 128 MiB, the other pinned inputs
together to 128 MiB, and the output to 128 MiB; a refusal names the largest contributors. Collections admit
100,000 subjects, claims and relations each, 4,096 journeys and 100,000 typed details. Each normalized
record plus its detail must fit 1 MiB so a bounded reader can return it. The older `/1`, behavior
adapter and retained receipt profiles keep their existing limits.

Each evidence object holds at most 64 anchors and 64 limitations (`DCI-V1-012`); more refuse the
build with `evidence bound exceeded`. A flow that cites more files declares one `implements`
relation from each further module subject to the flow, each relation carrying its own anchor;
module ownership counts those modules as owned.

`docs corpus manifest` no longer refuses a scope holding inputs over 4 MiB. Each such file is
pinned by blob and size in `over_bound_inputs`, never read, and the build reports it as an
`input-over-bound` gap naming its size and the bound, while the rest of the scope builds
(`DCI-V1-024`). When native
symbol and test declarations crowd the 128 MiB output bound, an ownership-only corpus can keep
just the module subjects with `"derive": {"kinds": ["module"], "native": true}` on the first
native provider; `paths` and `kinds` then also restrict its own inputs, and one
`native-derivation-filter` gap names what was excluded (`DCS-V1-590`).

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

A compiled input can also record where its upstream source stood when it was compiled, as
`"as_of": {"source": "ledger", "kind": "sequence", "value": "42", "probe": "upstream/ledger.seq"}`
(`DCP-V1-574..578`). `kind` is `timestamp`, `sha256` or `sequence`. The optional `probe` is one
local file that the live reader compares with `value`. A live reader marks the input `stale`, with
its lag, when the probe has moved past `value`. It marks the input `unknown` when there is no
probe or the probe cannot be read. Every answer then names that input in its limitations and is
never reported `fresh`. The `info` operation lists each declared input's state and lag as
`inputs_as_of`, so you can tell from it when to republish.

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
qualified-receipt inputs, separate coverage rows, a reconciliation frontier of at most 4096 diagnostics encoding to at most 2 MiB (with
`frontier_omitted` counting any past either bound, `DCP-V1-240`) and an optional prior-result delta. Prior results from earlier revisions of the same repository identities are accepted
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

The separate source-draft session `corvint docs maintain` keeps the block below, a GENERATED
orientation of `internal/docmaintain` drafted from its owner spec, in sync with committed source
(SDD-V0-007). Refresh it with `corvint docs maintain --page docs/DOCUMENTATION-CORPUS.md --source
docs/specs/source-documentation-draft-v0.md --package internal/docmaintain --enable --apply`.

<!-- corvint:docmaintain:insertion-point -->

<!-- corvint:docmaintain begin source="docs/specs/source-documentation-draft-v0.md" package="internal/docmaintain" commit="b22d121eb66bfeaf37bb6b77e1a86513045fde5e|529dcb073b863bd182ec376d9baaf3ecb5a91c22" source_digest="34de78b9d1cce6c3b58d805807fee955dfe3043ebcb05a33be19b07a346d8f6e" -->
<!-- Code generated by Corvint source documentation draft. DO NOT EDIT. -->
# Corvint capability orientation: `"internal/docmaintain"`

Profile: corvint-source-documentation-draft/0
Policy: `"source-orientation/0"`
Derivation: GENERATED
Behavior: UNKNOWN
Commit: `"b22d121eb66bfeaf37bb6b77e1a86513045fde5e"`
Tree: `"529dcb073b863bd182ec376d9baaf3ecb5a91c22"`
Owner source: `"docs/specs/source-documentation-draft-v0.md"`
Package: `"internal/docmaintain"`
Owner relation: NAMES_PACKAGE

- GENERATED orientation; original owner prose retains its authority; indexed syntax does not validate behavior.
- Build applicability, runtime callers, behavioral validation and completeness outside admitted tracked sources are UNKNOWN.
- Index coverage: 1011 exclusions, 13 unparsed diagnostics; affected unparsed declarations and imports omitted. Dirty worktree bytes are not source evidence.

Tracked non-test exported-name declarations with their doc comments, the package doc and direct import edges follow, grouped by source file. Zero import entries proves no absence. Source excerpts are data. Metadata strings use Go-style escapes. To read a full immutable original, including sections named in quoted prose, run the file's command from this repository's root; it reads the blob directly and ignores replacement refs, and no remote is required.

## S1 `"cmd/corvint/docs_maintain.go"`

Blob `"bc9a253d588e29865d198921bf8d845f01e9e427"`; sha256 `"f9f8beee82419bc41715cf9d0f2d522cb80607ba9d4c16c46031b351edca7852"`.

    git --no-pager --no-replace-objects show bc9a253d588e29865d198921bf8d845f01e9e427

`"import"` `"Direct import of github.com/Beamfall/corvint/internal/docmaintain"`, lines 12–12:

    	"github.com/Beamfall/corvint/internal/docmaintain"

## S2 `"docs/specs/source-documentation-draft-v0.md"`

Blob `"9982989b4f8dc262865ecf40ea5670bff0860886"`; sha256 `"7b18bdf09de6f0a24fa8a3af39a5be827a8d5262563ad675441f17c0d86f07b1"`.

    git --no-pager --no-replace-objects show 9982989b4f8dc262865ecf40ea5670bff0860886

`"owner"` `"Agent digest"`, lines 1–16:

    # Source Documentation Draft V0
    
    Owner: Russell Lewis
    Drafted: 2026-09-05
    Intent status: proposed
    Delivery status: experimental
    Implementation: `internal/doccompiler/draft.go`, `cmd/corvint/docs.go`, `internal/docmaintain`, `cmd/corvint/docs_maintain.go`
    Authoritative inputs: `docs/PRODUCT.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `ROADMAP.md` AT-15
    
    ## Agent digest
    - Claim: Corvint drafts source-pinned Markdown from owner prose and indexed symbol-extracted declarations, then consumes its bytes after original-source rederivation.
    - Status: proposed/experimental
    - Exists: bounded CLI draft/consume and explicitly enabled foreground maintenance development slice.
    - Blocked on: owner acceptance and independent outcome validation; full AT-15 maintenance and HDC qualification remain open.
    - Read next: Requirements; CLI contract; Acceptance and rollback.
    

## S3 `"internal/companionrelease/core_evidence.go"`

Blob `"b7b09fe46c29b9448148ef91d7f80fba21c3634e"`; sha256 `"25ef07cd95ef36c35733b7f023888ad745cd582005f0be2b8fe12c8f4ce61d8a"`.

    git --no-pager --no-replace-objects show b7b09fe46c29b9448148ef91d7f80fba21c3634e

`"import"` `"Direct import of github.com/Beamfall/corvint/internal/docmaintain"`, lines 13–13:

    	"github.com/Beamfall/corvint/internal/docmaintain"

## S4 `"internal/docmaintain/blinded.go"`

Blob `"1fc538d9a359129ce3ec984f0c09142be9297634"`; sha256 `"adf780c1bda707a54d02d72f1c64caae1e8bc342559d60fad02fbb7d8f502f1a"`.

    git --no-pager --no-replace-objects show 1fc538d9a359129ce3ec984f0c09142be9297634

Lines 15–15:

    	BlindedTaskSetProfile   = "corvint-docs-blinded-task-set/0"

Lines 16–16:

    	BlindedKeyProfile       = "corvint-docs-blinded-answer-key/0"

Lines 17–17:

    	BlindedResponsesProfile = "corvint-docs-blinded-responses/0"

Lines 18–18:

    	BlindedResultProfile    = "corvint-docs-blinded-result/0"

Lines 21–35:

    // BlindedTaskSet is what the answerer sees: the reference to generate and the questions. It never
    // carries answers.
    type BlindedTaskSet struct {
    	Profile string `json:"profile"`
    	// Reference names the generated reference: the docs draft selector at an immutable revision.
    	Reference struct {
    		Source   string `json:"source"`
    		Package  string `json:"package"`
    		Revision string `json:"revision"`
    	} `json:"reference"`
    	AnswerKeySHA256 string            `json:"answer_key_sha256"`
    	PassNumerator   int               `json:"pass_numerator"`
    	PassDenominator int               `json:"pass_denominator"`
    	Questions       []BlindedQuestion `json:"questions"`
    }

Lines 37–41:

    // BlindedQuestion is one declared question.
    type BlindedQuestion struct {
    	ID   string `json:"id"`
    	Text string `json:"text"`
    }

Lines 43–47:

    // BlindedKey holds, per question, the facts a correct answer must state.
    type BlindedKey struct {
    	Profile string              `json:"profile"`
    	Answers map[string][]string `json:"answers"`
    }

Lines 49–56:

    // BlindedResponses is one answerer's run: who or what answered, under which condition, and the
    // answers by question ID.
    type BlindedResponses struct {
    	Profile     string            `json:"profile"`
    	Participant string            `json:"participant"`
    	Condition   string            `json:"condition"`
    	Answers     map[string]string `json:"answers"`
    }

Lines 58–63:

    // BlindedScore is one question's mechanical score.
    type BlindedScore struct {
    	ID      string   `json:"id"`
    	Correct bool     `json:"correct"`
    	Missing []string `json:"missing"`
    }

Lines 65–77:

    // BlindedResult is the retained record of one scored run.
    type BlindedResult struct {
    	Profile         string         `json:"profile"`
    	TaskSetSHA256   string         `json:"task_set_sha256"`
    	AnswerKeySHA256 string         `json:"answer_key_sha256"`
    	ResponsesSHA256 string         `json:"responses_sha256"`
    	Participant     string         `json:"participant"`
    	Condition       string         `json:"condition"`
    	Correct         int            `json:"correct"`
    	Total           int            `json:"total"`
    	Verdict         string         `json:"verdict"`
    	Scores          []BlindedScore `json:"scores"`
    }

Lines 79–84:

    // ScoreBlinded applies the scoring rule: a question is correct when its answer states every
    // required fact of the key (case-insensitive, whitespace-collapsed substring); a missing answer
    // is incorrect. The run passes when correct*PassDenominator >= total*PassNumerator. It refuses a
    // key whose SHA-256 differs from the one the task set committed to, a key or response of the
    // wrong profile, and a key that does not cover exactly the declared questions.
    func ScoreBlinded(taskSet, key, responses []byte) (BlindedResult, error) {

## S5 `"internal/docmaintain/docmaintain.go"`

Blob `"c09942da5fcbddd5524e93676257f07f5a9c7862"`; sha256 `"d1430ba39296f7ae3429048d0b616569d75faf93f4ea36c878eb5a6333820cd5"`.

    git --no-pager --no-replace-objects show c09942da5fcbddd5524e93676257f07f5a9c7862

`"package-doc"` `"package docmaintain"`, lines 1–14:

    // Package docmaintain implements an explicitly enabled, bounded local session
    // that keeps one human documentation page's generated blocks in sync with
    // immutable Git source, per the IPR-05 slice of
    // docs/plans/integrated-product-roadmap-2026-09-12.md and the maintenance
    // requirements in docs/specs/source-documentation-draft-v0.md (SDD-V0-007+).
    //
    // A session never renders a document (docs/specs/human-documentation-compiler-v0.md
    // remains partial (HDCV0-023..029, HDCV0-041)) and never promotes generated prose to accepted intent
    // (AGENTS.md invariant 8); it only refreshes source-pinned Markdown blocks that
    // the same native corvint.docs_draft compiler
    // (internal/doccompiler.DraftSources over internal/contextindex.BuildContext)
    // already produces, inside explicit begin/end markers, leaving every other
    // byte of the page untouched.
    package docmaintain

Lines 30–35:

    // Selector names one (owner Markdown source, Go package directory) pair, the
    // same shape corvint.docs_draft accepts (docsbridge.draftInput).
    type Selector struct {
    	Source  string
    	Package string
    }

Lines 37–47:

    // Policy bounds one session. Enabled must be true or Run refuses outright.
    // Apply authorizes writing; false always previews only, regardless of what
    // eligible changes are found. MaxWrites and MaxWallClock bound how much of a
    // multi-selector session executes before it stops cleanly.
    type Policy struct {
    	Enabled      bool
    	Apply        bool
    	MaxWrites    int
    	MaxWallClock time.Duration
    	pageLimit    int
    }

Lines 49–60:

    // BlockOutcome reports what happened to one selector's generated block.
    type BlockOutcome struct {
    	Selector  Selector
    	Existed   bool // a generated block for this selector was already in the page
    	Eligible  bool // its content digest differs from the page's recorded digest (or it did not exist)
    	Applied   bool // this session actually rewrote it
    	Skipped   string
    	OldSHA256 string
    	NewSHA256 string
    	Commit    string
    	Tree      string
    }

Lines 62–78:

    // Receipt is the durable session record: source identities, page digest
    // before/after, and whether the session previewed or applied.
    type Receipt struct {
    	Page               string
    	Selectors          []Selector
    	StartedAt          time.Time
    	FinishedAt         time.Time
    	PageExistedAtStart bool
    	PageDigestBefore   string
    	PageDigestAfter    string
    	Applied            bool
    	PreviewOnly        bool
    	Conflict           bool
    	ConflictDetail     string
    	Blocks             []BlockOutcome
    	StoppedReason      string // "", "max-writes", "max-wall-clock"
    }

Lines 80–86:

    // Result is what Run returns: the proposed page content, a unified diff
    // against the bytes read at session start, and the session Receipt.
    type Result struct {
    	ProposedPage []byte
    	Diff         string
    	Receipt      Receipt
    }

Lines 88–91:

    // Refusal is a closed, sanitized session refusal (disabled policy, invalid
    // selector, unauthorized apply, or a write-time conflict). It never carries
    // underlying process output; Message is a closed library message, such as a draft limit's bound and size.
    type Refusal struct{ Code, Message string }

Lines 93–93:

    func (r *Refusal) Error() string { return r.Code }

Lines 97–105:

    // Preview computes, but never writes, the proposed page content for page
    // (repository-relative to root) over selectors, in order: it detects which
    // generated blocks are eligible for refresh (missing, or whose recorded
    // markdown_sha256 differs from a fresh redraft), skips ineligible/unchanged
    // blocks untouched, and stops accepting further writes once policy.MaxWrites
    // or policy.MaxWallClock is reached — remaining selectors are recorded as
    // skipped, not silently dropped. It requires policy.Enabled but never checks
    // or requires policy.Apply.
    func Preview(ctx context.Context, root, page string, selectors []Selector, policy Policy) (*Result, error) {

Lines 211–217:

    // Apply authorizes and performs the write a prior Preview proposed. It
    // refuses unless policy.Apply is true, and it refuses — writing nothing — if
    // the page's bytes on disk no longer equal exactly what Preview started
    // from: that is the session's whole conflict boundary, and it never applies
    // part of a proposal. Applying a Preview that found nothing eligible is a
    // harmless no-op (Applied stays false).
    func Apply(root string, preview *Result, policy Policy) (*Result, error) {

Lines 266–270:

    // Run is the convenience path: Preview, then Apply when policy.Apply is
    // true. Callers that need to inspect or gate the preview before authorizing
    // the write (e.g. an operator confirmation step) should call Preview and
    // Apply directly instead.
    func Run(ctx context.Context, root, page string, selectors []Selector, policy Policy) (*Result, error) {

Lines 390–392:

    // Summary is a small human-readable render used by callers (CLI, evidence)
    // that want a one-line-per-block summary without re-deriving it.
    func Summary(r Receipt) string {

## S6 `"internal/docmaintain/examples.go"`

Blob `"25184342b0f52658ff62028ec1c14e3826543475"`; sha256 `"b44d37815d901092237ec3211105ccf51e2438625abdbf0eb534a22fa74a8522"`.

    git --no-pager --no-replace-objects show 25184342b0f52658ff62028ec1c14e3826543475

Lines 35–36:

    // ExampleCheckProfile names the closed example-check report.
    const ExampleCheckProfile = "corvint-docs-example-check/0"

Lines 40–40:

    	ExamplePass   = "PASS"

Lines 41–41:

    	ExampleFail   = "FAIL"

Lines 42–42:

    	ExampleNotRun = "NOT_RUN"

Lines 44–44:

    	ExampleMalformed      = "example-malformed"

Lines 45–45:

    	ExampleInBlockBody    = "example-in-generated-body"

Lines 46–46:

    	ExampleUnboundBlock   = "example-unbound-block"

Lines 47–47:

    	ExamplePackageInvalid = "example-package-invalid"

Lines 50–57:

    // ExampleDecl is one declared example: the block it belongs to (Source, Package) and the Go
    // example function name, with the 1-based page line that declares it.
    type ExampleDecl struct {
    	Source  string `json:"source"`
    	Package string `json:"package"`
    	Name    string `json:"name"`
    	Line    int    `json:"line"`
    }

Lines 59–63:

    // ExampleProblem is a declaration line that cannot be checked; it fails the check, never silently.
    type ExampleProblem struct {
    	Line int    `json:"line"`
    	Code string `json:"code"`
    }

Lines 65–69:

    // ExampleResult is one declared example's observed outcome.
    type ExampleResult struct {
    	ExampleDecl
    	Status string `json:"status"`
    }

Lines 71–78:

    // ExampleReport is the closed result of one CheckExamples run. Verdict is PASS only when every
    // declaration parsed, bound to a block on the page, and its example passed.
    type ExampleReport struct {
    	Profile  string           `json:"profile"`
    	Verdict  string           `json:"verdict"`
    	Results  []ExampleResult  `json:"results"`
    	Problems []ExampleProblem `json:"problems"`
    }

Lines 80–83:

    // ExampleRunner runs the named examples of one package directory (relative to root) through an
    // existing test provider and returns each observed name's state (ExamplePass or ExampleFail). A
    // name absent from the map was not run.
    type ExampleRunner func(ctx context.Context, root, pkg string, names []string) (map[string]string, error)

Lines 85–89:

    // DeclaredExamples reads the example declarations of page without running anything. It returns
    // the page refusal code from scanPage when the page's blocks are malformed. A declaration must sit
    // outside fences and block bodies, parse exactly, name a generated block present on the page, and
    // name a package directory that is relative and inside the repository.
    func DeclaredExamples(page []byte) ([]ExampleDecl, []ExampleProblem, string) {

Lines 172–175:

    // CheckExamples runs every example declared on page through run, one call per package, and
    // reports each declaration's state. It is an operator check, never called by Preview, Apply,
    // Watch or docs consume.
    func CheckExamples(ctx context.Context, root string, page []byte, run ExampleRunner) (ExampleReport, error) {

Lines 227–230:

    // GoTestRunner is the ExampleRunner backed by the Go toolchain's own test provider: it runs
    // `go test -count=1 -json -run '^(A|B)$' ./pkg` under root and reads each example's pass or fail
    // event. Go checks an example's `// Output:` comment; an example without one only compiles.
    func GoTestRunner(goBin string) ExampleRunner {

Lines 300–300:

    func (l *limitedWriter) Write(p []byte) (int, error) {

## S7 `"internal/docmaintain/watch.go"`

Blob `"609fcc24d5bfa2c7d58816ac748582073165c838"`; sha256 `"6c118dc736bf0850ab20fab4133299fdc5a28342c84de27701e9dc4b42459764"`.

    git --no-pager --no-replace-objects show 609fcc24d5bfa2c7d58816ac748582073165c838

Lines 18–25:

    // WatchSummary retains source identity, never a tick's page or diff.
    type WatchSummary struct {
    	Commit       string `json:"commit"`
    	Tree         string `json:"tree"`
    	SourceDigest string `json:"source_digest"`
    	PageDigest   string `json:"page_digest"`
    	Status       string `json:"status"`
    }

Lines 27–36:

    // WatchReceipt is a bounded aggregate for the explicit foreground profile.
    type WatchReceipt struct {
    	Profile          string         `json:"profile"`
    	Cycles           int            `json:"cycles"`
    	Writes           int            `json:"writes"`
    	StoppedReason    string         `json:"stopped_reason"`
    	Complete         bool           `json:"complete"`
    	Summaries        []WatchSummary `json:"summaries"`
    	OmittedSummaries int            `json:"omitted_summaries"`
    }

Lines 49–51:

    // Watch refreshes one selector until its explicit bounds or a conflict stop it.
    // It never adopts externally changed page bytes as the next cycle's baseline.
    func Watch(ctx context.Context, root, page string, selector Selector, policy Policy) (*WatchReceipt, error) {

## S8 `"tools/docs-blinded-task/main.go"`

Blob `"82d76676049dd1a450c91414f16ad164eb98288d"`; sha256 `"cba4af9f62351c59303c01097a46119793644a36e1c2793ea8670f309f90ac97"`.

    git --no-pager --no-replace-objects show 82d76676049dd1a450c91414f16ad164eb98288d

`"import"` `"Direct import of github.com/Beamfall/corvint/internal/docmaintain"`, lines 21–21:

    	"github.com/Beamfall/corvint/internal/docmaintain"
<!-- corvint:docmaintain end -->
## Separate MCP profile

Build `./cmd/corvint-corpus-mcp` explicitly and start it with `--root /absolute/repository --artifact
corpus.json`. It serves stdio only. Validated capabilities gate ten possible read tools: `docs_info`,
`docs_search`, `docs_get`, `docs_locate`, `docs_find_related`, `docs_coverage`, `docs_gaps`,
`docs_get_journey`, `docs_get_stability`, `docs_trace`, each prefixed `corvint.`. Present-zero capabilities are callable;
absent capabilities are not advertised. Calls revalidate source/artifact identity. Changing the
configured artifact requires restarting the server. Model-facing text uses the repository-data
envelope; terminator collisions refuse. Structured receipts match the CLI reader, except that
`docs_search` answers with a compact receipt unless it is called with `receipt: "full"`. The compact
receipt lists each anchor once in `anchors` and each limitation once in `limitations`. Everywhere
else they are referred to by id (`a1`, ...) or by index. It omits `capabilities` unless `include`
names `capabilities`; `docs_info` serves them. See DCP-V1-653..655.

To find the source behind a behaviour, call `docs_locate` with a `query` instead of a `path`. It
ranks only `module` and `symbol` subjects, and `result_locations` gives each result's file and
line span. Pass `kind` (`module` or `symbol`) to narrow the results. See DCP-V1-650..651.

A `docs_locate` or `docs_recommend_tests` call on a path also returns each subject that documents a
subject anchored there, through one incoming `documents` relation, with that relation as the reason.
`docs_recommend_tests` adds those owners' test links; the selection still requires the full relevant
suite. Each `docs_claims` result carries a `claim_kind` taken from its typed details; pass `kind` or
`exclude_kind` to filter, and run records rank below behavioural claims. `docs_navigation` takes a
`query` instead of an `id` and selects each flow whose id, name, or linked test file or title holds
every query word. See DCS-V1-591..597.

Without `--protocol-version` the server speaks MCP 2026-07-28. A 2025-11-25 client's `initialize`
gets `-32602`, naming that version and `--protocol-version 2025-11-25`; restart the server with
that flag for such a client (DCP-V1-652).

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

## Additional repository anchors

A records provider may anchor evidence in another repository at its own pinned revision. Name
each such repository in the manifest's `additional_repositories`, sorted by `id`:
`{"id": ROOT_COMMIT, "revision": COMMIT, "root": "../lib"}`, where `root` is the relative path
from the primary checkout to a local checkout of that repository. Declare the anchored files as
scopes and inputs at that revision. The build verifies the root commit and reads only that
checkout; it never fetches. Freshness compares those inputs with that checkout, so a change there
makes the corpus `stale`. An anchor naming an undeclared repository is refused. See DCP-V1-175..179.
A repository may also list further pinned commits in `revisions` (sorted, within the
eight-revision bound); anchors there cite that revision, and freshness compares only `revision`.
`docs_locate` and `docs_recommend_tests` accept an optional `repository` (a declared root-commit
identity) and otherwise name each result's repository in `result_repositories`. See
DCP-V1-308..311.

## Estate inventory across corpora

One corpus covers at most eight repositories (`DCP-V1-175`), so a larger estate is several
corpora, each with `repository_inventory`. To reconcile them, list each corpus's primary checkout
(relative to `--root`, climbing with `../` if needed) and artifact in an estate file:

```json
{"schema": "corvint-corpus-estate/1", "corpora": [{"root": "../svc", "artifact": "corpus.json"}, {"root": "../lib", "artifact": "corpus.json"}]}
```

`corvint docs corpus estate --input estate.json` reads each artifact through the verified read and
prints one receipt: every covered repository once with its names and covering corpora, and every
`outside-corpus-reference` of every corpus joined by exact name to another corpus's declared
package, chart or alias (`resolved`), to several (`ambiguous`, never chosen), or to none
(`outside-estate`, NOT_OBSERVED). A shallow further checkout is refused naming its alias; fetch
its full history (`git fetch --unshallow`) and inventory again. See DCI-V1-001..005 in
[the estate inventory spec](specs/documentation-corpus-inventory-v1.md).

The inventory rules abstain without evidence (DCI-V1-006..010). A `package.json` is a published
`package` only with a `publishConfig` object or a `release`/`publish` job whose literal npm, pnpm
or yarn publish step publishes that manifest's directory, by working directory, `cd DIR &&` or a
folder argument. Otherwise it is a `package_manifest` whose publish evidence is NOT_OBSERVED,
naming any untied release or publish job as a candidate (`DCI-V1-021`). A `*.gemspec` is a gem
`package` and `Gemfile` `gem` lines are references. Only jobs whose key has the word `deploy`
(`deploy`, `deploy-prd`, `build_and_deploy`, not `pact_can_deploy` or `record_deployment`) are
deployable units, named `JOB (ENVIRONMENT)` when the job names a literal `environment:`
(`DCI-V1-027`). Without an environment key, one environment word in the job key, workflow name
or file name names it, labelled as inferred, weaker evidence (`DCI-V1-022`). Add your own words
in `repository_inventory.environment_words` (sorted, for example `["live"]`), and declare the
environment of a job its name does not reveal in `repository_inventory.deploy_jobs`, for example
`[{"repository": "ROOT", "path": ".circleci/config.yml", "job": "deploy", "environment":
"production"}]`; a declared job that no read input holds is a `deploy-declaration-unmatched` gap
(`DCI-V1-028`, `DCI-V1-029`). GitHub workflows and `.circleci/config.yml`
(`DCI-V1-023`) are read; a GitLab, Jenkins or other CI file is an `inventory-ci-unsupported`
gap. Declare your own
name prefixes in `repository_inventory.first_party` (for example `["@acme/", "ghcr.io/acme/"]`)
so an unjoined reference under one is a `first-party-reference-outside-corpus` gap, apart from
third-party dependencies.

## Carrying records across revisions

`docs corpus delta --previous OLD.json --artifact NEW.json` pages a worklist of the previous
corpus's records-provider subjects, claims and relations. Each one is `carry-forward`, `moved`,
`changed`, `removed` or `unresolved` against the current revision. `docs corpus carry-forward
--provider ID` emits one provider's confirmed records for the next build. Adoption kinds carry
forward like generic records:
- flows, paragraphs and the other adoption subject kinds;
- `asserts`, `navigates`, `retired_to`, `manual_case_for` and `migration_mapping` relations.

Their typed details travel unchanged in a `corvint-corpus-adoption-provider/1` record, which needs
a `/2` manifest. A flow with only some anchors changed is `changed`, and its entry lists each
anchor's state. Records whose typed details join to that flow are demoted with the reason. These
are never carried:
- anchorless records, which are summarized as `unresolved`;
- native or compiler-derived records;
- journeys, observations and supplied gaps.

Under the default views, a comparison of a corpus with itself lists no `unresolved` entry. See DCX-V1-360..369,
DCX-V1-420..425 and DCX-V1-440..445.
