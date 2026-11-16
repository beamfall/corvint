# Revision binding and startup coverage repair

Owner repair charter: r1cleanup, base df315e35056d15c7933b5ff97c605cedf5ef4919;
repair parent 306bc51fbed182b214ba5e6af5599c762cfcf920. Scope is the three
independent findings for V1-3496 and V1-4818. V1-4805 is unchanged.

## Delivery-screen binding (ASD-V0-2201)

The validator previously accepted a profile and CLEAR ticket rows without a
revision binding, including a result for another commit. Dispatch now admits
only the understood taskman-delivery-screen/0 summary form. Both ref and
refCommit must equal the pinned full dispatch SHA. The required count strings,
GitHub observation enum, diagnostic/lane arrays and JSON boolean scan/authority
fields must be present with their supported types. Mutation authority and both
truncation flags must be false. Optional unaudited metadata has only the
false/NOT_OBSERVED form. Unknown summary keys or profiles refuse. Ticket rows
still require exactly the requested IDs once each and only CLEAR, MENTIONED or
NOT_OBSERVED. Missing evidence never defaults to a clear screen.

The closed summary-key allowlist is profile, ref, refCommit, commitsScanned,
scanTruncated, github, prsExported, tickets, human, mutationAuthority, laneHeads,
laneScanTruncated, audited and ticketState. Diagnostic array contents are not
used as delivery authority. The output contract and existing refusal remain
unchanged; the consumer validates the existing Tasks result rather than defining
a new wire shape. The original producer evidence is deliveryScreenValue and its
lane/unaudited augmentation in the local corvint-tasks source checkout.

The tools-group regressions reject the reported minimal profile/ticket result,
a missing refCommit, a wrong commit, an unknown object-shaped binding, future
profile, future summary field and missing scan flag. The existing unknown-verdict
fixture remains. Positive current-profile and snapshot-host cases still pass.
The shell record-reader binding was updated for the changed scratch fixtures.

## Startup wiring and isolation contract (LGP-V0-020/8600, NGI-V0-980/005)

TestNativeCLISwiftReverseImpact invokes the native CLI with a committed SwiftPM
fixture and requires Sources/App/main.swift as a reverse import when Core changes.
It exercises both default and explicit syntax paths without injecting a reader.
TestProfileConstructorsInstallSwiftReader invokes the actual profileRegistries
constructors for default and flows, then inspects their installed reader type.
Reflection is confined to the test: the frozen MCP language boundary precludes a
Swift request, and adding a production accessor solely for a test is unnecessary.
The earlier bridge fixture remains useful for reader behavior but is not treated
as proof of startup installation. Existing Swift/unknown suffix refusals remain.

LGP-V0-020 now describes the provider-free dialect.SwiftPMReader interface and the
isolation check with no exemptions. It no longer requires a removed registration
function or stale exemption. Requirement amendments retain their standing
acceptance provenance; the generated requirement index is refreshed.

## Scope and rollback

No golden, existing assertion, wire field, error/exit code or stderr text changed.
No unrelated code or ticket was repaired. Rollback is this repair commit as a
unit, retaining the finding that an unbound screen is unsafe; do not promote the
old validator as proven. Startup tests and the isolation clarification are
independent of MCP admission expansion, which remains unauthorized.

## Verification

All Go tests use the mandated private TMPDIR and script/heavy.sh, one Go package
per command, with no identity exports or lock-directory override. The installed
affected binary refuses self-binary-stale; the checkout build supplies the plan
before tests. It retains UNKNOWN: 391 selected, 298 excluded, 55 unknowns
(51 LANGUAGE_FRONTIER, 2 NO_SELECTABLE_TEST, 2 UNSELECTED_BINARY_CONSUMER),
14 unresolved packages. The touched CLI packages, shell suite and specindex are
selected. The full fallback is not executed.

The shell tools group passes all 15 groups. New native Swift impact (default and
syntax), MCP startup-reader (default and flows), and unchanged MCP admission
regressions pass. The whole MCP CLI package fails only the existing unavailable
launch deleted_spare/existing_spare fixtures (8.044s): the sandbox exposes a
read-only ancestor /home/russell/lanes/.git that misclassifies scratch roots.
Spec generation, spec-requirements-check (including whole specindex), line
citations and changed-checks pass. Citation repin found no moved content.

CEM binding is unavailable: docs/DOGFOOD.md records removal of the entire workflow
by decision 0594. No replacement, dogfood-change rerun or seal is introduced.
The host owns independent review, broader verification, integration and native
completion. Earlier ticket-level external limits remain open.

The whole core CLI package also ran without a filter (184.919s). It fails the same
seven existing outside-root/profile groups recorded in the earlier round:
TestAHI373MCPVerbOwnsItsArgumentsAndProfile,
TestGPKV0084ImpactCapabilitiesOutsideGitRoot,
TestPRBV0008EveryNamedCommandWritesProfiles,
TestAffectedRejectsNonRepositoryAndExtraArguments, TestWatchHelpAndRefusals,
TestConvertedRefusalDiagnostics and
TestCoreVerbsRefuseAWorkingDirectoryOutsideTheRootAlike. The exact diagnostics
name /home/russell/lanes as the scratch worktree. New Swift cases pass. This is
not a passing whole-package verdict; host reruns without that mount remain due.
No failing assertion or guard was weakened.

Raw evidence is outside the repository under /home/russell/lanes/r1cleanup-evidence/:

| File | Bytes | SHA-256 |
|---|---:|---|
| review-affected.json | 327254 | 6c3d7f7b0c1b8b5c68cd27c218e9a4ae38872def8a2f205e7d8044056e2f63a8 |
| review-lane-tools-final.log | 36 | 37f5fbf2225257afc642c3f0dcb03cfdb90087187232cd82495583d76920e990 |
| review-test-corvint.log | 238979 | 90355a904028d06b32cc6614be2f9b634f60440fa3b5a0b4d2035bb8fa350c2f |
| review-test-cli.log | 19085 | 66d3e3c2a3947d6d9a96fc4f0108809b7443120835dfc125c416d574031e7463 |
| review-spec-citations.log | 1002 | e585a5608952de0a2cb39e6686962138565f7526fcf3e49c1d300afd7c9b7126 |
| review-changed-checks.log | 395 | 89a6ed41b17fdea915aa8a8e23896c227382c44c19b39c16490b79e16ef1a2ee |
