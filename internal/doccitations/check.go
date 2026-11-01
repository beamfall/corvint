// Package doccitations is the product line-citation check for any Git
// repository (SDD-V0-633): it reads tracked Markdown and the cited files' index
// blobs, and reports out-of-range, trivial, anchor-mismatched and missing-file
// citations. It reads only; Corvint's own script/check-line-citations.sh keeps
// its repository-specific scope rules.
package doccitations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// Failure kinds.
const (
	OutOfRange     = "out-of-range"
	TrivialSpan    = "trivial-span"
	AnchorMismatch = "anchor-mismatch"
	MissingFile    = "missing-file"
)

const (
	maxIndexBytes = 16 << 20
	maxBlobBytes  = 256 << 20
	maxFiles      = 1 << 16
)

// Failure is one citation the check refuses.
type Failure struct {
	Document string
	Line     int
	Token    string
	Kind     string
	Detail   string
}

// String renders a failure as "doc:line  `token`  kind: detail".
func (f Failure) String() string {
	return fmt.Sprintf("%s:%d  %s  %s: %s", f.Document, f.Line, f.Token, f.Kind, f.Detail)
}

// Report is the result over the scanned documents.
type Report struct {
	Documents int
	Citations int
	Failures  []Failure
}

var (
	tokenPattern    = regexp.MustCompile("`([^`\n]+)`")
	citationPattern = regexp.MustCompile(`\A(/?[A-Za-z0-9_][A-Za-z0-9_./-]*):([0-9]+)(?:-([0-9]+)|((?:,[0-9]+)*))(?:@([0-9a-f]{4,64}))?\z`)
)

type indexEntry struct{ blob string }

// Check scans the named tracked files, or every tracked .md file when paths is
// empty. An untracked named path, or a repository Git cannot read, is an error.
func Check(ctx context.Context, root string, paths []string) (Report, error) {
	budget := gitrun.NewBudget(3, 2*time.Minute)
	raw, err := git(ctx, budget, root, nil, maxIndexBytes, "ls-files", "--stage", "-z")
	if err != nil {
		return Report{}, fmt.Errorf("cannot read the Git index under %s: %w", root, err)
	}
	index, directories := map[string]indexEntry{}, map[string]bool{}
	for _, record := range bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0}) {
		meta, path, ok := strings.Cut(string(record), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[2] != "0" || !strings.HasPrefix(fields[0], "100") {
			continue
		}
		index[path] = indexEntry{blob: fields[1]}
		if first, _, nested := strings.Cut(path, "/"); nested {
			directories[first] = true
		}
	}
	documents := paths
	if len(documents) == 0 {
		for path := range index {
			if strings.HasSuffix(path, ".md") {
				documents = append(documents, path)
			}
		}
	}
	sort.Strings(documents)
	documents = compactSorted(documents)
	if len(documents) > maxFiles {
		return Report{}, fmt.Errorf("%d documents exceed the limit of %d", len(documents), maxFiles)
	}
	for _, document := range documents {
		if _, tracked := index[document]; !tracked {
			return Report{}, fmt.Errorf("not a tracked regular file: %s", document)
		}
	}
	type citation struct {
		document, token, path, pin string
		line                       int
		numbers                    []int
		first, last                int
		ranged                     bool
	}
	texts, err := blobs(ctx, budget, root, index, documents)
	if err != nil {
		return Report{}, err
	}
	report := Report{Documents: len(documents)}
	var citations []citation
	cited := map[string]bool{}
	for _, document := range documents {
		for n, line := range splitLines(texts[document]) {
			for _, match := range tokenPattern.FindAllStringSubmatch(line, -1) {
				parts := citationPattern.FindStringSubmatch(match[1])
				if parts == nil {
					continue
				}
				path := parts[1]
				if _, tracked := index[path]; !tracked {
					if first, _, nested := strings.Cut(path, "/"); nested && directories[first] {
						report.Citations++
						report.Failures = append(report.Failures, Failure{document, n + 1, match[0], MissingFile, "no such tracked file: " + path})
					}
					continue
				}
				report.Citations++
				c := citation{document: document, token: match[0], path: path, pin: parts[5], line: n + 1}
				c.first, _ = strconv.Atoi(parts[2])
				c.last, c.ranged = c.first, parts[3] != ""
				c.numbers = []int{c.first}
				if c.ranged {
					c.last, _ = strconv.Atoi(parts[3])
					for number := c.first + 1; number <= c.last; number++ {
						c.numbers = append(c.numbers, number)
					}
				} else if parts[4] != "" {
					for _, extra := range strings.Split(parts[4][1:], ",") {
						number, _ := strconv.Atoi(extra)
						c.numbers = append(c.numbers, number)
					}
				}
				citations = append(citations, c)
				cited[path] = true
			}
		}
	}
	var targets []string
	for path := range cited {
		if _, loaded := texts[path]; !loaded {
			targets = append(targets, path)
		}
	}
	sort.Strings(targets)
	more, err := blobs(ctx, budget, root, index, targets)
	if err != nil {
		return Report{}, err
	}
	for path, text := range more {
		texts[path] = text
	}
	lines := map[string][]string{}
	for _, c := range citations {
		target, ok := lines[c.path]
		if !ok {
			target = splitLines(texts[c.path])
			lines[c.path] = target
		}
		fail := func(kind, format string, values ...any) {
			report.Failures = append(report.Failures, Failure{c.document, c.line, c.token, kind, fmt.Sprintf(format, values...)})
		}
		if c.ranged && c.last < c.first {
			fail(OutOfRange, "range end %d is before its start %d", c.last, c.first)
			continue
		}
		if bad := outside(c.numbers, len(target)); bad != 0 {
			fail(OutOfRange, "line %d is outside %s (%d lines)", bad, c.path, len(target))
			continue
		}
		if c.pin != "" {
			if anchor := Anchor(target, c.numbers); !strings.HasPrefix(anchor, c.pin) {
				fail(AnchorMismatch, "cited lines of %s hash to @%s", c.path, anchor[:min(len(c.pin), len(anchor))])
				continue
			}
		}
		checked := c.numbers
		if c.ranged {
			checked = []int{c.first}
		}
		for _, number := range checked {
			if trivial(target[number-1]) {
				fail(TrivialSpan, "%s:%d is a blank or closing-punctuation line", c.path, number)
			}
		}
	}
	sort.SliceStable(report.Failures, func(a, b int) bool {
		left, right := report.Failures[a], report.Failures[b]
		if left.Document != right.Document {
			return left.Document < right.Document
		}
		return left.Line < right.Line
	})
	return report, nil
}

