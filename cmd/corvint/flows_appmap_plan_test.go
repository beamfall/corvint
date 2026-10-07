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

// planCLIRepo commits the planner fixture overlay onto the CLI repository and builds both maps
// through the CLI; it returns the root, the map files and the decoded maps.
func planCLIRepo(t *testing.T) (string, []string, []*appmap.Map) {
	t.Helper()
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
	return root, files, maps
}

// AMSP-V0-010: `flows appmap plan` serves the same bytes as the library over both maps of the
// planner fixture, with the draft on request, and writes nothing to the repository.
func TestAMSPV0010FlowsAppmapPlanCLI(t *testing.T) {
	root, files, maps := planCLIRepo(t)
	dir := filepath.Dir(files[0])
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

// AMSP-V0-007 AMSP-V0-010 RVN-V0-006: `flows appmap plan --receipt --bind` passes the
// run-verification overlay of every map. A passing receipt bound to a planned step reads
// VERIFIED@<rev> at the receipt's revision (the step stays candidate: its selector has no
// run-verification producer), and a later router change reads UNVERIFIED_AT_HEAD.
func TestAMSPV0010PlanReceiptVerificationCLI(t *testing.T) {
	root, files, maps := planCLIRepo(t)
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	rev := git("rev-parse", "HEAD")
	receipt, key := rvnReceipt(t, rev)
	bind := "step:check-slot-status/read-status=" + key
	plan := []string{"appmap", "plan", "--map", files[0], "--map", files[1], "--step", "check the slot status"}
	code, out, diagnostic := runFlowsCLI(root, append(plan, "--receipt", receipt, "--bind", bind)...)
	if code != 0 || !strings.Contains(out, `"verification":"VERIFIED@`+rev+`"`) || !strings.Contains(out, `"confidence":"candidate"`) {
		t.Fatalf("verified plan %d %s %s", code, diagnostic, out)
	}
	v, err := appmap.LoadPlanVerification(maps, []string{receipt}, []string{bind})
	if err != nil {
		t.Fatal(err)
	}
	o := appmap.Options{Root: root}
	for _, m := range maps {
		o.Overlays = append(o.Overlays, v.Overlay(m, appmap.Options{Root: root}))
	}
	if want, err := appmap.Plan(context.Background(), maps, []string{"check the slot status"}, appmap.PlanOptions{Options: o}); err != nil || string(want) != out {
		t.Fatalf("CLI and library differ (%v):\n%s\n%s", err, out, want)
	}
	for want, extra := range map[string][]string{
		"appmap-invalid-query":       {"--bind", bind},
		"appmap-verify-unknown-step": {"--receipt", receipt, "--bind", "step:none/x=" + key},
	} {
		if code, out, diagnostic := runFlowsCLI(root, append(append([]string{}, plan...), extra...)...); code != 2 || out != "" || !strings.Contains(diagnostic, want) {
			t.Errorf("%v: %d %q %s, want %s", extra, code, out, diagnostic, want)
		}
	}
	routes, err := os.ReadFile(filepath.Join(root, "app", "routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(routes), "flags: ['new_teesheet']", "flags: ['new_teesheet', 'beta']", 1)
	if changed == string(routes) {
		t.Fatal("fixture route not found")
	}
	if err := os.WriteFile(filepath.Join(root, "app", "routes.js"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	git("commit", "-qam", "change teesheet state")
	code, out, diagnostic = runFlowsCLI(root, append(plan, "--receipt", receipt, "--bind", bind)...)
	if code != 0 || !strings.Contains(out, `"verification":"UNVERIFIED_AT_HEAD"`) || strings.Contains(out, `"verification":"VERIFIED@`) {
		t.Fatalf("changed source %d %s %s", code, diagnostic, out)
	}
}
