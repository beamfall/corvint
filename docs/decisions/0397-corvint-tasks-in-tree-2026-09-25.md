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
