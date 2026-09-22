package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmptyRootsRefuseBeforeExecution(t *testing.T) {
	for _, args := range [][]string{
		{"-source-root=", "-tasks-root", t.TempDir()},
		{"-tasks-root=", "-source-root", t.TempDir()},
	} {
		scratch := filepath.Join(t.TempDir(), "must-not-exist")
		args = append(args, "-scratch", scratch, "-output-parent", t.TempDir(), "-bundle-name", "fixture", "-npm-cache", t.TempDir())
		var out, err bytes.Buffer
		if code := run(args, &out, &err); code != 2 || !strings.Contains(err.String(), "required") {
			t.Fatalf("%v: %d %s", args, code, err.String())
		}
		if _, e := os.Stat(scratch); !os.IsNotExist(e) {
			t.Fatalf("refusal touched scratch: %v", e)
		}
	}
}

func TestCoreBundleBuildSkipsEditorTools(t *testing.T) {
	t.Run("CRB-V0-019 no npm cache argument required", func(t *testing.T) {
		scratch := filepath.Join(t.TempDir(), "must-not-exist")
		args := []string{"-source-root", t.TempDir(), "-tasks-root", t.TempDir(), "-scratch", scratch, "-output-parent", t.TempDir(), "-bundle-name", "core", "-target", "unsupported"}
		var out, err bytes.Buffer
		if code := run(args, &out, &err); code != 1 || !strings.Contains(err.String(), "unsupported") {
			t.Fatalf("missing npm cache rejected before core admission: %d %s", code, err.String())
		}
		if _, e := os.Stat(scratch); !os.IsNotExist(e) {
			t.Fatalf("refusal touched scratch: %v", e)
		}
	})
}
