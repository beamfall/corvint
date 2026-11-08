# Lane rescue-c: Codex review fixes (2026-11-04)

One independent Codex review of `24d8c04f9f5ffaaed86759951e520cb775dc2eff..115c44f98` returned one
P1 finding and three P2 findings, with verdict CHANGES_REQUIRED. Each finding was checked against
the code before it was changed, and each fix has a regression test that fails on the reviewed head.

## Findings

- P1, `.corvint/test-read-scopes.json`: the `conformance/release-artifact-v0` declaration was
  unsound. **Fixed by keeping the floor.** This was confirmed:
  - With `CORVINT_HOST_ARCHIVE_PROOF=1` on Darwin arm64, `TestCorvintHostArchivePartialProof`
    archives the enclosing checkout's clean `HEAD` through Git and builds all of `cmd/corvint`.
  - The planner does not condition a read scope on that variable, so the declaration let a change
    confined to `cmd/corvint` exclude the package.
  - Decision (orchestrator-made, fail-closed): drop the declaration and keep the package on the
    `UNBOUNDED_READER` floor with that reason. The environment variable is not modelled.
  - The package is a main package with no importing dependents, so the floor costs only its own
    tests. `internal/companionrelease` still leaves the floor as V1-0643 intended.
  - PUB-V0-671 and the "Bounded release-test reads amendment" now say this.
  - `TestPUBV0671HostArchiveProofKeepsTheUnboundedFloor` requires a reasoned floor entry and no
    scope entry. It fails on the reviewed head.
- P2, `script/dogfood-closeout-repin.sh`: a failed repin kept its staged rewrite. **Fixed.** This
  was confirmed: `git checkout -- .` restores from the index, and `check-line-citations.sh --repin`
  stages what it rewrites.
  - The failure path now resets HEAD and the index to the starting commit and checks the tracked
    tree out of that index.
  - A restore that fails exits 1, naming the commit and the recovery command.
  - ASD-V0-568 now requires this.
  - The test adds three cases: a repinner that stages its rewrite, one that commits it, and one
    that leaves `index.lock` behind. With the reviewed script the staged case fails, with
    `M  doc.md` left in the index.
- P2, `script/changed-checks.sh`: `unbounded-readers-check` was skipped for Go test edits, Go
  source edits and read-scope declaration edits. **Fixed.** This was confirmed:
  `tools/unbounded-readers` rebuilds the affected graph with every language analyzer
  (`affected.Build(root, languages.All()...)`).
  - The check now runs for any changed path that is not Markdown, and for any added or deleted path.
    The trigger is fail-closed rather than an enumeration of the ratchet's inputs, because the check
    takes about 3.5 s on wsl-lanes.
  - ASD-V0-576 now says this.
  - The test covers six edits: a cited Go edit, an uncited Go source edit, a Go test edit, a
    `.corvint/test-read-scopes.json` edit, a Markdown-only edit (skipped) and an added Markdown
    file. With the reviewed script, the cited Go edit case fails.
- P2, `tools/py-signal-lint`: a registration split across lines escaped the lint. **Fixed.** This
  was confirmed.
  - Each `signal.signal(` call is now read as one logical call through its matching closing
    parenthesis. Parentheses in strings and stripped comments do not count, and an unclosed call
    runs to the end of the file.
  - A trailing comma before the closing parenthesis is accepted.
  - ASD-V0-574 now says this.
  - `TestASDV0574MultilineRegistration` covers a function and a lambda registered across lines, a
    trailing comma, a commented parenthesis, and text after the call that must not be read as part
    of it. With the reviewed lint it fails (`rc=0`).

## Tests (wsl-lanes, go1.27.3, Go through `script/heavy.sh`)

- `corvint affected --base 24d8c04f9f5ffaaed86759951e520cb775dc2eff` returned `scope=UNKNOWN`
  (selected 195, excluded 445, unknown 83: 79 `LANGUAGE_FRONTIER`, 4 `UNOWNED_DIRTY_PATH`). Over
  the fix alone (`--base 115c44f98`) it returned `UNKNOWN` (selected 97, unknown 2). Per the
  owner's standing preference, only the focused tests below ran; batch CI covers the rest.
- Each new test failed on the reviewed head and passed on the fix. Focused Go tests passed:
  - `conformance/release-artifact-v0` (whole package);
  - `TestASDV0574` in `tools/py-signal-lint`;
  - `tools/unbounded-readers` and `tools/test-read-scopes-check`;
  - `TestV12690DeclaredRuntimeTupleDrift` in `internal/companionrelease`;
  - `TestASDV0576AffectedListsChangedChecks` in `cmd/corvint`.
- `script/changed-checks_test.sh` and `script/dogfood-closeout-repin_test.sh`: ok.
- `make unbounded-readers-check`, `make test-read-scopes-check`, `make py-signal-lint` (102 files)
  and `make changed-checks BASE=24d8c04f9f5ffaaed86759951e520cb775dc2eff`: exit 0.
- `gofmt -l` reported nothing. `make spec-requirements` left `REQUIREMENTS.tsv` unchanged.
  `make line-citations-check` (staged tree) and `make windows-vet` exited 0.
- One full `make doc-gates` (360 s) passed every gate except `long-lines-check`. That gate failed
  only on `.corvint/change.cem.json`, the bound CEM the `115c44f98` bind commit carries. It is not
  part of this fix. The seal moves the CEM under `.corvint/changes/`, which
  `.corvint/agent-grep-exclude` already excludes.

## Follow-up

- `AGENTS.md`'s Verify advice for `make changed-checks` still names "a test, a script, the
  `Makefile` or a cited file". That advice now under-describes the `unbounded-readers-check`
  trigger for an uncited Go source edit. Project instructions were left unchanged in this lane.
- `long-lines-check` fails on any bound but unsealed head, because `.corvint/change.cem.json` is
  neither excluded nor allowed. A fix round on top of a bind commit therefore cannot pass
  `make doc-gates` before it rebinds and seals.
