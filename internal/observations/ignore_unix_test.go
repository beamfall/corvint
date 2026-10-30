//go:build darwin || linux

package observations

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// SOL-V0-001: an ignore file decides coverage only when it is a regular file,
// reached inside the repository, and within maxIgnoreBytes. Any other input,
// including a FIFO, skips the observation without blocking and leaves no
// ledger or temporary.
func TestAppendReadsOnlyBoundedRegularIgnoreFilesInsideTheRepository(t *testing.T) {
	covering := "/.corvint/self-observations.jsonl\n/.corvint/.self-observations.*\n"
	must := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		name        string
		wantIgnored bool
		setup       func(t *testing.T, root, outside string)
	}{
		{"root ignore at the cap", true, func(t *testing.T, root, _ string) {
			must(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte(covering+strings.Repeat("#", maxIgnoreBytes-len(covering))), 0o600))
		}},
		{"oversized root ignore", false, func(t *testing.T, root, _ string) {
			must(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte(covering+strings.Repeat("#", maxIgnoreBytes)), 0o600))
		}},
		{"symlinked root ignore", false, func(t *testing.T, root, outside string) {
			must(t, os.WriteFile(filepath.Join(outside, "ignore"), []byte(covering), 0o600))
			must(t, os.Symlink(filepath.Join(outside, "ignore"), filepath.Join(root, ".gitignore")))
		}},
		{"FIFO root ignore", false, func(t *testing.T, root, _ string) {
			must(t, syscall.Mkfifo(filepath.Join(root, ".gitignore"), 0o600))
		}},
		{"ignore under a symlinked .corvint", false, func(t *testing.T, root, outside string) {
			must(t, os.WriteFile(filepath.Join(outside, ".gitignore"), []byte("*\n"), 0o600))
			must(t, os.Symlink(outside, filepath.Join(root, ".corvint")))
		}},
		{"FIFO .corvint ignore", false, func(t *testing.T, root, _ string) {
			must(t, os.Mkdir(filepath.Join(root, ".corvint"), 0o700))
			must(t, syscall.Mkfifo(filepath.Join(root, ".corvint", ".gitignore"), 0o600))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			test.setup(t, root, outside)
			decided := make(chan bool, 1)
			go func() { decided <- ledgerIgnored(root) }()
			select {
			case ignored := <-decided:
				if ignored != test.wantIgnored {
					t.Fatalf("ignored = %v, want %v", ignored, test.wantIgnored)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("ignore read blocked")
			}
			err := Append(root, Event{Kind: "event"})
			written, _ := filepath.Glob(filepath.Join(root, ".corvint", "*self-observations*"))
			if test.wantIgnored && (err != nil || len(written) != 1) {
				t.Fatalf("Append() = %v, wrote %v", err, written)
			}
			if !test.wantIgnored && (err == nil || len(written) != 0) {
				t.Fatalf("Append() = %v, wrote %v", err, written)
			}
		})
	}
}
