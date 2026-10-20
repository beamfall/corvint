package patch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// metadataOnlyCase is one entry of the shared native/portable vector manifest
// interop/cem01-go/testdata/metadata-only/cases.json (V1-0609).
type metadataOnlyCase struct {
	Name         string  `json:"name"`
	Patch        string  `json:"patch"`
	NativeCode   string  `json:"nativeCode"`
	PortableCode string  `json:"portableCode"`
	Kind         *string `json:"kind"`
	Path         *string `json:"path"`
}

func metadataOnlyCases(t *testing.T) (string, []metadataOnlyCase) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "interop", "cem01-go", "testdata", "metadata-only")
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases []metadataOnlyCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Cases) == 0 {
		t.Fatal("metadata-only vector manifest is empty")
	}
	return dir, manifest.Cases
}

// TestMetadataOnlyChangeRefusalVectors pins CEM-V1-300 and CEM-V1-301: the
// headerless Git groups for empty-file creation, empty-file deletion and a
// mode-only change, alone or mixed with a text change, refuse with
// metadata-only-change-unsupported naming the kind and path; near misses keep
// their legacy malformed-patch error.
func TestMetadataOnlyChangeRefusalVectors(t *testing.T) {
	dir, cases := metadataOnlyCases(t)
	refused := 0
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := Parse(readFixture(t, dir, tc.Patch))
			if got := cemcode.CodeOf(err); got != tc.NativeCode {
				t.Fatalf("code = %q (%v), want %q", got, err, tc.NativeCode)
			}
			if tc.NativeCode != cemcode.MetadataOnlyChangeUnsupported {
				if tc.Kind != nil || tc.Path != nil {
					t.Fatal("a legacy case must not name a kind or path")
				}
				return
			}
			if tc.PortableCode != tc.NativeCode {
				t.Fatalf("portable code %q differs from native %q", tc.PortableCode, tc.NativeCode)
			}
			want := fmt.Sprintf("metadata-only change %s %q has no textual hunk", *tc.Kind, *tc.Path)
			if msg := cemcode.MessageOf(err); !strings.HasPrefix(msg, want) {
				t.Fatalf("message = %q, want prefix %q", msg, want)
			}
		})
		if tc.NativeCode == cemcode.MetadataOnlyChangeUnsupported {
			refused++
		}
	}
	if refused < 5 {
		t.Fatalf("only %d metadata-only refusal vectors, want creation, deletion, mode-only and both mixed orders", refused)
	}
}
