# Decision 0397 — corvint-tasks moves in tree as a separate companion binary

Date: 2026-09-25. Status: accepted. Authority: repository owner instruction "move corvint-tasks
into corvint and ticket everything" (2026-09-25). Ticket V1-0317.

Corvint Tasks lived in its own repository, `github.com/Beamfall/corvint-tasks`. Three problems
followed from that. The companion gate cloned the sibling checkout's unpinned `HEAD`
(panel finding M3). The Tasks binary printed a constant version, so two different builds could not
be told apart. Corvint also kept hand copies of the Tasks wire vocabulary with no compile-time link
(panel finding F5, V1-0311).

1. **In-tree separate companion binary.** The source of `beamfall/corvint-tasks@800682d` is
   imported as one squash commit, without its history (decision 0331):
   - `cmd/corvint-tasks` holds the command;
   - `internal/tasks/**` holds the packages.

   `corvint-tasks` stays a separate binary. It is never a `corvint` subcommand, and the `corvint`
   binary links no Tasks mutation code (invariant 7, decision 0322). It is still not a Core
   prerequisite, so `DCW-V0-011` holds. Its roadmap is not a Core claim.
2. **Import direction.** `internal/tasks/boundary_test.go` walks the module and enforces four rules:
   - Tasks imports no Core package.
   - `internal/tasks/wire` imports no module package.
   - Core imports only `internal/tasks/wire`.
   - `cmd/corvint` imports no Tasks package. It reaches `wire` only through `internal/taskman`.
3. **Module layout: the same module.** The measured basis is `go test -count=1
   ./internal/tasks/...` at the imported commit:
   - 549 s wall with packages in parallel, 104 s user CPU and 209 s system CPU;
   - the host load average was 255 to 310 on 12 cores, so the wall time is inflated;
   - `internal/tasks/authority` takes 544 s of that wall time, and every other package takes under
     61 s.

   The Core gate runs `go test -p 1`, so it serializes these packages. That adds roughly the 313 s
   of CPU time to `go-test`, which is comparable to `cmd/corvint` alone (about 591 s). The gate delta
   on a quiet host is `NOT_RUN`. A separate module would keep that time out of `./...`. It would
   also need a second `go.mod`, a replace or version link for the wire package, and a second vet and
   test path in every gate. The same module lets `internal/taskman/decode.go` import the Tasks
   error codes and ticket record keys (`wire.Codes`, `wire.TicketRecordKeys`) instead of keeping
   copies.
   That closes F5 with a compile-time link.

   `make tasks-test` runs the Tasks packages and vet on their own. `make tasks-build` builds the
   stamped binary.
4. **Build identity.** `cmd/corvint-tasks` takes `-ldflags "-X main.build=N"`, where N is the same
   first-parent build number `corvint` uses (`PUB-V0-021`). `--version` prints
   `0.0.0-tcp01-unverified+build.N`. The companion bundle stamps both binaries, and its README now
   says so instead of claiming an identity the binary lacked (M3, V1-0308).
5. **Companion release.** `corvint-companion-release` takes only `-source-root`, and `-tasks-root`
   is retired.
   - `script/corvint-companion-release-gate` no longer clones a Tasks checkout or reads
     `CORVINT_TASKMAN_REPO`. It refuses a positional argument.
   - The bundle keeps profile `corvint-companion-bundle/2`, module label `corvint-tasks` and
     `source/corvint-tasks-src.tar.gz`, so the retained verifier is unchanged.
   - That archive is now the standalone subset of the recorded Corvint commit: `go.mod`, the
     notices, `cmd/corvint-tasks/**` and `internal/tasks/**`.
6. **Specification.** `docs/specs/corvint-1.0-product-and-release-v1.md` now says "in-tree separate
   companion binary (decision 0397)". `PUB-V0-011` and the companion-inventory text in
   `docs/specs/public-release-v0.md` describe one checkout root.

The imported `SPEC.md`, reviews, examples and Makefile of the old repository are not carried. The
ATCP-V0 specification recovery is V1-0310, and freezing the old repository is V1-0318.

Rollback: revert the following, then restore `CORVINT_TASKMAN_REPO`, `-tasks-root` and the
two-root `Options`.

- the import commit;
- the `internal/taskman/decode.go` change;
- the companion-release and gate-script change;
- this specification amendment.

The old repository at `800682d` is untouched. There is no data migration: the `.taskman/` store
format and journal are unchanged.

## CAL-V0-022 addendum — explicit pack derivation

The owner's S8 approval on 2026-09-27 accepts in-process scope derivation with explicit
`CORVINT_SNAPSHOT_FORMAT=pack` opt-in and conservative default claims. The adapter in
`internal/tasks/scopes` may import `internal/contextindex` and `internal/runtimeenv`;
`internal/tasks/cli/scope_test.go` may import `internal/contextindex` to construct its pack fixture.
These are the only exceptions to rule 2. Other Tasks packages still import no Core package,
Tasks wire still imports no module package, and Core still links no Tasks mutation code.
`TestImportViolationControls` retains negative controls for imports outside those exact edges.

