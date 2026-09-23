# Receipt bundle V0

Owner: Russell Lewis
Date: 2026-09-23
Requirement prefix: `RCB-V0`
Intent status: accepted (ticket V1-0197, owner 2026-09-23)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariants 1, 2, 4 and 7, ticket V1-0197 (filed from the
owner's review of external agent-governance tooling, accepted for 1.0), `cem-0.2-canonical-binding.md`
for the CEM, `corvint-witness-v0.md` for the witness report, `../DOGFOOD.md` for the dogfood
report, `go-only-cutover-v0.md` GOC-V0-010 for the full-gate receipt, `gate-ledger-v0.md` GL-V0-006
for why ledger records stay out, and `release-artifact-integrity-v0.md`, whose publication receipt
reader this slice sits beside.

## Agent digest
- Claim: `corvint cem export` copies one change's CEM, witness, dogfood and gate receipts into a new directory whose manifest an offline script verifies.
- Status: accepted (ticket V1-0197, owner 2026-09-23) / implemented
- Exists: `internal/receiptbundle`, `cmd/corvint/cem_export.go`, `script/verify-receipt-bundle.sh` and its `_test.sh` (`make receipt-bundle-verify-test`), the `cem export` read-only case.
- Blocked on: nothing. Signing, publication, archives and any governance-system adapter are non-goals.
- Read next: Command choice; RCB-V0-002 (manifest) and RCB-V0-004 (inclusion); Non-goals; Failure modes.

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

## Requirements

- `RCB-V0-001`: `corvint [--root PATH] cem export --map MAP --target REV --output DIR
  [--witness REPORT]` is a `cem` action listed last in `cemActionOrder`, in the `cem` help and in
  the help-choice list. `--map` is repository-relative. `--target` is the revision the CEM is bound
  to. `--witness` names a saved `corvint witness --json` report, because that report is never
  persisted. On success it prints `{"ok": true, "mutates": false, "tool": "cem-export", "bundle",
  "manifestSha256", "receipts"}` and exits 0. Every refusal is the ordinary CEM error envelope with
  exit 2.
- `RCB-V0-002`: The bundle is the directory DIR holding `manifest.json` and `receipts/` with one
  exact byte copy per present receipt: `cem.json`, `witness.json`, `dogfood-report.json` or
  `gate-receipt.txt`. The manifest is line-oriented so a POSIX shell can read it. Line 1 is
  `{"profile":"corvint-receipt-bundle/0","base":B,"target":T,"receipts":[`, where B and T are the
  full commit IDs of the CEM's `baseRevision` and of `--target`. Then comes one receipt object per
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
  - `cem` comes from `--map`. It is required: a map that is absent, unreadable or not a valid CEM
    fails the export with `map-unavailable` or the map parser's own code, and writes nothing.
  - `witness` comes from `--witness`. Its `profile` must be `corvint-witness/0`, `range.base` must
    resolve to B, and `range.head` must resolve to T.
  - `dogfood` comes from `.corvint/dogfood-report.json`. Its `profile` must be
    `corvint-dogfood-change/0`, `base` must resolve to B, and `target` must resolve to T.
  - `gate-receipt` comes from `$GIT_DIR/corvint/release-gate-receipt`. It must be one
    `corvint-gate-receipt/0 COMMIT TREE DIGEST` line whose COMMIT resolves to T.

  An optional receipt that fails one of these checks is listed as absent, never synthesized, with
  one of four reasons. `not-supplied` means `--witness` was not given. `not-found` means the file
  does not exist. `unreadable` means it is not a bounded regular file, does not parse, or has the
  wrong profile. `binds-other-revision` means it names another base or target. Gate-ledger records
  (GL-V0-006) are excluded by rule, because nothing in Corvint's product path may read them. The
  full-gate receipt is the gate evidence a bundle carries.
- `RCB-V0-005`: The export writes only DIR. DIR must be absolute and must not exist, and its parent
  must resolve. After the parent's symlinks are resolved, DIR must lie outside the worktree root,
  the per-worktree Git directory and the common Git directory. A violation fails with
  `bundle-output-refused` before any receipt is read or anything is written. This follows from
  invariant 4: `cem report` and `ocm report` write inside the repository, but they are mutating
  actions, so they set no precedent for a read-only command. The directory is created mode 0700
  and every file mode 0600 with exclusive create. A write failure is `publish-failed` and removes
  the directory the export created, and nothing else.
- `RCB-V0-006`: `script/verify-receipt-bundle.sh DIR` needs only `sh`, `sed`, `awk`, `find`, `sort` and
  `sha256sum` or `shasum -a 256`: no Corvint binary, index, repository or network. It prints
  `manifest sha256 HEX`, then one line per listed file (`MATCH PATH HEX`, `MISMATCH PATH
  expected=HEX actual=HEX`, or `MISSING PATH`, where a symlink counts as missing), then one `EXTRA
  PATH` line per unlisted non-directory entry, then `PASS` (exit 0) or `FAIL` (exit 1). A manifest
  whose present-receipt count differs from its file and digest pairs prints `MALFORMED` and fails.
  An absent, symlinked or foreign manifest exits 2.
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
| DIR relative, existing, parentless, or inside the worktree or a Git directory | `bundle-output-refused`; nothing written |
| `--map` absent, a symlink, oversized, or not a CEM | `map-unavailable` or the parser's code; nothing written |
| CEM base or `--target` does not resolve to a commit | the resolver's existing code; nothing written |
| Optional receipt missing, unreadable, foreign, or bound to another revision | listed absent with its reason; the export still succeeds |
| Disk full or permission error mid-write | `publish-failed`; the created directory is removed |
| Receipt tampered, removed, or file added after export | verifier `MISMATCH`, `MISSING`, or `EXTRA`, then `FAIL` |
| Manifest edited so that digests match tampered files | not detected: integrity without signing (RCB-V0-007) |

## Acceptance criteria and testing matrix

- Unit: `internal/receiptbundle` tests cover present and absent receipts, axis pointers, binding by
  resolved revision, symlink refusal, output refusal, and the required map.
- CLI: an export on a canonical `cem/0.2` fixture verifies `PASS` with the manifest digest the
  envelope reported, an in-worktree output is refused, and the read-only-verbs case proves the
  repository tree, `.git` included, is byte-identical after an export.
- Script: `make receipt-bundle-verify-test` covers match, tampered, missing, symlinked, extra, and
  unusable manifests.
- Live: the bundle for PR #122's sealed CEM
  (`.corvint/changes/ace0a96bd5ffcfa2af8013e23a1cf3220b46c24f.cem.json`) with its witness report is
  exported and verified. The manifest digest and the verifier output are in `../BUILD-LOG.md`
  (2026-09-23 V1-0197).

## Traceability

| Requirement | Evidence |
| --- | --- |
| RCB-V0-001 | `TestCEMExportBundleVerifiesOffline`, `TestAnchorActionsParseLikeTheirSiblings`, `TestReadOnlyVerbsWriteNothing` |
| RCB-V0-002 | `TestExportCopiesBoundReceiptsAndListsTheRestAbsent` |
| RCB-V0-003 | `TestExportCopiesBoundReceiptsAndListsTheRestAbsent`, `TestExportBindsTheWitnessAndGateReceiptToTheTarget` |
| RCB-V0-004 | `TestExportCopiesBoundReceiptsAndListsTheRestAbsent`, `TestExportBindsTheWitnessAndGateReceiptToTheTarget`, `TestExportRequiresAValidMap` |
| RCB-V0-005 | `TestExportRefusesOutputsItMustNotWrite`, `TestCEMExportBundleVerifiesOffline`, `TestReadOnlyVerbsWriteNothing` |
| RCB-V0-006 | `script/verify-receipt-bundle_test.sh`, `TestCEMExportBundleVerifiesOffline` |
| RCB-V0-007 | structural: only `cmd/corvint/cem_export.go` imports `internal/receiptbundle`, and the package imports no network, signing, or ledger code |

## Rollout, rollback, and drift

Rollout is this change. To roll back, remove `internal/receiptbundle`, `cmd/corvint/cem_export.go`,
the `export` entry in `cemActions` and `cemActionOrder`, the help lines, and the verifier script
and its Makefile target. Also restore DR-0040 to the eleven-action list and its prior candidate
digest. No persisted state exists to migrate. Bundles already written stay valid for the verifier
that shipped with them. A new receipt kind, or a change to the manifest line shape, needs a new
`profile` value, and this spec and the verifier change in the same commit.
