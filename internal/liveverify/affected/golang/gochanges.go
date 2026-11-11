// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/gitrun"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// GoChangesAt compares dirty Go files at a pinned commit with head. A missing
// base or any failed analysis supplies no narrowing evidence (AFP-V0-3320).
func GoChangesAt(ctx context.Context, root, head, commit string, dirty []string) []affected.GoChange {
	if commit == "" || !slices.ContainsFunc(dirty, func(p string) bool { return strings.HasSuffix(p, ".go") }) {
		return nil
	}
	repository, err := gitauth.Open(root, gitrun.NewBudget(gitrun.DefaultOperations, gitrun.DefaultTotalBudget))
	if err != nil {
		return nil
	}
	defer repository.BeginObjectSession()()
	base, err := repository.RevisionFS(ctx, commit, affected.MaxSourceBytes)
	if err != nil {
		return nil
	}
	return GoChangeRecords(base, os.DirFS(head), dirty)
}

// GoChangeRecords returns only proven COMMENT_ONLY paths. Both endpoints
// must exist, parse in full, and have identical scanner tokens (including
// inserted semicolons) on identical lines, since line numbers are compiled
// into runtime.Caller, panics and log output. Directives, build-constraint
// text and cgo preambles are compared separately.
// Add/delete/rename endpoints and unreadable or invalid files fail closed.
func GoChangeRecords(base, head fs.FS, dirty []string) []affected.GoChange {
	if base == nil || head == nil {
		return nil
	}
	var changes []affected.GoChange
	for _, relative := range affected.NormalizePaths(dirty) {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		before, err := readChangeSource(base, relative)
		if err != nil {
			continue
		}
		after, err := readChangeSource(head, relative)
		if err != nil || !commentOnlyGo(before, after) {
			continue
		}
		if strings.HasSuffix(relative, "_test.go") && (commentReadingPackage(base, path.Dir(relative)) || commentReadingPackage(head, path.Dir(relative))) {
			continue
		}
		changes = append(changes, affected.GoChange{Path: relative, Kind: affected.GoCommentOnly})
	}
	return changes
}

func readChangeSource(tree fs.FS, relative string) ([]byte, error) {
	source := affected.FSSource(tree)
	body, err := source.Read(relative)
	if err != nil {
		return nil, err
	}
	return body, source.Err()
}

type goLexeme struct {
	Kind token.Token
	Text string
	// Line is the unadjusted line, 0 for EOF: a comment appended after the
	// last token moves only EOF, which no compiled position records.
	Line int
}

// protectedComment also pins attachment to the surrounding tokens: moving a
// directive across code or detaching it from a declaration is not whitespace.
type protectedComment struct {
	Before int
	Gap    string
	Text   string
	Prefix string
}

func commentOnlyGo(before, after []byte) bool {
	left, leftComments, ok := goContent(before)
	if !ok {
		return false
	}
	right, rightComments, ok := goContent(after)
	return ok && slices.Equal(left, right) && slices.Equal(leftComments, rightComments)
}

