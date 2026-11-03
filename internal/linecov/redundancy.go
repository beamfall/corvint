package linecov

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Kind classifies a redundancy candidate by its executed blocks relative to
// the other tests of its package (LPCV-V0-964). Coverage cannot see what a
// test asserts, so every kind is a candidate for review, never a verdict.
type Kind string

const (
	Equal     Kind = "equal"      // the same block set as another test
	Subset    Kind = "subset"     // a strict subset of one other test's blocks
	Union     Kind = "union"      // every block also executed by some other test
	LowUnique Kind = "low-unique" // at most the threshold of blocks no other test executes
	NoBlocks  Kind = "no-blocks"  // executed no instrumented block
)

// Candidate is one test the report lists.
type Candidate struct {
	Package, Name string
	Kind          Kind
	Blocks        int           // executed blocks
	Unique        int           // blocks no other test of the package executed
	By            []string      // Equal: peers; Subset: the tightest superset test
	SameBody      []string      // Equal peers whose test body is identical apart from the name
	Requirements  []string      // requirement IDs the test's source cites
	Elapsed       time.Duration `json:",omitempty"` // isolated run time; zero when not recorded
}

// PackageReport is one package's candidates and what was left out.
type PackageReport struct {
	Package    string
	Tests      int // indexed top-level tests considered
	Candidates []Candidate
	Unindexed  []string `json:",omitempty"` // declared but absent from the index: failed alone or added after it
	InOrder    bool     `json:",omitempty"` // an order-sensitive suite record exists and was excluded
}

// Source is what the report reads from one test's declaration.
type Source struct {
	Requirements []string
	BodyHash     string // the declaration with its name blanked; equal hashes mean an identical test
}

var requirementID = regexp.MustCompile(`\b[A-Z][A-Z0-9]*-V\d+-\d+\b`)

// TestSources reads every top-level test declared in dir under root.
func TestSources(root, dir string) (map[string]Source, error) {
	full := filepath.Join(root, filepath.FromSlash(dir))
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	out := map[string]Source{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		if ok, err := build.Default.MatchFile(full, name); err != nil || !ok {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filepath.Join(full, name), nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("linecov: parse %s/%s: %w", dir, name, err)
		}
		src, err := os.ReadFile(filepath.Join(full, name))
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			test, ok := topLevelTest(fn)
			if !ok {
				continue
			}
			from := fn.Pos()
			if fn.Doc != nil {
				from = fn.Doc.Pos()
			}
			text := src[fset.Position(from).Offset:fset.Position(fn.End()).Offset]
			ids := map[string]bool{}
			for _, id := range requirementID.FindAllString(string(text), -1) {
				ids[id] = true
			}
			var s Source
			for id := range ids {
				s.Requirements = append(s.Requirements, id)
			}
			sort.Strings(s.Requirements)
			var body bytes.Buffer
			blank := *fn
			blank.Name = ast.NewIdent("_")
			blank.Doc = nil
			if err := printer.Fprint(&body, fset, &blank); err != nil {
				return nil, err
			}
			sum := sha256.Sum256(body.Bytes())
			s.BodyHash = hex.EncodeToString(sum[:])
			out[test] = s
		}
	}
	return out, nil
}

// Redundancy reports, per package, the tests whose executed blocks another
// test of the same package also executes (LPCV-V0-964). declared lists the
// top-level tests each package declares at the index commit and sources their
// declarations; lowUnique is the most blocks of its own a test may have and
// still be listed. The order-sensitive in-order record is never a candidate
// and never a superset, and a declared test with no record is listed apart.
func (ix *Index) Redundancy(declared map[string][]string, sources map[string]map[string]Source, lowUnique int) []PackageReport {
	sets := make([][]int32, len(ix.Tests)) // test -> ascending global block numbers
	var owners [][]int32                   // global block -> tests executing it
	for _, f := range ix.Files {
		for _, b := range f.Blocks {
			id := int32(len(owners))
			owners = append(owners, b.Tests)
			for _, t := range b.Tests {
				sets[t] = append(sets[t], id)
			}
		}
	}
	byPkg := map[string][]int32{}
	inOrder := map[string]bool{}
	for i, t := range ix.Tests {
		if t.Name == InOrder {
			inOrder[t.Package] = true
			continue
		}
		byPkg[t.Package] = append(byPkg[t.Package], int32(i))
	}
	pkgs := make([]string, 0, len(byPkg))
	for p := range byPkg {
		pkgs = append(pkgs, p)
	}
	for p := range declared {
		if _, ok := byPkg[p]; !ok && len(declared[p]) > 0 {
			pkgs = append(pkgs, p)
		}
	}
	sort.Strings(pkgs)
	var reports []PackageReport
	for _, pkg := range pkgs {
		members := byPkg[pkg]
		in := map[int32]bool{}
		count := map[int32]int{} // block -> tests of the package executing it
		for _, t := range members {
			in[t] = true
			for _, b := range sets[t] {
				count[b]++
			}
		}
		r := PackageReport{Package: pkg, Tests: len(members), InOrder: inOrder[pkg]}
		for _, name := range declared[pkg] {
			if _, ok := ix.byName[pkg+"\x00"+name]; !ok {
				r.Unindexed = append(r.Unindexed, name)
			}
		}
		for _, t := range members {
			test := ix.Tests[t]
			c := Candidate{Package: pkg, Name: test.Name, Blocks: len(sets[t]), Elapsed: test.Elapsed}
			src := sources[pkg][test.Name]
			c.Requirements = src.Requirements
			if len(sets[t]) == 0 {
				c.Kind = NoBlocks
				r.Candidates = append(r.Candidates, c)
				continue
			}
			// Only tests executing t's rarest block can contain t's set.
			rarest := sets[t][0]
			for _, b := range sets[t] {
				if count[b] == 1 {
					c.Unique++
				}
				if count[b] < count[rarest] {
					rarest = b
				}
			}
			var tight int32 = -1
			for _, u := range owners[rarest] {
				if u == t || !in[u] || !subset(sets[t], sets[u]) {
					continue
				}
				if len(sets[u]) == len(sets[t]) {
					name := ix.Tests[u].Name
					c.By = append(c.By, name)
					if other, ok := sources[pkg][name]; ok && src.BodyHash != "" && other.BodyHash == src.BodyHash {
						c.SameBody = append(c.SameBody, name)
					}
				} else if tight < 0 || len(sets[u]) < len(sets[tight]) || len(sets[u]) == len(sets[tight]) && u < tight {
					tight = u
				}
			}
			switch {
			case len(c.By) > 0:
				c.Kind = Equal
			case tight >= 0:
				c.Kind, c.By = Subset, []string{ix.Tests[tight].Name}
			case c.Unique == 0:
				c.Kind = Union
			case c.Unique <= lowUnique:
				c.Kind = LowUnique
			default:
				continue
			}
			r.Candidates = append(r.Candidates, c)
		}
		if len(r.Candidates) > 0 || len(r.Unindexed) > 0 {
			reports = append(reports, r)
		}
	}
	return reports
}

// subset reports whether ascending a is contained in ascending b.
func subset(a, b []int32) bool {
	if len(a) > len(b) {
		return false
	}
	j := 0
	for _, x := range a {
		for j < len(b) && b[j] < x {
			j++
		}
		if j == len(b) || b[j] != x {
			return false
		}
		j++
	}
	return true
}
