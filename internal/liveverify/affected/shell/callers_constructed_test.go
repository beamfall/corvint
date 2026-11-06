// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AZU-V0-8901: new literal/constructed foreign callers fail closed; listing
// them makes completeness hold. An unrelated directory is no caller.
func TestConstructedForeignCallers_AZU_V0_8901(t *testing.T) {
	for _, body := range []string{
		`var command = "run/" + name + ".sh"`,
		`var command = "run" + "/" + name + ".sh"`,
		"cmd := fmt.Sprintf(`run/%s.sh`, name)",
		`cmd := filepath.Join("run", name + ".sh")`,
		`cmd := filepath.Join("run/", name + ".sh")`,
		`cmd := filepath.Join("./run/", name + ".sh")`,
		"cmd := filepath.Join(`run`, name + \".sh\")",
		`subprocess.run([f"run/{name}.sh"])`,
		`exec("run/$name.sh")`,
		"exec.Command(\"sh\", \"-c\", \"run/`printf %s run`.sh\").Run()",
		"exec.Command(\"sh\", \"-c\", \"run/`echo run`.sh\").Run()",
		`exec("run/run.sh")`,
	} {
		t.Run(body, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "run/run.sh", "echo ok\n")
			write(t, root, "foreign/caller.go", body)
			declared := map[string]ForeignCallers{"run/run.sh": {Callers: []string{"foreign/caller.go"}, Reason: "computed caller"}}
			if !foreignCallersComplete(affected.DiskSource(root), declared, nil) {
				t.Fatal("listed constructed caller refused")
			}
			declared["run/run.sh"] = ForeignCallers{Callers: []string{"other.go"}, Reason: "missing caller"}
			if foreignCallersComplete(affected.DiskSource(root), declared, nil) {
				t.Fatal("undeclared caller accepted")
			}
		})
	}
	if constructedForeignReference([]byte(`filepath.Join("elsewhere",name)`), "run/run.sh") {
		t.Fatal("unrelated directory matched")
	}
	if constructedForeignReference([]byte("# `run/other.sh`"), "run/run.sh") {
		t.Fatal("literal backtick-delimited path matched as a construction")
	}
	if constructedForeignReference([]byte("'run/other.sh: keep only `#` comments'"), "run/run.sh") {
		t.Fatal("backticks in prose after a literal path matched as a construction")
	}
}

// AZU-V0-8901: a root callee has computed callers too. An incomplete
// declaration must keep uncertainty even while its known callers stay linked.
func TestRootConstructedForeignCallers_AZU_V0_8901(t *testing.T) {
	for _, expression := range []string{
		`filepath.Join(".", name+".sh")`,
		`"./" + name + ".sh"`,
		`name + ".sh"`,
	} {
		t.Run(expression, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "go.mod", "module example.test/root\n\ngo 1.27\n")
			write(t, root, "run.sh", "echo ok\n")
			write(t, root, "caller/caller.go", "package caller\nvar script = \"run.sh\"\n")
			write(t, root, "caller/caller_test.go", "package caller\n")
			write(t, root, "extra/extra.go", "package extra\nimport (\"os/exec\"; \"path/filepath\")\nvar name string\nvar cmd = exec.Command(\"sh\", "+expression+")\n")
			write(t, root, affected.ShellForeignCallersPath, `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run.sh":{"callers":["caller/caller.go"],"reason":"literal caller"}}}`)
			result := units(t, root)
			unit := unitOf(t, result, "run.sh")
			if !slices.Contains(result.Frontier, FrontierForeignCallersInvalid) || !slices.Contains(unit.Frontier, FrontierForeignInvocation) {
				t.Fatalf("undeclared root caller lost uncertainty: unit=%+v frontier=%v", unit, result.Frontier)
			}
			graph, err := affected.Build(context.Background(), root, golang.New(), New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"run.sh"})
			if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) == 0 {
				t.Fatalf("undeclared root caller certified: %+v", plan)
			}
			if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/root/caller" }) {
				t.Fatalf("known caller dropped: %+v", plan)
			}
		})
	}
}

