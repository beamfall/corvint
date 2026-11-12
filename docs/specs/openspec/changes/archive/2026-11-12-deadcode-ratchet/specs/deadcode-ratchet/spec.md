# Spec delta: deadcode-ratchet

## ADDED Requirements

### Requirement: DCR-V0-001 Pinned analyzer

`make deadcode` MUST install `golang.org/x/tools/cmd/deadcode` at one exact version, named in one
Makefile variable, built for the host, into a scratch directory that the target removes when it
ends. The target MUST NOT use `@latest`, MUST NOT use an analyzer found on `PATH`, and MUST NOT
change `go.mod` or `go.sum`. When the version cannot be fetched or verified the target MUST fail.

#### Scenario: Version unavailable

- **GIVEN** a host that cannot fetch or verify the pinned version
- **WHEN** `make deadcode` runs
- **THEN** it exits non-zero and names the version, and no other analyzer is run

#### Scenario: Module files unchanged

- **GIVEN** a clean worktree
- **WHEN** `make deadcode` finishes
- **THEN** `go.mod` and `go.sum` have no diff and the scratch directory is gone

### Requirement: DCR-V0-002 Six configurations over every package

The target MUST run the analyzer with `-test` and `CGO_ENABLED=0` once for each of `linux/amd64`,
`linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64` and `windows/arm64`. Each run MUST
analyse every package of the main module, including packages under directories whose names begin
with a dot. When a run fails, or loads no package, the target MUST fail and name the configuration;
it MUST NOT compute a report from fewer than six runs.

#### Scenario: One configuration does not type-check

- **GIVEN** a tree that does not compile for `windows/arm64`
- **WHEN** `make deadcode` runs
- **THEN** it exits non-zero, names `windows/arm64`, and prints no verdict

#### Scenario: Dot-directory caller

- **GIVEN** a function whose only caller is a package under `.github/`
- **WHEN** `make deadcode` runs
- **THEN** the function is not reported

### Requirement: DCR-V0-003 Identity and intersection

A function's identity MUST be its package import path and the analyzer's name for it, without a
file or line. The report MUST be the set of identities that every one of the six runs reports.
Functions in generated files MUST NOT be reported.

#### Scenario: Unreachable on one operating system only

- **GIVEN** a function that the `windows` runs report and the `linux` and `darwin` runs do not
- **WHEN** the report is computed
- **THEN** the function is absent from it

#### Scenario: Function moved inside its package

- **GIVEN** a baselined function that moves to another file of the same package
- **WHEN** the report is computed
- **THEN** its identity is unchanged and the verdict does not change

### Requirement: DCR-V0-004 Strict, sorted baseline

`tools/deadcode-ratchet/baseline.json` MUST carry the profile `corvint-deadcode-baseline/0` and a
list of entries, each with `package`, `func` and `reason`, sorted by package then function, with
no duplicate. The tool MUST refuse a baseline that has an unknown field, a wrong profile, a
duplicate entry or an unsorted list. A test MUST assert the number of entries in the committed
baseline.

#### Scenario: Unsorted baseline

- **GIVEN** a baseline whose entries are not in order
- **WHEN** the tool reads it
- **THEN** it exits non-zero and names the first entry that is out of order

#### Scenario: Entry added without changing the count

- **GIVEN** a commit that adds a baseline entry and leaves the asserted count alone
- **WHEN** the tool's tests run
- **THEN** the count test fails

### Requirement: DCR-V0-005 Shrink-only verdict

The tool MUST exit 0 when the report equals the baseline. It MUST exit non-zero and name each
function that is in the report and not in the baseline, with a declaration position. It MUST exit
non-zero and name each baseline entry that is not in the report. `make deadcode` MUST NOT rewrite
the baseline.

#### Scenario: New unreachable function

- **GIVEN** a commit that adds an exported function with no caller in any binary or test
- **WHEN** `make deadcode` runs
- **THEN** it exits non-zero and prints the function's package, name and position

#### Scenario: Stale entry

- **GIVEN** a baselined function that a later commit deletes or starts calling
- **WHEN** `make deadcode` runs
- **THEN** it exits non-zero, names the entry and says to remove it from the baseline

### Requirement: DCR-V0-006 Hermetic proof

The tool's tests MUST run without the analyzer, without loading the module and without the
network. They MUST cover: a function reported by one run only, a new function, a stale entry, a
refused baseline of each kind in DCR-V0-004, a missing run, and the committed baseline's count.

#### Scenario: Tests on a host with no network

- **GIVEN** a host with no network and no analyzer installed
- **WHEN** the tests of `tools/deadcode-ratchet` run
- **THEN** they pass

### Requirement: DCR-V0-007 Fail-closed deletion

A function MAY be deleted only when the report regenerated at the change's base lists it. A
listed function MUST be kept, with a non-empty baseline `reason`, when it is declared under
`protocol/**`, carries or is the target of `//go:linkname` or `//export`, is named by an assembly
file, is cited by name under `docs/specs/**` or in a top-level `docs/*.md` file, or when its
removal breaks a build or vet of DCR-V0-008. No other declaration MAY be removed, except an import
left unused.

#### Scenario: Function cited by a spec

- **GIVEN** a listed function whose name a requirement's evidence column cites
- **WHEN** the deletion runs
- **THEN** the function is kept and its baseline entry gives the citation as the reason

#### Scenario: Declaration outside the list

- **GIVEN** a deletion commit that removes a function the regenerated list does not contain
- **WHEN** the removed-declaration check runs
- **THEN** it fails and names the function

### Requirement: DCR-V0-008 Deletion proof

After the deletion, `go build` and `go vet` MUST pass for every package of the module in each of
the six configurations and under each custom build tag present in the tree, `go vet` MUST pass in
`interop/cem01-go`, and the tests of every package that lost a function MUST pass. `make deadcode`
MUST then exit 0 against the updated baseline.

#### Scenario: Helper used only under a build tag

- **GIVEN** a listed function whose only caller is in a file constrained to the `race` tag
- **WHEN** `go vet -tags race` runs after its removal
- **THEN** vet fails, the function is restored and its baseline entry records the tag as the reason
