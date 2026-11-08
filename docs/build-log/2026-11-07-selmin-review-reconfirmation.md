# Selmin independent review findings after merge-forward

Date: 2026-11-07
Requirements: AFP-V0-9010, AFP-V0-2200 (V1-5029)
Base: c3b29cbb54d31c8526d3cce4ad74356b6fb9b943
Review input: 6a6ebafc29b674767b947117811ca56a5ad0c367

The supplied review repeats the computed command-directory and existing testdata
symlink findings from before the path-origin repair. Both fixes and their
regressions are already in this input: e901b396a introduced the origin proofs,
and 699b0a903 preserved conservative directory reads without inventing an
execution edge. The merge-forward retained them. No further implementation or
contract change is needed for these findings.

The directory proof is a closed allowlist: unchanged directory literals, fresh
testing TempDir calls on confined testing parameters, and an unescaped,
single-definition module-root derivation from runtime.Caller(0) through
filepath.Dir/Clean/Join, only when target resolution agrees. Computed byte
directories and unknown environment/function origins remain unproven.

The fixture proof admits literal paths or literal joins without climbing or
ambiguous separators, then checks unfollowed entries in the selected Source.
Only regular files and directories are certified; missing entries after checked
ancestors are harmless. Symlinks, unknown entry modes and lookup errors fail
closed. Source.ReadDir supplies unfollowed modes even when an FS Stat would
follow links. Neither proof enumerates only the reported bad forms.

Existing regressions require conservative classification and reader selection:
TestCommandDirectoryFallsBack_AFP_V0_9010 covers the reported computed ../lib
and unknown environment origin on import and body edits;
TestExistingTestdataSymlinkFallsBack_AFP_V0_9010 covers directory, root and file
aliases on body edits. TestTestdataUnknownEntryFailsClosed_AFP_V0_9010 covers
unknown modes and lookup errors. Proven controls remain unchanged. The earlier
path-repair entry retains the mutation evidence; it is not a new mutation run.

Current-source affected selection against the stated base reports UNKNOWN:
414 selected, 478 excluded, 66 LANGUAGE_FRONTIER and 2 UNSELECTED_BINARY_CONSUMER
unknowns. The Go plugin, parent and languages packages are all tier 0. Fresh
whole-package verification results and digests follow below. Raw evidence stays
outside the repository in /home/russell/lanes/selmin-evidence/review2-*.

No wire, golden, assertion, error code, exit code or stderr contract changed.
V1-5026 / AFP-V0-9009 and V1-5030 / AFP-V0-9011 retain their existing delivery;
this review requests no new behavior for them. The pre-change capture is reused.
Broad gates, model-backed review, publication, Tasks writes and sealing remain
excluded by the owner's repair instructions. The final handback records the
single CEM finish cycle and every retained NOT_PRODUCED result. Independent
review and host qualification remain orchestrator-owned. Rollback of this entry
changes only evidence; the existing implementation fixes remain intact.

## Fresh verification

- Whole golang package: PASS (8.198s); log SHA-256
  `60374e2474106df640a7c6a1d2aa565c930bb444280d1fdc38e1af0da6641908`.
- Whole parent package: PASS (106.355s); log SHA-256
  `1ec67c12d6368abd0e785e6e6886c2ab5bc84912ff01a41338aa9936a889a159`.
- Whole languages package: PASS (0.045s); log SHA-256
  `dfa67fa312ad292739486ce3333b7df55c5c99175f71a91f812f7e39731dcc32`.

Go toolchain: go1.27.3. Each package ran separately through script/heavy.sh
with the required TMPDIR, no identity export, no test filter and no lock override.
The parent run is Linux evidence only; its Darwin-only files were not built.
