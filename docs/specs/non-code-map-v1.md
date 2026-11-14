# Non-code map V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted
Delivery status: partial
Health: KEEP
Authoritative inputs: change folder `docs/specs/openspec/changes/non-code-map/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/non-code-map/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-002, PROD-TEST-008, PROD-TEST-012, PROD-TEST-019) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner answers G2, G3, G15b and G16,
owner decisions R31 and R32).

## Agent digest
- Claim: A changed non-code file runs every test unless a proven, accepted map entry narrows it; some paths always run everything.
- Status: accepted / partial. Change `non-code-map` auto-accepted under G14 (decision 0596); the FIH-V1 proof link waits for V1-5405.
- Exists: `internal/noncodemap` (strict `corvint-data-map/0` reader, entry status, never-narrow list, per-path decision, proposal from recorded reads); the AFP-V0-012 selector prints the decision and falls back to a full run for a path no runner attributes (`unowned-path`); a Go-attributed path stays owned and narrows (orchestrator ruling 2026-11-11); a `Makefile` edit narrows to its changed targets when every changed line is attributable, fail-closed otherwise (NCM-V1-010 to NCM-V1-019).
- Blocked on: V1-5405 (fault-injection harness) for the proof link; until it lands no entry can be `proven`, so no entry narrows.
- Read next: Delivered rules; Traceability.

## User and boundary

An agent or reviewer runs `corvint affected` and `corvint test` on a change that edits a fixture, a
configuration file, a lockfile or a Corvint declaration. Before this change the AFP-V0-012 selector
narrowed such a change to the Go packages its reader rules attribute, and an `UNOWNED_DIRTY_PATH`
entry stayed on the narrowing allowlist (AFP-V0-982), so a path no plugin claims could narrow with
no proof that its readers were all selected.

This spec is the canonical home of requirements NCM-V1-001 to NCM-V1-009, copied from the change's
`specs/non-code-map/spec.md`, and of NCM-V1-010 to NCM-V1-019, the owner-delegated amendment of
NCM-V1-005 for make file edits (2026-11-13, V1-5465). Design decisions, failure modes and rollback stay in the change's
`design.md`, which owns them; this file adds the delivered rules, the orchestrator decision on the
design's first open question, and the delivery record (Surfaces, Traceability).

## Requirements

- `NCM-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Full by default

