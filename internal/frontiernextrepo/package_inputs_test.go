package frontiernextrepo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func TestFixedPackageInputSet(t *testing.T) {
	cases := []struct {
		name, path       string
		symlink, allowed bool
	}{
		{"regular-test", "internal/wp3codec/codec_test.go", false, true},
		{"production", "internal/wp3codec/shadow.go", false, false},
		{"assembly", "internal/wp3codec/shadow.s", false, false},
		{"nested", "internal/wp3codec/nested/hidden.go", false, false},
		{"undeclared-data", "internal/wp3codec/config.txt", false, false},
		{"symlink-test", "internal/wp3codec/link_test.go", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			git(t, root, "init", "-q", "-b", "main")
			write(t, root, "internal/wp3codec/codec.go", []byte("package wp3codec\n"))
			if c.symlink {
				err = os.Symlink("codec.go", filepath.Join(root, c.path))
				if err != nil {
					t.Fatal(err)
				}
			} else {
				write(t, root, c.path, []byte("package wp3codec\n"))
			}
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "fixed package input case")
			target := git(t, root, "rev-parse", "HEAD")
			repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
			if err != nil {
				t.Fatal(err)
			}
			if got := completePackage(context.Background(), repo, target, "internal/wp3codec/codec.go"); got != c.allowed {
				t.Fatalf("allowed=%v want %v", got, c.allowed)
			}
		})
	}
}
