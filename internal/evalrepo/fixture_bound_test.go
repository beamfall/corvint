package evalrepo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
)

func TestTraceFixtureRefusesOverBoundFile(t *testing.T) {
	t.Run("LTA-V0-002 V1-0747 fixture byte bound before allocation", func(t *testing.T) {
		scored := strings.Repeat("a", 40)
		index := &contextindex.Index{CommitRevision: scored}
		path := filepath.Join(t.TempDir(), "fixture.json")
		document := []byte(`{"schema_version":1,"repositories":[{"scored_revision":"` + scored + `","traces":[]}]}`)
		write := func(size int) {
			t.Helper()
			data := append(append([]byte(nil), document...), bytes.Repeat([]byte(" "), size-len(document))...)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		write(trace.MaxTraceStoreBytes + 1)
		if _, err := loadTraceFixture(path, index, nil); err == nil || !strings.Contains(err.Error(), "fixture exceeds") {
			t.Fatalf("bound+1 fixture: %v", err)
		}
		write(trace.MaxTraceStoreBytes)
		if _, err := loadTraceFixture(path, index, nil); err != nil && strings.Contains(err.Error(), "fixture exceeds") {
			t.Fatalf("exact-bound fixture refused by size: %v", err)
		}
	})
}
