package specindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

type entry struct {
	Path           string   `json:"path"`
	Title          string   `json:"title"`
	ReqPrefix      string   `json:"reqPrefix"`
	Intent         string   `json:"intent"`
	Delivery       string   `json:"delivery"`
	Claim          string   `json:"claim"`
	DependsOn      []string `json:"dependsOn"`
	SupersededBy   *string  `json:"supersededBy"`
	ClosableToday  bool     `json:"closableToday"`
	Implementation []string `json:"implementation"`
}

var requiredFields = []string{
	"claim", "closableToday", "delivery", "dependsOn", "implementation",
	"intent", "path", "reqPrefix", "supersededBy", "title",
}

// TestIndexCoversSpecsAndHeaders is the index/digest/README contract check over the repository:
// INDEX.json, every spec's headers and Agent digest, and the docs/specs/README.md table must agree.
// The requirements-only generator check does not replace it (SRG-V0-045).
func TestIndexCoversSpecsAndHeaders(t *testing.T) {
	checkIndex("../..", t.Errorf)
}

// checkIndex reports, through report, every disagreement between docs/specs/INDEX.json, the spec
// headers and Agent digests, and the docs/specs/README.md table of the repository at root.
func checkIndex(root string, report func(format string, args ...any)) {
	b, err := os.ReadFile(filepath.Join(root, "docs/specs/INDEX.json"))
	if err != nil {
		report("%v", err)
		return
	}
	var objects []map[string]json.RawMessage
	if err := json.Unmarshal(b, &objects); err != nil {
		report("%v", err)
		return
	}
	for i, object := range objects {
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if !reflect.DeepEqual(keys, requiredFields) {
			report("INDEX entry %d fields %v, want exactly %v", i, keys, requiredFields)
		}
	}

	var entries []entry
	if err := json.Unmarshal(b, &entries); err != nil {
		report("%v", err)
		return
	}
	readme, err := readREADME(root)
	if err != nil {
		report("%v", err)
		return
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Path] {
			report("duplicate INDEX path %s", e.Path)
		}
		seen[e.Path] = true
		claimLength := utf8.RuneCountInString(e.Claim)
		if claimLength == 0 || claimLength > 160 {
			report("%s claim length is %d, want 1..160 characters", e.Path, claimLength)
		} else if !strings.ContainsAny(e.Claim[len(e.Claim)-1:], ".?!") {
			report("%s claim is not a sentence: %q", e.Path, e.Claim)
		}

		body, err := os.ReadFile(filepath.Join(root, e.Path))
		if err != nil {
			report("%s: %v", e.Path, err)
			continue
		}
		text := string(body)
		intent := header(text, "Intent status:")
		delivery := header(text, "Delivery status:")
		if intent == "" || delivery == "" {
			report("%s must declare Intent status and Delivery status", e.Path)
		}
		if e.Intent != intent {
			report("%s intent %q != header %q", e.Path, e.Intent, intent)
		}
		if e.Delivery != delivery {
			report("%s delivery %q != header %q", e.Path, e.Delivery, delivery)
		}
		checkDigest(report, e, text)

		rows := readme[e.Path]
		if len(rows) != 1 {
			report("%s has %d unanchored README rows, want exactly 1", e.Path, len(rows))
			continue
		}
		row := rows[0]
		if row.intent != e.Intent || row.delivery != e.Delivery {
			report("%s README status %q/%q != INDEX %q/%q", e.Path, row.intent, row.delivery, e.Intent, e.Delivery)
		}
		if !claimStarts(row.description, e.Claim) {
			report("%s README first clause %q does not start with claim %q", e.Path, row.description, e.Claim)
		}
	}

	files, err := filepath.Glob(filepath.Join(root, "docs/specs/*.md"))
	if err != nil {
		report("%v", err)
		return
	}
	for _, file := range files {
		if filepath.Base(file) == "README.md" {
			continue
		}
		path := "docs/specs/" + filepath.Base(file)
		if !seen[path] {
			report("missing INDEX entry %s", path)
		}
	}
	if len(seen) != len(files)-1 {
		report("INDEX has %d unique entries, want %d specification files", len(seen), len(files)-1)
	}
}

// indexKey turns a glob match under ../../ into its INDEX.json path. It converts
// separators before trimming, since a Windows glob returns ..\..\docs\specs\x.md.
func indexKey(file string) string {
	return strings.TrimPrefix(filepath.ToSlash(file), "../../")
}

