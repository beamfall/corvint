package specindex

import (
	"encoding/json"
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

func TestIndexCoversSpecsAndHeaders(t *testing.T) {
	b := mustRead(t, "../../docs/specs/INDEX.json")
	var objects []map[string]json.RawMessage
	if err := json.Unmarshal(b, &objects); err != nil {
		t.Fatal(err)
	}
	for i, object := range objects {
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if !reflect.DeepEqual(keys, requiredFields) {
			t.Errorf("INDEX entry %d fields %v, want exactly %v", i, keys, requiredFields)
		}
	}

	var entries []entry
	if err := json.Unmarshal(b, &entries); err != nil {
		t.Fatal(err)
	}
	readme := readREADME(t)
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Path] {
			t.Errorf("duplicate INDEX path %s", e.Path)
		}
		seen[e.Path] = true
		claimLength := utf8.RuneCountInString(e.Claim)
		if claimLength == 0 || claimLength > 160 {
			t.Errorf("%s claim length is %d, want 1..160 characters", e.Path, claimLength)
		} else if !strings.ContainsAny(e.Claim[len(e.Claim)-1:], ".?!") {
			t.Errorf("%s claim is not a sentence: %q", e.Path, e.Claim)
		}

		body, err := os.ReadFile(filepath.Join("../..", e.Path))
		if err != nil {
			t.Errorf("%s: %v", e.Path, err)
			continue
		}
		text := string(body)
		intent := header(text, "Intent status:")
		delivery := header(text, "Delivery status:")
		if intent == "" || delivery == "" {
			t.Errorf("%s must declare Intent status and Delivery status", e.Path)
		}
		if e.Intent != intent {
			t.Errorf("%s intent %q != header %q", e.Path, e.Intent, intent)
		}
		if e.Delivery != delivery {
			t.Errorf("%s delivery %q != header %q", e.Path, e.Delivery, delivery)
		}
		checkDigest(t, e, text)

		rows := readme[e.Path]
		if len(rows) != 1 {
			t.Errorf("%s has %d unanchored README rows, want exactly 1", e.Path, len(rows))
			continue
		}
		row := rows[0]
		if row.intent != e.Intent || row.delivery != e.Delivery {
			t.Errorf("%s README status %q/%q != INDEX %q/%q", e.Path, row.intent, row.delivery, e.Intent, e.Delivery)
		}
		if !claimStarts(row.description, e.Claim) {
			t.Errorf("%s README first clause %q does not start with claim %q", e.Path, row.description, e.Claim)
		}
	}

	files, err := filepath.Glob("../../docs/specs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if filepath.Base(file) == "README.md" {
			continue
		}
		path := filepath.ToSlash(strings.TrimPrefix(file, "../../"))
		if !seen[path] {
			t.Errorf("missing INDEX entry %s", path)
		}
	}
	if len(seen) != len(files)-1 {
		t.Errorf("INDEX has %d unique entries, want %d specification files", len(seen), len(files)-1)
	}
}

func checkDigest(t *testing.T, e entry, text string) {
	t.Helper()
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Agent digest" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Errorf("%s has no Agent digest", e.Path)
		return
	}
	if start >= 40 {
		t.Errorf("%s Agent digest starts at line %d, want within first 40", e.Path, start+1)
	}
	block := []string{lines[start]}
	values := map[string]string{}
	for i := start + 1; i < len(lines) && strings.TrimSpace(lines[i]) != ""; i++ {
		block = append(block, lines[i])
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "- ") {
			t.Errorf("%s malformed digest line %q", e.Path, line)
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(line, "- "), ": ", 2)
		if len(parts) != 2 {
			t.Errorf("%s malformed digest field %q", e.Path, line)
			continue
		}
		values[parts[0]] = parts[1]
	}
	if len(block) > 8 {
		t.Errorf("%s Agent digest is %d lines, want at most 8", e.Path, len(block))
	}

	superseded := e.Delivery == "superseded" || strings.HasPrefix(e.Intent, "superseded")
	if superseded {
		if len(block) != 3 || values["Successor"] == "" || values["Why"] == "" {
			t.Errorf("%s superseded digest must contain only Successor and Why", e.Path)
		}
		if e.SupersededBy == nil || *e.SupersededBy == "" {
			t.Errorf("%s superseded INDEX entry must name supersededBy", e.Path)
		} else if !strings.Contains(values["Successor"], filepath.Base(*e.SupersededBy)) {
			t.Errorf("%s digest successor %q != INDEX supersededBy %q", e.Path, values["Successor"], *e.SupersededBy)
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
		t.Errorf("%s digest fields %v, want %v", e.Path, keys, want)
	}
	if values["Claim"] != e.Claim {
		t.Errorf("%s digest claim %q != INDEX claim %q", e.Path, values["Claim"], e.Claim)
	}
	status := normalize(values["Status"])
	if !strings.Contains(status, normalize(e.Intent)) || !strings.Contains(status, normalize(e.Delivery)) {
		t.Errorf("%s digest status %q does not agree with %q/%q", e.Path, values["Status"], e.Intent, e.Delivery)
	}
}

type readmeRow struct {
	intent      string
	delivery    string
	description string
}

var contractLink = regexp.MustCompile(`\]\(([^)#]+\.md)(#[^)]*)?\)`)

func readREADME(t *testing.T) map[string][]readmeRow {
	t.Helper()
	rows := map[string][]readmeRow{}
	for _, line := range strings.Split(string(mustRead(t, "../../docs/specs/README.md")), "\n") {
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
	return rows
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
