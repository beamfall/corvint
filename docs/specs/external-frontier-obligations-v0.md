# External Frontier Obligations V0 — a reference-only Change Frontier sidecar

Owner: Russell Lewis
Date: 2026-09-18
Intent status: accepted (decision 0313)
Delivery status: experimental (file transport only)
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/change-frontier-v0.md`, `docs/specs/cem-0.2-canonical-binding.md`,
`docs/specs/external-evidence-provider-v0.md`, `docs/specs/external-evidence-provider-v1.md`,
decisions 0309 and 0313, and the feature request Beamfall/corvint#1 (section "CEM / Change
Frontier").

## Agent digest
- Claim: `corvint obligations` writes a reference-only sidecar joining each CEM hunk to the external obligations, tests, and unknowns an impact receipt reports.
- Status: accepted (decision 0313, a delegated call on Beamfall/corvint#1)/experimental (file transport only). Checked by `TestObligationsHunkAssociations` and `TestObligationsCommandRefusesRetiredMap`; since 2026-10-12 the cem/0.2 form admits no map (CEM-V1-210) and the shared join serves only the stable sidecar (CEM-V1-130).
- Exists: `internal/extevidence/obligations.go`, the `corvint obligations` verb in `cmd/corvint/obligations.go`.
- Blocked on: an ACC-V0 provider profile before any executed transport; an independent adopter record before promotion.
- Read next: Definitions; Requirements; Trust boundary, limits, and failure modes.

## User and measurable job

A reviewer reading a Change Frontier wants to see, next to each hunk the frontier cites, what an
external provider says that hunk touches: which entity, which downstream obligation, which test is
declared to verify it, and where the provider has nothing to say. CEM 0.2 rejects unknown fields
(`CEM-CB-002`) and the frontier wire is closed (`CF-V0-018`), so this evidence cannot enter either
document. The job: from one CEM and one saved `corvint impact --provider` receipt, produce one
deterministic sidecar whose hunk ids are exactly the CEM hunk ids a frontier's `relatedIds` cite
and whose `binding.cem_sha256` equals the frontier's `inputs.cemSha256` for the same CEM file, so
that the two documents join by identity while the CEM, the frontier, and every exit code stay
byte-for-byte what they were.

## Verified current state

Before this slice, EEP V0..V2 attached provider relations to `corvint impact` and ETS-V0 used
verification relations for `corvint affected` advice. No surface related provider evidence to a
CEM hunk, and decision 0309 deferred external obligations to a frontier sidecar. The ideas backlog
carried this request as "EEP slice 4".

## Definitions

- **Sidecar**: the `external-frontier-obligations/0` document this verb writes to stdout. It is
  derived state that the operator may save beside `.corvint/change.cem.json`; nothing reads it.
- **Impact receipt**: the complete stdout of one `corvint impact --provider FILE` run, saved as
  given. Its `context.external` section (EEP-V0-003) is the only member the sidecar reads.
- **Association**: one row under a hunk of kind `entity` (an EEP result on the hunk's path),
  `obligation` (an EEP downstream entity one relation hop from such an entity), `test` (an EEP
  verification row on such an entity, naming the test path), or `unknown` (a coded absence).
- **Provider evidence reference**: the relation members `rule` and `reference` with the provider
  id and revision, carried verbatim on every non-unknown association.
- **Reference-only**: the sidecar proves identity (digests, hunk ids, entity ids), integrity (EEP
  path verification state), and freshness (EEP provider freshness) of external references; it never
  states that a record justifies, closes, or scores a hunk.

## Requirements

- `EFO-V0-001`: `corvint obligations` MUST accept `--cem FILE` and `--impact FILE` (both required)
  and `--limit N` (default 64), each as `--flag VALUE` or `--flag=VALUE`. A missing or repeated
  option, an unknown argument, a non-positive limit, an unreadable input, or an input over 4 MiB
  MUST exit 2 with an error envelope and empty stdout. The verb takes no `--root`, reads exactly the
  two named files, runs no Git command, and writes nothing. Amendment 2026-10-28 (V1-3017): an
  input is read through the bounded regular-file reader after its symlinks resolve, so a FIFO or
  other non-regular file is an unreadable input refused without a blocking open, and at most
  4 MiB + 1 bytes are read before an oversized input is refused.
- `EFO-V0-002`: The CEM input MUST decode as a `cem/0.2` document (only `hunks[].id`,
  `hunks[].path`, and `hunks[].disposition` are read), and the impact input MUST decode as a
  receipt with `tool` equal to `impact`; otherwise the verb exits 2 with
  `invalid-obligations-input`. The verb does not verify the CEM:
  the frontier's shared verifier does, and the sidecar carries no CEM authority.
- `EFO-V0-003`: The sidecar has exactly the members `schema`, `id`, `authority`, `state`,
  `state_reason`, `binding`, `providers`, `hunks`, `note`, and `untrusted_text_fields`. `schema` is
  `external-frontier-obligations/0`; `authority` is `external-provider`; `binding` carries
  `cem_sha256` and `impact_sha256` (lowercase SHA-256 of each raw input), `patch_sha256`,
  `base_revision`, `excluded_path`, and `impact_tool`; `id` is `obligations:sha256:` plus the
  SHA-256 of the canonical compact sorted JSON of the document without `id`. `cem_sha256` is the
  raw-copy digest `CF-V0-019` defines for `inputs.cemSha256`, so a frontier over the same CEM file
  joins the sidecar by that value alone.
- `EFO-V0-004`: `hunks` has one row per CEM hunk in CEM order with `hunk` (the CEM hunk id),
  `path`, `disposition`, `associations`, and `omitted`. A result whose `path` equals the hunk path is
  an `entity` association; a downstream row whose relation has such an entity on either side is an
  `obligation`; a verification row on such an entity is a `test` with the test `path`. Kinds are
  exactly `entity|obligation|test|unknown`.
- `EFO-V0-005`: Every non-unknown association carries `authority` `external-provider`, `provider`,
  `entity`, `entity_kind`, `summary`, `relation` (the EEP relation with `rule` and `reference`),
  `verification` (the EEP path verification state), `freshness` (the provider's V0 freshness or its
  V1 root repository freshness), and `reason`. No association carries a closure, justification,
  score, or confidence member, and `note` states the reference-only boundary.
- `EFO-V0-006`: A hunk with no `entity` association carries one `unknown` with code
  `no-external-evidence`. Every provider row whose state is not `loaded` adds one `unknown` with
  code `provider-<state>` to every hunk, and `state` is `partial` (`provider-not-loaded`). A receipt
  without `context.external` yields `state` `unknown` (`no-external-section`) and one `unknown`
  per hunk. Otherwise `state` is `complete`.
- `EFO-V0-007`: The sidecar MUST NOT change any other surface: `corvint frontier` neither accepts
  nor reads it, the frontier wire and CEM wire are unchanged, and a sidecar run creates no file. The
  citation runs from the sidecar to the frontier through `binding.cem_sha256` and the hunk ids.
- `EFO-V0-008`: Associations sort by kind order then a total key over provider, entity, path, code,
  relation type, reference, and endpoints; at most `--limit` rows are kept per hunk and `omitted`
  counts the rest. Identical inputs, and receipts that differ only in row order, MUST produce
  identical bytes apart from `binding.impact_sha256` and `id`.
- `EFO-V0-009`: The sidecar carries no file body, credential, or resolved checkout directory, and
  lists `summary`, `relation.rule`, and `relation.reference` in `untrusted_text_fields`.

## Non-goals and simpler baseline

- Any CEM 0.2 or Change Frontier wire change; a frontier option that reads the sidecar; an
  acknowledgement, closure, or score derived from provider evidence; an MCP surface; a remote or
  executed provider transport; verifying the CEM or the impact receipt against the repository.
- Obligations beyond one downstream hop, and path-to-path (EEP-V2) rows, which name no entity.
- The simpler baseline is reading `context.external` beside the frontier by hand. The sidecar only
  fixes the hunk join and the identity binding so the two can be cited together.

## Trust boundary, limits, and failure modes

The receipt is provider-authored data already labelled `external-provider` by EEP; the sidecar
re-labels nothing and adds no authority. Binding is by digest of raw bytes, so a sidecar over an
edited CEM or receipt no longer matches. Limits: 4 MiB per input, `--limit` associations per hunk,
and the CEM's own hunk bound.

| Condition | Result |
|---|---|
| Option error, unreadable or oversized input | exit 2, `invalid-arguments` or `invalid-obligations-input` |
| CEM not `cem/0.2`, receipt not `impact` | exit 2, `invalid-obligations-input` |
| Receipt without `context.external` | `state` `unknown`, `no-external-section` on every hunk |
| Provider `unavailable` or `invalid` | `state` `partial`, `provider-<state>` on every hunk |
| Hunk path with no result | `no-external-evidence` |
| Stale provider or missing path reference | echoed as `freshness` and `verification`; never hidden |
| Over `--limit` associations | rows cut, `omitted` counted |

## Deterministic acceptance and testing matrix

| Case | Expected | Test |
|---|---|---|
| Real `impact --provider` receipt and a retired cem/0.2 map | exit 2 `unsupported-spec` with the recovery line; no stdout; no file written (2026-10-12 amendment) | `TestObligationsCommandRefusesRetiredMap` |
| Option errors | exit 2 with named message | `TestObligationsArguments` |
| Two hunks, one on the provider path | kinds `entity,entity,obligation,test`; reference-only members | `TestObligationsHunkAssociations` |
| No result, unloaded provider, no external section | coded unknowns and `partial`/`unknown` state | `TestObligationsExplicitUnknowns` |
| Binding and id (stable sidecar since 2026-10-12) | raw digests, map hunk order, recomputable id | `TestCEMV1130StableObligationsAccepts` |
| Reversed rows and limit 1 | identical body; omitted counted | `TestObligationsDeterministicAndBounded` |
| cem/0.x map; cem/1.0 map or broken JSON; affected receipt | `unsupported-spec` with the recovery line; `invalid-obligations-input`; `invalid-obligations-input` (2026-10-12 amendment) | `TestCEMV1210FrozenObligationsAdmitNoMap` |
| Output privacy | no body; untrusted fields listed | `TestObligationsPrivate` |

## Rollout, rollback, and compatibility

Additive and opt-in. Rollback removes `internal/extevidence/obligations.go` and its test,
`cmd/corvint/obligations.go` and its test, the verb's help text and its `topLevelCommands` entry,
this document, its index rows, and decision 0313. No other surface changes in either direction.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `EFO-V0-001` | `parseObligationsInvocation`, `readObligationsInput` in `cmd/corvint/obligations.go` | `TestObligationsArguments`, `TestObligationsCommandRefusesRetiredMap`; V1-3017 bounded input read: `TestObligationsInputReadIsBounded` |
| `EFO-V0-002` | `Obligations`, `joinImpact` in `internal/extevidence/obligations.go` | `TestCEMV1210FrozenObligationsAdmitNoMap` |
| `EFO-V0-003`, `EFO-V0-007` | `StableObligations` in `internal/extevidence/obligations_stable.go`, `digest` in `internal/extevidence/obligations.go` | `TestCEMV1130StableObligationsAccepts`, `TestObligationsCommandRefusesRetiredMap` |
| `EFO-V0-004`, `EFO-V0-005` | `associate`, `fromRow`, `toMap` in `internal/extevidence/obligations.go` | `TestObligationsHunkAssociations` |
| `EFO-V0-006` | `providerSummary`, `obligationsState`, `associate` in `internal/extevidence/obligations.go` | `TestObligationsExplicitUnknowns` |
| `EFO-V0-008` | `boundAssociations`, `associationKey` in `internal/extevidence/obligations.go` | `TestObligationsDeterministicAndBounded` |
| `EFO-V0-009` | `toMap`, `obligationsUntrusted` in `internal/extevidence/obligations.go` | `TestObligationsPrivate` |

## Unresolved decisions and promotion or kill criteria

- Promotion needs one independent adopter's receipt over a real CEM and a reviewer-labelled sample
  showing the joined rows were read as references, not as closure.
- Kill the sidecar if any consumer treats an association as closing a frontier item; the verb would
  then be withdrawn rather than given an authority field.

## Amendment 2026-10-12: the cem/0.2 form admits no map (V1-1278 S2a)

Decision 0525 retires cem/0.x, so the `external-frontier-obligations/0` form, defined over a
cem/0.2 map, composes no sidecar any more (CEM-V1-210). `Obligations` refuses a cem/0.x map
with `unsupported-spec` and the recovery line, and any other input, including a cem/1.0 map, with
`invalid-obligations-input` naming the four stable flags. The hunk join (EFO-V0-004..EFO-V0-006,
EFO-V0-008) moved unchanged into `joinImpact`, which the stable sidecar
(`external-frontier-obligations-stable/0`, CEM-V1-130..CEM-V1-135) reads; its unit tests now
exercise that join directly. The frozen binding/identity test was deleted with the form (0525 §6);
binding and identity are checked on the stable sidecar. EFO-V0-002 and EFO-V0-003 keep their text
as the historical cem/0.2 contract; they constrain no live output. Rollback: revert the V1-1278
S2a commits.
