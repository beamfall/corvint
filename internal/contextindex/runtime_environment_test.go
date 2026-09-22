package contextindex

import "testing"

func TestRuntimeEnvironmentCurrentSnapshotSettings(t *testing.T) {
	t.Setenv("CORVINT_SNAPSHOT_FORMAT", "pack")
	if !packEnabled() || sectionedEnabled() {
		t.Fatal("current snapshot selection differs")
	}
	t.Setenv("CORVINT_INDEX_SHARDS", "1")
	if !blobShardsEnabled() {
		t.Fatal("current shard opt-in ignored")
	}
}
