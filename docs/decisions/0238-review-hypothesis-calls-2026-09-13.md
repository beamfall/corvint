# Decision 0238 — calls from the gate receipt, clause admission, and console hypotheses

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

A bug-hunt entry in `docs/agent-memory/ideas.md` listed six unconfirmed hypotheses. Each was
reproduced against its real entry point. This decision records the contract calls the confirmed
ones needed. No requirement IDs are added.

(a) Cross-format anchor blobs (amends decision 0229(a), `HDCV0-023`). An anchor whose `blob` is 40
hex digits against a path pinned with a 64-hex SHA-256 blob, or the reverse, was reported
`STALE_ANCHOR`. IDs in different Git object formats never compare, so their difference says nothing
about whether the content changed. The call: staleness is decided only within one object format; a
`blob` in the other format disqualifies the anchor, and an unestablished clause downgrades with
`NO_QUALIFYING_SOURCE`. The alternative, keeping `STALE_ANCHOR`, was rejected because it asserts a
content change that was never observed.

(b) Format characters in clause IDs (amends decision 0229(f), `HDCV0-023`). A clause ID admitted
Unicode format characters (general category `Cf`), such as U+200B ZERO WIDTH SPACE or U+202E RIGHT-TO-LEFT
OVERRIDE, because they are neither whitespace nor controls. Two IDs that render identically were
then distinct, which defeats the duplicate check a reviewer relies on, and a bidirectional override
reorders the rendered line. The call: a clause ID is also free of `Cf` characters and is otherwise
`invalid-clause`. Review text is prose and is unchanged.

(c) Code-less console error objects (amends `LAC-V0-022`). The dashboard evidence reader admitted an
`corvint-dashboard-error/0` object on stdout with an empty `code` as a refusal carrying no code, while
the stderr path already required a code. A refusal without a code cannot render "its exact codes"
(`LAC-V0-009`), so the page claimed the tool refused without saying what it refused. The call: an
error object with no code is a failure on either stream, reported as a malformed reply.

(d) Changes `git status` does not report (amends `GOC-V0-010`). `script/gate-receipt` judged the
worktree clean from `git status --porcelain`, which omits a tracked file edited under skip-worktree
or assume-unchanged and every ignored file. The gate then ran on bytes that differ from the tree the
receipt binds, and an ignored `build/` or `dist/` directory holding `.go` files is compiled by
`go ./...`. The call: such a flagged path, or an ignored `.go` file outside a directory Go skips
(`.`- or `_`-prefixed, or `testdata`), refuses with the existing `worktree-not-clean` reason, at
both the start stamp and the record. Other ignored files stay outside the check: they are derived
local state (`.corvint/index/`, caches, built binaries) that a real checkout always carries, and no
tracked `//go:embed` names an ignored path. An ignored `.go` file under a nested module is refused
too; that is conservative, and the operator removes it.

(e) Interrupted injection harness (evidence integrity for `GAG-V0-006` and `GAG-V0-004`).
`script/go-archive-gate-injection_test.sh` trapped HUP, INT and TERM with its cleanup function, which
returned. After an interrupt it resumed; past a tolerated failure (`run_gate`, or the `wait` in case
4) its checks ran over the removed directory, where an empty command substitution passes, so an
interrupt during case 4's drain printed the final pass line and exited 0. That exit status is the
evidence the spec records. The call: a signal ends the harness with 128 plus the signal number
through its EXIT trap. The same idiom in other scripts is filed in `docs/agent-memory/ideas.md`.

Refuted, no change: `CanonicalJSONUnbounded` allocating before the bound. `CATN-V0-012` requires
`Verify` to measure the complete canonical bundle, so it must be encoded first; allocation is linear
in the caller's in-memory value (about 11 times its size in a 1 MiB and 16 MiB probe), and nothing
outside `internal/docviews` imports that package.

Rollback: revert the commit that carries each call; no emitted wire other than the named frontier,
refusal, or receipt changes.
