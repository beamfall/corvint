# Review repair for the stable release reader (V1-2323, PR #1651)

Date: 2026-10-23. Ticket: V1-2323. Spec: `docs/specs/cem-stable-v1.md` CEM-V1-569..572, still
proposed. This entry repairs the eight findings of the independent review of PR #1651. Each fix
has a test that fails without it. Nothing is promoted.

## Decisions

- **The verdict is hashed from the parsed bytes.** The first reader recomputed `verdictSha256`
  from the typed decoding. That decoding turned a `null` `citation.specificity` into `0`, so the
  original verdict still matched. The reader now hashes the canonical value of the parsed result,
  minus `runtime`, `repositoryEnvelope` and `verdictSha256`. It also re-encodes the typed decoding
  and requires the same canonical value, which refuses a `null` or a wrong type even when the
  verdict is resealed. A reference member that is not an array is refused.
- **Signed integers are a parser option, not a wire change.** `RESULT.schema.json` makes
  `citation.specificity` a signed integer, and `wire.Parse` refuses every negative number.
  `wire.ParseSigned` admits negatives down to `-MaxWireInteger` and still refuses `-0`. The CEM
  wire documents keep `wire.Parse`.
- **An accept must be one a verifier can emit.** The reader refuses a `QUALIFIED_EXACT_TUPLE`
  whose `goVersion` is outside the structural allowlist (`verify.StructuralTupleAdmitted`) as
  admission. On an accept, it refuses a sidecar other than `ABSENT` or `EXACT`, a structural proof
  other than `NOT_REQUIRED` or `PROVEN_DECLARED_FILE_PREDICATES`, and a qualified tuple without a
  proven structural predicate. These values come from `internal/cem/verify/stable.go`.
- **Build stamps are reported, not proven.** Go's `vcs.revision` is unauthenticated, and an edited
  binary can carry any stamp. The runner reports the revision as one the binaries report and pins
  each binary by SHA-256. It no longer says the binaries were built from a commit.
- **Suites read committed objects only.** Git runs without any inherited `GIT_*` variable. The
  runner writes the revision's whole tree from `git ls-tree` and `git cat-file --batch` into a
  private directory, refuses any entry that is not a regular file, and runs every suite there.
  `skip-worktree`, `assume-unchanged` and worktree edits cannot reach a suite. The old
  `status --porcelain` check is gone.
- **Only `runtime.goVersion` is masked.** The consumer comparison used to drop the whole
  `runtime` object, which hid a `structuralQualification` difference. It now empties
  `goVersion` only. Each `runtime` must have exactly its two members, and the qualification must
  agree with `structuralProof`. The neutral accept is also compared with the sealed
  `results/accept.json` line.
- **Envelope tests run in the committed tree.** The source-level S0E, envelope-cell and
  adversarial-shape tests moved from the Makefile into the runner, which runs them in the
  materialized tree.
- **A02 stays `OPEN`.** Its decision recognizes the stable result only after integration, and
  `verticalOrder` keeps every unfinished obligation `OPEN`. `MIGRATION.json` and its manifest pin
  are back to the base bytes (`65258c13…01b4`, 32369 B). `TestCEMV1569MigrationA02StaysOpen`
  pins the status.
- **The private directory is resolved.** On macOS, `TMPDIR` sits under the `/var` symbolic link,
  and both verifiers refuse such a repository with `repository-object-unavailable`. The first
  live run of the repaired runner failed every tamper case for this reason; the same binaries
  accepted the same repository on a resolved path. The runner now resolves its directory. The
  earlier PR run passed only because its `TMPDIR` had no link.

## Observed

- The nine new `TestCEMV1569StableReleaseReadiness` cases were run against the old reader with a
  digest shim. Eight failed. The ninth, a resealed `null`, was already refused because the old
  typed digest differed, and it stays as a guard.
- With the `GIT_*` scrub removed, `TestCEMV1572SuitesReadCommittedObjects` failed: `rev-parse`
  ran in the decoy repository that `GIT_DIR` named.
- `TestCEMV1569MigrationA02StaysOpen` fails on the PR's `MIGRATION.json`, whose A02 status was
  `IMPLEMENTED_EXPERIMENTAL`.
- Commit `4fdca31a1` was built in the clean lane worktree with go1.27.3.
  `make cem-release-suites` exited 0 with tamper-matrix 23, citation-vectors 8,
  neutral-criterion 8 and receipt-authentication 19 cases PASS on both binaries, and the
  repository-envelope tests PASS at source level in the committed tree. The sealed lines name
  go1.27.2, so the run also exercised the goVersion mask.
- `corvint affected` selected 175 Go packages and 22 shell units. The four changed packages
  pass (`internal/cem/wire`, `internal/cem/verify`, `internal/companionrelease`,
  `tools/cem-release-suites`). About 80 more packages passed with the host load average at
  360–600. Seven tests in `postmergeworkflow`, `tracerecordrepo`, `authoritystore` and
  `docmaintain` failed under that load with git timeouts or wall-clock stops, then passed when
  rerun alone. `cmd/corvint`, `internal/tasks/cli` and `internal/appmap` (each hit the 30 m
  timeout under load), about 84 remaining packages and the shell units are NOT_RUN. Only the
  four changed packages change behavior: `wire.Parse` is byte-for-byte the same on the
  unsigned path, and the new validator has no callers outside its own package.

## Open

- The verifiers' `repository-object-unavailable` for a repository reached through a symbolic link
  comes from `StableBoundary.Validate`, which reports it as "boundary changed during
  verification". The message misleads, and `OpenStable` had admitted the path.
- The review also noted two schema inconsistencies, left for the owner: the neutral-criterion
  fixture against the Tasks-only binding definition, and the `authentication` const
  `NOT_OBSERVED` against CEM-V1-084.
