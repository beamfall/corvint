package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildPinsOwnedVariantsAndRunnerRegistry(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	out := filepath.Join(t.TempDir(), "owned")
	if err = build(ctx, out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "fixture-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Fixtures []struct{ Name, SHA256 string } `json:"fixtures"`
	}
	if err = json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Fixtures) != 2 {
		t.Fatalf("%s", b)
	}
	for _, f := range m.Fixtures {
		binary, err := os.ReadFile(filepath.Join(out, f.Name))
		if err != nil {
			t.Fatal(err)
		}
		if sha(binary) != f.SHA256 {
			t.Fatal("registry does not pin binary")
		}
	}
}
