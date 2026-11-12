# Spec delta: core-bundled-apps

## ADDED Requirements

### Requirement: CMS-V0-930 The refusal names the install command

The `not-installed` refusal of `corvint docs`, `corvint flows`, `corvint how`, the
`corvint-docs --corpus` core read and the MCP docs bridge MUST name the exact install command
`corvint-update apply --allow-network --component core --bin-dir DIR`, with `DIR` the running
executable's directory after symlink resolution, single-quoted when it needs shell quoting, and
without `--bin-dir` when that directory cannot be read. It MUST also point at the manual core-set
archive install in `docs/INSTALL.md`. The `version-mismatch` refusal MUST name the same command.
The code and exit status of both refusals are unchanged.

#### Scenario: corvint alone

- **GIVEN** `corvint` installed in a directory without `corvint-docs` and none on `PATH`
- **WHEN** `corvint flows map` runs
- **THEN** it exits 2 with code `not-installed` and the message contains
  `corvint-update apply --allow-network --component core --bin-dir` and that directory
