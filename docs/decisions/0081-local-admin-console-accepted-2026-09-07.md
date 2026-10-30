# Decision 0081 — Local Admin Console V0 is accepted; invariant 7 gains a scoped console carve-out

Date: 2026-09-07. Status: accepted. Authority: repository owner, verbatim instruction
"now build the console", following "Corvint should be able to have a dashboard for admin of the task
management ... basicically a full web ui for all the user functionality" and the two binding calls
recorded in `docs/specs/local-admin-console-v0.md` (store first; one admin console over every
user-facing verb in both repositories).

## What this accepts

`docs/specs/local-admin-console-v0.md` moves from `proposed`/`not-started` to accepted intent. It
also answers condition 1 of the `local-observability-dashboard-v0.md` P1 gate; that document's
conditions 2, 3 and 4 are the console spec's own body, §9 and §8 respectively, and are settled
below. The prerequisite the spec put ahead of everything — `corvint-taskman` TCP-02 and TCP-02b, so
that a store exists and holds real tickets — was delivered on 2026-09-07 (corvint-taskman decisions
0003 and 0004).

## U1 — the invariant-7 and PRODUCT.md amendment

Accepted as drafted in the console spec's §9. `AGENTS.md` invariant 7 and the matching
`docs/PRODUCT.md` local-profile paragraph now carry:

> An optional, separately-built, loopback-only, read-through local console may be started
> explicitly by the operator. It is not part of the default local product, installs no service,
> holds no database, opens no outbound connection, and carries no authority; the default install
> remains one native-Go binary with no UI.

This is a carve-out, not a relaxation. The prohibition it amends exists so that the default install
cannot acquire a service, a database, a network dependency or an authority it did not have.
LAC-V0-001..005 hold every one of those closed for the console too: its own opt-in executable never
linked into `corvint`, loopback only, foreground only, no durable state, and no effect on the
measured hot path. An operator who never starts it has the product invariant 7 describes,
unchanged and byte-identical.

The `docs/PRODUCT.md` line that a UI is "an implementation option, not a product outcome" is
retained deliberately and is not amended: the console is exactly that, an option over sources that
already answer, and it earns nothing on its own.

## U2 — the qualification matrix

Frozen as proposed in §8: the two most recent stable major versions of one Chromium-based browser
and one Firefox release channel, on macOS at the host version used for Corvint development. No
support is claimed for any other browser or platform, and a browser outside the matrix is refused
explicitly at load rather than degraded silently.

The delivered S1 surface reduces the exposure this matrix covers: it is server-rendered HTML with
no client-side framework, no build step and no external asset of any kind, so the matrix bounds
what is *claimed*, not what is *relied on*.

## U3 — which repository holds the console

Selected: **Corvint**, as `cmd/corvint-console`.

The boundary argument in the spec favoured `corvint-taskman` because it owns the operative store. The
deciding consideration is the other direction: three of the four panes are Corvint-native. The spec
pane resolves a requirement id through `docs/specs/REQUIREMENTS.tsv` and `docs/specs/INDEX.json`
(LAC-V0-018), the code pane reads content at an immutable Git object id (LAC-V0-019), and stage S3
renders Corvint evidence, dogfood runs and the agent-memory backlogs. Only the board and ticket panes
are taskman-owned, and those are reached through `taskman-command-result/0`, the envelope
`corvint-taskman` publishes precisely so a second program can consume it without linking to it.

Putting the binary in Corvint also keeps the amended invariant and the capability it admits inside
one governance domain: invariant 7 is a Corvint invariant, and a console living outside the
repository whose invariant it amends would be governed at a distance.

Consequence, and the drift signal to watch: Corvint must never link `corvint-taskman`. The console
invokes `atm` as a subprocess and parses its envelope. A Go import in that direction would make the
operative store a build dependency of the context compiler and voids this decision.

Licensing is unaffected: `cmd/corvint-console/**` is AGPL-3.0 like every path outside `protocol/**`.

## U4 — the baseline-beating measurement

Not resolved, and deliberately not claimed. The console spec requires it to beat the two CLIs on
operator time from a ticket to its governing clause and its code at revision, and says the contract
is withdrawn rather than delivered if the CLI turns out to be sufficient. No instrument and no
control arm exist, so no such claim is made here. U4 stays open in the spec's Unresolved section,
and the delivered stages are labelled by what they render, never by an operator-time win.

## Scope delivered under this decision

S1 and the ticket half of S2 only: board, ticket detail, spec pane and code pane over the `atm`
read verbs, plus mutation controls that delegate to `atm`'s own mutation verbs. S3 — Corvint evidence
verbs, dogfood runs, benchmarks and the agent-memory backlogs on the same surface — is not started.

## Rollback

Delete `cmd/corvint-console` and `internal/console`, and revert the two amended paragraphs. Nothing
in `corvint` or `atm` may come to depend on the console; that dependency is the drift signal that
voids this contract, and LAC-V0-001 and LAC-V0-004 exist to make it visible.
