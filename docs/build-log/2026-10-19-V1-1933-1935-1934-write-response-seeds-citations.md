# 2026-10-19 V1-1933, V1-1935, V1-1934: write responses, seed collisions and pinned citations

Tickets: V1-1933 (GitHub #1194), V1-1935 (GitHub #1196) and V1-1934 (GitHub #1195).

## Decision

`TSA-V0-200..204`, `TSA-V0-210..216` and `TSA-V0-220..226` are added to
`docs/specs/test-source-analysis-v0.md`. They are accepted under the owner's standing auto-accept
of lane requirements (2026-10-11). The ID blocks skip the ranges held by open PRs (#1178, #1179,
#1184 and #1187, TSA-V0-100..146) and by the peer lane for V1-1919 (TSA-V0-160..176).

- V1-1933 (`WRITE-RESPONSE-UNASSERTED`, `WRITE-RESPONSE-UNRESOLVED`):
  - The reading is static, in its own file (`internal/testsource/validity_js_write_response.go`),
    and called once from `exampleFacts`.
  - Recorded network evidence is not read: `corvint-network-evidence/0` carries no method or
    step. That is a residual.
  - The abstention is an advisory finding with an `abstained:` message, not
    `SeverityAbstained`, so an advisory rule's uncertainty never degrades hygiene.
- V1-1935 (`SEED-*`):
  - The server's validation is declared by the project in `.corvint/seed-rules.json`
    (`corvint-seed-rules/0`) and read at the report revision. Corvint does not infer it from
    server code.
  - Seed findings are advisory and go into the file report only.
  - Values the rules need must be literals; anything else abstains.
- V1-1934 (`corvint tests citations`):
  - A new read-only subcommand reads documents and cited files through `rev-parse`, `ls-tree`,
    `cat-file` and `log` only.
  - Citation syntax: `path[:START[-END]][#Symbol][@REV]` in inline code. A
    `<!-- corvint-cite-revision: REV -->` line sets the document default.
  - No new refusal codes were added; the existing `tests-*` codes are reused.

## Evidence

- Focused tests: `go test ./internal/testsource/` passes, and so does
  `go test -run TestTests ./cmd/corvint/`.
- Base fails: on 714393264 the new tests do not build (`undefined: CitationFinding`).
- Mutations, 8 of 8 killed:
  - writes: GET filter removed, asserted check removed, wait-started-before-action check removed
    (killed only after `TestWriteResponseLateWaitIsNotExemplar` was added);
  - seeds: inclusive overlap made strict, tenant scoping removed, weekly step changed to 8 days;
  - citations: range bound loosened, missing-path check removed.
- No live qualification on a real suite or review corpus was run (NOT_RUN).

## Independent review

One Codex round (`codex exec -s read-only`) raised 13 findings. Eleven were fixed in the second
commit, and each fix has a test that fails on the first commit:
- `TestCitationsReviewForms`: explicit `@HEAD` resolves to the repository HEAD; `:0` is out of
  bounds; a default-revision declaration in fenced code is ignored; fences close only on their
  own character and length.
- `TestDecodeSeedRules`: a trailing `]` after the rule object is refused.
- `TestSeedReviewForms`: the last key wins; spread/computed/shorthand records abstain; values are
  typed; two seeds of one example are not siblings; RFC 3339 timestamps compare as instants.
- `TestWriteResponseReviewForms`: any `!` negation abstains; a then callback must read its
  response parameter.

Two findings are recorded as residuals rather than fixed:
- a response binding reassigned before its assertion is still credited to the wait;
- a nested `test.step` inside a step is read as part of the outer step.
