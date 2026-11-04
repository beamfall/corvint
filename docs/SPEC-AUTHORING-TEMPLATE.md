# Capability spec authoring template

Before running `make spec-requirements`, fill in the owning spec's header and digest:

```markdown
Owner: Russell Lewis
Date: YYYY-MM-DD
Intent status: proposed
Delivery status: experimental

## Agent digest
- Claim: One sentence of at most 160 characters, including the final punctuation.
- Status: proposed / experimental
- Exists: Exact implementation and evidence.
- Blocked on: Unobserved acceptance evidence.
- Read next: Requirements; Acceptance; Rollback.

## Requirements

- `PREFIX-V0-NNN`: (accepted under the owner's standing auto-accept (2026-10-11); V1-NNNN) Observable behavior.
```

Keep the Claim and the README row consistent: edit the spec's Claim, then stage the spec with
`git add -- docs/specs/NAME.md` and regenerate with `make spec-requirements`. The generator
projects the Claim into README and INDEX; editing only that README cell cannot change it.
See [the spec contract](SPEC-DRIVEN-DEVELOPMENT.md) for the required acceptance, non-goals,
trust boundary, traceability and rollback sections. A template confers no acceptance.

For dogfood inputs, sort the owning-spec manifest with `LC_ALL=C sort -u -o intents.txt intents.txt`.
A lane with no declared intent uses a single `#no-intent-declared` line, not an empty manifest.

The Claim bound is `SRG-V0-763`; the index-only staging refusal is `SRG-V0-954`.
