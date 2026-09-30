package opencodequalification

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type frame struct {
	data  Object
	text  string
	width int
}

func currentFrame(root string) frame {
	x, e := readObject(root + "/screen.json")
	if e != nil {
		return frame{}
	}
	lines := []string{}
	for _, line := range array(x["lines"]) {
		s := ""
		for _, span := range array(line) {
			s += str(object(span)["text"])
		}
		lines = append(lines, s)
	}
	return frame{x, strings.Join(lines, "\n"), int(number(x["width"]))}
}
func waitFrame(ctx context.Context, root, label string, has, absent []string, width int) error {
	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		f := currentFrame(root)
		valid := len(f.data) > 0 && (width == 0 || f.width == width)
		for _, s := range has {
			valid = valid && strings.Contains(f.text, s)
		}
		for _, s := range absent {
			valid = valid && !strings.Contains(f.text, s)
		}
		if valid {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			_ = os.WriteFile(root+"/failed-frame.txt", []byte(f.text), 0600)
			return errors.New("native UI witness missing: " + label)
		case <-ticker.C:
		}
	}
}
func waitFrameSize(ctx context.Context, root string, width, height int) error {
	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		f := currentFrame(root)
		if f.width == width && int(number(f.data["height"])) == height {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("native frame dimensions not observed: requested %dx%d, observed %dx%d", width, height, f.width, int(number(f.data["height"])))
		case <-ticker.C:
		}
	}
}
func saveFrame(root, name string) error {
	f := currentFrame(root)
	if e := writeJSON(root+"/"+name+".json", f.data); e != nil {
		return e
	}
	return os.WriteFile(root+"/"+name+".txt", []byte(f.text), 0600)
}
func clickText(t *terminal, root, value string, before int) error {
	for y, line := range strings.Split(currentFrame(root).text, "\n") {
		if col := strings.Index(line, value); col >= 0 && (before == 0 || col < before) {
			return t.send(fmt.Sprintf("\x1b[<0;%d;%dM\x1b[<0;%d;%dm", col+1, y+1, col+1, y+1))
		}
	}
	return errors.New("clickable text not visible: " + value)
}
func appendText(path, text string) error {
	f, e := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = f.WriteString(text)
	return e
}
func Inspector(ctx context.Context, c Config) (failure error) {
	if e := mkdir(c.Output); e != nil {
		return e
	}
	root, e := os.MkdirTemp(c.Output, "ui-")
	if e != nil {
		return e
	}
	repo, helper := root+"/repo", root+"/helper"
	for _, p := range []string{repo, helper, repo + "/examples"} {
		if e = mkdir(p); e != nil {
			return e
		}
	}
	env := []string{}
	for _, k := range []string{"PATH", "HOME", "USER", "TMPDIR", "SHELL"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	for key, name := range map[string]string{"CONFIG": "config", "DATA": "data", "CACHE": "cache", "STATE": "state"} {
		if e = mkdir(root + "/" + name); e != nil {
			return e
		}
		env = append(env, "XDG_"+key+"_HOME="+root+"/"+name)
	}
	env = append(env, "TERM=xterm-256color", "OPENCODE_DISABLE_AUTOUPDATE=1")
	command := func(args ...string) (string, error) { return capture(ctx, repo, env, args) }
	version, e := observedHostVersion(func(int) (string, error) { return command(c.Host, "--version") })
	if e != nil {
		return e
	}
	hostVersion, e := ParseSupportedHostVersion(version)
	if e != nil {
		return fmt.Errorf("unqualified UI host: %w", e)
	}
	c.HostVersion = hostVersion
	if _, e = command("git", "init", "-q"); e != nil {
		return e
	}
	multiply := "package inspector\n\n"
	for i := 0; i < 35; i++ {
		multiply += fmt.Sprintf("// Before citation %02d\n", i)
	}
	multiply += "// Multiply preserves the NATIVE_MULTIPLY_WITNESS contract.\nfunc Multiply(a, b int) int { return a * b }\n"
	for i := 0; i < 45; i++ {
		multiply += fmt.Sprintf("// After citation %02d\n", i)
	}
	for name, text := range map[string]string{
		"go.mod":      "module example.com/inspector\n\ngo 1.27.1\n",
		"add.go":      "package inspector\n\n// Add preserves the NATIVE_INSPECTOR_WITNESS contract.\nfunc Add(a, b int) int { return a + b }\n",
		"multiply.go": multiply, "examples/multiply.go": "package examples\n\n// Multiply uses NATIVE_SECOND_WITNESS.\nfunc Multiply(a, b int) int { return a * b }\n",
		"multiply_test.go": "package inspector\n\nimport \"testing\"\n\nfunc TestMultiply(t *testing.T) { if Multiply(3, 4) != 12 { t.Fatal(\"wrong product\") }; t.Log(\"COCKPIT_PROOF_WITNESS\") }\n",
		"AGENTS.md":        "# Inspector fixture\nUse Add in add.go for addition.\n", ".gitignore": "opencode.json\n.corvint/\n.context-corvint/\n.taskman/\n",
	} {
		if e = os.WriteFile(filepath.Join(repo, name), []byte(text), 0600); e != nil {
			return e
		}
	}
	commit := func(message string) error {
		_, e := command("git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", message)
		return e
	}
	if _, e = command("git", "add", "."); e != nil {
		return e
	}
	if e = commit("fixture"); e != nil {
		return e
	}
	base, e := command("git", "rev-parse", "HEAD")
	if e != nil {
		return e
	}
	key := strings.Repeat("c", 64)
	if e = writeJSON(root+"/verification-plan.json", Object{"base": strings.TrimSpace(base), "intents": []string{"AGENTS.md"}, "checks": []Object{{"id": "unit", "argv": []string{"go", "test", "-v", "./..."}, "timeoutSeconds": 60}}}); e != nil {
		return e
	}
	if _, e = command(c.Corvint, "dogfood", "begin", "--plan", root+"/verification-plan.json", "--session-key", key); e != nil {
		return e
	}
	if e = appendText(repo+"/multiply.go", "// Committed change for the cockpit witness.\n"); e != nil {
		return e
	}
	if _, e = command("git", "add", "multiply.go"); e != nil {
		return e
	}
	if e = commit("change"); e != nil {
		return e
	}
	if _, e = command(c.Corvint, "dogfood", "verify", "--session-key", key, "--check", "unit"); e != nil {
		return e
	}
	if _, e = command(c.Corvint, "index", "--if-stale"); e != nil {
		return e
	}
	if e = mkdir(repo + "/.taskman"); e != nil {
		return e
	}
	for _, name := range []string{"queue.json", "policy.json"} {
		bytes, readErr := os.ReadFile(filepath.Join(c.Source, "internal/tasks/cli/testdata/external-agents", name))
		if readErr != nil {
			return readErr
		}
		if e = os.WriteFile(filepath.Join(repo, ".taskman", name), bytes, 0600); e != nil {
			return e
		}
	}
	tasksBinary := root + "/corvint-tasks"
	if _, e = capture(ctx, c.Source, append(env, "GOCACHE="+root+"/go-cache"), []string{"go", "build", "-o", tasksBinary, "./cmd/corvint-tasks"}); e != nil {
		return e
	}
	if _, e = command(tasksBinary, "init", "--role", "OWNER", "--request-id", "opencode-ui-witness-init"); e != nil {
		return e
	}
	payload, e := os.ReadFile(filepath.Join(c.Source, "internal/tasks/cli/testdata/external-agents/ticket-create.json"))
	if e != nil {
		return e
	}
	// UI-only fixture delay keeps a transient loading frame observable. Each read delegates to the real built Tasks binary.
	delayedTasks := helper + "/tasks-ui-delay"
	if e = os.WriteFile(delayedTasks, []byte(fmt.Sprintf("#!/bin/sh\nsleep 0.25\nexec %s \"$@\"\n", quoted(tasksBinary))), 0700); e != nil {
		return e
	}
	if e = writeJSON(repo+"/opencode.json", Object{"plugins": []Object{{"package": fileURL(c.Source + "/integrations/opencode/src"), "options": Object{"corvintBinary": c.Corvint, "tasksBinary": delayedTasks, "queryTimeoutMs": 10000}}}}); e != nil {
		return e
	}
	if e = os.WriteFile(helper+"/tui.tsx", []byte(template(inspectorTemplate, map[string]string{"LOG": root + "/witness.jsonl", "SCREEN": root + "/screen.json", "ROOT": repo})), 0600); e != nil {
		return e
	}
	if e = mkdir(root + "/config/opencode"); e != nil {
		return e
	}
	if e = writeJSON(root+"/config/opencode/cli.json", Object{"plugins": []string{helper}, "theme": Object{"mode": c.Theme}, "mouse": true}); e != nil {
		return e
	}
	terminal, e := startTerminal(ctx, c, root, repo, env)
	if e != nil {
		return e
	}
	defer func() {
		if e := terminal.close(root); failure == nil {
			failure = e
		}
	}()
	step := func(input, label string, has, absent []string, width int) error {
		if input != "" {
			if e := terminal.send(input); e != nil {
				return e
			}
		}
		return waitFrame(ctx, root, label, has, absent, width)
	}
	type action struct {
		input, label string
		has, absent  []string
		width        int
		save         string
	}
	drive := func(actions []action) error {
		for _, a := range actions {
			if e := step(a.input, a.label, a.has, a.absent, a.width); e != nil {
				return e
			}
			if a.save != "" {
				if e := saveFrame(root, a.save); e != nil {
					return e
				}
			}
		}
		return nil
	}
	if e = drive([]action{
		{label: "home", has: []string{"Ask anything"}},
		{input: "\x1b[17~", label: "sidebar", has: []string{"Corvint", "Work", "Evidence", "0/0 completed", "0 draft", "0 held", "0 archived"}},
	}); e != nil {
		return e
	}
	if e = clickText(terminal, root, "Open Work queue", 0); e != nil {
		return e
	}
	if e = step("", "native empty Work queue", []string{"0/0 completed", "No open tasks supplied"}, nil, 0); e != nil {
		return e
	}
	if e = saveFrame(root, "task-empty"); e != nil {
		return e
	}
	if _, e = command(tasksBinary, "ticket", "create", "--request-id", "opencode-ui-witness-ticket", "--payload", strings.TrimSpace(string(payload))); e != nil {
		return e
	}
	if e = step("r", "empty queue refresh", []string{"Reading Corvint Tasks"}, []string{"0/0 completed", "Open tickets"}, 0); e != nil {
		return e
	}
	if e = step("", "created fixture task", []string{"0/1 completed", "APP-0001"}, []string{"Reading Corvint Tasks"}, 0); e != nil {
		return e
	}
	if e = drive([]action{
		{label: "sidebar task action", has: []string{"Corvint · Work queue", "APP-0001"}, save: "task-sidebar"},
		{input: "c", label: "sidebar task return", has: []string{"Corvint · Change"}},
		{input: "\x1b[18~", label: "context evidence", has: []string{"Ready", "2 of 2 locations"}},
		{input: "f", label: "wide master detail", has: []string{"Why included:"}, width: 160},
		{input: "\x1b[B\r", label: "selected cited source", has: []string{"NATIVE_MULTIPLY_WITNESS", "cited line 39"}},
	}); e != nil {
		return e
	}
	for _, p := range []string{root + "/data/opentui/tree-sitter/languages/*", root + "/data/opentui/tree-sitter/queries/*"} {
		files, _ := filepath.Glob(p)
		if len(files) > 0 {
			return errors.New("plain source reading fetched parser into cold cache")
		}
	}
	if e = saveFrame(root, "plain-terminal"); e != nil {
		return e
	}
	if e = drive([]action{
		{input: "h", label: "explicit syntax opt-in", has: []string{"Syntax enabled"}, save: "wide-terminal"},
		{input: "\x1b[5~", label: "source scroll", has: []string{"Before citation 05"}},
		{input: "l", label: "return to cited line", has: []string{"NATIVE_MULTIPLY_WITNESS"}},
	}); e != nil {
		return e
	}
	if c.InterruptProbe {
		if e = writeJSON(c.Output+"/ready.json", Object{"pid": os.Getpid(), "run": root}); e != nil {
			return e
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if e = drive([]action{
		{input: "\t/", label: "search dialog", has: []string{"Find evidence"}},
		{input: "rgx/\t\x1b", label: "cancel isolation", has: []string{"2 of 2 locations"}, absent: []string{"Find evidence"}},
		{input: "/", label: "reopen search", has: []string{"Find evidence"}},
		{input: "rgx/nonexistent", label: "dialog focus", has: []string{"rgx/nonexistent", "Find evidence"}},
		{input: "\r", label: "empty filter", has: []string{"No matching evidence."}},
		{input: "x", label: "clear filter", has: []string{"2 of 2 locations"}},
		{input: "/", label: "second search", has: []string{"Find evidence"}},
		{input: "examples\r", label: "matching filter", has: []string{"1 of 2 locations"}, absent: []string{"NATIVE_MULTIPLY_WITNESS"}},
	}); e != nil {
		return e
	}
	if e = clickText(terminal, root, "multiply.go:4", 0); e != nil {
		return e
	}
	if e = step("", "pointer source", []string{"NATIVE_SECOND_WITNESS"}, nil, 0); e != nil {
		return e
	}
	if e = step("x\x1b[B\r", "selection after filter", []string{"NATIVE_MULTIPLY_WITNESS"}, nil, 0); e != nil {
		return e
	}
	if e = terminal.resize(72, 24); e != nil {
		return e
	}
	if e = drive([]action{
		{label: "short narrow source", has: []string{"NATIVE_MULTIPLY_WITNESS", "Tab list/source"}, width: 72, save: "narrow-terminal"},
		{input: "\t", label: "compact list", has: []string{"2 of 2 locations"}},
		{input: "\x1b[A\r", label: "narrow source selection", has: []string{"NATIVE_SECOND_WITNESS"}},
		{input: "g", label: "gaps", has: []string{"Gaps and limitations"}, save: "gaps-terminal"},
		{input: "i", label: "identity", has: []string{"Evidence details", "Receipt:"}},
		{input: "\x1b[18~", label: "fresh query", has: []string{"Ready", "Receipt:"}},
		{input: "\t\t", label: "source invalidation", has: []string{"Enter Open pinned source"}, absent: []string{"NATIVE_SECOND_WITNESS"}},
	}); e != nil {
		return e
	}
	if e = terminal.resize(160, 48); e != nil {
		return e
	}
	if e = drive([]action{
		{input: "\x1b[19~", label: "change cockpit", has: []string{"Corvint · Change", "1 files", "obligations open"}},
		{input: "f", label: "wide change details", has: []string{"Recorded intent scopes"}, save: "cockpit-files"},
		{input: "\r", label: "dependency witness", has: []string{"Why selected:", "multiply_test.go"}, save: "cockpit-impact"},
		{input: "4\t", label: "attention limitations", has: []string{"Scope limitations", "Affected selection is advisory", "Verification records are caller-owned"}, save: "cockpit-attention"},
		{input: "3", label: "verification", has: []string{"Passed · recorded commit · unit"}},
	}); e != nil {
		return e
	}
	if e = clickText(terminal, root, "Passed · recorded commit · unit", 40); e != nil {
		return e
	}
	if e = step("", "recorded output", []string{"COCKPIT_PROOF_WITNESS"}, nil, 0); e != nil {
		return e
	}
	if e = saveFrame(root, "cockpit-proof"); e != nil {
		return e
	}
	if e = drive([]action{
		{input: "t", label: "task metrics", has: []string{"Corvint · Work queue", "0/1 completed", "APP-0001"}, save: "task-metrics"},
		{input: "\r", label: "task detail", has: []string{"Acceptance criteria", "The repository verify target passes."}, save: "task-detail"},
		{input: "r", label: "task refresh loading", has: []string{"Reading Corvint Tasks"}, absent: []string{"0/1 completed", "Open tickets"}, save: "task-loading"},
		{label: "task refresh settled", has: []string{"0/1 completed", "APP-0001"}, absent: []string{"Reading Corvint Tasks"}},
	}); e != nil {
		return e
	}
	if e = os.Rename(tasksBinary, tasksBinary+".unavailable"); e != nil {
		return e
	}
	defer func() { _ = os.Rename(tasksBinary+".unavailable", tasksBinary) }()
	if e = step("r", "native Tasks unavailable", []string{"r Read Work queue", "Work queue unavailable", "task-manager-command-failed"}, []string{"0/1 completed", "Open tickets"}, 0); e != nil {
		return e
	}
	if e = saveFrame(root, "task-unavailable"); e != nil {
		return e
	}
	if e = os.Rename(tasksBinary+".unavailable", tasksBinary); e != nil {
		return e
	}
	if e = step("r", "Tasks recovery", []string{"0/1 completed", "APP-0001"}, []string{"Work queue unavailable"}, 0); e != nil {
		return e
	}
	if e = clickText(terminal, root, "s Focus task", 0); e != nil {
		return e
	}
	if e = drive([]action{
		{label: "pointer workbench focus", has: []string{"Corvint · Work", "APP-0001", "criterion-specific proof 0 observed"}, save: "workbench-overview"},
		{input: "2", label: "workbench proof", has: []string{"Acceptance criteria", "UNOBSERVED", "The repository verify target passes."}, save: "workbench-proof"},
		{input: "3", label: "work blockers", has: []string{"Native task blockers", "Eligibility: UNKNOWN"}, save: "work-blockers"},
		{input: "4", label: "work gates", has: []string{"Native Tasks gate summary: NOT_OBSERVED", "verify · result NOT_OBSERVED"}, save: "work-gates"},
		{input: "m", label: "workbench doctor", has: []string{"Native integration: UNQUALIFIED", "Qualification action"}, save: "workbench-doctor"},
		{input: "o", label: "workbench economics", has: []string{"OpenCode session cost:", "Cost per verified criterion: NOT_OBSERVED"}, save: "workbench-cost"},
		{input: "u", label: "workbench sessions", has: []string{"OpenCode session family", "Worktree"}, save: "workbench-sessions"},
		{input: "d", label: "keyboard Doctor transition", has: []string{"Native integration: UNQUALIFIED"}},
		{input: "1", label: "task next actions", has: []string{"Next steps", "Inspect Criteria"}},
		{input: "\x1b[B\r", label: "keyboard next action", has: []string{"Acceptance criteria", "UNOBSERVED"}, save: "work-action-criteria"},
		{input: "t", label: "workbench to tasks", has: []string{"Corvint · Work queue", "APP-0001"}},
		{input: "c", label: "return from tasks", has: []string{"Corvint · Change", "1 files"}},
	}); e != nil {
		return e
	}
	if e = step("\x1b[20~", "reopen focused Work", []string{"Corvint · Work", "APP-0001", "ticket revision 1"}, nil, 0); e != nil {
		return e
	}
	focusedBinding := ""
	for _, line := range strings.Split(currentFrame(root).text, "\n") {
		if strings.Contains(line, "Queue ") && strings.Contains(line, "ticket revision") {
			focusedBinding = strings.TrimSpace(line[strings.Index(line, "Queue ") : strings.Index(line, "ticket revision 1")+len("ticket revision 1")])
		}
	}
	if focusedBinding == "" {
		return errors.New("focused task binding not visible")
	}
	if e = os.WriteFile(root+"/focused-binding.txt", []byte(focusedBinding), 0600); e != nil {
		return e
	}
	for _, point := range []struct {
		label string
		has   []string
	}{
		{"2 Criteria", []string{"Acceptance criteria", "UNOBSERVED"}},
		{"3 Blockers", []string{"Native task blockers", "Eligibility: UNKNOWN"}},
		{"4 Gates", []string{"verify · result NOT_OBSERVED", "Native Tasks completion: NOT_OBSERVED"}},
		{"m More", []string{"d Doctor", "o Cost", "u Sessions"}},
		{"o Cost", []string{"OpenCode session cost:"}},
		{"d Doctor", []string{"Native integration: UNQUALIFIED"}},
		{"o Cost", []string{"OpenCode session cost:"}},
		{"u Sessions", []string{"OpenCode session family"}},
	} {
		if e = clickText(terminal, root, point.label, 0); e != nil {
			return e
		}
		if e = step("", "pointer "+point.label, point.has, nil, 0); e != nil {
			return e
		}
	}
	if e = terminal.resize(72, 24); e != nil {
		return e
	}
	if e = drive([]action{
		{input: "1", label: "compact Work", has: []string{"Corvint · Work", "APP-0001", "Next steps"}, width: 72, save: "work-compact"},
		{input: "\r", label: "keyboard queue action", has: []string{"Corvint · Work queue", "APP-0001"}},
		{input: "\r", label: "compact task detail", has: []string{"OPEN · UNKNOWN", "Acceptance criteria"}, save: "task-compact"},
		{input: "s", label: "compact task focus", has: []string{"focused task", "1 Task", "m More"}},
	}); e != nil {
		return e
	}
	if e = terminal.resize(72, 18); e != nil {
		return e
	}
	if e = waitFrameSize(ctx, root, 72, 18); e != nil {
		return e
	}
	if e = drive([]action{
		{input: "2", label: "short Criteria", has: []string{"Acceptance criteria", "UNOBSERVED"}, width: 72, save: "work-short"},
		{input: "4", label: "short Gates", has: []string{"verify · result NOT_OBSERVED"}, save: "work-short-gates"},
		{input: "c", label: "Work to Change", has: []string{"Corvint · Change", "Checks"}},
		{input: "3", label: "Change checks", has: []string{"Passed · recorded commit · unit"}},
		{input: "e", label: "Change to Evidence", has: []string{"Evidence", "c Change"}},
		{input: "\x1b[20~", label: "return focused task", has: []string{"Corvint · Work", "APP-0001"}},
	}); e != nil {
		return e
	}
	if e = terminal.resize(160, 48); e != nil {
		return e
	}
	if e = step("1", "retained task receipt", []string{focusedBinding}, nil, 160); e != nil {
		return e
	}
	if e = saveFrame(root, "work-return"); e != nil {
		return e
	}
	if _, e = command(tasksBinary, "ticket", "refine", "--target", "APP-0001", "--expected-revision", "1", "--role", "OWNER", "--request-id", "opencode-ui-witness-drift", "--payload", `{"body":"Native fixture receipt drift witness."}`); e != nil {
		return e
	}
	if e = drive([]action{
		{input: "t", label: "queue before receipt refresh", has: []string{"Corvint · Work queue", "APP-0001"}},
		{input: "r", label: "queue new receipt", has: []string{"Reading Corvint Tasks"}},
		{label: "new receipt settled", has: []string{"APP-0001", "0/1 completed"}, absent: []string{"Reading Corvint Tasks"}},
		{input: "\x1b[20~", label: "stale focus", has: []string{"Task queue changed", "stale"}, save: "work-stale"},
		{input: "t", label: "reselect task", has: []string{"Corvint · Work queue", "APP-0001"}},
		{input: "s", label: "new bound focus", has: []string{"focused task", "ticket revision 2"}, save: "work-rebound"},
	}); e != nil {
		return e
	}
	if e = step("c", "return Change for existing checks", []string{"Corvint · Change"}, nil, 0); e != nil {
		return e
	}
	if e = drive([]action{{input: "b", label: "base dialog", has: []string{"Compare from revision"}}, {input: "HEAD\x1b", label: "base cancel", absent: []string{"Compare from revision"}}}); e != nil {
		return e
	}
	if e = terminal.resize(72, 24); e != nil {
		return e
	}
	if e = drive([]action{
		{label: "compact proof", has: []string{"Tab list/details"}, width: 72, save: "cockpit-narrow"},
		{input: "e", label: "governing context", has: []string{"Evidence", "c Change"}},
		{input: "c", label: "return to cockpit", has: []string{"Corvint · Change", "1 files"}},
	}); e != nil {
		return e
	}
	if e = appendText(repo+"/multiply.go", "// External edit invalidates recorded check.\n"); e != nil {
		return e
	}
	if e = step("r", "refresh edit", []string{"1 files"}, nil, 0); e != nil {
		return e
	}
	if e = step("3", "old result stale", []string{"Stale result · unit"}, []string{"Passed · recorded commit · unit"}, 0); e != nil {
		return e
	}
	if e = saveFrame(root, "cockpit-stale"); e != nil {
		return e
	}
	if e = terminal.close(root); e != nil {
		return e
	}
	interruptedOut := root + "/interrupted"
	interruption, e := interruptedCommand(ctx, c.Source, []string{c.Self, "--inspector", "--source", c.Source, "--host", c.Host, "--corvint", c.Corvint, "--output", interruptedOut, "--theme", c.Theme, "--interrupt-probe"}, root+"/interrupted-command", func() bool {
		x, e := readObject(interruptedOut + "/ready.json")
		return e == nil && str(x["run"]) != ""
	})
	if e != nil {
		return e
	}
	id, e := identities(ctx, c, false)
	if e != nil {
		return e
	}
	checks := []string{"native-sidebar", "context-rpc", "evidence-panel", "pinned-source", "keyboard-source-selection", "narrow-width", "keyboard-gaps", "native-frame-capture", "cited-line", "source-scroll", "filter", "empty-filter", "dialog-focus", "pointer-open", "short-height", "source-invalidation", "cold-cache-plain-source", "explicit-syntax-opt-in", "cockpit-files", "cockpit-impact", "cockpit-proof", "cockpit-unsatisfied-workflow", "cockpit-base-dialog", "cockpit-narrow", "cockpit-context", "cockpit-stale-check", "tasks-sidebar", "tasks-page", "tasks-detail", "tasks-refresh", "workbench-focus", "workbench-proof", "work-blockers", "work-gates-declared-unknown-results", "work-keyboard-actions", "work-pointer-tabs", "tasks-compact", "tasks-empty-loading-unavailable-recovery", "work-receipt-drift-rebind", "work-compact-short", "work-change-evidence-bound-return", "workbench-doctor", "workbench-cost", "workbench-sessions", "interruption-no-descendants"}
	report := Object{"profile": "corvint-opencode-inspector-witness/0", "result": "PASS", "host": version, "theme": c.Theme, "sourceCommit": id.SourceCommit, "sourceSHA256": id.SourceFiles, "harnessSHA256": id.HarnessSHA256, "qualificationBinarySHA256": id.QualificationBinarySHA256, "checks": checks, "root": root, "interruption": interruption, "authority": "NONE", "qualification": "UI witness only; does not promote integration support"}
	if c.Theme == "dark" {
		light := c
		light.Theme = "light"
		light.Output = c.Output + "/light"
		if e = Inspector(ctx, light); e != nil {
			return e
		}
		checks = append(checks, "light-theme")
		report["checks"] = checks
		report["lightReport"] = light.Output + "/report.json"
	}
	if e = writeJSON(c.Output+"/report.json", report); e != nil {
		return e
	}
	b, _ := jsonBytes(Object{"result": "PASS", "report": c.Output + "/report.json"})
	fmt.Println(string(b))
	return nil
}
