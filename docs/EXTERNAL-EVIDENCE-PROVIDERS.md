# External evidence providers

Status: experimental; file and local command transports; `context.external` is additive and never changes the
core receipt.

A third-party project can keep evidence that Corvint cannot derive from Git or code alone —
documented capabilities, user journeys, coverage gaps, cross-repository relationships — and hand it
to `corvint impact` as one JSON record file. This page is written for the author of such a record:
what the file must contain, what Corvint does with it, what ends up in the receipt, and what never
does. The binding contracts are `docs/specs/external-evidence-provider-v0.md` (EEP-V0, the base
record and section), `external-evidence-provider-v1.md` (EEP-V1, repository identity and
cross-repository relations), `external-evidence-provider-v2.md` (EEP-V2, path-to-path relations),
and `external-test-selection-v0.md` (ETS-V0, fail-closed test selection). Every claim below cites
the requirement it restates; read the cited requirement for the exact rule.

## Worked example

`internal/extevidence/testdata/mock-provider.json` is a complete, minimal EEP-V0 record used by the
project's own tests. It declares three entities (a capability, a journey, and a coverage gap) and
five relations that between them exercise every classification this page describes: one `declared`
relation Corvint admits, one `inferred` relation it also admits, one `observed` relation it admits,
one `learned` relation it must exclude, and one relation from a foreign provider's entity it must
report as unresolved. `internal/extevidence/testdata/conformance-v1/` and
`internal/extevidence/testdata/conformance-path/` hold the analogous EEP-V1 (repository identity)
and EEP-V2 (path relation) fixtures, and
`internal/extevidence/testdata/conformance-selection/cases.json` is the labelled corpus for ETS-V0
test selection. Reading a fixture next to the requirement it exercises is the fastest way to see a
rule in its concrete form.

## Record format

A record is one JSON document. `schema` selects the version: `external-evidence-provider/0`
(entities and entity relations only), `/1` (adds `repositories` for identity and cross-repository
endpoints), or `/2` (adds path-to-path relations on top of V1). Unknown top-level members, a
duplicate entity id, or a malformed field make the whole record `invalid` with one reason — Core
never repairs a record (`EEP-V0-001`).

- **V0** (`EEP-V0-001`): exactly `schema`, `provider`, `repository`, `entities`, `relations`. An
  **entity** has a stable `id`, a `kind`, and a `summary`; its receipt identity is
  `<provider-id>:<entity-id>`. A **relation** joins two **endpoints** — `path:<repository-relative
  path>` or `<provider-id>:<entity-id>` — with a `type`, an `evidence` kind, a `rule`, a `reference`,
  and an optional `blob` pin (`EEP-V0-006`).
- **V1** (`EEP-V1-001`, `EEP-V1-002`): `repositories` lists 1–8 entries, each with an `id` and a
  `revision`; `origin` (a root commit id) and `tree` are the only fields Corvint uses to establish
  identity. `remote` and `role` are validated and echoed but never bind, resolve, or rank anything —
  a local path, branch name, display name, or record filename is never an identity.
- **V2** (`EEP-V2-001`, `EEP-V2-002`): a path relation's `from` and `to` are both
  `{repository, path[, blob]}` endpoints, each resolved independently under the V1 endpoint rules.
- **`capabilities`** (optional on every version; `EEP-TR-012`, `EEP-TR-013`): `{"schemas": [...],
  "evidence_kinds": [...]}` lists the complete set of record schemas and evidence kinds the provider
  supports. Leave it out and nothing changes. Declare it and Core checks it before composing:
  `schemas` must name the record's own schema, and `evidence_kinds` must cover every kind the
  record uses under `impact`, or include `declared` or `observed` under `affected`. A shortfall
  closes the provider as `unsupported` with a Core-authored reason (`blocked`,
  `provider-unsupported` in test selection); it never widens what Core accepts.

## Running it

`corvint impact --provider FILE PATH...` selects a record; `--provider` may repeat up to four times,
and a relative `FILE` resolves against `--root` (`EEP-V0-002`). `--repository ID=DIR` binds an EEP-V1
declared repository to a local checkout, up to eight times, and only alongside `--provider`
(`EEP-V1-009`). `--provider` is incompatible with `--base` and `--working-tree-untracked`, and a
fifth `--provider` is an argument error. No other verb reads a provider record in this slice except
`corvint affected`, which reads the same records for test selection (see below).

`corvint impact --provider-command ARGV_JSON PATH...` runs a provider instead of reading a file
(`docs/specs/external-evidence-provider-transports-v0.md`). `ARGV_JSON` is a JSON array whose first
element is an absolute executable path, for example `'["/usr/local/bin/export-docs","--json"]'`;
there is no shell and no `PATH` lookup (`EEP-TR-002`). The command gets empty stdin, only `PATH`,
`TMPDIR`, `LANG=C`, and `LC_ALL=C` in its environment, and `--root` as its working directory
(`EEP-TR-003`); it must print one record within 10 seconds and 1 MiB, and its process group is
killed at the time bound (`EEP-TR-004`). Its stdout is decoded exactly as a record file
(`EEP-TR-005`); stderr is discarded (`EEP-TR-008`), and any failure is one `unavailable` or
`invalid` provider row with no record content (`EEP-TR-006`). It counts toward the four-provider
bound. `corvint affected --provider-command ARGV_JSON` takes the same option with the same rules
and yields the same `test_selection` as the record file would (`EEP-TR-001`, `ETS-V0-001`).

