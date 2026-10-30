package contextindex

import (
	"context"
	"reflect"
	"testing"
)

// declaredReceipts builds files into a repository, commits an edit to changed,
// and returns the path receipt for changed and the range receipt over the edit.
func declaredReceipts(t *testing.T, files map[string]string, changed, replacement string) (map[string]any, map[string]any) {
	t.Helper()
	root := impactRepositoryWithFiles(t, files)
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, changed, replacement)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "changed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	path, err := Impact(index, []string{changed}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	ranged, err := RangeImpact(context.Background(), index, base, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	return path, ranged
}

func npmRepository(extra map[string]string) map[string]string {
	files := map[string]string{
		"package.json": "{\"name\":\"docs\",\"scripts\":{\"build\":\"docusaurus build\",\"test\":\"vitest\"}}\n",
		"src/a.ts":     "export const a = 1;\n",
	}
	for name, body := range extra {
		files[name] = body
	}
	return files
}

// TestNGIV0060DeclaredVerifyCommandClosesMakefileLessPlan: NGI-V0-060,
// NGI-V0-061, AFP-V0-196. In a Makefile-less npm repository whose AGENTS.md
// declares `npm run build` on an inline Verify line, path and range impact
// close their plans with it instead of the inferred `git diff --check`, and
// label it mandatory and the inferred `npm test` inferred.
func TestNGIV0060DeclaredVerifyCommandClosesMakefileLessPlan(t *testing.T) {
	t.Parallel()
	files := npmRepository(map[string]string{"AGENTS.md": "# Agents\n\nRun corvint first. **Verify:** `npm run build` (Tasks gate id `build`).\n"})
	path, ranged := declaredReceipts(t, files, "src/a.ts", "export const a = 2;\n")
	want := []any{"npm test", "npm run build"}
	label := map[string]any{"source": "AGENTS.md", "mandatory": []any{"npm run build"}, "inferred": []any{"npm test"}}
	if got := anySlice(path["verification"]); !reflect.DeepEqual(got, want) {
		t.Fatalf("path verification = %v, want %v", got, want)
	}
	if got := path["verification_authority"]; !reflect.DeepEqual(got, label) {
		t.Fatalf("path verification_authority = %#v, want %#v", got, label)
	}
	if got := ranged["verification_authority"].(map[string]any)["mandatory"]; !reflect.DeepEqual(got, []any{"npm run build"}) {
		t.Fatalf("range mandatory = %#v (verification %v)", got, ranged["verification"])
	}
}

// TestNGIV0061InferredCommandBesideDeclaredGate: NGI-V0-061. A web path in a
// repository whose AGENTS.md Verify block declares only `make gate` keeps the
// inferred `npm test` but labels it inferred, not mandatory; a repository that
// declares nothing carries no label and keeps its bytes.
func TestNGIV0061InferredCommandBesideDeclaredGate(t *testing.T) {
	t.Parallel()
	declared := npmRepository(map[string]string{
		"Makefile":  "gate:\n\tnpm run gate\n",
		"AGENTS.md": "# Agents\n\n## Verify\n\n```sh\nmake gate\n```\n",
	})
	path, _ := declaredReceipts(t, declared, "src/a.ts", "export const a = 2;\n")
	label := map[string]any{"source": "AGENTS.md", "mandatory": []any{"make gate"}, "inferred": []any{"npm test"}}
	if got := path["verification_authority"]; !reflect.DeepEqual(got, label) {
		t.Fatalf("verification_authority = %#v, want %#v (verification %v)", got, label, path["verification"])
	}
	plain := npmRepository(map[string]string{"Makefile": "gate:\n\tnpm run gate\n"})
	path, ranged := declaredReceipts(t, plain, "src/a.ts", "export const a = 2;\n")
	for name, receipt := range map[string]map[string]any{"path": path, "range": ranged} {
		if _, present := receipt["verification_authority"]; present {
			t.Fatalf("%s receipt without a declaration carries verification_authority: %#v", name, receipt["verification_authority"])
		}
	}
	if got := anySlice(path["verification"]); !reflect.DeepEqual(got, []any{"npm test", "make gate"}) {
		t.Fatalf("undeclared path verification = %v", got)
	}
}

// TestNGIV0062RangeAndPathChooseOneGate: NGI-V0-062 (V1-1848). In a Ruby
// repository with a Makefile test target, path and range impact close with the
// same gate: the Makefile's `make test`, not the generic `git diff --check` the
// range plan used to take from the profile alone.
func TestNGIV0062RangeAndPathChooseOneGate(t *testing.T) {
	t.Parallel()
	files := rubyVerificationFiles(map[string]string{
		"Gemfile":  "source \"https://rubygems.org\"\ngem \"rspec\"\n",
		".rspec":   "--require spec_helper\n",
		"Makefile": "test:\n\tbundle exec rspec\n",
	})
	const changed = "app/api_clients/foo_client.rb"
	path, ranged := declaredReceipts(t, files, changed, "class FooClient\n  def fetch = :changed\nend\n")
	want := []any{"bundle exec rspec", "make test"}
	if got := anySlice(path["verification"]); !reflect.DeepEqual(got, want) {
		t.Fatalf("path verification = %v, want %v", got, want)
	}
	if got := anySlice(ranged["verification"]); !reflect.DeepEqual(got, want) {
		t.Fatalf("range verification = %v, want %v", got, want)
	}
}
