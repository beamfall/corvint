// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// LPCV-V0-954, LPCV-V0-7600: a symlinked root and inherited PWD must not
// make the Go build silently omit the test-window overlay (V1-4701).
func TestCollectSymlinkedRootPWD(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":          fixture["go.mod"],
		"ord/ord.go":      fixture["ord/ord.go"],
		"ord/ord_test.go": fixture["ord/ord_test.go"],
	})
	linked := filepath.Join(t.TempDir(), "checkout")
	if err := os.Symlink(root, linked); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, pwd string }{
		{"matching-symlink", linked},
		{"stale", t.TempDir()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PWD", tc.pwd)
			out, err := Collect(context.Background(), linked, CollectOptions{
				Module: "example.com/m", Packages: []string{"ord"}, Work: t.TempDir(),
				Timeout: time.Minute, SuiteTimeout: time.Minute,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(out) != 3 {
				t.Fatalf("got %d outcomes, want TestA, TestB and InOrder", len(out))
			}
			for _, o := range out {
				if o.Record == nil || o.Divergent != "" || o.WindowFallback != "" {
					t.Fatalf("outcome %+v", o)
				}
			}
		})
	}
}
