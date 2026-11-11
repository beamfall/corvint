# Affsym fix round: link targets that leave the tree (V1-5368)

The link-target part of the first review-fix round on the affstatus and V1-5368 stack, carried
from `docs/build-log/2026-11-11-affstack-fix-round.md` on branch lane/affstack-fix (78f494bd3)
when the symlink work was split from the held name-listing gate (see
`docs/build-log/2026-11-11-affstack-split.md`). Range base `origin/main`
`4c3c203336a6cc4a9772af39b1a771f34236138e`. AFP-V0-9965 stays proposed, with experimental
delivery; no requirement ID was added and no owner acceptance was recorded. Each defect got a
failing test first, and every fix has one recorded mutation. Source commits: e4547588e (failing
test), aa39c84b9 (fix), ce9a9959b (failing test), c02d2f643 (fix).

## What was wrong

- **P2.4: link targets that leave the tree.** A relative target that climbed above the root
  resolved to no path, even when it came back by the repository's own name (`../repo/x`) or
  through a link outside. An absolute target that named the tree through an alias of an
  ancestor (`/tmp` against `/private/tmp`) also resolved to no path. Both are misses.
- **Found in the hunt: lexical cleaning of a link target.** `linkTarget` passed an absolute
  text inside the tree through `filepath.Rel`, which cleans it lexically. `<tree>/hopin/../m9`,
  with `hopin -> r9/deep`, got the target `m9`, but the kernel reaches `r9/m9`. This predates
  the range.

## Invariants now enforced

- A link target that leaves the tree is resolved on the file system from the tree's physical
  path (`Symlinks.Root`). `physicalPath` cuts raw components so that a `..` after a link climbs
  from the link's target. A target that re-enters becomes the repository path it reaches. A
  target that cannot be evaluated becomes the root, so every path is UNKNOWN. A target that
  stays outside keeps its text. An absolute target below the tree or repository by name is cut,
  not cleaned.

Precision cost: one file-system evaluation per link whose target leaves the tree. A dangling or
unreadable outside path widens the plan to every dirty path. A tree whose physical path cannot
be read leaves the inventory unknown (one UNKNOWN for the whole plan).

## Hunt table

Each form tried has one maintained row in the named test. "Pre" is the result before the fix;
every row passes at the head.

| Class | Test | Forms tried | Pre |
|---|---|---|---|
| Link re-entry (P2.4) | `TestSymlinksReenter_AFPV09965`, physical and alias trees | reenter, deep/reenter, chain, climb_through_alias, climb_dangling (both trees); absolute_alias (physical tree); absolute_physical (alias tree) | missed |
| | | climb_link_dotdot, absolute_alias_climb (added with the fix; they fail under its mutations); hopin plus absolute_link_dotdot | absolute_link_dotdot missed in the physical tree |
| | | `away` (stays outside) covers nothing; every other row/tree pair | selected |

## Mutations

Each mutation was applied to the committed fix, run, and restored with `git checkout`.

| Fix | Mutation | Result |
|---|---|---|
| P2.4 climb | resolveSymlink returns outside on a climb | FAIL, 11 relative rows over both trees |
| P2.4 raw components | `filepath.Clean` the prefix in physicalPath | FAIL, climb_link_dotdot in both trees |
| P2.4 alias | drop the physicalPath fallback in linkTarget | FAIL: absolute_alias, absolute_alias_climb (physical tree); absolute_physical (alias tree) |
| Absolute cut | `filepath.Clean(text)` before the cut | FAIL, absolute_link_dotdot (physical tree) |

## Not modelled, and open misses

- **A case-insensitive read.** This predates the range; it is recorded in AFP-V0-9965's "Not
  modelled". Reproducer, a probe run in the shell package on the source branch: a unit holding
  only `cat Script/a.sh`, `. Script/a.sh` or `bash SCRIPT/A.SH` is not selected for
  `script/a.sh`. The control `cat script/a.sh` is selected. The same class probably covers other
  plugins' path literals and link targets; that is untested.
- **Windows link forms.** Junctions, other reparse points, `.lnk` files and the text stand-ins
  Git writes under `core.symlinks=false` are not read as links.
- **Outside links that change after the plan.** An outside link created or retargeted after
  the plan is not seen, because outside links are read when the plan is built.
