# Controlled edit oracle

Private measurement tool for V1-5181, `affected-edit-oracle/0`. It reads local source repositories,
clones each pinned case, applies/commits one edit, and compares the installed planner's selections
with independently authored required/permitted units. It does not change selection policy.

```sh
HEAVY_REMOTE=0 HEAVY_LOCAL_REASON='local controlled experiment' script/heavy.sh env GOTOOLCHAIN=local go build -o /private/tmp/affected-edit-oracle ./tools/affected-edit-oracle
mkdir -p /private/tmp/oracle-clones
/private/tmp/affected-edit-oracle --root . --clones /private/tmp/oracle-clones --out /private/tmp/oracle.json
/private/tmp/affected-edit-oracle --root . --clones /private/tmp/oracle-clones --mutate --out /private/tmp/oracle-mutations.json
```

`--corvint` defaults to the installed `corvint` on PATH; `--workspace` locates the source Beamfall
parent; `--corpus` selects a versioned corpus. `--help` gives all flags. Source dirty edits are never
copied. Clones are deleted after each case, including errors. An existing case destination is refused.
The committed path must match the edit, so ignored additions fail.
The JSON report is deterministic; progress goes to stdout only when `--out` is given, with one line
per case and every OVER's unit/witness/dirtyPath. Exit 1 means a measured quality or qualification
failure; exit 2 means invocation/input failure. `--allow-over` waives only OVER, never recall misses,
failed commands, surviving mutations or SAFETY_FAIL.

Mutation commands must pass on the pinned base, then fail with an observed assertion on the mutant.
Go JSON failures are attributed to their emitted package IDs and named tests, not just process exit.
Node assertion scripts name one test-file unit and must fail with `ERR_ASSERTION`. Build failures
and missing dependencies are errors. Go mutations are single-package, 90-second tests; all commands
have a two-minute wall cap. Corvint Go commands run through `script/heavy.sh`, with remote execution
disabled. No dependencies are installed and no workflow/Makefile recipes are executed.

The corpus is trusted executable input, not a sandbox. Read each `why`: required sets are dependency
lower bounds and excesses are candidates for investigation, especially tests that inspect source text.
Package/file recall does not prove selected test-filter completeness or mutation adequacy. Raw
unknown counts and planner summaries remain in results. An empty unknown list is not a safety proof.
See `docs/specs/affected-edit-oracle-v0.md` for the contract, bounds and rollback.

Corpus applicability: the web repository has no Go module, so its comment/body/API/new-test
cases use TypeScript. The SDK has no Makefile. Cloud and SDK have no TypeScript/Swift/Kotlin
production leaf; Corvint's Swift/Kotlin examples are test fixtures, so its native non-Go leaf
probe uses the VS Code TypeScript implementation. Beamfall is read only at its pinned HEAD;
uncommitted release-workflow edits are excluded. Go test-only importer edges terminate at
the consumer test package and do not propagate through that package's production importers.
