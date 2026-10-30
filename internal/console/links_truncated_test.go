package console

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestConsoleTruncatedSpecIsPartialNotAbsent covers LAC-V0-030 for a spec
// blob read only to maxBlobBytes (V1-3032): a Traceability row past the bound
// is not read, so the code and requirement pages report a partial read as a
// gap instead of an absent link. An untruncated spec keeps its links.
func TestConsoleTruncatedSpecIsPartialNotAbsent(t *testing.T) {
	big, _ := linkFixture(t)
	// Prose ahead of the table pushes the Traceability section past the read bound.
	padding := strings.Repeat("Padding prose that is not a Traceability row.\n", maxBlobBytes/40)
	spec := strings.Replace(linkFixtureSpec, "## Traceability", padding+"\n## Traceability", 1)
	if err := os.WriteFile(filepath.Join(big, "docs", "specs", "fixture-v0.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-q", "--no-gpg-sign", "-m", "pad"}} {
		cmd := exec.Command("git", append([]string{"-C", big}, args...)...)
		cmd.Env = gitEnvironment()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	server, err := New(Options{Addr: "127.0.0.1:0", Repo: big, Binary: "atm"})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/code?path=src/linked.go", "/requirement?id=FX-V0-001"} {
		body := get(t, server, target)
		if strings.Contains(body, "that is an absent link") {
			t.Errorf("%s reported a row past the read bound as absent", target)
		}
		if !strings.Contains(body, "read only its first") || !strings.Contains(body, `class="unknown">gap: `) {
			t.Errorf("%s did not report the truncated spec as a partial read", target)
		}
	}

	root, head := linkFixture(t)
	plain, err := New(Options{Addr: "127.0.0.1:0", Repo: root, Binary: "atm"})
	if err != nil {
		t.Fatal(err)
	}
	if links := pageLinks(get(t, plain, "/code?path=src/linked.go")); len(links) != 1 || !strings.Contains(links[0], "at="+head) {
		t.Fatalf("an untruncated spec's backlinks = %v", links)
	}
	if body := get(t, plain, "/code?path=src/linked.go"); strings.Contains(body, "read only its first") {
		t.Fatal("an untruncated spec was reported as partial")
	}
}
