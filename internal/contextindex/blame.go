package contextindex

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TCP-V0-036/037 (decision 0369): the blame last-touch feature and the
// CODEOWNERS/blame ownership check. Blame is bounded in files, in history
// (the recency window) and in output bytes; beyond any bound it abstains.
const (
	contextBlameCap       = 10
	contextBlameMaxBytes  = 4 << 20
	contextOwnersMaxBytes = 256 << 10
)

// codeOwnersLocations is GitHub's lookup order; the first tracked file wins.
var codeOwnersLocations = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}

// blameTouch is one blamed path. inWindow counts the current lines whose last
// change is a commit the window reaches; a boundary line (older than the
// window, a root commit's, or the full window's oldest commit, which Git marks
// as the OLDEST..COMMIT range boundary) is outside it.
type blameTouch struct {
	state           string
	lines, inWindow int
	freshness       float64
	newestDays      int64
	authors         map[string]int
	owners          ownershipCheck
}

type blameCommit struct {
	mail     string
	when     int64
	boundary bool
}

// blameHead blames the first contextBlameCap rows, in their current order;
// recencyLexical passes only the rows take could still admit.
func (recency *contextRecency) blameHead(rows []contextRow) {
	if recency.state != "examined" {
		return
	}
	for _, row := range rows[:min(contextBlameCap, len(rows))] {
		recency.blame[row.path] = recency.blamePath(row.path)
		recency.blamed++
	}
}

func (recency *contextRecency) blamePath(candidate string) blameTouch {
	arguments := []string{"blame", "--porcelain", recency.index.CommitRevision, "--", candidate}
	if recency.windowFull {
		arguments[2] = recency.oldest + ".." + recency.index.CommitRevision
	}
	raw, err := git(recency.ctx, recency.index.Root, contextBlameMaxBytes, nil, arguments...)
	if err != nil {
		return blameTouch{state: "blame-unreadable"}
	}
	touch := recency.touch(parseBlame(raw))
	touch.owners = recency.checkOwners(candidate, touch.authors)
	return touch
}

// parseBlame reads `git blame --porcelain`: a header line per line group,
// commit details the first time a commit appears, then the tab-led content.
func parseBlame(raw []byte) (map[string]*blameCommit, map[string]int) {
	commits, lines := map[string]*blameCommit{}, map[string]int{}
	current, header := "", true
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "\t"):
			lines[current]++
			header = true
		case header && strings.TrimSpace(line) != "":
			current, header = strings.Fields(line)[0], false
			if commits[current] == nil {
				commits[current] = &blameCommit{}
			}
		case strings.HasPrefix(line, "author-mail "):
			commits[current].mail = strings.Trim(strings.TrimPrefix(line, "author-mail "), "<>")
		case strings.HasPrefix(line, "committer-time "):
			commits[current].when, _ = strconv.ParseInt(strings.TrimPrefix(line, "committer-time "), 10, 64)
		case line == "boundary":
			commits[current].boundary = true
		}
	}
	return commits, lines
}

func (recency *contextRecency) touch(commits map[string]*blameCommit, lines map[string]int) blameTouch {
	touch := blameTouch{state: "examined", authors: map[string]int{}, newestDays: -1}
	weighted := 0.0
	for commit, count := range lines {
		touch.lines += count
		detail := commits[commit]
		if detail.boundary {
			continue
		}
		touch.inWindow += count
		touch.authors[strings.ToLower(detail.mail)] += count
		weighted += float64(count) * recency.decay(detail.when)
		if age := recency.ageDays(detail.when); touch.newestDays < 0 || age < touch.newestDays {
			touch.newestDays = age
		}
	}
	if touch.lines > 0 {
		touch.freshness = weighted / float64(touch.lines)
	}
	return touch
}

