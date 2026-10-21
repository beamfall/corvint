package contextindex

import "testing"

// The external bridge release test (bridge_release_test.go) observes the
// process's live snapshot mappings through these test-only seams; they are not
// part of the package API (MCPV0-090, V1-1004).

func LiveSnapshotMappingsForTest() int64 { return liveSnapshotMappings.Load() }

func ResetSnapshotRetentionForTest() { resetPackRetention() }

// PackFixtureRootForTest builds the pack fixture, under CORVINT_SNAPSHOT_FORMAT=pack
// for the rest of the test, and returns its repository root.
func PackFixtureRootForTest(t *testing.T) string {
	index, _ := packFixture(t)
	return index.Root
}