## `context.external`

Provider output appears only under `context.external`; every other member of `context` is
byte-identical to a run without `--provider`, and without `--provider` the `external` member is
absent entirely (`EEP-V0-003`). The section holds:

- `providers`: one entry per selected record, with `source`, the `sha256` of the bytes read, `id`,
  `revision`, `repository_revision`, `freshness`, `state`, and `reason` (`EEP-V0-004`).
- `results`: each entity joined by a relation to a requested changed path.
- `downstream`: each entity one relation away from a result entity that is not itself a result.
- `verification`: each `verifies`/`covers`/`asserts` relation between a non-changed path and a
  listed entity (`EEP-V0-011`).
- `path_relations` and `omitted.path_relations` (only when a V2 record loads): every path-to-path
  relation with a changed root path on either side, anchored on the changed side (`EEP-V2-003`).
- `unknowns`: everything the section could not or must not admit (see below).
- `omitted`: counts for every list entry dropped by a bound.

Every item carries a `reason` naming the changed path or entity and the relation type and evidence
kind that admitted it, and entries are ordered by provider id, then entity id, then relation `from`,
`to`, and `type`, so identical inputs always produce identical section bytes (`EEP-V0-004`,
`EEP-V0-011`).

## Authority

Every item in `context.external` carries `authority` `external-provider`, assigned by Core — a
record cannot state its own authority, and no external item ever receives a repository authority
label (`EEP-V0-007`). This is the same rule `AGENTS.md` states at the product level: project-owned
authority always outranks provider evidence. Nothing a provider declares can promote itself into
`context.results`, ranking, learning, a CEM, an OCM, or the Change Frontier; that boundary is
absolute in this slice (`EEP-V0-015`).

## Freshness and verification

