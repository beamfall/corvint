# Spec-cited code reservation in the task-context packet (V1-1269, TCP-V0-990..994)

Lane ctxpkt-w1, base `origin/main` `f586f8a69c52180b1cbd6efceb1566d3f5f2cfd3`. The ticket names
`internal/context`; the packet compiler lives in `internal/contextindex`.

## Reproduction

The original task text and session were not retained, so the replay uses an equivalent 190-character
spec-routed implementation task on the exact frozen tree (`git archive fab866738`, re-committed; its
`HEAD^{tree}` is `fab866738af74c7e2169adadb9e1defbe164b705`):

`Implement slice 1 of the Session Test Runner (corvint test) in docs/specs/session-test-runner-v0.md:
reuse the affected plan, run the selected Go tests once, and type results per STR-V0-006.`

`corvint context --task TEXT --limit 25` at the base carried `AGENTS.md` and the spec, then lexical
and documentation rows (`cmd/corvint/help.go`, `.github/workflows/ci.yml`, sibling specs). It carried
none of `cmd/corvint/affected.go`, `tools/gate-affected-select` or the `Makefile`, and named none of
them anywhere in `coverage`. The spec cites all three in backquotes (`corvint affected`,
`tools/gate-affected-select`, `make gate-affected`), but they share almost no terms with the task.

## Decisions

- **A reserved relation, not a ranking boost.** `spec-cited` sits beside the other reserved
  relations, after `instruction-routed`. It reads only the bodies of clean `spec-mentioned` rows,
  which are project-owned routing for this task (invariant 3). Lexical scores are not changed, so
  packets without a routed spec cannot move.
- **Cap 3, score 825.** The cap matches `spec-mentioned` (3), and the score sits below
  `instruction-routed` (850). A spec like STR cites about 90 tracked paths, so an uncapped
  reservation would push every slot row out of the packet.
- **Order by citation count, then first citation.** The paths a spec keeps returning to are its
  working surface. On the current tree this puts `Makefile` (41 citations) and `cmd/corvint/test.go`
  (37) first.
- **Resolution rules.** A span's first word resolves as an indexed file, with `./`, `:LINE` and a
  trailing `/` dropped. Failing that it resolves as a directory through `main.go` or `<base>.go`.
  `make X` resolves to the root `Makefile`, and `NAME VERB` to `cmd/NAME/VERB.go`. Documentation
  paths are excluded because the existing documentation relations already rank them. The rules use
  only the index, never the filesystem.
- **Withheld candidates are named, not dropped.** Every candidate past the cap that no carried row
  names goes into `coverage.omitted_spec_cited`, which holds `{path, reason}` samples bounded at 50
  (the largest `--limit`). The member mirrors `omitted_pairs` (TCP-V0-161). This satisfies the
  ticket's "or names each as withheld with its reason" for paths beyond the cap. A path another
  relation carries is filtered out, so `withheld` and the member's `count` agree.
- **Byte stability.** The relation is listed in `unexamined` only when it has a candidate, which
  mirrors `graph`. Packets without a spec citation therefore keep their bytes, including the default
  wire, core-freeze, recipe goldens and every frozen bench packet.
- **No trust-table change.** The rows reuse authority `document-reference`, which maps to
  `project-authority`, the label `impact` already gives to explicit document references.
- **Derived Tasks scopes include the rows.** `internal/tasks/scopes` skips the governing,
  spec-mentioned and instruction-routed rows because they are instructions. Spec-cited rows are the
  code the change edits, so they stay in a derived scope.
- **Candidate class.** In TCP-V0-163's union the rows belong to `governing-instruction`, as a
  relation-gated reservation that gives no authority credit.
- **Analyzer pin.** `TestAnalyzerSchemaInputs` hashes every production source in the package, so
  `script/analyzer-schema.sh bump` repinned only the input digest. The output digest is unchanged,
  so the schema stays `corvint-analyzer/150`.
- **Line-citation repin.** The new compiler field and relation cases shift the lines of
  `internal/contextindex/taskcontext.go` that other documents cite. `script/check-line-citations.sh
  --repin` rewrote 14 citations whose content is unchanged, including one in decision 0377. Only
  line numbers changed, and the lane rule against editing decision files yields to that gate.

## Evidence

- Acceptance criterion 1, exact frozen replay with limit 25 and the branch binary:
  - `spec-cited` rows carry `tools/corvint-pr-tests/main.go` (7 citations), `Makefile` (4) and
    `cmd/corvint/affected.go` (3).
  - `tools/gate-affected-select/main.go` appears in `omitted_spec_cited` with the reason
    ``docs/specs/session-test-runner-v0.md cites `tools/gate-affected-select` (2 citations);
    omitted: spec-cited cap 3``.
  - `withheld` for `spec-cited` is 4, which equals the member's `count`.
- Current tree, same task: `Makefile` and `cmd/corvint/test.go` are carried. `cmd/corvint/affected.go`
  and `tools/gate-affected-select/main.go` are named in `omitted_spec_cited` with their reasons
  (65 candidates, 50 samples).
- Acceptance criterion 2: `tools/retrieval-bench --arms context` was run over the frozen v2 releases,
  base binary against branch binary, with identical `samples_sha256` and 0 errors in both arms.

  | subset | n | recall@20 | recall@10 | recall@5 | identical rankings |
  |---|---|---|---|---|---|
  | v2_code2test | 106 | 0.521069 → 0.521069 | 0.426415 → 0.426415 | 0.327044 → 0.327044 | 106/106 |
  | v2_comment2context | 80 | 0.504167 → 0.504167 | 0.370833 → 0.370833 | 0.264583 → 0.264583 | 80/80 |
  | v2_edit2ripple | 58 | 0.656609 → 0.656609 | 0.563218 → 0.563218 | 0.410920 → 0.410920 | 58/58 |
  | v2_trace2code | 101 | 0.764026 → 0.764026 | 0.516502 → 0.516502 | 0.435644 → 0.435644 | 101/101 |

  No chunk file in any v2 corpus carries a `docs/specs/*.md` path, so the relation cannot fire
  there. The run bounds harm but cannot measure the gain. A held-out set of spec-routed tasks is the
  open measurement (TCP-V0-994).
- Tests: `internal/contextindex` passes as a whole package (changed package). The focused
  `cmd/corvint` context, core-freeze, answerability and slot-weight tests pass, and the goldens are
  unchanged.