func (recency *contextRecency) blameReason(candidate string) string {
	touch, ok := recency.blame[candidate]
	switch {
	case recency.state != "examined":
		return "blame abstained (" + recency.state + ")"
	case !ok:
		return fmt.Sprintf("blame abstained (beyond the %d-file blame bound)", contextBlameCap)
	case touch.state != "examined":
		return "blame abstained (" + touch.state + ")"
	case touch.inWindow == 0:
		return fmt.Sprintf("blame 0.00 (0 of %d lines last touched within the window)", touch.lines)
	}
	reason := fmt.Sprintf("blame %.2f (%d of %d lines last touched within the window, newest %d days before the indexed commit)",
		touch.freshness, touch.inWindow, touch.lines, touch.newestDays)
	if touch.owners.state != "" {
		reason += "; ownership " + touch.owners.state + ": CODEOWNERS names " + strings.Join(touch.owners.owners, " ") + ", blame names " + touch.owners.author
	}
	return reason
}

// codeOwners is the governing CODEOWNERS file at the indexed commit.
type codeOwners struct {
	path  string
	rules []codeOwnersRule
}

type codeOwnersRule struct {
	line    int
	text    string
	pattern *regexp.Regexp
	owners  []string
}

// ownershipCheck is a disagreement between CODEOWNERS and blame; an empty
// state is agreement or no check.
type ownershipCheck struct {
	state, rule, author string
	line, authorLines   int
	owners              []string
}

func (recency *contextRecency) codeOwners() *codeOwners {
	if recency.owners != nil {
		return recency.owners
	}
	recency.owners = &codeOwners{}
	for _, location := range codeOwnersLocations {
		if !trackedPath(recency.index, location) {
			continue
		}
		raw, err := git(recency.ctx, recency.index.Root, contextOwnersMaxBytes, nil, "cat-file", "blob", recency.index.CommitRevision+":"+location)
		if err == nil {
			recency.owners = &codeOwners{path: location, rules: parseCodeOwners(string(raw))}
		}
		break
	}
	return recency.owners
}

// parseCodeOwners keeps each rule GitHub's syntax admits; negation and
// bracket ranges are not CODEOWNERS syntax and their lines are skipped.
func parseCodeOwners(text string) []codeOwnersRule {
	rules := make([]codeOwnersRule, 0)
	for number, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") || strings.HasPrefix(fields[0], "!") || strings.Contains(fields[0], "[") {
			continue
		}
		pattern := codeOwnersPattern(fields[0])
		if pattern == nil {
			continue
		}
		rules = append(rules, codeOwnersRule{line: number + 1, text: fields[0], pattern: pattern, owners: ownerFields(fields[1:])})
	}
	return rules
}

func ownerFields(fields []string) []string {
	owners := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.HasPrefix(field, "#") {
			break
		}
		owners = append(owners, field)
	}
	return owners
}

// codeOwnersPattern translates GitHub's gitignore-style pattern: a pattern
// with an inner or leading slash is anchored at the root, `**` crosses
// directories, `*` and `?` stay inside one, a trailing slash matches only
// what is under the directory, and a last segment carrying a wildcard
// matches that segment only (`docs/*` does not reach `docs/a/b`).
func codeOwnersPattern(pattern string) *regexp.Regexp {
	body := strings.Trim(pattern, "/")
	if body == "" {
		return nil
	}
	prefix := "^(?:.*/)?"
	if strings.HasPrefix(pattern, "/") || strings.Contains(body, "/") {
		prefix = "^"
	}
	suffix := "(?:/.*)?$"
	if strings.HasSuffix(pattern, "/") {
		suffix = "/.*$"
	} else if strings.ContainsAny(body[strings.LastIndex(body, "/")+1:], "*?") {
		suffix = "$"
	}
	var translated strings.Builder
	for index := 0; index < len(body); index++ {
		switch {
		case strings.HasPrefix(body[index:], "**/"):
			translated.WriteString("(?:.*/)?")
			index += 2
		case strings.HasPrefix(body[index:], "**"):
			translated.WriteString(".*")
			index++
		case body[index] == '*':
			translated.WriteString("[^/]*")
		case body[index] == '?':
			translated.WriteString("[^/]")
		default:
			translated.WriteString(regexp.QuoteMeta(body[index : index+1]))
		}
	}
	compiled, err := regexp.Compile(prefix + translated.String() + suffix)
	if err != nil {
		return nil
	}
	return compiled
}

