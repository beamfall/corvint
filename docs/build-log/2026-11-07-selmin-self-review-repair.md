# Selmin self-review repair: mutator errors observe source files

Date: 2026-11-07
Requirements: AFP-V0-9010, AFP-V0-2200 (V1-5029)
Base: cb403735903b1e54a8469373489960c0b2b89d99
Repair input: 339961454514d891f1194d022ca4d6862ca80722

The reported P1 is a missed selection: a proven import listing plus `os.Chmod`
on a filename computed from bytes can observe a new test file through
`os.IsNotExist(err)`. If its imports were already present, the folded header
stays unchanged and there is no literal filename edge to rescue the selection.

## Decision and class boundary

Filesystem mutation is an observation channel: returned errors can reveal file
existence, metadata and access. Remove every filesystem mutator from the
unconditional `os` and `io/ioutil` member allowlists, including creation,
temporary creation, directory creation, copying, writes, truncation, metadata
changes and removal. No mutation path is certified in this repair, even for
fixtures or temporary outputs; no confinement proof for those writes exists.
This deliberately accepts wider selection for such readers.

The classifier remains a closed allowlist, not a blacklist of the reported
operations. Only explicit non-observing members (constants, types, environment
reads, path arithmetic and the existing inert APIs), the existing own-testdata
read proof and proven confined go-list selectors are admitted. Unknown members,
including future filesystem operations, remain observing. No new path form or
exception is admitted. The unchanged testdata read proof rejects links and unknown
filesystem entries. No wire shape, field, error/exit code, stderr text, golden or
existing test assertion changes.

## Regression evidence

`TestFilesystemMutatorsObserveSource_AFP_V0_9010` covers mutator references as
function values and calls, including a testdata target, plus unknown members of
both classified packages. These are parser fixtures, including deliberately
unknown APIs, not compiled fixture programs.
`TestMutatorSeesFoldedTestFileAddition_AFP_V0_9010` covers Chmod, Chtimes, Remove
and an unknown operation with a byte-computed `../lib/b_test.go`. Adding that
file with only the existing testing import must leave the folded header unchanged,
mark the probe as a whole-module source reader, and select it by the source
witness. A proven import-only control stays out. Dropping Source loses the probe,
proving no literal fallback masks the bug.

The initial new fixture listed lib directly and its module-path literal supplied
a separate PATH_LITERAL_READER edge; its new dropped-Source assertion failed.
The fixture now lists cmd/tool, which imports lib, preserving the same proven
listing while eliminating the competing literal edge. No assertion was removed
or weakened. The failed run is retained as self-review-golang.log outside Git.

- Whole `internal/liveverify/affected/golang`: PASS, 9.404s; all existing and new
  tests, through heavy.sh. Log SHA-256
  `91b0e9031635e9ae5e497a046bfa20c819e7bb813844df1b6e5655da0a7f4f7f`.
- `make -s spec-requirements-check line-citations-check`: PASS. Repin found no
  moved citation; 180 skipped-basename notices remain. Log SHA-256
  `4592dea39b416ffe25b9c43202e4401696e765cc4611614836c2e7dd977425c2`.
- Whole `internal/liveverify/affected`: PASS, 107.459s, including its unchanged
  selection precision bounds, through heavy.sh. Log SHA-256
  `911be6bc328abdd02e7f0137b424ee20f689a8a8d7df93e1a7e091d18a5d92f2`.
- Go toolchain: go1.27.3. No identity was exported during tests or checks.

## Scope, self-use and limits

Only V1-5029 is repaired; V1-5026 / AFP-V0-9009 and V1-5030 / AFP-V0-9011 are
unchanged. No native ticket completion is claimed because Tasks writes are
prohibited. Existing pre-change capture is reused without recapture. The applicable
self-development routes are affected selection, focused tests and final CEM
binding/checking. Other product routes and repository-wide gates are outside this
repair. Independent review and broader qualification belong to the orchestrator.

The current-source `affected --base cb403735903b1e54a8469373489960c0b2b89d99
--summary` selected both tested packages in tier 0: UNKNOWN, 401 selected,
476 excluded, 68 unknowns (66 LANGUAGE_FRONTIER, 2 UNSELECTED_BINARY_CONSUMER).
Its output SHA-256 is
`7ca8b401aed071a5dbf818af0d05aea9c5fbc7470d9aa482e9f05d2ee7f1c016`.
Raw evidence is under `/home/russell/lanes/selmin-evidence/self-review-*`.

Rollback: revert this repair's code, regressions and spec clarification together;
that restores the reported unsound certification and is not a fix for the P1.
