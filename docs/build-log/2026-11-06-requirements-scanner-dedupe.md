# Requirements layout scanner dedupe (V1-4754, V1-4747 slice 3b)

The V1-4747 scope review found that the dogfood coverage reader
(`internal/dogfoodflow/ocm_coverage.go` `parseRequirements`, `fenceRun`, `requirementLine`) re-implements
the OCM-V0-001 Requirements layout that the OCM verifier reads (`internal/lrfrepo/ocm.go`
`scanRequirements`, `fencedLines`, `markdownFenceRun`, `headingStarts`, `sectionEnd`). This entry
records sharing that layout. The policies built on it stay separate.

## Scope

- New `internal/lrfrepo/requirement_layout.go` holds the shared mechanics.
  - `ScanRequirementLayout(count, text)` returns a `RequirementLayout`: the fenced lines, every
    unfenced `## Requirements` heading and, only when there is exactly one, the section's end line
    and its requirement lines with their IDs. Duplicates are kept, and nothing is refused.
  - `MarkdownFenced` is the former `fencedLines` body. `markdownFenceRun` moves unchanged.
  - `RequirementLineID` is the former inline requirement-line match of `scanRequirements`.
  - Everything works on line numbers. The caller passes each line's text without its ending, so
    each consumer keeps its own byte offsets and line-ending rule.
- `scanRequirements` keeps its strict policy and refusal order. The order is `invalid-intent`,
  `invalid-requirements-section`, `duplicate-requirement`, `missing-requirements` and
  `too-many-obligations`, and the per-map bound is unchanged. `fencedLines`, `headingStarts` and
  `sectionEnd` stay as offset wrappers for the ADR, OpenSpec and SpecKit forms.
- `parseRequirements` keeps its tolerant policy. Two headings read as no section and no outside
  lines, repeated IDs and invalid UTF-8 are kept, and the OCM-V0-880 outside items are still built
  here. The local fence scanner and the requirement-line regular expression are removed.
- The one difference between the copies stays explicit in each caller's line-text function.
  Coverage strips every trailing carriage return (`strings.TrimRight(line, "\r\n")`), while the
  verifier strips one (`lineWithoutEnding`). A `## Requirements\r\r\n` line is therefore a section
  for coverage and `invalid-requirements-section` for the verifier, before and after this change.
- `docs/specs/ocm-v0-dogfood.md` rows OCM-V0-001, OCM-V0-019 and OCM-V0-952 name the shared
  layout and its tests. The verifier citations that the move shifted are re-pinned, including
  three in that spec's refusal table that already pointed at the wrong line. Their rows are
  deleted from `script/line-citation-unpinned-inventory.txt`.

Non-goals:
- The CEM and OCM domains are not merged. Coverage stays advisory, and the verifier stays the only
  refusal authority.
- `internal/console/chain.go` `requirementLine` and `internal/specindex/requirements.go`
  `fenceRun` were not part of the ticket and were not compared.
- The refusal-table citations in `ocm-v0-dogfood.md` that were already stale before this change
  (`ocm.go:716`, `736`, `770`, `775` and others) are left as they were.

## Evidence

Base `8abf8133f` (slice 3a), Go 1.27.3.

- Comparison. The two fence scanners differ only in the line-ending trim described above:
  - Indent counting, the three-run minimum, the backtick-info rule and the closing rule
    (same character, at least as long, then only spaces or tabs) agree.
  - The section end agrees: the next unfenced `##` or `## ` line.
  - The requirement-line regex and the verifier's marker scan accept the same lines with the same
    IDs, because the ID grammar holds no backtick, colon or dot.
- Differential. Temporary tests, since removed, compared the pre-change copies with the new code
  over 300,000 random documents each. The documents were built from heading, fence, requirement,
  UTF-8, invalid-byte and plain fragments, with `\n`, `\r\n`, `\r\r\n`, `\r` and no line ending.
  - Coverage: `reflect.DeepEqual` of the whole `requirementSection`.
  - Verifier: `scanRequirements` intent, IDs, scope and error at limits 1, 2 and unbounded, plus
    `fencedLines`, `headingStarts` and `sectionEnd` at every start offset from -1 to the length.
  - There were no divergences.
- Maintained fixtures:
  - `internal/dogfoodflow/requirement_layout_test.go:TestRequirementLayoutMatchesForBothConsumers`
    pins, for 17 fixtures, the coverage spans and the verifier verdict (IDs and section span, or
    the refusal code). The expected values were observed on the pre-change code and pass
    unchanged after it. The fixtures cover LF, CRLF, multi-byte UTF-8, fenced headings, duplicate
    headings and items, fence edge cases, multiple carriage returns, invalid UTF-8, an empty blob
    and 257 items.
  - `internal/lrfrepo/requirement_layout_test.go` pins the shared layout and line grammar directly.
- Dependencies: `internal/dogfoodflow` already imported `internal/lrfrepo`, and `lrfrepo` gains no
  import, so neither package's dependency closure changes.

## Rollback

Revert the commit. The coverage reader gets its own scanner back and the verifier its inline
scan. There is no persistent state, wire or refusal-code change to unwind.
