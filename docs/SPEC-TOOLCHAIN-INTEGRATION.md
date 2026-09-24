# Corvint with SpecKit and OpenSpec

Status: **assessment, not an accepted direction.** Produced 2026-08-29 by a three-expert panel
(spec-toolchain analyst, domain-model architect, release/pipeline engineer) against the question:
should Corvint integrate with SpecKit and OpenSpec, and should Corvint itself act as the AI harness that
takes a spec all the way to built-and-tested code?

## Answer

**Integrate: yes, and the fit is unusually clean. Be the harness: no.**

The two toolchains own exactly the stages Corvint does not — spec authoring, planning, task
decomposition — and are empty exactly where Corvint is strongest: verification, evidence, provenance,
and refusal. Neither pins a Git revision anywhere. Neither has an abstention state. Neither produces
a machine-verifiable artifact. Neither can tell you whether the code an agent wrote satisfies the
requirement it cited. Both stop at "the artifacts are well-formed and the agent said it's done."

That is a genuine complement rather than an overlap, and it is the whole opportunity.

## Why Corvint must not be the harness

This is not caution. Corvint's arithmetic refuses it, and the refusal is already written down.

A caller's own report of what it executed is `CALLER_REPORTED`, and `CF-V0-014`
(`docs/specs/change-frontier-v0.md:198-202@c291e702`) states that signatures, clean-target statements,
passing rows, and exit zero **cannot upgrade that authority**. If Corvint drives spec -> code -> tested,
Corvint *is* the caller. Every test it ran to satisfy an obligation it generated from a spec it parsed
would enter its own ledger at the weakest authority class and close nothing. Corvint would produce a
permanently open frontier over its own work — or it would upgrade its own evidence in place, which
`TCQ-V0-046` and `CF-V0-014` forbid by name.

Corvint's distinguishing idea is not context. It is a refusal to let the party that produced an
artifact also witness it. Driving the build inverts exactly that: Corvint becomes the producer of the
code, the tests, and the execution, and no second party is left to witness any of it.

The proposition is also already a stated non-goal, three times over: **autonomous code editing** and
**a new specification language** are both listed at `docs/PRODUCT.md:361-363@27e81383`, and
`docs/specs/live-proof-carrying-verification-v0.md:516-517@38b12d34` forbids "generating tests, weakening
assertions, rewriting product code … auto-fixing failures, merging, deploying, or publishing".

One apparent counterexample is not one: `internal/liveverify/gorunner/` really does run `go test`.
But LPCV positions Corvint as owning "identity, composition, conservative scope, provenance, and honest
uncertainty" while "runtime providers remain the authority for what executed"
(`docs/specs/live-proof-carrying-verification-v0.md:53-55@bf0b8fa7`). Corvint executes as an instrument
under an owning verifier's authority. That is the precedent, and it is the opposite of owning the
build.

## The shape that does work

**The harness owns acts. Corvint owns warrants.**

The harness authors, plans, writes, and runs. Corvint says what was known, what is cited, and what
remains unwitnessed — sitting on *both* sides of someone else's edit: the minimum witness before,
the frontier after. SpecKit and OpenSpec are adapted as **intent sources**, never replaced; the
adapter contract already exists at `docs/specs/verified-absence-frontier-v0.md:183-185@76be4742`, which
requires preserving native requirement IDs and lifecycle state.

The translation boundary is one line: `native requirement ID -> ocmIntent{path, blobOID, span,
spanSHA256}`. Corvint pins; it does not author.

## Which toolchain to target first

**OpenSpec**, clearly. Its `Requirement:` / `Scenario:` (WHEN/THEN) grammar is already near-
enumerable; its truth-versus-delta split (`specs/` as current truth, `changes/<id>/specs/` as a
delta) maps directly onto Corvint's base/target revision model; and `openspec schema fork` is a
declared file contract rather than a prompt-template convention.

SpecKit's only extension point is a Markdown prompt template — "write a better prompt". A prompt
cannot carry a verification obligation, so a SpecKit integration can only ever be one-directional:
Corvint reads its `specs/NNN-slug/spec.md` output. That is still worth doing, but it is an import, not
a contract.

Neither tool emits an identifier matching Corvint's requirement grammar
(`^- \`[A-Z][A-Z0-9-]{2,31}-[0-9]{3}\`: `, `internal/lrfrepo/ocm.go:37@13c45269`), so an ID-mapping layer
is unavoidable in both cases.

## The three constraints any integration must respect

1. **CEM 0.1 cannot cite evidence introduced in the same commit** (`docs/DOGFOOD.md:179-182@2d724bbb`). A flow
   where the agent writes the spec and the code together needs a two-commit shape or an explicit
   bootstrap-unknown. This is a hard ordering constraint, not a preference.
2. **Intent identity is a byte span, and specs get edited.** `ocmIntent` pins
   `{path, blobOID, start, end, spanSHA256}`, so ordinary spec editing invalidates every obligation
   bound to it. An **intent-relocation rule** — mirroring CEM's exact-unique evidence relocation — is
   the concrete unbuilt piece this integration actually needs. Without it the union produces mass
   false staleness on day one. This is worth more than a harness.
3. **A test written in the same change is a material hunk requiring its own witness, not a witness
   for the change.** `VAF-007` and `VAF-008` say so directly. Agent-written tests closing
   agent-written obligations is the self-certification trap, and the repo has already reasoned it
   through to a strict answer.

## What is actually missing

Corvint can already gate almost everything: hermetic pinned-toolchain builds, byte-identical
reproducibility, differential parity against an independent oracle, live test execution with three
independent verdict axes. Those are shipped and running in CI.

What does not exist is the **closing decision**. `corvint frontier` is unimplemented, and the harness
`stop` event returns `frontier.state: UNAVAILABLE` and "cannot continue or block the host"
(`docs/specs/agent-harness-integration-v0.md:68-70@3208c7de`). Everything downstream of it is a solved
problem; that one gate is the whole bet.

## Staged adoption

1. **Observe only.** Wire an OpenSpec change to a pinned intent scope; emit a read-only frontier
   report on agent-authored PRs. Block nothing. `verified-absence-frontier-v0.md:135-153` already
   preregisters this experiment's success and kill thresholds — reuse them rather than inventing new
   ones.
2. **First real gate: build and parity, not tests.** Already runs in CI, and its soundness is
   unaffected by whether a human or a model wrote the code.
3. **Live verification with honest axes.** Publish `PASSED|FAILED|INCOMPLETE`,
   `BOUNDED|UNKNOWN`, and `CURRENT|STALE` separately; let nothing collapse them. Early runs will
   legitimately show `PASSED + UNKNOWN`.
4. **Close the loop.** Implement `corvint frontier` and give `stop` a real blocking verdict. This is
   the first stage that can stop an agent, and it should not ship before step 1's signal gates pass.

## Modelling debt to clear first

`authorityClass` currently carries four incompatible value sets — in the MCP bridge
(`internal/mcp/bridge/bridge.go:531@87b754dd`), in the frontier spec
(`change-frontier-v0.md:217`), as a lowercase constant in LRF (`internal/lrf/types.go:12@fc816169`), and as a
seven-value lattice in the dashboard spec. The ordering between them is asserted per context and
defined globally nowhere. Any spec-toolchain integration adds a fifth. One ordered lattice, one
casing, one owner — before the adapter, not after.
