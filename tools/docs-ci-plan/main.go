// SPDX-License-Identifier: AGPL-3.0-or-later
// docs-ci-plan implements the repository-specific DCI-V0 policy, not affected-plan narrowing.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const readme = "README.md"
const citationFile = "docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md"
const qualificationFile = ".github/qualifications/docs-only.json"
const profile = "corvint-docs-ci/0"
const maxOutput = 8 << 20

var checks = []string{"immutable-local-links", "unchanged-executable-blocks", "identical-citation-spans", "doc-gates", "docs-ci-plan-tests", "specindex-tests", "full-static-build-interop-archive"}
var oidPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var citation = regexp.MustCompile(`README\.md:([0-9]+)-([0-9]+)@([0-9a-f]{8})`)
var markdownLink = regexp.MustCompile(`\]\(\s*([^\s)]+)(?:\s+"[^"]*")?\s*\)`)
var containerIndent = regexp.MustCompile(`^ {0,3}(?:[-+*]|[0-9]{1,9}[.)])(?: {2,}|\t)`)
var referenceLink = regexp.MustCompile(`(?m)^ {0,3}\[[^]\n]+\]:|\[[^]\n]+\]\[[^]\n]*\]`)

type qualification struct {
	Profile    string   `json:"profile"`
	Approved   bool     `json:"approved"`
	Tree       string   `json:"treeSha256"`
	Classifier string   `json:"classifierSha256"`
	Checks     []string `json:"checks"`
	Inventory  string   `json:"inventory"`
	Review     string   `json:"review"`
}

type receipt struct {
	Profile       string   `json:"profile"`
	Mode          string   `json:"mode"`
	Reason        string   `json:"reason"`
	Base          string   `json:"base"`
	Head          string   `json:"head"`
	Target        string   `json:"target"`
	Qualification string   `json:"qualificationSha256,omitempty"`
	Tree          string   `json:"treeSha256,omitempty"`
	Classifier    string   `json:"classifierSha256,omitempty"`
	Changed       []string `json:"changed"`
	Checks        []string `json:"checks"`
	Verified      bool     `json:"verified"`
	Unknown       string   `json:"unknown"`
}

type repo struct{ root string }

