# External Evidence Provider V0

Owner: Russell Lewis
Date: 2026-09-18
Intent status: accepted (decision 0309)
Delivery status: experimental (file records and local authoring kit; kit promotion blocked on V1-0013)
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/analyzer-capability-contract-v0.md`, `docs/specs/affected-plan-v0.md`,
`docs/specs/change-frontier-v0.md`, and the feature request Beamfall/corvint#1.

## Agent digest
- Claim: `corvint impact --provider FILE` attaches provider records in a separated `context.external` section and changes nothing in the core receipt.
- Status: accepted (decision 0309)/experimental (file records and local authoring kit; kit promotion blocked on V1-0013); checked by `TestImpactProviderSectionSeparation`.
- Exists: experimental authoring kit `examples/evidence-provider/v0` and exact-pin consumer `internal/extevidence/pin.go`; `internal/extevidence` and the `--provider` option in `cmd/corvint`; provider items carry the Core-assigned `external-provider` authority, Git-ancestry freshness, and per-path reference verification; `EEP-V0-023..026` (proposed, experimental, decision 0371) add the Core-owned in-process gopls provider `internal/lspprovider`, off unless `CORVINT_CONTEXT_LSP=gopls`.
- Blocked on: nothing for a local command, which is `external-evidence-provider-transports-v0.md` (decision 0316); Kit promotion needs V1-0013 and owner acceptance; kit MCP remains proposed. Separately accepted MCP/remote profiles are unchanged. Cross-repository relationships are `external-evidence-provider-v1.md` (decision 0310).
- Read next: Definitions; Requirements; Non-goals and simpler baseline.

## User and measurable job

A project that keeps higher-level product evidence outside the repository (documented capabilities,
routes, journeys, test relationships, coverage gaps) exports that evidence as one record file per
provider and asks `impact` which externally documented entities a changed path touches. The job is
done when the receipt names every linked entity with the provider that said so, the typed relation
and its evidence kind, whether the provider's view is fresh against the captured revision, whether
each cited path still exists at that revision, and what was omitted or could not be resolved.

## Verified current state

- `corvint impact PATH...` compiles Git-pinned path impact into `context.results`; every evidence
  item carries `path`, `line`, `blob_hash`, `authority`, `confidence`, and `reason`
  (`internal/contextindex/impact.go`).
- External participants join Core only through the Analyzer Capability Contract: a signed native
  executable launched once, with daemons, network services, and in-process plugins out of scope
  (`docs/specs/analyzer-capability-contract-v0.md`).
- The CEM 0.2 decoder refuses unknown fields (`internal/cem/wire/map.go`), so an external
  obligation cannot ride inside a CEM without a wire change; the Change Frontier is the accepted
  home for obligations that lack witnesses.
- The affected plan already emits a read-only, non-authoritative test selection with an unknown
  frontier (`docs/specs/affected-plan-v0.md`); test-claim qualification already refuses to treat a
  test's existence as verification.

## Definitions

- **provider record**: one JSON document with `schema` `external-evidence-provider/0`, one provider
  identity and revision, the full commit id of the repository revision the provider observed, a
  list of entities, and a list of typed relations.
- **entity**: one externally documented thing with a stable id, a kind, and a summary. Its identity
  in the receipt is `<provider-id>:<entity-id>`.
- **endpoint**: either `path:<repository-relative path>` or `<provider-id>:<entity-id>`. A relation
  joins two endpoints.
- **relation**: one typed, directed link between two endpoints with an evidence kind, a derivation
  rule, and a reference into the provider's own material. An optional `blob` pins the Git blob the
  provider observed at a path endpoint.
- **evidence kind**: `declared` (the provider's own authored statement), `observed` (a recorded
  execution or measurement), `inferred` (derived by the provider's rule), or `generated` (produced
  by a model or heuristic with no observation behind it; added 2026-09-22 for issue 64, decision
  0350).
- **external section**: the `context.external` member of the impact receipt. It is the only place
  provider output appears.

## Requirements

- `EEP-V0-001`: A provider record MUST be one JSON document whose top-level members are exactly
  `schema`, `provider`, `repository`, `entities`, and `relations`, plus the optional
  `capabilities` declaration of `EEP-TR-012` (decision 0351); `schema` MUST equal
  `external-evidence-provider/0`. Any unknown member at any level, a member name repeated within
  one object (worded as in `EEP-V0-020`), a duplicate entity id, an identifier or text outside
  the bounds of `EEP-V0-012` and `EEP-V0-013`, or a malformed field makes the whole record
  `invalid` with one reason. Core never repairs a record.
- `EEP-V0-002`: `corvint impact --provider FILE PATH...` selects a provider record; the option MAY
  repeat up to four times. A relative `FILE` resolves against `--root`. `--provider` with `--base`
  or `--working-tree-untracked` is an argument error, and a fifth `--provider` is an argument
  error. No other verb accepts providers in V0.
- `EEP-V0-003`: Provider output appears only under `context.external`. With the same paths, limit,
  and index, every other member of `context` MUST be byte-identical to a run without `--provider`.
  Without `--provider` the `external` member is absent.
- `EEP-V0-004`: The section MUST list every selected provider with `source` as given, the
  `sha256` of the bytes read, `id`, `revision`, `repository_revision`, `freshness`, `state`, and
  `reason`. Identical record bytes, changed paths, limit, and index MUST produce an identical
  section.
- `EEP-V0-005`: A record that cannot be read is reported with state `unavailable`; a record that
  fails `EEP-V0-001` is reported with state `invalid`; a record whose declared capabilities omit
  what the invocation requires is reported with state `unsupported` (`EEP-TR-013`); a loaded
  record has state `loaded`. No failure state changes the exit code, `ok`, or the core receipt. An
  unavailable, invalid, or unsupported provider contributes no results and no unknowns beyond its
  own provider entry.
- `EEP-V0-006`: An endpoint MUST be `path:<path>` or `<provider-id>:<entity-id>`. A path MUST be
  repository-relative, non-empty, without a leading slash or `..` segment, and at most 1024 bytes.
  An endpoint with neither prefix, a path outside those bounds, an entity endpoint whose provider
  id differs from the record's own, or an entity id the record does not declare makes that relation
  `unresolved`: it is listed under `unknowns` with a reason and contributes nothing else.
  V0 defines no cross-repository identity; `EEP-V1` does.
- `EEP-V0-007`: A relation's `evidence` MUST be `declared`, `observed`, `inferred`, or `generated`
  (`EEP-V0-019`). Any other value, including `learned`, excludes that relation to `unknowns` with
  a reason. Every item in the
  section carries `authority` `external-provider`, assigned by Core; a record cannot state an
  authority, and external items never receive a repository authority label.
- `EEP-V0-008`: A relation `type` is preserved exactly as the provider wrote it: lowercase, digits,
  hyphens, an optional `<provider-id>:` namespace, at most 64 bytes. Core MUST NOT rename, merge,
  or collapse types, and MUST NOT derive one type from another.
- `EEP-V0-009`: `freshness` compares the record's `repository.revision` with the captured commit
  revision by Git ancestry and reports exactly one of `equal`, `repository-ahead` (the provider's
  revision is an ancestor of the captured revision), `provider-ahead` (the captured revision is an
  ancestor of the provider's), `unrelated-history`, or `revision-unavailable` (the value is not a
  full commit id known to the repository). Timestamps are never an input.
- `EEP-V0-010`: Every path endpoint in an included relation carries `verification`: `verified`
  (tracked at the captured revision and, when the relation pins a `blob`, equal to the pinned
  blob), `stale` (tracked, pinned blob differs), `deleted` (not tracked at the captured revision
  but tracked at the record's declared revision, decided only when freshness is `repository-ahead`,
  `provider-ahead`, or `unrelated-history`), or `missing` (not tracked at the captured revision and
  not shown to have been tracked at the declared revision; amended 2026-09-22, issue 64). An entity
  endpoint carries `unsupported`. Verification proves identity at the revision, never that the
  provider's statement is correct. Every consumer that treats `missing` as stale MUST treat
  `deleted` the same way.
- `EEP-V0-011`: `results` lists each entity joined by a relation to a requested changed path;
  `downstream` lists each entity one relation away from a result entity that is not itself a
  result; `verification` lists each relation of type `verifies`, `covers`, or `asserts` between a
  path endpoint that is not a changed path and a listed entity. Every item carries a `reason`
  naming the changed path or entity and the relation type and evidence kind that admitted it.
  Ordering is by provider id, then entity id, then relation `from`, `to`, and `type`.
- `EEP-V0-012`: Bounds: at most four providers, 1048576 bytes per record, 1000 entities and 4000
  relations per record, and `--limit` entries each in `results`, `downstream`, and `verification`.
  Entries beyond a bound are dropped from the end of the ordering and counted under `omitted`.
- `EEP-V0-013`: `summary`, `rule`, `reference`, and `reason` text from a record MUST be valid
  UTF-8 of at most 512 bytes; identifiers and revisions at most 128 bytes. The section names its
  provider-authored free-text members under `untrusted_text_fields` so a host can envelope them.
- `EEP-V0-014`: `impact --provider` reads the record and Git and writes no repository, index, or
  trace state; the receipt reports `mutates` false.
- `EEP-V0-015`: External items never enter `context.results`, ranking, learning, a CEM, an OCM,
  or the Change Frontier. A later slice may consume the section explicitly; V0 does not.

- `EEP-V0-016`: The experimental versioned local authoring kit MUST contain one copyable native-Go
  provider using only the standard library, an explicit provider revision and explicit existing
  record profile, and a reproducible offline build/conformance command from a clean checkout.
  Unsupported sample provider revisions/profiles MUST fail explicitly without emitting a record.
- `EEP-V0-017`: The kit consumer MUST strictly decode bounded regular-file or contained-command
  bytes, compare the exact requested schema, provider ID/revision and full repository revision,
  and emit the original complete bytes only after agreement. Profiles `/1` and `/2` MUST also
  pin a nominated repository ID and root commit. The experimental record compatibility window
  is exactly `/0`, `/1`, `/2` with the current consumer, not arbitrary historical engine execution.
  Pins grant neither repository verification nor governing authority; Core still verifies evidence.
- `EEP-V0-018`: Kit conformance MUST reuse valid, stale, malformed, ambiguous,
  repository-mismatched and unsupported fixtures, exercise old/current records, prove a provider
  authored from copied source through file and command transports, and retain Core separation.
  Promotion MUST remain blocked on V1-0013's portable-proof freeze and owner acceptance; passing
  local synthetic conformance MUST NOT be reported as acceptance or external validation.
- `EEP-V0-019`: A `generated` relation composes exactly as the other admitted kinds, and every
  item it admits carries `generated` in `relation.evidence` and names the kind in its `reason`, so
  a consumer can down-weight or exclude generated items from that member alone. Core neither ranks
  nor down-weights external items (`EEP-V0-015`); the one Core consumer, test selection, treats
  `generated` as weak evidence that never qualifies and never blocks (`ETS-V0-014`). `learned`
  stays excluded: it names a feedback-trained source whose derivation the record cannot cite,
  whereas a `generated` relation still carries the generator as `rule` and its material as
  `reference`.
- `EEP-V0-020`: The kit declares its own version, currently `0.2.0` (kit README title and the
  runner's `kitVersion`), separately from any provider revision. Kit 0.2.0 supports exactly the
  record profiles `external-evidence-provider/0`, `/1` and `/2`; its compatibility window is those
  three profiles read by the current checkout's consumer, and pins saved under kit 0.1.0 keep
  their meaning. The kit consumer MUST refuse each profile
  failure with its own reason and no record bytes: a member name repeated within one object,
  compared ignoring letter case under the simple case folding Go's decoder applies, is
  `ambiguous record profile: repeated schema member` for the top-level `schema` and
  `ambiguous record: repeated member "PATH"` otherwise, any other declared profile or none is
  `unsupported record profile`, a supported profile other than the pin is
  `record schema differs from pin`, and a `/1` or `/2` record in which more than one repository
  declares the pinned origin is `ambiguous pinned repository origin`.
- `EEP-V0-021`: The kit MUST carry a standard-library Go conformance runner that builds the
  provider from a copy of the kit's `main.go` offline, commits a scratch Git repository, and runs
  `corvint impact` over the file transport (`--provider`) and the contained command transport
  (`--provider-command`) for valid `/0`, `/1`, `/2`, stale, repository-mismatched, malformed,
  ambiguous and unsupported-profile cases plus a refused unsupported-profile request. Each case
  MUST produce the same external section on both transports apart from the provider source, a
  core receipt equal to the receipt without a provider, `mutates` false, and its pinned outcome;
  any disagreement exits nonzero.
- `EEP-V0-022`: The kit MUST carry a clean-checkout authoring proof script that checks out only the
  kit directory at a named commit into a fresh scratch clone, refuses when any other path is
  present, builds the runner there offline, and runs it against a named Corvint binary with only
  Git, local Go 1.27.1 and Darwin or Linux as prerequisites.

### Core-owned local language-server provider (proposed 2026-09-23; experimental; decision 0371)

- `EEP-V0-023`: A Core-owned local provider MAY produce one `external-evidence-provider/2` record
  in process. Its first and only instance is gopls, selected by `CORVINT_CONTEXT_LSP=gopls` on
  `corvint context` (`TCP-V0-043`); it is off by default and installs no daemon, service or
  configuration. The record takes the one decode, capability check, Git-ancestry freshness and
  endpoint verification every transport shares (`InlineSection` in
  `internal/extevidence/section.go`), under provider source `lsp:gopls`, so its items carry the
  Core-assigned `external-provider` authority and trust class (`TCP-V0-023`), are never project
  authority, and never enter core results or ranking (`EEP-V0-015`, `EEP-V2-003`).
- `EEP-V0-024`: Each invocation owns one `gopls serve` process: the executable found on `PATH`,
  started in the canonical repository root inside an owned process group
  (`internal/procgroup`), with an environment limited to the Go toolchain's locations plus
  `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local` and a private `GOPLSCACHE` directory removed
  afterwards. It is bounded by a 20 s hard wall time that kills the group, a 15 s soft deadline
  after which no query is issued, 64 MiB of server output counted by the client, 4 MiB per
  message, and 64 KiB of captured stderr. A record exists only after a clean exit 0 with the
  process group proven cleaned up. The record's `provider.revision` is the gopls module version
  from the `initialize` response's `serverInfo`, reduced to the identifier grammar (`unknown` when
  absent). A server whose `serverInfo.name` is not exactly `gopls` (including an absent name) is
  refused before any query, which tightens the otherwise unconstrained server identity. The
  executable is only an absolute `PATH` match; a `PATH` lookup error, including a match relative to
  the working directory, is `gopls executable not found`. The query digest is SHA-256 over the canonical JSON of the provider id, the index
  commit, the seeds and the bounds; it is reported as `query.sha256` and opens every relation's
  `reference`.
- `EEP-V0-025`: The expansion is at most two hops. Seeds are at most three `.go` paths whose
  indexed text equals the working tree. Hop one queries each seed at up to eight top-level
  function, method and type declarations (`textDocument/references`, declarations excluded,
  skipping `_`, `init` and `main`) and up to eight distinct selector and call names
  (`textDocument/definition`). Hop two queries the three hop-one files most often reached (then by
  path) and never relates to a seed or hop-one file. At most 64 queries run. Each distinct
  (origin, file, type) is one relation from the queried file to the answered file, of type
  `gopls:referenced-by` or `gopls:uses-definition`, evidence `inferred`, both endpoints pinned to
  their index blobs; its `rule` names the method, the one-based position and the symbol, and its
  `reference` names the digest, the hop, the seed and, at hop two, the hop-one file it came
  through. A location outside the repository is counted (`outside_repository`); a file not
  indexed as text, or whose working-tree bytes differ from the index, is omitted and counted
  (`omitted_rows`); a query the server answers with an error is counted (`failed_queries`, beside
  `queries_issued`); the record keeps at most 32 relations and 64 KiB.
- `EEP-V0-026`: An absent or failing provider yields no record and no partial rows: the section
  holds one `unavailable` provider row whose reason names the cause (`not applicable: no
  committed, unmodified Go file among the seeds`, `gopls executable not found`, `gopls did not
  start`, `gopls exceeded 20s wall time; process group killed`, `gopls cancelled`, `gopls session
  failed`, `gopls did not exit cleanly`, `gopls process group not proven cleaned up`, `language
  server identified as NAME, not gopls; refused`, `gopls answered all N queries with an error;
  first: ERROR` when at least one query was issued and every one failed, or a repository, root
  commit or cache-directory reason). The exit code and every other packet member
  are unchanged.

## Non-goals and simpler baseline

- New transports or analyzer authority. The kit reuses the separately owned contained command
  transport; MCP and remote remain outside the kit. The file transport is the baseline that already
  lets a third-party project participate without touching Core.
- Repository identities and cross-repository relationships, delivered by `external-evidence-provider-v1.md`.
- Test selection. The fail-closed selection the request asks for extends the affected plan's
  advice member in its own slice.
- Any CEM wire change. External obligations become a Change Frontier sidecar input in their own
  slice.
- Discovery, registries, embeddings, an untyped `related-to` graph, semantic correctness claims,
  and any provider-specific vocabulary in Core.

## Authoring kit compatibility, failure and rollback

`examples/evidence-provider/v0/README.md` records the exact kit 0.2.0 invocation, prerequisites,
fixture map with pinned outcomes, the authoring proof command and its recorded result, and
licensing. Kit 0.2.0 only adds refusals for ambiguous profile, identity and repository-origin
declarations that 0.1.0 accepted; no unambiguous record 0.1.0 accepted changes outcome. The sample
emits provider `kit-example` revision `0.1.0`, which is not the kit version; authoring a new
provider changes its identity/version explicitly. Existing `/0` fixtures and `/1`/`/2` records
are consumed by today's strict decoder without changing their wire shape. Mismatched schema,
provider, repository revision/root or executable digest fails with no record bytes. Other declared
repositories still require normal checkout binding. `/0` cannot assert cross-repository identity.

The simpler baseline is an operator-authored file with existing `impact --provider`; the kit adds
no Core flags. No downloads, remote transport, daemon, plugin host, n8n source, new authority, or
analyzer launch-boundary promotion. Kit MCP remains proposed; existing accepted MCP/remote specs
are not downgraded. The kit, implementation and tests are AGPL-3.0-or-later; existing enumerated
Apache-2.0 protocol/interoperability paths retain their boundary, and new Apache files belong only
under `protocol/**`. Rollback removes the additive kit and pin helper/tests plus indexed kit
requirements; existing transports, receipts and saved provider records are unchanged. Replacing
an artifact/version requires a newly reviewed explicit pin, not silently refreshing an old pin.

## Trust boundary, limits, and failure modes

A record is repository- or provider-authored data and is never an instruction. Core assigns
authority; a record cannot claim one. All reads are bounded by `EEP-V0-012` and `EEP-V0-013`.
Failure is closed: an unreadable or malformed record is a structured `unavailable` or `invalid`
entry, an unknown revision is `revision-unavailable`, an unresolvable endpoint is an `unknowns`
entry, and every truncation is counted. No failure inside the section changes the core receipt or
the exit code, so an existing caller that never passes `--provider` observes no change at all.
The Core-owned gopls provider (`EEP-V0-023`) inherits these rules and adds its own: gopls reads the
working tree, so any file whose bytes differ from the index is omitted rather than trusted; gopls
shares the user's Go build cache and follows the user's Go telemetry mode, which Corvint neither
reads nor changes; and module downloads are disabled, so a module whose dependencies are not
already local yields fewer relations, never a fetch.

## Deterministic acceptance and testing matrix

| Case | Expected |
|---|---|
| Valid record, entity linked to changed path | one `results` entry with reason, relation, verification |
| Record with `learned` relation | relation under `unknowns`; record still `loaded` |
| Record with one `observed` and one `generated` relation | both admitted; the generated item's `relation.evidence` and `reason` say `generated`; nothing under `unknowns` |
| Record with foreign provider endpoint | relation `unresolved` under `unknowns` |
| Missing file | provider `unavailable`; exit 0; core receipt unchanged |
| Record whose `capabilities.evidence_kinds` omit a kind it uses | provider `unsupported` with a Core-authored reason; nothing composes; exit 0 |
| Unknown top-level member | provider `invalid`; exit 0 |
| Provider revision equal / ancestor / descendant / orphan / unknown | the five `EEP-V0-009` states |
| Path tracked with equal, differing, and absent pinned blob; untracked path | `verified`, `stale`, `verified`, `missing` |
| Untracked path that the declared ancestor revision tracked; same path under `equal` or `revision-unavailable` | `deleted`; `missing` |
| More entities than `--limit` | `omitted.results` counts the rest |
| Same inputs twice | identical section bytes |
| Evaluation over the mock-provider fixture | precision, recall, false-positive relationships 0, abstention accuracy, latency, receipt bytes |
| gopls record fixture `testdata/conformance-path/lsp-gopls.json` in process and from a file | same `path_relations`; `external-provider` trust; each reference names its hop origin |
| Live gopls over a committed four-package module | hop-one and hop-two relations pinned to blobs; a modified file omitted and counted |
| No Go seed; no executable; a server that exits at once | one `unavailable` row with its reason; no record |

## Rollout, rollback, and compatibility

The option is additive. Rollback removes `internal/extevidence`, the `--provider` option, this
document, and decision 0309; no wire other than the impact receipt's optional `external` member is
touched, and that member is absent for every existing caller. The `generated` kind (decision 0350)
rolls back on its own by removing `EvidenceGenerated` from the kind map and the weak-evidence
table; a record carrying it then returns to `excluded-evidence-kind`, and no other record changes.
`EEP-V0-023..026` roll back alone: delete `internal/lspprovider`, `cmd/corvint/context_lsp.go`,
their tests, the fixture `lsp-gopls.json`, the one `attachLSPEvidence` call in
`compileTaskContext`, and `InlineSection` with its `sectionOf` split in
`internal/extevidence/section.go`; unset `CORVINT_CONTEXT_LSP`. No other wire changes.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `EEP-V0-001`, `EEP-V0-013` | `internal/extevidence/record.go`, `internal/extevidence/section.go` | `TestProviderRecordSchemaStrict`, `TestCapabilitiesDecodeStrict`, `TestRepeatedMemberIsInvalid` |
| `EEP-V0-002` | `cmd/corvint/main.go` | `TestImpactProviderFlagParsing` |
| `EEP-V0-003`, `EEP-V0-015` | `cmd/corvint/main.go` | `TestImpactProviderSectionSeparation` |
| `EEP-V0-004` | `internal/extevidence/section.go` | `TestProviderSectionDeterministicAndPinned` |
| `EEP-V0-005` | `internal/extevidence/section.go` | `TestProviderUnavailableAndInvalidAreStructured`, `TestCapabilitiesNegotiation` |
| `EEP-V0-006` | `internal/extevidence/compose.go` | `TestEndpointIdentitiesResolve` |
| `EEP-V0-007` | `internal/extevidence/compose.go` | `TestEvidenceKindLearnedExcluded`, `TestEvidenceKindGeneratedAdmitted`, `TestImpactProviderEvaluation` |
| `EEP-V0-008` | `internal/extevidence/compose.go` | `TestRelationTypesPreserved` |
| `EEP-V0-009` | `internal/extevidence/freshness.go` | `TestFreshnessStatesFromAncestry` |
| `EEP-V0-010` | `internal/extevidence/compose.go` | `TestReferenceVerificationStates`, `TestReferenceVerificationDeleted` |
| `EEP-V0-011` | `internal/extevidence/compose.go` | `TestResultCompositionDirectDownstreamVerification`, `TestItemOrderGroupsByProvider` (provider-id ordering corrected 2026-09-18; section bytes change only for runs where two providers declare the same entity id), `TestImpactProviderEvaluation` |
| `EEP-V0-012` | `internal/extevidence/compose.go` | `TestLimitsAndOmissions` |
| `EEP-V0-014` | `cmd/corvint/main.go` | `TestImpactProviderReadOnly` |

| `EEP-V0-016` | `examples/evidence-provider/v0/main.go` | `TestProviderKitAuthoredProvider`, `TestProviderKitProducerRefusals` |
| `EEP-V0-017` | `internal/extevidence/pin.go`, `examples/evidence-provider/v0/check/main.go` | `TestProviderKitExactPins`, `TestProviderKitChecker` |
| `EEP-V0-018` | `internal/extevidence/pin_test.go`, kit README | `TestProviderKitFixtureConformance`, `TestProviderKitAuthoredProvider`, `TestImpactProviderSectionSeparation`; V1-0013 freeze and owner acceptance NOT_OBSERVED |
| `EEP-V0-019` | `evidenceKinds` in `internal/extevidence/compose.go`, `weakEvidence` in `internal/extevidence/selection.go` | `TestEvidenceKindGeneratedAdmitted`, `TestSelectionConformance` |
| `EEP-V0-020` | `profileReason`, `repeatedMember` in `internal/extevidence/pin.go` | `TestProviderKitProfileReasons` |
| `EEP-V0-021` | `examples/evidence-provider/v0/conformance/main.go` | `TestProviderKitConformanceRunner` |
| `EEP-V0-022` | `examples/evidence-provider/v0/authoring-proof.sh` | recorded run in the kit README and `docs/BUILD-LOG.md` |
| `EEP-V0-023` | `InlineSection`, `sectionOf` in `internal/extevidence/section.go`; `attachLSPEvidence` in `cmd/corvint/context_lsp.go` | `TestLSPRecordConformance`, `TestContextLSPOffKeepsTheGoldenAndOnDegrades` |
| `EEP-V0-024` | `Expand`, `run`, `environment`, `querySummary`, `serverVersion`, `serverName` in `internal/lspprovider/provider.go`; `internal/lspprovider/session.go` | `TestExpandLiveGopls`, `TestExpandEveryQueryFailedIsUnavailable`, `TestSessionAnswersServerRequests` |
| `EEP-V0-025` | `dialogue`, `walker`, `record`, `relation` in `internal/lspprovider/provider.go`; `targets` in `internal/lspprovider/targets.go` | `TestExpandLiveGopls`, `TestTargets`, `TestLSPRecordConformance` |
| `EEP-V0-026` | `Expand`, `run` in `internal/lspprovider/provider.go`; `InlineSection` | `TestExpandDegrades`, `TestExpandEveryQueryFailedIsUnavailable`, `TestLSPUnavailableIsVisible`, `TestContextLSPOffKeepsTheGoldenAndOnDegrades` |

## Unresolved decisions and promotion or kill criteria

- Promotion to a supported option requires one independent adopter record produced by a system
  that is not the Beamfall product documentation system, plus a measured false-positive relation
  rate over a labelled fixture set.
- Kill if the section cannot stay byte-separate from the core receipt, or if any consumer starts
  reading external items as authority.