// AZU-V0-8901: the wrapper's repository-root query resolves a known shell
// target, while querying another directory cannot certify the current root.
func TestDeclaredRootWrapper_AZU_V0_8901(t *testing.T) {
	for _, query := range []string{"git rev-parse --show-toplevel", "env -u GIT_CONFIG_GLOBAL git rev-parse --show-toplevel"} {
		root := t.TempDir()
		write(t, root, "script/worker.sh", "echo ok\n")
		write(t, root, "script/wrapper.sh", "top=$("+query+")\nscript=${top:+$top/script/worker.sh}\nexec \"$script\" \"$@\"\n")
		result := units(t, root)
		wrapper := unitOf(t, result, "script/wrapper.sh")
		if !slices.Contains(wrapper.Imports, "shell:script/worker.sh") {
			t.Fatalf("root target not linked: %+v", wrapper)
		}
	}
	if repositoryRootCommand("git -C elsewhere rev-parse --show-toplevel") {
		t.Fatal("foreign root accepted")
	}
}

// AZU-V0-8901: extending completeness can add an unknown, but must preserve
// caller edges already verified by the legacy literal/shell-expansion check.
func TestConstructedDriftKeepsVerifiedCallers_AZU_V0_8901(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/legacy\n\ngo 1.27\n")
	write(t, root, "run/run.sh", "echo ok\n")
	write(t, root, "caller/caller.go", "package caller\n\nvar script = \"run/run.sh\"\n")
	write(t, root, "caller/caller_test.go", "package caller\n")
	write(t, root, "extra/extra.go", "package extra\nimport \"path/filepath\"\nvar name string\nvar command = filepath.Join(\"run\", name)\n")
	write(t, root, affected.ShellForeignCallersPath, `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{"callers":["caller/caller.go"],"reason":"legacy literal caller"}}}`)
	result := units(t, root)
	unit := unitOf(t, result, "run/run.sh")
	if !slices.Contains(result.Frontier, FrontierForeignCallersInvalid) || !slices.Contains(unit.Frontier, FrontierForeignInvocation) || !slices.Equal(unit.Callers, []string{"caller/caller.go"}) {
		t.Fatalf("lost verified callers or uncertainty: unit=%+v frontier=%v", unit, result.Frontier)
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"run/run.sh"})
	if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/legacy/caller" }) {
		t.Fatalf("legacy caller dropped: %+v", plan)
	}
}

// AZU-V0-004, AZU-V0-8901: a Go raw-string directory join must not
// certify completeness while its undeclared caller's tests stay excluded.
func TestRawStringJoinRetainsUnknown_AZU_V0_8901(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/rawjoin\n\ngo 1.27\n")
	write(t, root, "run/run.sh", "echo ok\n")
	write(t, root, "caller/caller.go", "package caller\nvar script = \"run/run.sh\"\n")
	write(t, root, "caller/caller_test.go", "package caller\n")
	write(t, root, "extra/extra.go", "package extra\nimport (\"os/exec\"; \"path/filepath\")\nvar name string\nvar cmd = exec.Command(\"sh\", filepath.Join(`run`, name+\".sh\"))\n")
	write(t, root, "extra/extra_test.go", "package extra\n")
	write(t, root, affected.ShellForeignCallersPath, `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{"callers":["caller/caller.go"],"reason":"literal caller"}}}`)
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"run/run.sh"})
	if plan.Scope != affected.ScopeUnknown || !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool {
		return u.Detail == FrontierForeignInvocation
	}) {
		t.Fatalf("undeclared raw-string caller certified: %+v", plan)
	}
	if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/rawjoin/caller" }) {
		t.Fatalf("known caller dropped: %+v", plan)
	}
}

