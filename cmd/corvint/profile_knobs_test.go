package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/procprofile"
)

func profileKnobLookup(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}

// profileCorpusFixture is a one-page committed repository with a built corpus artifact.
func profileCorpusFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.md"), []byte("# Evidence\n\nOriginal source.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		c.Dir = root
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("add", "source.md")
	git("commit", "-qm", "source")
	m, err := doccorpus.Inventory(context.Background(), root, git("rev-parse", "HEAD"), "source.md", "2026-10-23T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{"corpus.json": a, "manifest.json": m} {
		data, err := doccorpus.Encode(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Keep the worktree clean so reads report fresh and affected --path admits it.
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte("corpus.json\nmanifest.json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// PRB-V0-001, PRB-V0-003, PRB-V0-008 (GitHub #1600 rows): every command the issue names, and
// the paths the knob already covered, writes both profiles through main's single bracket, and
// the knobs never change a stdout byte or the exit status.
func TestPRBV0008EveryNamedCommandWritesProfiles(t *testing.T) {
	root := profileCorpusFixture(t)
	corpus := func(op string, extra ...string) []string {
		return append([]string{"--root", root, "docs", "corpus", op}, extra...)
	}
	rows := map[string][]string{
		"docs corpus build":    corpus("build", "--manifest", "manifest.json"),
		"docs corpus info":     corpus("info", "--artifact", "corpus.json"),
		"docs corpus search":   corpus("search", "--artifact", "corpus.json", "--query", "Original"),
		"docs corpus get":      corpus("get", "--artifact", "corpus.json", "--id", "native:file:source.md"),
		"docs corpus flow":     corpus("flow", "--artifact", "corpus.json", "--id", "native:file:source.md"),
		"docs corpus coverage": corpus("coverage", "--artifact", "corpus.json"),
		"docs corpus locate":   corpus("locate", "--artifact", "corpus.json", "--path", "source.md"),
		"docs corpus delta":    corpus("delta", "--previous", "corpus.json", "--artifact", "corpus.json"),
		"affected --path":      {"--root", root, "affected", "--path", "source.md"},
		"context":              {"--root", root, "context", "--task", "Original source evidence"},
		"help --json":          {"help", "--json"},
	}
	names := make([]string, 0, len(rows))
	for name := range rows {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		arguments := rows[name]
		dir := t.TempDir()
		cpu, mem := filepath.Join(dir, "cpu.pprof"), filepath.Join(dir, "heap.pprof")
		var profiled, plain, profiledErr, plainErr bytes.Buffer
		code := runProfiledContext(context.Background(), context.Background(), arguments, strings.NewReader(""), &profiled, &profiledErr, profileKnobLookup(map[string]string{"CORVINT_CPUPROFILE": cpu, "CORVINT_MEMPROFILE": mem}))
		for _, path := range []string{cpu, mem} {
			if err := procprofile.Verify(path); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		if code != 0 {
			t.Fatalf("%s: exit %d stderr %.300s", name, code, profiledErr.String())
		}
		plainCode := runProfiledContext(context.Background(), context.Background(), arguments, strings.NewReader(""), &plain, &plainErr, profileKnobLookup(nil))
		if code != plainCode || !bytes.Equal(profiled.Bytes(), plain.Bytes()) {
			t.Fatalf("%s: knobs changed the result: exit %d vs %d, stdout %d vs %d bytes", name, code, plainCode, profiled.Len(), plain.Len())
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 2 {
			t.Fatalf("%s: unprofiled run wrote into the profile directory: %v", name, entries)
		}
	}
}

// PRB-V0-006: an unusable knob refuses with a typed JSON error before any command runs.
func TestPRBV0006UnusableKnobRefusesCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "absent", "cpu.pprof")
	code := runProfiledContext(context.Background(), context.Background(), []string{"help", "--json"}, strings.NewReader(""), &stdout, &stderr, profileKnobLookup(map[string]string{"CORVINT_CPUPROFILE": missing}))
	var refusal struct {
		Code string `json:"code"`
		OK   bool   `json:"ok"`
	}
	if code != 2 || stdout.Len() != 0 || json.Unmarshal(stderr.Bytes(), &refusal) != nil || refusal.Code != "profile-unavailable" || refusal.OK {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, &stdout, &stderr)
	}
}

// PRB-V0-008: the bracket is the only CPU profiler start in the binary, so no command can
// collide with it ("cpu profiling already in use") or bypass it.
func TestPRBV0008NoNestedProfilerStart(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("pprof.StartCPUProfile")) || bytes.Contains(data, []byte(`runtimeenv.Value("CPUPROFILE")`)) {
			t.Errorf("%s starts its own profile; the knobs belong to main's bracket (profile_knobs.go)", file)
		}
	}
}

