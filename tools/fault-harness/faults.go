// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Operator families named by a pre-registration (design decision 6). The
// harness defines its own operators (G26); it imports no mutation package.
var operatorFamilies = []string{"relational-swap", "arithmetic-swap", "boolean-negation", "constant-replacement", "statement-removal"}

// fault is one injected change. A mutant or non-code fault replaces exact
// bytes at an offset of one file; a revert restores a fix PR's pre-fix source
// (FIH-V1-003).
type fault struct {
	ID       string `json:"id"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Before   string `json:"before,omitempty"`
	After    string `json:"after,omitempty"`
	Operator string `json:"operator"`           // a family, "revert" or "non-code"
	Category string `json:"category,omitempty"` // non-code: configuration, fixture, snapshot, lockfile, generated
	// Revert names the fix PR's parent and merge commits and its source paths.
	Revert       *revertFault `json:"revert,omitempty"`
	FlowReaching bool         `json:"flowReaching,omitempty"`
}

type revertFault struct {
	FixBase string   `json:"fixBase"`
	FixHead string   `json:"fixHead"`
	Paths   []string `json:"paths"`
}

var nonCodeCategories = []string{"configuration", "fixture", "snapshot", "lockfile", "generated"}

func validPath(p string) error {
	if p == "" || !filepath.IsLocal(p) || filepath.ToSlash(filepath.Clean(p)) != p || strings.Contains(p, "\\") {
		return fmt.Errorf("fault needs a canonical local path, got %q", p)
	}
	for _, part := range strings.Split(p, "/") {
		if strings.EqualFold(part, ".git") {
			return errors.New("faulting Git metadata is forbidden")
		}
	}
	return nil
}

// lineSet is a set of 1-based line numbers of one file.
type lineSet map[int]bool

var hunkHeader = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,([0-9]+))? @@`)

// changedLines parses `git diff -U0` output into the new-side line numbers
// each file's hunks add or change.
func changedLines(diff string) map[string]lineSet {
	out := map[string]lineSet{}
	current := ""
	for _, line := range strings.Split(diff, "\n") {
		if name, ok := strings.CutPrefix(line, "+++ "); ok {
			current = ""
			if name != "/dev/null" {
				current = strings.TrimPrefix(name, "b/")
				if out[current] == nil {
					out[current] = lineSet{}
				}
			}
			continue
		}
		m := hunkHeader.FindStringSubmatch(line)
		if m == nil || current == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		for i := start; i < start+count; i++ {
			out[current][i] = true
		}
	}
	return out
}

func prChangedLines(ctx context.Context, root, base, head string) (map[string]lineSet, error) {
	diff, err := git(ctx, root, "diff", "--no-color", "--no-ext-diff", "--unified=0", "--no-renames", base, head)
	if err != nil {
		return nil, err
	}
	return changedLines(diff), nil
}

// goSites lists every operator site of the named families in one Go file
// whose line is in changed; a site outside the PR's changed lines is skipped
// (FIH-V1-003).
func goSites(name string, src []byte, changed lineSet, families []string) ([]fault, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var sites []fault
	add := func(pos token.Pos, before, after, family string) {
		if !slices.Contains(families, family) {
			return
		}
		p := fset.Position(pos)
		if !changed[p.Line] || string(src[p.Offset:p.Offset+len(before)]) != before {
			return
		}
		sites = append(sites, fault{Path: name, Line: p.Line, Offset: p.Offset, Before: before, After: after, Operator: family})
	}
	relational := map[token.Token]token.Token{token.LSS: token.GEQ, token.GEQ: token.LSS, token.GTR: token.LEQ, token.LEQ: token.GTR, token.EQL: token.NEQ, token.NEQ: token.EQL}
	arithmetic := map[token.Token]token.Token{token.ADD: token.SUB, token.SUB: token.ADD, token.MUL: token.QUO, token.QUO: token.MUL}
	logical := map[token.Token]token.Token{token.LAND: token.LOR, token.LOR: token.LAND}
	isString := func(e ast.Expr) bool { l, ok := e.(*ast.BasicLit); return ok && l.Kind == token.STRING }
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BinaryExpr:
			if to, ok := relational[n.Op]; ok {
				add(n.OpPos, n.Op.String(), to.String(), "relational-swap")
			} else if to, ok := arithmetic[n.Op]; ok && !isString(n.X) && !isString(n.Y) {
				add(n.OpPos, n.Op.String(), to.String(), "arithmetic-swap")
			} else if to, ok := logical[n.Op]; ok {
				add(n.OpPos, n.Op.String(), to.String(), "boolean-negation")
			}
		case *ast.UnaryExpr:
			if n.Op == token.NOT {
				add(n.OpPos, "!", "", "boolean-negation")
			}
		case *ast.BasicLit:
			if n.Kind == token.INT {
				if v, err := strconv.ParseInt(n.Value, 0, 64); err == nil {
					add(n.ValuePos, n.Value, strconv.FormatInt(v+1, 10), "constant-replacement")
				}
			}
		case *ast.Ident:
			if n.Name == "true" || n.Name == "false" {
				add(n.NamePos, n.Name, map[string]string{"true": "false", "false": "true"}[n.Name], "constant-replacement")
			}
		case *ast.ExprStmt:
			if _, ok := n.X.(*ast.CallExpr); ok {
				start, end := fset.Position(n.Pos()), fset.Position(n.End())
				if start.Line == end.Line {
					add(n.Pos(), string(src[start.Offset:end.Offset]), "", "statement-removal")
				}
			}
		}
		return true
	})
	slices.SortFunc(sites, func(a, b fault) int {
		if a.Offset != b.Offset {
			return a.Offset - b.Offset
		}
		return strings.Compare(a.Operator, b.Operator)
	})
	return sites, nil
}

