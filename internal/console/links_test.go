package console

import (
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const linkFixtureSpec = "# Fixture V0\n\n## Requirements\n\n" +
	"- `FX-V0-001`: Linked.\n- `FX-V0-002`: Uncited.\n- `FX-V0-003`: Absent.\n\n" +
	"## Traceability\n\n| Requirement | Implementation | Test |\n|---|---|---|\n" +
	"| FX-V0-001 | `src/linked.go` | none |\n| FX-V0-003 | `src/absent.go` | none |\n"

// linkFixture commits a one-spec repository and returns its root and commit.
func linkFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"docs/specs/fixture-v0.md": linkFixtureSpec,
		"docs/specs/REQUIREMENTS.tsv": "id\tfile\tline\ttitle\n" +
			"FX-V0-001\tdocs/specs/fixture-v0.md\t5\tLinked.\n" +
			"FX-V0-002\tdocs/specs/fixture-v0.md\t6\tUncited.\n" +
			"FX-V0-003\tdocs/specs/fixture-v0.md\t7\tAbsent.\n",
		"docs/specs/INDEX.json": `[{"path":"docs/specs/fixture-v0.md","title":"Fixture V0","reqPrefix":"FX-V0","intent":"fixture","delivery":"fixture"}]`,
		"src/linked.go":         "package linked\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var head string
	for _, args := range [][]string{{"init", "-q"}, {"add", "."},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-q", "--no-gpg-sign", "-m", "fixture"},
		{"rev-parse", "HEAD"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = gitEnvironment()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
		head = strings.TrimSpace(string(out))
	}
	return root, head
}

var fixtureHref = regexp.MustCompile(`href="(/(?:code|requirement)\?[^"]*)"`)

func pageLinks(body string) []string {
	var links []string
	for _, match := range fixtureHref.FindAllStringSubmatch(body, -1) {
		links = append(links, html.UnescapeString(match[1]))
	}
	return links
}

// TestConsoleRequirementCodeLinks covers LAC-V0-030: a requirement links to
// the code its Traceability row cites and that code links back, both pinned
// to the page's commit; an uncited requirement and an absent path are gaps.
func TestConsoleRequirementCodeLinks(t *testing.T) {
	root, head := linkFixture(t)
	server, err := New(Options{Addr: "127.0.0.1:0", Repo: root, Binary: "atm"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("a cited file is linked at the commit and links back", func(t *testing.T) {
		links := pageLinks(get(t, server, "/requirement?id=FX-V0-001"))
		if len(links) != 1 || !strings.Contains(links[0], "at="+head) {
			t.Fatalf("requirement links = %v, want one link pinned to %s", links, head)
		}
		code := get(t, server, links[0])
		if !strings.Contains(code, "package linked") {
			t.Fatal("the linked code page did not render the cited file")
		}
		back := pageLinks(code)
		if len(back) != 1 || !strings.Contains(back[0], "id=FX-V0-001") || !strings.Contains(back[0], "at="+head) {
			t.Fatalf("code backlinks = %v, want FX-V0-001 pinned to %s", back, head)
		}
		if !strings.Contains(get(t, server, back[0]), "Linked.") {
			t.Error("the backlink target did not render its clause")
		}
	})
	t.Run("an uncited requirement and an absent path are gaps, not links", func(t *testing.T) {
		for _, id := range []string{"FX-V0-002", "FX-V0-003"} {
			body := get(t, server, "/requirement?id="+id)
			if links := pageLinks(body); len(links) != 0 {
				t.Errorf("%s rendered links %v for missing evidence", id, links)
			}
			if !strings.Contains(body, `class="unknown">gap: `) {
				t.Errorf("%s rendered no explicit gap", id)
			}
		}
	})
	t.Run("an over-bound Traceability expansion is a gap, not a reading", func(t *testing.T) {
		big, _ := linkFixture(t)
		row := "| " + strings.Repeat("FX-V0-000..999 ", maxTraceabilityCitations/1000+1) + "| `src/linked.go` | none |\n"
		spec := filepath.Join(big, "docs", "specs", "fixture-v0.md")
		if err := os.WriteFile(spec, []byte(linkFixtureSpec+row), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"add", "."},
			{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-q", "--no-gpg-sign", "-m", "expand"}} {
			cmd := exec.Command("git", append([]string{"-C", big}, args...)...)
			cmd.Env = gitEnvironment()
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %s %v", args, out, err)
			}
		}
		bigServer, err := New(Options{Addr: "127.0.0.1:0", Repo: big, Binary: "atm"})
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range []string{"/requirement?id=FX-V0-001", "/code?path=src/linked.go"} {
			body := get(t, bigServer, target)
			if links := pageLinks(body); len(links) != 0 {
				t.Errorf("%s rendered links %v from an over-bound table", target, links)
			}
			if !strings.Contains(body, "requirement citations, so no link is read from them") {
				t.Errorf("%s did not name the bound as a gap", target)
			}
		}
	})
	t.Run("a link pinned to another commit renders nothing", func(t *testing.T) {
		body := get(t, server, "/code?path=src/linked.go&at=0000000000000000000000000000000000000000")
		if strings.Contains(body, "package linked") || !strings.Contains(body, "is pinned to commit") {
			t.Error("a stale pinned link rendered content from the current commit")
		}
	})
}
