# Friction lane: check tools and test hygiene (2026-10-30)

Lane `claude/lane-friction-checks-w4`, base `origin/main` c5d5f0829. Tickets V1-3234, V1-3235,
V1-3221, V1-3220, V1-3219. Requirement IDs: `AFP-V0-829`, `TTG-V0-012`, `PUB-V0-054`,
`ASD-V0-064`; V1-3220 is test hygiene under the existing `WQO-V0-049` row.

## Decisions

- V1-3234 (`AFP-V0-829`): a refused path that only the repository-root resolution reaches now ends
  with `it is read from the repository root because FILE:LINE holds LITERAL, which locates the
  root` and both rules (AFP-V0-021, AFP-V0-828). The clause is appended, so the existing refusal
  prefix that other tests and lanes match is unchanged. The first locating string literal is named;
  a token the lexicon reads outside any string literal falls back to its first textual line.
- V1-3235 (`TTG-V0-012`): the gate stays an index snapshot (TTG-V0-001/002); it does not read
  working-tree content as evidence. An unresolved name declared in an untracked or unstaged
  modified `_test.go` file is named with that file and `git add -- FILE`.
- V1-3221 (`PUB-V0-054`): measured on this host at c5d5f0829 with go1.27.3, a cold
  `go build -trimpath ./cmd/corvint` under the companion's closed environment leaves a 272 MB
  home and a 53 MB binary (about 650 MB for its two builds). The test reads free space under its
  scratch, runs at most `free / 1 GiB` components at once (up to 4), deletes each component's homes
  when it passes, and skips naming both figures below 1 GiB. Byte-identity and two cold caches per
  component are unchanged. Live run: all four components passed in 84 s with 18 GB free.
- V1-3220: `testing.T.TempDir`, `os.MkdirAll`, `os.WriteFile` and `go build -o` all apply the
  process umask, so under `umask 0002` fixture executables and their parents became 0775. The
  fixtures now set 0755 explicitly (`workFixtureRoot`, `workMkdirAll`, `workBuildExecutable`,
  `workCopyExecutable`), and new 0775 and 0777 executable cases assert the refusal. Before: 8
  `^TestWork` failures under `umask 0002`; after: pass.
- V1-3219 (`ASD-V0-064`): no lane script in the repository called Codex, so the preflight is a new
  `script/codex-review.sh` (`--preflight` for lane setup; otherwise the read-only review), with
  `docs/LANE-ORCHESTRATION.md` section 10. This host has no `codex` CLI: the script prints
  `NOT_AVAILABLE codex-not-installed`, and the lane review is `NOT_RUN`.

## Adversarial refusal pass (docs/LANE-ORCHESTRATION.md section 9)

| Rule | Category | Case |
|---|---|---|
| AFP-V0-829 | malformed input | an unparsable test file is still refused by `TestTempRootOnlyRefusesAnUnparsableFile_V1_3137` before any culprit is named; `rootLocator` skips a file it cannot parse |
| AFP-V0-829 | aliasing | a path the package directory also resolves names no culprit: second half of `TestRootResolvedRefusalNamesLocatingLiteralAndRule_V1_3234` |
| AFP-V0-829 | spread/variadic | join runs resolve against the package only and never carry a culprit (`target != path.Join(directory, value)` is false for them); `TestJoinedSegmentsOutsideScopeFail_V1_2117` still passes |
| AFP-V0-829 | quoting | the literal is printed as written (`literal.Value`), so a raw or escaped literal is named exactly; not separately tested |
| AFP-V0-829 | historical vs worktree | not applicable: the check reads the working tree only, as before |
| AFP-V0-829 | unobserved state | message-only change; a package with no locating literal keeps `fromRoot` false and names nothing |
| AFP-V0-829 | concurrency | not applicable: a read-only, single-process check |
| TTG-V0-012 | malformed input / quoting | file names come from `git ls-files -z`; a name with a newline is read by path like any other |
| TTG-V0-012 | historical vs worktree | the core of the rule: the index decides, the working tree only explains; `script/check-traceability-tests_test.sh` untracked and unstaged-edit cases fail on the base checker and pass after |
| TTG-V0-012 | unobserved state | a never-declared name gets no hint (the nested-heading case asserts it); an ignored file gets none (documented limit) |
| TTG-V0-012 | aliasing, spread, concurrency | not applicable: lexical names in one read-only pass |
| ASD-V0-064 | unobserved state | missing CLI, status exit 1, and status exit 0 saying `Not logged in` are each NOT_AVAILABLE with no `exec` call (`script/codex-review_test.sh`) |
| ASD-V0-064 | malformed input | `--preflight extra` exits 2 |
| ASD-V0-064 | quoting / spread | review arguments pass through `"$@"` unchanged; the stub records the argv |
| ASD-V0-064 | historical vs worktree, aliasing | not applicable |
| ASD-V0-064 | concurrency | a login that expires between the check and `exec` is a documented limit |
| PUB-V0-054 | unobserved state | a `Statfs` failure fails the test rather than guessing; below 1 GiB skips with both figures (`TestDoubleBuildWorkersFitFreeScratch_V1_3221` pins the arithmetic) |
| PUB-V0-054 | concurrency | another writer filling the file system after the check is a documented limit |
| PUB-V0-054 | others | not applicable: no parsing or name resolution |

## Retained limits

- Codex review `NOT_RUN`: `codex` is not installed on this host (`codex-review: NOT_AVAILABLE
  codex-not-installed`).
- The PUB-V0-054 skip branch is exercised only through its arithmetic; this host had room for 4.
- No `make gate`, per the owner's standing preference.