// Commands never invoke a shell, read global/system Git config, fetch objects, or honour replacements.
// CI supplies a fresh checkout with trusted repository-local config and executable PATH.
func (r repo) git(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-replace-objects", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-C", r.root}, args...)...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=/nonexistent", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0"}
	var b limitedBuffer
	cmd.Stdout = &b
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if err != nil {
		return nil, errors.New("git-read-failed")
	}
	return b.Bytes(), nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxOutput {
		return 0, errors.New("git-output-limit")
	}
	return b.Buffer.Write(p)
}
func digest(b []byte) string                         { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func (r repo) blob(rev, name string) ([]byte, error) { return r.git("show", rev+":"+name) }

// Tree rows include pathname, mode, object type and immutable identity. Only the two
// reviewed inputs and the non-executable qualification artifact are exempt from drift.
func (r repo) identities(rev string) (string, string, error) {
	b, err := r.git("ls-tree", "-r", "-z", rev)
	if err != nil {
		return "", "", err
	}
	var all, tool bytes.Buffer
	for _, row := range bytes.Split(b, []byte{0}) {
		if len(row) == 0 {
			continue
		}
		parts := bytes.SplitN(row, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return "", "", errors.New("invalid-tree")
		}
		name := string(parts[1])
		meta := strings.Fields(string(parts[0]))
		if len(meta) != 3 || meta[1] != "blob" || (meta[0] != "100644" && meta[0] != "100755") {
			return "", "", errors.New("unsupported-tree-entry")
		}
		if name == readme || name == citationFile || name == qualificationFile {
			continue
		}
		all.Write(row)
		all.WriteByte(0)
		if strings.HasPrefix(name, "tools/docs-ci-plan/") {
			tool.Write(row)
			tool.WriteByte(0)
		}
	}
	if tool.Len() == 0 {
		return "", "", errors.New("classifier-source-missing")
	}
	return digest(all.Bytes()), digest(tool.Bytes()), nil
}

func (r repo) stable(target string) error {
	b, err := r.git("rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(b)) != target {
		return errors.New("target-head-mismatch")
	}
	b, err = r.git("status", "--porcelain=v1", "--untracked-files=all")
	if err != nil || len(b) != 0 {
		return errors.New("dirty-worktree")
	}
	b, err = r.git("rev-parse", "--git-path", "info/grafts")
	if err != nil {
		return err
	}
	g := strings.TrimSpace(string(b))
	if !filepath.IsAbs(g) {
		g = filepath.Join(r.root, g)
	}
	if _, err = os.Lstat(g); !os.IsNotExist(err) {
		return errors.New("grafts-present")
	}
	b, err = r.git("for-each-ref", "--format=%(refname)", "refs/replace/")
	if err != nil || len(b) != 0 {
		return errors.New("replacements-present")
	}
	return nil
}

func (r repo) plan(base, head, target string) receipt {
	p := receipt{Profile: profile, Mode: "FULL", Reason: "invalid-input", Base: base, Head: head, Target: target, Changed: []string{}, Checks: []string{}, Unknown: "Dynamic-reader uncertainty remains; DOCS is a reviewed repository policy, not equivalent Go coverage."}
	fail := func(err error) receipt { p.Reason = err.Error(); return p }
	if !oidPattern.MatchString(base) || !oidPattern.MatchString(head) || !oidPattern.MatchString(target) {
		return p
	}
	if err := r.stable(target); err != nil {
		return fail(err)
	}
	b, err := r.git("rev-list", "--parents", "-n", "1", target)
	if err != nil {
		return fail(err)
	}
	if !slices.Equal(strings.Fields(string(b)), []string{target, base, head}) {
		return fail(errors.New("merge-topology-mismatch"))
	}
	b, err = r.blob(base, qualificationFile)
	if err != nil {
		return fail(errors.New("qualification-missing"))
	}
	var q qualification
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || d.Decode(new(any)) != io.EOF || q.Profile != "corvint-docs-ci-qualification/0" || !q.Approved || q.Inventory != "docs/specs/documentation-ci-v0.md#consumer-inventory" || strings.TrimSpace(q.Review) == "" || !slices.Equal(q.Checks, checks) {
		return fail(errors.New("qualification-unapproved-or-invalid"))
	}
	p.Qualification = digest(b)
	p.Tree, p.Classifier, err = r.identities(base)
	if err != nil {
		return fail(err)
	}
	if p.Tree != q.Tree || p.Classifier != q.Classifier {
		return fail(errors.New("qualification-source-drift"))
	}
	b, err = r.git("diff-tree", "--no-commit-id", "--no-renames", "-r", "--raw", "-z", base, target)
	if err != nil {
		return fail(err)
	}
	rows := bytes.Split(b, []byte{0})
	for i := 0; i < len(rows)-1; i += 2 {
		if i+1 >= len(rows)-1 {
			return fail(errors.New("invalid-diff"))
		}
		meta := strings.Fields(string(rows[i]))
		name := string(rows[i+1])
		if len(meta) != 5 || meta[0] != ":100644" || meta[1] != "100644" || meta[4] != "M" {
			return fail(errors.New("unsupported-change-kind"))
		}
		if name != readme && name != citationFile {
			return fail(errors.New("outside-documentation-scope"))
		}
		p.Changed = append(p.Changed, name)
	}
	if len(p.Changed) == 0 || !slices.Contains(p.Changed, readme) {
		return fail(errors.New("readme-change-required"))
	}
	old, err := r.blob(base, readme)
	if err != nil {
		return fail(err)
	}
	newBody, err := r.blob(target, readme)
	if err != nil {
		return fail(err)
	}
	a, err := executableBlocks(old)
	if err != nil {
		return fail(err)
	}
	c, err := executableBlocks(newBody)
	if err != nil {
		return fail(err)
	}
	if !bytes.Equal(a, c) {
		return fail(errors.New("executable-block-changed"))
	}
	if slices.Contains(p.Changed, citationFile) {
		x, e := r.blob(base, citationFile)
		if e != nil {
			return fail(e)
		}
		y, e := r.blob(target, citationFile)
		if e != nil {
			return fail(e)
		}
		if !citationOnly(x, y, old, newBody) {
			return fail(errors.New("nonmechanical-citation-change"))
		}
	}
	if err = r.stable(target); err != nil {
		return fail(err)
	}
	p.Mode = "DOCS"
	p.Reason = "qualified-presentation-change"
	p.Checks = append([]string{}, checks...)
	return p
}

// This deliberately accepts a small Markdown subset. Unsupported containers select FULL.
func splitCode(b []byte) ([]byte, []byte, error) {
	if len(b) > 1<<20 || !utf8.Valid(b) || bytes.ContainsAny(b, "\x00\r") {
		return nil, nil, errors.New("unsupported-document-bytes")
	}
	var code, prose bytes.Buffer
	var marker byte
	size := 0
	for _, line := range strings.SplitAfter(string(b), "\n") {
		t := strings.TrimLeft(line, " ")
		indent := len(line) - len(t)
		fence := false
		if indent <= 3 && len(t) > 0 && (t[0] == '`' || t[0] == '~') {
			n := 0
			for n < len(t) && t[n] == t[0] {
				n++
			}
			if n >= 3 {
				if marker == 0 {
					marker = t[0]
					size = n
					fence = true
				} else if marker == t[0] && n >= size && strings.TrimSpace(t[n:]) == "" {
					marker = 0
					fence = true
				}
			}
		}
		leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if fence || marker != 0 || strings.HasPrefix(line, "    ") || strings.Contains(leading, "\t") || strings.HasPrefix(t, ">") || containerIndent.MatchString(line) {
			code.WriteString(line)
			prose.WriteByte('\n')
			continue
		}
		if strings.Contains(line, "```") || strings.Contains(line, "~~~") {
			return nil, nil, errors.New("unsupported-nested-fence")
		}
		prose.WriteString(line)
	}
	if marker != 0 {
		return nil, nil, errors.New("unclosed-code-fence")
	}
	return code.Bytes(), prose.Bytes(), nil
}
func executableBlocks(b []byte) ([]byte, error) {
	code, prose, err := splitCode(b)
	if err != nil {
		return nil, err
	}
	links := markdownLink.FindAllSubmatch(prose, -1)
	if len(links) != bytes.Count(prose, []byte("](")) {
		return nil, errors.New("unsupported-inline-link")
	}
	for _, link := range links {
		if bytes.ContainsAny(link[1], "(\\<>") {
			return nil, errors.New("unsupported-inline-link")
		}
	}
	if referenceLink.Match(prose) {
		return nil, errors.New("unsupported-reference-link")
	}
	if _, err = htmlLinks(prose); err != nil {
		return nil, err
	}
	return code, nil
}

// Quote-aware scanning, with closed tags/attributes and no unquoted values. This is
// a conservative admission check, not a general HTML parser or sanitizer.
func htmlLinks(b []byte) ([]string, error) {
	allowed := map[string]bool{"p": true, "picture": true, "source": true, "img": true, "h1": true, "h2": true, "h3": true, "h4": true, "a": true, "strong": true, "em": true, "details": true, "summary": true, "br": true}
	attrs := map[string]bool{"align": true, "media": true, "srcset": true, "src": true, "width": true, "height": true, "alt": true, "href": true, "title": true}
	var links []string
	bad := func() ([]string, error) { return nil, errors.New("unsupported-html") }
	letter := func(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' }
	space := func(c byte) bool { return c == ' ' || c == '\n' || c == '\t' }
	for i := 0; i < len(b); i++ {
		if b[i] != '<' {
			continue
		}
		i++
		if i == len(b) {
			return bad()
		}
		closing := false
		if b[i] == '/' {
			closing = true
			i++
		}
		start := i
		for i < len(b) && letter(b[i]) {
			i++
		}
		tag := strings.ToLower(string(b[start:i]))
		if !allowed[tag] {
			return bad()
		}
		if i < len(b) && b[i] != '>' && !space(b[i]) && b[i] != '/' {
			return bad()
		}
		seen := map[string]bool{}
		for {
			for i < len(b) && space(b[i]) {
				i++
			}
			if i >= len(b) {
				return bad()
			}
			if b[i] == '>' {
				break
			}
			if b[i] == '/' && i+1 < len(b) && b[i+1] == '>' && !closing {
				i++
				break
			}
			if closing {
				return bad()
			}
			start = i
			for i < len(b) && letter(b[i]) {
				i++
			}
			attr := strings.ToLower(string(b[start:i]))
			if !attrs[attr] || seen[attr] {
				return bad()
			}
			seen[attr] = true
			for i < len(b) && space(b[i]) {
				i++
			}
			if i >= len(b) || b[i] != '=' {
				return bad()
			}
			i++
			for i < len(b) && space(b[i]) {
				i++
			}
			if i >= len(b) || (b[i] != '\'' && b[i] != '"') {
				return bad()
			}
			quote := b[i]
			i++
			start = i
			for i < len(b) && b[i] != quote {
				i++
			}
			if i == len(b) {
				return bad()
			}
			value := html.UnescapeString(string(b[start:i]))
			i++
			if i < len(b) && !space(b[i]) && b[i] != '>' && b[i] != '/' {
				return bad()
			}
			if attr == "srcset" && strings.ContainsAny(value, ", \n\t") {
				return bad()
			}
			if attr == "href" || attr == "src" || attr == "srcset" {
				links = append(links, value)
			}
		}
	}
	return links, nil
}

func span(b []byte, start, end int) []byte {
	lines := bytes.SplitAfter(b, []byte{'\n'})
	if start < 1 || end < start || end > len(lines) {
		return nil
	}
	return bytes.Join(lines[start-1:end], nil)
}
func citationOnly(a, b, old, newBody []byte) bool {
	x, y := citation.FindAllSubmatchIndex(a, -1), citation.FindAllSubmatchIndex(b, -1)
	if len(x) == 0 || len(x) != len(y) {
		return false
	}
	ax, bx := 0, 0
	for i, m := range x {
		n := y[i]
		if !bytes.Equal(a[ax:m[0]], b[bx:n[0]]) {
			return false
		}
		start, _ := strconv.Atoi(string(a[m[2]:m[3]]))
		end, _ := strconv.Atoi(string(a[m[4]:m[5]]))
		ns, _ := strconv.Atoi(string(b[n[2]:n[3]]))
		ne, _ := strconv.Atoi(string(b[n[4]:n[5]]))
		before, after := span(old, start, end), span(newBody, ns, ne)
		hash, nh := a[m[6]:m[7]], b[n[6]:n[7]]
		if before == nil || !bytes.Equal(before, after) || !bytes.Equal(hash, nh) || digest(before)[:8] != string(hash) {
			return false
		}
		ax, bx = m[1], n[1]
	}
	return bytes.Equal(a[ax:], b[bx:])
}

func headingIDs(b []byte) map[string]bool {
	ids := map[string]bool{}
	_, prose, err := splitCode(b)
	if err != nil {
		return ids
	}
	b = prose
	counts := map[string]int{}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		text := strings.TrimLeft(line, "#")
		if !strings.HasPrefix(text, " ") {
			continue
		}
		var s strings.Builder
		for _, c := range strings.ToLower(strings.TrimSpace(text)) {
			if unicode.IsLetter(c) || unicode.IsNumber(c) || c == '_' || c == '-' {
				s.WriteRune(c)
			} else if c == ' ' {
				s.WriteByte('-')
			}
		}
		id := s.String()
		n := counts[id]
		counts[id]++
		if n > 0 {
			id += "-" + strconv.Itoa(n)
		}
		ids[id] = true
	}
	return ids
}

func (r repo) verify(p *receipt) error {
	b, err := r.blob(p.Target, readme)
	if err != nil {
		return err
	}
	_, prose, err := splitCode(b)
	if err != nil {
		return err
	}
	links, err := htmlLinks(prose)
	if err != nil {
		return err
	}
	for _, m := range markdownLink.FindAllSubmatch(prose, -1) {
		links = append(links, string(m[1]))
	}
	for _, link := range links {
		u, err := url.Parse(link)
		if err != nil {
			return errors.New("invalid-link")
		}
		if u.Scheme == "https" || u.Scheme == "http" {
			if u.Host == "" {
				return errors.New("invalid-remote-link")
			}
			continue
		}
		if u.Scheme != "" || u.Host != "" || strings.HasPrefix(u.Path, "/") {
			return errors.New("unsupported-link")
		}
		name := path.Clean(u.Path)
		if name == "." {
			name = readme
		}
		if name == ".." || strings.HasPrefix(name, "../") {
			return errors.New("escaping-link")
		}
		content, err := r.blob(p.Target, name)
		if err != nil {
			if u.Fragment != "" {
				return errors.New("missing-link-target")
			}
			kind, e := r.git("cat-file", "-t", p.Target+":"+name)
			if e != nil || strings.TrimSpace(string(kind)) != "tree" {
				return errors.New("missing-link-target")
			}
			continue
		}
		if u.Fragment != "" && strings.HasSuffix(name, ".md") && !headingIDs(content)[u.Fragment] {
			return errors.New("missing-heading-anchor")
		}
	}
	// The actual citation consumer is checked even when its file did not change.
	c, err := r.blob(p.Target, citationFile)
	if err != nil {
		return err
	}
	for _, m := range citation.FindAllSubmatch(c, -1) {
		s, _ := strconv.Atoi(string(m[1]))
		e, _ := strconv.Atoi(string(m[2]))
		v := span(b, s, e)
		if v == nil || digest(v)[:8] != string(m[3]) {
			return errors.New("broken-readme-citation")
		}
	}
	if err = r.stable(p.Target); err != nil {
		return err
	}
	p.Verified = true
	return nil
}

func main() {
	fs := flag.NewFlagSet("docs-ci-plan", flag.ExitOnError)
	root := fs.String("root", ".", "tested checkout")
	base := fs.String("base", "", "full base SHA")
	head := fs.String("head", "", "full PR head SHA")
	target := fs.String("target", "", "full tested merge SHA")
	proposal := fs.Bool("qualification-proposal", false, "emit an unapproved qualification for --base; does not authorize skipping")
	fs.Parse(os.Args[1:])
	r := repo{*root}
	if *proposal {
		if !oidPattern.MatchString(*base) {
			fmt.Fprintln(os.Stderr, "full base SHA required")
			os.Exit(2)
		}
		t, c, e := r.identities(*base)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(2)
		}
		json.NewEncoder(os.Stdout).Encode(qualification{Profile: "corvint-docs-ci-qualification/0", Tree: t, Classifier: c, Checks: checks, Inventory: "docs/specs/documentation-ci-v0.md#consumer-inventory"})
		return
	}
	p := r.plan(*base, *head, *target)
	exit := 0
	if p.Mode == "DOCS" {
		if err := r.verify(&p); err != nil {
			p.Reason = err.Error()
			exit = 1
		}
	}
	json.NewEncoder(os.Stdout).Encode(p)
	os.Exit(exit)
}
