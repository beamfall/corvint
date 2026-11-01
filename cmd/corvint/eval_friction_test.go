package main

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestURE997EvalFrictionReportsWithoutMutating(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	record := `{"profile":"corvint-friction-sessions/0","sessions":[{"id":"s","events":[{"kind":"read","path":"a","planned":false},{"kind":"read","path":"a","planned":true}]}]}`
	if err := os.WriteFile(filepath.Join(root, "sessions.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"eval", "--friction", "sessions.json"}, {"eval", "--friction=sessions.json"}} {
		code, stdout, stderr := slotWeightsRun(t, root, arguments...)
		if code != 0 {
			t.Fatalf("%v: exit %d: %s", arguments, code, stderr)
		}
		var payload struct {
			Ok       bool           `json:"ok"`
			Mutates  bool           `json:"mutates"`
			Friction map[string]any `json:"friction"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil || !payload.Ok || payload.Mutates {
			t.Fatalf("%v: payload %s, %v", arguments, stdout, err)
		}
		metric := payload.Friction["sessions"].([]any)[0].(map[string]any)["metrics"].(map[string]any)["rereads"].(map[string]any)
		if metric["value"] != 1.0 {
			t.Fatalf("rereads = %v", metric)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("eval --friction wrote state: %v, %v", entries, err)
	}
	for _, arguments := range [][]string{{"eval", "--friction"}, {"eval", "--friction", "sessions.json", "--goldens", "x"}, {"eval", "--friction", "missing.json"}} {
		if code, _, stderr := slotWeightsRun(t, root, arguments...); code != 2 || !strings.Contains(stderr, "error") {
			t.Fatalf("%v: exit %d, stderr %s", arguments, code, stderr)
		}
	}
}

// TestURE997OnlyTheEvalCommandImportsFrictionMetric keeps the metrics out of
// ranking, retrieval, evidence, authority and learning code paths.
func TestURE997OnlyTheEvalCommandImportsFrictionMetric(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	const importPath = "github.com/Beamfall/corvint/internal/frictionmetric"
	var importers []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); path != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return nil
		}
		for _, spec := range parsed.Imports {
			if value, _ := strconv.Unquote(spec.Path.Value); value == importPath {
				relative, _ := filepath.Rel(root, path)
				importers = append(importers, filepath.ToSlash(relative))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"cmd/corvint/eval_friction.go"}; !reflect.DeepEqual(importers, want) {
		t.Fatalf("importers = %v, want %v", importers, want)
	}
}
