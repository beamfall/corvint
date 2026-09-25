package contextindex

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// legacyPythonWhitespace and legacySecretPattern are the pre-migration
// contextindex-local copies of internal/secretscreen's whitespace class and
// pattern (see docs/agent-memory/fixes.md "observations ledger has no
// secret screen; secret regex duplicated in two packages"), kept here only
// to prove containsSecret's migration to secretscreen.MatchString is
// byte-identical in behaviour to the regex it replaced.
const legacyPythonWhitespace = `\x09-\x0d\x1c-\x20\x{0085}\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

var legacySecretPattern = regexp.MustCompile(
	`(?i)[a-z0-9_.-]*(?:api[_-]?key|access[_-]?key|authorization|password|` +
		`private[_-]?key|secret|token)[a-z0-9_.-]*[` + legacyPythonWhitespace + `]*[:=][` + legacyPythonWhitespace + `]*[^` + legacyPythonWhitespace + `]+|` +
		`-----BEGIN [A-Z ]*PRIVATE KEY-----|-----BEGIN PGP PRIV[A]TE KEY BLOCK-----|` +
		`[a-z][a-z0-9+.-]*://[^` + legacyPythonWhitespace + `/:]+:[^` + legacyPythonWhitespace + `/@]+@|` +
		`gh[pousr]_[a-z0-9]{20,}|github_pat_[a-z0-9_]{20,}|` +
		`glpat-[a-z0-9_-]{20,}|xox[a-z]-[a-z0-9-]{10,}|` +
		`(?:akia|asia)[a-z0-9]{16}|aiza[a-z0-9_-]{30,}|npm_[a-z0-9]{20,}|` +
		`sk-[a-z0-9_-]{20,}|(?:sk|rk)_(?:live|test)_[a-z0-9]{16,}|` +
		`pypi-ageichlwasi5vcmc[a-z0-9_-]{20,}|` +
		`sg\.[a-z0-9_-]{16,}\.[a-z0-9_-]{32,}|sk[0-9a-f]{32}|` +
		`bearer[` + legacyPythonWhitespace + `]+[a-z0-9][a-z0-9_.~+/-]{15,}|` +
		`[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}`,
)

func historyRecord(commit, tree, subject string, paths ...[]byte) []byte {
	var output bytes.Buffer
	output.WriteByte(0x1e)
	fmt.Fprintf(&output, "%s\x1f%s\x1f%s", commit, tree, subject)
	for _, path := range paths {
		output.WriteByte(0)
		output.Write(path)
		output.WriteByte('\n')
	}
	output.WriteByte(0)
	return output.Bytes()
}

func TestParseHistoryBuildsPythonCanonicalShape(t *testing.T) {
	commit := strings.Repeat("1", 40)
	tree := strings.Repeat("2", 40)
	entries, canonical, err := parseHistory(historyRecord(commit, tree, "roadmap: add workflow gate", []byte("AGENTS.md")), "sha1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].subject != "roadmap: add workflow gate" || len(entries[0].paths) != 1 {
		t.Fatalf("entries=%#v", entries)
	}
	encoded, err := CanonicalJSON(canonical)
	if err != nil {
		t.Fatal(err)
	}
	want := `[["` + commit + `","` + tree + `","roadmap: add workflow gate",["AGENTS.md"]]]`
	if string(encoded) != want {
		t.Fatalf("canonical=%s want=%s", encoded, want)
	}
}

