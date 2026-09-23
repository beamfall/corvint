# External Evidence Provider Transports V0

Owner: Russell Lewis
Date: 2026-09-19
Intent status: accepted (decisions 0316, 0317, 0318)
Delivery status: implemented
Authoritative inputs: `AGENTS.md` invariant 7, `docs/specs/external-evidence-provider-v0.md`,
`docs/specs/external-evidence-provider-v1.md`, `docs/specs/external-evidence-provider-v2.md`,
`internal/procgroup`, and the feature request Beamfall/corvint#11.

## Agent digest
- Claim: `corvint impact` and `corvint affected --provider-command ARGV_JSON` run one contained local provider command and decode its stdout exactly as a record file.
- Status: accepted (decisions 0316, 0317, 0318)/implemented; checked by `TestCommandTransportConformance`.
- Exists: `internal/extevidence/transport.go` and the `--provider-command` option; MCP and optional HTTPS are governed by decisions 0324 and 0325.
- Blocked on: no in-scope delivery prerequisite.
- Read next: Requirements; Trust boundary, limits, and failure modes; Traceability.

## User and measurable job

A provider whose evidence is produced on demand (a documentation system, a coverage exporter, a
test-inventory tool) should not have to be run by hand into a temporary file before every
`corvint impact`. The job is done when the operator can name the provider's executable once, the
receipt is byte-identical to the one the same bytes would produce as a `--provider FILE`, and every
way the command can fail is one closed provider row with no partial record.

## Verified current state

- The file transport reads one bounded record and decodes it with `Decode`/`Decode1`, then applies
  Git-ancestry freshness, per-path reference verification, `learned` exclusion, and the Core-assigned
  `external-provider` authority (`internal/extevidence/section.go`, EEP-V0/V1/V2).
- `internal/procgroup.Run` launches one argv with an absolute executable, an explicit environment,
  fixed stdin, a wall-time bound, stdout and stderr byte bounds, its own process group, and a
  group kill after exit or timeout; it reports each of those facts as an observation.
- Decision 0309 deferred executed providers to "an ACC-V0 profile family". The Analyzer Capability
  Contract governs digest-pinned analyzers whose output becomes Core evidence; a provider command's
  output never becomes Core evidence, so decision 0316 admits it under this narrower profile.

## Definitions

- **provider command**: an operator-supplied argv, given as one JSON array of strings, whose
  complete stdout is one provider record.
- **command source**: the internal provider source produced only by `ParseCommand`; it is reported
  in the receipt as `command:` followed by the compact JSON argv.
- **transport failure**: any outcome in which the command did not run to a clean zero exit within
  its bounds with its process group proven cleaned up.
- **capability declaration**: the optional top-level record member `capabilities`, carried by the
  record bytes themselves on every transport, in which a provider lists the complete set of record
  `schemas` and relation `evidence_kinds` it supports (added 2026-09-22, decision 0351).

## Requirements

- `EEP-TR-001`: The command transport MUST be selected only by `corvint impact --provider-command
  ARGV_JSON` and `corvint affected --provider-command ARGV_JSON` (affected added 2026-09-22). A
  `--provider` value is always a file path, whatever it spells, and never launches a process.
  `--provider-command` MAY repeat; together with `--provider` it counts toward the shared
  four-provider bound and, under `impact`, carries the same incompatibilities (`--base`,
  `--working-tree-untracked`) and the same `--repository` pairing; under `affected` it carries the
  ETS-V0-001 dependencies of `--provider`. No other verb launches a provider command.
- `EEP-TR-002`: `ARGV_JSON` MUST be one JSON array of 1 to 32 strings, at most 4096 bytes, valid
  UTF-8, with no trailing content and no NUL byte in any element; element 0 MUST be an absolute,
  clean executable path. Core makes no `PATH` lookup and invokes no shell. Any defect is an argument
  error before anything is launched.
- `EEP-TR-003`: The command MUST run with only `PATH` and `TMPDIR` (when set) plus `LANG=C` and
  `LC_ALL=C` in its environment, an empty and closed stdin, the repository root as its working
  directory, and its own process group. Corvint grants the command no network access, no
  credential, no `HOME`, and no Corvint state; the executable itself runs with the operator's
  privileges, which the operator accepts by naming it.
