package testsource

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// LPCV-V0-152: in static mode the test files and every RSpec support set draw on one command
// budget, so support sets that each fit alone but not together refuse tests-bound-exceeded instead
// of reading as rspec-support-unreadable.
func TestLPCVV0152StaticSupportSharesTheCommandBudget(t *testing.T) {
	spec := "RSpec.describe User do\n  it 'works' do\n    expect(1).to eq(1)\n  end\nend\n"
	helperA, helperB := "# a\n"+strings.Repeat("x", 2000)+"\n", "# b\n"+strings.Repeat("y", 2000)+"\n"
	root := t.TempDir()
	commitFiles(t, root, map[string]string{
		"a/spec/x_spec.rb": spec, "b/spec/y_spec.rb": spec + "\n",
		"a/spec/spec_helper.rb": helperA, "b/spec/spec_helper.rb": helperB,
	})
	repo := OpenRepo(context.Background(), root)
	tests := len(spec) + 128 + len(spec) + 1 + 128
	_, _, err := staticExamplesWithin(repo, "", nil, &readBudget{files: MaxFiles, bytes: tests + len(helperA) + 128 + 1000})
	if !hasCode(err, "tests-bound-exceeded") {
		t.Fatalf("err=%v want tests-bound-exceeded", err)
	}
	_, files, err := staticExamplesWithin(repo, "", nil, &readBudget{files: MaxFiles, bytes: tests + len(helperA) + len(helperB) + 256})
	if err != nil || len(files) != 2 || files[0].Reason != "" || files[1].Reason != "" {
		t.Fatalf("files=%+v err=%v", files, err)
	}
	if _, _, err := staticExamplesWithin(repo, "", nil, &readBudget{files: 3, bytes: maxTotalBytes}); !hasCode(err, "tests-bound-exceeded") {
		t.Fatalf("file bound err=%v want tests-bound-exceeded", err)
	}
}

// LPCV-V0-147: two declarations on one line cannot be told apart, so neither records a title path.
func TestLPCVV0147JSAmbiguousLineHasNoTitlePath(t *testing.T) {
	text := "test.describe('g', () => {\n  test('a', () => { expect(1).toBe(1); }); test('b', () => { expect(1).toBe(1); });\n});\n"
	examples, reason := JSExampleSources("e2e/two.spec.ts", text)
	if reason != "" {
		t.Fatalf("reason %q", reason)
	}
	got, ok := examples[2]
	if !ok || got.Read != testvalidity.HygieneReasonJSLineAmbiguous || got.TitlePath != nil {
		t.Fatalf("ambiguous line=%+v", got)
	}
}

// LPCV-V0-147: a computed RSpec description (a literal joined with an expression) is not literal,
// for a group and for an example; a literal followed by metadata, a block or a paren still is.
func TestLPCVV0147RSpecComputedTitleNotLiteral(t *testing.T) {
	text := "RSpec.describe \"group\" + suffix do\n" +
		"  it 'inner' do\n" +
		"    expect(1).to eq(1)\n" +
		"  end\n" +
		"end\n" +
		"RSpec.describe('plain') do\n" +
		"  it \"works\" + suffix do\n" +
		"    expect(1).to eq(1)\n" +
		"  end\n" +
		"  it('kept', :focus) { expect(1).to eq(1) }\n" +
		"  it 'block' do\n" +
		"    expect(1).to eq(1)\n" +
		"  end\n" +
		"end\n"
	examples, reason := RSpecExampleSources("spec/computed_spec.rb", text)
	if reason != "" {
		t.Fatalf("reason %q", reason)
	}
	got := titlePaths(examples)
	want := map[int][]string{2: nil, 7: nil, 10: {"plain", "kept"}, 11: {"plain", "block"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("title paths %v want %v", got, want)
	}
}
