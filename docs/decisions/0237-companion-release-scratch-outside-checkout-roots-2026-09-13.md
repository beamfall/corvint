# Decision 0237 — companion release refuses a scratch directory inside a checkout root

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`PUB-V0-011` bounded the output location and the bundle name but said nothing about the scratch
directory. `companionrelease.Run` ran `os.MkdirAll(opts.Scratch)`, created `toolchain/` under it,
and ran `go env GOVERSION` there before any containment check. A scratch inside a checkout root
was refused only later, by `gitstatus.StatusIn` inside the clean-tree check, with the unattributed
"not supported safely" error, after the directories were already created in the checkout.
`TestRunRefusesScratchInsideCheckoutRoot` failed at the base with `[d scratch/]` left under the
Corvint root and `[d toolchain/]` left under a symlink-reached taskman directory.

The call: `Run` refuses, before any write, a scratch directory that is or lies inside either
checkout root. It uses `internal/gitstatus`'s directory-identity ancestry walk, exported as
`ScratchOutside(ctx, candidate, roots...)` with the three gitstatus roots passed unchanged, rather
than a second copy. The walk compares `os.SameFile` identity, so a symlink, case or Unicode alias of
a root cannot pass. A not-yet-created scratch suffix cannot be a root, so the walk starts at the
nearest existing ancestor of the symlink-resolved path. The alternative, a lexical comparison on
resolved paths like `validateOutputParent`, was rejected because `EvalSymlinks` preserves case
aliases on a case-insensitive volume.

Not covered, filed in `docs/agent-memory/bugs.md`: a checkout root that lies inside the scratch
directory (Run later clears named scratch subdirectories), and a case-alias output parent, which
the lexical `validateOutputParent` does not refuse.

Consequences: `PUB-V0-011` text and its traceability row are amended. `internal/gitstatus` changes
only the helper's name and arity; `StatusIn` behavior is unchanged. No `analyzerSchemaID` bump:
neither package is under `internal/contextindex`. `script/corvint-companion-release-gate` already
places scratch at `$parent/build`, disjoint from both roots.

Rollback: revert the commit. That restores the unexported three-root `scratchOutside`, removes
`validateScratch` and its test, and restores the unamended `PUB-V0-011`.
