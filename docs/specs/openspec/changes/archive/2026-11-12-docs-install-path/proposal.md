# Change proposal: `corvint-update --component core` installs the whole core set

Change name: `docs-install-path`
Ticket: V1-5466 (`corvint flows|how|docs` exit 2 with `not-installed` because nothing installs
`corvint-docs` beside `corvint`).
Product requirement served: PROD-INT-004 (each product installable and usable on its own),
PROD-DOCS-005 (`corvint flows`) and PROD-DOCS-007 (`corvint how`) in
`docs/specs/PRODUCT-PLAN-1.0.md`, all accepted (decision 0596), and the still open whole-set clause
of CMS-V0-002 in `docs/specs/core-bundled-apps-v0.md`.
Owning capability spec: `docs/specs/operator-update-v0.md` (prefix `UPD-V0`) for the updater and
`docs/specs/core-bundled-apps-v0.md` (prefix `CMS-V0`) for the refusal text.
Requirement IDs this change adds: UPD-V0-920, UPD-V0-921, UPD-V0-922 and CMS-V0-930. On
`origin/main` at `072760041` the `UPD-V0` IDs in use are 001 to 017, 367, 368 and 910, and the
`CMS-V0` IDs in use end at 916; the fetched branch `claude/appmap-gaps` adds none in either prefix
(checked 2026-11-12).
Intent status: accepted (auto, 2026-11-12; standing auto-accept of 2026-10-11 and G14, see the
acceptance log)
Delivery status: experimental (implemented on `claude/docs-install`; a published release carrying
the app archives is NOT_RUN until the release workflow publishes them)
Written before dispatch: yes.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/operator-update/spec.md`).

## Why

Decision 0595 moved `flows` and `how` into `corvint-docs`; decision 0594 had already made
`corvint docs` an exec-only shim. `corvint` alone now refuses all three with `not-installed`
(exit 2). The release ships `corvint-docs`, `corvint-docs-mcp` and `corvint-mcp` as their own
archives beside the one-binary `corvint` archive (ARTIFACT-BIN-V0-509, -522), and
`corvint-release-install` places them together (ARTIFACT-BIN-V0-511). The updater does not:
`corvint-update` knows the components `core` and `tasks`, and `core` installs only `corvint`.
An operator who keeps Core current with the updater therefore never gets the docs product, and
the refusal says only "install corvint-docs beside this executable or on PATH" without naming how.

CMS-V0-002 already states the rule this change delivers: "`corvint-update` MUST install and roll
back the whole set", one version for every executable. Its traceability row says the clause is
still open.

## What Changes

- `corvint-update apply --component core` installs `corvint` and, on a platform the release ships
  app archives for, every core-set app (`corvint-docs`, `corvint-docs-mcp`, `corvint-mcp`) from the
  same release into the same directory. Each app archive is bound by the release `SHA256SUMS`, its
  inner `SHA256SUMS` row and notices equal to the `corvint` archive's, and its `--version` must name
  the `corvint` candidate's version (UPD-V0-920).
- The prepared receipt records every member with its previous and candidate digest (an absent
  previous member is recorded as absent). `rollback` restores the whole set: it puts back each
  member's previous bytes and removes a member that was absent before. A run that fails or is
  cancelled after some members switched restores the switched members before it returns
  (UPD-V0-921).
- A missing or unbound app archive on such a platform refuses the apply with the app named and
  every destination unchanged. When the installed `corvint` already is the release's build, apply
  installs the missing or differing members instead of answering `unchanged` (UPD-V0-922).
- The `not-installed` refusal of `corvint docs|flows|how`, of `corvint-docs --corpus` and of the
  MCP docs bridge names the exact command,
  `corvint-update apply --allow-network --component core --bin-dir DIR`, with `DIR` the running
  executable's directory; a missing `corvint` (the `--corpus` read) names the manual install, since
  the updater needs an installed `corvint`; the `version-mismatch` refusal names the same command
  (CMS-V0-930).
- `docs/INSTALL.md`, the two owning specs, the CMS-V0-002 traceability row and the release spec's
  non-goal line are updated.

## Impact

- Code: `internal/update` (apply, receipt, rollback), `internal/docsenvelope` (refusal text),
  `internal/mcp/toolgroup` (hint), `cmd/corvint-update` (usage line).
- Operators: one `corvint-update apply --component core` now writes up to four executables in the
  bin directory; the state directory keeps one previous copy per member that existed.
- No wire change: refusal codes and exit statuses stay; the update result gains a `set` list.
- Releases: the updater needs the app archives on the GitHub release (a `.github/` change outside
  this folder); without them the core apply refuses on the three app platforms.

## Non-goals

- A new `docs` component of the updater. It would let `corvint-docs` and `corvint` drift to
  different releases, which CMS-V0-002 forbids, and it would add a second apply for one version set.
- Putting `corvint-docs` inside the `corvint` archive. ARTIFACT-BIN-V0-509 keeps that archive one
  binary and the ARTIFACT-GO-V0-002 gate pins its inventory.
- Publishing the app archives and their `SHA256SUMS` rows on GitHub releases. That is the release
  workflow under `.github/`, a separate change; until a release carries them, the apply refuses
  with the missing archive named and the manual archive install in `docs/INSTALL.md` stays the path.
- Installing a set where `corvint` is not installed yet (the updater still replaces an existing
  install), changing the Tasks component, or adding Windows.
- Power-loss durability across several files. The set switch is a sequence of atomic renames.

## Failure modes

- **A release without app archives** (every release published before the workflow change): the
  apply refuses on darwin/arm64, linux/amd64 and linux/arm64, naming the first missing archive, and
  changes nothing. `check` is unchanged.
- **A platform without app archives** (darwin/amd64): the release spec ships none there, so the
  apply installs `corvint` alone, as today, and says so in the result's qualification line. The
  docs product stays `not-installed` on that platform.
- **An app archive from another build or tampered**: an outer checksum, inner checksum, notice or
  `--version` disagreement refuses before any destination changes.
- **An interruption between member switches** (error or cancellation): the switched members are
  restored under a bounded cleanup context before the run returns. A process kill between two
  renames leaves a mixed set; every member then refuses with `version-mismatch` (CMS-V0-002), and
  rerunning the apply completes the set. This is stated, not hidden.
- **A member changed by hand after the apply**: rollback refuses with that member named, as it does
  for `corvint` today.
- **An old receipt without members**: rollback treats it as a `corvint`-only receipt, as before.

## Acceptance evidence

- UPD-V0-920: an apply over a fixture release with the three app archives leaves all four
  executables at the candidate digests; an app whose `--version` names another version refuses and
  changes nothing.
- UPD-V0-921: rollback after that apply restores `corvint` and removes the three apps that were
  absent before; a run cancelled after the first app switched returns an error and leaves every
  destination as it was.
- UPD-V0-922: a release missing one app archive, or one whose `SHA256SUMS` lacks its row, refuses
  with the app named; an apply at the installed build with the apps absent installs them.
- CMS-V0-930: the refusal text test names the command and the executable's directory, and the shim
  test asserts it on the real `corvint docs` path.
- Focused package tests, gofmt, vet, `make windows-vet` and the documentation gates, recorded in
  the handback. A real published release is NOT_RUN until the workflow publishes app archives.

## Rollback

Revert the change. `corvint-update --component core` again installs `corvint` alone, the refusal
text returns to the old wording and the requirement IDs leave the specs. Receipts written with
members stay readable by the reverted code, which ignores the unknown field and restores `corvint`.
Apps installed by the new code stay on disk until the operator removes them.