func goContent(body []byte) ([]goLexeme, []protectedComment, bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "change.go", body, parser.ParseComments)
	if err != nil {
		return nil, nil, false
	}
	// An import spec's own doc, or a single import declaration's doc, is
	// the cgo preamble. Use raw bytes, since CommentGroup.Text normalizes them.
	preambles := map[token.Pos]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			imp := spec.(*ast.ImportSpec)
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil || name != "C" {
				continue
			}
			doc := imp.Doc
			if doc == nil && len(gen.Specs) == 1 {
				doc = gen.Doc
			}
			if doc != nil {
				for _, comment := range doc.List {
					preambles[comment.Pos()] = true
				}
			}
		}
	}
	var scan scanner.Scanner
	tokens := fset.File(file.Pos())
	scan.Init(tokens, body, nil, scanner.ScanComments)
	var stream []goLexeme
	var protected []protectedComment
	var ends []int
	for {
		pos, kind, literal := scan.Scan()
		if kind == token.COMMENT {
			if !preambles[pos] && !sensitiveComment(literal) {
				continue
			}
			offset := tokens.Offset(pos)
			// Scanner drops CR from comment literals. Retain the physical
			// lines as well so no directive-line byte change is hidden.
			end := offset
			if bytes.HasPrefix(body[offset:], []byte("/*")) {
				end += bytes.Index(body[offset:], []byte("*/")) + 2
			} else if n := bytes.IndexByte(body[offset:], '\n'); n >= 0 {
				end += n
			} else {
				end = len(body)
			}
			start := bytes.LastIndexByte(body[:offset], '\n') + 1
			lineEnd := end
			if n := bytes.IndexByte(body[end:], '\n'); n >= 0 {
				lineEnd += n
			} else {
				lineEnd = len(body)
			}
			comment := protectedComment{Before: len(stream), Text: string(body[start:lineEnd])}
			if legacyBuildComment(literal) {
				// A block comment inserted before +build ends the header
				// region in which Go accepts legacy constraints.
				comment.Prefix = string(body[:offset])
			}
			protected = append(protected, comment)
			ends = append(ends, end)
			continue
		}
		for i := len(protected) - 1; i >= 0 && protected[i].Before == len(stream); i-- {
			if tokens.Offset(pos) < ends[i] {
				// An inserted semicolon inside a multiline comment makes
				// attachment ambiguous; retain full propagation.
				return nil, nil, false
			}
			// Preserve blank-line structure too: replacing the blank after
			// a legacy +build line with an ordinary comment can disable the
			// constraint even though the newline count stays unchanged.
			protected[i].Gap = string(body[ends[i]:tokens.Offset(pos)])
		}
		line := 0
		if kind != token.EOF {
			line = tokens.PositionFor(pos, false).Line
		}
		stream = append(stream, goLexeme{Kind: kind, Text: literal, Line: line})
		if kind == token.EOF {
			return stream, protected, scan.ErrorCount == 0
		}
	}
}

// sensitiveComment also protects any comment mentioning a build constraint,
// including inside a block comment, because vet's buildtag check reads them.
func sensitiveComment(text string) bool {
	return strings.Contains(text, "+build") || strings.Contains(text, "//go:build") || strings.HasPrefix(text, "//go:") || strings.HasPrefix(text, "//line") || strings.HasPrefix(text, "/*line") || legacyBuildComment(text) || strings.HasPrefix(text, "//export")
}

func legacyBuildComment(text string) bool {
	return strings.HasPrefix(text, "//") && strings.HasPrefix(strings.TrimSpace(text[2:]), "+build")
}

// Conservatively retain test-file propagation when the package could inspect
// comments. The roots of parser calls need not be guessed: go/doc, or go/ast
// with parser.ParseComments anywhere in the package (aliases included), is
// sufficient. A directory/read/parse error also refuses narrowing.
func commentReadingPackage(tree fs.FS, directory string) bool {
	entries, err := fs.ReadDir(tree, directory)
	if err != nil {
		return true
	}
	usesAST, parsesComments := false, false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		body, err := readChangeSource(tree, path.Join(directory, entry.Name()))
		if err != nil {
			return true
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), body, parser.ParseComments)
		if err != nil {
			return true
		}
		for _, imp := range file.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil || name == "go/doc" {
				return true
			}
			usesAST = usesAST || name == "go/ast"
			if name == "go/parser" {
				// Identifiers, rather than raw text, exclude fixture strings
				// and comments and include renamed and dot imports.
				ast.Inspect(file, func(node ast.Node) bool {
					if ident, ok := node.(*ast.Ident); ok && ident.Name == "ParseComments" {
						parsesComments = true
					}
					return true
				})
			}
		}
	}
	return usesAST && parsesComments
}
