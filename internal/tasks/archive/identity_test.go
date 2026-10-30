package archive

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTMV0022_AS09_ContentAddressedNames(t *testing.T) {
	for _, dir := range []string{"evidence", "pinned"} {
		for _, kind := range []string{"valid", "mismatch", "uppercase", "nonhex", "short", "suffix", "nested"} {
			t.Run(dir+"-"+kind, func(t *testing.T) {
				payload := []byte("blob")
				digest := wire.Sum(payload)
				name := string(digest)
				switch kind {
				case "mismatch":
					name = strings.Repeat("a", 64)
				case "uppercase":
					name = strings.ToUpper(name)
				case "nonhex":
					name = strings.Repeat("z", 64)
				case "short":
					name = name[:63]
				case "nested":
					name = "nested/" + name
				}
				if dir == "pinned" {
					name += ".json"
				}
				if kind == "suffix" {
					name += ".bad"
				}
				path := dir + "/" + name
				m, entries := handStore(t, 1, 1)
				m.Files = append(m.Files, FileEntry{Path: path, Sha256: digest, Bytes: wire.SizeOf(uint64(len(payload)))})
				entries = append(entries, entry{path, payload})
				SortFiles(m.Files)
				ordered := make([]entry, 0, len(entries))
				for _, fe := range m.Files {
					for _, e := range entries {
						if e.name == fe.Path {
							ordered = append(ordered, e)
						}
					}
				}
				_, err := Verify(bytes.NewReader(buildStream(t, m, ordered, true)))
				if kind == "valid" {
					if err != nil {
						t.Fatal(err)
					}
				} else if wire.CodeOf(err) != wire.CodeMalformed {
					t.Fatalf("bad name verified: %v", err)
				}
				r, repo := repoWithStore(t)
				if kind == "uppercase" && dir == "evidence" {
					if err := os.Remove(filepath.Join(r.StateDir, "evidence", string(digest))); err != nil {
						t.Fatal(err)
					}
				}
				fixture.Write(t, filepath.Join(r.StateDir, filepath.FromSlash(path)), payload)
				out, _, err := export(t, repo, fixture.TempDirOutside(t))
				if kind == "valid" {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil || len(out) != 0 {
					t.Fatalf("bad source exported %d bytes: %v", len(out), err)
				}
			})
		}
	}
}