// Anchor is the SHA-256 of the cited right-trimmed lines joined by LF, the
// rule script/check-line-citations.sh pins with; a pin is any prefix of it.
func Anchor(lines []string, numbers []int) string {
	text := make([]string, len(numbers))
	for n, number := range numbers {
		text[n] = strings.TrimRight(lines[number-1], " \t\r\n\f\v")
	}
	sum := sha256.Sum256([]byte(strings.Join(text, "\n")))
	return hex.EncodeToString(sum[:])
}

func outside(numbers []int, count int) int {
	for _, number := range numbers {
		if number < 1 || number > count {
			return number
		}
	}
	return 0
}

func trivial(line string) bool {
	return strings.Trim(strings.TrimSpace(line), "}),];") == ""
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

func compactSorted(values []string) []string {
	out := values[:0:0]
	for n, value := range values {
		if n == 0 || value != values[n-1] {
			out = append(out, value)
		}
	}
	return out
}

// blobs reads the index blobs of paths with one `git cat-file --batch`.
func blobs(ctx context.Context, budget *gitrun.Budget, root string, index map[string]indexEntry, paths []string) (map[string]string, error) {
	texts := map[string]string{}
	if len(paths) == 0 {
		return texts, nil
	}
	var request strings.Builder
	for _, path := range paths {
		request.WriteString(index[path].blob + "\n")
	}
	raw, err := git(ctx, budget, root, []byte(request.String()), maxBlobBytes, "cat-file", "--batch")
	if err != nil {
		return nil, fmt.Errorf("cannot read indexed blobs: %w", err)
	}
	for _, path := range paths {
		header, rest, ok := bytes.Cut(raw, []byte{'\n'})
		fields := strings.Fields(string(header))
		if !ok || len(fields) != 3 || fields[0] != index[path].blob || fields[1] != "blob" {
			return nil, fmt.Errorf("unexpected cat-file output for %s", path)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || len(rest) < size+1 {
			return nil, fmt.Errorf("truncated cat-file output for %s", path)
		}
		texts[path], raw = string(rest[:size]), rest[size+1:]
	}
	return texts, nil
}

func git(ctx context.Context, budget *gitrun.Budget, root string, stdin []byte, limit int, args ...string) ([]byte, error) {
	return gitrun.Run(ctx, budget, gitrun.Options{Dir: root, Env: environment(), Stdin: stdin, StdoutLimit: limit}, append([]string{"--no-replace-objects", "-c", "core.quotePath=false"}, args...)...)
}

// environment is the scrubbed child environment other gitrun callers pass.
func environment() []string {
	values := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			values = append(values, name+"="+value)
		}
	}
	return append(values,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}
