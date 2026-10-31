package patch

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestCEMV1980BinaryGroupGrammar pins CEM-V1-980 and CEM-V1-981 (V1-2752):
// Git's canonical full-index `Binary files` section parses as a hunk-less
// binary group carrying its path, kind, mode and full object IDs, beside a
// textual group whose hunk is unchanged. Every other binary shape keeps the
// CEM-V1-037 binary-patch refusal naming the path, and a binary-only patch
// still refuses because a map needs a textual hunk.
func TestCEMV1980BinaryGroupGrammar(t *testing.T) {
	t.Parallel()
	sha1Old, sha1New := strings.Repeat("a", 40), strings.Repeat("b", 40)
	sha256Old, sha256New := strings.Repeat("c", 64), strings.Repeat("d", 64)
	zero40, zero64 := strings.Repeat("0", 40), strings.Repeat("0", 64)
	text := "diff --git a/t.txt b/t.txt\nindex " + sha1Old + ".." + sha1New + " 100644\n--- a/t.txt\n+++ b/t.txt\n@@ -1 +1 @@\n-a\n+b\n"
	textOnly, err := Parse([]byte(text))
	if err != nil {
		t.Fatalf("text-only Parse: %v", err)
	}
	accepted := []struct {
		name, section        string
		kind                 GroupKind
		mode, oldOID, newOID string
	}{
		{"modify-sha1", "diff --git a/img/x.png b/img/x.png\nindex " + sha1Old + ".." + sha1New + " 100644\nBinary files a/img/x.png and b/img/x.png differ\n",
			KindModify, "100644", sha1Old, sha1New},
		{"modify-sha256-executable", "diff --git a/img/x.png b/img/x.png\nindex " + sha256Old + ".." + sha256New + " 100755\nBinary files a/img/x.png and b/img/x.png differ\n",
			KindModify, "100755", sha256Old, sha256New},
		{"create", "diff --git a/img/x.png b/img/x.png\nnew file mode 100644\nindex " + zero40 + ".." + sha1New + "\nBinary files /dev/null and b/img/x.png differ\n",
			KindCreate, "100644", "", sha1New},
		{"delete-sha256", "diff --git a/img/x.png b/img/x.png\ndeleted file mode 100755\nindex " + sha256Old + ".." + zero64 + "\nBinary files a/img/x.png and /dev/null differ\n",
			KindDelete, "100755", sha256Old, ""},
	}
	for _, tc := range accepted {
		for _, order := range []string{"binary-first", "binary-last"} {
			t.Run(tc.name+"/"+order, func(t *testing.T) {
				input := tc.section + text
				if order == "binary-last" {
					input = text + tc.section
				}
				parsed, err := Parse([]byte(input))
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				if len(parsed.Groups) != 2 || len(parsed.Binary) != 1 || len(parsed.Hunks) != 1 {
					t.Fatalf("groups=%d binary=%d hunks=%d, want 2/1/1", len(parsed.Groups), len(parsed.Binary), len(parsed.Hunks))
				}
				if parsed.Hunks[0].ID != textOnly.Hunks[0].ID {
					t.Fatalf("textual hunk ID %s changed from %s", parsed.Hunks[0].ID, textOnly.Hunks[0].ID)
				}
				group := parsed.Binary[0]
				if !group.Binary || len(group.Hunks) != 0 || group.Kind != tc.kind || group.Mode != tc.mode ||
					group.OldOID != tc.oldOID || group.NewOID != tc.newOID {
					t.Fatalf("group = %+v", group)
				}
				if got := binaryGroupPath(group); got != "img/x.png" {
					t.Fatalf("path = %q", got)
				}
				if (group.OldPath == nil) != (tc.kind == KindCreate) || (group.NewPath == nil) != (tc.kind == KindDelete) {
					t.Fatalf("paths old=%v new=%v for kind %v", group.OldPath, group.NewPath, tc.kind)
				}
			})
		}
	}

	refused := []struct{ name, section string }{
		{"abbreviated-ids", "diff --git a/img/x.png b/img/x.png\nindex 1111111..2222222 100644\nBinary files a/img/x.png and b/img/x.png differ\n"},
		{"mixed-id-lengths", "diff --git a/img/x.png b/img/x.png\nindex " + sha1Old + ".." + sha256New + " 100644\nBinary files a/img/x.png and b/img/x.png differ\n"},
		{"no-index", "diff --git a/img/x.png b/img/x.png\nBinary files a/img/x.png and b/img/x.png differ\n"},
		{"modify-without-mode", "diff --git a/img/x.png b/img/x.png\nindex " + sha1Old + ".." + sha1New + "\nBinary files a/img/x.png and b/img/x.png differ\n"},
		{"modify-same-ids", "diff --git a/img/x.png b/img/x.png\nindex " + sha1Old + ".." + sha1Old + " 100644\nBinary files a/img/x.png and b/img/x.png differ\n"},
		{"create-nonzero-old", "diff --git a/img/x.png b/img/x.png\nnew file mode 100644\nindex " + sha1Old + ".." + sha1New + "\nBinary files /dev/null and b/img/x.png differ\n"},
		{"create-with-mode-suffix", "diff --git a/img/x.png b/img/x.png\nnew file mode 100644\nindex " + zero40 + ".." + sha1New + " 100644\nBinary files /dev/null and b/img/x.png differ\n"},
		{"delete-nonzero-new", "diff --git a/img/x.png b/img/x.png\ndeleted file mode 100644\nindex " + sha1Old + ".." + sha1New + "\nBinary files a/img/x.png and /dev/null differ\n"},
		{"modify-marker-says-create", "diff --git a/img/x.png b/img/x.png\nindex " + sha1Old + ".." + sha1New + " 100644\nBinary files /dev/null and b/img/x.png differ\n"},
		{"marker-other-path", "diff --git a/img/x.png b/img/x.png\nindex " + sha1Old + ".." + sha1New + " 100644\nBinary files a/img/y.png and b/img/y.png differ\n"},
		{"rename", "diff --git a/img/old.png b/img/x.png\nsimilarity index 60%\nrename from img/old.png\nrename to img/x.png\nindex " + sha1Old + ".." + sha1New + " 100644\nBinary files a/img/old.png and b/img/x.png differ\n"},
		{"git-binary-patch", "diff --git a/img/x.png b/img/x.png\nindex " + sha1Old + ".." + sha1New + " 100644\nGIT binary patch\nliteral 3\n"},
		{"trailing-line", "diff --git a/img/x.png b/img/x.png\nindex " + sha1Old + ".." + sha1New + " 100644\nBinary files a/img/x.png and b/img/x.png differ\n--- a/img/x.png\n"},
	}
	for _, tc := range refused {
		t.Run("refused/"+tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.section + text))
			if cemcode.CodeOf(err) != cemcode.BinaryPatch {
				t.Fatalf("Parse error = %v, want %s", err, cemcode.BinaryPatch)
			}
			if !strings.Contains(cemcode.MessageOf(err), `binary change to "img/x.png"`) {
				t.Fatalf("message = %q, want it to name img/x.png", cemcode.MessageOf(err))
			}
		})
	}

	// Rename metadata is refused by its presence, not by its value: an empty
	// `rename from ` / `rename to ` line must not let the section through
	// (CEM-V1-980). Copy metadata is outside the grammar and also refuses.
	modify := "index " + sha1Old + ".." + sha1New + " 100644\nBinary files a/img/x.png and b/img/x.png differ\n"
	present := []struct{ name, metadata string }{
		{"empty-rename-from-and-to", "rename from \nrename to \n"},
		{"empty-rename-from", "rename from \n"},
		{"empty-rename-to", "rename to \n"},
		{"similarity", "similarity index 100%\n"},
		{"copy", "copy from img/x.png\ncopy to img/x.png\n"},
		{"empty-copy", "copy from \ncopy to \n"},
	}
	for _, tc := range present {
		t.Run("metadata-present/"+tc.name, func(t *testing.T) {
			parsed, err := Parse([]byte("diff --git a/img/x.png b/img/x.png\n" + tc.metadata + modify + text))
			if err == nil {
				t.Fatalf("Parse accepted a binary section carrying %q metadata: %d binary groups", tc.metadata, len(parsed.Binary))
			}
		})
	}

	t.Run("binary-only", func(t *testing.T) {
		_, err := Parse([]byte(accepted[0].section))
		if cemcode.CodeOf(err) != cemcode.BinaryPatch ||
			!strings.Contains(cemcode.MessageOf(err), `binary change to "img/x.png" is bound by blob identity, but the patch contains no textual hunks`) {
			t.Fatalf("binary-only Parse error = %v", err)
		}
	})
}
