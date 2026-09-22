package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/plansnapshot"
)

func TestAffectedSnapshotMatchesCommittedPlanAcrossDirtySources(t *testing.T) {
	t.Run("AFP-V0-019 TestAffectedSnapshotMatchesCommittedPlanAcrossDirtySources", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		path := "core/core.go"
		body, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, path), append(body, []byte("\n// changed\n")...), 0600); err != nil {
			t.Fatal(err)
		}
		affectedGit(t, root, "add", path)
		affectedGit(t, root, "commit", "-qm", "change")
		r := plansnapshot.Receipt{Schema: plansnapshot.Schema, Base: base, Commit: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD")), Tree: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD^{tree}")), Paths: []string{path}, Digest: plansnapshot.PathDigest([]string{path})}
		raw, _ := json.Marshal(r)
		name := filepath.Join(t.TempDir(), "receipt.json")
		os.WriteFile(name, raw, 0600)
		clean, err := compileSnapshotAffected(context.Background(), affectedInvocation{Root: root, Snapshot: name})
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(root, path), []byte("unparseable dirty module"), 0600)
		os.WriteFile(filepath.Join(root, "untracked.go"), []byte("package bad"), 0600)
		before := treeDigest(t, root)
		dirty, err := compileSnapshotAffected(context.Background(), affectedInvocation{Root: root, Snapshot: name})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(clean, dirty) {
			t.Fatal("dirty worktree changed immutable plan")
		}
		if before != treeDigest(t, root) {
			t.Fatal("planning mutated repository")
		}
		got := dirty.(affectedReceipt)
		if got.Snapshot["accepting"] != false || got.Advice.Status != "PLAN_ONLY" || len(got.Plan.Selected) == 0 {
			t.Fatalf("%+v", got)
		}
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), []string{"--root", root, "affected", "--snapshot", name}, strings.NewReader(""), &stdout, &stderr)
		if code != 0 || stdout.Len() == 0 {
			t.Fatalf("%d %s", code, stderr.String())
		}
		r.Digest = strings.Repeat("0", 64)
		raw, _ = json.Marshal(r)
		os.WriteFile(name, raw, 0600)
		stdout.Reset()
		stderr.Reset()
		code = runContext(context.Background(), []string{"--root", root, "affected", "--snapshot", name}, strings.NewReader(""), &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported-planning-snapshot") {
			t.Fatalf("%d %s %s", code, stdout.String(), stderr.String())
		}
		for _, args := range [][]string{{"--snapshot", name, "--base", base}, {"--snapshot", name, "--snapshot", name}, {"--snapshot", name, "--provider", "foo"}} {
			if _, err := parseAffectedOptions(args); err == nil {
				t.Fatalf("admitted %v", args)
			}
		}
	})
}

func TestAffectedSnapshotPlaywrightPinsConfigAndSource(t *testing.T) {
	t.Run("AFP-V0-019 TestAffectedSnapshotPlaywrightPinsConfigAndSource", func(t *testing.T) {
		root := t.TempDir()
		writePlaywrightCLIFile(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
		writePlaywrightCLIFile(t, root, "playwright.config.ts", `export default {projects:[{name:"chromium"}]}`)
		writePlaywrightCLIFile(t, root, "src/page.ts", `export const page = "base"`)
		writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import {test} from "@playwright/test"; import {page} from "../src/page"; test("page",()=>page)`)
		affectedGit(t, root, "init", "-q")
		affectedGit(t, root, "config", "user.name", "Test")
		affectedGit(t, root, "config", "user.email", "test@example.invalid")
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "base")
		base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		writePlaywrightCLIFile(t, root, "src/page.ts", `export const page = "target"`)
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "target")
		paths := []string{"src/page.ts"}
		r := plansnapshot.Receipt{Schema: plansnapshot.Schema, Base: base, Commit: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD")), Tree: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD^{tree}")), Paths: paths, Digest: plansnapshot.PathDigest(paths)}
		raw, _ := json.Marshal(r)
		name := filepath.Join(t.TempDir(), "snapshot.json")
		os.WriteFile(name, raw, 0600)
		invocation := affectedInvocation{Root: root, Snapshot: name, PlaywrightConfig: "playwright.config.ts"}
		clean, err := compileSnapshotAffected(context.Background(), invocation)
		if err != nil {
			t.Fatal(err)
		}
		writePlaywrightCLIFile(t, root, "playwright.config.ts", `throw new Error("dirty config must not run")`)
		writePlaywrightCLIFile(t, root, "src/page.ts", `invalid mutable source`)
		dirty, err := compileSnapshotAffected(context.Background(), invocation)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(clean, dirty) {
			t.Fatal("mutable config/source changed snapshot")
		}
		plan := dirty.(playwrightAffectedReceipt)
		if plan.Snapshot["accepting"] != false || len(plan.Plan.Selected) != 0 || len(plan.Plan.FallbackArgv) == 0 || plan.Plan.Config.SHA256 == "" || plan.Plan.SourceDigest == "" {
			t.Fatalf("snapshot lost evidence or missing-discovery fallback: %+v", plan)
		}
	})
}