func TestParseHistoryIdentityRequiresExactlyTwoObjectIDs(t *testing.T) {
	commit := strings.Repeat("1", 40)
	tree := strings.Repeat("2", 40)
	for _, test := range []struct {
		name  string
		raw   string
		valid bool
	}{
		{"two lines", commit + "\n" + tree + "\n", true},
		{"missing terminal newline", commit + "\n" + tree, false},
		{"extra empty line", commit + "\n" + tree + "\n\n", false},
		{"extra value", commit + "\n" + tree + "\nextra\n", false},
		{"invalid object ID", commit + "\nnot-an-oid\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			gotCommit, gotTree, err := parseHistoryIdentity([]byte(test.raw), "sha1")
			if test.valid {
				if err != nil || gotCommit != commit || gotTree != tree {
					t.Fatalf("identity=(%q, %q, %v)", gotCommit, gotTree, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("identity=(%q, %q, nil)", gotCommit, gotTree)
			}
		})
	}
}

func TestDecodePythonUTF8ReplacementVectors(t *testing.T) {
	for _, test := range []struct {
		raw  []byte
		want string
	}{
		{[]byte{0xff, 0xff}, "��"},
		{[]byte{0xe1, 0x80, 'A'}, "�A"},
		{[]byte{0xf0, 0x90, 0x80, 'A'}, "�A"},
		{[]byte{0xc0, 0xaf}, "��"},
		{[]byte{0xed, 0xa0, 0x80}, "���"},
		{[]byte{0xe1, 0x80}, "�"},
	} {
		if got := decodePythonUTF8(test.raw); got != test.want {
			t.Fatalf("decodePythonUTF8(%x)=%q want=%q", test.raw, got, test.want)
		}
	}
}

func TestParseHistoryRejectsHostileRecords(t *testing.T) {
	commit := strings.Repeat("1", 40)
	tree := strings.Repeat("2", 40)
	for _, test := range []struct {
		name string
		raw  []byte
		code string
	}{
		{"malformed header", []byte("not-history"), ""},
		{"invalid path encoding", historyRecord(commit, tree, "subject", []byte{0xff}), "unsupported-query-history"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := parseHistory(test.raw, "sha1", false)
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != test.code {
				t.Fatalf("error=%#v", err)
			}
		})
	}

	var oversized bytes.Buffer
	for index := 0; index <= maxHistoryCommits; index++ {
		oversized.Write(historyRecord(commit, tree, fmt.Sprintf("subject %d", index)))
	}
	if _, _, err := parseHistory(oversized.Bytes(), "sha1", false); err == nil || !strings.Contains(err.Error(), "commit limit") {
		t.Fatalf("error=%v", err)
	}
}

func TestSecretPatternMatchesHistorySecretShapes(t *testing.T) {
	for _, value := range []string{
		`{"password":"synthetic-example-value"}`,
		`config/{"api_key":"synthetic-example-value"}.go`,
		"password=correct-horse-battery-staple",
		"password\u2003=\u2003correct-horse-battery-staple",
		"prıvate_key=value",
		"prİvate_key=value",
		"ſecret=value",
		"toKen=value",
		"Authorization: bearer abcdefghijklmnop",
		"github_pat_abcdefghijklmnopqrstuvwxyz",
		// LTA-V0-004 writer-only shapes: query drops these history candidates.
		`password = "correct horse battery"`,
		"export DB_PASS='synthetic-example-value'",
		"whsec_abcdefghijklmnopqrstuvwxyz0123456789",
		"https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
	} {
		if !containsSecret(value) {
			t.Fatalf("secret pattern missed %q", value)
		}
	}
	if containsSecret("roadmap: add workflow gate") {
		t.Fatal("ordinary subject matched secret pattern")
	}
	if containsSecret("password=\u2003") {
		t.Fatal("Python Unicode whitespace must not satisfy \\S+")
	}
}

// TestContainsSecretMatchesLegacyPattern proves the migration of
// containsSecret to secretscreen.MatchString is byte-identical in
// behaviour to the contextindex-local regex it replaced, across every case
// the history secret-pattern suite exercises (the case's own normalisation
// via pythonIgnoreCaseSpecials stays in containsSecret unchanged).
func TestContainsSecretMatchesLegacyPattern(t *testing.T) {
	for _, value := range []string{
		"password=correct-horse-battery-staple",
		"password\u2003=\u2003correct-horse-battery-staple",
		"pr\u0131vate_key=value",
		"pr\u0130vate_key=value",
		"\u017fecret=value",
		"toKen=value",
		"Authorization: bearer abcdefghijklmnop",
		"github_pat_abcdefghijklmnopqrstuvwxyz",
		"roadmap: add workflow gate",
		"password=\u2003",
	} {
		got := containsSecret(value)
		want := legacySecretPattern.MatchString(pythonIgnoreCaseSpecials.Replace(value))
		if got != want {
			t.Fatalf("containsSecret(%q)=%v, legacy pattern=%v", value, got, want)
		}
	}
}