func checkDigest(report func(format string, args ...any), e entry, text string) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Agent digest" {
			start = i
			break
		}
	}
	if start < 0 {
		report("%s has no Agent digest", e.Path)
		return
	}
	if start >= 40 {
		report("%s Agent digest starts at line %d, want within first 40", e.Path, start+1)
	}
	block := []string{lines[start]}
	values := map[string]string{}
	fieldLine := map[string]int{}
	for i := start + 1; i < len(lines) && strings.TrimSpace(lines[i]) != ""; i++ {
		block = append(block, lines[i])
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "- ") {
			report("%s malformed digest line %q", e.Path, line)
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(line, "- "), ": ", 2)
		if len(parts) != 2 {
			report("%s malformed digest field %q", e.Path, line)
			continue
		}
		values[parts[0]] = parts[1]
		fieldLine[parts[0]] = i + 1
	}
	if len(block) > 8 {
		report("%s Agent digest is %d lines, want at most 8", e.Path, len(block))
	}

	superseded := e.Delivery == "superseded" || strings.HasPrefix(e.Intent, "superseded")
	if superseded {
		if len(block) != 3 || values["Successor"] == "" || values["Why"] == "" {
			report("%s superseded digest must contain only Successor and Why", e.Path)
		}
		if e.SupersededBy == nil || *e.SupersededBy == "" {
			report("%s superseded INDEX entry must name supersededBy", e.Path)
		} else if !strings.Contains(values["Successor"], filepath.Base(*e.SupersededBy)) {
			report("%s digest successor %q != INDEX supersededBy %q", e.Path, values["Successor"], *e.SupersededBy)
		}
		return
	}

	want := []string{"Blocked on", "Claim", "Exists", "Read next", "Status"}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, want) {
		report("%s digest fields %v, want %v", e.Path, keys, want)
	}
	if values["Claim"] != e.Claim {
		report("%s digest claim %q != INDEX claim %q", e.Path, values["Claim"], e.Claim)
	}
	status := normalize(values["Status"])
	if !strings.Contains(status, normalize(e.Intent)) || !strings.Contains(status, normalize(e.Delivery)) {
		report("%s", statusDriftMessage(e, values["Status"], fieldLine["Status"], text))
	}
	if problem := deliveryClauseProblem(values["Status"], e.Delivery); problem != "" {
		report("%s digest status %q %s %q", e.Path, values["Status"], problem, e.Delivery)
	}
}

type readmeRow struct {
	intent      string
	delivery    string
	description string
}

var contractLink = regexp.MustCompile(`\]\(([^)#]+\.md)(#[^)]*)?\)`)

func readREADME(root string) (map[string][]readmeRow, error) {
	body, err := os.ReadFile(filepath.Join(root, "docs/specs/README.md"))
	if err != nil {
		return nil, err
	}
	rows := map[string][]readmeRow{}
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		columns := strings.Split(line, "|")
		if len(columns) < 7 {
			continue
		}
		match := contractLink.FindStringSubmatch(columns[2])
		if match == nil || match[2] != "" {
			continue
		}
		path := match[1]
		if !strings.HasPrefix(path, "../") {
			path = "docs/specs/" + path
		}
		rows[path] = append(rows[path], readmeRow{
			intent:      strings.TrimSpace(columns[3]),
			delivery:    strings.TrimSpace(columns[4]),
			description: strings.TrimSpace(columns[5]),
		})
	}
	return rows, nil
}

// statusDriftMessage is the SRG-V0-047 refusal for an Agent digest Status that does not carry the
// header statuses: it names the digest field and both header fields with their file:line and says
// they are edited together, since changing only one of them is how they drift apart.
func statusDriftMessage(e entry, status string, statusLine int, text string) string {
	return fmt.Sprintf("%s:%d Agent digest \"- Status: %s\" does not agree with the header "+
		"%s:%d \"Intent status: %s\" and %s:%d \"Delivery status: %s\"; the digest Status must "+
		"contain both header values, so edit these fields together",
		e.Path, statusLine, status, e.Path, headerLine(text, "Intent status:"), e.Intent,
		e.Path, headerLine(text, "Delivery status:"), e.Delivery)
}

// headerLine is the 1-based line of the first header that header reads for key, or 0 when absent.
func headerLine(text, key string) int {
	bold := "**" + key[:len(key)-1] + ":**"
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		if strings.HasPrefix(line, key) || strings.HasPrefix(line, bold) {
			return i + 1
		}
	}
	return 0
}

