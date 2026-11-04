<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selection precision 2, round 4: AFP-V0-2245 withdrawn, AFP-V0-2246 hardened

Date: 2026-11-03. Lane selprec2, round 4 (on `89cbd5108`). Tickets: V1-4173. Orchestrator-made,
owner-delegated decision: soundness over precision.

## Decisions

- **AFP-V0-2245 is withdrawn (the ID is kept and not reused).** An independent re-review of the
  round-3 head (`27d87691b..7f7458dc9`) ran `affected.Build` and `Select` over `typescript.New()`
  and found TypeScript tests that the round-2 relation selected and the round-3 certificate
  dropped. Each joins an anchored `"/repo"` to a directory the certificate cannot see:
  1. a glob;
  2. a directory listing;
  3. any environment variable other than the three refused;
  4. a directory parsed from JSON;
  5. launchers outside the manifest (shell, Makefile, CI `working-directory`, nx);
  6. a `.mocharc.yml`, which the config check lexed as script;
  7. setup or `--require` modules that call `process.chdir`;
  8. a dynamic `import()` of a template, or package `"imports"`;
  9. identifiers turned into components;
  11. case transforms;
  12. `module.parent.filename` or a stack trace.

  The reason recorded: "unsound: data- and filesystem-derived directories escape the
  certificate". These sources are open-ended, so no allowlist of the module's own text can bound
  them. The TypeScript certification code (`RunDirectories`, `AnchorWords`, `AnchorsPlaced`,
  `anchorsPlacedName`) is reverted. `typescript.go`, `path_tokens.go`, `unit.go`, `graph.go`,
  `graphcache.go` and `readers.go` are byte-identical to `27d87691b`. The counterexamples are
  kept as `TestTypeScriptAnchoredCounterexamplesStaySelected_AFPV02245` (17 cases), and the spec
  lists them as failure modes.
- **AFP-V0-2246 admits a canonicalization only in a file an allowlist accepts.** The
  re-review found that these shapes still canonicalized:
  - `cd(){ :; }`, `pwd(){ echo .; }`, `alias pwd='echo .'`;
  - `exit(){ :; }`, `function exit { :; }`, `enable -n exit`;
  - a sourced helper that is never inspected.

  A first fix refused those words. The independent review of round 4 then found a DEBUG `trap`,
  a quoted `ev''al` or `en''able`, and zsh and ksh hooks getting through, so the check is now an
  allowlist. `canonicalOverride` admits only:
  - a bash, sh or dash shebang;
  - no `BASH_ALIASES`;
  - function definitions with plain names that override nothing (the names it relies on and the
    shell hooks are refused);
  - command words, substituted or in functions, that are plain literals, or the once-assigned
    expansion of a value holding a literal `/` (a slashed name runs a file, never a builtin);
  - none of `.`/`source`, `eval`, `enable`, `disable`, `alias`, `builtin`, `command`, `mapfile`,
    `readarray`, `shopt`, `typeset`, `declare`, `functions` or `autoload`;
  - no `trap` beyond EXIT, INT, TERM, HUP and QUIT, and no `set` that turns on tracing.

  `script/batch-assemble_test.sh` still passes: its EXIT trap and its `"$script"` command
  (`script=$(...)/batch-assemble.sh`) are admitted. Environment-imported Bash functions
  (`BASH_FUNC_*`) and `BASH_ENV` remain the accepted residual AFP-V0-2204 already has for
  `mktemp`.
- **AFP-V0-2247 is unchanged.**

## Measurements

Same commands with three builds: r2 = `27d87691b`, r3 = `7f7458dc9`, r4 = this tree. The CI
selector is unchanged throughout (28 reader lines, 209 packages on PR 86).

| Run | r2 | r3 | r4 |
|---|---|---|---|
| PR 86 (`--base f89db15bd` at `64321325b`): selected, unknown, PATH_LITERAL | 166, 83, 62 | 162, 83, 58 | 165, 83, 61 |
| `lib.js` by-path probe: additions + shared | 57 + 26 | 53 + 26 | 56 + 26 |
| `lib.js` PATH_LITERAL readers (Go, shell, TypeScript) | 6, 16, 5 | 6, 15, 2 | 6, 15, 5 |

Round 4 keeps the one shell reader AFP-V0-2246 drops (`script/batch-assemble_test.sh`, which
sources nothing and overrides nothing) and restores the TypeScript readers.

## Soundness evidence

- **Counterexamples.** All 17 cases pass on this tree. Run against the round-3 TypeScript code (a
  scratch worktree at `7f7458dc9` with the test copied in), all 17 fail.
- **Shell controls.** 29 `canonical` cases in `TestTempRootPathsAreNoTokens_AFPV02204`, 23 of
  them new this round: functions, quoted names, aliases, `enable`, `eval` (plain and quoted), a
  computed command word, sourced files, DEBUG traps (plain and quoted condition), `mapfile -C`,
  `BASH_ALIASES`, `shopt`, `declare`, and the admitted EXIT trap and path command.
  `TestCanonicalizationNeedsAPlainShell_AFPV02246` covers the shebangs.
- **Shell fault injection** (each mutation applied, the test run, then reverted):
  - Shebang check off: the zsh, ksh and no-shebang cases fail.
  - `BASH_ALIASES` off: the aliases-array case fails.
  - Function-name check off: the six function cases fail.
  - Non-literal command words admitted: quoted `enable` fails.
  - Refused-word list off: nine cases fail.
  - Trap condition check off: the quoted DEBUG trap fails.
  - Path commands always refused: the path-command positive fails.
  - The `set` tracing refusal has no unique case: the DEBUG trap it enables is refused anyway.
  - A first round of three injections did not compile, and the harness hid the build failure;
    they were rerun with compiling mutations.

## Independent review

A read-only subagent reviewed the round-4 diff (`89cbd5108` to the working tree) and confirmed
its findings by running bash and overlay probes:
- 2 HIGH: a DEBUG trap; a quoted `eval` or `enable`.
- 1 MEDIUM: zsh and ksh overrides (`disable cd`, `chpwd`, `builtin -d cd`).
- 2 LOW: the AFP-V0-2246 text promised more than the code checked; three counterexample cases
  (2, 6, 12) do not resolve their directory to `app/x` and so do not literally read that path, and
  the list skips 10.

All five are fixed by the allowlist and the wording above. The review confirmed the revert is
clean (only shell files differ from `27d87691b` under `internal/liveverify/affected/`), that all
17 counterexamples fail at `7f7458dc9`, and that no code names the withdrawn fields.
