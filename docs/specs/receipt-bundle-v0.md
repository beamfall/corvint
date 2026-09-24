# Receipt bundle V0

Owner: Russell Lewis
Date: 2026-09-23
Requirement prefix: `RCB-V0`
Intent status: accepted (decision 0376; ticket V1-0197 with two owner-confirmed deviations)
Delivery status: experimental
Authoritative inputs: `../../AGENTS.md` invariants 1, 2, 4 and 7, ticket V1-0197 (filed from the
owner's review of external agent-governance tooling, accepted for 1.0), `cem-0.2-canonical-binding.md`
for the CEM, `corvint-witness-v0.md` for the witness report, `../DOGFOOD.md` for the dogfood
report, `go-only-cutover-v0.md` GOC-V0-010 for the full-gate receipt, `gate-ledger-v0.md` GL-V0-006
for why ledger records stay out, and `release-artifact-integrity-v0.md`, whose publication receipt
reader this slice sits beside.

## Agent digest
- Claim: `corvint cem export` copies one change's CEM, witness, dogfood and gate receipts into a new directory whose manifest an offline script verifies.
- Status: accepted (decision 0376; ticket V1-0197 with two owner-confirmed deviations) / experimental
- Exists: `internal/receiptbundle`, `cmd/corvint/cem_export.go`, `script/verify-receipt-bundle.sh` and its `_test.sh` (`make receipt-bundle-verify-test`), the `cem export` read-only case.
- Blocked on: nothing for intent; the owner confirmed both ticket deviations (decision 0376). Signing, publication, archives and governance adapters are non-goals.
- Read next: Ticket deviations (owner-confirmed); RCB-V0-002 (manifest), RCB-V0-004 (inclusion) and RCB-V0-005 (output); Failure modes.

## Human intent and scope

Corvint's content-addressed receipts are the evidence an external audit or governance system wants:
which change was bound, what evidence it cites, which obligations stayed `NOT_RUN`, which dogfood
steps were `NOT_PRODUCED`, and whether a full gate passed. Today they live as separate files with
separate readers, some under ignored paths and some only on stdout. An auditor must learn each
format and trust the machine that holds them.

Measurable job: for one change, one read-only command gathers the receipts that already exist into
one directory with one manifest, and a script that needs only `sh` and a SHA-256 tool proves every
listed byte is the byte that was exported. The bundle is data. It proves integrity of the copies,
not the truth of their claims, and grants no authority.

## Command choice

The export is an action of the existing `cem` verb, not a new root verb (a standing owner
constraint). A bundle is keyed by one change, and the CEM is what defines a change's identity: its
base revision and the target it is bound to. `cem` already carries read-only actions (`status`,
`verify`, `provenance`) and lists its Go-only actions last. `witness` was rejected because it
compiles one report and its spec scopes it to that report. `dogfood` was rejected because it is
the session-keyed, mutating completion-lease lifecycle.

## Ticket deviations (owner-confirmed)

The owner confirmed two departures from ticket V1-0197 on 2026-09-24 (decision 0376):

1. The ticket asks for the "gate ledger". The bundle carries the GOC-V0-010 full-gate receipt
   instead, because GL-V0-006 forbids any product path from reading gate-ledger records.
2. The ticket offers a directory "or archive". Only the directory is delivered: an archive adds a
   format and a tool dependency to the offline verifier, and the directory already carries every
   byte.

## Requirements

- `RCB-V0-001`: `corvint [--root PATH] cem export --map MAP --expected-base REV --target REV
  --output DIR [--witness REPORT]` is a `cem` action listed last in `cemActionOrder`, in the `cem`
  help and in the help-choice list. `--map` is repository-relative. `--expected-base` is the
  caller's independent base, as `cem verify` requires (CEM-CB-010); the map's own base is never
  the authority. `--target` is the commit that binds the map. `--witness` names a saved `corvint witness --json` report, because that report is never
  persisted. On success it prints `{"ok": true, "mutates": false, "tool": "cem-export", "bundle",
  "manifestSha256", "receipts"}` and exits 0. Every refusal is the ordinary CEM error envelope with
  exit 2.
- `RCB-V0-002`: The bundle is the directory DIR holding `manifest.json` and `receipts/` with one
  exact byte copy per present receipt: `cem.json`, `witness.json`, `dogfood-report.json` or
  `gate-receipt.txt`. The manifest is line-oriented so a POSIX shell can read it. Line 1 is
  `{"profile":"corvint-receipt-bundle/0","base":B,"target":T,"receipts":[`, where B and T are the
  full commit IDs of the verified CEM's `baseRevision` (equal to `--expected-base`) and of
  `--target`. Then comes one receipt object per
  line, comma-terminated except the last, and the final line is `]}`. A present receipt's keys, in
  this order, are `kind`, `state` (`present`), `file`, `sha256` (lowercase hex of the copied bytes),
  `source`, `base` (omitted for the gate receipt, which binds no base), `target`, and
  `notRunOrNotProduced`. An absent receipt's keys, in order, are `kind`, `state` (`absent`), `source`
  and `reason`. Receipts appear in the fixed order `cem`, `witness`, `dogfood`, `gate-receipt`, and
  each kind appears exactly once.
- `RCB-V0-003`: `notRunOrNotProduced` is always an array. For a JSON receipt it lists every string
  value equal to `NOT_RUN`, `NOT_PRODUCED` or the CEM discrimination state `not-run`, as
  `{"pointer","value"}` with an RFC 6901 pointer, walking object keys in sorted order and arrays in
  index order. A receipt that is not JSON (the gate receipt) carries an empty array. The export
  never drops, adds or rewrites an axis.
- `RCB-V0-004`: Inclusion rules. Every receipt is read without following a final symlink, must be
  a regular file of at most 4 MiB, and is copied byte for byte.
  - `cem` comes from `--map`. It is required. It must pass the canonical verification `cem verify`
    runs with the same `--expected-base` and `--target`: the resolved expected base equals the
    map's base, the patch derived from B to T matches `patchSha256`, and the inherited checks pass
    (`cem verify` reports evidence drift as not ok, so drift is refused with `evidence-drift`). T
    must also commit the map at
    `.corvint/change.cem.json`, and the working-tree map must equal that committed blob byte for
    byte (CEM-CB-009, `excluded-artifact-mismatch`). A map that T does not commit fails with
    `bundle-map-uncommitted`: the export refuses rather than record a difference. A map that is
    absent, unreadable, not a valid CEM, or fails any of these checks fails the export with
    `map-unavailable` or the verifier's own code, and writes nothing.
  - Receipt revisions are compared as strings with the full lowercase commit IDs B and T, before
    and without any resolution, so `HEAD`, a branch name or a short ID binds another revision.
  - `witness` comes from `--witness`. Its `profile` must be `corvint-witness/0`, `range.base` must
    equal B, and `range.head` must equal T.
  - `dogfood` comes from `.corvint/dogfood-report.json`. Its `profile` must be
    `corvint-dogfood-change/0`, `base` must equal B, and `target` must equal T.
  - `gate-receipt` comes from `$GIT_DIR/corvint/release-gate-receipt`. Its bytes must be exactly
    the canonical line `corvint-gate-receipt/0 COMMIT TREE DIGEST` and one final LF, as
    `script/release-checklist` reads it: no leading space, no CR, no other line. COMMIT must equal T
    and TREE must equal the tree of T; any other value binds another revision.

  An optional receipt that fails one of these checks is listed as absent, never synthesized, with
  one of four reasons. `not-supplied` means `--witness` was not given. `not-found` means the file
  does not exist. `unreadable` means it is not a bounded regular file, does not parse, or has the
  wrong profile. `binds-other-revision` means it names another base or target. Gate-ledger records
  (GL-V0-006) are excluded by rule, because nothing in Corvint's product path may read them. The
  full-gate receipt is the gate evidence a bundle carries.
- `RCB-V0-005`: The export writes only DIR. DIR must be absolute and must not exist, and its parent
  must resolve. The resolved parent is opened once as a directory handle (`os.Root`), and that
  handle and each directory above it are compared by file identity, never by path text, with
  every protected directory: the worktree root, the per-worktree Git directory, the common Git
  directory, the primary worktree (the common directory's parent, when not bare), a `core.worktree`
  that the common `config` sets, and each linked worktree that `$COMMON/worktrees/*/gitdir`
  records. Case variants and volume aliases therefore cannot slip through. Known limit: in a
  `git init --separate-git-dir` layout without `core.worktree`, nothing in the common directory
  names the primary worktree (Git itself reports the Git directory as the main worktree), so an
  export run from a linked worktree cannot protect it; an export run from that primary worktree
  itself is refused earlier, because the repository opener rejects that layout. A violation fails with `bundle-output-refused` before any receipt is read or
  anything is written. This follows from
  invariant 4: `cem report` and `ocm report` write inside the repository, but they are mutating
  actions, so they set no precedent for a read-only command. Every directory and file is created
  through the opened handle, so a parent swapped after the check is never written: the directory
  mode 0700 and every file mode 0600 with exclusive create. A write failure is `publish-failed` and removes
  the directory the export created, and nothing else.
- `RCB-V0-006`: `script/verify-receipt-bundle.sh DIR` needs only `sh`, `sed`, `awk`, `find`, `sort` and
  `sha256sum` or `shasum -a 256`: no Corvint binary, index, repository or network. It first checks
  the manifest's structure and exits 2 unless the manifest is exactly six LF-terminated lines: the
  header line, the `cem`, `witness`, `dogfood` and `gate-receipt` lines in that order with `cem`
  present, and `]}`. A present line must name its kind's one fixed file, which makes every file
  name unique and rules out traversal, and carry a 64-hex digest; `base` and `target` must be
  exactly 40 or 64 lowercase hex characters. It then prints
  `manifest sha256 HEX`, then one line per listed file (`MATCH PATH HEX`, `MISMATCH PATH
  expected=HEX actual=HEX`, or `MISSING PATH`, where a symlink counts as missing), then one `EXTRA
  PATH` line per unlisted non-directory entry, then `PASS` (exit 0) or `FAIL` (exit 1). An absent,
  symlinked, foreign or malformed manifest exits 2.
- `RCB-V0-007`: A bundle grants nothing. The export signs nothing, publishes nothing, contacts no
  network, and records nothing in any ledger. No Corvint command reads a bundle, so a bundle is
  never an input to ranking, evidence, authority, or learning. A `PASS` proves only that the copies
  equal the manifest. It does not prove that the receipts' claims are true, or that the manifest
  came from Corvint.

## Non-goals and simpler baseline

The simpler baseline is copying the four files by hand and running `shasum`. That gives no binding
check, no record of which receipts were missing and why, and no axis listing.

Out of scope: signing or attestation, publication or upload, archives (tar or zip), redaction of
receipt content, a hosted or daemon ingestion service, adapters for any governance product, bundles
that span several changes, reading the private gate ledger, and any authority for a bundle's
content. A bundle can disclose repository information and stays wherever the caller put it.

## Failure modes

| Failure | Behaviour |
| --- | --- |
| DIR relative, existing, parentless, or inside any worktree or Git directory, by any alias | `bundle-output-refused`; nothing written |
| `--map` absent, a symlink, oversized, or not a CEM | `map-unavailable` or the parser's code; nothing written |
| `--expected-base` differs from the map's base, or the patch digest differs | `base-revision-mismatch` or `patch-digest-mismatch`; nothing written |
| `--target` does not commit the map, or commits different bytes | `bundle-map-uncommitted` or `excluded-artifact-mismatch`; nothing written |
| Cited evidence is stale, ambiguous, or deleted at `--target` | `evidence-drift`; nothing written |
| `--expected-base` or `--target` does not resolve to a commit | the resolver's existing code; nothing written |
| Optional receipt missing, unreadable, foreign, or bound to another revision | listed absent with its reason; the export still succeeds |
| Disk full or permission error mid-write | `publish-failed`; the created directory is removed |
| Receipt tampered, removed, or file added after export | verifier `MISMATCH`, `MISSING`, or `EXTRA`, then `FAIL` |
| Manifest lines reordered, duplicated, dropped, or naming another file | verifier exits 2 |
| Manifest edited so that digests match tampered files | not detected: integrity without signing (RCB-V0-007) |

## Acceptance criteria and testing matrix

- Unit: `internal/receiptbundle` tests cover present and absent receipts, axis pointers with
  `~0`/`~1` escaping, sorted keys and `not-run`, binding by exact full ID (`HEAD` refused), the
  canonical gate line and tree, wrong profiles, oversize and symlinked receipts, a symlinked,
  uncommitted, edited, wrong-base or drifted map, and output refusal by case variant, symlinked
  parent, Git directory outside the worktree, `core.worktree`, primary and sibling worktrees, plus
  cleanup after a failed write. Each was checked to fail when its guard is removed.
- CLI: an export on a canonical `cem/0.2` fixture verifies `PASS` with the manifest digest the
  envelope reported, an in-worktree output is refused, and the read-only-verbs case proves the
  repository tree, `.git` included, is byte-identical after an export.
- Script: `make receipt-bundle-verify-test` covers match, tampered, missing, symlinked, extra, and
  unusable manifests, including duplicated, reordered, traversal, absent-cem, unclosed, CRLF and
  50-hex-base manifests.
- Live: the bundle for PR #122's sealed CEM
  (`.corvint/changes/ace0a96bd5ffcfa2af8013e23a1cf3220b46c24f.cem.json`) with its witness report is
  exported and verified. The manifest digest and the verifier output are in `../BUILD-LOG.md`
  (2026-09-23 V1-0197).

## Traceability

| Requirement | Evidence |
| --- | --- |
| RCB-V0-001 | `TestCEMExportBundleVerifiesOffline`, `TestAnchorActionsParseLikeTheirSiblings`, `TestReadOnlyVerbsWriteNothing` |
| RCB-V0-002 | `TestExportCopiesBoundReceiptsAndListsTheRestAbsent` |
| RCB-V0-003 | `TestExportCopiesBoundReceiptsAndListsTheRestAbsent`, `TestExportBindsTheWitnessAndGateReceiptToTheTarget`, `TestNotRunAxesEscapePointersAndSortKeys` |
| RCB-V0-004 | `TestExportCopiesBoundReceiptsAndListsTheRestAbsent`, `TestExportBindsTheWitnessAndGateReceiptToTheTarget`, `TestExportRequiresAValidMap`, `TestExportRefusesADriftedCEM` |
| RCB-V0-005 | `TestExportRefusesOutputsItMustNotWrite`, `TestExportRefusesACaseVariantOfTheWorktree`, `TestExportRefusesEveryWorktreeAndGitDirectory`, `TestWriteFailureRemovesTheBundle`, `TestCEMExportBundleVerifiesOffline`, `TestReadOnlyVerbsWriteNothing` |
| RCB-V0-006 | `script/verify-receipt-bundle_test.sh`, `TestCEMExportBundleVerifiesOffline` |
| RCB-V0-007 | structural: only `cmd/corvint/cem_export.go` imports `internal/receiptbundle`, and the package imports no network, signing, or ledger code |

## Rollout, rollback, and drift

Rollout is this change. To roll back, remove `internal/receiptbundle`, `cmd/corvint/cem_export.go`,
the `export` entry in `cemActions` and `cemActionOrder`, the help lines, and the verifier script
and its Makefile target. Also restore DR-0040 to the eleven-action list and its prior candidate
digest. No persisted state exists to migrate. Bundles already written stay valid for the verifier
that shipped with them. A new receipt kind, or a change to the manifest line shape, needs a new
`profile` value, and this spec and the verifier change in the same commit.
