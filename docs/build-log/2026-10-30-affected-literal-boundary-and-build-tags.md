# Affected over-selection reports: literal boundary and build-tagged tests

Orchestrator lane request (2026-10-30, Corvint 1.0 friction sweep): fix two `corvint affected`
over-selection tickets, V1-3207 (path literals matched by substring) and V1-3201 (a `//go:build`
edit in a test file treated as an unbounded fan-out). Base `b469ab710f361cd0ae0c4f456311f6457b6fc94f`.
Spec: `docs/specs/affected-plan-v0.md` AFP-V0-840 and AFP-V0-841.

## Reproduction

The evidence below comes from a binary built from the lane worktree (`go build ./cmd/corvint`),
run with `--no-cache`.

- V1-3207: adding `script/core-n1-source.sh` (from `bb7cfc04e`) selects 49 units: 43
  `UNBOUNDED_READER` (AFP-V0-012 rule (d), reached by every dirty path) and six
  `PATH_LITERAL_READER`. The installed `~/.local/bin/corvint` (built 2026-10-27) selects 74 for the
  same tree, and only it picks `internal/analyzerhtmlcss`, `internal/jstestprovider` and
  `internal/contextindex`. The reported 90 units therefore came from a stale binary. A single-word
  literal already names only a whole file name (V1-0290), so `source.sh` never matched
  `core-n1-source.sh`. Five of the six literal readers are shell tests with `"$dir/script/"`
  directory prefixes, which the requested rule (exact path, or a directory prefix ending in `/`)
  keeps. The sixth, `internal/console`, matched through `"&lt;/script&gt;"`, which lexed to the
  root-anchored `/script`.
- V1-3201: the reported file `internal/doccompiler/draft_fifo_unix_test.go` and
  `internal/analyzercap/race_disabled_test.go` each select 56 units for a `//go:build` edit, the
  same set as a comment-only edit, both dirty and committed with `--base`. The owning package is
  the only `DIRECT_TEST_CHANGE`; the remainder are rule (d) and declared-read-scope readers that
  any change reaches. `go:build-constraint-variants` is a repository-wide unknown (167 frontier
  sites) carried by every plan, so it does not depend on this change. Not reproduced at
  `b469ab710`.

## Decision

AFP-V0-840 extends the AFP-V0-505 closing-tag rule to the escaped `&lt;/`, in any letter case. On
this repository the change alters only `internal/console`'s witness: it reads the Git top level, so
it stays selected as an unbounded reader and the plan is still 49 units. That keeps the fail-closed
rule intact. AFP-V0-841 pins the existing build-constrained test-file behaviour, with the variant
unknown retained. Rule (d) is left unchanged because its readers really can read any path.

## Limits

The TypeScript and shell lexicons still read `</script>` as `/script` (TS route stubs under
`internal/jstestprovider/testdata/` and `tools/web-flows/test/e2e.mjs`). None of those units was
selected here. Rule (d) unbounded readers remain the dominant cost of small plans.
