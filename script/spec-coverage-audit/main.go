package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type requirement struct{ id, spec string }
type mentions struct{ evidence, comments map[string]bool }
type scanResult struct {
	mentions         map[string]*mentions
	binary, symlinks int
}

var literalID = regexp.MustCompile(`[A-Z][A-Z0-9-]*-[0-9]{3}`)
var testName = regexp.MustCompile(`(^test_|_test\.|[._](test|spec)\.)`)

func member(s string, options ...string) bool {
	for _, v := range options {
		if s == v {
			return true
		}
	}
	return false
}
func loadRequirements(path string) ([]requirement, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 || strings.Join(rows[0], "\t") != "id\tfile\tline\ttitle" {
		return nil, fmt.Errorf("invalid requirement registry: %s", path)
	}
	out := []requirement{}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		line, e := strconv.Atoi(row[2])
		if row[0] == "" || row[1] == "" || e != nil || line < 1 {
			return nil, fmt.Errorf("invalid requirement row in %s", path)
		}
		if seen[row[0]] {
			return nil, fmt.Errorf("duplicate requirement IDs in %s", path)
		}
		seen[row[0]] = true
		out = append(out, requirement{row[0], row[1]})
	}
	return out, nil
}
func classification(path string) (test, fixture, caseArtifact bool) {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(path)), "/")
	name := parts[len(parts)-1]
	test = testName.MatchString(name)
	for _, p := range parts {
		test = test || p == "tests"
		fixture = fixture || member(p, "fixture", "fixtures", "testdata", "golden", "goldens", "snapshot", "snapshots")
	}
	caseArtifact = parts[0] == "conformance" && (member(name, "case.json", "cases.json", "manifest.json") || strings.Contains(name, ".tcq."))
	for _, p := range parts {
		caseArtifact = caseArtifact || (parts[0] == "conformance" && p == "vectors")
	}
	return
}
func idByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
}
func digit(c byte) bool { return c >= '0' && c <= '9' }

type matcher struct {
	known    map[string]bool
	stripped map[string]string
	pattern  *regexp.Regexp
}

func newMatcher(reqs []requirement) *matcher {
	m := &matcher{known: map[string]bool{}, stripped: map[string]string{}}
	forms := []string{}
	for _, r := range reqs {
		m.known[r.id] = true
		s := strings.ReplaceAll(r.id, "-", "")
		m.stripped[s] = r.id
	}
	for s := range m.stripped {
		forms = append(forms, s)
	}
	sort.Slice(forms, func(i, j int) bool {
		if len(forms[i]) != len(forms[j]) {
			return len(forms[i]) > len(forms[j])
		}
		return forms[i] < forms[j]
	})
	if len(forms) > 0 {
		m.pattern = regexp.MustCompile(strings.Join(forms, "|"))
	}
	return m
}
func (m *matcher) each(data []byte, visit func(string, int)) {
	for _, loc := range literalID.FindAllIndex(data, -1) {
		if loc[0] > 0 && idByte(data[loc[0]-1]) || loc[1] < len(data) && idByte(data[loc[1]]) {
			continue
		}
		id := string(data[loc[0]:loc[1]])
		if m.known[id] {
			visit(id, loc[0])
		}
	}
	if m.pattern != nil {
		for _, loc := range m.pattern.FindAllIndex(data, -1) {
			if loc[0] > 0 && digit(data[loc[0]-1]) || loc[1] < len(data) && digit(data[loc[1]]) {
				continue
			}
			visit(m.stripped[string(data[loc[0]:loc[1]])], loc[0])
		}
	}
}
func scan(root string, reqs []requirement) (scanResult, error) {
	r := scanResult{mentions: map[string]*mentions{}}
	m := newMatcher(reqs)
	for _, req := range reqs {
		r.mentions[req.id] = &mentions{map[string]bool{}, map[string]bool{}}
	}
	for _, name := range []string{"conformance", "internal", "cmd", "tools", "tests"} {
		err := filepath.WalkDir(filepath.Join(root, name), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) && p == filepath.Join(root, name) {
					return nil
				}
				return err
			}
			if p == filepath.Join(root, name) {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			test, fixture, caseArtifact := classification(rel)
			if test || fixture || caseArtifact {
				m.each([]byte(rel), func(id string, _ int) { r.mentions[id].evidence["artifact-name:"+rel] = true })
			}
			if d.Type()&os.ModeSymlink != 0 {
				r.symlinks++
				return nil
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if bytes.IndexByte(raw, 0) >= 0 {
				r.binary++
				return nil
			}
			if test && !fixture || caseArtifact {
				m.each(raw, func(id string, _ int) { r.mentions[id].evidence[rel] = true })
				return nil
			}
			if member(strings.ToLower(filepath.Ext(p)), ".c", ".cc", ".cpp", ".go", ".h", ".java", ".js", ".jsx", ".kt", ".kts", ".m", ".mm", ".py", ".rb", ".rs", ".sh", ".swift", ".ts", ".tsx", ".zsh") {
				for _, line := range bytes.Split(raw, []byte("\n")) {
					m.each(line, func(id string, start int) {
						comment := bytes.HasPrefix(bytes.TrimSpace(line), []byte("*"))
						for _, marker := range []string{"//", "#", "/*", "<!--"} {
							comment = comment || bytes.Contains(line[:start], []byte(marker))
						}
						if comment {
							r.mentions[id].comments[rel] = true
						}
					})
				}
			}
			return nil
		})
		if err != nil {
			return r, err
		}
	}
	return r, nil
}
func render(reqs []requirement, r scanResult) string {
	bySpec := map[string][]string{}
	evidence, comments := 0, 0
	for _, req := range reqs {
		bySpec[req.spec] = append(bySpec[req.spec], req.id)
		m := r.mentions[req.id]
		if len(m.evidence) > 0 {
			evidence++
		} else if len(m.comments) > 0 {
			comments++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Requirement-ID mention inventory\n\n> **Warning:** literal citation is weak evidence. This inventory shows only that an\n> eligible artifact mentions a requirement ID; it does **not** show that a case would\n> fail if the requirement were violated, and it is not a verification claim.\n\nRegistry: %d unique requirement IDs. Test/case/fixture mention: %d. Comment only: %d. No eligible mention: %d.\nEvidence surface: named test files, conformance case/manifest/vector artifacts, and fixture pathnames; other source-code comments are separated.\nSkipped: %d binary files and %d symlinks.\n\n| Owning spec | Requirement IDs | Test/case/fixture mention | Comment only | No eligible mention |\n|---|---:|---:|---:|---:|\n", len(reqs), evidence, comments, len(reqs)-evidence-comments, r.binary, r.symlinks)
	specs := []string{}
	for s := range bySpec {
		specs = append(specs, s)
	}
	sort.Strings(specs)
	for _, s := range specs {
		e, c := 0, 0
		for _, id := range bySpec[s] {
			m := r.mentions[id]
			if len(m.evidence) > 0 {
				e++
			} else if len(m.comments) > 0 {
				c++
			}
		}
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d |\n", s, len(bySpec[s]), e, c, len(bySpec[s])-e-c)
	}
	return b.String()
}
func main() {
	reqs, err := loadRequirements("docs/specs/REQUIREMENTS.tsv")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r, err := scan(".", reqs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(render(reqs, r))
}