// AZU-V0-004, AZU-V0-8901: raw formats and separately concatenated
// directory separators and embedded backticks must not certify an incomplete
// caller declaration.
func TestConstructedDirectoryRetainsUnknown_AZU_V0_8901(t *testing.T) {
	for _, expression := range []string{
		"fmt.Sprintf(`run/%s.sh`, name)",
		`"run" + "/" + name + ".sh"`,
		`filepath.Join("run/", name+".sh")`,
		"\"-c\", \"run/`printf %s run`.sh\"",
		"\"-c\", \"run/`echo run`.sh\"",
	} {
		t.Run(expression, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "go.mod", "module example.test/constructed\n\ngo 1.27\n")
			write(t, root, "run/run.sh", "echo ok\n")
			write(t, root, "caller/caller.go", "package caller\nvar script = \"run/run.sh\"\n")
			write(t, root, "caller/caller_test.go", "package caller\n")
			imports := "import \"os/exec\"\n"
			if expression == "fmt.Sprintf(`run/%s.sh`, name)" {
				imports += "import \"fmt\"\n"
			}
			if expression == `filepath.Join("run/", name+".sh")` {
				imports += "import \"path/filepath\"\n"
			}
			write(t, root, "extra/extra.go", "package extra\n"+imports+"var name = \"run\"\nvar cmd = exec.Command(\"sh\", "+expression+")\n")
			write(t, root, "extra/extra_test.go", "package extra\n")
			write(t, root, affected.ShellForeignCallersPath, `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{"callers":["caller/caller.go"],"reason":"literal caller"}}}`)
			graph, err := affected.Build(context.Background(), root, golang.New(), New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"run/run.sh"})
			if plan.Scope != affected.ScopeUnknown || !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool {
				return u.Detail == FrontierForeignInvocation
			}) {
				t.Fatalf("undeclared constructed caller certified: %+v", plan)
			}
			if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/constructed/caller" }) {
				t.Fatalf("known caller dropped: %+v", plan)
			}
		})
	}
}

// AZU-V0-004, AZU-V0-8901: literal join components are normalized before
// completeness can discharge uncertainty, including unresolved suffixes.
func TestNormalizedJoinRetainsUnknown_AZU_V0_8901(t *testing.T) {
	for _, tc := range []struct{ script, expression string }{
		{"run/run.sh", `filepath.Join(".", "run", name+".sh")`},
		{"run/run.sh", "filepath.Join(`.`, `run`, name+`.sh`)"},
		{"run/run.sh", `filepath.Join("scratch", "..", "run", name+".sh")`},
		{"run/run.sh", `filepath.Join("./scratch/../run/.", name+".sh")`},
		{"run/run.sh", `filepath.Join(".", directory, name+".sh")`},
		{"run/run.sh", `filepath.Join("", directory, name+".sh")`},
		{"café/run.sh", `filepath.Join(".", "caf\xc3\xa9", name+".sh")`},
		{"tools/run/run.sh", `filepath.Join("tools", "run", name+".sh")`},
		{"tools/run/run.sh", `filepath.Join(".", "tools", directory, name+".sh")`},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "go.mod", "module example.test/normalized\n\ngo 1.27\n")
			write(t, root, tc.script, "echo ok\n")
			write(t, root, "caller/caller.go", "package caller\nvar script = "+strconv.Quote(tc.script)+"\n")
			write(t, root, "caller/caller_test.go", "package caller\n")
			write(t, root, "extra/extra.go", "package extra\nimport (\"os/exec\"; \"path/filepath\")\nvar name = \"run\"\nvar directory = \"run\"\nvar cmd = exec.Command(\"sh\", "+tc.expression+")\n")
			write(t, root, "extra/extra_test.go", "package extra\n")
			write(t, root, affected.ShellForeignCallersPath, `{"profile":"corvint-shell-foreign-callers/0","scripts":{`+strconv.Quote(tc.script)+`:{"callers":["caller/caller.go"],"reason":"literal caller"}}}`)
			graph, err := affected.Build(context.Background(), root, golang.New(), New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{tc.script})
			if plan.Scope != affected.ScopeUnknown || !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool {
				return u.Detail == FrontierForeignInvocation
			}) {
				t.Fatalf("undeclared normalized join certified: %+v", plan)
			}
			if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/normalized/caller" }) {
				t.Fatalf("known caller dropped: %+v", plan)
			}
		})
	}
}

