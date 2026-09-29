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
	timer := time.NewTimer(25 * time.Second)
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
	version, e := command(c.Host, "--version")
	if e != nil {
		return e
	}
	version = strings.TrimSpace(version)
	if version != "opencode v2.0.18" {
		return errors.New("unqualified UI host: " + version)
	}
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
		"AGENTS.md":        "# Inspector fixture\nUse Add in add.go for addition.\n", ".gitignore": "opencode.json\n.corvint/\n.context-corvint/\n",
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
	if e = writeJSON(repo+"/opencode.json", Object{"plugins": []Object{{"package": fileURL(filepath.Join(c.Source, "integrations/opencode/src")), "options": Object{"corvintBinary": c.Corvint, "queryTimeoutMs": 10000}}}}); e != nil {
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
		{input: "\x1b[17~", label: "sidebar", has: []string{"Corvint context"}},
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
		{input: "3", label: "verification", has: []string{"PASS · unit"}},
	}); e != nil {
		return e
	}
	if e = clickText(terminal, root, "PASS · unit", 40); e != nil {
		return e
	}
	if e = step("", "recorded output", []string{"COCKPIT_PROOF_WITNESS"}, nil, 0); e != nil {
		return e
	}
	if e = saveFrame(root, "cockpit-proof"); e != nil {
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
		{input: "e", label: "governing context", has: []string{"Corvint · Evidence", "c Change"}},
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
	if e = step("3", "old result stale", []string{"STALE · unit"}, []string{"PASS · unit"}, 0); e != nil {
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
	checks := []string{"native-sidebar", "context-rpc", "evidence-panel", "pinned-source", "keyboard-source-selection", "narrow-width", "keyboard-gaps", "native-frame-capture", "cited-line", "source-scroll", "filter", "empty-filter", "dialog-focus", "pointer-open", "short-height", "source-invalidation", "cold-cache-plain-source", "explicit-syntax-opt-in", "cockpit-files", "cockpit-impact", "cockpit-proof", "cockpit-unsatisfied-workflow", "cockpit-base-dialog", "cockpit-narrow", "cockpit-context", "cockpit-stale-check", "interruption-no-descendants"}
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
