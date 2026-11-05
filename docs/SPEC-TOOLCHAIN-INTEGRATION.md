# Corvint with SpecKit and OpenSpec

Status: **assessment, not an accepted direction.** Produced 2026-08-29 by a three-expert panel
(spec-toolchain analyst, domain-model architect, release/pipeline engineer) against the question:
should Corvint integrate with SpecKit and OpenSpec, and should Corvint itself act as the AI harness that
takes a spec all the way to built-and-tested code?

Superseded in part (2026-10-17): decision 0540 accepts the integration, and
`docs/specs/spec-toolchain-adapters-v0.md` is its contract. The claim below that SpecKit's only
extension point is a prompt template is stale: Spec Kit 1.2.1 has an `extension.yml` manifest with
command hooks. The harness answer stands.

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
**a new specification language** are both listed at `docs/PRODUCT.md:377-379@27e81383`, and
`docs/specs/live-proof-carrying-verification-v0.md:1099-1100@38b12d34` forbids "generating tests, weakening
