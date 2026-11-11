# Tracked-symlink operand (V1-5368, AFP-V0-9965)

Carried from the affstatus build-log on branch lane/affstack-fix (commit 1f43cd7d7) when the
symlink-target work was split from the held name-listing gate (see
`docs/build-log/2026-11-11-affstack-split.md`). Source commit IDs below are on that branch; on
this branch the pin is 0ca9f59f2, the spec dcb15572d and the fix a7c5cc314. The pin and the
fix test plan without a change status here; every row's expected misses are unchanged.

V1-5368 (lane symop) suspected that a shell unit listing a directory through a tracked symbolic
link is not selected for a change below the link's target. `TestSymlinkOperand_V15368` (new file
`internal/liveverify/affected/shell/symlink_operand_test.go`) builds the head tree of each change
(an added file present, a deleted one absent) with six links and eight shell tests, and records
per change the tests whose output it can alter and the subset the plan neither selects nor covers
by an UNKNOWN scope. It was committed first against the unchanged code (02269337a); every row
matched the probe. Verdict: **confirmed**. The planner matches path tokens by text and the shell
plugin records `ls link` as `/link/` and `cat link/a.sh` as `link/a.sh`, so no token names a path
below `real/`. The miss is older than the name-listing narrowing: the content read misses too.

| Case | Change | Tests it can alter | At the base | Can miss | Now |
| --- | --- | --- | --- | --- | --- |
| (a) added below target | `real/new.sh` added | `ls link` | not selected, BOUNDED | yes | `shell:symlink-target` |
| (a) added data file | `real/new.txt` added | `ls link` | UNKNOWN (`UNOWNED_DIRTY_PATH`, by chance) | no at plan level; gate selection allows that reason | frontier as well |
| (b) deleted below target | `real/gone.sh` deleted | `ls link` | UNKNOWN (`UNINDEXED_SOURCE_PATH`, by chance) | no | frontier as well |
| (b) deleted below declared documentation | `guide/old.md` deleted | `ls docslink` | not selected, BOUNDED | yes | frontier |
| (a) added below declared documentation | `guide/new.md` added | `ls docslink` | not selected, BOUNDED | yes | frontier |
| (c) edit below target | `real/a.sh` modified | `cat link/a.sh` | not selected, BOUNDED | yes | frontier |
| (c) deep edit | `real/sub/deep.sh` modified | none (listings only) | not selected | no | frontier (wider) |
| trailing component | `real/sub/new.sh` added | `ls link/sub` | not selected, BOUNDED | yes | frontier |
| link to outside a listed tree | `ext/new.sh` added (`listed/out -> ../ext`) | none: `ls -R` does not follow it | not selected | no (refuted) | frontier (wider) |
| that link retargeted | `listed/out` type-changed | `ls -R listed` | selected | no | selected, frontier |
| target below another listed directory | `listed/sub/new.sh` added (`intolisted -> listed/sub`) | `ls -R listed`, `ls intolisted` | only `ls -R listed` | yes | frontier |
| link retargeted | `link` type-changed | all three readers | UNKNOWN (`UNOWNED_DIRTY_PATH`) | no at plan level | frontier |
| documentation link retargeted | `guide/glink` type-changed | `cat guide/glink/a.sh` | not selected, BOUNDED (declared documentation) | yes | frontier |
| dangling target filled | `missing/x.sh` added (`dangle -> missing`) | `ls dangle` | not selected, BOUNDED | yes | frontier |
| dangling link deleted | `dangle` deleted | `ls dangle` | selected by its own path | no | selected, frontier |

Eight cases can miss a test. Note: `internal/gateselect` allows `UNOWNED_DIRTY_PATH`, and
`corvint test` uses that allowlist too, so the two "by chance" rows are covered only at plan level.

Rule: AFP-V0-9965 (proposed, delivery experimental; spec commit c1298bf79, fix 9d870dd31).
`corvint affected` lists the tree's links (`git ls-tree -r` of the measured commit, with targets
read from the tree, and each dirty path checked at both sides). In a graph with the shell plugin,
the plan carries `LANGUAGE_FRONTIER shell:symlink-target:<path>` for each dirty path that is a
link, lies below one, or lies at or below a link's resolved target. Gate selection, `corvint test`
and `tools/shell-test-select` already turn a `shell:` frontier into the shell suite, so no wire
reason was added. (The source branch also amended the AFP-V0-9961 limit text, which is held.) A
caller of `Select` that passes no list plans exactly as before; the test asserts the table's "At
the base" column for that path.

Tracked symlinks in this repository: `git ls-files -s | grep -c '^120000'` prints `0` on the base
151e466a4 (as on origin/main, measured by the orchestrator) and at the lane head;
`git ls-tree -r 151e466a4 | grep -c '^120000'` prints `0`. Here the rule fires only for a change
that adds a link. `corvint affected --base 151e466a4` from the lane binary carries no
`shell:symlink-*` entry. The added `git ls-tree -r` reads about 1.0 MB in about 10 ms on this
repository.

Mutations (each reverted afterwards; commands run through `script/heavy.sh env GOTOOLCHAIN=local
GOMAXPROCS=2 go test -count=1 -run ...`):

| Mutation | Test | Result |
| --- | --- | --- |
| M1: delete the `graph.symlinkTargets(normalized)` append in `select.go` | `TestSymlinkOperand_V15368` | FAIL: exactly the eight "can miss" rows |
| M2: delete the `WithSymlinks(affected.TreeSymlinks(...))` line in `planAffectedGraph` | `TestAffectedSymlinkTargetCLI_AFPV09965` | FAIL: "frontier false scope BOUNDED", with and without `--base` |
| M3: drop `belowAny(dirtyPath, regions)` from the rule | `TestSymlinkOperand_V15368`, `TestSymlinks_AFPV09965` | both FAIL |
| M4: drop the `Changed` check from the rule | `TestSymlinks_AFPV09965` | FAIL ("gone: [], want covered true"); the shell test passes because a deleted link is matched by its own path |

Windows: each new symlink test skips at run time on `GOOS=windows` (creating a link needs a
privilege Windows test hosts lack), so `make windows-vet` still type-checks them. No build tag.

Rollback: revert a7c5cc314 (code, tests, a citation repin, and the UC-CHANGE-CONSEQUENCE
receipt and use-case record-entry digest repins) and dcb15572d (spec), with the later link-target
commits of this branch. Keep 0ca9f59f2 as the pin of the base behaviour, with the misses as its
expectation.
