# Decision 0142 — Untracked-path allowance for range impact

Date: 2026-09-12. Status: accepted. Authority: repository owner instruction, 2026-09-12.

One untracked file refused range `impact` (`unsupported-impact-worktree`), emptied learned-trace
reads (`blocked-mixed-worktree`), and failed the archive gate. That held even when Git ignored the
file or no path the result depends on could reach it. Each refusal was correct under its contract,
so the fix is a contract change. `internal/gokernel/repository.go:198@80c62a46` still reports every dirty
path, untracked ones included; that observation is unchanged, and each consumer classifies it.

The owner call: an untracked path is allowed without degradation only when it is (a) excluded by
the repository's ignore rules or (b) provably disjoint from every path the result depends on.
Anything not provably allowed still degrades, and every tracked change still refuses.

1. Ignored paths. Git status never lists an ignored path, so it never reaches classification. The
   committed `.gitignore` files belong to the captured tree. Status also honours the repository's
   `.git/info/exclude`, which no commit carries. That was already true before this decision and is
   filed in `docs/agent-memory/ideas.md`. The global `core.excludesFile` is disabled for range
   impact.
2. Range impact. The receipt depends on the committed range, the Go sources and markers of the
   captured tree, and accepted ADRs. It names verification commands but does not run them.
   Under rule `corvint-untracked-allowance/0` (`internal/untrackedallowance`), a listed untracked
   path overlaps, and so still refuses, when any of these holds:
   - it is a directory entry (a nested repository);
   - it ends in `.go`;
   - its basename is `go.mod`, `go.sum`, `go.work`, `go.work.sum`, `.gitignore`, or
     `.gitattributes`;
   - it has a `vendor` segment;
   - it lies under a directory that holds a tracked `.go` file, or under any descendant of one.
     `//go:embed` and `testdata` reach there.

   Every other untracked path is allowed.
3. Binding. Range impact keeps `range.status: "CLEAN"` and its bytes when nothing is allowed.
   Otherwise it sets `range.status: "UNTRACKED-ALLOWED"` and adds
   `range.untrackedAllowance {rule, count, sha256}`. The digest is SHA-256 over the rule followed by
   NUL, then each sorted allowed path followed by NUL. `range.statusSha256` still binds the raw
   status bytes.
4. Not applied to the loose and archive gates. `go build` stamps `vcs.modified` from
   `git status --porcelain` (Go 1.27 `cmd/go/internal/vcs/vcs.go`), which lists untracked paths.
   The gate pins `vcs.modified` to `false`. In a scratch module, one untracked `notes.md` turned the
   stamp from `false` to `true`. Every listed untracked path therefore reaches the gated binary and
   is not disjoint from it. The gates keep refusing it. An ignored path is invisible to both and is
   already allowed.
5. Not applied to learned traces. `internal/tracerecordrepo/read.go:95@241a6e10` and `authorityTraceState`
   in `internal/contextindex/history.go` stay blocked on any dirty path. A trace record names
   opened and changed paths across historical revisions, so proving disjointness means reading the
   store, and the blocked state exists to forbid that read. The query packet is also a
   Python-oracle compatibility surface. A tree-bounded trace allowance is filed in
   `docs/agent-memory/ideas.md`.
6. The range change alters `internal/contextindex` production code, so `analyzerSchemaID` becomes
   `corvint-analyzer/31`.

Specs: `GPK-V0-060` and `GPK-V0-061` (`go-production-kernel-migration-v0.md`) and
`ARTIFACT-GO-V0-009` (`release-artifact-integrity-v0.md`, which records the gate refusal).

Rollback: restore the unconditional refusal in `verifyRangeSnapshot`, remove
`range.untrackedAllowance` and the `internal/untrackedallowance` package, and bump
`analyzerSchemaID` again. A receipt that already carries an allowance then names a producer that no
longer exists.
