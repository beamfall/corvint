# Clicontract: complete discovery and shared CLI refusals

Owner input: the clicontract lane charter and the full exported records for V1-2685,
V1-3781, V1-3738, V1-3976, V1-3938, V1-3277 and V1-3736, in that order. The immutable base is
`d436dff76de1b8e75c427069ae7f80003ae99bc2`. This records implementation and local evidence;
it does not close native tickets, publish a branch, supply independent review or seal a CEM.
Those operations are outside this lane's authorization.

## Decisions and accepted intent

The new clauses use the lane's reserved number block. Exact `git log --all -S ID --oneline`
checks in the primary repository returned no existing commit for each ID before publication.
Acceptance is the owner's standing approval, not a claim of individual owner read-back.

| Ticket | Requirement | Delivered behavior and witness |
| --- | --- | --- |
| V1-2685 | CAP-V0-7700 | All 56 shipped binaries have an offline command tree; the 38 legacy exceptions are removed. Every node answers both help spellings before accessing inputs. Existing authored help retains its detail; Tasks help stays JSON. Trees, the full-depth node table and root links drift together. `TestCAPV07700ShippedHelp`, `TestCAPV07700EveryBinaryHasTree`, `TestCAPV07700NestedRouteDrift`, `TestCAPV07700MCPNamespaceDoesNotHideTool` and existing synthetic command/flag/binary/MCP omission controls. |
| V1-3781 | CAP-V0-7701 | Refuted at BASE: CAP-V0-972 already parses required leading options. The exact Pulse synopsis is now frozen by `TestCAPV07701PulseRequiredPreamble`; existing `TestCAPV0972RequiredLeadingOptions` retains the other forms. |
| V1-3738 | CAP-V0-7702, TCL-V0-7703 | Payload types and per-mutation templates were already implemented under TCL-V0-610/611 and remain tested. Core/companions share the VERSION release line. The source installer builds all native tools, validates the complete set first, and retains one updater-owned rollback executable per tool. Historical foreign `.bak-*` files are preserved. `TestTCLV0610_HelpNamesPayloadKeyTypes`, `TestTCLV0611_EveryMutationTemplateRuns`, `TestTCLV07703ReleaseLine`, `TestTCLV07703CompleteBoundedInstall` and all-binary version checks. |
| V1-3976 | TCL-V0-7704 | Positional search refusals now name `--text T`. Attempt fields already name valid keys, queue-local gate history already works, and terse progress help already exposes required keys, nullability and phase values. `TestTCLV07704ReadPapercuts` covers all four. |
| V1-3938 | LPCV-V0-7705 | Unknown receipt kinds list accepted documents and producer commands. A plain Playwright report is recognized and routed to a new provider-bound reporter run; no conversion or evidence is invented. `TestLPCVV07705PlainPlaywrightReceiptGuidance` pins the refusal text. |
| V1-3277 | CAP-V0-7706 | Every Tasks verb and nested verb has a generated catalog row with purpose, example, help and tree link. `TestCAPV07700NodeCatalog` proves a missing Tasks row fails. |
| V1-3736 | ACX-V0-7707 | The existing compiled conformance suite now derives every binary from the shipped trees and applies one stderr refusal contract, input/operational exit table, explanatory-code lint and spelling suggestions. Native refusal evidence is retained; successful native JSON and protocol traffic retain their shapes. `TestCLIErrorContracts_ACXV0859`, `TestCAPV07707NativeEnvelopes`, `TestCAPV07707PlainFailureIsOneEnvelope`, `TestACXV07707NativeResultMetadata`, `TestCAPV07707TasksMissingTicketProcess`. |

Presentation is shared at the executable boundary, rather than changing persisted Tasks
records or analyzer/MCP protocol serializers. Single-mode help derives from parser declarations
and authored usage; folded shims reuse their owning command group's tree. Runtime RPC errors
remain protocol messages on stdout even with an explicit root. Shared helpers use only the
standard library and existing presentation packages. Tasks' exact boundary exceptions and
source export include that closure, verified by the existing boundary and export checks.
Native closeout also exposed the private `dogfood-record` entry: it now has an explicit reasoned
internal tree entry, and the compiled refusal suite proves its own argument parser remains reachable.
The native outdated-map reader now matches its exact admitted code/message independently of
additional envelope metadata; its regression rejects every other reason, preserving the replacement fence.

