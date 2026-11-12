# Spec delta: operator-update

## ADDED Requirements

### Requirement: UPD-V0-920 Core apply installs the whole core set

`corvint-update apply --component core` MUST install `corvint` and, on a platform the release
catalog ships app archives for (darwin/arm64, linux/amd64, linux/arm64), every core-set app
(`corvint-docs`, `corvint-docs-mcp`, `corvint-mcp`) from the same release into the same directory.
Each app MUST come from the release asset `<app>_<goos>_<goarch>.tar.gz` whose digest equals its
row in the release `SHA256SUMS`; its root `<app>_<goos>_<goarch>/` MUST hold the executable, a
`SHA256SUMS` that is exactly the executable's row, and only notice files byte-equal to the
`corvint` archive's. Each app MUST pass the platform inspection and its `--version` MUST name the
app and agree with the `corvint` candidate's line (one version, one commit when both are stamped)
before any destination changes. The receipt MUST record every member with its previous digest
(empty when absent) and candidate digest, and apps MUST switch before `corvint`. On a supported
core platform outside that list the apply MUST install `corvint` alone and say so in the result.

#### Scenario: Whole set installed

- **GIVEN** an installed `corvint` build 162 and a release with build 163 and the three app archives
- **WHEN** `corvint-update apply --allow-network --component core` runs
- **THEN** `corvint`, `corvint-docs`, `corvint-docs-mcp` and `corvint-mcp` are at the release's
  digests in the bin directory and the receipt lists the three apps

#### Scenario: App at another version

- **GIVEN** a release whose `corvint-docs` reports another version than its `corvint`
- **WHEN** the apply runs
- **THEN** it refuses naming `corvint-docs` and every destination is unchanged

### Requirement: UPD-V0-921 Whole-set rollback and interrupted-set restore

`rollback` of a receipt with members MUST restore every member: put back its previous bytes, or
remove it when it was absent before, with `corvint` last. It MUST first check that every member is
at its candidate digest or already at its previous state, and refuse with the member named
otherwise, before changing anything. When an apply fails or is cancelled after some members
switched, it MUST restore those members under a bounded cleanup context before returning the
error. A receipt without members MUST roll back `corvint` alone.

#### Scenario: Rollback removes apps that were absent

- **GIVEN** an apply that installed the three apps where none existed
- **WHEN** `corvint-update rollback --component core` runs
- **THEN** `corvint` has its previous digest and the three apps are absent

#### Scenario: Cancelled between members

- **GIVEN** an apply cancelled after the first app switched
- **WHEN** the run returns
- **THEN** it reports an error and every destination has its digest from before the run

### Requirement: UPD-V0-922 Missing app refuses; equal build completes the set

On a platform in the app list, a release that lacks an app archive, or whose `SHA256SUMS` lacks
its row, MUST refuse the apply with the app named and every destination unchanged. When the
candidate `corvint` is the installed build with equal bytes, the apply MUST install every app that
is absent or differs from its candidate and leave `corvint` unrewritten, and MUST answer
`unchanged` only when every member already equals its candidate.

#### Scenario: Release without app archives

- **GIVEN** a release that carries only the `corvint` archive
- **WHEN** the apply runs on linux/amd64
- **THEN** it refuses naming the first missing app archive and nothing changes

#### Scenario: corvint current, docs absent

- **GIVEN** an installed `corvint` equal to the release's and no `corvint-docs`
- **WHEN** the apply runs
- **THEN** the three apps are installed and the action is `applied`, not `unchanged`
