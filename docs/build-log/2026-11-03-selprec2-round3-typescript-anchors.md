<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selection precision 2, round 3: TypeScript runner directories, canonical temp roots, webdialect

Date: 2026-11-03. Lane selprec2, round 3 (on `27d87691b`). Tickets: V1-4173 remainder, V1-3282.
Requirements: AFP-V0-2245 (amends AFP-V0-072 (3)), AFP-V0-2246 (amends AFP-V0-2204) and
AFP-V0-2247 (amends LGP-V0-027) in `docs/specs/affected-plan-selection-v0.md`. All three carry
out orchestrator-made, owner-delegated decisions (proposals 1 to 3 of round 2).

Superseded in part: round 4 withdrew AFP-V0-2245 and reverted its code (build-log entry
`2026-11-03-selprec2-round4-withdraw-typescript-anchors`); the TypeScript sections below are
history.

## Decisions

- **TypeScript runner directories (AFP-V0-2245).** Round 2 withdrew a TypeScript narrowing
  because a runner's working directory is unknown. The plugin now records where a runner it knows
  may start a module:
  - A runner starts in the repository root or a package root, both of which are the root or an
    ancestor of the module.
  - A config whose membership is not plain local literals (`localMembership`) may own the module
    from elsewhere, so its directory and that directory's ancestors are recorded as run
    directories. The 11 fixture configs in this tree (`testDir: './tests'`,
    `include: ["test/vitest.test.js"]`) are local.

  With those bases, `process.cwd()`, `path.resolve()` and relative joins start in a recorded
  directory. Everything else that can reach a directory is refused, by allowlist:
  - `process` may appear only as `process.cwd()`, a literal-keyed `process.env.NAME` other than
    `PWD`/`OLDPWD`/`INIT_CWD`, or a member that reaches no directory;
  - `require` and `import` only on literals, or as a module-relative `resolve`;
  - any repository manifest script with a directory change, a `..`, a root, prefix or config
    option, or an embedded runner config naming roots refuses every certificate.

  Because a TypeScript unit is one file, a module certifies only if its whole local import
  closure certifies and every relative specifier resolves to a module unit. Its words and run
  directories then cover the closure, so a root built from an imported constant
  (`` `${APP}/repo` ``) is still placed. The accepted residual is the Go plugin's: a runner started
  in a directory nothing in the repository records (a CI job's `working-directory`, a Makefile
  `cd`), or a root handed in at run time.
- **Canonical temp roots (AFP-V0-2246).** `tmp=$(cd "$tmp" && pwd)` is no second assignment, but
  only after `tmp=$(mktemp -d) || exit`. Under dash (`/bin/sh` here), `cd ""` succeeds and `pwd`
  prints the working directory, so an unguarded mktemp failure would turn the root into a
  repository path. Checked: `sh -c 'tmp=""; tmp=$(cd "$tmp" && pwd); echo "[$tmp]"'` prints the
  working directory, and bash 5.3 refuses `cd ""`.
- **The TypeScript impact profile moves to `internal/webdialect` (AFP-V0-2247).** The probes, the
  load pattern and the static-load rule move. The patterns are compiled inside `Impact`, so the
  package has no call initializer (ADN-V0-004). The web mask stays in `internal/contextindex`,
  whose readers share it (LGP-V0-027), and core registers `webdialect.Impact(flowWebCodeLines)`.
  A move of the whole web lexer would change nothing measurable: it would make contextindex
  import the lexer's package, and a lexer edit would still reach every contextindex importer.
- **V1-3282 still does not narrow, with the reason retained.** After the move, a `flowWordBefore`
  body edit no longer reaches a var initializer. It still falls back on the registering `init`,
  and the fallback is the real reach: `Build` reaches `nonGoImpactUnknowns`, whose
  `dialect.LookupImpact` runs the registered `CodeLines`, which runs `flowWordBefore`. A sound
  registration rule would need two things, and would select at most 147 of 151 units (round 2's
  measurement): store-only proofs of `webdialect.Impact` and `dialect.RegisterImpact`, and a check
  that no other importer calls the registry's lookups.

## Measurements

Same build pair: `r2` built at `27d87691b`, `r3` built from this tree. The probes ran on the
PR 86 tree (`64321325b`) and on a scratch commit of this tree.