- `EEP-TR-004`: Each selected command MUST be launched once, with no retry, under a 10-second wall
  time, a stdout bound equal to the record bound of `EEP-V0-013` (1 MiB), and a 64 KiB stderr
  bound. At the wall-time bound the whole process group MUST be killed.
- `EEP-TR-005`: A command that completes cleanly MUST have its complete stdout decoded by the same
  function, with the same digest, schema dispatch, strict decode, freshness, reference verification,
  `learned` exclusion, repository binding, and Core-assigned authority as a record file. The
  receipt produced from a command MUST equal the receipt produced from the same bytes as a file,
  apart from the provider row's `source`.
- `EEP-TR-006`: Every transport failure MUST yield one provider row in a closed state with an empty
  `sha256`, and nothing from the command's output in `results`, `downstream`, `verification`,
  `path_relations`, or `unknowns`: `unavailable` for did-not-start, timeout, cancellation, a
  non-zero exit, an unobserved exit, or an unproven group cleanup; `invalid` for stdout or stderr
  over its bound. Partial stdout is never decoded. A decode failure of complete stdout is the file
  transport's `invalid` row. No failure changes the core receipt or the exit code.
- `EEP-TR-007`: `impact` with a provider command MUST remain a read command: the receipt reports
  `mutates: false`, and Corvint writes nothing to the repository, the worktree, or trace state.
  What the command itself does is outside Corvint's guarantee and inside the operator's opt-in.
- `EEP-TR-008`: stderr MUST be bounded and discarded: it never appears in the receipt, a reason, or
  Corvint's own stderr. Every provider-row reason for a transport failure is Core-authored text;
  record text reaches the receipt only through the untrusted text fields EEP-V0 already lists.
- `EEP-TR-009`: MCP MUST be selected only through the accepted bounded stdio profile in
  `external-evidence-provider-mcp-v0.md` (decision 0324, superseding 0317). Its extracted record
  bytes retain the strict decode and authority separation of the command/file transports.
- `EEP-TR-010`: A remote (network) transport is NO-GO on the default local path (decision 0318).
  No option, environment variable, or record member may cause Corvint to open a network
  connection to fetch a provider record.

- `EEP-TR-011`: The optional authoring-kit checker MUST require and verify an exact lowercase
  executable SHA-256 before one command launch, reuse `ParseCommand` and the contained runner,
  and reject transport or record pin failures with no output bytes. It MUST cancel on SIGINT or
  SIGTERM and preserve existing timeout, stdout/stderr, environment and descendant cleanup bounds.
  The trusted local executable and parent paths MUST stay immutable between hashing and launch;
  concurrent hostile substitution is outside this local operator trust contract. Executable hashing
  precedes the runner timeout. A saved invocation's expected pins MUST NOT update automatically.
- `EEP-TR-012`: A record on any transport MAY carry one optional top-level member `capabilities`
  with the optional lists `schemas` and `evidence_kinds`, each of at most 32 unique identifiers
  under the `EEP-V0-012` identifier bound. An absent member, or an absent list inside it, is
  undeclared and changes nothing: a record without the member decodes and composes exactly as
  before. A present list, even empty, is the complete set the provider supports. Any other member
  inside `capabilities`, a duplicate, or a non-identifier makes the record
  `invalid` under `EEP-V0-001`. No transport adds a handshake of its own: negotiation is the
  record member.
- `EEP-TR-013`: When a list is declared, Core MUST check it against what the invocation requires
  before anything from the record composes: `schemas` must contain the record's own `schema`;
  under `impact`, `evidence_kinds` must contain every kind a relation in the record uses; under
  `affected`, `evidence_kinds` must contain `declared` or `observed`, the only kinds that qualify
  a selection (`ETS-V0-005`). A missing requirement yields one provider row in the closed state
  `unsupported` whose reason is Core-authored, names the provider id and the first missing
  capability, and carries nothing from the record: no `results`, `downstream`, `verification`,
  `path_relations`, or `unknowns` under `impact`, and `blocked` (`provider-unsupported`) under
  `affected`. The check is deterministic: the same bytes and verb always yield the same row.
