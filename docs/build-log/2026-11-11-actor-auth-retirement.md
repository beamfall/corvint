# 2026-11-11: retire the test-only actor authorization check (V1-4822)

Owner accepted the deletion on 2026-11-06 (expert-panel restructuring plan, step 24d). Base and
rollback source: `b55635262788687c3eea9a08a94378e4ab8856a4`. No new requirement IDs and no
product wire change.

## Reachability

`testsource.ActorAuthorization`, `AnalyzeActorAuthorization` and the `corvint-tests-actor-authorization/0`
report had no caller outside `internal/testsource/actor_auth_test.go` and
`actor_auth_roles_test.go`: `git grep` over the repository (excluding change evidence) finds no
importer, command, MCP tool or `.corvint` inventory naming them.

## What was removed and what stayed

- Removed: `internal/testsource/actor_auth.go` (813 lines), `actor_auth_rails.go` (712),
  `actor_auth_test.go` (344) and `actor_auth_roles_test.go` (145): 2014 Go lines.
- Kept, moved unchanged into `internal/testsource/ruby_lexical.go` (197 lines): the Ruby
  logical-line reader (`rubyLine`, `rubyLines`, heredoc skipping, `stripRubyComment`), the block
  openers and `rubyBlockEnd`, the value-position keyword reader, and `sortedKeys`. The RSpec
  validity, seed-parity, test-discovery and teardown readers use them (TSA-V0-084, TSA-V0-090,
  TSA-V0-396). This is why the older `claude/deadcode` branch trimmed the two files instead of
  deleting them; a file of its own names what the helpers are now for.
- Net: 1817 Go lines removed.
- The only code that the retired check alone used was in the deleted files: a
  `golang.org/x/tools/cmd/deadcode -test` run over `./internal/testsource/` after the deletion
  reports one unreachable function, `writeStep.has` in `validity_js_write_response.go`, which the
  deleted files never called (already unreachable before this change; left alone).

## Spec

`docs/specs/test-source-analysis-v0.md`: TSA-V0-033 to TSA-V0-039 are marked
`(retired 2026-11-11 by V1-4822; ...)` with their IDs and bodies kept as history; their traceability
rows mark the five tests `(removed)` and the status `retired`. The header, digest and the
TSA-V0-396 row now name `ruby_lexical.go`. The rest of the spec stays live.

Open follow-up tickets that extend the retired check (V1-1774 CLI or lint surface, V1-1775 Pundit,
V1-1776 more shapes) are now moot; closing them is a task-store write outside this lane.

## Verification

- `GOTOOLCHAIN=local go vet ./internal/testsource/` and the same with `GOOS=windows`: pass.
- `GOTOOLCHAIN=local go build ./...`: pass.
- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/testsource/`:
  `ok` in 7.7 s.
