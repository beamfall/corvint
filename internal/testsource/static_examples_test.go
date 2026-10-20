package testsource

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

func titlePaths(examples map[int]testvalidity.ExampleSource) map[int][]string {
	out := map[int][]string{}
	for line, source := range examples {
		out[line] = source.TitlePath
	}
	return out
}

// LPCV-V0-147: the JS/TS reader records an example's literal title path, outermost describe
// first; a template title, an each-generated title or a non-literal ancestor records none.
func TestLPCVV0147JSTitlePath(t *testing.T) {
	text := "import { test, expect } from '@playwright/test';\n" +
		"test.describe('login', () => {\n" +
		"  test.describe(\"form\", () => {\n" +
		"    test('works', async () => { expect(1).toBe(1); });\n" +
		"  });\n" +
		"  test(`tpl ${x}`, async () => { expect(1).toBe(1); });\n" +
		"  test.each([1])('row %s', async () => { expect(1).toBe(1); });\n" +
		"});\n" +
		"test.describe(name, () => {\n" +
		"  test('inner', async () => { expect(1).toBe(1); });\n" +
		"});\n" +
		"test('top', async () => { expect(1).toBe(1); });\n"
	examples, reason := JSExampleSources("e2e/login.spec.ts", text)
	if reason != "" {
		t.Fatalf("reason %q", reason)
	}
	got := titlePaths(examples)
	want := map[int][]string{4: {"login", "form", "works"}, 6: nil, 7: nil, 10: nil, 12: {"top"}}
	for line, path := range want {
		if _, ok := got[line]; !ok || !reflect.DeepEqual(got[line], path) {
			t.Fatalf("line %d title path %q want %q (all %v)", line, got[line], path, got)
		}
	}
}

// LPCV-V0-147: the RSpec reader records the literal group titles and the example title; a constant
// group is its constant name, and an interpolated title records none.
func TestLPCVV0147RSpecTitlePath(t *testing.T) {
	text := "RSpec.describe Admin::User do\n" +
		"  context 'when admin' do\n" +
		"    it \"is admin\" do\n" +
		"      expect(1).to eq(1)\n" +
		"    end\n" +
		"    it \"is #{role}\" do\n" +
		"      expect(1).to eq(1)\n" +
		"    end\n" +
		"  end\n" +
		"  describe User, 'extra' do\n" +
		"    it 'mixed' do\n" +
		"      expect(1).to eq(1)\n" +
		"    end\n" +
		"  end\n" +
		"end\n"
	examples, reason := RSpecExampleSources("spec/user_spec.rb", text)
	if reason != "" {
		t.Fatalf("reason %q", reason)
	}
	got := titlePaths(examples)
	want := map[int][]string{3: {"Admin::User", "when admin", "is admin"}, 6: nil, 11: nil}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("title paths %v want %v", got, want)
	}
}

// LPCV-V0-150, LPCV-V0-152: the static reader reads only committed RSpec and JS/TS test files at
// HEAD under the named paths, in path order, and leaves the worktree unchanged; an uncommitted
// edit is not read; a malformed path and a path holding no test file are refused.
func TestLPCVV0152StaticExamplesReadsCommittedSource(t *testing.T) {
	root := t.TempDir()
	head := commitFiles(t, root, map[string]string{
		"e2e/login.spec.ts":   "test('works', async () => { expect(1).toBe(1); });\n",
		"spec/user_spec.rb":   "RSpec.describe User do\n  xit 'skips' do\n    expect(1).to eq(1)\n  end\nend\n",
		"lib/user.rb":         "class User; end\n",
		"docs/readme.md":      "# readme\n",
		"other/a.test.js":     "it('a', () => { expect(1).toBe(1); });\n",
		"spec/spec_helper.rb": "RSpec.configure { }\n",
	})
	if err := os.WriteFile(filepath.Join(root, "e2e", "login.spec.ts"), []byte("test.only('edited', () => {});\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := gitTest(t, root, "status", "--porcelain", "--untracked-files=all")
	repo := OpenRepo(context.Background(), root)
	revision, files, err := StaticExamples(repo, "", []string{"spec", "e2e"})
	if err != nil {
		t.Fatal(err)
	}
	if revision != head || len(files) != 2 || files[0].Path != "e2e/login.spec.ts" || files[1].Path != "spec/user_spec.rb" {
		t.Fatalf("revision=%s files=%+v", revision, files)
	}
	if works := files[0].Examples[1]; !reflect.DeepEqual(works.TitlePath, []string{"works"}) || works.Read != testvalidity.ExampleSourceLocated {
		t.Fatalf("committed example=%+v", works)
	}
	if skipped := files[1].Examples[2]; len(skipped.Findings) != 1 || skipped.Findings[0].Rule != testvalidity.RuleRSpecSkipped {
		t.Fatalf("rspec example=%+v", skipped)
	}
	if _, all, err := StaticExamples(repo, "HEAD", nil); err != nil || len(all) != 3 {
		t.Fatalf("whole tree=%d err=%v", len(all), err)
	}
	if after := gitTest(t, root, "status", "--porcelain", "--untracked-files=all"); after != before {
		t.Fatalf("status changed:\n%s\n->\n%s", before, after)
	}
	for _, refused := range []struct{ path, code string }{{"../x", "tests-invalid-arguments"}, {".git", "tests-invalid-arguments"}, {"lib", "tests-source-unavailable"}} {
		if _, _, err := StaticExamples(repo, "", []string{refused.path}); !hasCode(err, refused.code) {
			t.Fatalf("%s: err=%v want %s", refused.path, err, refused.code)
		}
	}
}
