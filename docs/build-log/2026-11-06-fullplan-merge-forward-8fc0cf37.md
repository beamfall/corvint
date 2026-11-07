# Fullplan mainline merge-forward at 8fc0cf37

The owner requested merging mainline `8fc0cf37642e7012f4c26671c9ed97a8706d6e09`
into lane head `6f30523c5e00e91a71eafa21e1e2e576a62b8a93`, preserving both
parents and rebinding the full lane against that mainline. No prior commit is rewritten.

The only textual conflict was `docs/specs/falsifiable-packet-v0.md`: both parents
relocated the same content-pinned `NormalizePaths` citation. Keep the assertion and
its anchor, and relocate to the merged source with the line-citation tool.
Generated `REQUIREMENTS.tsv` and `INDEX.json` start from mainline; the owning
`make spec-requirements` target restores the lane requirements after the merge.
The automatically merged README retains both sides' entries. AZU-V0-9920..9922
have no mainline ID collision; STR-V0-069 is the lane's amendment of an existing
requirement, not a new conflicting ID.

The shared Go edits compose independently: hidden-helper discovery and declared
non-source handling remain alongside mainline reader-kind narrowing, driver
containment fixes and session receipt completion timestamps. Mainline renamed
`select.py` to `darwin_select.py`; update the runtime edge, reduced replay and
AZU-V0-9920 reference to retain the lane's dynamically loaded helper dependency.

The complete affected-package run exposed an outdated repository replay
assertion: Python CI sources can be reached without a selected test. Retain the
LANGUAGE_FRONTIER uncertainty for these testless readers, as AFP-V0-560 already
requires. The replay now admits only a selected test or an actually reached
source unit with no tests; its unowned-path and frontier-floor assertions remain.
The focused replay passes. A subsequent whole-package attempt failed the live
dirty-worktree witness check on the uncommitted replay source (`PATH_GLOB_READER`,
`*.go`); retain that failed observation and rerun after committing the repair.
The Go analyzer and session runner complete packages pass.

Rebinding preserves the previous ignored OCM files under the worktree's private
Git evidence directory, because the new base makes them stale. Earlier pre-change
receipts remain explicitly STALE. Preparation without inputs reports missing
citations, intent scope and local outcome; these are not passing verification.
The reviewed previous plan is migrated by exact base-span content, with a fresh
read of the renamed Python loader. Keep every evidence row and every unproduced
reason. Detailed commands, return codes and the final binding are retained in
`/home/russell/lanes/fullplan.tmp/merge-round/` and the owner's merge handback.

Scope: focused package tests and spec/citation checks only. Whole repository
gates, native Darwin qualification and hosted workflow execution are NOT_RUN.
The lane was unsealed before this round and remains unsealed after dogfood-check.
