# reuse: review round 6 (typed validation cache keys)

Base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3`. Ticket: V1-1097. Requirement amended in place,
with its ID unchanged: AFP-V0-2321 in `docs/specs/affected-plan-reuse-v0.md`. An independent
fix-only review of the round-5 head `ba0107fb0` confirmed the three round-5 findings fixed and
returned one new P1. Round 6 is the capped fix round. It makes exactly the bounded fail-closed fix
below and nothing else.

## Finding and decision

**Worktree validation cache keys collided across kinds (P1, AFP-V0-2321).** The snapshot cached
every worktree check in one string-keyed map. A file's raw-bytes check used the file path as its
key, a granted-directory walk used `walk:` plus the directory, and an absent-input check used
`absent:` plus the path. A tracked file named `walk:a`, declared as a scope of package `a`,
cached its file success under `walk:a`. The walk of package `a`'s directory then found that entry
and returned the file's empty detail without walking, so an empty directory added below `a`
(which an `os.ReadDir` in the test sees) still let `a` be reused.

Decision: every cached check is keyed by a `validationKey{kind, path}` struct, where the kind is
`file`, `walk` or `absent`. No path string can stand in for another kind's check. `Recheck` reruns
the same typed set in (kind, path) order and names the kind and path in its drift error. Evidence:
`TestValidationKeysDoNotCollide` in `internal/testreuse/cachekey_test.go`. With the colliding
scope committed, the unchanged checkout reuses both packages (negative control). After the empty
`a/testdata/extra/`, package `a` is `UNTRACKED_GRANTED_ENTRY` and `b` stays reused. Against the
round-5 code the same test failed, with `a` REUSE `RECORD_DIGEST_EQUAL`. The test is skipped on
Windows, where a colon is not valid in a file name. Reuse is refused off linux in any case
(round 5).

## Disabled for 1.0

None added in this round.
