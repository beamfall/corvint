package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestDCWV0922VerifyRunMatchingNoTestIsRefused covers V1-3928: a verify
// line whose -run pattern selects no test in its package is refused before
// any step with a typed code naming the line, while a pattern that selects a
// test, a subtest path, a recursive package, an unknown package and a script
// line pass.
func TestDCWV0922VerifyRunMatchingNoTestIsRefused(t *testing.T) {
	root := t.TempDir()
	for path, body := range map[string]string{
		"cmd/tool/main_test.go":  "package main\n\nfunc TestLocalCompletion(t *testing.T) {}\nfunc BenchmarkOnly(b *testing.B) {}\n",
		"cmd/tool/sub/x_test.go": "package sub\n\nfunc FuzzDeep(f *testing.F) {}\n",
		"cmd/tool/main.go":       "package main\n\nfunc TestNotInATestFile() {}\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		line  string
		empty bool
	}{
		{"go test -count=1 ./cmd/tool -run 'TestNoSuchThing'", true},
		{"script/heavy.sh go test -count=1 -run TestLocalCompletionEvent ./cmd/tool", true},
		{"go test -run BenchmarkOnly ./cmd/tool", true},
		{"go test -run TestNotInATestFile ./cmd/tool", true},
		{"go test -run 'TestNoSuch|TestAlsoMissing' ./cmd/tool", true},
		{"go test -count=1 -run TestLocalCompletion ./cmd/tool", false},
		{"go test -run 'TestNoSuch|TestLocal' ./cmd/tool", false},
		{"go test -run=TestLocalCompletion/sub_case ./cmd/tool", false},
		{"go test -run FuzzDeep ./cmd/tool/...", false},
		{"go test -run FuzzDeep ./cmd/tool", true},
		{"go test -timeout 30m -run TestNoSuch ./cmd/missing", false},
		{"go test -run TestNoSuch github.com/x/y", false},
		{"go test -run '.' ./cmd/tool", false},
		{"sh script/check.sh", false},
	} {
		_, found := emptyRunLineOf(root, ChangeOptions{Verify: "sh script/first.sh\n" + test.line})
		if found != test.empty {
			t.Errorf("%q: refused=%v, want %v", test.line, found, test.empty)
		}
	}

	verify := filepath.Join(t.TempDir(), "verify.txt")
	if err := os.WriteFile(verify, []byte("go test -count=1 -run TestLocalCompletion ./cmd/tool\ngo test -count=1 ./cmd/tool -run 'TestNoSuchThing'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	lane := &finishLane{flow: flow{ctx: context.Background(), prefix: "dogfood-finish-lane", root: root, stderr: &stderr}, options: FinishLaneOptions{Change: ChangeOptions{Base: "B", VerifyFile: verify}}}
	code, err := boundary(context.Background(), func() int { lane.requireVerifyInput(); return 0 }, func() {})
	if err != nil {
		t.Fatal(err)
	}
	want := "dogfood-finish-lane: REFUSE verify-run-matches-no-test\n" +
		"  input: --verify " + verify + " line 2: go test -count=1 ./cmd/tool -run 'TestNoSuchThing'\n" +
		"  refused: -run TestNoSuchThing matches no Test, Example or Fuzz function in the _test.go files of ./cmd/tool, so go test would pass having run no test (DCW-V0-922)\n" +
		"  fix: name a test that exists in that package (grep -n '^func Test' in its _test.go files), or drop -run, then rerun corvint dogfood finish-lane --base B\n"
	if code != 2 || stderr.String() != want {
		t.Fatalf("exit=%d stderr=\n%s\nwant\n%s", code, stderr.String(), want)
	}
}
