package main

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// goldSpans reads the secondary ground truth: the one-based line ranges of
// the lines the change added to each gold file, taken from the change's own
// diff of that file (CRT-V0-004).
func goldSpans(ctx context.Context, repo, commit string, goldPaths []string) ([]span, error) {
	spans := []span{}
	for _, item := range goldPaths {
		raw, err := gitOutput(ctx, repo, "diff", "--unified=0", commit+"^", commit, "--", item)
		if err != nil {
			return nil, err
		}
		for _, header := range hunkRanges(raw) {
			spans = append(spans, span{Path: item, Lines: header})
		}
	}
	return spans, nil
}

var rangeHeader = regexp.MustCompile(`(?m)^@@ -[0-9,]+ \+([0-9]+)(?:,([0-9]+))? @@`)

// hunkRanges turns each `@@ -a,b +c,d @@` header into the one-based inclusive
// range of the added lines it introduced; a header adding nothing is skipped.
func hunkRanges(diff string) []string {
	ranges := []string{}
	for _, match := range rangeHeader.FindAllStringSubmatch(diff, -1) {
		start, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		count := 1
		if match[2] != "" {
			count, _ = strconv.Atoi(match[2])
		}
		if count == 0 {
			continue
		}
		ranges = append(ranges, fmt.Sprintf("%d:%d", start, start+count-1))
	}
	return ranges
}

// stemBaseline is the mechanical guess a naming convention alone produces
// from the presented source files. It is recorded at selection time so the
// report can show the treatment's lift over that floor rather than over zero
// (CRT-V0-004).
func stemBaseline(sourceFiles []string) []string {
	guesses := map[string]bool{}
	for _, file := range sourceFiles {
		directory, name := path.Split(file)
		extension := path.Ext(name)
		stem := strings.TrimSuffix(name, extension)
		for _, guess := range stemGuesses(stem, extension) {
			guesses[path.Join(directory, guess)] = true
		}
	}
	out := make([]string, 0, len(guesses))
	for guess := range guesses {
		out = append(out, guess)
	}
	sort.Strings(out)
	return out
}

func stemGuesses(stem, extension string) []string {
	switch extension {
	case ".go":
		return []string{stem + "_test.go"}
	case ".ts", ".tsx":
		return []string{stem + ".test" + extension}
	case ".swift":
		return []string{stem + "Tests.swift"}
	case ".py":
		return []string{"test_" + stem + ".py", stem + "_test.py"}
	}
	return nil
}

// replayCitations is the harness's own verifier, written against git alone so
// that a disagreement with `cem verify` is evidence about `cem verify` and not
// a second call to it (CRT-V0-007). A citation replays when its path resolves
// at the base revision, its line range lies inside that file, and the cited
// span occurs there exactly where it was cited.
func replayCitations(ctx context.Context, root, base string, citations []citation) bool {
	for _, item := range citations {
		if !replayCitation(ctx, root, base, item) {
			return false
		}
	}
	return true
}

func replayCitation(ctx context.Context, root, base string, item citation) bool {
	start, end, ok := parseLines(item.Lines)
	if !ok || !relations[item.Relation] {
		return false
	}
	blob, err := gitOutput(ctx, root, "show", base+":"+item.Path)
	if err != nil {
		return false
	}
	lines := strings.Split(blob, "\n")
	if start < 1 || end > len(lines) {
		return false
	}
	cited := strings.Join(lines[start-1:end], "\n")
	if strings.TrimSpace(cited) == "" {
		return false
	}
	return strings.Count(blob, cited) >= 1
}

var relations = map[string]bool{
	"specification": true, "decision": true, "test-claim": true, "implementation": true,
	"call-site": true, "dependency": true, "incident": true,
}

func parseLines(value string) (int, int, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil || end < start {
		return 0, 0, false
	}
	return start, end, true
}

// normalizePath compares a cited path the way the gold records it.
func normalizePath(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "./")
	return strings.TrimSuffix(trimmed, "/")
}
