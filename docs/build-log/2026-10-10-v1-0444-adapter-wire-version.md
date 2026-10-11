# V1-0444: native plugin adapter wire version

The constant in `cmd/corvint/host_adapter.go` is a wire-format version, independent of
plugin package releases. At base `32bfc85a44fd82b5793c7e30b2231200e16c025a`, it and the
local completion reader use `0.1.0`; the shipped Claude Code and Codex package manifests
are already `0.3.1` and `0.2.4`, newer than the ticket's examples.

Rename the constant to `adapterWireVersion`, use it in both native plugin producers and
the local completion admission check, and clarify the existing AHI-001 contract in the
owning AHI spec. The flag, JSON member, wire value, receipt encoding and admitted tuples
are unchanged. No new or amended normative requirement is needed for this mechanical
clarification. Pi's separate tuple stays unchanged. Rollback is to revert this scoped diff.

`TestAHI001AdapterWireVersion` pins the literal wire value, accepts it for both plugin
hosts, rejects package-version substitutions, and exercises producer-to-reader receipt
serialization. The existing wire/session, Pi tuple and native host tests provide focused
regression coverage.

## Evidence and limits

- Corvint query succeeded but returned an unrelated Tasks spec, with three ranked results
  omitted and test candidates withheld; the supplied paths and spec index resolved AHI.
- Corvint affected selected `cmd/corvint` plus broader reader surfaces and retained 22
  frontier unknowns. Focused tests and the requested documentation gates are the authorized
  scope; `make gate` and repository-wide tests are NOT_RUN by owner instruction.
- The baseline wire/session and Pi tuple tests passed (`18.159s`).
- Independent review used a bounded read-only Sol/low reviewer for this mechanical rename.
  It found misplaced traceability prose, which was moved into the proper spec section;
  the reviewer confirmed the repair and reported no remaining findings.
- The first race run failed because the new test supplied a duration where the existing
  deadline seam expects a callback, leaving the production 1.6-second limit active. The
  fixture now uses the existing callback type and one-minute test deadline; production
  timeout behavior is unchanged. The reviewer checked this repair too.
- The first documentation gate run exposed seven frozen line citations shifted by a new
  comment line. Keeping that comment inline with the constant restores original source
  positions without editing decision files; independent review confirmed the repair.
- The explicit receipt check passed: all 18 receipts are current. `cmd/corvint/main.go`
  is unchanged and no receipt repin is needed.
- Final focused tests passed (`11.717s`) with `GOTOOLCHAIN=local`,
  `TMPDIR=/private/tmp/claude-501/gotmp`, `GOMAXPROCS=3` and
  `go test -race -count=1 -timeout 30m ./cmd/corvint`, restricted to
  `TestAHI001AdapterWireVersion`, `TestDogfoodEventGoPythonWireAndSession`,
  `TestPiLocalCompletionTuple`, `TestHostAdapterNormalizesAllClaudeEvents` and
  `TestCodexHostAdapterNormalizesAndFramesNativeEvent`. Toolchain: `go1.27.1`.
- Requested documentation gates passed: `spec-requirements-check`,
  `requirement-definitions-check`, `traceability-tests-check`, `decision-numbers-check`,
  `line-citations-check`, `error-code-ownership-check`, `unbounded-readers-check`,
  `use-case-receipts-check` and `use-case-receipts-test` (five fixture cases). The generated
  REQUIREMENTS.tsv diff changes only 51 existing AHI line locators, with no ID/text changes.
- Installed Core is `1.0.0-rc.3` build 407; Tasks is
  `0.0.0-tcp01-unverified+build.407`. Both official update checks failed with
  `process-after-start-failed: descendant snapshot unavailable`; available versions and
  freshness remain UNKNOWN, with no update attempted. Account usage was NOT_OBSERVED:
  no account-usage tool is exposed in this session.
- The actual worktree index is outside the writable sandbox. Documentation checks use a
  disposable checkout at the same base with the exact changed files staged. The system
  `mktemp` ignores the supplied TMPDIR for a no-argument invocation here; a scratch wrapper
  supplies an explicit template under `/private/tmp/claude-501/gotmp` for those calls.
- Post-commit CEM binding/sealing is NOT_PRODUCED because commits are prohibited. No
  commit, push, decision file, ticket-store write or plugin-manifest edit is part of this
  change. The reported hook fallback `corvint-event-rejected:dogfood-event-deadline` is
  retained as a limitation, not repaired by this rename.