// owning is the last rule matching the path (GitHub's precedence).
func (owners *codeOwners) owning(candidate string) (codeOwnersRule, bool) {
	for index := len(owners.rules) - 1; index >= 0; index-- {
		if owners.rules[index].pattern.MatchString(candidate) {
			return owners.rules[index], true
		}
	}
	return codeOwnersRule{}, false
}

// checkOwners compares the owning rule with the authors of the path's
// in-window lines. An owner matching any such author agrees. An email owner
// is compared exactly; a `@user` owner matches only a GitHub noreply
// address, and a `@org/team` owner never matches, so neither can prove a
// disagreement: when every owner is an email and none matches, the state is
// `disagrees`, otherwise `unverifiable`. Both are reported, never resolved.
func (recency *contextRecency) checkOwners(candidate string, authors map[string]int) ownershipCheck {
	rule, ok := recency.codeOwners().owning(candidate)
	if !ok || len(rule.owners) == 0 || len(authors) == 0 {
		return ownershipCheck{}
	}
	allEmail := true
	for _, owner := range rule.owners {
		if ownerMatchesAny(owner, authors) {
			return ownershipCheck{}
		}
		allEmail = allEmail && !strings.HasPrefix(owner, "@")
	}
	author, lines := topAuthor(authors)
	check := ownershipCheck{state: "unverifiable", rule: rule.text, line: rule.line, owners: rule.owners, author: author, authorLines: lines}
	if allEmail {
		check.state = "disagrees"
	}
	return check
}

func ownerMatchesAny(owner string, authors map[string]int) bool {
	owner = strings.ToLower(owner)
	handle, isHandle := strings.CutPrefix(owner, "@")
	for author := range authors {
		if !isHandle && author == owner {
			return true
		}
		if isHandle && !strings.Contains(handle, "/") && (author == handle+"@users.noreply.github.com" || strings.HasSuffix(author, "+"+handle+"@users.noreply.github.com")) {
			return true
		}
	}
	return false
}

func topAuthor(authors map[string]int) (string, int) {
	names := make([]string, 0, len(authors))
	for author := range authors {
		names = append(names, author)
	}
	sort.Slice(names, func(left, right int) bool {
		if authors[names[left]] != authors[names[right]] {
			return authors[names[left]] > authors[names[right]]
		}
		return names[left] < names[right]
	})
	return names[0], authors[names[0]]
}

var ownershipReasons = map[string]string{
	"disagrees":    "no CODEOWNERS owner wrote any line changed within the window: the rule may have drifted; neither side is chosen",
	"unverifiable": "no CODEOWNERS owner matches an in-window blame author, and a handle or team owner cannot be compared with a commit email; neither side is chosen",
}

// ownership lists, in packet order, every row whose blame disagrees with or
// cannot verify its CODEOWNERS rule.
func (recency *contextRecency) ownership(rows []contextRow) []any {
	entries := make([]any, 0)
	for _, row := range rows {
		check := recency.blame[row.path].owners
		if check.state == "" {
			continue
		}
		owners := make([]any, 0, len(check.owners))
		for _, owner := range check.owners {
			owners = append(owners, owner)
		}
		entries = append(entries, map[string]any{
			"path": row.path, "state": check.state, "rule": check.rule, "rule_line": check.line,
			"codeowners": owners, "blame_author": check.author, "blame_author_lines": check.authorLines,
			"reason": ownershipReasons[check.state],
		})
	}
	return entries
}
