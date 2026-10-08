# Flow Document Maintenance V0

Owner: Russell Lewis
Drafted: 2026-09-28
Intent status: accepted (decision 0451; V1-0465)
Delivery status: experimental
Implementation: `internal/appflows/docs_maintain.go`, `internal/doccorpus/apply_pair.go`, `cmd/corvint/flows_docs.go`
Authoritative inputs: `docs/specs/application-flow-understanding-v1.md` AFU-V1-030..033 and AFU-V1-036..038; owner-approved V1-0465 bounded maintenance plan.

## Agent digest
- Claim: Explicit flow-document maintenance conditionally publishes a generated page and claims pair with digest-bound previews and committed provenance receipts.
- Status: accepted (decision 0451; V1-0465)/experimental
- Exists: additive preview/apply command, committed receipt and explicit legacy adoption, retained original inodes on replacement.
- Blocked on: independent review and release-boundary qualification (owner acceptance recorded by decision 0451); no general narrative or API inference.
- Read next: Requirements; CLI and failure model; Acceptance and rollback.

## User job and verified current state

Maintain a dedicated generated flow page as committed intents and run evidence change. Preserve
human prose in a separate authored companion. The accepted AFU renderer and claim states stay
unchanged. Existing immediate `flows docs` generation and `flow-doc-claims/0` remain compatible.
The new receipt records operator-selected provenance; it is neither authentication nor accepted
intent. A user who commits and explicitly names a replacement receipt selects that ownership.

## Requirements

- `FDM-V0-001`: `flows docs maintain --preview` MUST write nothing, returning proposed page and
  claims bytes, current destination content identities, immutable source/evidence identities,
  rendering profile, evaluated commit/tree and a canonical SHA-256 proposal digest.
- `FDM-V0-002`: `--apply --expected-proposal SHA256` MUST freshly derive the proposal and refuse a
  digest mismatch before mutation. Encoding failures MUST refuse without a digest. It MUST recheck
  after staging. Changed source bytes, evidence,
  receipt or output cannot authorize the old proposal. Working generation inputs and existing
  outputs/receipt MUST match committed bytes; generation never silently uses dirty source. A target
  absent from Git MUST also be physically absent through confined non-symlink parents, including
  ignored files, directories and symlinks.
- `FDM-V0-003`: Both outputs absent permits exclusive creation. Both present requires an explicitly
  named committed `flow-doc-maintenance/0` receipt matching paths and SHA-256 output hashes, or
  explicit `--adopt` of a committed generated-shaped page and valid claims. Adoption MUST retain
  historical provenance `UNKNOWN`. Headers and sidecars alone never authorize replacement. Mixed
  existence, duplicate paths, hard-link aliases, symlinks and authored output MUST refuse.
- `FDM-V0-004`: Stage both files before publication, pin parents, capture original inodes and retain
  named recovery paths. Publish using no-clobber links. Second-file failure, cancellation or detected
  writer interference MUST return nonzero partial state and no success receipt. Recovery MUST NOT
  overwrite a competing writer. A held-open writer's original inode MUST remain reachable.
- `FDM-V0-005`: Only after both publications succeed may stdout contain the canonical versioned
  maintenance receipt with profile, source/evidence identities, output hashes, proposal digest and
  optional previous-receipt digest. Before any filesystem write, the exact prospective receipt,
  including allocated publication/recovery names, MUST encode and decode within the shared bound.
  Return those preflighted bytes only after both publications succeed; do not serialize for the
  first time after publication. The operator saves and commits it with the pair. A failed stdout
  write MUST report that receipt delivery failed; recovery remains and explicit adoption is needed
  for an unreceipted pair. Receipt integrity is not authentication or historical rederivation.
- `FDM-V0-006`: With identical material inputs, options and outputs, apply MUST return the previous
  receipt byte-identically. Unrelated HEAD movement alone MUST NOT replace that receipt. Changed
  input identities may issue a new receipt even when rendered bytes equal the previous pair.
- `FDM-V0-007`: Maintenance MUST preserve existing AFU fixed-template rendering and claim semantics.
  Separately authored companion bytes MUST stay unchanged. Existing `flows docs --check` MUST assess
  the newly committed pair at its current HEAD, preserving stale, unproven and contradicted evidence.

## CLI and failure model