// AZU-V0-004, AZU-V0-8901: an unresolved intermediate operand can cancel
// an unrelated literal prefix. Completeness must retain the caller frontier.
func TestTraversalJoinRetainsUnknown_AZU_V0_8901(t *testing.T) {
	for _, operand := range []string{"up", "(up)", "strings.TrimSpace(up)"} {
		t.Run(operand, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "go.mod", "module example.test/traversal\n\ngo 1.27\n")
			write(t, root, "run/run.sh", "echo ok\n")
			write(t, root, "caller/caller.go", "package caller\nvar script = \"run/run.sh\"\n")
			write(t, root, "caller/caller_test.go", "package caller\n")
			imports := "import (\"os/exec\"; \"path/filepath\")\n"
			if strings.Contains(operand, "strings.") {
				imports += "import \"strings\"\n"
			}
			body := "package extra\n" + imports + "var up = \"..\"\nvar name = \"run\"\nvar cmd = exec.Command(\"sh\", filepath.Join(\"scratch\", " + operand + ", \"run\", name+\".sh\"))\n"
			write(t, root, "extra/extra.go", body)
			write(t, root, "extra/extra_test.go", "package extra\n")
			declared := map[string]ForeignCallers{
				"run/run.sh": {Callers: []string{"caller/caller.go"}, Reason: "literal caller"},
			}
			writeDeclaration := func() {
				t.Helper()
				declaration, err := json.Marshal(map[string]any{"profile": ForeignCallersProfile, "scripts": declared})
				if err != nil {
					t.Fatal(err)
				}
				write(t, root, affected.ShellForeignCallersPath, string(declaration))
			}
			writeDeclaration()
			graph, err := affected.Build(context.Background(), root, golang.New(), New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"run/run.sh"})
			if plan.Scope != affected.ScopeUnknown || !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool {
				return u.Detail == FrontierForeignInvocation
			}) {
				t.Fatalf("undeclared traversal caller certified: %+v", plan)
			}
			if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/traversal/caller" }) {
				t.Fatalf("known caller dropped: %+v", plan)
			}
			if foreignCallersComplete(affected.DiskSource(root), declared, nil) {
				t.Fatal("undeclared traversal caller accepted")
			}
			declared["run/run.sh"] = ForeignCallers{Callers: []string{"caller/caller.go", "extra/extra.go"}, Reason: "literal and traversal callers"}
			writeDeclaration()
			if !foreignCallersComplete(affected.DiskSource(root), declared, nil) {
				t.Fatal("declared traversal caller refused")
			}
		})
	}
}

// AZU-V0-8901: a terminal literal suffix may exclude a shell filename only
// when it excludes every supported shell extension, including partial suffixes.
func TestTraversalJoinSuffixBounds_AZU_V0_8901(t *testing.T) {
	for _, expression := range []string{
		`filepath.Join("scratch", up, "run", name+".json")`,
		`filepath.Join("scratch", up, "data.txt")`,
		`filepath.Join("scratch", up, "cmdline")`,
	} {
		if constructedForeignReference([]byte(expression), "run/run.sh") {
			t.Fatalf("non-shell suffix matched: %s", expression)
		}
	}
	for _, tail := range []string{`name+".sh"`, `name+"sh"`, `name+"h"`, `name+".bash"`, `name+".zsh"`, `name+".ksh"`, `name+".bats"`, `name+".SH"`, `name+".sh. "`, `".."`, `"."`, `"data.json/.."`, `"data.json\\.."`, `name`} {
		expression := `filepath.Join("scratch", up, "run", ` + tail + `)`
		for _, script := range []string{"run/a.sh", "run/run.sh"} {
			if !constructedForeignReference([]byte(expression), script) {
				t.Fatalf("possible shell suffix excluded (%s): %s", script, expression)
			}
		}
	}
}