A changed non-code path that no runner attributes and no proven, accepted entry covers MUST select
every test of every runner in the repository, with reason `unowned-path` and the path printed. A
non-code path a Go rule already attributes is owned and MUST NOT force a full run: it selects its
attribution (the enclosing Go package directory owns the file, so its tests and dependents are
selected; go.mod/go.sum readers; a package's testdata; a declared Go reader). A runner that was
removed or is unavailable MUST select all tests, with reason `runner-removed` and the runner or path
printed. Paths declared as documentation keep the `DECLARED_DOCUMENTATION` class and add no test.
(G15b, G2; owner decision R31 (2026-11-09); owner decision R32 (2026-11-09); orchestrator ruling
2026-11-11 under the owner's 2026-11-02 delegation, recorded in
`docs/build-log/2026-11-11-ncm-owned-paths.md`)

Earlier wording, superseded by the 2026-11-11 ruling: "A changed non-code path without a proven,
accepted entry MUST select every test of every runner in the repository, with reason
`non-code-unmapped` and the path printed. A path no runner owns [...] MUST likewise select all
tests, with a named reason (`UNOWNED_DIRTY_PATH` or `runner-removed`) [...]; `UNOWNED_DIRTY_PATH`
no longer narrows through the allowlist in `internal/gateselect/allowlist.go`."

**Scenario: Fixture change, no entry**
- GIVEN a change to `tests/fixtures/users.json` that no Go package encloses or names, no runner
  owns, and no map entry covers
- WHEN `corvint affected --base FULL_SHA` runs
- THEN every test is selected and the reason names the path

**Scenario: Unowned path**
- GIVEN a change to `tools/notes.xyz` that no runner attributes and no map entry covers
- WHEN `corvint affected --base FULL_SHA` runs
- THEN every test is selected with reason `unowned-path` naming the path

**Scenario: Data beside a package**
- GIVEN a change to `internal/app/data.json`, which the Go package `internal/app` encloses
- WHEN `corvint affected --base FULL_SHA` runs
- THEN `internal/app` and its dependents are selected and no full run is added for that path

**Scenario: Declared documentation**
- GIVEN a change to a path listed in `.corvint/documentation-paths.json` only
- WHEN `corvint affected --base FULL_SHA` runs
- THEN no test is added for that path

- `NCM-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Map entry record

An entry MUST hold `path` (or `glob`), `tests`, `evidence`, `accepted_by`, `accepted_at`,
`pinned_commit` and `proof` (the identifier of the fault case that proved it). (G15b)

**Scenario: Missing proof**
- GIVEN an entry without `proof`
- WHEN the map is read
- THEN the entry is reported `proposed` and narrows nothing

- `NCM-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Recordings only propose

Entries derived from recorded test reads MUST be written as proposals to stdout and MUST NOT narrow
until a person accepts them. (G15b)

**Scenario: Proposal output**
- GIVEN a recorded run that read `config/app.yml` from three tests
- WHEN a proposal is requested
- THEN stdout holds a proposed entry and the map file is unchanged

- `NCM-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Narrowing needs a caught injected fault

An entry MUST narrow only if its `proof` names a fault case in the FIH-V1 non-code set where a fault
in the mapped file was killed and the killing test is within the entry's `tests`. (G15b, G3)

**Scenario: Fault not caught**
- GIVEN an accepted entry whose proving fault killed no test
- WHEN the map is read
- THEN the entry does not narrow and is reported `unproven`

- `NCM-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Paths never narrowed

The following MUST always select everything, whatever the map says: `go.mod`, `go.sum` and every
other lockfile; the toolchain pin; `.github/**`; `Makefile`; runner configuration; release
configuration; generator inputs; build-tag inputs; snapshot update paths; the map file itself;
`.corvint/test-read-scopes.json`; secret and environment configuration. No map entry narrows a
`Makefile`, `makefile` or `GNUmakefile`; NCM-V1-010 to NCM-V1-019 alone decide an edit of one, and
narrow it to its changed targets only when every changed line is attributable, while an included
`*.mk` file stays on this list (owner-delegated amendment 2026-11-13, V1-5465). A `go:embed` input is owned
by its embedding package (NCM-V1-001) and selects that package and its dependents. (G15b;
orchestrator ruling 2026-11-11) No map entry narrows `.corvint/test-read-scopes.json`; NCM-V1-006
alone decides it, and keeps the selector's own entry-by-entry attribution only when that
attribution is provable (owner-delegated amendment 2026-11-13, V1-5465).

Earlier wording, superseded by the 2026-11-11 ruling, listed "`go:embed` and build-tag inputs"
together. Until the 2026-11-13 amendment for V1-5465, every `Makefile` edit selected everything.

**Scenario: Lockfile**
- GIVEN a change to `package-lock.json` and an entry claiming two tests
- WHEN the plan is computed
- THEN every test is selected with reason `never-narrowed-path`

- `NCM-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Corvint configuration runs everything

A change to a Corvint configuration file under `.corvint/`, including the map file, MUST select
everything, with reason `corvint-config` and the path printed. Configuration means every file under
`.corvint/` except: the flow file (`.corvint/flows.json`, or the deprecated `.corvint/e2e-flows.json`)
and its link records, whose change MUST select the tests linked to the changed flows; and the two
private ledgers `self-observations.jsonl` and `unplanned-reads.jsonl`, whose change MUST add no
test; and a sealed CEM directly under `.corvint/changes/` named by a 40- or 64-digit lowercase
hexadecimal commit, which is change evidence, not configuration, and MUST keep the selector's
ADN-V0-011 evidence attribution (its planned readers, else rule (c)). Any other name under
`changes/` stays configuration. The read-scope declaration `.corvint/test-read-scopes.json` and its
literal exemptions `.corvint/test-read-scope-literals.json` are configuration too, but a change the
selector can attribute MUST keep the selector's attribution, printed `read-scope-declaration` with
the path: for the declaration, when the current declaration is valid and the plan's base
declaration is known, only the packages whose entries were added, changed or removed, those whose
declared scope holds the declaration and the declaration's own readers (AFP-V0-050, AFP-V0-9712);
for the literal exemptions, which only the declaration tools read, when the current file is absent
or a valid record, its readers (AFP-V0-9713). A change it cannot attribute MUST select everything
with reason `corvint-config`. (G16, narrowed by owner decision R31 (2026-11-09); sealed CEMs
added 2026-11-12, `docs/build-log/2026-11-12-ncm-sealed-cem-evidence.md`; read-scope declarations
added 2026-11-13 as an owner-delegated amendment under the owner's 2026-11-12 decision that filed
V1-5465, `docs/build-log/2026-11-13-selwalk1113.md`)

Earlier wording, superseded by R31: "A change to any `.corvint/` declaration or to the map file MUST
select everything."

**Scenario: Map edit**
- GIVEN a change that adds one map entry
- WHEN the plan is computed for that change
- THEN every test is selected with reason `corvint-config`

**Scenario: Flow file edit**
- GIVEN a change to `.corvint/flows.json` that edits one flow linked to two tests
- WHEN the plan is computed for that change
- THEN exactly those two tests are added for that path

**Scenario: Ledger append**
- GIVEN a change that only appends to `.corvint/self-observations.jsonl`
- WHEN the plan is computed for that change
- THEN no test is added for that path

**Scenario: One read-scope entry changed**
- GIVEN a change to one package's entries in `.corvint/test-read-scopes.json` and a known plan base
- WHEN the plan is computed for that change
- THEN that package and the declaration's own readers are selected, with the line
  `noncode read-scope-declaration .corvint/test-read-scopes.json`, and with no base every test is
  selected with reason `corvint-config`

- `NCM-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Versioned and strict

The map MUST carry the profile version `corvint-data-map/0` (review docs F17: the first version is
`/0`, as for every other declaration new in 1.0), and a reader MUST refuse an unknown version or
unknown member with `data-map-invalid`. (G15b; strictness mirrors `internal/gateselect/readers.go`)

**Scenario: Newer version**
- GIVEN `corvint-data-map/1`
- WHEN a 1.0 reader loads it
- THEN it exits 2 with `data-map-invalid`

- `NCM-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Stale entries stop narrowing

An entry whose cited test is gone, or whose `pinned_commit` is not an ancestor of the head, MUST NOT
narrow and MUST be reported `stale`. (G15b)

**Scenario: Deleted test**
- GIVEN an entry citing a test that was removed
- WHEN the plan is computed
- THEN the entry is `stale` and the path selects everything

- `NCM-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Deterministic and read-only

Reading the map and planning with it MUST write nothing and MUST give byte-identical output for the
same inputs. (invariant 4; G15b)

**Scenario: Repeated run**
- GIVEN the same commit and map
- WHEN the plan is computed twice
- THEN both outputs are identical and the worktree is unchanged

- `NCM-V1-010`: (accepted under owner delegation 2026-11-13; V1-5465) Make file edits narrow to their changed targets

An edit of a make file GNU make reads by default (`Makefile`, `makefile` or `GNUmakefile`, at any
depth) MUST NOT select everything when the selector attributes every changed line to explicit rules
(NCM-V1-011, NCM-V1-012) and bounds what those rules can run (NCM-V1-013, NCM-V1-014). It then
prints `noncode make-targets <path> -> <targets>` naming the changed targets, keeps the make file's
own Go readers (rule (c), declared and unresolved readers, as for any other path), and adds the
packages of NCM-V1-013 and NCM-V1-014. Qualification (`SelectQualified`, which `corvint test` and
the CI selector run) MUST keep each test-bearing package NCM-V1-013 and NCM-V1-014 add, because its
Go-only graph holds no make edge and so its exclusion certificate cannot remove one; and because
`corvint test` must run nothing the plan does not select (AFP-V0-2403), it MUST select everything,
with `a changed make target runs <pkg>, which the plan does not select (NCM-V1-010)`, when the
plan's receipt (its selected `go:` units and Go provider packages) does not select one (amended
2026-11-13 after the independent review, V1-5465). An edit no target reads (a comment or blank line)
names no target and adds only those readers. Any edit the selector cannot attribute or bound MUST select
everything with reason `never-narrowed-path`, the path and `: make edit: <why>`. An included `*.mk`
file stays never narrowed (NCM-V1-005). (owner decision on V1-5465, delegated, 2026-11-13: widen
NCM-V1-005 for Makefile edits, fail-closed, with misses kept at 0 on the selection-baseline replay)

**Scenario: One recipe changed**
- GIVEN a change to the recipe of target `check`, which runs `go test ./tool`, and a known plan base
- WHEN the selector decides the plan
- THEN it prints `noncode make-targets Makefile -> check` and `make <module>/tool <- Makefile`, and
  does not run everything; qualification keeps `<module>/tool` when the plan selects it and runs
  everything when the plan does not

- `NCM-V1-011`: (accepted under owner delegation 2026-11-13; V1-5465) What a changed make file line is attributed to

