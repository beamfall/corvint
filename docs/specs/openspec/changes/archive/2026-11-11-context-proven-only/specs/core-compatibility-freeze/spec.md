## MODIFIED Requirements

### Requirement: CCF-V1-002 Frozen Core modes

Each frozen Core mode MUST exit 0 and emit one canonical JSON document on stdout with `ok=true`
and `mutates=false`, unless its row says otherwise, and the identifiers of its row byte for byte.
The `context --task` mode MUST NOT be in the frozen set: it is removed from Corvint by owner
direction of 2026-11-11 ("for context we should focus on the parts that are proven to work, and
scrap the rest", decision 0597), and its goldens `context-task.json`,
`context-reserved-rows.json`, `context-downgraded-governing-row.json` and
`context-default-wire.golden` are retired with it. `context` stays a Core verb for the path brief.

#### Scenario: The removed mode refuses

- **WHEN** a caller runs `corvint context --task TEXT`
- **THEN** it gets the usage refusal and exit 2, and the freeze test has no row for that mode
