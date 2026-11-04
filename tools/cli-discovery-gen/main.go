// SPDX-License-Identifier: Apache-2.0
// cli-discovery-gen migrates single-mode and folded companion help to command trees.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/clitree"
)

const treeDir = "internal/capcatalog/trees"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--docs" {
		writeDocs()
		return
	}
	entries, err := os.ReadDir("cmd")
	must(err)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		binary := entry.Name()
		dest := filepath.Join(treeDir, binary+".json")
		data, err := os.ReadFile(dest)
		if err == nil && !strings.Contains(string(data), `"source": "companion root contract"`) {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			must(err)
		}
		var tree clitree.Tree
		if group := map[string]string{"corvint-behavior-falsify": "falsify", "corvint-playwright-minimize": "minimize", "corvint-test-runner": "runner", "corvint-tests-accept": "accept"}[binary]; group != "" {
			tree = clitree.Tree{Schema: clitree.Schema, Binary: binary, HelpJSON: binary + " help --json", Source: "companion root contract", Global: []string{"--version"}}
			data, err = os.ReadFile(filepath.Join(treeDir, "corvint-test.json"))
			must(err)
			var folded clitree.Tree
			must(json.Unmarshal(data, &folded))
			tree.Sources = []string{"cmd/" + binary, map[string]string{"falsify": "internal/behaviorfalsify/cli", "minimize": "internal/playwrightminimize/cli", "runner": "internal/testrunner/cli", "accept": "internal/testacceptance/cli"}[group]}
			tree.Internal = folded.Internal
			tree.Nodes = nil
			for _, node := range folded.Nodes {
				if len(node.Path) == 0 || node.Path[0] != group {
					continue
				}
				node.Path = slices.Clone(node.Path[1:])
				node.Command = strings.Replace(node.Command, "corvint-test "+group, binary, 1)
				node.Help = strings.Replace(node.Help, "corvint-test "+group, binary, 1)
				node.Example = strings.Replace(node.Example, "corvint-test "+group, binary, 1)
				for i, form := range node.Usage {
					node.Usage[i] = strings.Replace(form, "corvint-test "+group, binary, 1)
				}
				tree.Nodes = append(tree.Nodes, node)
			}
		} else {
			tree = rootTree(binary)
		}
		tree.Nodes = append(tree.Nodes, clitree.Node{Path: []string{"help"}, Command: binary + " help", Summary: "Show command help or the offline machine catalog.", Usage: []string{binary + " help [COMMAND...] [--json]"}, Flags: []string{"--json"}, Required: []string{}, Example: binary + " help --json", Help: binary + " help --help"})
		encoded, err := clitree.Encode(tree)
		must(err)
		must(os.WriteFile(dest, encoded, 0644))
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootTree(binary string) clitree.Tree {
	dir := "cmd/" + binary
	dirs := []string{dir}
	if binary == "corvint-mcp" { // This entrypoint delegates its parser and authored usage.
		dirs = append(dirs, "internal/mcp/cli")
	}
	usage := ""
	descriptions := map[string]string{}
	values := map[string]string{}
	summaryText := ""
	for _, source := range dirs {
		entries, err := os.ReadDir(source)
		must(err)
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(source, entry.Name()), nil, 0)
			must(err)
			if file.Doc != nil && summaryText == "" {
				summaryText = strings.TrimSpace(file.Doc.Text())
			}
			ast.Inspect(file, func(n ast.Node) bool {
				// Printed usage literals are as authoritative as named constants.
				if expr, ok := n.(ast.Expr); ok {
					s := literal(expr)
					if strings.Contains(strings.ToLower(s), "usage: "+binary) && len(s) > len(usage) {
						usage = s
					}
				}
				if c, ok := n.(*ast.CallExpr); ok {
					if f, ok := c.Fun.(*ast.SelectorExpr); ok {
						index := -1
						switch f.Sel.Name {
						case "String", "Bool", "Int", "Int64", "Uint", "Uint64", "Duration", "Float64", "Func", "BoolFunc":
							index = 0
						case "StringVar", "BoolVar", "IntVar", "Int64Var", "UintVar", "Uint64Var", "DurationVar", "Float64Var", "Var":
							index = 1
						}
						if index >= 0 && len(c.Args) > index+1 {
							name := literal(c.Args[index])
							if name != "" {
								key := "--" + name
								descriptions[key] = literal(c.Args[len(c.Args)-1])
								if !strings.HasPrefix(f.Sel.Name, "Bool") {
									values[key] = "VALUE"
								}
							}
						}
					}
				}
				return true
			})
		}
	}
	scan, err := clitree.ScanDirs(dirs...)
	must(err)
	flags := []string{}
	for _, f := range scan.Flags {
		flags = append(flags, f.Value)
	}
	forms := clitree.Synopses(binary, usage)
	required := []string{}
	if len(forms) == 0 {
		if !strings.HasPrefix(binary, "corvint-analyzer-") || len(flags) != 0 {
			panic(binary + ": missing authored usage; parser declarations cannot establish required inputs or conditional modes")
		}
		forms = []string{binary + " < REQUEST.json"}
		required = []string{"stdin: protocol request JSON"}
	}
	for _, f := range flags {
		if !slices.Contains(clitree.Flags(forms), f) {
			token := f
			if values[f] != "" {
				token += " " + values[f]
			}
			forms[0] += " [" + token + "]"
		}
	}
	summary := strings.TrimSpace(usage)
	if summary == "" {
		summary = summaryText
	}
	if summary == "" {
		summary = "Experimental local JSON protocol companion; reads one request on stdin and writes its response on stdout."
	}
	for _, f := range flags {
		if descriptions[f] != "" {
			summary += "\n" + f + ": " + descriptions[f]
		}
	}
	if len(required) == 0 { // Required inputs from authored usage remain explicit.
		builder := clitree.NewBuilder(binary)
		builder.AddForm(strings.Replace(forms[0], binary, binary+" run", 1), "")
		derived := builder.Tree("", "", nil, nil, nil)
		if len(derived.Nodes) > 0 {
			required = derived.Nodes[0].Required
		}
	}

	flags = append(flags, clitree.Flags(forms)...)
	slices.Sort(flags)
	flags = slices.Compact(flags)
	root := clitree.Node{Path: []string{}, Command: binary, Summary: summary, Usage: forms, Flags: flags, Required: required, Example: clitree.MinimalExample(forms[0]), Help: binary + " --help"}
	return clitree.Tree{Schema: clitree.Schema, Binary: binary, HelpJSON: binary + " help --json", Source: "companion root contract", Sources: dirs, Global: []string{"--version"}, Internal: []clitree.Internal{}, Nodes: []clitree.Node{root}}
}

