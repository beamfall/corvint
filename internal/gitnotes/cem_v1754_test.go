package gitnotes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// buildPortableLedgerReader builds the independent interop/cem01-go module.
func buildPortableLedgerReader(t *testing.T) string {
	t.Helper()
	module, err := filepath.Abs(filepath.Join("..", "..", "interop", "cem01-go"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "cem01-go")
	command := exec.Command("go", "build", "-o", out, ".")
	command.Dir = module
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build interop/cem01-go: %v\n%s", err, raw)
	}
	return out
}

// portableLedger runs `cem01-go read-verdict-ledger` and decodes its one line.
func portableLedger(t *testing.T, binary, root, rev string) (map[string]any, int) {
	t.Helper()
	command := exec.Command(binary, "read-verdict-ledger", "--repository", root, "--commit", rev)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	status := 0
	if err := command.Run(); err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		status = exit.ExitCode()
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil || bytes.Count(stdout.Bytes(), []byte("\n")) != 1 {
		t.Fatalf("portable output %q: %v", stdout.String(), err)
	}
	return decoded, status
}

// CEM-V1-754: the portable reader of refs/notes/corvint-verdicts reports the
// native listing's state and records for every ledger state, and the native
// refusal code for a commit that does not resolve, and writes nothing.
func TestCEMV1754PortableVerdictLedgerMatchesNative(t *testing.T) {
	binary := buildPortableLedgerReader(t)
	a, b := strings.Repeat("a", 64), strings.Repeat("b", 64)
	cases := []struct {
		name, state string
		body        func(t *testing.T, root string) []byte
	}{
		{"absent", "absent", nil},
		{"one record", "verified", func(t *testing.T, root string) []byte {
			return append(recordLine(t, handRecord(t, root, a, b, nil)), '\n')
		}},
		{"two chained records", "verified", func(t *testing.T, root string) []byte {
			first := recordLine(t, handRecord(t, root, a, b, nil))
			link := lineDigest(first)
			second := handRecord(t, root, b, a, &link)
			second.Outcome = "REJECT"
			return []byte(string(first) + "\n" + string(recordLine(t, second)) + "\n")
		}},
		{"unknown member", "malformed", func(t *testing.T, root string) []byte {
			line := recordLine(t, handRecord(t, root, a, b, nil))
			return append(append(line[:len(line)-1:len(line)-1], []byte(`,"extra":1}`)...), '\n')
		}},
		{"non-canonical spacing", "malformed", func(t *testing.T, root string) []byte {
			return append(bytes.Replace(recordLine(t, handRecord(t, root, a, b, nil)), []byte(`":"`), []byte(`": "`), 1), '\n')
		}},
		{"no final line feed", "malformed", func(t *testing.T, root string) []byte {
			return recordLine(t, handRecord(t, root, a, b, nil))
		}},
		{"unknown outcome", "malformed", func(t *testing.T, root string) []byte {
			record := handRecord(t, root, a, b, nil)
			record.Outcome = "UNSUPPORTED"
			return append(recordLine(t, record), '\n')
		}},
		{"non-UTC recordedAt", "malformed", func(t *testing.T, root string) []byte {
			record := handRecord(t, root, a, b, nil)
			record.RecordedAt = "2026-10-19T09:00:00+01:00"
			return append(recordLine(t, record), '\n')
		}},
		{"too many records", "malformed", func(t *testing.T, root string) []byte {
			var body []byte
			var link *string
			for index := 0; index <= MaxVerdictRecords; index++ {
				line := recordLine(t, handRecord(t, root, lineDigest([]byte{byte(index), byte(index >> 8)}), b, link))
				body = append(append(body, line...), '\n')
				digest := lineDigest(line)
				link = &digest
			}
			return body
		}},
		{"edited earlier record", "chain-broken", func(t *testing.T, root string) []byte {
			first := recordLine(t, handRecord(t, root, a, b, nil))
			link := lineDigest(first)
			second := recordLine(t, handRecord(t, root, b, a, &link))
			edited := bytes.Replace(first, []byte(b), []byte(strings.Repeat("d", 64)), 1)
			return []byte(string(edited) + "\n" + string(second) + "\n")
		}},
		{"removed first record", "chain-broken", func(t *testing.T, root string) []byte {
			link := lineDigest(recordLine(t, handRecord(t, root, a, b, nil)))
			return append(recordLine(t, handRecord(t, root, b, a, &link)), '\n')
		}},
		{"record names another commit", "scope-mismatch", func(t *testing.T, root string) []byte {
			record := handRecord(t, root, a, b, nil)
			record.Target.GitCommit = fixtureGit(t, root, nil, "rev-parse", "HEAD~1")
			return append(recordLine(t, record), '\n')
		}},
		{"record names another tree", "scope-mismatch", func(t *testing.T, root string) []byte {
			record := handRecord(t, root, a, b, nil)
			record.Target.GitTree = fixtureGit(t, root, nil, "rev-parse", "HEAD~1^{tree}")
			return append(recordLine(t, record), '\n')
		}},
		{"repeated key", "conflict", func(t *testing.T, root string) []byte {
			first := recordLine(t, handRecord(t, root, a, b, nil))
			link := lineDigest(first)
			return []byte(string(first) + "\n" + string(recordLine(t, handRecord(t, root, a, a, &link))) + "\n")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := fixtureRepo(t)
			head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
			if c.body != nil {
				attachLedger(t, root, head, c.body(t, root))
			}
			native, err := json.Marshal(listVerdicts(t, root))
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal(native, &want); err != nil {
				t.Fatal(err)
			}
			if want["state"] != c.state {
				t.Fatalf("native state %v, want %s", want["state"], c.state)
			}
			before := storeState(t, root)
			got, status := portableLedger(t, binary, root, "HEAD")
			if status != 0 || !reflect.DeepEqual(got, want) {
				t.Fatalf("portable exit %d\n got %v\nwant %v", status, got, want)
			}
			if storeState(t, root) != before {
				t.Fatal("the portable read changed the repository")
			}
		})
	}
	t.Run("unresolvable commit", func(t *testing.T) {
		root := fixtureRepo(t)
		_, err := ProvenanceWithVerdicts(context.Background(), root, "no-such-revision", "", true)
		want := cemcode.CodeOf(err)
		got, status := portableLedger(t, binary, root, "no-such-revision")
		if want != cemcode.RepositoryObjectUnavailable || status != 2 || got["code"] != want || got["ok"] != false {
			t.Fatalf("native %q, portable exit %d %v", want, status, got)
		}
	})
	t.Run("relative repository", func(t *testing.T) {
		got, status := portableLedger(t, binary, "relative/repository", "HEAD")
		if status != 2 || got["code"] != cemcode.InvalidArguments {
			t.Fatalf("portable exit %d %v", status, got)
		}
	})
}