Final catalog inspection exposed mandatory release flags labelled optional, a positional GOROOT
mislabelled stdin, and a delegated MCP parser omitted from the root source route. The generator
now consumes printed usage literals and the delegated parser, and requires authored synopses
for non-protocol roots instead of guessing required inputs from descriptions. Source-owned
synopses retain release/readiness and corpus mode alternatives and paired-process forms.
`TestCAPV07700RootInputs` failed before repair for all eleven inspected roots;
`TestCAPV07700UnauthoredInputsRefused` pins refusal of ambiguous prose inference.
Root positionals now reach their native parser; a relative GOROOT is an operational input,
not an unknown verb, pinned by the tree and compiled process regressions.

The lexical drift check retains explicit limits: dynamic names, semantic option preconditions,
profile membership and the truth of prose are not inferred. Required-option coverage remains
`partial`. Nested fixed-index dispatch retains route context so existing words cannot hide a
new route; literal MCP descriptors are checked independently of their namespace. The generated
node table is a separate complete index beside the authored capability descriptions.

`install-local` is operator-invoked source installation. It starts no companion service and
changes no default authority or deployment profile. There is no host installation or deletion
of historical user backups in this lane. Rollback restores the former entrypoints, version
references and conformance expectations together; persisted records need no migration. The
installer's existing bounded rollback path restores the prior executable.

## Evidence and limits

The required starting `make dogfood-change` ran before edits. Directory impact inputs refused
`unsupported-impact-path-suffix`; those receipts were retained and a second pre-change capture
using source-file paths bound query, context and impact to the BASE tree. Query and impact were
READY. Their full private receipts retain the packet's scope and omissions; READY is not a
completeness claim. Billed tokens, cache use, complete end-to-end cost and independent reviewer
findings are `NOT_OBSERVED`.

`corvint affected --base d436dff76de1b8e75c427069ae7f80003ae99bc2` selected the relevant command,
CLI, wire, catalog, conformance and generator packages. Its retained plan has scope UNKNOWN,
including Makefile/non-Go ownership and language frontiers; it does not justify exhaustive
coverage. Focused Go checks ran through `script/heavy.sh`, one package per command. The native
compiled suite passed across all 56 binaries, including both help spellings on every node,
common version lines, unknown verbs/options/help, missing root values, native failures and the
MCP protocol-error control. Template, read papercut, receipt, installer, tree, framing, wire,
Core help, import-boundary and Tasks source-closure checks passed. The final command list and
outputs are retained in the lane handback/evidence directory and dogfood verification input.

Regression controls observed the missing positional-search remedy and plain-report guidance
before repair. The original synthetic fixture assumed a ratchet exception and was updated to
ship both required trees; its read-only assertion remains. The legacy release companion tests
now exercise the universal tree contract instead of the superseded help-document exception.

Source moves required citation maintenance. Thirty-five anchored citations were relocated by
the existing repinner. Seventy-two legacy Go-provider citations were mapped only across
byte-identical source ranges, then given content anchors and removed from the unpinned-debt
inventory. The line-citation check passes. Binary-building test read scopes now include their
actual presentation dependency closure; no confinement exception or unbounded-reader floor was
added. No `.github` or `.taskman` file changed.

The exhaustive gate, all-package tests/vet, Windows push gate, live external provider
qualification and native ticket writes were not run: the owner requires focused local checks,
forbids publication and Tasks writes, and assigns independent review to the orchestrator.
Frozen ranking/learning evaluations are inapplicable because this changes presentation, not
ranking, evidence admission or learning. The known BASE failure in `script/batch-precheck_test.sh`
(V1-4525) remains untouched. Final CEM binding and dogfood-check follow this implementation
commit; their exact outcome is retained by the native report. Sealing is explicitly excluded.
