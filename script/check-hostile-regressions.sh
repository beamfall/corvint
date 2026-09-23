#!/bin/sh
# Hostile-input regression matrix for the native Core (SOP-V0-007..009,
# docs/specs/stable-operations-v0.md). Each row runs the named existing Go tests of one package
# by exact `-run` regex under GOTOOLCHAIN=local and reads the verbose result per test: a test
# without a `--- PASS` line is reported NOT_RUN (its file is host-conditional), never counted as
# a pass. Categories the tree has no regression for are printed as NOT_COVERED and are never
# faked. `--list` prints the matrix without running anything. Read-only: the only writes are a
# private temporary file removed on exit.
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)

# category<TAB>package<TAB>space-separated test functions. Keep in step with the acceptance
# matrix of docs/specs/stable-operations-v0.md, which cites each test by file:line.
matrix() {
  cat <<'ROWS'
hostile-repository	./internal/genesis	TestInventoryDoesNotClaimAbsenceOverUnsafePathEntries TestInventoryDoesNotReadUnsafePathBlobs TestSummaryOrdersUnsafePathSamplesWithoutPanicking
hostile-repository	./internal/contextindex	TestParseHistoryRejectsHostileRecords
paths	./internal/releasegate	TestTarArchiveRejectsTraversalAndSymlink
paths	./internal/companionrelease	TestValidateBundleNameRefusesPathTraversal
paths	./internal/trace	TestMigrationPlanRejectsCandidateLinksWithoutReadingOutside
paths	./internal/doccompiler	TestPlanRejectsSymlinkAndPathEscape
symlinks	./internal/contextindex	TestReadCleanFileRejectsSymlinkComponents TestBuildFallsBackToGitWhenTrackedFileBecomesSymlink TestBlobShardReadRefusesInRootLeafSymlink TestBlobShardPublicationPinsDirectoryAcrossSymlinkSwap TestWriteSnapshotRefusesCommittedSymlinkedSnapshotDirectory TestSnapshotReadersMissThroughCommittedSymlinkedSnapshotDirectory
symlinks	./internal/trace	TestStoreRejectsSymlinkAndHardlinkCandidates TestAppendRejectsTemporarySymlinkWithoutOutsideWrite
symlinks	./internal/releasecandidate	TestPUBV0025HostileStore
symlinks	./internal/worktreeimpact	TestReadStableTargetRejectsSymlinkHardlinkAndSpecialFile
case-folds	./internal/companionrelease	TestListTreeEntriesRefusesCaseFoldCollisions TestVerifyTarGzRefusesCaseFoldedMembers
case-folds	./internal/mcp/testvaliditybridge	TestReceiptRefusesCaseFoldedGitDirectory
case-folds	./internal/releasecandidate	TestPUBV0025HostileStore
bounded-output	./internal/procgroup	TestRunProcessTruncateOverflowPolicy TestTruncateCaptureConsumesCrossingWrite
bounded-output	./internal/contextindex	TestBuildWithGitExecutionRejectsTruncatedOutput TestExtractionSamplesAreBounded
bounded-output	./internal/trace	TestPhysicalRowAndStoreBounds
bounded-output	./internal/releasecandidate	TestPUBV0026CandidateInventoryBounds
time	./internal/procgroup	TestRunProcessTimeout TestRunProcessShutdownDeadlineLeavesExitUnobserved
time	./internal/contextindex	TestDogfoodPromptBoundsAndCancellation
interruption-cleanup	./internal/procgroup	TestRunProcessInterruptionLeavesNoDescendant TestRunProcessContextCancellation TestBeforeStopFailureCannotBypassGroupCleanup
interruption-cleanup	./internal/releasecandidate	TestPUBV0026InterruptedInstallReapsDescendant TestPUBV0026CancelledInstallRetainsNothing
interruption-cleanup	./internal/contextindex	TestBuildCancellationKillsGitDescendants
interruption-cleanup	./internal/trace	TestAppendRecoversInterruptedRetention
secret-screening	./internal/secretscreen	TestSecretPatternParityCorpus TestScreenRedactsAWSSecretAdjacentToItsKeyID
secret-screening	./internal/trace	TestLTAV0004RecordRefusesWriterOnlySecretShapes
secret-screening	./internal/contextindex	TestSecretPatternMatchesHistorySecretShapes
corrupted-derived-state	./internal/contextindex	TestPackSnapshotRefusesCorruptionTruncationAndOutOfRangeAsAMiss TestBlobShardCorruptionRefusesAccelerationAndReadFallsBack TestSnapshotRefusesTermTableOffsetsOutsideTheirSlices TestAnalyzerPackProbeMissesACorruptBody
memory	./internal/contextindex	TestBuildAllocationStaysBoundedOverAnOversizedTrackedSource
case-folds-context-index	./internal/contextindex	TestBuildPinsEachCaseFoldedTrackedPathToItsOwnBlob
ROWS
}

# category<TAB>reason for the categories no tracked test exercises.
not_covered() {
  cat <<'ROWS'
memory-resident	no regression bounds whole-process resident memory or git child memory; the memory row bounds the Go heap one index build allocates
ROWS
}

print_not_covered() {
  not_covered | while IFS='	' read -r category reason; do
    printf 'category=%s status=NOT_COVERED reason=%s\n' "$category" "$reason"
  done
}

if test "${1:-}" = --list; then
  matrix | while IFS='	' read -r category package tests; do
    printf 'category=%s package=%s tests=%s\n' "$category" "$package" "$(printf '%s' "$tests" | tr ' ' ',')"
  done
  print_not_covered
  exit 0
fi
test "$#" -eq 0 || { echo "usage: script/check-hostile-regressions.sh [--list]" >&2; exit 2; }

log=$(mktemp "${TMPDIR:-/tmp}/corvint-hostile-regressions.XXXXXX")
trap 'rm -f "$log" "$log.failed"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

matrix | while IFS='	' read -r category package tests; do
  pattern="^($(printf '%s' "$tests" | tr ' ' '|'))\$"
  go_status=0
  (cd "$root" && GOTOOLCHAIN=local go test -count=1 -v -run "$pattern" "$package") > "$log" 2>&1 || go_status=$?
  passed=0
  not_run=0
  failed_names=""
  for name in $tests; do
    if grep -q "^--- PASS: $name " "$log"; then
      passed=$((passed + 1))
    elif grep -q "^--- FAIL: $name " "$log"; then
      failed_names="${failed_names:+$failed_names,}$name"
    else
      not_run=$((not_run + 1))
    fi
  done
  status=PASS
  if test "$go_status" -ne 0 || test -n "$failed_names"; then
    status=FAIL
  elif test "$passed" -eq 0; then
    status=NOT_RUN
  fi
  printf 'category=%s package=%s tests=%s pass=%s not_run=%s status=%s%s\n' \
    "$category" "$package" "$(printf '%s' "$tests" | wc -w | tr -d ' ')" "$passed" "$not_run" "$status" "${failed_names:+ failed=$failed_names}"
  if test "$status" = FAIL; then
    sed 's/^/  /' "$log"
    echo "category=$category status=FAIL" >> "$log.failed"
  fi
done
print_not_covered
if test -e "$log.failed"; then
  rm -f "$log.failed"
  echo "check-hostile-regressions: FAIL" >&2
  exit 1
fi
echo "check-hostile-regressions: PASS"