Attribution MUST use the driver's own make parse (AZU-V0-001), never a second parser, of the file at
the plan's base, read from immutable Git objects, and at the head. A changed line is attributable
when it lies in an explicit rule's text (its header and prerequisite list, a target-specific
assignment, its recipe, or its `.PHONY` membership), which selects that rule; or when it is a plain
top-level assignment (no `export`, `override`, `private` or `unexport`, outside every conditional
and define block) of a variable whose every use site, at the base and at the head, is an explicit
rule's text or another plain assignment, followed through each such assignment; the variable then
selects every rule that uses it. A use site is any whole-token occurrence of the name, so an
attribution can only widen. A new explicit rule is a changed rule. Lines are compared per rule and
per variable only once the file's statements keep their relative order (NCM-V1-018).

**Scenario: A variable used by two recipes**
- GIVEN a change to `FLAGS = -v`, which only the recipes of `a` and `b` reference
- WHEN the selector decides the plan
- THEN it prints `noncode make-targets Makefile -> a b`

- `NCM-V1-012`: (accepted under owner delegation 2026-11-13; V1-5465) Make file edits that run everything

The selector MUST select everything, naming why, for: an unknown base (no full commit id, or a
`--snapshot` plan); a make file deleted, renamed, new at the head or not a regular file; either side
outside the parsed subset (`$(eval)`, `.ONESHELL`, `.RECIPEPREFIX`, an unbalanced conditional or
define block); a changed include, conditional, define, export, override, private, vpath,
special-target, computed-target or default-goal line, or any other line outside every explicit rule;
a removed rule; a changed pattern rule, double-colon rule or rule any line of which sits inside a
conditional; a changed variable that is a global setting (any name starting with `.` or `MAKE`,
`SHELL`, `VPATH`, `GPATH`, `SUFFIXES`, `CURDIR`, `GNUMAKEFLAGS`, `MFLAGS`), is read by make's
built-in rules, is exported, sits in a file that exports what the driver does not certify, is used
outside every explicit rule (unless NCM-V1-015 evaluates every use), or is used by no rule; a changed assignment that runs a shell (`!=`) or
calls `$(shell)`, `$(error)`, `$(warning)`, `$(info)`, `$(file)`, `$(eval)`, `$(call)`, `$(guile)`
or `$(value)`; and any changed variable in a file that names a variable by a computed reference.

**Scenario: An include line**
- GIVEN a change that adds `include extra.mk`
- WHEN the selector decides the plan
- THEN every test is selected with `never-narrowed-path Makefile: make edit: include line changed: ...`

- `NCM-V1-013`: (accepted under owner delegation 2026-11-13; V1-5465) What the changed targets can run

From each changed target the selector MUST follow the driver units through prerequisites and the
make invocations of recipes, and add, printed `make <pkg> <- <path>`, every Go package whose
directory a reached unit runs or reads, or holds a file it does. The driver's whole-tree pattern
(`./...`, `all`, the module path, or any pattern it cannot place, recorded as `...`) MUST add every
package, the root package among them, and the root itself (`.`) and a root-level file the root
package. The reach MUST select everything when a reached unit runs a command or make goal that does
not resolve, runs a command whose own execution the scan does not follow (NCM-V1-019), sits in a
file outside the parsed subset, or imports a unit the driver units do not hold. A reached unit whose
commands resolve and are followed but read what the graph does not bound (an evaluated, uncertified
computed site) still runs only what it names, so it stays bounded; a whole-tree reader such as `find`
is not followed (NCM-V1-019). NCM-V1-016, NCM-V1-017 and NCM-V1-019 refine which targets and
commands resolve. A go package selector a reached unit runs (a package directory, a `/...` pattern
or a `.go` file, the whole tree aside) MUST name an indexed package (one at or below a `/...`
pattern's directory, or the `.go` file's own); the driver scans a nested make file's text from its
own directory and from the root, and the text runs from one of them, so the selectors of one text
name packages when every selector of either scan does. Otherwise the selector misread the command
or names no package of the repository, and the edit MUST select everything with
`a changed make target runs Go package "<selector>", which names no package of the repository`
rather than drop it, or, when the selector's directory is or lies below one holding a nested
`go.mod`, whose packages the index skips, with
`a changed make target runs Go selector "<selector>" inside a nested module`. (root encodings and
NCM-V1-019 amended 2026-11-13 after the independent review, and the whole-tree read after the round 4
review, V1-5465; the missing-package selector amended 2026-11-13 for V1-5552, and its nested-module
reason after the lane's independent review)

**Scenario: A prerequisite's package**
- GIVEN a changed target `all: check` and `check`, which runs `go test ./tool`
- WHEN the selector decides the plan
- THEN `<module>/tool` is selected

- `NCM-V1-014`: (accepted under owner delegation 2026-11-13; V1-5465) Make targets CI invokes directly

When a job of a workflow under `.github/workflows/` invokes a changed target directly, the selector
MUST add the Go readers of that workflow file, printed `make-ci <pkg> <- <workflow>`; that target's
reach is NCM-V1-013, so a CI-invoked target whose reach cannot be bounded selects everything. A
changed target a local action invokes, or any workflow outside the parsed subset, MUST select
everything.

**Scenario: A CI step's target**
- GIVEN `.github/workflows/ci.yml` runs `make check`, `check` changed, and package `ciread` names
  the workflow file
- WHEN the selector decides the plan
- THEN `<module>/ciread` is selected

- `NCM-V1-015`: (accepted under owner delegation 2026-11-13; V1-5465) Literal word-list variables

A changed plain variable MUST be attributed to the words it adds or removes, instead of to every rule
that uses it, when all of these hold at the base and at the head: its one assignment, the only one
of the name (no target-specific, conditional, define or appended value), is a plain `=`, `:=` or
`::=` with a literal value (no `$` or backslash); the word sets differ; the file enables no
secondary expansion; and every use of the name is read after that assignment and is one of:

1. a reference in the prerequisite list of an explicit rule, outside its recipe, inline recipe and
   target-specific assignments, where the rule is no pattern, `::` or conditional rule and no line
   of it reads an automatic variable of its prerequisites (`$^`, `$+`, `$<`, `$?`, `$|`);
2. a reference in the target list of a computed rule line outside every explicit rule, whose
   prerequisites are literal words and which has no recipe.

Each such reference MUST be `$(name)`, `${name}`, or the last argument of `addprefix`, `addsuffix`
or `patsubst` whose other arguments are literal, evaluated on the changed words alone. The changed
targets are the evaluated changed words, plus the literal prerequisites of each computed line that
evaluates to any. A word in both lists is not selected. The rules of item 1 are touched: what they
run changed only by what the changed words run, so the reach (NCM-V1-013) does not follow them,
but a workflow job or local action that invokes one invokes a changed target (NCM-V1-014). Any
other use, a reordering that changes no word, a non-literal value, `$(shell)` or another function
leaves the variable to NCM-V1-011 and NCM-V1-012. (the repository's `GATE_STEPS`, used as
`gate: gate-receipt-clear $(addprefix ledger/,$(GATE_STEPS))` and
`$(GATE_STEPS) $(addprefix ledger/,$(GATE_STEPS)): | gate-receipt-clear`)

**Scenario: A step added to the gate**
- GIVEN `STEPS = check` becomes `STEPS = check gen`, used only as `gate: clear $(addprefix ledger/,$(STEPS))`
  and `$(STEPS) $(addprefix ledger/,$(STEPS)): | clear`
- WHEN the selector decides the plan
- THEN it prints `noncode make-targets Makefile -> clear gen ledger/gen`, and `check` is not a target

- `NCM-V1-016`: (accepted under owner delegation 2026-11-13; V1-5465) A changed target a pattern rule builds

A changed target no explicit rule names MUST be followed, in the reach of NCM-V1-013, through the one
pattern rule that matches it, with its recipe expanded for that target and stem (`$@`, `$*` and
their `D` and `F` forms). There is no such unit, so the reach selects everything, when no pattern
rule or more than one matches the target, the pattern rule has prerequisites or is a `::` rule, or
a pattern with no slash would match a target in a directory. The instance exists only in the
reach: the planner's units are unchanged.

**Scenario: A ledger step**
- GIVEN `ledger/%:` runs `go vet ./ledger` and `$(MAKE) $*`, and `ledger/gen` changed (NCM-V1-015)
- WHEN the reach follows `ledger/gen`
- THEN it adds `ledger`'s package and what target `gen` runs, not what every target of the file runs;
  the repository's own `ledger/%`, which runs `go run ./ledger`, selects everything (NCM-V1-019)

- `NCM-V1-017`: (accepted under owner delegation 2026-11-13; V1-5465) An environment assignment in front of a command

In the reach of NCM-V1-013, a POSIX `NAME=value` assignment directly in front of the command word,
with no wrapper (`env`, `nice`, `xargs` and the like) between them, MUST NOT by itself make the
command unresolved when its value, quoted or not, is literal (no command substitution, backticks,
parameter expansion or make value the graph does not know, a `$(shell)` result among them) and its
name, compared without case, neither chooses nor loads the program: not `PATH`, `ENV`, `BASH_ENV`,
`SHELLOPTS`, `BASHOPTS`, `IFS`, `CDPATH`, `PS4`, `SHELL`, `GLOBIGNORE`, `HOME`, a make, Java, Perl,
Ruby or Lua option or path name, `CC`, `CXX`, `CLASSPATH`, or a name starting with `LD_`, `DYLD_`,
`GO`, `CGO_`, `GIT_`, `NODE_`, `NPM_`, `PYTHON`, `BASH_FUNC_` or `XDG_`. The command word is then
resolved as before. Such a site stays computed and uncertified for the planner (AFP-V0-3128); only
the make edit's reach reads it as resolved.

**Scenario: A literal reason in front of go test**
- GIVEN a changed target whose recipe is `HEAVY_WHOLE_REASON='x' go test ./tool`
- WHEN the reach follows it
- THEN `<module>/tool` is selected and the reach stays bounded; the same prefix in front of
  `script/heavy.sh go test ./tool` still selects everything, because the script is not followed
  (NCM-V1-019)

- `NCM-V1-018`: (accepted under owner delegation 2026-11-13; V1-5465) Make file statements keep their order

Make expands an immediate (`:=`, `::=`) assignment, a prerequisite list and an include against the
values assigned before it, so moving a statement can change what a target runs while every
per-variable assignment list and every rule's text stays the same. The selector MUST compare the
file's top-level statements in order at the base and at the head, each by its identity (a plain
assignment by its variable's name, an explicit rule line by its targets, any other line outside every
rule by its text, recipes left out), restricted to the identities each side holds equally often,
and MUST select everything, with `the order of make file statements changed at <statement>`, when
the two orders differ. A statement added, removed or repeated a different number of times is a
changed variable, a new or removed rule or a changed line, which NCM-V1-011 and NCM-V1-012
attribute or refuse. A moved first target changes the default goal (NCM-V1-012).

