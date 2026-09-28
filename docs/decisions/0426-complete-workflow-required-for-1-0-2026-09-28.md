# Decision 0426: require the complete workflow for Corvint 1.0

Date: 2026-09-28. Status: accepted owner product scope; implementation and technical amendments remain unqualified.
Tickets: V1-0461..0465, V1-0249, V1-0252..0255.

## Owner instruction

The owner requested a ticket audit for 1.0, then required “the Flows/navigation/docs/MCP features”, directed “build this out. get to 1.0”, and added “Corvint tasks should be able to totally take over the roadmap from Beamfall by 1.0, as well as provide auto documentation”. The owner authorized task creation, subagents and orchestration.

## Decision

1. Stable 1.0 now requires the named Flows, navigation, documentation, MCP and Tasks outcomes in addition to the existing Core qualification. Their prior optional placement cannot justify shipping 1.0 without them.
2. Core remains one local native Go binary and can still be assembled independently. Independently packaged Tasks and MCP capabilities keep their own contracts and qualification; requiring their delivery for the product release does not put a daemon, service, account or network dependency in Core.
3. Full Beamfall roadmap takeover includes planning/read consumers, ticket mutation, claims/leases, independent candidate review and repair, real gates, single- and multi-repository integration, native completion, history/provenance retention, writer fencing and post-cutover recovery. Existing code or a fixture lifecycle is insufficient evidence. Production authority activation requires the final concrete owner approval under standing instructions.
4. Automatic documentation includes useful developer references and user-facing flow documentation from revision-pinned sources and observations. Generated output preserves human prose, separates accepted intent from extracted or observed claims, reports gaps and drift, previews changes, supports safe explicit apply and repeat no-ops, and retains rollback evidence. It never accepts generated intent or publishes itself.
5. Existing Core gates, failed held-out evidence, platform qualification and final promotion approval remain binding. This decision does not authorize stable publication, choose signing, accept unqualified technical profiles, or mark a ticket complete.
6. Unrelated consoles, editors, additional host authority, generalized learning, new analyzer languages, hosted services and unselected experimental renderer promises remain outside the critical path unless a required workflow demonstrates a dependency.

## Evidence and completion

The native release definition is v1-0 revision 6, with 18 explicit required tickets. New ticket boundaries are V1-0461 (release scope), V1-0462 (independent review), V1-0463 (multi-repository integration), V1-0464 (migration/activation/recovery), and V1-0465 (automatic documentation). They remain OPEN. Structural receipt consistency is not runtime qualification.

The source amendment adds PRS-V1-013..017 and updates the surface table. Technical implementation changes must retain their owning specifications, focused tests, independent review and exact candidate evidence. Stable delivery remains blocked until all required outcomes and release gates pass.

## Rollback

Before promotion, an explicit owner scope change can revise the release definition and this decision while retaining history. Do not erase failed evidence, downgrade completed native history, re-enable a retired writer or move a published tag as rollback.
