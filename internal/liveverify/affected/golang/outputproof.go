// SPDX-License-Identifier: Apache-2.0
package golang

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
)

// outputSinkPackages names the imports whose local names outputOnlyLiterals
// resolves: the formatting calls, the writer helper and the standard streams.
var outputSinkPackages = map[string]bool{"fmt": true, "io": true, "os": true}

// outputOnlyLiterals returns the body offsets of the string literals that
// reach only the process's standard output or error stream (AFP-V0-2952): a
// literal that is itself an argument of `fmt.Print`, `fmt.Printf` or
// `fmt.Println`; an argument after the writer of `fmt.Fprint`, `fmt.Fprintf`,
// `fmt.Fprintln` or `io.WriteString` whose writer is `os.Stdout` or
// `os.Stderr`; or the argument of `os.Stdout.WriteString` or
// `os.Stderr.WriteString`, under the local names qualifiers gives fmt, io and
// os. Nothing else is evidence: text formatted into a value
// (`fmt.Sprintf`, `fmt.Appendf`), written into a buffer or another writer,
// assigned, returned or concatenated may be trimmed, split or joined into a
// path the program opens. Offsets refer to body, which may start after the
// imports; the synthetic package clause lets the same proof operate on either
// scanner's input. A parse failure proves nothing.
func outputOnlyLiterals(body []byte, qualifiers map[string]string) map[int]bool {
	// "rint" is in Print and Fprint alike.
	if len(qualifiers) == 0 || !bytes.Contains(body, []byte("rint")) && !bytes.Contains(body, []byte("WriteString")) {
		return nil
	}
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, "", body, 0)
	shift := 0
	if err != nil {
		const prefix = "package outputproof\n"
		positions = token.NewFileSet()
		file, err = parser.ParseFile(positions, "", append([]byte(prefix), body...), 0)
		shift = len(prefix)
	}
	if err != nil {
		return nil
	}
	var output map[int]bool
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || call.Ellipsis.IsValid() {
			return true
		}
		for _, argument := range outputArguments(call, qualifiers) {
			literal, ok := argument.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			if output == nil {
				output = map[int]bool{}
			}
			output[positions.Position(literal.Pos()).Offset-shift] = true
		}
		return true
	})
	return output
}

// outputArguments returns the arguments of call that a standard stream
// receives as text, nil when call is no such sink.
func outputArguments(call *ast.CallExpr, qualifiers map[string]string) []ast.Expr {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	if stream := selector.X; selector.Sel.Name == "WriteString" && standardStream(stream, qualifiers) {
		return call.Args
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Obj != nil {
		return nil
	}
	switch qualifiers[qualifier.Name] + "." + selector.Sel.Name {
	case "fmt.Print", "fmt.Printf", "fmt.Println":
		return call.Args
	case "fmt.Fprint", "fmt.Fprintf", "fmt.Fprintln", "io.WriteString":
		if len(call.Args) > 1 && standardStream(call.Args[0], qualifiers) {
			return call.Args[1:]
		}
	}
	return nil
}

// standardStream reports `os.Stdout` or `os.Stderr` under the local name
// qualifiers gives os.
func standardStream(expression ast.Expr, qualifiers map[string]string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Stdout" && selector.Sel.Name != "Stderr" {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	return ok && qualifier.Obj == nil && qualifiers[qualifier.Name] == "os"
}
