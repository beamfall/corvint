# Design: canonical-test-id

## Decisions

1. **Form.** `<runner>:<file>::<segment>[::<segment>...]`. `<runner>` is one of `go`, `vitest`,
   `jest`, `playwright`, `pytest`, `rspec`, `minitest`. `<file>` is a clean repository-relative
   slash path. For Go, `<file>` is the package directory, because Go names and runs tests per
   package (the receipt and the line index already key Go by package, see proposal.md). A file-level
   ID is `<runner>:<file>` with no segments.
   Source: owner decision R11 (2026-11-09); review recommendation testing T-B4.

2. **Escape rule.** In `<file>` and in each segment, `%` is written `%25`, `:` is written `%3A`, and
   every control character and invalid UTF-8 byte is written as `%XX`. Nothing else is escaped. The
   separators `:` and `::` are therefore unambiguous, and an ID round-trips through parse and format
   byte for byte.

3. **Name path per runner.**

   | Runner | `<file>` | Segments, in order | Source of the names |
   | --- | --- | --- | --- |
   | Go | package directory | top-level test function, then each subtest name as `go test -json` reports it, split on `/` | `go test -json` `Package` and `Test` |
   | Vitest | test file | each enclosing `describe` title, then the test title | JSON reporter `ancestorTitles`, `title` |
   | Jest | test file | each enclosing `describe` title, then the test title | `--json` report `ancestorTitles`, `title` |
   | Playwright | spec file | project name (empty when the config defines no projects), each `describe` title, then the test title | JSON reporter title path |
   | pytest | file part of the node ID | each `::` part of the node ID after the file, split outside `[...]`, parameter part kept verbatim | node ID |
   | RSpec | example file (the rerun file path for shared examples) | each example group description, then the example description | `--format json` and the Corvint hook |
   | Minitest | source file of the test method | class name, then method name | Corvint's Minitest hook (`Class#method`) |

   Titles are used as the runner reports them after its own interpolation (`test.each`,
   `pytest.mark.parametrize`). No case folding or whitespace collapsing is applied.

4. **File-level match.** A selection of `<runner>:<file>` matches every ID that starts with
   `<runner>:<file>::`, and a selection of a whole runner instance matches every ID of that runner
   under the instance root. Matching is on the escaped form.

5. **Duplicates.** When two tests in one listing or one report produce the same ID, the second and
   later ones get a suffix segment `#2`, `#3` in report order, and every ID in that group is marked
   `duplicate`. A selection that names any duplicated ID selects the whole file, with reason
   `duplicate-test-id`. This is safe: the file always runs.

6. **Unknowns.** An ID whose runner token is not one of the seven, or that does not parse, is never
   matched by guess. A selection that cannot be expressed in IDs widens: to the file when the file is
   known, otherwise to every test of the runner, with reason `test-id-unmapped`. A change owned by no
   runner runs every test with a named reason (owner decision R32, 2026-11-09).

7. **Legacy keys.** `go:<package>` reads as the Go package-level ID; `typescript:<runner>:<path>`
   reads as the file-level ID `<runner>:<path>`; a receipt outcome without `testId` is read at the
   granularity of its `unit`. Old records are never rewritten.

## Failure modes

| Failure | Behaviour |
| --- | --- |
| A runner reports no title path (only a file) | The outcome is recorded with the file-level ID and granularity `file` (RNC-V1-005). |
| Two tests share a full name | Ordinal suffix, `duplicate` mark, file-level selection with `duplicate-test-id`. |
| A test is renamed | New ID. Links and evidence under the old ID are dangling and reported; they never narrow. |
| Title contains `::` or `:` | Escaped as `%3A%3A` or `%3A`; parsing stays unambiguous. |
| pytest parameter ID contains `::` | Kept inside its `[...]` part, which is split-protected, then escaped. |
| Unknown runner token or malformed ID | Not matched; widen to the file or the runner with `test-id-unmapped`. |
| Go test declared in an external `_test` package with the same name as an internal one | Same ID twice in one package report; handled as a duplicate. |

## Acceptance evidence

- Table-driven tests in the ID package: format, parse and round trip for every row of the table in
  decision 3, the escape cases, duplicates, legacy keys and malformed input.
- One fixture per runner in which the plan, the receipt and watch print the same ID for the same
  test (shared with `runner-contract` RNC-V1-010 for Jest).
- The trace check maps CTI-V1-001 to CTI-V1-012 to those tests.

## Rollback

The ID is an added field (`testId`) and an added package. To withdraw the change, revert its
commits: readers fall back to the `unit` key, which is unchanged, and no stored record has to be
rewritten. Requirement IDs CTI-V1-001 to CTI-V1-012 stay listed as retired if the change is
withdrawn; they are never reused.
