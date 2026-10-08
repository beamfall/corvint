# 2026-10-07: know-how symbol anchors and explicit re-confirm (V1-0963)

## Intent

V1-0963 follows up V1-0955 / GitHub issue beamfall/corvint#655. A whole-file anchor goes STALE on
any edit of its file, so a note about one function decays when a neighbour changes, and the only
refresh was a superseding ADD that restates the text. The slice adds symbol anchors whose freshness
follows one declaration, and an explicit write that re-pins a STALE note with provenance. The
requirements are KHN-V0-016..020 in `docs/specs/corvint-tasks-know-how-notes-v0.md`, proposed
pending owner acceptance; delivery is experimental.

## Decisions

- **Extraction reuse.** `contextindex.SymbolExtents` exposes the extents the index's own
  extractors already compute (go/parser for Go, the Python and the other language extractors for
  the rest). No parser or language is added. A Go method is `Receiver.Method` because the index's
  bare method name is ambiguous. The one new Tasks-to-Core edge is the single file
  `internal/tasks/store/know_how_symbols.go`, recorded as the V1-0963 addendum to decision 0397
  and enforced by `TestImportViolationControls`. No decision file is added.
- **Digest.** The pin is the SHA-256 of the declaration's extent text, doc comment included.
  Moving the declaration or editing a neighbour keeps it CURRENT; any byte inside it makes it STALE.
- **Unresolvable is UNKNOWN.** A missing, renamed, duplicated or unsupported symbol, a deleted
  file, a blob over 1 MiB, or any read failure reads UNKNOWN and is refused `KNOWHOW_UNRESOLVED:`
  (MALFORMED) at pin time.
- **Fast path.** An equal blob is CURRENT without reading content. Only changed blobs of symbol
  anchors are read, in one `git cat-file --batch` after the existing `--batch-check`.
- **Re-confirm refuses, never no-ops.** `KNOWHOW_RECONFIRM` appends a RECONFIRM entry with the
  re-pinned anchors, commit, actor, time, attempt and generation. A re-pin that changes no pin is
  refused with the detail prefix `KNOWHOW_NOT_STALE:` (VALIDATION_FAILED/MALFORMED) in Apply, the
  Tasks codec and the Core reader, through one shared rule in `internal/tasks/wire`. Refusing
  keeps uninformative entries out of the 32-entry cap, tells the caller its premise is wrong, and
  adds no §11 code. Reads overlay the latest RECONFIRM's pins; the ADD's pins and every earlier
  RECONFIRM stay in `ticket show`.
- **Unchanged contracts.** The caps (1024 bytes, 4 anchors, 32 entries), the secret screen (code and
  prefix as V1-0964 sets them), the role grants (OWNER, OPERATOR by policy row; no WORKER) and the read-only `list` and
  claim delivery are unchanged. Symbol names are screened like paths.
- **Compatibility.** A file anchor keeps `{blob, path}` and a legacy ledger re-encodes
  byte-identically. An older binary refuses the new keys closed.
- **Merged with V1-0964.** This lane merges `claude/v1-0964-knowhow-hardening` (546b2b4e), which
  owns KHN-V0-008..015, so these requirements were renumbered from KHN-V0-008..012 to
  KHN-V0-016..020. Secret hits now carry V1-0964's `SECRET_DETECTED` code. A reconfirm naming an
  attempt or a generation goes through the same `CheckKnowHowProvenance` check as ADD, against the
  audited attempt inventory, and so never takes the writer fast route. `KNOWHOW_NOT_STALE:` stays a
  detail on VALIDATION_FAILED/MALFORMED. `KNOWHOW_UNRESOLVED:` stays a detail on MALFORMED.

## Evidence

- Focused tests: `internal/tasks`, `wire`, `ticket`, `mutation`, `intent`, `transaction`,
  `store`, `cli` and `internal/taskman` each run once; `internal/contextindex` with
  `-run 'KHN|SymbolExtent|Symbol'`. The test names are in the spec's traceability table.
- Freshness latency for 32 notes x 4 anchors over 32 changed Go files of 40 functions
  (`-benchtime 50x -count 3`, host load average about 50 on 12 cores, so the runs are noisy):
  - before, file anchors: 76.7, 398.0 and 76.2 ms/op;
  - after, file anchors: 350.1, 278.6 and 121.8 ms/op;
  - after, symbol anchors (every blob changed, every declaration re-extracted): 130.9, 286.2 and
    653.0 ms/op.
  The file-anchor path issues the same single `--batch-check` as before; the difference is
  within this host's load noise. The symbol path adds one `--batch` read and a parse per changed
  file.

## Not done

- Owner acceptance of KHN-V0-016..020.
- A durable qualification on a non-Go repository; concurrent two-process CAS and an
  interrupted-commit redo of a reconfirm receipt; an older released binary refusing a
  RECONFIRM-bearing store.
- Any WORKER grant (V1-0987).
