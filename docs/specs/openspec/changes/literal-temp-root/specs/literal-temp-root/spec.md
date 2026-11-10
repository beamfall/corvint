# Spec delta: literal-temp-root

The lane copies each statement below into `affected-plan-selection-v0.md`. Each one becomes a
numbered definition line that begins `(proposed 2026-11-10; delivery not-started; V1-5201)`.

## ADDED Requirements

### Requirement: AFP-V0-107 Temp-root literal occurrence

The Go plugin MUST treat a test-file string literal as proven when it is a trailing argument of
`filepath.Join` or `path.Join` whose first argument is a temp root (AFP-V0-108). It MUST also
treat as proven the right operand of a `+` whose left operand is a temp root. A proven occurrence
MUST NOT contribute its tokens to the unit. (PROD-TEST-001)

#### Scenario: Fixture file under TempDir

- **GIVEN** a test writes `filepath.Join(t.TempDir(), "AGENTS.md")` and no other literal of the
  unit names `AGENTS.md`
- **WHEN** `AGENTS.md` changes
- **THEN** the unit is not selected as PATH_LITERAL_READER

### Requirement: AFP-V0-108 Proven temp root

A temp root MUST be one of the following, and nothing else:

- `TempDir()` called on a `*testing.T`, `*testing.B`, `*testing.F` or `testing.TB` parameter;
- the first result of `os.MkdirTemp`;
- a local variable assigned once from a temp root, whose address is never taken and which no
  closure reassigns;
- a join whose first argument is a temp root;
- a call to a same-package function that takes no root parameter and whose every return yields a
  temp root.

#### Scenario: Helper with a mixed return

- **GIVEN** a helper that returns `t.TempDir()` on one branch and `".."` on another
- **WHEN** a literal is joined to its result
- **THEN** the literal is not proven

### Requirement: AFP-V0-109 Escape screens for a temp root

A temp root MUST lose its proof for every literal joined to it when it reaches any of these:

- `os.Symlink`, `os.Link` or `os.Rename`;
- the destination of `os.CopyFS`;
- an `exec.Command` argument whose command `Dir` is not a temp root;
- a function outside the package that is not on the allowed standard library list.

#### Scenario: Worktree added from the package directory

- **GIVEN** a test runs `git worktree add <tmp>` with no `Dir` and joins `Makefile` to `<tmp>`
- **WHEN** `Makefile` changes
- **THEN** the unit is still selected

### Requirement: AFP-V0-110 All occurrences proven

The Go plugin MUST drop a token from a unit only when every occurrence of it is proven or already
excluded. This covers every file of the unit, under AFP-V0-107, AFP-V0-2950 or AFP-V0-3334. Any
occurrence in a non-test file MUST keep the token.

#### Scenario: Token also in production code

- **GIVEN** a test joins `Makefile` to a temp root and a non-test file of the unit compares a name
  with `"Makefile"`
- **WHEN** `Makefile` changes
- **THEN** the unit is selected

### Requirement: AFP-V0-111 Bounded climb join

A `..` occurrence MUST NOT count as an AFP-V0-012 rule (d) reason when all of these hold:

- it is a leading `..` argument of a join whose other arguments are plain literal components, or
  it is part of a single climbing literal;
- the joined path, resolved from the package directory, lies inside the module root;
- the only consumer is a bounded read (AFP-V0-2950).

Such an occurrence MUST contribute the root-anchored token of the joined path. A path that
resolves at or above the module root MUST stay unbounded.

#### Scenario: Reading one specs file by climbing

- **GIVEN** a test reads `filepath.Join("..", "..", "docs", "specs", "INDEX.json")` from a package
  two levels deep
- **WHEN** `docs/specs/eol-policy-gate-v0.md` changes and nothing else selects the unit
- **THEN** the unit is not selected, and a change to `docs/specs/INDEX.json` still selects it

### Requirement: AFP-V0-112 Bounded repository-root local

A local variable assigned once from `filepath.Abs` of a climbing literal, or from a join of
`os.Getwd()` with `..` literals, MUST be bounded when every use of it is a bounded join under
AFP-V0-2950. Each such join MUST contribute its root-anchored token. Any other use MUST leave the
unit unbounded.

#### Scenario: Root passed to a helper

- **GIVEN** `root, _ := filepath.Abs("../..")` and `root` is passed to a same-package helper
- **WHEN** any path changes
- **THEN** the unit stays an unbounded reader

### Requirement: AFP-V0-113 Comparison-only climb literal

A `..` occurrence whose only consumer is one of the following MUST NOT be a rule (d) reason:

- `==` or `!=`;
- a `case` label;
- the second argument of `strings.HasPrefix`, `strings.HasSuffix` or `strings.Contains`.

A string element of a composite literal MUST stay a rule (d) reason.

#### Scenario: Relative-path guard

- **GIVEN** a test checks `strings.HasPrefix(relative, "..")` and has no other climb
- **WHEN** any path changes
- **THEN** that occurrence is not the unit's rule (d) cause

### Requirement: AFP-V0-114 Every selection reason reported

The plan MUST list, for each selected unit, every rule that would select it, in a sorted
`alsoSelectedBy` list of at most 8 entries with file:line. The plan MUST leave the first witness
field unchanged. A unit that is both a path-literal reader and an unbounded reader MUST show both.

#### Scenario: Hidden unbounded reason

- **GIVEN** a unit with a matching path literal and an unbounded `os.Getwd`
- **WHEN** the literal's path changes
- **THEN** the plan shows PATH_LITERAL_READER first and UNBOUNDED_READER in `alsoSelectedBy`
