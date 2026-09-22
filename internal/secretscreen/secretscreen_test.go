package secretscreen

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type parityCorpus struct {
	Baseline   []parityCase `json:"baseline"`
	WriterOnly []parityCase `json:"writer_only"`
}

type parityCase struct {
	Name     string `json:"name"`
	Text     string `json:"text"`
	Match    bool   `json:"match"`
	Redacted string `json:"redacted"`
}

func loadParityCorpus(t *testing.T) parityCorpus {
	t.Helper()
	raw, err := os.ReadFile("testdata/parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus parityCorpus
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	return corpus
}

func TestSecretPatternParityCorpus(t *testing.T) {
	corpus := loadParityCorpus(t)
	for _, test := range corpus.Baseline {
		t.Run(test.Name, func(t *testing.T) {
			if got := MatchString(test.Text); got != test.Match {
				t.Fatalf("MatchString(%q) = %v, want %v", test.Text, got, test.Match)
			}
			if got := MatchStoredV1String(test.Text); got != test.Match {
				t.Fatalf("MatchStoredV1String(%q) = %v, want %v", test.Text, got, test.Match)
			}
		})
	}
	for _, test := range corpus.WriterOnly {
		t.Run(test.Name, func(t *testing.T) {
			if !MatchString(test.Text) {
				t.Fatalf("writer did not match %q", test.Text)
			}
			if MatchStoredV1String(test.Text) {
				t.Fatalf("stored-v1 matcher unexpectedly matched %q", test.Text)
			}
			if redacted, _ := Screen(test.Text); test.Redacted != "" && redacted != test.Redacted {
				t.Fatalf("Screen(%q) = %q, want %q", test.Text, redacted, test.Redacted)
			}
			if redacted, hit := Screen(test.Text); !hit || redacted == test.Text || MatchString(redacted) || strings.Contains(redacted, "synthetic-example-value") {
				t.Fatalf("Screen(%q) = (%q, %v)", test.Text, redacted, hit)
			}
		})
	}
}

// TestPatternMatchesUnionOfFormerTables asserts the shared pattern matches
// every secret shape either of the two former per-package copies matched.
// The corpus below is the union of internal/trace/record_test.go's and
// internal/contextindex/history_test.go's secret-pattern cases (both
// packages carried a byte-identical pattern; see internal/secretscreen
// doc comment). internal/contextindex's Unicode-homoglyph cases (e.g. a
// dotless-i "private_key") are caught by its separate containsSecret
// wrapper's pythonIgnoreCaseSpecials normalizer, not by the pattern that
// was duplicated, so they are not part of this union.
func TestPatternMatchesUnionOfFormerTables(t *testing.T) {
	for _, value := range []string{
		// former internal/trace cases
		"token=abcdefghijklmnopqrstuvwxyz",
		"token = abcdefghijklmnopqrstuvwxyz",
		// former internal/contextindex cases
		"password=correct-horse-battery-staple",
		"password = correct-horse-battery-staple",
		"toKen=value",
		"Authorization: bearer abcdefghijklmnop",
		"github_pat_abcdefghijklmnopqrstuvwxyz",
		// vendor and structural shapes both copies carried in their
		// alternation but neither package's tests exercised directly
		"-----BEGIN PRIVATE KEY-----",
		"https://user:pass@example.invalid",
		"AKIAABCDEFGHIJKLMNOP",
		"glpat-abcdefghijklmnopqrst",
		"sk-abcdefghijklmnopqrstuvwxyz",
	} {
		if !MatchString(value) {
			t.Fatalf("shared pattern missed %q", value)
		}
		redacted, hit := Screen(value)
		if !hit {
			t.Fatalf("Screen reported no hit for %q", value)
		}
		if redacted == value || MatchString(redacted) {
			t.Fatalf("Screen did not redact %q, got %q", value, redacted)
		}
	}

	for _, value := range []string{
		"roadmap: add workflow gate",
		"password= ",
		// a path whose word ends in "sk" is not an sk- vendor key
		"docs/plans/AGENT-TASK-MANAGER-LANDSCAPE-2026-09-04.md",
	} {
		if MatchString(value) {
			t.Fatalf("shared pattern false-matched %q", value)
		}
		redacted, hit := Screen(value)
		if hit || redacted != value {
			t.Fatalf("Screen altered non-secret %q, got %q hit=%v", value, redacted, hit)
		}
	}
}

// Redacting only the BEGIN line labelled the material and then emitted it.
func TestScreenRedactsThePrivateKeyBodyNotOnlyItsHeader(t *testing.T) {
	const body = "b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2g"
	screened, hit := Screen("prefix\n-----BEGIN OPENSSH PRIVATE KEY-----\n" + body + "\n-----END OPENSSH PRIVATE KEY-----\nsuffix\n")
	if !hit {
		t.Fatal("a private key block was not detected")
	}
	if strings.Contains(screened, body) {
		t.Fatalf("key body survived redaction: %q", screened)
	}
	if !strings.Contains(screened, "prefix") || !strings.Contains(screened, "suffix") {
		t.Fatalf("redaction consumed text outside the block: %q", screened)
	}
	// A block with no END line still redacts its header rather than nothing.
	truncated, hit := Screen("-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA3Tz2mr7SZiAMfQyuvBjM9OiJjRazXBZ1BjP5CE\n")
	if !hit || strings.Contains(truncated, "BEGIN RSA PRIVATE KEY") {
		t.Fatalf("unterminated block = %q", truncated)
	}
}

// A quoted assignment value used to truncate at its first internal space
// (e.g. `password = "correct horse battery"` redacted only through
// "correct"), leaking the rest of the secret. It must now consume the
// complete quoted value while still leaving a following benign field
// untouched, matching the existing quoted-JSON-property behavior.
func TestScreenConsumesWholeQuotedAssignmentValue(t *testing.T) {
	screened, hit := Screen(`password = "correct horse battery" status = "ok"`)
	if !hit {
		t.Fatal("a quoted password assignment was not detected")
	}
	if strings.Contains(screened, "horse") || strings.Contains(screened, "battery") {
		t.Fatalf("quoted value tail survived redaction: %q", screened)
	}
	if !strings.Contains(screened, `status = "ok"`) {
		t.Fatalf("redaction consumed a following benign field: %q", screened)
	}
}

// The `akia|asia` rule only ever covered the AWS access-key ID; its paired
// 40-character secret access key survived when no assignment name sat
// beside it. The pairing fix must not swallow unrelated trailing text, and
// must not turn a bare 40-character string with no key-ID context into a
// new false positive.
func TestScreenRedactsAWSSecretAdjacentToItsKeyID(t *testing.T) {
	const secret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

	screened, hit := Screen("AKIAABCDEFGHIJKLMNOP " + secret)
	if !hit {
		t.Fatal("an AWS key ID with adjacent secret was not detected")
	}
	if strings.Contains(screened, secret) {
		t.Fatalf("adjacent AWS secret survived redaction: %q", screened)
	}

	trailing, hit := Screen("AKIAABCDEFGHIJKLMNOP is a fake key")
	if !hit || !strings.Contains(trailing, "is a fake key") {
		t.Fatalf("key ID redaction consumed unrelated trailing text: %q", trailing)
	}

	if MatchString(secret) {
		t.Fatalf("a bare 40-character string with no AWS key ID context false-matched: %q", secret)
	}
}

func TestScreenRedactsCredentialAfterAuthorizationScheme(t *testing.T) {
	for _, header := range []string{"Authorization: Bearer", "authorization: basic", "Authorization=Token"} {
		const credential = "opaque0123456789abcdef"
		screened, hit := Screen("curl -H '" + header + " " + credential + "' https://example.invalid")
		if !hit || strings.Contains(screened, credential) {
			t.Fatalf("credential after %q survived redaction: %q", header, screened)
		}
		if !strings.HasSuffix(screened, " https://example.invalid") {
			t.Fatalf("scheme redaction consumed unrelated trailing text: %q", screened)
		}
	}
}

// secretPattern's credentialed-URL branch stops the password at its first
// `@`, so a raw `@` inside the password left the tail (`ssw0rd@`) in the
// redacted text.
func TestScreenRedactsWholePasswordContainingAtSign(t *testing.T) {
	screened, hit := Screen("git remote add origin https://user:p@ssw0rd@example.invalid/org/repo")
	if !hit || screened != "git remote add origin "+Placeholder+"example.invalid/org/repo" {
		t.Fatalf("password tail survived or host was consumed: (%q, %v)", screened, hit)
	}
}

// TestCredentialedURLPasswordStopsAtQueryFragmentOrQuote pins that neither
// the writer URL user nor its password runs past `?`, `#` or a quote to a
// later `@`, which would end the match inside a following secret and leak
// its tail.
func TestCredentialedURLPasswordStopsAtQueryFragmentOrQuote(t *testing.T) {
	for text, want := range map[string]string{
		`https://u:p@host;password="a@b c d"`:                Placeholder + "host;" + Placeholder,
		"https://u:p@api.example.invalid?access_token=ab@cd": Placeholder + "api.example.invalid?" + Placeholder,
		`https://host?password="a:b@c d"`:                    "https://host?" + Placeholder,
		`https://u:x#password='p@ss word'`:                   "https://u:x#" + Placeholder,
	} {
		if screened, hit := Screen(text); !hit || screened != want {
			t.Fatalf("Screen(%q) = (%q, %v), want %q", text, screened, hit, want)
		}
	}
}

// TestUnterminatedQuotedValueRedactsThroughEOF is the executable EAF-V0-010
// boundary: an unterminated quoted credential value redacts through absolute
// end of input, past newlines, structural delimiters and a dangling
// backslash, while the stored-v1 matcher keeps ignoring quoted values.
func TestUnterminatedQuotedValueRedactsThroughEOF(t *testing.T) {
	for _, tail := range []string{
		"top secret value",
		"top secret value\nnext: line",
		`top secret value,task:repair config}`,
		`top secret value\`,
		`top \"secret\" value]`,
	} {
		text := `config/{"password":"` + tail
		redacted, hit := Screen(text)
		if !hit || redacted != "config/{"+Placeholder {
			t.Fatalf("Screen(%q) = (%q, %v)", text, redacted, hit)
		}
		if MatchStoredV1String(text) {
			t.Fatalf("stored-v1 matcher unexpectedly matched %q", text)
		}
	}
}
