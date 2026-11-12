# Design: docs-install-path

## Decisions

### The core component is the core version set

Three designs were open:

1. Ship `corvint-docs` inside the `corvint` archive. Rejected: ARTIFACT-BIN-V0-509 keeps that
   archive one binary and the ARTIFACT-GO-V0-002 gate pins its members.
2. A new updater component `docs`. Rejected: CMS-V0-002 asks one install and one rollback for the
   whole set at one version. A second component would be applied and rolled back on its own and
   could leave `corvint-docs` at another release than `corvint`; UPD-V0-004 also binds one apply to
   one component.
3. `--component core` installs the whole set. Chosen. It is what CMS-V0-002 already says, it is the
   unit `corvint-release-install` installs (ARTIFACT-BIN-V0-511), and an operator keeps one command.

UPD-V0-004's "one selected component" still holds: the set is the `core` component's executables.

### Which apps, on which platforms

`internal/update` keeps its own copy of the app list and platform list and a test holds them equal
to `releasecandidate.CoreSetApps` and `releasecandidate.AppPlatforms`, which tests already hold
equal to the release catalog. The production import is avoided because `releasecandidate` pulls
the candidate assembly code into the updater binary.

On a platform in the list every app is required. On a supported core platform outside it
(darwin/amd64) the release spec ships no app archive, so the apply installs `corvint` alone and the
result's qualification names the gap. Refusing there would take away a working Core updater on a
platform the release still ships `corvint` for.

### Release assets

Each app is the release asset `<app>_<goos>_<goarch>.tar.gz`, the name `tools/release-binary`
gives it, with its row in the release's `SHA256SUMS`. The archive must have the layout the
release-binary tool writes and `extractAppBinary` reads: the root `<app>_<goos>_<goarch>/` holding
the executable `<app>`, a `SHA256SUMS` that is exactly the executable's row, and only the notice
files `LICENSE`, `LICENSE-APACHE-2.0`, `LICENSING.md` and `PROVENANCE.md`, each byte-equal to the
`corvint` archive's. The same extraction limits as the `corvint` archive apply.

### Identity

After the platform inspection each app runs `--version` in the smoke home. The line must parse
(`docsenvelope.ParseVersionLine`), name the app, and agree with the `corvint` candidate's line by
`docsenvelope.Mismatch`: one version, and one commit when both are stamped. That is the check the
installed shims run later, so an apply never installs a set its own members would refuse.

### Receipt, activation order and restore

The receipt gains `Members`, one entry per app with its destination, previous digest (empty when
the app was absent) and candidate digest. Each present previous app is kept as
`previous-<app>` in the transaction. An old receipt has no `Members` and means `corvint` only.

Activation switches the apps first and `corvint` last. `corvint` is the receipt anchor that
rollback matches on, so a receipt only matches once the whole set has switched. A member whose
installed bytes already equal its candidate is not rewritten.

When an activation step fails or the context is cancelled, the run restores every member it
already switched, in reverse order, under `context.WithoutCancel` with a bound, and then returns
the error. A test hook runs before each member switch so a test can cancel between two members.

Rollback first checks every member: each must be at its candidate digest or already at its
previous state (bytes equal to the previous digest, or absent when it was absent). Any other state
refuses with the member named, before anything changes. It then restores the apps and `corvint`
last. Because an already restored member passes the check, a rollback interrupted part way can be
run again.

### Equal build

When the candidate `corvint` is the installed build and its bytes are equal, the apply compares
each app destination with its candidate. When all are equal the result stays `unchanged`. When
one is absent or different the apply goes on and installs those members; `corvint` is not
rewritten. This is the reported case: `corvint` installed, `corvint-docs` absent.

### Refusal text

`docsenvelope.NotInstalledMessage` appends the install command. `DIR` is the directory of the
running executable after symlink resolution; when it cannot be read, `--bin-dir` is left out and
the updater's default directory applies. A directory that needs shell quoting is single-quoted.
The message also points at the manual archive install in `docs/INSTALL.md`. The `version-mismatch`
message of `CheckBeside` names the same command. The MCP tool-group hint names it too.

## Failure modes

- A release without app archives refuses with the first missing app named; nothing changes.
- A platform without app archives installs `corvint` alone and says so.
- An outer checksum, inner checksum, notice or version disagreement refuses before any switch.
- An error or cancellation between switches restores the switched members; a process kill between
  two renames leaves a mixed set that every member refuses with `version-mismatch`, and a rerun of
  the apply completes it.
- A member changed after the apply makes rollback refuse with the member named.
- A receipt written before this change rolls back `corvint` alone.

## Acceptance evidence

- `TestUPDV0920ApplyInstallsTheWholeCoreSet`: four executables at candidate digests; result lists
  each member; an app at another version refuses with nothing changed.
- `TestUPDV0921RollbackAndInterruptedSetRestore`: rollback restores `corvint` and removes apps
  absent before; a cancellation after the first app switched restores it.
- `TestUPDV0922MissingAppRefusesAndEqualBuildCompletesTheSet`: a missing archive and a missing
  checksum row refuse with the app named; an equal build installs the absent apps; a platform
  outside the list installs `corvint` alone.
- `TestUPDV0920SetListsMatchTheReleaseCatalog`: the copies equal `releasecandidate`'s lists.
- `TestCMSV0930NotInstalledNamesTheInstallCommand` in `internal/docsenvelope`, and the existing
  shim test in `cmd/corvint` asserting the command in the real refusal.

## Rollback

Revert the change. Receipts with `Members` stay readable by the old code (it ignores the field and
restores `corvint`). Apps already installed stay on disk until removed by hand.