// pickMutants chooses min(max, len(sites)) sites deterministically from the
// pre-registered seed and the PR number. Fewer than min sites is an error:
// FIH-V1-002 asks for 3 to 5 mutants per PR.
func pickMutants(sites []fault, seed int64, pr, lo, hi int) ([]fault, error) {
	if len(sites) < lo {
		return nil, fmt.Errorf("PR %d has %d operator sites on changed lines, fewer than %d", pr, len(sites), lo)
	}
	key := func(f fault) string {
		var b [16]byte
		binary.BigEndian.PutUint64(b[:8], uint64(seed))
		binary.BigEndian.PutUint64(b[8:], uint64(pr))
		h := sha256.Sum256(append(b[:], fmt.Sprintf("%s:%d:%s", f.Path, f.Offset, f.Operator)...))
		return string(h[:])
	}
	ordered := slices.Clone(sites)
	slices.SortStableFunc(ordered, func(a, b fault) int { return strings.Compare(key(a), key(b)) })
	n := min(hi, len(ordered))
	picked := ordered[:n]
	slices.SortFunc(picked, func(a, b fault) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return a.Offset - b.Offset
	})
	return picked, nil
}

// generateGoMutants builds the mutants of one PR: operator sites on its
// changed lines in non-test Go files at head, then a seeded pick of 3 to 5.
func generateGoMutants(ctx context.Context, root, base, head string, pr int, seed int64, families []string) ([]fault, error) {
	changed, err := prChangedLines(ctx, root, base, head)
	if err != nil {
		return nil, err
	}
	var names []string
	for name := range changed {
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	var sites []fault
	for _, name := range names {
		src, err := gitRaw(ctx, root, "show", head+":"+name)
		if err != nil {
			return nil, err
		}
		got, err := goSites(name, src, changed[name], families)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		sites = append(sites, got...)
	}
	picked, err := pickMutants(sites, seed, pr, 3, 5)
	if err != nil {
		return nil, err
	}
	for i := range picked {
		picked[i].ID = fmt.Sprintf("pr%d-m%d", pr, i+1)
	}
	return picked, nil
}

// applyFault writes f into the checked-out tree at root. A byte fault must find
// exactly its recorded bytes at its offset and line; a revert must restore the
// fix's pre-fix lines, proved by the fix applying cleanly forward again.
func applyFault(ctx context.Context, root string, f fault) error {
	if f.Revert != nil {
		return applyRevert(ctx, root, *f.Revert)
	}
	if err := validPath(f.Path); err != nil {
		return err
	}
	at := root
	for _, part := range strings.Split(f.Path, "/") {
		at = filepath.Join(at, part)
		if st, err := os.Lstat(at); err == nil && st.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlink fault refused")
		}
	}
	name := filepath.Join(root, filepath.FromSlash(f.Path))
	body, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	if f.Before == "" || f.Before == f.After || f.Offset < 0 || f.Offset+len(f.Before) > len(body) || string(body[f.Offset:f.Offset+len(f.Before)]) != f.Before {
		return fmt.Errorf("%s: fault %s does not match its recorded site", f.Path, f.ID)
	}
	if line := 1 + bytes.Count(body[:f.Offset], []byte("\n")); f.Line != 0 && line != f.Line {
		return fmt.Errorf("%s: fault %s is on line %d, recorded %d", f.Path, f.ID, line, f.Line)
	}
	out := append(append(slices.Clone(body[:f.Offset]), f.After...), body[f.Offset+len(f.Before):]...)
	return os.WriteFile(name, out, 0o600)
}

func applyRevert(ctx context.Context, root string, r revertFault) error {
	if len(r.Paths) == 0 {
		return errors.New("revert names no source paths")
	}
	for _, p := range r.Paths {
		if err := validPath(p); err != nil {
			return err
		}
	}
	patch, err := gitRaw(ctx, root, append([]string{"diff", "--no-color", "--no-ext-diff", "--binary", "--no-renames", r.FixBase, r.FixHead, "--"}, r.Paths...)...)
	if err != nil {
		return err
	}
	if len(patch) == 0 {
		return errors.New("revert has an empty source diff")
	}
	file := filepath.Join(root, ".git", "fault-harness-revert.patch")
	if err := os.WriteFile(file, patch, 0o600); err != nil {
		return err
	}
	defer os.Remove(file)
	if _, err := git(ctx, root, "apply", "-R", "--whitespace=nowarn", file); err != nil {
		return fmt.Errorf("source-only revert does not apply: %w", err)
	}
	// The faulted tree holds the pre-fix lines exactly when the fix applies
	// forward onto it again (FIH-V1-003).
	if _, err := git(ctx, root, "apply", "--check", "--whitespace=nowarn", file); err != nil {
		return fmt.Errorf("revert did not restore the pre-fix lines: %w", err)
	}
	return nil
}
