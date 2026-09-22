package releasegate

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type runtimeMatch struct {
	kind, detail string
	start, end   int
}

func detect(name string, content []byte, evidence Evidence) []Finding {
	findings := pathFindings(name, content, evidence)
	matches := runtimeMatches(content)
	if strings.HasSuffix(name, ".go") {
		// Go execution is analyzed by the package-level, target-selected pass
		// in modularity.go.  Per-file type checks cannot resolve aliases or
		// distinguish a shadowed selector from os/exec.
		matches = nil
	}
	for _, match := range matches {
		findings = append(findings, spanFinding(match.kind, match.detail, evidence, content, match.start, match.end))
	}
	return findings
}

func pathFindings(name string, content []byte, evidence Evidence) []Finding {
	base := strings.ToLower(path.Base(name))
	if strings.HasSuffix(base, ".py") {
		return []Finding{wholeFinding("python-source", "Python source is present in artifact inventory", evidence, content)}
	}
	packaging := map[string]bool{"pyproject.toml": true, "setup.py": true, "setup.cfg": true, "pipfile": true, "pipfile.lock": true, "poetry.lock": true, "tox.ini": true}
	if packaging[base] || strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt") {
		return []Finding{wholeFinding("python-packaging", "Python packaging metadata is present in artifact inventory", evidence, content)}
	}
	return nil
}

func goRuntimeMatches(content []byte) []runtimeMatch {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "release-tree.go", content, parser.AllErrors)
	if err != nil {
		return []runtimeMatch{{"go-analysis-unknown", "Go source parse ambiguity prevents safe execution analysis", 0, len(content)}}
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}}
	conf := types.Config{Importer: releaseGateImporter{}, Error: func(error) {}}
	_, _ = conf.Check("releasegate/scan", files, []*ast.File{file}, info)
	var matches []runtimeMatch
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		kind, shell, known := commandCall(info, call.Fun)
		if !known && maybeExecSelector(file, call.Fun) {
			matches = append(matches, runtimeMatch{"go-analysis-unknown", "import-bound execution call could not be resolved exactly", files.Position(call.Pos()).Offset, files.Position(call.End()).Offset})
			return true
		}
		if !known {
			return true
		}
		start := files.Position(call.Pos()).Offset
		end := files.Position(call.End()).Offset
		argStart := 0
		if kind == "exec-command-context" {
			argStart = 1
		}
		if len(call.Args) <= argStart {
			return true
		}
		command, exact := constantString(info, call.Args[argStart])
		if !exact {
			matches = append(matches, runtimeMatch{"go-analysis-unknown", "dynamic command executable prevents safe release analysis", start, end})
			return true
		}
		if pythonInterpreter(command) {
			matches = append(matches, runtimeMatch{"python-execution", "Python interpreter execution command", start, end})
			return true
		}
		if environmentInterpreter(command) {
			for _, argument := range call.Args[argStart+1:] {
				value, exact := constantString(info, argument)
				if !exact {
					matches = append(matches, runtimeMatch{"go-analysis-unknown", "dynamic environment interpreter arguments prevent safe release analysis", start, end})
					return true
				}
				if pythonInterpreter(value) {
					matches = append(matches, runtimeMatch{"python-execution", "environment launcher executes Python", start, end})
					return true
				}
			}
		}
		if shellInterpreter(command) {
			payload, exact := shellPayload(info, call.Args[argStart+1:])
			if !exact {
				matches = append(matches, runtimeMatch{"go-analysis-unknown", "dynamic shell payload prevents safe release analysis", start, end})
				return true
			}
			if hasPythonCommand(payload) {
				matches = append(matches, runtimeMatch{"python-shell-execution", "shell payload executes Python", start, end})
			} else {
				matches = append(matches, runtimeMatch{"shell-execution", "shell execution is forbidden in a native release", start, end})
			}
		}
		_ = shell
		return true
	})
	return matches
}

func commandCall(info *types.Info, expression ast.Expr) (string, bool, bool) {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return "", false, false
	}
	selection := info.Uses[selector.Sel]
	function, ok := selection.(*types.Func)
	if !ok || function.Pkg() == nil {
		return "", false, false
	}
	if function.Pkg().Path() == "os/exec" && (function.Name() == "Command" || function.Name() == "CommandContext") {
		if function.Name() == "CommandContext" {
			return "exec-command-context", false, true
		}
		return "exec-command", false, true
	}
	return "", false, false
}

type releaseGateImporter struct{}

