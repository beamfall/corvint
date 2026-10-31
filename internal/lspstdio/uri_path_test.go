// SPDX-License-Identifier: Apache-2.0
package lspstdio

import (
	"path/filepath"
	"testing"
)

// V1-2488 item 6: a Windows editor URI names a drive path `/C:/...`; it maps to `C:\...` on
// Windows, keeps its slash elsewhere, and an unclean or backslashed URI path is refused on every
// host instead of passing native path cleaning.
func TestURINativePathKeepsWindowsDrivePaths(t *testing.T) {
	for _, c := range []struct {
		uriPath, goos, want string
	}{
		{"/repo/a.go", "linux", filepath.FromSlash("/repo/a.go")},
		{"/C:/repo/a.go", "windows", filepath.FromSlash("C:/repo/a.go")},
		{"/c:/repo/a.go", "windows", filepath.FromSlash("c:/repo/a.go")},
		{"/C:/repo/a.go", "linux", filepath.FromSlash("/C:/repo/a.go")},
		{"/1:/repo/a.go", "windows", filepath.FromSlash("/1:/repo/a.go")},
	} {
		if got, ok := uriNativePath(c.uriPath, c.goos); !ok || got != c.want {
			t.Errorf("uriNativePath(%q, %s) = %q, %v; want %q", c.uriPath, c.goos, got, ok, c.want)
		}
	}
	for _, refused := range []string{"", "/repo/x/../a.go", "/repo//a.go", "/repo/a.go/", `/repo\a.go`, "/repo/a\x00.go"} {
		for _, goos := range []string{"linux", "windows"} {
			if got, ok := uriNativePath(refused, goos); ok {
				t.Errorf("uriNativePath(%q, %s) admitted as %q", refused, goos, got)
			}
		}
	}
}