// nativeLedger returns the native `verdicts` member as decoded JSON, or the
// native refusal code.
func nativeLedger(t *testing.T, root string) (map[string]any, string) {
	t.Helper()
	read, err := ProvenanceWithVerdicts(context.Background(), root, "HEAD", "", true)
	if err != nil {
		return nil, cemcode.CodeOf(err)
	}
	raw, err := json.Marshal(read["verdicts"])
	if err != nil {
		t.Fatal(err)
	}
	var listing map[string]any
	if err := json.Unmarshal(raw, &listing); err != nil {
		t.Fatal(err)
	}
	return listing, ""
}

// flatNotes points refs/notes/corvint-verdicts at a notes commit holding count
// notes on objects that need not exist, so `git notes list` prints count lines
// of 82 bytes each.
func flatNotes(t *testing.T, root string, count int) {
	t.Helper()
	var input bytes.Buffer
	input.WriteString("blob\nmark :1\ndata 2\nx\n\ncommit " + VerdictRef + "\ncommitter t <t@example.invalid> 946684800 +0000\ndata 1\nn\n")
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&input, "M 100644 :1 %040x\n", i)
	}
	fixtureGit(t, root, input.Bytes(), "fast-import", "--quiet")
}

// CEM-V1-754: the portable reader admits the repository and bounds its Git
// reads as the native reader does: a subdirectory, a bare repository, an
// alternates-backed clone and effective info/attributes refuse with the native
// code, a linked worktree and a repository below a search-only ancestor are
// read, a notes listing over 4 MiB and a note over 1 MiB refuse
// git-output-exceeded, and each bound admits its limit.
func TestCEMV1754PortableLedgerAdmissionAndBoundsMatchNative(t *testing.T) {
	binary := buildPortableLedgerReader(t)
	cases := []struct {
		name, code, state string
		repo              func(t *testing.T) string
	}{
		{"subdirectory", cemcode.RepositoryObjectUnavailable, "", func(t *testing.T) string {
			return filepath.Join(fixtureRepo(t), "src")
		}},
		{"bare repository", cemcode.RepositoryObjectUnavailable, "", func(t *testing.T) string {
			root := fixtureRepo(t)
			bare := filepath.Join(filepath.Dir(root), "bare.git")
			fixtureGit(t, filepath.Dir(root), nil, "clone", "-q", "--bare", root, bare)
			return bare
		}},
		{"alternates-backed clone", cemcode.UnsupportedObjectAlternates, "", func(t *testing.T) string {
			root := fixtureRepo(t)
			clone := filepath.Join(filepath.Dir(root), "shared")
			fixtureGit(t, filepath.Dir(root), nil, "clone", "-q", "--shared", root, clone)
			return clone
		}},
		{"effective info/attributes", cemcode.UnsupportedRepositoryAttributes, "", func(t *testing.T) string {
			root := fixtureRepo(t)
			writeFile(t, root, ".git/info/attributes", "* -text\n")
			return root
		}},
		{"linked worktree", "", "absent", func(t *testing.T) string {
			root := fixtureRepo(t)
			linked := filepath.Join(filepath.Dir(root), "linked")
			fixtureGit(t, root, nil, "worktree", "add", "-q", "--detach", linked)
			return linked
		}},
		{"note at 1 MiB", "", "malformed", func(t *testing.T) string {
			root := fixtureRepo(t)
			attachLedger(t, root, fixtureGit(t, root, nil, "rev-parse", "HEAD"), bytes.Repeat([]byte("x"), MaxNoteBytes))
			return root
		}},
		{"note over 1 MiB", cemcode.GitOutputExceeded, "", func(t *testing.T) string {
			root := fixtureRepo(t)
			attachLedger(t, root, fixtureGit(t, root, nil, "rev-parse", "HEAD"), bytes.Repeat([]byte("x"), MaxNoteBytes+1))
			return root
		}},
		{"notes list within 4 MiB", "", "absent", func(t *testing.T) string {
			root := fixtureRepo(t)
			flatNotes(t, root, MaxListBytes/82)
			return root
		}},
		{"notes list over 4 MiB", cemcode.GitOutputExceeded, "", func(t *testing.T) string {
			root := fixtureRepo(t)
			flatNotes(t, root, MaxListBytes/82+1)
			return root
		}},
		// An ancestor with search but no read permission, as an 0711 home or
		// the CI Landlock confinement presents "/", is admitted on both.
		{"search-only ancestor", "", "absent", func(t *testing.T) string {
			root := fixtureRepo(t)
			parent := filepath.Dir(root)
			if err := os.Chmod(parent, 0o311); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
			if dir, err := os.Open(parent); err == nil {
				dir.Close()
				t.Skip("this host opens a directory without read permission")
			}
			return root
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := c.repo(t)
			want, code := nativeLedger(t, repo)
			if code != c.code || (code == "" && want["state"] != c.state) {
				t.Fatalf("native code %q listing %v, want code %q state %q", code, want, c.code, c.state)
			}
			got, status := portableLedger(t, binary, repo, "HEAD")
			if code != "" {
				if status != 2 || got["code"] != code || got["ok"] != false {
					t.Fatalf("native %q, portable exit %d %v", code, status, got)
				}
				return
			}
			if status != 0 || !reflect.DeepEqual(got, want) {
				t.Fatalf("portable exit %d\n got %v\nwant %v", status, got, want)
			}
		})
	}
}
