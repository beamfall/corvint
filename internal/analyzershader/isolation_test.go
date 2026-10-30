package analyzershader

import (
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// This is a source-level regression guard, not a replacement for an OS trace.
// Its positive controls prove every forbidden-channel detector is live.
func TestCandidateSourceForbidsAmbientChannels(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	for _, name := range []string{"analyzer.go", "lexer.go", "glsl.go", "metal.go"} {
		source, err := os.ReadFile(filepath.Join(filepath.Dir(file), name))
		if err != nil {
			t.Fatal(err)
		}
		if got := forbiddenChannel(string(source)); got != "" {
			t.Fatalf("%s exposes %s", name, got)
		}
	}
	commandSource, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "cmd", "corvint-analyzer-shader", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if got := forbiddenCommandChannel(string(commandSource)); got != "" {
		t.Fatalf("command exposes %s", got)
	}
	for channel, source := range map[string]string{
		"process-shell":   "package p\nimport shell \"os/exec\"\nfunc f(){shell.Command(\"sh\")}\n",
		"network":         "package p\nimport socket \"net\"\nfunc f(){socket.Dial(\"tcp\", \"127.0.0.1:1\")}\n",
		"loader":          "package p\nimport dynamic \"plugin\"\nfunc f(){dynamic.Open(\"x\")}\n",
		"descriptor":      "package p\nimport descriptor \"os\"\nfunc f(){_ = descriptor.Stdin; _ = descriptor.Stdout; _ = descriptor.Stderr}\n",
		"cwd-home-env-fs": "package p\nimport ambient \"os\"\nfunc f(){ambient.Chdir(\"x\"); ambient.Getwd(); ambient.UserHomeDir(); ambient.Getenv(\"HOME\"); ambient.ReadFile(\"x\"); ambient.WriteFile(\"x\",nil,0)}\n",
		"path":            "package p\nimport traversal \"path/filepath\"\n",
		"command-alias":   "package p\nimport commandOS \"os\"\nfunc f(){commandOS.ReadFile(\"x\")}\n",
	} {
		if got := forbiddenChannel(source); got == "" {
			t.Fatalf("SPY_INEFFECTIVE %s", channel)
		}
		if channel == "command-alias" && forbiddenCommandChannel(source) == "" {
			t.Fatalf("COMMAND_SPY_INEFFECTIVE %s", channel)
		}
	}
}

func TestCandidateSourceSizeRatchet(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	total := 0
	for _, name := range []string{"analyzer.go", "lexer.go", "glsl.go", "metal.go"} {
		source, err := os.ReadFile(filepath.Join(filepath.Dir(file), name))
		if err != nil {
			t.Fatal(err)
		}
		total += len(source)
	}
	const ceiling = 65_536
	if total > ceiling {
		t.Fatalf("source=%d ceiling=%d", total, ceiling)
	}
}
func forbiddenChannel(source string) string {
	file, err := parser.ParseFile(gotoken.NewFileSet(), "candidate.go", source, 0)
	if err != nil {
		return "parse"
	}
	aliases := map[string]string{}
	for _, importSpec := range file.Imports {
		path := strings.Trim(importSpec.Path.Value, "\"")
		if map[string]bool{"os/exec": true, "net": true, "net/http": true, "net/url": true, "plugin": true, "syscall": true, "path/filepath": true, "io/fs": true}[path] {
			return path
		}
		alias := strings.TrimSuffix(path, "/")
		if slash := strings.LastIndex(alias, "/"); slash >= 0 {
			alias = alias[slash+1:]
		}
		if importSpec.Name != nil {
			alias = importSpec.Name.Name
		}
		aliases[alias] = path
	}
	var found string
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if ok && aliases[identifier.Name] == "os" && map[string]bool{"Chdir": true, "Create": true, "Getenv": true, "Getwd": true, "LookupEnv": true, "Open": true, "OpenFile": true, "ReadFile": true, "Setenv": true, "Stdin": true, "Stdout": true, "Stderr": true, "UserHomeDir": true, "WriteFile": true}[selector.Sel.Name] {
			found = "os." + selector.Sel.Name
		}
		return found == ""
	})
	return found
}

// The command is permitted only the explicit process boundary: arguments,
// stdin/stdout, and exit status. All other ambient OS selectors are forbidden.
func forbiddenCommandChannel(source string) string {
	file, err := parser.ParseFile(gotoken.NewFileSet(), "command.go", source, 0)
	if err != nil {
		return "parse"
	}
	aliases := map[string]string{}
	for _, importSpec := range file.Imports {
		path := strings.Trim(importSpec.Path.Value, "\"")
		if map[string]bool{"os/exec": true, "net": true, "net/http": true, "net/url": true, "plugin": true, "syscall": true, "path/filepath": true, "io/fs": true}[path] {
			return path
		}
		alias := path
		if slash := strings.LastIndex(alias, "/"); slash >= 0 {
			alias = alias[slash+1:]
		}
		if importSpec.Name != nil {
			alias = importSpec.Name.Name
		}
		aliases[alias] = path
	}
	var found string
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if ok && aliases[identifier.Name] == "os" && !map[string]bool{"Args": true, "Exit": true, "Stdin": true, "Stdout": true}[selector.Sel.Name] {
			found = "os." + selector.Sel.Name
		}
		return found == ""
	})
	return found
}