func literal(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			s, _ := strconv.Unquote(v.Value)
			return s
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			return literal(v.X) + literal(v.Y)
		}
	}
	return ""
}

func writeDocs() {
	entries, err := os.ReadDir(treeDir)
	must(err)
	var trees []clitree.Tree
	var links strings.Builder
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(treeDir, entry.Name()))
		must(err)
		var tree clitree.Tree
		must(json.Unmarshal(data, &tree))
		trees = append(trees, tree)
		fmt.Fprintf(&links, "- `%s`: [%s](%s/%s)\n", tree.HelpJSON, tree.Binary, treeDir, entry.Name())
	}
	replaceBlock("docs/CAPABILITIES.md", "<!-- capability-gate:cli-nodes -->", "<!-- /capability-gate:cli-nodes -->", clitree.CatalogMarkdown(trees))
	replaceBlock("llms.txt", "<!-- cli-tree-links -->", "<!-- /cli-tree-links -->", links.String())
}
func replaceBlock(path, open, close, body string) {
	data, err := os.ReadFile(path)
	must(err)
	text := string(data)
	before, after, found := strings.Cut(text, open)
	if found {
		_, after, found = strings.Cut(after, close)
		if !found {
			panic("unclosed generated block " + path)
		}
	} else {
		before = text + "\n"
		after = ""
	}
	must(os.WriteFile(path, []byte(before+open+"\n"+body+close+after), 0644))
}