func header(text, key string) string {
	bold := "**" + key[:len(key)-1] + ":**"
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		for _, prefix := range []string{key, bold} {
			if strings.HasPrefix(line, prefix) {
				return strings.TrimSpace(strings.TrimPrefix(line, prefix))
			}
		}
	}
	return ""
}

func claimStarts(description, claim string) bool {
	return strings.HasPrefix(normalize(description), normalize(claim))
}

func normalize(value string) string {
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return ' '
	}, value)), " ")
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHeaderForms(t *testing.T) {
	if got := header("- **Intent status:** accepted  \n", "Intent status:"); got != "accepted" {
		t.Fatalf("bold header = %q", got)
	}
	if got := header("Intent status: proposed\n", "Intent status:"); got != "proposed" {
		t.Fatalf("plain header = %q", got)
	}
}

// TestRequirementsAcceptanceColumns pins SRG-V0-009: every REQUIREMENTS.tsv row carries one of
// the three closed acceptance modes, an owner row names its anchor, and an unrecorded row none.
func TestRequirementsAcceptanceColumns(t *testing.T) {
	text := strings.TrimSuffix(string(mustRead(t, "../../docs/specs/REQUIREMENTS.tsv")), "\n")
	lines := strings.Split(text, "\n")
	if want := "id\tfile\ttitle\tacceptance\tacceptedBy"; lines[0] != want {
		t.Fatalf("REQUIREMENTS.tsv header %q, want %q", lines[0], want)
	}
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 5 {
			t.Errorf("REQUIREMENTS.tsv row %q has %d fields, want 5", line, len(fields))
			continue
		}
		mode, by := fields[3], fields[4]
		valid := mode == "unrecorded" && by == "-" ||
			mode == "owner" && by != "-" && by != "" ||
			mode == "standing" && by != ""
		if !valid {
			t.Errorf("%s acceptance %q by %q is not owner (anchored), standing or unrecorded (-)", fields[0], mode, by)
		}
	}
}

// TestIndexImplementationPathsExist refuses an INDEX.json implementation path that is absent from
// the tree (V1-1342): a moved or deleted file otherwise stays named as the implementation.
func TestIndexImplementationPathsExist(t *testing.T) {
	var entries []entry
	if err := json.Unmarshal(mustRead(t, "../../docs/specs/INDEX.json"), &entries); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		for _, path := range e.Implementation {
			if _, err := os.Stat(filepath.Join("../..", path)); err != nil {
				t.Errorf("%s: implementation path %q is absent from the tree: %v", e.Path, path, err)
			}
		}
	}
}

// deliveryClauseProblem reports how a digest Status disagrees with a qualified
// Delivery status header (SRG-V0-014), or "" when it agrees. A one-word header
// may be elaborated in the digest; a longer one must appear verbatim, followed
// by nothing or by a sentence boundary, so a stale or truncated header that is
// only a prefix of the digest clause is caught.
func deliveryClauseProblem(status, delivery string) string {
	if !strings.ContainsAny(delivery, " ;(") {
		return ""
	}
	index := strings.Index(status, delivery)
	if index < 0 || (index > 0 && isWordByte(status[index-1])) {
		return "does not carry the qualified delivery header"
	}
	if rest := strings.TrimSpace(status[index+len(delivery):]); rest != "" && !strings.HasPrefix(rest, ".") {
		return "continues the qualified delivery header beyond a sentence boundary"
	}
	return ""
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-'
}

func TestDeliveryClauseConvention(t *testing.T) {
	for _, c := range []struct {
		name, status, delivery string
		bad                    bool
	}{
		{"one word elaborated", "accepted / experimental delivery; more", "experimental", false},
		{"qualified exact", "accepted / experimental; milestone NOT_QUALIFIED", "experimental; milestone NOT_QUALIFIED", false},
		{"qualified then sentence", "accepted / experimental (file only). Checked by X", "experimental (file only)", false},
		{"stale prefix", "accepted / experimental; slice A; slice B", "experimental; slice A", true},
		{"stale prefix words", "accepted / experimental source implementation and tests", "experimental source implementation", true},
		{"absent", "accepted / experimental", "experimental; slice A", true},
		{"punctuation dropped", "accepted / experimental file only", "experimental (file only)", true},
		{"punctuation extended", "accepted / experimental (file only); more", "experimental (file only)", true},
		{"inside a word", "accepted / unimplemented locally", "implemented locally", true},
	} {
		if got := deliveryClauseProblem(c.status, c.delivery) != ""; got != c.bad {
			t.Errorf("%s: problem=%v, want %v", c.name, got, c.bad)
		}
	}
}
