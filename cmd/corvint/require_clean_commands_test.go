package main

import (
	"bytes"
	"context"
	"github.com/Beamfall/corvint/internal/testevidencedoc"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// requireCleanHonouring names the verbs that implement --require-clean
// (DIRTY-CACHE-015, AFP-V0-045). Every other verb must refuse it.
var requireCleanHonouring = map[string]bool{"impact": true, "affected": true}

// requireCleanRefusalForms gives every other dispatched verb an invocation
// that reaches its own option handling; the test appends --require-clean.
// noRoot marks verbs dispatched only without a leading --root.
var requireCleanRefusalForms = map[string]struct {
	args   []string
	noRoot bool
}{
	"init": {args: []string{"init"}}, "adopt": {args: []string{"adopt"}},
	"query": {args: []string{"query", "--task", "change main"}}, "feature": {args: []string{"feature", "impact"}},
	"eval":    {args: []string{"eval"}},
	"repro":   {args: []string{"repro", "record"}},
	"mcp":     {args: []string{"mcp"}, noRoot: true},
	"harness": {args: []string{"harness", "event"}},
	"context": {args: []string{"context", "--task", "change main"}},
	"adapter": {args: []string{"adapter", "codex"}, noRoot: true},
	"index":   {args: []string{"index"}}, "batch": {args: []string{"batch"}},
	"docs":      {args: []string{"docs", "nav", "--trusted-project", "--request", "request.json"}},
	"depsource": {args: []string{"depsource", "example.com/m"}}, "necessity": {args: []string{"necessity"}},
	"surprise": {args: []string{"surprise"}}, "answerability": {args: []string{"answerability"}},
	"kernel":        {args: []string{"kernel"}},
	"calibrate":     {args: []string{"calibrate"}},
	"test-evidence": {args: []string{"test-evidence", "file.go"}},
	"test-validity": {args: []string{"test-validity"}},
	"test-plan":     {args: []string{"test-plan", "check", "--plan", "a.json", "--input", "b.json"}},
	"features":      {args: []string{"features"}}, "overview": {args: []string{"overview"}},
	"review":   {args: []string{"review", "--base", "HEAD"}},
	"flows":    {args: []string{"flows", "--manifest", "manifest.json"}},
	"how":      {args: []string{"how", "--source", "e2e", "ship an order"}},
	"breakage": {args: []string{"breakage"}}, "graph": {args: []string{"graph", "imports", "--revision", "HEAD"}},
	"changes": {args: []string{"changes", "--from", "HEAD", "--to", "HEAD"}},
	"runs":    {args: []string{"runs", "stability", "--runs", "a.json"}},
	"test":    {args: []string{"test", "--base", "HEAD", "--dry-run"}},
	"tests":   {args: []string{"tests", "lint", "--base", "HEAD"}},
}

// requireCleanParseWitness covers verbs whose CLI diagnostic names no argv
// element (CF-V0-024) and is the same for a missing input file: their parser
// must accept the form and refuse it once --require-clean is appended.
var requireCleanParseWitness = map[string]func([]string) error{
	"test-evidence": func(args []string) error { _, _, err := testevidencedoc.Parse(args[1:]); return err },
}

// DIRTY-CACHE-016 (V1-1174): every dispatched verb and the feature form
// either honours --require-clean or refuses it with exit 2 and a
// diagnostic the flag itself causes; none accepts and ignores it.
func TestRequireCleanHonouredOrRefusedByEveryCommand(t *testing.T) {
	root := impactCLIRepository(t)
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	invoke := func(noRoot bool, args ...string) (int, string, string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if !noRoot {
			args = append([]string{"--root", root}, args...)
		}
		var stdout, stderr bytes.Buffer
		code := runContext(ctx, args, &forbiddenImpactReader{}, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	refuses := func(noRoot bool, form ...string) {
		t.Helper()
		for index, value := range form {
			if value == "HEAD" {
				form[index] = head
			}
		}
		_, plainOut, plainErr := invoke(noRoot, form...)
		before := treeDigest(t, root)
		code, stdout, stderr := invoke(noRoot, append(slices.Clone(form), "--require-clean")...)
		// The refusal must be caused by the flag: it names the flag, or the
		// diagnostic differs from the same invocation without it.
		caused := strings.Contains(stdout+stderr, "require-clean") || stdout+stderr != plainOut+plainErr
		if parse := requireCleanParseWitness[form[0]]; parse != nil {
			caused = parse(form) == nil && parse(append(slices.Clone(form), "--require-clean")) != nil
		}
		failed := stdout == "" || strings.Contains(stdout, `"state":"ERROR"`)
		if code != 2 || !failed || !caused {
			t.Errorf("%v --require-clean: exit=%d stdout=%q stderr=%q (without the flag: %q %q)", form, code, stdout, stderr, plainOut, plainErr)
		}
		if treeDigest(t, root) != before {
			t.Errorf("%v wrote repository state", form)
		}
	}
	listed := slices.Sorted(maps.Keys(requireCleanRefusalForms))
	for command := range requireCleanHonouring {
		listed = append(listed, command)
	}
	if want := slices.Sorted(slices.Values(topLevelCommands)); !slices.Equal(slices.Sorted(slices.Values(listed)), want) {
		t.Fatalf("every dispatched verb needs a --require-clean form or an honouring entry:\nlisted %v\nwant   %v", slices.Sorted(slices.Values(listed)), want)
	}
	for _, command := range slices.Sorted(maps.Keys(requireCleanRefusalForms)) {
		if command == "adapter" {
			continue
		}
		form := requireCleanRefusalForms[command]
		refuses(form.noRoot, slices.Clone(form.args)...)
	}
	// The host adapter fails open by contract (exit 0): an unexpected trailing
	// argument degrades with unsupported-hook-event instead of being ignored.
	for _, form := range [][]string{requireCleanRefusalForms["adapter"].args, {"adapter", "claude-code", "stop"}} {
		adapter := func(args []string) string {
			var stdout bytes.Buffer
			if code := runContext(context.Background(), args, strings.NewReader("{}"), &stdout, &bytes.Buffer{}); code != 0 {
				t.Fatalf("%v: exit=%d", args, code)
			}
			return stdout.String()
		}
		if plain, flagged := adapter(form), adapter(append(slices.Clone(form), "--require-clean")); strings.Contains(plain, "unsupported-hook-event") || !strings.Contains(flagged, "unsupported-hook-event") {
			t.Errorf("%v --require-clean was not refused: %q (without the flag: %q)", form, flagged, plain)
		}
	}
	// The honouring verbs act on the dirty worktree instead of ignoring the flag.
	if code, stdout, _ := invoke(false, "impact", "--require-clean", "pkg/main.go"); code != 0 || !strings.Contains(stdout, `"reason":"mixed-worktree"`) {
		t.Errorf("impact did not abstain: exit=%d stdout=%s", code, stdout)
	}
	if code, stdout, stderr := invoke(false, "affected", "--require-clean"); code != 2 || stdout != "" || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") {
		t.Errorf("affected did not refuse: exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
