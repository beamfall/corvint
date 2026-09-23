# Corvint Witness Report V0

Owner: Russell Lewis
Date: 2026-09-12
Intent status: accepted (decisions 0163 and 0290)
Delivery status: experimental (checked by `TestCLIReadVerbsLeaveTheRepositoryByteIdentical`)

## Agent digest
- Claim: corvint witness reports the unwitnessed surface of one committed change range and writes no repository, index, or trace state.
- Status: accepted (decisions 0163 and 0290)/experimental (checked by `TestCLIReadVerbsLeaveTheRepositoryByteIdentical`)
- Exists: `cmd/corvint/witness.go` and `internal/witness` (`Profile = "corvint-witness/0"`).
- Blocked on: none stated for the read-only clause; the report's closure-authority table stays owned by CF-V0-012/014/031, not restated here.
- Read next: Scope; Requirements.

## Scope

`corvint witness` is a standalone report command, not a migrated parity verb and not the
`ocm-change-witnessed-v0` evaluator. `cmd/corvint/witness.go`'s `parseWitnessInvocation` intercepts
the `witness` verb ahead of the shared argument parser specifically so it adds no verb to the
parity-compared top-level command vocabulary `GPK-V0-007` enumerates. `internal/witness`
(`Profile = "corvint-witness/0"`) is its own package: it composes `internal/contextindex` and
`internal/cem/{cemcode,gitrun,wire}` to count closed, unproven, and not-run obligations over one
committed range, citing `CF-V0-012`, `CF-V0-014`, and `CF-V0-031` for its non-closing authority
table. It is not `internal/changewitness`, the `ocm-change-witnessed-v0` evaluator
`docs/specs/change-witness-relation-v0.md` owns; that spec's own Traceability section already states
no Frontier profile, verifier, or command consumes it, and that statement does not describe this
report, which is a distinct package with its own CLI verb.

## Requirements

### Repository read-only guarantee

- `AGW-V0-001`: `corvint witness` MUST be read-only, per AGENTS.md invariant 4: on every path — a
  rendered or `--json` report, an `invalid-arguments`/`output-failed` `gokernel.Error`, or a refused
  invocation before the index builds (an unknown option is named before any value is consumed, and
  an explicit `--root` that is not a Git repository is refused as `not a Git repository: PATH`,
  decision 0205) — it writes no tracked or untracked repository content,
  `.git`, or `.corvint` state, leaving the repository byte-for-byte unchanged. `runWitness` only
  reads the committed tree's index snapshot or, on a miss, builds `internal/contextindex` from the
  Git object store (`IDX-SNAP-V0-020`, a read that writes nothing), and calls `witness.Compile`/
  `witness.Render`; no call in that composition creates, prunes, or mutates persisted state.

- `AGW-V0-002`: Committed-range ancestry MUST follow immutable commit parents. The bounded Git
  runner MUST disable replacement objects and pin `GIT_GRAFT_FILE` to the platform null device,
  ignoring both repository `info/grafts` and ambient graft paths without changing their bytes. A
  graft MUST NOT invent an admitted ancestor or hide a real available ancestor. Graft deprecation
  advice is disabled explicitly; operation/output limits and read-only behavior remain unchanged.

### Packet cost

- `AGW-V0-003`: (proposed 2026-09-23, V1-0200, not accepted) the report MUST carry
  `packetCoverage`, one entry per context packet `Compile` compiled, in compile order: the
  `admission` packet from `contextindex.RangeImpact`, then one `closure` packet per admitted path
  from `contextindex.Impact` (naming that `path`). Each entry copies the packet's own
  `coverage.packet_bytes`, `budget_bytes`, `within_budget`, `included_results` and
  `omitted_results` under those names. A refused stage compiled no packet and contributes no entry,
  so the list is empty, never absent, when nothing was compiled. The text rendering adds a `PACKETS`
  section with the same numbers. The field is additive: `corvint-witness/0` keeps its profile and
  every existing member.

## Non-goals

This spec states the CLI report's read-only guarantee, immutable-parent ancestry boundary and
per-packet cost projection. It does not define, restate, or amend
the `ocm-change-witnessed-v0` closure semantics `change-witness-relation-v0.md` owns, and it adds no
verb to `GPK-V0-007`'s parity-compared vocabulary.

## Traceability and rollback

| Requirement | Planned implementation surface | Required evidence |
|---|---|---|
| `AGW-V0-002` (decision 0290) | `internal/witness` bounded Git runner | `TestBaseTreeIgnoresGraftedAncestry` |
| `AGW-V0-003` (proposed) | `internal/witness` `packetCoverage`, `Report.PacketCoverage`, `renderPackets` | `TestPacketCoverageEqualsEveryCompiledReceipt` |
| `AGW-V0-001` (accepted 2026-09-12, decision 0163) | `cmd/corvint/witness.go` and `internal/witness` | `cmd/corvint`: `TestCLIReadVerbsLeaveTheRepositoryByteIdentical`, subtest "witness refuses an unknown base without reading further" (CLI-level repository-byte assertion); pre-index refusals: `TestParseWitnessOptionsNamesAnUnknownOptionBeforeItsValue`, `TestParseWitnessInvocationRefusesARootThatIsNotARepository` |

Rollback of AGW-V0-002 removes the graft-isolation runner change and its clause; the read-only
AGW-V0-001 guarantee remains. Rollback of AGW-V0-003 removes the `packetCoverage` member and the
`PACKETS` section; readers never required them. Retiring the report removes both requirements and this document.
