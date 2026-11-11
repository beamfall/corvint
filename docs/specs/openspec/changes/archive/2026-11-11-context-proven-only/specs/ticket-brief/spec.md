## MODIFIED Requirements

### Requirement: TBC-V1-009 Path brief needs no ticket

`corvint context --path PATH [--path PATH ...]` MUST build a brief for the named paths from the
committed tree at HEAD, with no ticket, no task text and no Tasks store, and MUST name that commit.
It MUST refuse a path outside the repository with a typed refusal and exit 2. It MUST NOT accept
`--task`, `--base` or a diff: the report for a change range is `corvint impact`. It MUST refuse
`--path` together with `--ticket`, exit 2. A directory stands for the tracked files beneath it,
and their items are merged under the limit of TBC-V1-010. The sections are printed once per
`--path`, in the order given. A path absent from HEAD is not refused and the command exits 0: its
`instructions` section is answered for its directory, and its `tests` and `requirements` sections
are `NOT_OBSERVED` with reason `path-not-at-head`.

#### Scenario: Path absent from HEAD

- **GIVEN** a path that HEAD does not hold, inside a directory that holds `AGENTS.md`
- **WHEN** the agent runs `corvint context --path` for it
- **THEN** the command exits 0, lists the `AGENTS.md`, and marks `tests` and `requirements`
  `NOT_OBSERVED` with reason `path-not-at-head`

### Requirement: TBC-V1-010 Three sections, each from a named link

The path brief MUST carry these sections in this order, each with the state and reason of
CRI-V1-002, a `total` and a `truncated` field, and every item MUST name the link that produced it:

1. `instructions`: each `AGENTS.md` and `CLAUDE.md` in the path's directory and its parents, root
   first. Link: directory ancestry.
2. `tests`: the tests the retained test evidence and the affected plan tie to the path, each with
   its plan reason, and the plan's scope verdict. Link: the evidence record or plan reason.
3. `requirements`: each requirement tied to the path by a kept link (TBC-V1-012). Link: the link
   record.

A section MUST list at most 64 items for each `--path`, ordered by path or ID ascending. The
`requirements` section MUST NOT be `OBSERVED`: with items it is `INCOMPLETE` with reason
`links-not-exhaustive`; with none it is `NOT_OBSERVED` with reason `no-link-for-path`. The brief
MUST NOT carry `owners`, `flows` or `tickets` sections, and MUST NOT match tickets by claim scope,
touch paths or recorded commits. The profile MUST be `corvint-path-brief/1`.

#### Scenario: The cut sections are absent

- **GIVEN** a repository with `CODEOWNERS`, `.corvint/flows.json` and a Tasks store
- **WHEN** the agent runs `corvint context --path` for a tracked file
- **THEN** the brief has exactly the sections `instructions`, `tests` and `requirements`, and its
  profile is `corvint-path-brief/1`

### Requirement: TBC-V1-011 A missing source never blocks

Every source except the Git repository is optional. The optional sources are the retained test
evidence and `.corvint/requirement-links.json`. A section whose sources are all absent MUST be
`NOT_OBSERVED` with a reason that names each missing source. The `tests` section, which has two
sources (retained evidence and the affected plan), MUST keep the items of the source that is
present when the other is absent; with such items it MUST be `INCOMPLETE`, with none
`NOT_OBSERVED`, and the reason names the missing source. The command MUST still exit 0.

#### Scenario: No link file

- **GIVEN** a repository without `.corvint/requirement-links.json`
- **WHEN** the agent runs `corvint context --path` for a tracked file
- **THEN** `requirements` is `NOT_OBSERVED`, its reason names the link file, and the exit is 0
