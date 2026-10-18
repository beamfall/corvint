# Decision 0540 — Support OpenSpec and Spec Kit as intent sources, 2026-10-17

Date: 2026-10-17. Status: accepted. Authority: owner (chat, 2026-10-17).

## Context

The owner asked that Corvint support the latest OpenSpec and Spec Kit releases and use them in the
build process. On 2026-10-17 the latest releases are OpenSpec `@fission-ai/openspec` 1.14.3
(2026-10-17) and Spec Kit `github/spec-kit` v1.2.1 (2026-10-16). Corvint had no adapter: only the
2026-08-29 assessment `docs/SPEC-TOOLCHAIN-INTEGRATION.md` and the promotion-triggered follow-on in
`docs/specs/verified-absence-frontier-v0.md` ("starts only after three pilot repositories show
demand").

The owner was offered three scopes and chose "Product + CI": Corvint reads both toolchains'
projects as intent sources, ships host packaging that runs Corvint from their workflows, and CI runs
the pinned upstream CLIs against fixtures to catch format drift. Migrating Corvint's own specs onto
either toolchain was offered and not chosen.

## Decisions

### 1. Owner demand replaces the three-pilot trigger

The owner's request satisfies the demand condition of the verified-absence-frontier follow-on for
both ecosystems at once. The frontier signal gates and the "adapter does not replace either authoring
flow" constraint stay in force.

### 2. Adapters are declared OCM intent forms

Both toolchains enter through the `ocm prepare --intent-form` registry (`OIF-V0`), never by content
guessing, and native identity is preserved: OpenSpec requirement names, Spec Kit `FR-`/`SC-` IDs.
The contract is `docs/specs/spec-toolchain-adapters-v0.md` (`STA-V0`).

### 3. Corvint stays the witness, not the harness

Host packaging only runs read-only Corvint commands from the toolchains' own hooks or instructions,
and is advisory. The reasoning in `docs/SPEC-TOOLCHAIN-INTEGRATION.md` ("Why Corvint must not be the
harness") stands.

### 4. Upstream tools stay out of the binary

The default product stays one native-Go binary (invariant 7). Node and Python are used only by the
CI conformance job and the operator's version check, both pinned and checksum-verified.

### 5. Corvint's own spec process is unchanged

Corvint keeps `docs/specs/` and `docs/SPEC-DRIVEN-DEVELOPMENT.md`. Its build process uses the
toolchains through the conformance job, not by authoring Corvint specs in them.

## Correction

The assessment says Spec Kit's only extension point is a Markdown prompt template. That is stale.
Spec Kit 1.2.1 has a versioned `extension.yml` manifest with `before_`/`after_` hooks on its
commands. OpenSpec 1.14.3 still has no hook system; its `config.yaml` `context`/`rules` and
`docs/agent-contract.md` JSON outputs are the integration surface.