V1-0456 amends rule 5's source-archive subset to retain the pack adapter's dependency closure:
`internal/contextindex`, `internal/diagnostic`, `internal/gitstatus`, `internal/projectprofile`,
`internal/pythongrammar`, `internal/pythonsyntax`, `internal/runtimeenv`, `internal/secretscreen`
and `internal/untrackedallowance`, including their files and notices at the same immutable source
commit/tree. The module and root licensing/provenance files remain exact. This changes packaging,
not the allowed import edges above. The actual exported tarball must rebuild offline using
`go build ./cmd/corvint-tasks`, independently of the npm/VSIX gate; a passing import-direction
test alone does not qualify the artifact. The full companion release remains separately gated.

## Issue 464 addendum — dispatcher process-group Owner

Proposed as an agent decision on 2026-10-04 under the owner's goal of landing the open ticket
work. The owner approved it the same day ("Admit dispatch + #481 runner", 2026-10-04), together
with the #481 attempt-runner edge below.
`docs/specs/process-group-owner-v0.md` names #464 (native V1-0654) as a consumer of the Owner in
`internal/groupreap` and says reimplementation is unnecessary. Core's `cem/gitrun` also uses that
package, so it cannot move under `internal/tasks/`. Rule 2 therefore gains one more exact edge:
`internal/tasks/dispatch` may import `internal/groupreap`, which imports only the Go standard
library. No other Tasks package may import it except the two #481 files admitted below, and no other Core edge is admitted.
`TestImportViolationControls` keeps negative controls for `internal/groupreap` imported from
another Tasks package and for another Core package imported from `internal/tasks/dispatch`.
Rule 5's source-archive subset adds `internal/groupreap` so the exported tarball still rebuilds
offline. Rollback is reverting the dispatcher's Owner use and removing this edge and prefix.

## Issue 443 addendum — contextindex Git runner closure

Agent decision, 2026-10-04, made while integrating PR #524 (issue #443); it is not a direct owner
statement. That change routes `internal/contextindex` Git reads through Core's `internal/cem/gitrun`,
which imports `internal/cem/cemcode` and `internal/groupreap`. This is a transitive dependency of the
admitted contextindex edge, not a new Tasks import edge; rule 2 is unchanged. Rule 5's source-archive
subset adds `internal/cem/cemcode` and `internal/cem/gitrun` so the exported tarball still rebuilds
offline (`TestCurrentTasksSourceArchiveBuildsOffline`). Rollback is removing these two prefixes
together with the contextindex change that needs them.

## Issue 481 addendum — attempt-runner process-group Owner

Owner decision, 2026-10-04: the owner approved the attempt-runner `internal/groupreap` edge in the
same statement and directed that the import-direction exception be applied on 2026-10-04. The
experimental attempt runner (`docs/specs/corvint-tasks-attempt-runner-v0.md`, issue #481, native
V1-0677) starts its command through the same Owner. Rule 2 gains one more exact edge, limited to
two files: `internal/tasks/cli/attempt_run.go` and `internal/tasks/cli/attempt_run_test.go` may
import `internal/groupreap`. The rest of `internal/tasks/cli` stays refused, and these files admit
no other Core package. `TestImportViolationControls` keeps negative controls for another cli file
importing `internal/groupreap` and for another Core package imported from the admitted files, and
positive controls for the two admitted edges. Rule 5 needs no change because `internal/groupreap`
is already in the source-archive subset. This approves the boundary only; the attempt-runner
profile stays experimental. Rollback is reverting the attempt runner and removing this two-file
edge; the #464 edge and the archive prefix stay.

## V1-0804 addendum — service helper process-group Owner

Agent decision, 2026-10-05, made while delivering the owner-requested issue #500 follow-up
(native V1-0804, SERVICE500-007); it is not a direct owner statement and is listed as an owner
question in that change. The optional user-service helper wrapper (`service run-helper`,
`docs/specs/corvint-tasks-user-service-v0.md`) starts each helper through the same #464 Owner
instead of reimplementing group ownership. Rule 2 gains one more exact edge, limited to two files:
`internal/tasks/service/descendants_linux.go` and `internal/tasks/service/descendants_linux_test.go`
may import `internal/groupreap`. The rest of `internal/tasks/service` stays refused, and these
files admit no other Core package. `TestImportViolationControls` keeps a negative control for
another service file importing `internal/groupreap` and for another Core package imported from the
admitted file, and positive controls for the two admitted edges. Rule 5 needs no change because
`internal/groupreap` is already in the source-archive subset. Rollback is reverting the helper
wrapper and removing this two-file edge; the #464 and #481 edges stay.

## V1-0955 addendum — know-how write secret screen

Agent decision, 2026-10-07, made while delivering GitHub issue beamfall/corvint#655 (native
V1-0955); it is not a direct owner statement and is listed as an owner question in that change.
The proposed know-how note writer (`docs/specs/corvint-tasks-know-how-notes-v0.md`, KHN-V0-004)
screens note text, reasons, route tokens and paths with Core's shared `internal/secretscreen`
patterns instead of keeping a second, drifting copy. Rule 2 gains one more exact edge, limited to
two files: `internal/tasks/mutation/know_how.go` and `internal/tasks/mutation/know_how_test.go`
may import `internal/secretscreen`. The rest of `internal/tasks/mutation` and every other Tasks
file stay refused, and these files admit no other Core package. `TestImportViolationControls`
keeps negative controls for another mutation file and a CLI file importing `internal/secretscreen`
and for another Core package imported from the admitted file, and positive controls for the two
admitted edges. `internal/secretscreen` imports only the standard library. Rule 5 needs no change
because it is already in the source-archive subset (V1-0456). Rollback is reverting the know-how
writer and removing this two-file edge; the other edges stay.
