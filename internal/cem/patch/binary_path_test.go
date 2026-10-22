package patch

import (
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestBinaryRefusalNamesPath pins CEM-V1-037 (V1-0301, decision 0548): every
// binary marker keeps the binary-patch refusal code, and the message names
// the changed path whenever the diff --git line, rename metadata or the
// `Binary files` marker declares one. A marker with no declared path keeps
// the generic message.
func TestBinaryRefusalNamesPath(t *testing.T) {
	cases := []struct{ name, input, want string }{
		{"modify", "diff --git a/img/x.png b/img/x.png\nindex 1111111..2222222 100644\nBinary files a/img/x.png and b/img/x.png differ\n",
			`binary change to "img/x.png" is outside the CEM patch grammar; cem/1.0 cannot represent binary content`},
		{"git-binary-patch", "diff --git a/img/x.png b/img/x.png\nindex 1111111..2222222 100644\nGIT binary patch\nliteral 3\n",
			`binary change to "img/x.png" is outside the CEM patch grammar; cem/1.0 cannot represent binary content`},
		{"rename", "diff --git a/old.bin b/new.bin\nsimilarity index 60%\nrename from old.bin\nrename to new.bin\nindex 1111111..2222222\nBinary files a/old.bin and b/new.bin differ\n",
			`binary change to "new.bin" is outside the CEM patch grammar; cem/1.0 cannot represent binary content`},
		{"bare-creation-marker", "Binary files /dev/null and b/a.bin differ\n",
			`binary change to "a.bin" is outside the CEM patch grammar; cem/1.0 cannot represent binary content`},
		{"bare-deletion-marker", "Binary files a/a.bin and /dev/null differ\n",
			`binary change to "a.bin" is outside the CEM patch grammar; cem/1.0 cannot represent binary content`},
		{"bare-git-binary-patch", "GIT binary patch\nliteral 3\n", "binary patches are outside the CEM patch grammar"},
		{"traversal-marker", "Binary files a/../x and b/../x differ\n", "binary patches are outside the CEM patch grammar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.input))
			if cemcode.CodeOf(err) != cemcode.BinaryPatch {
				t.Fatalf("Parse error = %v, want %s", err, cemcode.BinaryPatch)
			}
			if got := cemcode.MessageOf(err); got != tc.want {
				t.Fatalf("message = %q, want %q", got, tc.want)
			}
		})
	}
}
