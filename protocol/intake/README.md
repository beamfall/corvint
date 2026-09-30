# Intake wire (experimental)

Apache-2.0; see ../../LICENSING.md. `schema.json` defines shape; `vectors.json` defines valid and rejected shapes.
The Go validator also requires trusted full commit pins, immutable path existence, exact names/types,
unique work-item IDs, and resource limits. Count and identifier fields preserve claims, never authority.

`hostile-twins.json` supplies raw fixtures and explicit reader oracles for the actual author-input
builder battery. It proves deterministic author-side containment conditional on those candidate
records; reader extraction and model immunity are NOT_OBSERVED. See the owning spec and CLI help.
