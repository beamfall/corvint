package dogfoodflow

import (
	"strings"
	"testing"
)

// TestDCWV0720RecordHunkSharesItsTestSpan (V1-4131): a build-log hunk that
// cites the same span as the change's one test hunk is not a uniform plan,
// while two non-record hunks citing that span beside it still are.
func TestDCWV0720RecordHunkSharesItsTestSpan(t *testing.T) {
	log := map[string]string{"docs/build-log/2026-11-01-lane.md": "# lane\n\nRecords the change.\n"}
	t.Run("one test hunk and its build log", func(t *testing.T) {
		root, base := preflightRepository(t, nil)
		// Reduce the change to one pre.go hunk plus the build-log entry.
		content := testGit(t, root, "show", base+":pre.go") + "\n"
		changed := strings.Replace(content, "\t// first line 6\n", "\t// first line 6 changed\n", 1)
		files := map[string]string{"pre.go": changed}
		for name, text := range log {
			files[name] = text
		}
		testGit(t, root, "checkout", "-q", "-b", "one-hunk", base)
		writeTree(t, root, files)
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-q", "-m", "change")
		plan := writeInput(t, "1\tpre.go\t3:3\tdependency\n2\tpre.go\t3:3\timplementation\n")
		code, stdout, stderr, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan})
		if code != 0 || strings.Contains(stdout, "citation-plan-uniform") || !strings.Contains(stdout, "inputs-check PASS") {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
		}
	})
	t.Run("two code hunks and a build log", func(t *testing.T) {
		root, base := preflightRepository(t, log)
		plan := writeInput(t, "1\tpre.go\t3:3\tdependency\n2\tpre.go\t3:3\timplementation\n3\tpre.go\t3:3\timplementation\n")
		code, stdout, stderr, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan})
		if code == 0 || !strings.Contains(stdout, "citation-plan-uniform") || !strings.Contains(stderr, " hunks=2 span=pre.go:3:3") {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
		}
	})
}

// TestDCWV0720UniformCitationSkipsRecordHunks pins the map-level rule.
func TestDCWV0720UniformCitationSkipsRecordHunks(t *testing.T) {
	hunks := []map[string]string{{"id": "a", "path": "docs/build-log/x.md"}, {"id": "b", "path": "x_test.go"}}
	plan := []byte("1\tx.go\t3:3\tdependency\n2\tx.go\t3:3\ttest\n")
	if _, _, uniform := uniformCitation(plan, hunks); uniform {
		t.Fatal("one non-record hunk is uniform")
	}
	hunks = append(hunks, map[string]string{"id": "c", "path": "x.go"})
	plan = []byte("1\tdocs/specs/x.md\t9:9\tdependency\n2\tx.go\t3:3\ttest\n3\tx.go\t3:3\timplementation\n")
	if count, span, uniform := uniformCitation(plan, hunks); !uniform || count != 2 || span != "x.go:3:3" {
		t.Fatalf("count=%d span=%q uniform=%v", count, span, uniform)
	}
}