```sh
corvint flows docs maintain --flows flows --page docs/flows.md --claims docs/claims.json --preview
corvint flows docs maintain --flows flows --page docs/flows.md --claims docs/claims.json --apply --expected-proposal SHA256
# Save stdout as a receipt, then commit page, claims and receipt together.
corvint flows docs maintain --flows flows --page docs/flows.md --claims docs/claims.json --receipt docs/receipt.json --preview
```

Existing `--evidence`, `--registry` and `--docs-root` retain their meaning. Parent directories must
already exist. `--adopt` and `--receipt` are mutually exclusive. `--check` and `--waivers` stay with
existing `flows docs`, not maintenance. Canonical digests hash canonical JSON including an empty
`digest` member; previous-receipt digest hashes its complete canonical bytes. Evidence paths and
raw SHA-256 identities are retained; receipt validation does not require absent historical evidence.

This is two sequential conditional publications, not a pair-atomic or crash-atomic transaction.
There is an absent-name window during each original capture. Ordinary failures report per-file
published state and recovery paths; abrupt process death may leave stage/recovery paths without
stdout. No hidden receipt ledger or automatic rollback exists. Recovery files deliberately retain
original inode identity, so editor writes may change their bytes. Restore only after inspecting
current destinations; never blindly replace an intervening writer. Concurrent modification after
the final checks remains possible. Save failed stdout externally and inspect the pair before adoption.

Existing AFU input, evidence and Markdown limits apply; maintenance adds at most 4096 source
identities. Its complete encoded proposal (including digest), each output and the complete encoded
receipt use the shared 4 MiB corpus encoder/safe-reader bound. This narrower additive profile does
not reduce the existing AFU 5 MiB input bound. Source identities cover the intent
subtree, declared link targets, registry and committed companion Markdown. Missing targets remain
explicit `ABSENT` identities and do not become proven facts. No filesystem watcher is started.

## Non-goals and trust

No source selector, daemon, renderer, general API extraction, authored prose synthesis, in-page
preservation, historical receipt authentication, runtime proof from replay, MCP worktree widening,
or HDC promotion. Plain-repository docs MCP parity remains an independent qualification; linked
worktree `.git` markers stay excluded under MCPV0-001. Receipt hashes detect inconsistency, not a
malicious operator who fabricates a fully consistent record and commits it.

## Acceptance and rollback

Run the compiled CLI create → save and commit receipt/page/claims → change a declared outcome →
preview → apply → commit → existing check against the new HEAD. Compare companion bytes and
byte-identical repeat receipts. Retain actual source replay identity and evidence limitations.
Exercise dirty/changed inputs, outputs and receipts, stale digest, adoption, aliases, second-file
failure, cancellation, competing publication, held-open writer and recovery bytes. Full release
validation is deferred to the integrated release boundary, not replaced by focused checks.

| Requirements | Implementation | Evidence |
|---|---|---|
| FDM-V0-001..003 | `internal/appflows/docs_maintain.go` | `TestFlowDocMaintenanceLifecycle`, `TestFlowDocMaintenanceRefusals`, `TestFlowDocMaintenanceAdoption`, `TestFlowDocMaintenanceOversizedProposal`, `TestFlowDocMaintenanceAbsentIgnoredInput`, `TestFlowDocMaintenanceDerivationRaces` |
| FDM-V0-004..005 | `internal/doccorpus/apply_pair.go`, `cmd/corvint/flows_docs.go` | `TestMaintenancePairRecovery`, `TestMaintenancePairPostPublishRecheck`, `TestFlowDocMaintenanceCLIGuards`, `TestFlowDocMaintenanceCompiled`, `TestFlowDocMaintenanceEncodingBounds`, `TestFlowDocMaintenanceReceiptRoundTrip` |
| FDM-V0-006..007 | `internal/appflows/docs_maintain.go` | `TestFlowDocMaintenanceLifecycle`, `TestFlowDocMaintenanceCompiled` |

Rollback removes the additive command/profile and keeps existing immediate generation. Retain
receipts and recovery files as evidence; explicitly restore reviewed prior bytes if needed. Promote
only after the bounded acceptance path and independent review pass and the owner accepts this
profile (intent accepted by decision 0451; promotion remains open). Wider HDC and 1.0 release obligations remain open; none are inferred from this profile.