- `EEP-TR-014`: A declared capability never widens what Core accepts: an unknown identifier in
  either list is carried as declared and ignored, `learned` stays excluded whether or not it is
  declared, and no declaration changes ranking, authority, freshness, or the core receipt.

## Non-goals and simpler baseline

- The baseline remains running the provider by hand into a file and passing `--provider FILE`; the
  command transport must never produce a receipt that baseline could not.
- No shell, no `PATH` search, no environment pass-through option, no stdin payload, no streaming or
  multi-record protocol, no retries, and no caching of command output.
- No sandbox of the executable's own behaviour: Corvint bounds and contains the process it launches
  but does not claim to confine what an operator-chosen executable does.
- No general-purpose MCP client and no network client in Core.

## MCP profile (accepted bounded stdio)

Decision 0324 supersedes decision 0317 for `external-evidence-provider-mcp-v0.md`. A fixed tool,
bounded interactive exchange and single text block reuse the process-group cleanup and strict
record decoder. Every server-initiated message is refused. The existing record conformance cases
run unchanged through this transport.

## Remote profile (NO-GO on the default path)

Decision 0318's default-path NO-GO remains binding. Decision 0325 accepts the separately built
`corvint-remote-provider` under `external-evidence-provider-remote-v0.md`: explicit per-invocation
consent, normal TLS plus SPKI pinning, private-file credentials, bounded complete bytes and closed
failure. The adapter is an operator-selected command and is never imported or installed by Core.

## Trust boundary, limits, and failure modes

A provider command's output is provider-authored data and never an instruction; its stderr is not
data at all. Core assigns authority and writes every failure reason.

| Failure | Provider state | Reason (Core-authored) |
|---|---|---|
| Executable absent or not executable | `unavailable` | `command did not start` |
| Wall time exceeded | `unavailable` | `command exceeded 10s wall time; process group killed` |
| Invocation cancelled | `unavailable` | `command cancelled` |
| stdout over 1 MiB | `invalid` | `record exceeds 1048576 bytes` |
| stderr over 64 KiB | `invalid` | `command stderr exceeds 65536 bytes` |
| Non-zero exit | `unavailable` | `command exited with status N` |
| Exit not observed | `unavailable` | `command exit not observed` |
| Process group not proven cleaned up | `unavailable` | `command process group not proven cleaned up` |
| Malformed, trailing, or schema-invalid stdout | `invalid` | the file transport's decode reason |
| Invalid `ARGV_JSON` | argument error | nothing launched |
| Declared `capabilities` omit the record schema or a required evidence kind | `unsupported` | `provider ID declares capabilities without schema S` or `... without evidence kind K` |

## Deterministic acceptance and testing matrix

| Case | Expected |
|---|---|
| Every EEP-V0 record, conformance-v1 record, and ETS-V0 `conformance-selection`/`conformance-path` case served by the test binary as the command | section and selection identical to the file transport apart from `source` |
| Partial output then sleep past the bound | `unavailable`, no digest, returns near the bound |
| stdout over 1 MiB; stderr over 64 KiB | `invalid`, no digest |
| Malformed stdout; two JSON documents | the file transport's `invalid` row |
| Non-zero exit; absent executable | `unavailable`, no digest |
| stderr carrying instruction-shaped text with a valid record | record `loaded`; the text absent from the section |
| Probe secret and `HOME` in the parent environment | child sees only the four permitted names, empty stdin, the repository root |
| `command:[...]` passed as `--provider` | read as a file path; `unavailable` |
| `impact --provider-command '["/bin/cat",RECORD]'` | receipt equals `--provider RECORD` apart from `source`; `mutates: false`; worktree unchanged |
| `affected --provider-command '["/bin/cat",RECORD]'` | `test_selection` equals `--provider RECORD` apart from `source`; a fifth provider by command is refused |
| Record without `capabilities` | `Capabilities` nil; state, receipt, and every existing fixture unchanged |
| `capabilities` covering the record's schema and every used kind | `loaded`; receipt equals the undeclared record's |
| `capabilities.evidence_kinds` omitting a used kind, or `schemas` omitting the record schema (including an empty list) | `unsupported`; Core-authored reason; empty `results`, `downstream`, `verification`, `unknowns` |
| `affected` with `evidence_kinds: ["observed"]`; with `["inferred"]` | `narrow-selection-allowed`; `blocked` with `provider-unsupported` |
| Unknown member inside `capabilities`; duplicate entry; empty identifier | `invalid` |

