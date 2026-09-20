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

The optional `corvint-corpus-behavior-stability-provider/1` profile adds digest-bound repeated
Playwright evidence without changing behavior-contract coverage. `stability --id ID` returns the
selected one-spec, feature-batch or suite policy identity, separate raw outcome/cleanup/retry counts,
and every immutable contributing receipt and attempt. Planned repetitions, retries and manual reruns
remain distinct. Missing, duplicate, stale, cross-revision or contradictory contributors refuse;
failed cleanup or any threshold miss yields `not-stable`. A stability report is repeated-run evidence,
not test adequacy, behavior parity or selection authority.

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