// AZU-V0-004, AZU-V0-8901: completeness caches constructed matches by
// directory. An escaped basename in embedded Python must not disappear when
// another script in the same directory is matched first.
func TestNormalizedJoinSharedDirectoryRetainsUnknown_AZU_V0_8901(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/sharedjoin\n\ngo 1.27\n")
	for _, script := range []string{"run/a.sh", "run/run.sh"} {
		write(t, root, script, "echo ok\n")
	}
	write(t, root, "caller/caller.go", "package caller\nvar scripts = []string{\"run/a.sh\", \"run/run.sh\"}\n")
	write(t, root, "caller/caller_test.go", "package caller\n")
	body := "package extra\nimport \"os/exec\"\nvar cmd = exec.Command(\"python3\", \"-c\", `import os.path, subprocess\nsubprocess.run([os.path.join(\".\", \"run\", \"r\\u0075n.sh\")])`)\n"
	write(t, root, "extra/extra.go", body)
	write(t, root, "extra/extra_test.go", "package extra\n")
	declared := map[string]ForeignCallers{
		"run/a.sh":   {Callers: []string{"caller/caller.go"}, Reason: "literal caller"},
		"run/run.sh": {Callers: []string{"caller/caller.go"}, Reason: "literal caller"},
	}
	declaration, err := json.Marshal(map[string]any{"profile": "corvint-shell-foreign-callers/0", "scripts": declared})
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, affected.ShellForeignCallersPath, string(declaration))
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"run/run.sh"})
	if plan.Scope != affected.ScopeUnknown || !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool {
		return u.Detail == FrontierForeignInvocation
	}) {
		t.Fatalf("undeclared escaped join certified through directory cache: %+v", plan)
	}
	if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/sharedjoin/caller" }) {
		t.Fatalf("known caller dropped: %+v", plan)
	}
	if foreignCallersComplete(affected.DiskSource(root), declared, nil) {
		t.Fatal("directory cache certified an undeclared escaped join")
	}
	for script, callers := range declared {
		if !constructedForeignReference([]byte(body), script) {
			t.Fatalf("constructed match depends on basename: %s", script)
		}
		callers.Callers = append(callers.Callers, "extra/extra.go")
		declared[script] = callers
	}
	if !foreignCallersComplete(affected.DiskSource(root), declared, nil) {
		t.Fatal("declared shared-directory caller refused")
	}
}

// AZU-V0-8901: consume the complete literal prefix, so a leading dot
// does not turn an unrelated directory or another identifier into a caller.
func TestNormalizedJoinBounds_AZU_V0_8901(t *testing.T) {
	for _, expression := range []string{
		`filepath.Join(".", "elsewhere")`,
		`driverJoin(".", value)`,
		`items.join('\\n')`,
		`items.join('\0')`,
		`join("\0", values)`,
	} {
		if constructedForeignReference([]byte(expression), "run/run.sh") {
			t.Fatalf("unrelated join matched: %s", expression)
		}
	}
	for _, expression := range []string{
		`filepath.Join(".", "run", "run.sh")`,
		// A dynamic final operand may be "../run/run" (AZU-V0-8901 review P1).
		`filepath.Join(".", "elsewhere", name+".sh")`,
		`filepath.Join("scratch", "..", "elsewhere", name+".sh")`,
		`joinpath('.', 'run', name)`,
		`filepath.Join(".", "\x72un", name+".sh")`,
	} {
		if !normalizedForeignJoin([]byte(expression), "run/run.sh") {
			t.Fatalf("normalized join missed: %s", expression)
		}
	}
}