| Run | r2 (selected, unknown) | r3 | Change |
|---|---|---|---|
| PR 86 (`--base f89db15bd`) | 166, 83 | 162, 83 | PATH_LITERAL 62 to 58; CI selector unchanged (28 reader lines, 209 packages) |
| `lib.js` by-path probe | 57 additions + 26 shared | 53 + 26 | PATH_LITERAL readers: TypeScript 5 to 2, shell 16 to 15 (`batch-assemble_test.sh`), Go 6 to 6 |
| contextindex body edit (`flowWordBefore`) | 151, 4 | 151, 4 | fallback reason now only `init in dialect_hooks.go` |
| `heavy.sh` edit (declaration committed) | 52, 1 | 52, 1 | none |
| gate-prerequisite edit | 50, 1 | 50, 1 | none |

The two TypeScript readers that stay are `integrations/opencode/cockpit.test.mjs` and
`task-metrics.test.mjs`. Their import closure holds `integrations/opencode/src/runtime.js`, which
passes the whole `process.env` on (`options.environment ?? process.env`), so code holding it can
read `PWD`. Before the review fixes, all five dropped. That earlier result relied on per-file
vocabulary and name-based refusals, which the review showed unsound.

## Independent review

A subagent reviewed the uncommitted round-3 diff read-only and confirmed each finding with
throwaway overlay tests. It found 3 HIGH, 4 MEDIUM and 2 LOW; all are fixed, each with a negative
control:
1. **HIGH: helper-module constants.** Fixed by the import-closure merge and the unresolved
   relative-specifier refusal.
2. **HIGH: manifest-embedded runner configs** (Jest `roots`). Fixed by `embeddedMove`.
3. **HIGH: scripts that run tests elsewhere without `cd`** (`--rootDir`, `-c`, `node --test ../`).
   Fixed by the stricter `directoryMove`.
4. **MEDIUM: climbing that `climbs` missed** (`'/..'`, `'../'`, `path.parse().dir`). Fixed:
   run-directory ancestors are now always bases, and `climbs` is gone.
5. **MEDIUM: refusals bypassed** (renamed destructuring, `process['chdir']`, an aliased `require`,
   a computed env key). Fixed by the allowlist.
6. **MEDIUM: `localMembership` holes** (concatenation, escapes, bare-package imports, `rootDir`).
   Fixed.
7. **MEDIUM: `|| exit 1 | cat`.** Fixed: the exit must end its list.
8. **LOW: `alias exit=:` under dash.** Fixed: the canonicalization is refused.
9. **LOW: concatenated components.** Kept as the documented residual.

It found nothing in the webdialect move: the profile is byte-identical and registered in the same
`init`.

## Soundness evidence

- Negative controls:
  - the four the orchestrator named: relative join, renamed destructured `cwd`, `path.resolve()`,
    `process.env.PWD`;
  - `process.cwd()`, `INIT_CWD`, manifest `cd` and `--prefix`, `chdir`, `argv`, `require.resolve`,
    run-directory ancestors, and local and imported config membership;
  - the review's escapes: imported constant and JSON, embedded Jest roots, script `rootDir`, `-c`
    and `..`, renamed `argv`, `process['chdir']`, aliased `require`, computed env key, and the
    four computed config shapes;
  - shell: `canonical without exit`, `canonical other name`, `canonical first`,
    `canonical exit piped`, `canonical exit aliased`.
- Fault injection (each mutation applied, the test run, then reverted). Every one failed the
  intended case:
  - Run directories dropped from the bases: the cwd, rename, resolve, relative, key and fake
    controls fail.
  - Words dropped from the vocabulary: the relative-join control fails.
  - The `PWD`/`INIT_CWD` refusal emptied: the pwd and initcwd controls fail.
  - The manifest move check dropped: manifest `cd` and `--prefix` fail.
  - The resolver refusal disabled: `require.resolve` fails.
  - (Before the review) `climbs` always false: the dirname and dots controls failed. `argv`
    refused even as a key: the flat control failed. The module-relative exception removed: the
    local control failed.
  - (After the review fixes) Closure merge off: the imported constant fails. Unplaced check off:
    the imported JSON fails.
  - Embedded check off: Jest roots fails.
  - Script `..` off: `node --test ../tests` fails. `rootDir`/`--config` off: those fail.
  - `process` allowlist off: renamed `cwd` and renamed `argv` fail.
  - `require` allowlist off: the aliased `require` and `require.resolve` refusals fail.
  - `localMembership` concatenation, escape decoding or import allowlist off: each fails its case.
  - Shell: list-ending exit check off fails `canonical exit piped`; alias check off fails
    `canonical exit aliased`.
  - `localMembership` always false: fixture configs become run directories.
  - `localMembership` always true: the web config is lost and the cwd controls fail.
  - The canonical `|| exit` guard dropped: `canonical without exit` fails.
  - The same-name check dropped: `canonical other name` fails.
