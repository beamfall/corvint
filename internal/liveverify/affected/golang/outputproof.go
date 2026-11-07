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

// streamCaptureNames are the identifiers whose presence in any Go file of a
// directory is evidence that the process can capture a standard stream and
// read it back (AFP-V0-2960): a pipe (`os.Pipe`, `io.Pipe`, `net.Pipe`,
// `syscall.Pipe`, `unix.Pipe2`), a duplicated descriptor or replaced handle
// (`Dup`, `Dup2`, `Dup3`, `SetStdHandle`), a file over a raw descriptor
// (`os.NewFile`), and a child process's captured output (`Output`,
// `CombinedOutput`, `StdoutPipe`, `StderrPipe`), which lets a test re-run
// its own program and read what it printed.
var streamCaptureNames = map[string]bool{
	"Pipe": true, "Pipe2": true, "Dup": true, "Dup2": true, "Dup3": true, "SetStdHandle": true,
	"NewFile": true, "Output": true, "CombinedOutput": true, "StdoutPipe": true, "StderrPipe": true,
}

// streamCaptureHints are substrings at least one of which every capture
// evidence holds, so a file with none of them needs no parse.
var streamCaptureHints = [][]byte{
	[]byte("Pipe"), []byte("Dup"), []byte("SetStdHandle"), []byte("NewFile"), []byte("Output"),
	[]byte("Stdout"), []byte("Stderr"), []byte("go:linkname"),
}

// capturesStream reports whether one whole Go file, test files included,
// gives any evidence that the process can capture a standard stream
// (AFP-V0-2960): an identifier in streamCaptureNames under any qualifier; an
// assignment to, the address of, or a composite-literal key named `Stdout`
// or `Stderr` (`os.Stdout = w`, `&os.Stdout`, `cmd.Stdout = &buffer`,
// `exec.Cmd{Stdout: w}`); or a `//go:linkname` directive, which can bind a
// local name to the stream variable. A file that does not parse is evidence
// too. The test is lexical and broad on purpose: a printed literal is text
// only while nothing can read the stream back, so any doubt keeps rule (d).
func capturesStream(body []byte) bool {
	hinted := false
	for _, hint := range streamCaptureHints {
		if bytes.Contains(body, hint) {
			hinted = true
			break
		}
	}
	if !hinted {
		return false
	}
	if bytes.Contains(body, []byte("go:linkname")) {
		return true
	}
	file, err := parser.ParseFile(token.NewFileSet(), "", body, parser.SkipObjectResolution)
	if err != nil {
		return true
	}
	stream := func(expression ast.Expr) bool {
		switch typed := expression.(type) {
		case *ast.SelectorExpr:
			return typed.Sel.Name == "Stdout" || typed.Sel.Name == "Stderr"
		case *ast.Ident:
			return typed.Name == "Stdout" || typed.Name == "Stderr"
		}
		return false
	}
	captured := false
	ast.Inspect(file, func(node ast.Node) bool {
		if captured {
			return false
		}
		switch typed := node.(type) {
		case *ast.Ident:
			captured = streamCaptureNames[typed.Name]
		case *ast.AssignStmt:
			for _, target := range typed.Lhs {
				captured = captured || stream(ast.Unparen(target))
			}
		case *ast.UnaryExpr:
			captured = typed.Op == token.AND && stream(ast.Unparen(typed.X))
		case *ast.KeyValueExpr:
			captured = stream(typed.Key)
		}
		return !captured
	})
	return captured
}
