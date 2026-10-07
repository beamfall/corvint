package main

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// AMSP-V0-010: `flows appmap plan` serves the same bytes as the library over both maps of the
// planner fixture, with the draft on request, and writes nothing to the repository.
func TestAMSPV0010FlowsAppmapPlanCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	src := filepath.Join("..", "..", "internal", "appmap", "testdata", "plan")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "overlay"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	dir := t.TempDir()
	files := []string{}
	maps := []*appmap.Map{}
	for _, manifest := range []string{"appmap.json", "marketplace.json"} {
		code, out, diagnostic := runFlowsCLI(root, "appmap", "build", "--manifest", manifest)
		if code != 0 {
			t.Fatalf("build %s: %d %s", manifest, code, diagnostic)
		}
		name := filepath.Join(dir, manifest)
		if err := os.WriteFile(name, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
		m, err := appmap.LoadMap(name)
		if err != nil {
			t.Fatal(err)
		}
		files, maps = append(files, name), append(maps, m)
	}
	request := "book a tee time; check the slot status; buy a gift card; change the club settings"
	code, out, diagnostic := runFlowsCLI(root, "appmap", "plan", "--map", files[0], "--map", files[1], "--request", request, "--draft")
	if code != 0 {
		t.Fatalf("plan %d %s", code, diagnostic)
	}
	want, err := appmap.Plan(context.Background(), maps, appmap.SplitRequest(request), appmap.PlanOptions{Options: appmap.Options{Root: root}, Draft: true})
	if err != nil || !bytes.Equal([]byte(out), want) {
		t.Fatalf("CLI and library differ (%v):\n%s\n%s", err, out, want)
	}
	if !strings.Contains(out, `"status":"COMPLETE"`) || strings.Count(out, "await test.step(") != 4 {
		t.Fatalf("plan: %s", out)
	}
	code, steps, _ := runFlowsCLI(root, "appmap", "plan", "--map", files[0], "--map", files[1], "--step", "book a tee time", "--step", "check the slot status",
		"--step", "buy a gift card", "--step", "change the club settings", "--draft")
	if code != 0 || steps != out {
		t.Fatalf("--step and --request differ: %d", code)
	}
	for _, args := range [][]string{
		{"appmap", "plan", "--request", request},
		{"appmap", "plan", "--map", files[0]},
		{"appmap", "plan", "--map", files[0], "--request", request, "--step", "x"},
		{"appmap", "plan", "--map", files[0], "--map", files[0], "--request", request},
		{"appmap", "plan", "--map", files[0], "--request", request, "--budget", "300"},
		{"appmap", "plan", "--map", filepath.Join(dir, "missing.json"), "--request", request},
	} {
		if code, out, _ := runFlowsCLI(root, args...); code != 2 || out != "" {
			t.Errorf("%v: %d %q", args, code, out)
		}
	}
	cmd := exec.Command("git", "status", "--porcelain", "--ignored")
	cmd.Dir = root
	if status, err := cmd.Output(); err != nil || len(status) != 0 {
		t.Fatalf("worktree changed: %s %v", status, err)
	}
}
