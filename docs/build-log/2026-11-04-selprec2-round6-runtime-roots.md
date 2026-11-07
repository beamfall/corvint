<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selection precision 2, round 6: runtime roots, closing tags and compound lists

Date: 2026-11-04. Lane selprec2, on `0c8cb2884`. Tickets: V1-4173, V1-4437.
Requirements: AFP-V0-2203, AFP-V0-2240, AFP-V0-2242 and AFP-V0-2243 amended in place; no new IDs.
Orchestrator-made, owner-delegated decisions, taken in-task: soundness over precision.

## Independent findings

The independent review of `0c8cb2884` returned two P1 findings and one P2 finding:

1. P1, shell: a temp-root assignment whose list enters a compound command and ends in `&`
   (`tmp=$(mktemp -d) && { :; } &`) runs in the background. The list scan read the lexer's
   empty trail after `}` as the end of the list, so the name became a temp root and a real
   reader of `$tmp/...` paths lost its tokens.
2. P1, rule (c) and the planner: an anchored run that is not resolved from a base was placed
   by the package's own literals. A root taken at run time (`os.Getenv("FIXTURES") +
   "/repo/lib.js"`) is not bounded by those literals, so a reader of `app/x/repo/lib.js` was
   dropped.
3. P2, CI selector lexicon: the planner reads `"</repo/lib.js>"` as the unanchored
   `repo/lib.js` (AFP-V0-505, AFP-V0-840), whose first component may end a longer name. The
   CI selector kept only the anchored `/repo/lib.js`, so AFP-V0-2242's superset claim failed
   for a path such as `app/xrepo/lib.js`.

## Decisions

- Shell (AFP-V0-2243): a list the assignment leads is no temp root once a later member is a
  compound command, a function body, or a command with a reserved word, or once a trail is
  `(` or `)`. The scan cannot see where such a list ends. Commands inside a command or process
  substitution are skipped, because they end nothing. The lexer now records the operator after a
  compound closer (`}`, `)`, `esac`) as the next command's lead. As a result,
  `{ :; } || tmp=...` counts as an assignment after `||`. Before, the lead was stale.
- Runtime roots (AFP-V0-2203, AFP-V0-2240): an anchored run of two or more components keeps
  the component-run relation at any depth, as an unanchored one always did. Only a
  single-component anchored run is still placed. That case matches the accepted lone-token
  limitation of V1-0290 and AFP-V0-072, which now names it in the failure modes. The planner
  and the CI selector change together, so AFP-V0-2242's equality still holds. The other option
  was to classify such joins as unbounded readers. It was rejected because it would select every
  package that joins any anchored literal to a variable.
- Closing tags (AFP-V0-2242): the CI selector's `pathTokens` keeps both interpretations of a
  token whose `/` directly follows `<` or a case-insensitive `&lt;`: the anchored token and the
  token without the `/`.

## Acceptance evidence

- `TestBackgroundedCompoundListsKeepTokens_AFPV02243` (`internal/liveverify/affected/shell`)
  has 22 cases. Sixteen keep the token: brace group, subshell, `if`, `while`, `for`, `case`,
  `time`, a substitution in the list, a group after `||` or `&&` (also across a
  backslash-newline), and `esac &&`. Six plain-line and simple-list controls still make temp
  roots.
- `TestRuntimeRootedRunsKeepComponentRuns_AFPV02203` (planner) and
  `TestRuleCRuntimeRootedRuns_AFPV02240` (CI selector verdict) cover the review case. The reader
  of `os.Getenv("FIXTURES") + "/repo/lib.js"` is selected for `app/x/repo/lib.js` beside `app/x`,
  the package that encloses the path. A package holding the lone `"/lib.js"` is still placed and
  not selected.
- The amended expectations follow the widened relation:
  - `TestAnchoredTokensResolveFromModuleBases_AFPV02203`: `placed` now reads
    `internal/x/testdata/repo/lib.js` through `"/testdata/repo"`.
  - `TestRuleCAnchoredTokens_AFPV02240`: the same `placed` change, plus a new
    `app/xrepo/page.html` case.
  - `TestChangeEvidenceReadersAreNarrowed_V1_0230`: `anchored` reads
    `docs/.corvint/change.cem.json` through `root + "/.corvint/change.cem.json"`.
- `TestRuleCMatchesThePlanner_AFPV02242` now has packages holding `"</repo/page.html>"` and
  `"&lt;/repo/page.html&gt;"`, and probes `app/xrepo/page.html`. `TestFastTierLiteralLexiconDiffers_AFPV0765`
  pins `</script>` to `/script` and `script`.
- Fault injection: the new and amended tests were run against the round-5 sources with `git stash`
  restoring `naming.go`, both `readers.go` files, `shell.go` and `lex.go`. With the round-5
  shell sources, 16 subtests of the shell test fail. With the round-5 naming and lexicon sources:
  - `TestRuleCRuntimeRootedRuns_AFPV02240` fails, with the verdict missing `reader`.
  - `TestRuleCMatchesThePlanner_AFPV02242` fails: the planner selects `esctag tag` and rule (c)
    selects nothing.
  - The closing-tag lexicon cases fail.
  - `TestRuntimeRootedRunsKeepComponentRuns_AFPV02203` fails.
  - The three amended expectations fail.
- These full packages pass through `script/heavy.sh`, one package per command:
  - `internal/liveverify/affected/shell`
  - `internal/liveverify/affected/golang`
  - `internal/liveverify/affected`
  - `internal/gateselect`
  - `tools/gate-affected-select`
  - `tools/gate-ledger`
  - `internal/sessiontest`
- Orientation only: `corvint affected --base cb491adb15dd88da032be365eee7f1f0ae06a958` over the
  whole lane gives scope UNKNOWN with 205 selected units.

## Limits

The AFP-V0-2203 by-path measurement on the PR 86 tree was not repeated. Every token it names is a
single-component run that round 6 still places. The selection widening this round causes on the
repository is NOT_OBSERVED. `make gate` and broad Go tests are NOT_RUN under the scoped
instructions. The CI selector reaches CI only with the next `CORVINT_PR_TOOL_SOURCE` re-pin.
Rollback: revert this round's code commit. Round 6 only widens selection.
