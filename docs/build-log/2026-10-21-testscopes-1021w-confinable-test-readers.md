# Test read scopes: confinable absolute readers and empty declared scopes

Date: 2026-10-21. Lane: claude/testscopes-1021w (base f29ddc0db). Tickets: V1-2217, V1-0883
delivered in part, V1-2069 progressed; V1-2293 and V1-1961 not delivered. Requirement `AFP-V0-361`
in `docs/specs/affected-plan-v0.md` is accepted under the owner's standing auto-accept (2026-10-11).

## Problem

- V1-2217 / V1-2069: `internal/tasks/transaction` had no AFP-V0-023 read scope, so a ticket-only
  diff (`.taskman/tickets/*.json`) selected it as a path-literal reader of dirty paths.
- V1-0883: Landlock confinement grants nothing on the repository root's ancestors, so any test
  reader that opens `/` for reading cannot be confined. `internal/flowdocs`, `internal/flowcoverage`
  (through flowdocs), `internal/testrunner/sql` (through `os.OpenRoot("/")`) and the
  `internal/stepverify/safeopen` walk each opened `/` or an ancestor with `O_RDONLY`.

## Decisions

- The transaction tests use in-memory fixtures and temporary state directories and read no
  repository path, so its scope is empty. The same holds for flowdocs, flowcoverage and
  testrunner/sql once they stop opening `/`; the three leave the AFP-V0-025 ratchet.
- flowdocs and testvaliditydoc pin the first component by absolute path (`Lstat`, open, `SameFile`)
  and keep their original failure messages; `/` is opened only when it is the target.
  `testvaliditydoc.ReadAbsoluteFileBounded` is the shared entry; testrunner/sql uses it.
- stepverify/safeopen copies the Tasks safeopen pattern (decision 0397 keeps them separate): Linux
  `O_PATH|O_DIRECTORY` for `/` and intermediate parts, `O_RDONLY|O_DIRECTORY` on Darwin.

## Evidence

- `corvint affected --path .taskman/tickets/V1-0019.json`: 47 selected (41 UNBOUNDED_READER,
  6 PATH_LITERAL_READER) before, 46 (41, 5) after; transaction drops out.
- `TestAFPV0361_OpenDirectoryNeverReadsFilesystemRoot` failed on the old flowdocs code
  (`nested directory opened the filesystem root: ["/"]`) and passes now;
  `TestAFPV0361_ReadAbsoluteFileBoundedSkipsFilesystemRoot` passes; the four packages pass on
  darwin; `go vet` passes for darwin, linux and windows.

## NOT_RUN

- Linux: `TestAFPV0361_TraversalOnlyAncestors` and the Landlock-confined runs of the four newly
  declared packages are CI evidence (macOS host). V1-0883 items 2 (attribute the `/` opens of
  corpus-mcp, lspstdio, docsbridge) and 3 (confined replay, V1-0868 five-PR counts) need Linux.
- V1-2069 full acceptance stays blocked by 41 unbounded readers and 5 path-literal readers
  (cemchain, contextindex, liveverify/affected, opencodequalification, tasks/cli).
- V1-2293: gitauth tests mutate PATH and GIT_* through process env read by `scrubbedEnv`,
  `requireSupportedObjects`, the stable boundary and the gitrun binary lookup; argument-scoped env
  needs seams in all four. The 9 serial gitnotes tests mutate package hooks (`verdictNow`,
  `beforePublish`, `anchorRead`, `verdictsRead`) read by every anchor path. Baseline: gitnotes
  70.4 s at load average 213.
- V1-1961: part 1 already delivered (05945c5e2, OCM-V0-025); `preflight --plan --witnesses` has no
  specified witness input; the capability listing bound is not started.

## Rollback

Revert the two code commits: restore `os.OpenRoot("/")` and `O_RDONLY`, remove the four scopes and
restore the three ratchet units.