**Scenario: An immediate assignment moved**
- GIVEN `PKG = ./a`, `RUN := go test $(PKG)`, `PKG = ./b` and target `t` running `$(RUN)`, and a
  change that moves `RUN := go test $(PKG)` after both assignments of `PKG`
- WHEN the selector decides the plan
- THEN every test is selected with
  `never-narrowed-path Makefile: make edit: the order of make file statements changed at "assign PKG"`

- `NCM-V1-019`: (accepted under owner delegation 2026-11-13; V1-5465) Commands whose execution the reach does not follow

In the reach of NCM-V1-013, a command MUST count as unresolved unless the scan follows its whole
execution. The rule is closed: the reach follows only

1. `make`, `gmake` or `$(MAKE)` invocations of a make file the driver models, whose goals are the
   unit's edges, with no option but `-C`, `-f` (which the driver models), `-j` and `-l` (with an
   optional count) and the options that change only make's messages, output order or stop on
   failure (`-s`, `--silent`, `--quiet`, `-k`, `--keep-going`, `-S`, `--no-keep-going`, `--stop`,
   `-w`, `--print-directory`, `--no-print-directory`, `-O` and `--output-sync`, the single letters
   combined as in `-ks`). A command-line variable assignment (`SHELL=...`, or one a recipe runs)
   leaves the invocation unfollowed with `make:command-line-assignment:<name>`, and records no
   path; any other option (`--eval`, `-e`, `-n`, `-I`, `-o`, `-W` or one the scan does not know)
   leaves it unfollowed with `make:unfollowed-option:<option>`. Make reads every environment
   variable as a make variable, which overrides a `?=` default, so an assignment in front of the
   command that runs the invocation, directly or through an inline script (`PKG=./tool $(MAKE)
   check`), leaves it unfollowed with `make:environment-assignment:<name>`; make reads every `-f`
   file in order, so a second `-f`, `--file` or `--makefile`, in any spelling, leaves it unfollowed
   with `make:several-makefiles`; and a variable the calling make file exports, for the file or a
   target, is in the environment of every make it calls, so an invocation of any other make file,
   or one whose file the scan cannot name, is unfollowed with `make:exported-variable:<name>`.
   `$(MAKE)` names `make` only in a file that does not assign `MAKE`; in one that does, it runs
   the program that value names, which this rule follows like any other command word;
2. `go` whose first argument is `test`, `vet`, `build` or `list`, with no word beginning `-exec`,
   `-toolexec` or `-vettool` (with one or two dashes), which runs only the packages it names. Any
   global flag before the subcommand (`-C` or any other word beginning `-`) and every other
   subcommand (`generate`, `tool`, `run` and the rest) is not followed;