// PRB-V0-009, PRB-V0-010, PRB-V0-012: help --json lists the knobs, every dispatched command, and every MCP
// server, and each listed server's entry point applies the bracket.
func TestPRBV0009HelpJSONListsProfileKnobScope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"help", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("help --json: exit=%d stderr=%q", code, &stderr)
	}
	var catalog struct {
		Profiling struct {
			Schema string `json:"schema"`
			Knobs  []struct {
				Name    string `json:"name"`
				Profile string `json:"profile"`
			} `json:"knobs"`
			Default     string   `json:"default"`
			Commands    []string `json:"commands"`
			Plumbing    []string `json:"plumbing"`
			Servers     []string `json:"servers"`
			NotHonoured struct {
				Entries []string `json:"entries"`
				Reason  string   `json:"reason"`
			} `json:"not_honoured"`
		} `json:"profiling"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	p := catalog.Profiling
	if p.Schema != "corvint-profile-knobs/0" || p.Default != "off" || len(p.Knobs) != 2 || p.Knobs[0].Name != "CORVINT_CPUPROFILE" || p.Knobs[1].Name != "CORVINT_MEMPROFILE" || p.Knobs[1].Profile != "heap" {
		t.Fatalf("profiling member: %+v", p)
	}
	if !slices.Equal(p.Commands, topLevelCommands) || !slices.Contains(p.Commands, "docs") || !slices.Contains(p.Commands, "affected") {
		t.Fatalf("commands %v, want every dispatched command %v", p.Commands, topLevelCommands)
	}
	if !slices.Equal(p.Plumbing, hookPlumbingVerbs) {
		t.Fatalf("plumbing %v, want the dispatched plumbing verbs %v", p.Plumbing, hookPlumbingVerbs)
	}
	// PRB-V0-012: the workers main runs before the bracket are named as not honouring the knobs.
	if !slices.Equal(p.NotHonoured.Entries, profileKnobWorkers) || p.NotHonoured.Reason == "" {
		t.Fatalf("not_honoured %+v", p.NotHonoured)
	}
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	bracket := bytes.Index(main, []byte("runProfiledContext(signalled, ctx, os.Args[1:]"))
	for _, worker := range []string{"runSnapshotRefreshWorker(", "runDogfoodOutcomeWorker(", "runDogfoodVerifierWorker("} {
		if at := bytes.Index(main, []byte(worker)); at < 0 || bracket < 0 || at > bracket {
			t.Errorf("%s is not dispatched before the profile bracket", worker)
		}
	}
	servers, err := filepath.Glob(filepath.Join("..", "corvint-*-mcp"))
	if err != nil {
		t.Fatal(err)
	}
	// corvint-mcp's entry point moved to internal/mcp/cli so `corvint mcp` shares it (AHI-373).
	servers = append(servers, filepath.Join("..", "..", "internal", "mcp", "cli"))
	want := []string{}
	for _, dir := range servers {
		name, call := filepath.Base(dir), "os.Exit(runProfiled("
		if name == "cli" {
			name, call = "corvint-mcp", "return runProfiled("
		}
		want = append(want, name)
		source, err := os.ReadFile(filepath.Join(dir, "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(source, []byte("procprofile.Run(")) || !bytes.Contains(source, []byte(call)) {
			t.Errorf("%s does not apply the profile bracket at its entry point", dir)
		}
	}
	slices.Sort(want)
	if !slices.Equal(p.Servers, want) {
		t.Fatalf("servers %v, want every MCP server %v", p.Servers, want)
	}
	if stdout.String() != capabilityCatalogJSON() || !strings.HasSuffix(stdout.String(), "}\n") {
		t.Fatal("help --json is not the deterministic catalog")
	}
}

// PRB-V0-013 (review of GitHub #1600): a profile destination inside the inspected worktree is
// refused before the read runs, so it can neither create an untracked file that turns a clean
// `affected --path` into a refusal nor truncate a tracked file; the read itself still succeeds.
func TestPRBV0013ProfileInsideWorktreeRefusesBeforeRead(t *testing.T) {
	root := profileCorpusFixture(t)
	arguments := []string{"--root", root, "affected", "--path", "source.md"}
	var plain, plainErr bytes.Buffer
	if code := runProfiledContext(context.Background(), context.Background(), arguments, strings.NewReader(""), &plain, &plainErr, profileKnobLookup(nil)); code != 0 {
		t.Fatalf("unprofiled read: exit %d stderr %.300s", code, plainErr.String())
	}
	tracked, err := os.ReadFile(filepath.Join(root, "source.md"))
	if err != nil {
		t.Fatal(err)
	}
	for knob, destination := range map[string]string{"CORVINT_CPUPROFILE": filepath.Join(root, "cpu.pprof"), "CORVINT_MEMPROFILE": filepath.Join(root, "source.md")} {
		var stdout, stderr bytes.Buffer
		code := runProfiledContext(context.Background(), context.Background(), arguments, strings.NewReader(""), &stdout, &stderr, profileKnobLookup(map[string]string{knob: destination}))
		var refusal struct {
			Code string `json:"code"`
		}
		if code != 2 || stdout.Len() != 0 || json.Unmarshal(stderr.Bytes(), &refusal) != nil || refusal.Code != "profile-invalid" {
			t.Fatalf("%s=%s: exit=%d stdout=%.200q stderr=%.300q", knob, destination, code, &stdout, &stderr)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, "cpu.pprof")); !os.IsNotExist(err) {
		t.Fatalf("refused profile created an untracked file: %v", err)
	}
	if after, _ := os.ReadFile(filepath.Join(root, "source.md")); !bytes.Equal(after, tracked) {
		t.Fatal("refused profile truncated a tracked file")
	}
	var again, againErr bytes.Buffer
	if code := runProfiledContext(context.Background(), context.Background(), arguments, strings.NewReader(""), &again, &againErr, profileKnobLookup(nil)); code != 0 || !bytes.Equal(again.Bytes(), plain.Bytes()) {
		t.Fatalf("read after refusals: exit %d stderr %.300s", code, againErr.String())
	}
}
