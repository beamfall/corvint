# Decision 0226 — the sql-native Core comparison base is HEAD without the candidate

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`SNR-V0-007` and `SNR-V0-008` compared the current `cmd/corvint` build and its sorted `go list -deps`
closure against a `git archive` of the pinned commit `core_parent` (`718dfc7d`, 2026-08-25). The pin
was 1,842 commits behind. Core had changed in 1,290 files since then for reasons unrelated to the
candidate, so byte identity could not hold, and no run could reach `SNR-V0-008` or later. Any fixed
pin fails the same way as soon as main moves. The pinned commit also resolved only through
`refs/codex/snapshots/*` refs, so a fresh clone could not export it.

The check exists to show that adding the candidate does not change Core.

The call:

(a) The Core comparison base is `HEAD` with the candidate's two source trees excluded:
`git archive "$core_base" -- . ':(exclude)experimental/analyzers/sqlnative'
':(exclude)cmd/corvint-analyzer-sql-native'`, where `core_base` is `git rev-parse --verify
'HEAD^{commit}'`. `cmd/corvint` is built from the current working tree and from that export with
identical flags and equal-length output paths, and the two binaries and sorted dependency lists
must be byte-identical. The `core_parent` pin is removed.

(b) The candidate's wiring into Core can only be an import of one of those two trees, because the
candidate imports only the standard library and its own package (`go list -deps
./cmd/corvint-analyzer-sql-native`). If Core imports the candidate, the export cannot build or list
`./cmd/corvint`, and the gate refuses. This was shown with a scratch commit that added a blank
import of the candidate package to `cmd/corvint`. Any other difference between the tree and the
export also refuses at `cmp`.

(c) A merge-base against the commit that introduced the candidate was rejected. That commit is as
old as the stale pin, so Core has moved just as far since then.

(d) Limits, recorded in the spec: uncommitted or untracked changes to Core source in the working
tree also refuse, because the export is `HEAD`. Run the gate on a committed tree. A candidate-driven
change to shared root files such as `go.mod` is not isolated by this check. Today the candidate has
no module requirement, so no such change exists.

(e) The `SNR-V0-014` report field `core_parent=<sha>` becomes `core_base=<sha>`.

Consequences: `SNR-V0-007`, `SNR-V0-008`, and `SNR-V0-014` are amended in
`docs/specs/sql-native-ratchet-gate-v0.md`. `script/check-sql-native-ratchets_test.sh` runs the
script's own export command against a fixture commit, and checks that Core source is kept and both
candidate trees are excluded.

Rollback: revert the commit. That restores the `core_parent` pin, which is unpassable on the current
tree, and the previous requirement text and report field.