// AZU-V0-004, AZU-V0-8901: every supported construction spelling must
// retain foreign-invocation uncertainty for root and nested callees alike,
// without removing selections of already verified literal callers.
func TestConstructedBasenameSpellings_AZU_V0_8901(t *testing.T) {
	type construction struct{ name, expression string }
	spellings := []construction{
		{"backtick", "sh PREFIX`echo run`.sh"},
		{"command", "sh PREFIX$(echo run).sh"},
		{"braced", "sh PREFIX${name}.sh"},
		{"variable", "sh PREFIX$name.sh"},
		{"format", "PREFIX%s.sh"},
	}
	constructions := []construction{
		{"format-spaced-basename", `fmt.Sprintf("PREFIX %s.sh", name)`},
		{"concat", `"PREFIX" + name + ".sh"`},
		{"concat-split-separator", `"DIRECTORY" + "/" + name + ".sh"`},
		{"concat-raw", "`PREFIX` + name + `.sh`"},
		{"join", `filepath.Join("DIRECTORY", name+".sh")`},
		{"join-trailing-slash", `filepath.Join("PREFIX", name+".sh")`},
		{"join-dot-slash", `filepath.Join("./PREFIX", name+".sh")`},
		{"join-raw", "filepath.Join(`DIRECTORY`, name+`.sh`)"},
		{"join-lowercase", `join("DIRECTORY", name+".sh")`},
		{"joinpath", `joinpath("DIRECTORY", name+".sh")`},
	}
	for _, script := range []string{"run.sh", "run/run.sh"} {
		prefix, directory := "", "."
		if script == "run/run.sh" {
			prefix, directory = "run/", "run"
		}
		cases := append([]construction{}, constructions...)
		for _, spelling := range spellings {
			// Exercise quoted command strings, including a command before the
			// path and shell quotes inside the host-language string.
			for _, quote := range []string{"", "'", "\""} {
				command := spelling.expression
				if strings.HasPrefix(command, "sh ") {
					command = "sh " + quote + strings.TrimPrefix(command, "sh ") + quote
				} else {
					command = quote + command + quote
				}
				expression := `"-c", ` + strconv.Quote(command)
				if spelling.name == "format" {
					expression = `"-c", fmt.Sprintf(` + strconv.Quote(command) + `, name)`
				}
				cases = append(cases, construction{spelling.name + "/quoted-" + strconv.Quote(quote), expression})
			}
			if spelling.name != "backtick" {
				expression := "\"-c\", `" + spelling.expression + "`"
				if spelling.name == "format" {
					expression = "\"-c\", fmt.Sprintf(`" + spelling.expression + "`, name)"
				}
				cases = append(cases, construction{spelling.name + "/raw", expression})
			}
		}
		for _, tc := range cases {
			t.Run(script+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				write(t, root, "go.mod", "module example.test/spellings\n\ngo 1.27\n")
				write(t, root, script, "echo ok\n")
				write(t, root, "caller/caller.go", "package caller\nvar script = "+strconv.Quote(script)+"\n")
				write(t, root, "caller/caller_test.go", "package caller\n")
				expression := strings.NewReplacer("PREFIX", prefix, "DIRECTORY", directory).Replace(tc.expression)
				write(t, root, "extra/extra.go", "package extra\nimport (\"os/exec\"; \"path/filepath\"; \"fmt\")\nvar name = \"run\"\nvar cmd = exec.Command(\"sh\", "+expression+")\n")
				write(t, root, "extra/extra_test.go", "package extra\n")
				declaration := map[string]any{"profile": "corvint-shell-foreign-callers/0", "scripts": map[string]ForeignCallers{script: {Callers: []string{"caller/caller.go"}, Reason: "literal caller"}}}
				body, err := json.Marshal(declaration)
				if err != nil {
					t.Fatal(err)
				}
				write(t, root, affected.ShellForeignCallersPath, string(body))
				graph, err := affected.Build(context.Background(), root, golang.New(), New())
				if err != nil {
					t.Fatal(err)
				}
				plan := affected.Select(graph, []string{script})
				if plan.Scope != affected.ScopeUnknown || !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool { return u.Detail == FrontierForeignInvocation }) {
					t.Fatalf("undeclared constructed caller certified (%s): %+v", expression, plan)
				}
				if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/spellings/caller" }) {
					t.Fatalf("known caller dropped: %+v", plan)
				}
			})
		}
	}
}

// AZU-V0-8901: command-prefix support must keep markers in later arguments
// and diagnostic prose outside the computed basename of a nested script.
func TestConstructedCommandPrefixBounds_AZU_V0_8901(t *testing.T) {
	for _, body := range []string{
		`"sh run/other.sh --flag=$name"`,
		`"check-test: FAIL %s:%s names %s, but run/%s"`,
		`'fixture: REFUSE missing-script run/%s'`,
	} {
		if constructedForeignReference([]byte(body), "run/run.sh") {
			t.Errorf("marker outside computed basename matched: %s", body)
		}
	}
}
