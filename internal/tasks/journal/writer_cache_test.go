package journal

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-026: historical requests cannot consume the live-record selection budget.
func TestCALV0026_WriterRequestHistoryKeepsOnlyDigests(t *testing.T) {
	repo, r := setup(t)
	r.writerCache = true
	first, err := r.AuditForWrite()
	if err != nil {
		t.Fatal(err)
	}
	liveBytes := 0
	for _, record := range first.Records {
		liveBytes += len(record.Raw)
	}
	want := map[string]wire.Digest{}
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("history-%d", i)
		path, _ := snapshot.RequestPath(id)
		raw := requestBytes(id, uint64(i+2), wire.Sum([]byte(id)), false)
		want[path] = wire.Sum(raw)
		appendReceipt(t, repo, "MUTATION", map[string][]byte{path: raw}, id, true, true, false)
	}
	result, err := r.audit(nil, "", limits{scan: wire.MaxArchiveScanEntries, selected: liveBytes}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RequestDigests) != len(want) {
		t.Fatalf("request digests: %d, want %d", len(result.RequestDigests), len(want))
	}
	for path, digest := range want {
		if result.RequestDigests[path] != digest || wire.Sum(read(t, filepath.Join(repo.StateDir, path))) != digest {
			t.Fatalf("unbound request digest: %s", path)
		}
	}
	for path := range result.Records {
		if strings.HasPrefix(path, "requests/") {
			t.Fatalf("historical bytes retained: %s", path)
		}
	}
}