`freshness` compares the record's declared repository revision against the captured commit revision
by Git ancestry alone — never a timestamp — and is exactly one of `equal`, `repository-ahead`,
`provider-ahead`, `unrelated-history`, or `revision-unavailable` (`EEP-V0-009`). Each path endpoint
also carries `verification`: `verified` (tracked at the captured revision and, when the relation
pins a blob, equal to it), `stale` (tracked, pinned blob differs), `deleted` (not tracked now but
tracked at the record's declared revision, decided only when that revision is a known commit), or
`missing` (not tracked, and not shown to have been tracked at the declared revision); an entity
endpoint carries `unsupported` (`EEP-V0-010`). Verification proves the path's identity at that
revision — it never proves the provider's statement is correct.

The contract keeps its accepted wire names rather than the vocabulary issue 64 proposed; the
mapping is: `provider-is-ancestor` is freshness `repository-ahead`; `reference-missing` is
verification `missing` or `deleted`; `not-observed` is the V1 endpoint state `not-verified`;
`ambiguous` is identity `ambiguous` (`EEP-V1-004`); `reference-ambiguous` needs symbol identity,
which the contract does not carry (ticket V1-0101).

With EEP-V1 repositories in play, identity is `resolved`, `unresolved` (no declared origin), or
`ambiguous` (two declared repositories share an origin) per repository, and binding is one of
`checkout`, `root`, `mismatch`, `unavailable`, `ambiguous`, `unresolved`, or `unbound`
(`EEP-V1-004`, `EEP-V1-005`). A relation's overall `relation_state` is the worst side of its two
endpoints, ordered `unresolved` > `stale` > `not-verified` > `fresh` (`EEP-V1-008`, `EEP-V2-004`).

## `learned` evidence is excluded

A relation's `evidence` must be `declared` (the provider's own authored statement), `observed` (a
recorded execution or measurement), `inferred` (derived by the provider's own rule), or `generated`
(produced by a model or heuristic with no observation behind it). Any other value — most
importantly `learned` — excludes that relation to `unknowns` instead of admitting it
(`EEP-V0-007`). This keeps Corvint's own learning loop out of the receipt: a `learned` relation
never reaches `results`, `downstream`, `verification`, or `path_relations`, no matter how confident
the provider's evidence claim.

A `generated` relation is admitted, but every item it admits says so in `relation.evidence` and in
its `reason`, so a consumer that wants only observed evidence can drop those items by that one
member (`EEP-V0-019`). Test selection never qualifies on it: a generated test is a candidate coded
`generated-only-evidence`, like an `inferred` one (`ETS-V0-014`). Use `generated` for a
model-suggested or heuristic link you have not executed, and `observed` only for a link a recorded
run or measurement supports.

## Unknowns

An endpoint outside the shapes above, a path outside its bounds, an entity endpoint naming another
provider's id or an entity the record does not declare, and any `learned` relation are all recorded
under `unknowns` with a reason instead of being admitted (`EEP-V0-006`, `EEP-V0-007`). Failure is
closed throughout: an unreadable record is `unavailable`, a record that fails the schema rules is
`invalid`, and an unresolvable endpoint becomes an `unknowns` entry — none of these change the exit
code or the core receipt (`EEP-V0-005`).

## Fail-closed test selection (ETS-V0)

`corvint affected --provider FILE --selection-profile strict|coverage` adds `advice.test_selection`.
Its `state` is exactly one of `narrow-selection-allowed`, `full-relevant-suite-required`, `blocked`,
or `unknown`, decided in a fixed order: any unavailable or invalid record forces `blocked`; an
unbounded plan scope or no changed path forces `unknown`; any uncovered obligation or blocking
reason forces `full-relevant-suite-required`; only then is `narrow-selection-allowed` possible
(`ETS-V0-003`). A relation qualifies a test only when its type is admitted by the profile, its
evidence is `declared` or `observed`, the test side's repository identity is resolved with a `root`
or `checkout` binding, the record is fresh, the test path is verified, and the path is not dirty in
the worktree (`ETS-V0-004`, `ETS-V0-005`); anything else blocks the obligation it touches rather than
silently narrowing (`ETS-V0-006`). A V0 record never qualifies a test at all, because it declares no
repository identity (`ETS-V0-005`).

ETS-V1 (`docs/specs/external-test-selection-v1.md`) replaces the one-hop widening with a bounded
obligation walk of at most 4 hops and 256 entities per record that follows only declared relations
(`ETS-V1-001`, `ETS-V1-004`); a cut blocks the edge with `obligation-depth-truncated` or
`obligation-budget-exhausted`. It also reads each resolved checkout's dirty paths once, and a
test side bound to a dirty or unreadable checkout blocks with `checkout-worktree-dirty` or
`checkout-worktree-unreadable` (`ETS-V1-005`).

## Bounds and limits

At most four provider records, 1,048,576 bytes per record, 1,000 entities and 4,000 relations per
record, and `--limit` entries in each of `results`, `downstream`, and `verification`; entries beyond
a bound are dropped from the end of the deterministic ordering and counted under `omitted`
(`EEP-V0-012`). `summary`, `rule`, `reference`, and `reason` text is capped at 512 bytes of valid
UTF-8, and identifiers and revisions at 128 bytes (`EEP-V0-013`). EEP-V1 adds its own bound of 1–8
declared repositories (`EEP-V1-002`) and up to 8 `--repository` bindings (`EEP-V1-009`); ETS-V0 bounds
every advice list at 64 rows with omissions counted (`ETS-V0-010`).

## Privacy model: what never enters a receipt

- `impact --provider` reads only the record file and Git; it writes no repository, index, or trace
  state, and the receipt reports `mutates` false (`EEP-V0-014`).
- A record's `summary`, `rule`, `reference`, and `reason` text is provider-authored, untrusted data,
  never an instruction; the section names every such member under `untrusted_text_fields` so a host
  can envelope it before display (`EEP-V0-013`).
- No credential, file body, or resolved local checkout path is ever emitted. A `--repository`
  checkout is echoed exactly as given on the command line, never as its canonicalized directory
  (`EEP-V1-009`, `EEP-V1-014`, `ETS-V0-013`).
- Provider-declared identity — `origin`, `remote`, `role` — grants no authority; it is validated and
  echoed, never treated as a permission or a ranking signal (`EEP-V1-014`).
- External items never enter `context.results`, ranking, learning, a CEM, an OCM, or the Change
  Frontier in this slice (`EEP-V0-015`).

## Not yet supported

- MCP and remote provider transports. MCP is a proposed profile (`EEP-TR-009`, decision 0317);
  wrap an MCP server in a local command that prints one record. A remote fetch is NO-GO on the
  default local path (`EEP-TR-010`, decision 0318).
- Obligations more than 4 relation hops or 256 entities from a changed path (`ETS-V1-001`);
  `impact` itself still widens one hop (`EEP-V2-008`).
- Inspecting a `--repository` checkout's worktree contents beyond its Git status and metadata;
  `affected` reads a checkout's dirty paths (`ETS-V1-005`) and nothing else in it.
- Symbol identity (`repository-id:symbol`); ticket V1-0101. Capability negotiation is the record
  member above (V1-0102); the `generated` kind is `EEP-V0-019` (V1-0107).
- Feeding external evidence into the Change Frontier itself. External items stay out of the frontier
  wire (`EEP-V0-015`); `corvint obligations --cem FILE --impact FILE` instead writes a separate,
  reference-only `external-frontier-obligations/0` sidecar
  (`docs/specs/external-frontier-obligations-v0.md`) that nothing reads.