3. the shell builtins `echo`, `printf`, `true`, `false`, `:`, `test`, `[` and `exit`, which
   cannot execute anything; `set` whose arguments are only `-e`, `-u`, `-x` (alone or combined,
   with `-` or `+`) and `-o pipefail`; and `cd` whose options are only `-L` and `-P` (alone or
   combined) and an optional `--`, followed by exactly one directory operand that is not `-`, which
   the scan resolves as the working directory of what follows. `cd` with any other option, no
   operand (the home directory), `-` (the previous directory) or a second operand is not followed.
   A `cd` inside a `( ... )` subshell sets the working directory until its closing parenthesis;
   one inside a `{ ...; }` group holds after it. A `)` the scan cannot pair with a `(` leaves the
   working directory unknown, so a go selector after it names the whole tree, and the text
   uncertified (`shell:unpaired-group`); and
4. shell syntax that is no command (`if`, `then`, `fi`, `while`, `do`, `done`, `!` and the like).

Every other command MUST leave the target's reach unbounded, so the edit selects everything: a
command word that is a path (a repository script or binary), an interpreter (`sh -c` included), a
wrapper (`env`, `nice`, `timeout`, `xargs`, `exec`, `time`, `command` and the like), `find`, a
sourced file, a `trap`, `export` and every program the driver's scanner calls pure or bounded for
its reads (`sort`, `grep`, `sed`, `awk`, `cat`, `cp`, `mkdir`, `rm` and the rest), whatever its
options: such a command may run tests it does not name, as `sort --compress-program=script/x.sh`
does. A make file whose recipe shell (`SHELL`, for the file or a target, or `.SHELLFLAGS`) is not a
plain shell the driver certifies hands every recipe line to that program, so none of its targets'
commands is followed, with reason `make:recipe-shell-uncertified`. A word holding a command substitution (`$(` or a backtick) leaves it unbounded too, even in a
followed command. The selector MUST print, after the NCM-V1-013 reason, each reason the target's
run set is open, naming the command (`shell:unfollowed-command:<command>`,
`shell:command-substitution:<command>`, `make:unmodelled-file:<file>` or the computed site's own
reason). The mark exists only in the reach: the planner's units and computed sites are unchanged.
(closed rule amended 2026-11-13 after the round 4 review, V1-5465; `cd` options and the recipe
shell amended 2026-11-13 for V1-5552 and V1-5551; the make command line and the subshell's working
directory amended 2026-11-13 after the lane's independent review; the invocation's environment,
makefile list, exported variables and `MAKE` amended 2026-11-13 after the lane's second review)

**Scenario: A script that runs go test**
- GIVEN target `check` runs `script/check.sh`, which runs `go test ./tool`, and a change makes it
  `script/check.sh --race`
- WHEN the selector decides the plan
- THEN every test is selected with
  `never-narrowed-path Makefile: make edit: make target "check" runs a command or make goal that does not resolve: shell:unfollowed-command:script/check.sh`

**Scenario: A bounded program's option runs a script**
- GIVEN target `sorted` runs `sort -S 1 --compress-program=script/check.sh data/lines.txt`, and a
  change edits its arguments
- WHEN the selector decides the plan
- THEN every test is selected with
  `never-narrowed-path Makefile: make edit: make target "sorted" runs a command or make goal that does not resolve: shell:unfollowed-command:sort`;
  `go -C . generate ./gen` gives `shell:unfollowed-command:go -C` the same way

**Scenario: An unrecognized recipe shell**
- GIVEN `SHELL = ./tools/test-shell`, unchanged, where the wrapper runs `go test ./tool`, and a
  change to target `check`'s `echo` recipe line
- WHEN the selector decides the plan
- THEN every test is selected with
  `never-narrowed-path Makefile: make edit: make target "check" runs a command or make goal that does not resolve: make:recipe-shell-uncertified`

**Scenario: cd with options**
- GIVEN target `t` runs `cd -- tool && go test ./b`, and a change edits it
- WHEN the selector decides the plan
- THEN `<module>/tool/b` is selected; `cd -e tool` gives `shell:unfollowed-command:cd` instead

**Scenario: A recursive make command line**
- GIVEN target `top` runs `$(MAKE) SHELL=./tools/test-shell check`, where the wrapper runs
  `go test ./tool`, and a change edits it
- WHEN the selector decides the plan
- THEN every test is selected with
  `never-narrowed-path Makefile: make edit: make target "top" runs a command or make goal that does not resolve: make:command-line-assignment:SHELL`;
  `$(MAKE) '--eval=x: ; go test ./tool' x` gives `make:unfollowed-option:--eval` the same way

**Scenario: A recursive make's environment and makefile list**
- GIVEN a make file holding `PKG ?= ./other`, whose target `top` runs `PKG=./tool $(MAKE) check`,
  where `check` runs `go test $(PKG)`, and a change edits `top`
- WHEN the selector decides the plan
- THEN every test is selected with
  `never-narrowed-path Makefile: make edit: make target "top" runs a command or make goal that does not resolve: make:environment-assignment:PKG`;
  `$(MAKE) -f extra.mk -f Makefile check` gives `make:several-makefiles` the same way

**Scenario: cd in a subshell**
- GIVEN target `t` of `sub/Makefile` runs `(cd tool && go vet .); go test ./c`, and a change edits it
- WHEN the selector decides the plan
- THEN `<module>/sub/tool` and `<module>/sub/c` are selected, not `<module>/sub/tool/c`

## Delivered rules

These rules state how V1-5424 reads the requirements above. They add no requirement.

1. Where the decision is printed. The AFP-V0-012 selector (`internal/gateselect`, read by
   `corvint test` and by `tools/gate-affected-select`) decides whether a run narrows, so it is the
   planner that applies this spec. It prints one `noncode <reason> <path>` line per path a rule
   below decides, one `data-map <key> <status>: <detail>` line per map entry that covers a dirty
   path, and, when any rule needs a full run, the verdict `FALLBACK <reason> <path>` naming the
   first such path in byte order. `corvint test` turns that verdict into a FULL scope, which runs
   every declared language suite, so every test of every runner. The affected plan itself keeps
   its `UNOWNED_DIRTY_PATH` entries and scope `UNKNOWN`, which name the path. A root module
   definition (`go.mod`, `go.sum`, `go.work`, `go.work.sum`) keeps the selector's earlier
   `FALLBACK root module definition is dirty: <path>`, which runs everything first.
2. Order of the per-path rules, first match wins:
   1. the two private ledgers add nothing (`private-ledger`);
   2. the flow file and link records under `.corvint/` (the declared intent directory, when it lies
      under `.corvint/`) add no full run (`flow-linked`): the flow join of `corvint affected`
      (APK-V0-004) selects the linked tests. When the plan reports the flow declaration invalid
      (`FLOWS_DECLARATION_INVALID` in `plan.e2e.unknown`, or `E2E_FLOWS_INVALID`), or names no flow
      join for a changed flow file (`.corvint/flows.json` or the deprecated `.corvint/e2e-flows.json`),
      the changed flows cannot be computed and the path selects everything (`flows-unreadable`);
   3. a declared documentation path under `.corvint/` keeps `DECLARED_DOCUMENTATION`; a read-scope
      declaration edit the selector can attribute keeps its attribution (`read-scope-declaration`,
      no full run); every other path under `.corvint/` selects everything (`corvint-config`), the
      map file and an unattributable read-scope declaration edit included;
   4. a path on the never-narrow list (rule 4) selects everything (`never-narrowed-path`), or
      `runner-removed` when the path is runner configuration the change deleted; a `Makefile`,
      `makefile` or `GNUmakefile` edit the selector attributes adds no full run
      (`make-targets`, NCM-V1-010), and one it cannot attribute prints `: make edit: <why>` after
      the path;
   5. a declared documentation path adds nothing (`DECLARED_DOCUMENTATION`; the existing
      `documentation <path>` audit line);
   6. a path the plan reports as `UNOWNED_DIRTY_PATH` narrows to a `proven` entry's tests when one
      covers it (`proven-entry`); otherwise it is owned, gets no line and keeps the AFP-V0-012
      attribution when a Go rule attributes it; otherwise it selects everything (`unowned-path`).
      Go attribution (`repositoryIndex.attributed`) is: a Go package directory encloses the path
      (rule (b): the enclosing packages, the nearest one's dependents and every embedding
      ancestor's dependents, which covers a package's testdata, data beside it and its go:embed
      inputs); a package's files name it (rule (c)); a declared read scope bears on it
      (AFP-V0-023); or the plan selects a Go unit for it with a declared-reader witness
      (`DECLARED_EXISTENCE_READER`, AFP-V0-980, or `DECLARED_READ_SCOPE`). A map entry that covers
      an attributed path but is not `proven` is still reported on its `data-map` line. Because the
      selector, which holds the Go index, decides this, `UNOWNED_DIRTY_PATH` stays on the AFP-V0-982
      allowlist that `corvint test`, the CI selector and the line-evidence plan share: wherever the
      selector decides the run, an admitted entry is an attributed one. Ownership is the full
      plan's: the qualified selector's Go-only re-observation may report another plugin's source
      (a Playwright spec) as `UNOWNED_DIRTY_PATH`, which the allowlist admits, so it adds nothing;
      nothing is set aside, so no observation's uncertainty is dropped;
   7. every other path is code, or non-code data whose every possible reader the plan already
      proves selected (`goReadData`, AFP-V0-600), or a path an existing reviewed declaration
      classifies (`DECLARED_RUNTIME_READERS`, `DECLARED_NON_SOURCE`, `DECLARED_TEST_OWNER`,
      `TASK_STORE_PROJECTION`); it keeps the existing AFP-V0-012 attribution. Whether those
      hand-written declarations fold into the map is the design's second open question, which the
      owner keeps (proposal, Impact).
3. A runner the plan reports unavailable (`provider.go.state` neither `RUNNABLE` nor
   `EMPTY_SELECTION`) selects everything with `runner-removed go: provider.go.state is <state>`.
4. The never-narrow list (`noncodemap.NeverNarrowed`), matched on the slash path; a base-name rule
   matches at any depth:
   - lockfiles and module files: `go.mod`, `go.sum`, `go.work`, `go.work.sum`, `package-lock.json`,
     `npm-shrinkwrap.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock`, `bun.lockb`, `Gemfile.lock`,
     `poetry.lock`, `Pipfile.lock`, `uv.lock`, `pdm.lock`, `Cargo.lock`, `composer.lock`;
   - toolchain pins: `.go-version`, `.tool-versions`, `.nvmrc`, `.node-version`,
     `.python-version`, `.ruby-version`, `rust-toolchain`, `rust-toolchain.toml`, `mise.toml`,
     `.mise.toml`;
   - `.github/**`;
   - make files: `Makefile`, `makefile`, `GNUmakefile`, `*.mk`;
   - runner configuration: `package.json`, `tsconfig.json`, `tsconfig.*.json`, `vitest.config.*`,
     `vitest.workspace.*`, `vite.config.*`, `jest.config.*`, `playwright.config.*`, `pytest.ini`,
     `pyproject.toml`, `setup.cfg`, `tox.ini`, `conftest.py`, `Gemfile`, `.rspec`, `Rakefile`;
   - release configuration: `.goreleaser.yml`, `.goreleaser.yaml`,
     `tools/release-binary/binaries.json`;
   - generator inputs: `*.proto`, `buf.yaml`, `buf.gen.yaml`, `sqlc.yaml`, `sqlc.yml`,
     `gqlgen.yml`;
   - `go:embed` inputs are not on the list: rule 2.6 attributes a non-Go file below a package to
     the enclosing packages and the dependents of every embedding ancestor. The index records that
     a package embeds, not which files, so every non-Go file below an embedding package is
     attributed to the whole package and its dependents: a sound over-approximation of the exact
     embed patterns, which would need a gateselect scan-memo profile bump to record. Build-tag
     inputs are Go sources, which are code and keep the Go rules;
   - snapshot update paths: anything under a `__snapshots__` directory, `*.snap`, `*.golden`;
   - the map file `.corvint/data-map.json` and `.corvint/test-read-scopes.json` (decided first by
     rule 2.3 as `corvint-config`);
   - secret and environment configuration: `.env`, `.env.*`, `.envrc`, `config/credentials.yml.enc`,
     `config/master.key`, `config/secrets.yml`.
5. Map status, first match wins: `invalid` never reaches status (the reader refuses the map);
   `proposed` when `proof`, `accepted_by`, `accepted_at` or `pinned_commit` is missing; `ignored`
   when the entry's path or glob is on the never-narrow list; `stale` when a cited test is gone or
   `pinned_commit` is not an ancestor of the head (or cannot be read); `unproven` when no FIH-V1
   resolver confirms that `proof` names a non-code fault case in the mapped file that a test in
   `tests` killed; `proven` otherwise. Only `proven` narrows. V1-5424 wires no resolver, so every
   accepted entry with a proof is `unproven` with detail `fault case <proof> is unknown: FIH-V1 is
   not linked (V1-5405)`.
6. A missing map is an empty map: every non-code path runs everything (the design's rollback).
   A map that is not a regular file, larger than 256 KiB, malformed, or that carries an unknown
   profile or member, is refused with `data-map-invalid`: the selector falls back with
   `FALLBACK data-map-invalid: <detail>`, and `corvint affected --propose-data-map` exits 2. It never
   falls back to an empty map. An invalid map therefore forces that fallback, so a full run, on
   every plan of the repository, whatever its dirty paths, until the map is repaired: fail-closed
   by design (review P3-1).
7. Proposal (NCM-V1-003). `corvint affected --propose-data-map RECORDED` reads a recorded-reads
   file (JSON lines `{"test": "...", "path": "..."}`, at most 8 MiB) and writes to stdout a
   `corvint-data-map/0` document holding one entry per read path, with the reading tests sorted,
   `evidence` naming the recording, and no `accepted_by`, `accepted_at`, `pinned_commit` or `proof`,
   so every proposed entry reads `proposed`. It writes no file; a person copies an entry into
   `.corvint/data-map.json` and adds the acceptance and proof.
8. A full run the decision names keeps its reason: `SelectQualified` returns the selector's
   `FALLBACK <reason> <path>` (`noncodemap.NamesFullRun`) before its generic input checks, which
   would otherwise replace it with another full run such as `the selection input has no supported
   scope`.
9. A narrowed make file edit (NCM-V1-010) keeps its `noncode` lines in `SelectQualified`'s audit,
   which otherwise keeps only `data`, `suite` and package lines. Its `make` and `make-ci` packages
   are attributions like any other: `SelectQualified` drops one only with its observation's
   exclusion certificate (AFP-V0-9500). tools/gate-ledger's frozen selector fixture stubs
   `makeedit.go`, so a make file edit there still runs everything.

## Orchestrator decision on the design's open question 1

OWNER-delegated orchestrator call (2026-11-11, recorded by lane ncm under V1-5424): for 1.0,
`.corvint/changes/`, `.corvint/deferred-intents/`, the committed inventory file and the
`.corvint/unplanned-reads.enabled` marker stay configuration: a change to any of them selects
everything with reason `corvint-config`. The design's second open question, folding the
hand-written read declarations into the map, stays open for the owner and is out of scope here.

## Orchestrator ruling on owned paths

Orchestrator decision (2026-11-11, under the owner's 2026-11-02 delegation; the same ruling was given
to lane rnc; recorded in `docs/build-log/2026-11-11-ncm-owned-paths.md`): a non-code path that a Go
rule already attributes is owned and must not force a full run. Go attribution means the enclosing Go
package directory owns the file (its tests and dependents are selected, as `select.go` already
does), go.mod/go.sum readers, a package's testdata, and declared readers. A full run with reason
`unowned-path` follows only when no runner attributes the path at all. NCM-V1-001, NCM-V1-005 (its
go:embed item) and rule 2.6 carry it; the `non-code-unmapped` reason is retired.

## Supersession

The selector now runs everything where these earlier rules narrowed; each test that asserted the
narrowing asserts the named full run instead, and keeps covering the attribution where
`packageBounds` (tools/gate-ledger) still applies it:

- An edit of `.corvint/test-read-scopes.json` was narrowed against the plan's base by rule
  AFP-V0-050; it ran everything (`corvint-config`) from V1-5424 until V1-5465, which restores the
  narrowing when the base declaration is known (`read-scope-declaration`, AFP-V0-9712) and keeps
  the full run otherwise.
- A sealed CEM under `.corvint/changes/` and the retired sidecar `.corvint/change.cem.json` were
  narrowed to their resolving readers by rules AFP-V0-152, AFP-V0-2744, AFP-V0-2746, AFP-V0-3423
  and ADN-V0-011; both now run everything (`corvint-config`, the orchestrator's call above), and
  build-log entries keep their narrowing.
- An edit of `.corvint/gated-root-locators.json` was narrowed by the proof of rule TSP-V0-608; it
  now runs everything (`corvint-config`).
- A Go package under `.github/` was narrowed by rules AZU-V0-005, AZU-V0-9920 and AFP-V0-9012; it
  now runs everything (`never-narrowed-path`).
- An `UNOWNED_DIRTY_PATH` no Go rule attributes now runs everything (`unowned-path`): an unclaimed
  tracked addition outside every package (AZU-V0-9900) is the case a test asserts. Under the
  orchestrator ruling above, the attributed cases of rules CIN-V0-860 (V1-3480), AFP-V0-9013,
  AFP-V0-9310, AFP-V0-980 and AFP-V0-2377 (a hosted fixture under a package's testdata, a deleted
  manifest-named file a declared existence reader reads, a data file beside a package, an embedded
  data file) keep their narrowing, and so does a Go assembly file the plan leaves unowned
  (TSP-V0-006, case "an assembly file"), which its package directory encloses.

## Surfaces

- `internal/noncodemap`: `Parse`, `Read`, `Evaluate`, `NeverNarrowed`, `Decide`, `Propose`.
- `internal/liveverify/affected`: `AttributeMakeEdit`, `ReachMakeTargets` and `ReachMakeEdit`
  (NCM-V1-011 to NCM-V1-019), on the driver's make parse, units and shell scanner.
- `internal/gateselect`: `makeEditAttribution` (`makeedit.go`) reads the base make file and wires
  NCM-V1-010 to NCM-V1-019 into `Decide` as `Input.MakeEdit`.
- `internal/gateselect`: `selectPackagesFrom` applies `Decide` to every plan, with
  `repositoryIndex.attributed` and the plan's declared-reader witnesses as Go attribution;
  `AllowedUnknown` keeps admitting `UNOWNED_DIRTY_PATH`; `SelectQualified` keeps a named NCM full
  run's reason and, through `selectPackagesObliged`, every package a make edit adds, or runs
  everything when the plan does not select one (NCM-V1-010).
- `corvint affected --propose-data-map RECORDED`.

## Delivery state

- Delivered under V1-5424: NCM-V1-001, 002, 003, 005, 006, 007, 008 and 009; the refusal side of
  NCM-V1-004 (no entry narrows without a confirmed fault case).
- Delivered under V1-5465: NCM-V1-010 to NCM-V1-019 (`docs/build-log/2026-11-13-mknarrow1113.md`).
- Amended under V1-5551 and V1-5552: NCM-V1-019 (recipe shell, `cd` options, the recursive make
  command line, the subshell's working directory) and NCM-V1-013 (missing-package selector and its
  nested-module reason) (`docs/build-log/2026-11-13-mk1114.md`).
- Waiting: NCM-V1-004's proof link to FIH-V1 non-code fault cases, after V1-5405 lands (change task 6).
- Acceptance fixtures (change task 7): see the build-log entry
  `docs/build-log/2026-11-11-non-code-map.md` for the fixtures delivered and missing.

## Traceability

| Requirement | Evidence (package) |
| --- | --- |
| NCM-V1-001 | `TestNCMV1001UnmappedFixtureSelectsEverything`, `TestNCMV1001AttributedPathStaysOwned`, `TestNCMV1001UnownedPathSelectsEverything`, `TestNCMV1001DeclaredDocumentationAddsNothing` (also inside an embedding package), `TestNCMV1001RemovedRunnerSelectsEverything` (`internal/noncodemap`); `TestNCMV1SelectorUnownedFallsBack`, `TestNCMV1SelectorEmbeddedInputNarrowsToItsPackage`, `TestNCMV1SelectorUnavailableRunner`, `TestNCMV1001UnownedAttributedOnAllowlist`, `TestNCMV1QualifiedKeepsThePlansOwnership`, `TestNCMV1QualifiedKeepsTheNamedReason`, `TestUnknownPlanNarrowsOnlyThroughLanguageFrontiers_CINV0860` (`internal/gateselect`); `TestDeletedManifestNameSelectsExistenceReaderCLI_AFPV0980`, `TestOwnershipNamedDryRun_AZU_V0_9900` (`cmd/corvint`) |
| NCM-V1-002 | `TestNCMV1002MissingProofIsProposed` (`internal/noncodemap`) |
| NCM-V1-003 | `TestNCMV1003ProposalWritesStdoutOnly` (`internal/noncodemap`); `TestNCMV1003ProposeDataMapCommand` (`cmd/corvint`) |
| NCM-V1-004 | `TestNCMV1004UnprovenEntryDoesNotNarrow` (`internal/noncodemap`) |
| NCM-V1-005 | `TestNCMV1005NeverNarrowedPaths` (go:embed input not listed), `TestNCMV1005LockfileIgnoresEntry` (`internal/noncodemap`); `TestNCMV1SelectorNeverNarrowed`, `TestNCMV1SelectorEmbeddedInputNarrowsToItsPackage` (`internal/gateselect`) |
| NCM-V1-006 | `TestNCMV1006CorvintConfigSelectsEverything`, `TestNCMV1006FlowFileAddsLinkedTestsOnly` (both flow files unjoined), `TestNCMV1006LedgerAppendAddsNothing`, `TestNCMV1006ReadScopeDeclarationNarrowsWhenAttributable_V15465` (`internal/noncodemap`); `TestNCMV1SelectorCorvintConfig`, `TestNCMV1SelectorLedgerAndFlow`, `TestSelectPackagesNarrowsDeclarationEditsAgainstTheBase_AFPV0050`, `TestReadScopeDeclarationNarrowingFaults_AFPV09712`, `TestReadScopeLiteralsNarrowToReaders_AFPV09713` (`internal/gateselect`); `TestSelectorNarrowsDeclarationEditAgainstThePlanBase_AFPV0050` (`tools/gate-affected-select`) |
| NCM-V1-007 | `TestNCMV1007RefusesUnknownVersion`, `TestNCMV1007RefusesUnknownMember`, `TestNCMV1007RefusesMalformedOrOversized` (`internal/noncodemap`); `TestNCMV1SelectorInvalidMap` (`internal/gateselect`); `TestNCMV1003ProposeDataMapCommand` (`cmd/corvint`, exit 2) |
| NCM-V1-008 | `TestNCMV1008StaleEntryDoesNotNarrow` (`internal/noncodemap`); `TestNCMV1SelectorInvalidMap`, `TestNCMV1SelectorConfirmsCitedTests` (`internal/gateselect`) |
| NCM-V1-009 | `TestNCMV1009RepeatedReadIsByteIdentical`, `TestNCMV1009DecisionIsDeterministic` (`internal/noncodemap`) |
| NCM-V1-010 | `TestNCMV1010MakeEditNarrowsWhenAttributable` (`internal/noncodemap`); `TestNCMV1010SelectorNarrowsMakeEdit`, `TestNCMV1SelectorNeverNarrowed` (no base) (`internal/gateselect`) |
| NCM-V1-011 | `TestAttributeMakeEditTargets_NCMV1011` (`internal/liveverify/affected`); `TestNCMV1010SelectorNarrowsMakeEdit` (`internal/gateselect`) |
| NCM-V1-012 | `TestAttributeMakeEditFailsClosed_NCMV1012` (`internal/liveverify/affected`); `TestNCMV1012SelectorMakeEditFailsClosed` (`internal/gateselect`) |
| NCM-V1-013 | `TestReachMakeTargets_NCMV1013`, `TestReachMakeSelectorPasses_NCMV1013` (`internal/liveverify/affected`); `TestNCMV1010SelectorNarrowsMakeEdit` (final qualified set), `TestNCMV1013SelectorRootPatterns`, `TestNCMV1013SelectorNestedMakefileSelectors`, `TestNCMV1019SelectorChdirOptions` (missing selector), `TestNCMV1013SelectorNestedModule` (`internal/gateselect`) |
| NCM-V1-014 | `TestReachMakeTargetsCI_NCMV1014`, `TestReachMakeEditInstance_NCMV1016` (touched rule) (`internal/liveverify/affected`); `TestNCMV1010SelectorNarrowsMakeEdit`, `TestNCMV1015SelectorNarrowsWordListEdit` (`internal/gateselect`) |
| NCM-V1-015 | `TestAttributeMakeEditWordList_NCMV1015`, `TestAttributeMakeEditWordListFailsClosed_NCMV1015` (`internal/liveverify/affected`); `TestNCMV1015SelectorNarrowsWordListEdit` (`internal/gateselect`) |
| NCM-V1-016 | `TestReachMakeEditInstance_NCMV1016` (`internal/liveverify/affected`); `TestNCMV1015SelectorNarrowsWordListEdit` (`internal/gateselect`) |
| NCM-V1-017 | `TestDriverEnvPrefix_NCMV1017` (`internal/liveverify/affected`) |
| NCM-V1-018 | `TestAttributeMakeEditOrder_NCMV1018` (`internal/liveverify/affected`); `TestNCMV1018SelectorOrderMove` (`internal/gateselect`) |
| NCM-V1-019 | `TestDriverUnfollowedCommand_NCMV1019`, `TestReachMakeRecipeShell_NCMV1019`, `TestReachMakeChdir_NCMV1019`, `TestDriverChdirPlanner_AFPV09051`, `TestReachMakeInvocationCommandLine_NCMV1019`, `TestDriverMakeAssignmentPath_NCMV1019`, `TestDriverSubshellChdir_NCMV1019`, `TestReachMakeInvocationEnvironment_NCMV1019` (`internal/liveverify/affected`); `TestNCMV1019SelectorUnfollowedScript`, `TestNCMV1019SelectorClosedRule`, `TestNCMV1019SelectorRecipeShell`, `TestNCMV1019SelectorChdirOptions`, `TestNCMV1019SelectorMakeCommandLine`, `TestNCMV1019SelectorSubshellChdir`, `TestNCMV1019SelectorSubshellPrecision`, `TestNCMV1019SelectorMakeEnvironment` (`internal/gateselect`) |
