package witnesspaths

import (
	"testing"

	"github.com/Beamfall/corvint/internal/sharedpath"
)

// TestMatchAgreesWithSharedpath pins the local glob matcher to
// sharedpath.Match, which it copies so this package stays stdlib-only; a
// test-only import is not a dependency of the CEM seams.
func TestMatchAgreesWithSharedpath(t *testing.T) {
	globs := []string{"**", "docs/**", "**/*.md", "docs/*.md", "a/**/b", "a/**/**/b", "*", "*/x", "src/[ab]*.go", "a?c/d", "locale/**/*.json", "", "a/", "[", "x/**/"}
	names := []string{"", "a", "a/b", "a/x/b", "a/x/y/b", "docs", "docs/guide.md", "docs/sub/guide.md", "README.md", "src/a1.go", "src/c1.go", "abc/d", "locale/en/app.json", "locale/app.json", "x/y", "x/", "a/b/"}
	matched := 0
	for _, glob := range globs {
		for _, name := range names {
			got, want := match(glob, name), sharedpath.Match(glob, name)
			if got != want {
				t.Errorf("match(%q, %q) = %v, sharedpath.Match = %v", glob, name, got, want)
			}
			if got {
				matched++
			}
		}
	}
	if matched == 0 {
		t.Fatal("the table matched nothing")
	}
}
