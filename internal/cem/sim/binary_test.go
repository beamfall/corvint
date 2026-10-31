package sim

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
)

type modeEntry struct {
	data []byte
	mode string
}

type modeSource map[string]modeEntry

func (m modeSource) BaseBlob(path string) ([]byte, string, bool, error) {
	entry, ok := m[path]
	return entry.data, entry.mode, ok, nil
}

// TestCEMV1982BinaryGroupBaseSimulation pins CEM-V1-982 (V1-2752): a binary
// modify or delete group requires the base blob at its path to be the named
// object with the named mode, and a binary create requires the destination
// to be absent, while the textual groups are simulated as before.
func TestCEMV1982BinaryGroupBaseSimulation(t *testing.T) {
	t.Parallel()
	// Expected values: `git hash-object` and the SHA-256 of "blob 6\x00hello\n".
	if got, want := BlobObjectID([]byte("hello\n"), 40), "ce013625030ba8dba906f756967f9e9ca394464a"; got != want {
		t.Fatalf("SHA-1 blob id = %s, want %s", got, want)
	}
	if got, want := BlobObjectID([]byte("hello\n"), 64), "2cf8d83d9ee29543b34a87727421fdecb7e3f3a183d337639025de576db9ebb4"; got != want {
		t.Fatalf("SHA-256 blob id = %s, want %s", got, want)
	}
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	if BlobObjectID(png, 12) != "" {
		t.Fatal("an unsupported width must not produce an object id")
	}
	sha1Png, sha256Png := BlobObjectID(png, 40), BlobObjectID(png, 64)
	other40, other64 := strings.Repeat("e", 40), strings.Repeat("f", 64)
	zero40 := strings.Repeat("0", 40)
	text := "diff --git a/t.txt b/t.txt\nindex " + other40 + ".." + strings.Repeat("1", 40) + " 100644\n--- a/t.txt\n+++ b/t.txt\n@@ -1 +1 @@\n-a\n+b\n"
	section := func(header, marker string) string {
		return "diff --git a/x.png b/x.png\n" + header + "Binary files " + marker + " differ\n"
	}
	modify := func(oldOID, newOID, mode string) string {
		return section("index "+oldOID+".."+newOID+" "+mode+"\n", "a/x.png and b/x.png")
	}
	deleteOf := func(oldOID string) string {
		return section("deleted file mode 100644\nindex "+oldOID+".."+zero40+"\n", "a/x.png and /dev/null")
	}
	create := section("new file mode 100644\nindex "+zero40+".."+sha1Png+"\n", "/dev/null and b/x.png")
	cases := []struct {
		name, binary string
		baseMode     string // mode of x.png in the base tree; "" means absent
		code         string
		message      string
	}{
		{"modify-sha1", modify(sha1Png, other40, "100644"), "100644", "", ""},
		{"modify-sha256-executable", modify(sha256Png, other64, "100755"), "100755", "", ""},
		{"delete", deleteOf(sha1Png), "100644", "", ""},
		{"create", create, "", "", ""},
		{"modify-wrong-object", modify(other40, sha1Png, "100644"), "100644", cemcode.BaseMismatch, `base blob of "x.png" is not object ` + other40},
		{"modify-absent", modify(sha1Png, other40, "100644"), "", cemcode.BaseMismatch, `"x.png"`},
		{"modify-mode-differs", modify(sha1Png, other40, "100755"), "100644", cemcode.DiffMetadataMismatch, `"x.png"`},
		{"delete-wrong-object", deleteOf(other40), "100644", cemcode.BaseMismatch, `base blob of "x.png" is not object`},
		{"create-destination-exists", create, "100644", cemcode.BaseMismatch, `destination "x.png" already exists`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := patch.Parse([]byte(tc.binary + text))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			source := modeSource{"t.txt": {[]byte("a\n"), "100644"}}
			if tc.baseMode != "" {
				source["x.png"] = modeEntry{png, tc.baseMode}
			}
			err = Simulate(parsed, source)
			if tc.code == "" {
				if err != nil {
					t.Fatalf("Simulate: %v", err)
				}
				return
			}
			if cemcode.CodeOf(err) != tc.code || !strings.Contains(cemcode.MessageOf(err), tc.message) {
				t.Fatalf("Simulate error = %v, want %s containing %q", err, tc.code, tc.message)
			}
		})
	}
}