func (releaseGateImporter) Import(path string) (*types.Package, error) {
	pkg := types.NewPackage(path, pathBase(path))
	if path != "os/exec" {
		return pkg, nil
	}
	stringType := types.Typ[types.String]
	cmd := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Cmd", nil), types.NewStruct([]*types.Var{types.NewVar(token.NoPos, pkg, "Path", stringType)}, nil), nil)
	result := types.NewPointer(cmd)
	command := types.NewFunc(token.NoPos, pkg, "Command", types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewVar(token.NoPos, pkg, "name", stringType), types.NewVar(token.NoPos, pkg, "arg", types.NewSlice(stringType))), types.NewTuple(types.NewVar(token.NoPos, pkg, "result", result)), true))
	contextType := types.NewInterfaceType(nil, nil)
	commandContext := types.NewFunc(token.NoPos, pkg, "CommandContext", types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewVar(token.NoPos, pkg, "ctx", contextType), types.NewVar(token.NoPos, pkg, "name", stringType), types.NewVar(token.NoPos, pkg, "arg", types.NewSlice(stringType))), types.NewTuple(types.NewVar(token.NoPos, pkg, "result", result)), true))
	pkg.Scope().Insert(cmd.Obj())
	for _, name := range []string{"Run", "Start", "Wait", "Output", "CombinedOutput"} {
		method := types.NewFunc(token.NoPos, pkg, name, types.NewSignatureType(types.NewVar(token.NoPos, pkg, "cmd", result), nil, nil, nil, nil, false))
		cmd.AddMethod(method)
	}
	pkg.Scope().Insert(command)
	pkg.Scope().Insert(commandContext)
	pkg.MarkComplete()
	return pkg, nil
}
func pathBase(value string) string {
	_, base, _ := strings.Cut(value, "/")
	for strings.Contains(base, "/") {
		_, base, _ = strings.Cut(base, "/")
	}
	return base
}
func maybeExecSelector(file *ast.File, expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || (selector.Sel.Name != "Command" && selector.Sel.Name != "CommandContext") {
		return false
	}
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	for _, imp := range file.Imports {
		pathValue, err := strconv.Unquote(imp.Path.Value)
		if err != nil || pathValue != "os/exec" {
			continue
		}
		name := "exec"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == owner.Name {
			return true
		}
	}
	return false
}

func constantString(info *types.Info, expression ast.Expr) (string, bool) {
	value, ok := info.Types[expression]
	if !ok || value.Value == nil || value.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(value.Value), true
}

func shellPayload(info *types.Info, args []ast.Expr) (string, bool) {
	for index, arg := range args {
		value, ok := constantString(info, arg)
		if !ok {
			return "", false
		}
		if (strings.EqualFold(value, "-c") || strings.EqualFold(value, "/c") || strings.EqualFold(value, "-command")) && index+1 < len(args) {
			return constantString(info, args[index+1])
		}
	}
	return "", true
}

func pythonInterpreter(value string) bool {
	value = strings.TrimSpace(value)
	base := strings.ToLower(path.Base(strings.ReplaceAll(value, "\\", "/")))
	base = strings.TrimSuffix(base, ".exe")
	return regexp.MustCompile(`^python(?:[0-9.]*)?$`).MatchString(base) || base == "py"
}
func environmentInterpreter(value string) bool {
	base := strings.TrimSuffix(strings.ToLower(path.Base(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))), ".exe")
	return base == "env"
}
func shellInterpreter(value string) bool {
	switch strings.TrimSuffix(strings.ToLower(path.Base(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))), ".exe") {
	case "sh", "bash", "dash", "zsh", "ksh", "fish", "cmd", "powershell", "pwsh":
		return true
	}
	return false
}
func hasPythonCommand(value string) bool {
	for _, field := range strings.FieldsFunc(value, func(r rune) bool { return r == ';' || r == '&' || r == '|' || r == '\n' || r == '\t' || r == ' ' }) {
		if pythonInterpreter(field) {
			return true
		}
	}
	return false
}

func runtimeMatches(content []byte) []runtimeMatch {
	patterns := []struct {
		kind, detail string
		expression   *regexp.Regexp
	}{
		{"python-shebang", "Python interpreter shebang", regexp.MustCompile(`(?m)^#![^\n]*(?:/|\b)python(?:[0-9.]*)?(?:\s|$)`)},
		{"python-environment", "Python runtime environment variable", regexp.MustCompile(`\bPYTHON(?:PATH|HOME)\b`)},
		{"python-runtime-reference", "Python runtime or package reference", regexp.MustCompile(`(?i)(?:\bpip(?:3)?\b|\.venv\b|site-packages|\bcorvint_cli\b|\bcontext_corvint\b)`)},
	}
	var out []runtimeMatch
	for _, p := range patterns {
		for _, index := range p.expression.FindAllIndex(content, -1) {
			out = append(out, runtimeMatch{p.kind, p.detail, index[0], index[1]})
		}
	}
	return out
}

func wholeFinding(kind, detail string, evidence Evidence, data ...[]byte) Finding {
	content := []byte(nil)
	if len(data) > 0 {
		content = data[0]
	}
	return spanFinding(kind, detail, evidence, content, 0, len(content))
}
func spanFinding(kind, detail string, evidence Evidence, content []byte, start, end int) Finding {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(content) {
		end = len(content)
	}
	evidence.SpanStart, evidence.SpanEnd = start, end
	evidence.SpanSHA256 = sha256Hex(content[start:end])
	evidence.Line, evidence.Column = position(content, start)
	return Finding{kind, detail, evidence}
}
func position(content []byte, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	return bytes.Count(content[:offset], []byte{'\n'}) + 1, offset - bytes.LastIndexByte(content[:offset], '\n')
}