## Rollout, rollback, and compatibility

The option is additive; a caller that never passes `--provider-command` observes no change. Rollback
removes `internal/extevidence/transport.go`, the command branch in `load`, the
`--provider-command` option and help text, this document, decisions 0316 to 0318, and their index
rows. The `decodeRecord` extraction in `section.go` is behaviour-preserving and may stay. The
capability declaration (decision 0351) rolls back on its own by removing `Capabilities` from both
record types, `rootRepository.unsupported`, and `StateUnsupported`; a record carrying the member
then becomes `invalid` under strict decode, and every other record is unchanged.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `EEP-TR-001` | `cmd/corvint/main.go`, `cmd/corvint/affected.go`, `internal/extevidence/transport.go` | `TestImpactProviderCommandFlagParsing`, `TestCommandTransportSelectedOnlyExplicitly`, `TestAffectedSelectionArguments` |
| `EEP-TR-002` | `internal/extevidence/transport.go` | `TestCommandTransportSelectedOnlyExplicitly`, `TestImpactProviderCommandFlagParsing` |
| `EEP-TR-003` | `internal/extevidence/transport.go` | `TestCommandTransportContainment` |
| `EEP-TR-004` | `internal/extevidence/transport.go` | `TestCommandTransportFailuresAreClosed` |
| `EEP-TR-005` | `internal/extevidence/section.go`, `internal/extevidence/selection.go` | `TestCommandTransportConformance`, `TestImpactProviderCommandEndToEnd`, `TestAffectedProviderCommandMatchesFile` |
| `EEP-TR-006` | `internal/extevidence/transport.go` | `TestCommandTransportFailuresAreClosed` |
| `EEP-TR-007` | `cmd/corvint/main.go` | `TestImpactProviderCommandEndToEnd` |
| `EEP-TR-008` | `internal/extevidence/transport.go` | `TestCommandTransportFailuresAreClosed` |
| `EEP-TR-009` | `internal/extevidence/mcp.go`; decision 0324 | `TestMCPTransportConformance` |
| `EEP-TR-010` | this document; decision 0318 | review: no network client in `internal/extevidence` |

| `EEP-TR-012` | `Capabilities`, `validateCapabilities` in `internal/extevidence/record.go`; `Record1` in `record1.go` | `TestCapabilitiesNegotiation`, `TestCapabilitiesDecodeStrict` |
| `EEP-TR-013` | `rootRepository.unsupported`, `decodeRecord` in `internal/extevidence/section.go`; `Selection` in `selection.go` | `TestCapabilitiesNegotiation`, `TestSelectionConformance` |
| `EEP-TR-014` | `rootRepository.unsupported` in `internal/extevidence/section.go`; `evidenceKinds` in `compose.go` | `TestCapabilitiesNegotiation` |
| `EEP-TR-011` | `internal/extevidence/pin.go`, `examples/evidence-provider/v0/check/main.go` | `TestProviderKitCommandPins`, `TestCommandTransportContainment`, `TestCommandTransportFailuresAreClosed`, `TestRunProcessInterruptionLeavesNoDescendant`, `TestSupervisorSignalReapsNestedOwnedGroup` |

The kit checker is additive and experimental (V1-0027); its `/0`, `/1`, `/2` compatibility window
and promotion hold are owned by `EEP-V0-016` through `EEP-V0-018`, and its profile reasons by
`EEP-V0-020`, and its conformance runner, which drives the unchanged `--provider-command` option of
a Corvint binary and adds no transport, by `EEP-V0-021`. Rollback removes only the kit
checker/pin helper and its requirements; no existing command transport changes are necessary.

## Unresolved decisions and promotion or kill criteria

- Promotion to supported requires one independent provider that ships a command producing records,
  run against a real repository, plus the EEP-V0 promotion evidence.
- Kill if a command-transport receipt ever differs from the file-transport receipt for the same
  bytes apart from `source`, or if any transport failure is observed to yield record content.
