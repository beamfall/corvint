## MODIFIED Requirements

### Requirement: IID-V0-001 Explicit inputs under the instructions verb

`corvint instructions --subject PATH --host HOST --host-version VERSION --cwd DIR --profile
default` MUST print the `instruction_load_set` object for the tracked subject, with no task text.
The diagnostic MUST require explicit host, version, cwd, default assumption profile and tracked
subject. Cwd MUST be normalized, repository-relative and an ancestor of the subject. Traversal,
absolute paths, repeat flags and depth over 64 directories MUST refuse. All evidence MUST bind to
the captured tree and source blob; no home, config or session discovery, instruction execution or
remote retrieval occurs. For one release `corvint context --subject PATH --instruction-host HOST
--instruction-host-version VERSION --instruction-cwd DIR --instruction-profile default` MUST print
the same object and one deprecation line on stderr that names `corvint instructions`; it MUST NOT
require `--task`.

#### Scenario: The verb and the alias agree

- **GIVEN** a tracked subject under a directory with `AGENTS.md`
- **WHEN** the agent runs `corvint instructions` and the `context --instruction-*` alias with the
  same inputs
- **THEN** both print the same `instruction_load_set`, and only the alias prints the deprecation
  line on stderr

#### Scenario: A missing input refuses

- **WHEN** `corvint instructions` runs without `--cwd`
- **THEN** it refuses with exit 2 and observes nothing
