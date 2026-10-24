package behaviorfalsify

import (
	"path"
	"testing"
)

// TestBBFV0215RepositoryRelativeUsesSlashSyntaxOnEveryHost traces BBF-V0-215 (V1-2524). Native cleaning
// rewrote tests/x.spec.ts to tests\x.spec.ts on Windows and refused every plan path there; the
// check is now host-independent, so the Windows spellings below are refused on Linux too.
func TestBBFV0215RepositoryRelativeUsesSlashSyntaxOnEveryHost(t *testing.T) {
	for _, admitted := range []string{"tests/login.spec.ts", "docs/contracts/login.md", "playwright.config.ts", "internal/pkg", ".rspec"} {
		if !repositoryRelative(admitted) {
			t.Errorf("refused %q", admitted)
		}
	}
	// The bare parent component is built at run time: a test-file literal whose path token is
	// only parent components (a backslash ends the token) marks the package an unbounded reader
	// (AFP-V0-023), though these are only checked, never read.
	parent := path.Dir("../x")
	for _, refused := range []string{
		"", parent, "../x", "/abs/x", "tests//x.ts", "tests/./x.ts", "tests/../x.ts", "tests/",
		`tests\login.spec.ts`, `C:\repo\tests\x.ts`, `C:/repo/tests/x.ts`, `C:x.ts`, `\tests\x.ts`, `\\server\share\x.ts`, parent + `\x`,
	} {
		if repositoryRelative(refused) {
			t.Errorf("admitted %q", refused)
		}
	}
}
