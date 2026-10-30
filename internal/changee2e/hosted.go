package changee2e

import (
	"bytes"
	"context"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// HostedSelection selects an existing hosted Playwright test that Prove runs as given, without
// compilation (CCE-V0-034).
type HostedSelection struct {
	File  string // repository-relative test file, read at head
	Title string // the test's own title; optional when the file declares exactly one test
}

// hostedSuffixes are the Playwright test file names a hosted test may have.
var hostedSuffixes = []string{".spec.ts", ".test.ts", ".spec.tsx", ".test.tsx", ".spec.js", ".test.js", ".spec.mjs", ".test.mjs", ".spec.cjs", ".test.cjs"}

// testDecl matches a `test(` or `test.only(` declaration and its literal title.
var testDecl = regexp.MustCompile("^\\s*test(?:\\.only)?\\(\\s*(?:'((?:[^'\\\\]|\\\\.)*)'|\"((?:[^\"\\\\]|\\\\.)*)\"|`([^`\\\\$]*)`)\\s*,")

// hostedScenario reads the hosted test at head into a scenario: its assertions are the outcome
// assertions (CCE-V0-035), the awaited action before each one is a behaviour-removal control
// (CCE-V0-036), and its oracle_origin joins the test file at base and head (CCE-V0-034).
func hostedScenario(ctx context.Context, root, base, head string, h HostedSelection) (Scenario, error) {
	p := h.File
	if !hostedPlacement(p) {
		return Scenario{}, refuse("e2e-hosted-test-invalid", "test file %q is not a clean relative Playwright test path", p)
	}
	text, _, ok, err := blobAt(ctx, root, head, p)
	if err != nil {
		return Scenario{}, refuse("e2e-hosted-test-invalid", "read %s at head: %v", p, err)
	}
	if !ok {
		return Scenario{}, refuse("e2e-hosted-test-invalid", "%s is not a file at head", p)
	}
	lines := strings.Split(string(text), "\n")
	title, decl, err := selectTest(lines, h.Title)
	if err != nil {
		return Scenario{}, err
	}
	end := bodyEnd(lines, decl)
	var assertions, events []int
	for i := decl + 1; i <= end; i++ {
		body := strings.TrimSpace(lines[i-1])
		if !strings.HasPrefix(body, "expect(") && !strings.HasPrefix(body, "await expect(") {
			continue
		}
		event := 0
		for j := i - 1; j > decl; j-- {
			b := strings.TrimSpace(lines[j-1])
			if strings.HasPrefix(b, "await ") && !strings.Contains(b, "expect(") {
				event = j
				break
			}
		}
		if event == 0 {
			return Scenario{}, refuse("e2e-hosted-test-invalid", "assertion at %s:%d has no preceding awaited action for its behaviour-removal control", p, i)
		}
		assertions = append(assertions, i)
		if len(events) == 0 || events[len(events)-1] != event {
			events = append(events, event)
		}
	}
	if len(assertions) == 0 {
		return Scenario{}, refuse("e2e-hosted-test-invalid", "test %q in %s has no assertion line", title, p)
	}
	origin := OriginNewInDiff
	if baseText, _, ok, err := blobAt(ctx, root, base, p); err != nil {
		return Scenario{}, refuse("e2e-hosted-test-invalid", "read %s at base: %v", p, err)
	} else if ok && bytes.Equal(baseText, text) {
		origin = OriginAdmittedAtBase
	} else if ok {
		origin = OriginChangedInDiff
	}
	return Scenario{Obligation: "hosted:" + p + "::" + title, Placement: p, Text: text, TestTitle: title,
		AssertionID: "hosted:" + p + ":" + strconv.Itoa(assertions[0]), AssertionLine: assertions[0], OracleOrigin: origin,
		Hosted: true, DeclLine: decl, AssertionLines: assertions, EventLines: events}, nil
}

func hostedPlacement(p string) bool {
	if p == "" || path.IsAbs(p) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") || strings.Contains(p, "node_modules") {
		return false
	}
	for _, s := range hostedSuffixes {
		if strings.HasSuffix(p, s) {
			return true
		}
	}
	return false
}

// selectTest returns the selected test's title and 1-based declaration line: the declaration whose
// literal title equals want, or the file's only declaration when want is empty.
func selectTest(lines []string, want string) (string, int, error) {
	var titles []string
	var decls []int
	for i, l := range lines {
		m := testDecl.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		title := m[1] + m[2] + m[3]
		if want == "" || title == want {
			titles, decls = append(titles, title), append(decls, i+1)
		}
	}
	switch {
	case len(decls) == 1:
		return titles[0], decls[0], nil
	case len(decls) == 0 && want != "":
		return "", 0, refuse("e2e-hosted-test-invalid", "no test titled %q", want)
	case len(decls) == 0:
		return "", 0, refuse("e2e-hosted-test-invalid", "the file declares no test with a literal title")
	}
	return "", 0, refuse("e2e-hosted-test-ambiguous", "%d tests match; select one by its title", len(decls))
}

// bodyEnd is the 1-based line that closes the test declared on decl, by brace balance from the
// declaration; an unbalanced body runs to the end of the file.
func bodyEnd(lines []string, decl int) int {
	depth, opened := 0, false
	for i := decl; i <= len(lines); i++ {
		for _, r := range lines[i-1] {
			switch r {
			case '{':
				depth, opened = depth+1, true
			case '}':
				depth--
			}
		}
		if opened && depth <= 0 {
			return i
		}
	}
	return len(lines)
}

// grepTitle is a Playwright --grep pattern matching the title literally.
func grepTitle(title string) string {
	return regexp.QuoteMeta(title)
}
