package doccorpus

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
)

func TestLoadInputRefusesOverBoundBlobBeforeBody(t *testing.T) {
	t.Parallel()
	t.Run("DCP-V1-018 V1-0747 input byte bound before allocation", func(t *testing.T) {
		root := t.TempDir()
		git(t, root, "init", "-q")
		git(t, root, "config", "user.name", "Corpus test")
		git(t, root, "config", "user.email", "corpus@example.invalid")
		const body = "123456789"
		if err := os.WriteFile(filepath.Join(root, "big.md"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", "source")
		auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
		if err != nil {
			t.Fatal(err)
		}
		index := &contextindex.Index{CommitRevision: git(t, root, "rev-parse", "HEAD"), Sources: map[string]contextindex.Source{}}
		var refused *Error
		if _, err := loadInput(context.Background(), auth, index, "big.md", len(body)-1); !errors.As(err, &refused) || refused.Code != "corpus-refused" || refused.Message != "input bound exceeded" {
			t.Fatalf("bound+1 input: %v", err)
		}
		source, err := loadInput(context.Background(), auth, index, "big.md", len(body))
		if err != nil || string(source.Data) != body {
			t.Fatalf("exact-bound input: %q %v", source.Data, err)
		}
	})
}

func TestLocalInputPathRuleNamesTheBrokenRule(t *testing.T) {
	t.Parallel()
	t.Run("DCP-V1-054 V1-1226 reader refusals name the rule without widening admission", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "input.json"), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
		for value, rule := range map[string]string{
			"":                        "got an empty path",
			"/abs/input.json":         "got an absolute path",
			"..":                      "got a parent traversal",
			"../input.json":           "got a parent traversal",
			".":                       "got the root itself",
			"./input.json":            "got a non-canonical path",
			"a//input.json":           "got a non-canonical path",
			"a/../input.json":         "got a non-canonical path",
			"input.json/":             "got a non-canonical path",
			"a\\input.json":           "got a backslash, NUL, tab, CR or LF",
			"input.json\n":            "got a backslash, NUL, tab, CR or LF",
			string([]byte{0xff}):      "got a path that is not UTF-8",
			strings.Repeat("a", 1025): "got a path over 1024 bytes",
		} {
			if got := LocalInputPathRule(value); got != rule {
				t.Errorf("LocalInputPathRule(%q) = %q, want %q", value, got, rule)
			}
			_, err := ReadFile(root, value)
			var refusal *Error
			want := "invalid local input path: input paths must be canonical and relative to the input root (" + rule + ")"
			if !errors.As(err, &refusal) || refusal.Code != "corpus-refused" || refusal.Message != want {
				t.Errorf("ReadFile(%q) = %v, want corpus-refused %q", value, err, want)
			}
		}
		for _, value := range []string{"input.json", "a/b.json", strings.Repeat("a", 1024)} {
			if rule := LocalInputPathRule(value); rule != "" {
				t.Fatalf("canonical relative path %q refused: %s", value, rule)
			}
		}
		if data, err := ReadFile(root, "input.json"); err != nil || string(data) != "{}" {
			t.Fatalf("canonical relative path not read: %q %v", data, err)
		}
	})
}
