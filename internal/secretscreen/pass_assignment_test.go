package secretscreen

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestStoredV1PatternIdentity(t *testing.T) {
	t.Run("LTA-V0-004 frozen-stored-pattern", func(t *testing.T) {
		const want = "6881463ef1568e369982842c14947be917ab7ad25f8a9a5f7e05581bd6511ff7"
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(StoredV1Pattern.String()))); got != want {
			t.Fatalf("stored-v1 pattern digest = %s, want %s", got, want)
		}
	})
}

func TestPassAssignmentBoundary(t *testing.T) {
	for _, test := range []struct {
		name, text, want string
	}{
		{name: "LTA-V0-004 public-validator-success", text: "Plugin validation passed: /private/tmp/corvint-log-admission-20260915/integrations/codex/plugins/corvint\n", want: "Plugin validation passed: /private/tmp/corvint-log-admission-20260915/integrations/codex/plugins/corvint\n"},
		{name: "LTA-V0-004 benign-bare-quoted-field", text: `passed = "all checks" status = "ok"`, want: `passed = "all checks" status = "ok"`},
		{name: "LTA-V0-004 benign-json-field", text: `{"passed":"all checks","status":"ok"}`, want: `{"passed":"all checks","status":"ok"}`},
		{name: "LTA-V0-004 benign-json-number", text: `{"passed":123,"status":"ok"}`, want: `{"passed":123,"status":"ok"}`},
		{name: "LTA-V0-004 pass-direct", text: `pass=synthetic123`, want: Placeholder},
		{name: "LTA-V0-004 prefixed-pass-direct", text: `DB_PASS: synthetic123`, want: Placeholder},
		{name: "LTA-V0-004 password-direct", text: `password=synthetic123`, want: Placeholder},
		{name: "LTA-V0-004 passphrase-direct", text: `passphrase=synthetic123`, want: Placeholder},
		{name: "LTA-V0-004 passwd-direct", text: `passwd=synthetic123`, want: Placeholder},
		{name: "LTA-V0-004 other-vocabulary-suffix", text: `db_password_hash=synthetic123`, want: Placeholder},
		{name: "LTA-V0-004 pass-double-quoted", text: `pass="synthetic 123 value" status="ok"`, want: Placeholder + ` status="ok"`},
		{name: "LTA-V0-004 prefixed-pass-single-quoted", text: `DB_PASS='synthetic 123 value' status="ok"`, want: Placeholder + ` status="ok"`},
		{name: "LTA-V0-004 password-single-quoted", text: `password='synthetic 123 value' status="ok"`, want: Placeholder + ` status="ok"`},
		{name: "LTA-V0-004 passphrase-double-quoted", text: `passphrase="synthetic 123 value" status="ok"`, want: Placeholder + ` status="ok"`},
		{name: "LTA-V0-004 passwd-double-quoted", text: `passwd="synthetic 123 value" status="ok"`, want: Placeholder + ` status="ok"`},
		{name: "LTA-V0-004 pass-json", text: `{"pass":"synthetic 123 value","status":"ok"}`, want: `{` + Placeholder + `,"status":"ok"}`},
		{name: "LTA-V0-004 prefixed-pass-json", text: `{"DB_PASS":"synthetic 123 value","status":"ok"}`, want: `{` + Placeholder + `,"status":"ok"}`},
		{name: "LTA-V0-004 password-json", text: `{"password":"synthetic 123 value","status":"ok"}`, want: `{` + Placeholder + `,"status":"ok"}`},
		{name: "LTA-V0-004 passphrase-json", text: `{"passphrase":"synthetic 123 value","status":"ok"}`, want: `{` + Placeholder + `,"status":"ok"}`},
		{name: "LTA-V0-004 passwd-json", text: `{"passwd":"synthetic 123 value","status":"ok"}`, want: `{` + Placeholder + `,"status":"ok"}`},
		{name: "LTA-V0-004 pass-escaped-value", text: `pass="synthetic \"123\" value" status="ok"`, want: Placeholder + ` status="ok"`},
		{name: "LTA-V0-004 pass-unterminated-value", text: "pass=\"synthetic 123\nvalue\\", want: Placeholder},
		{name: "LTA-V0-004 pass-json-unterminated-value", text: "{\"pass\":\"synthetic 123\nvalue\\", want: `{` + Placeholder},
		{name: "LTA-V0-004 pass-flag", text: `cmd --pass synthetic123`, want: `cmd ` + Placeholder},
		{name: "LTA-V0-004 pass-flag-quoted", text: `cmd --pass "synthetic 123 value" --verbose`, want: `cmd ` + Placeholder + ` --verbose`},
		{name: "LTA-V0-004 pass-flag-joined", text: `cmd --pass=synthetic123`, want: `cmd ` + Placeholder},
		{name: "LTA-V0-004 prefixed-pass-flag", text: `cmd --db-pass synthetic123`, want: `cmd ` + Placeholder},
	} {
		t.Run(test.name, func(t *testing.T) {
			screened, hit := Screen(test.text)
			wantHit := test.want != test.text
			if screened != test.want || hit != wantHit {
				t.Fatalf("Screen(%q) = (%q, %v), want (%q, %v)", test.text, screened, hit, test.want, wantHit)
			}
			if got := MatchString(test.text); got != wantHit {
				t.Fatalf("MatchString(%q) = %v, want %v", test.text, got, wantHit)
			}
		})
	}
}

func TestGoVerbosePassMarkerBoundary(t *testing.T) {
	for _, test := range []struct {
		name, text string
		wantHit    bool
		wantMarker bool
	}{
		{name: "top-level", text: "--- PASS: TestExample (0.00s)\n"},
		{name: "subtest", text: "    --- PASS: TestExample/case (0.01s)\n"},
		{name: "embedded-bare-assignment", text: "--- PASS: TestExample/pass=synthetic123 (0.00s)\n", wantHit: true, wantMarker: true},
		{name: "embedded-token", text: "--- PASS: TestExample/ghp_abcdefghijklmnopqrst (0.00s)\n", wantHit: true, wantMarker: true},
		{name: "real-secret-after-marker", text: "--- PASS: TestExample (0.00s)\npass: synthetic123\n", wantHit: true, wantMarker: true},
		{name: "non-marker", text: "prefix --- PASS: TestExample (0.00s)\n", wantHit: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := MatchString(test.text); got != test.wantHit {
				t.Fatalf("MatchString(%q) = %v, want %v", test.text, got, test.wantHit)
			}
			screened, hit := Screen(test.text)
			if hit != test.wantHit {
				t.Fatalf("Screen(%q) hit = %v, want %v", test.text, hit, test.wantHit)
			}
			if !test.wantHit && screened != test.text {
				t.Fatalf("Screen altered Go marker: got %q, want %q", screened, test.text)
			}
			if test.wantMarker && !strings.Contains(screened, "--- PASS: TestExample") {
				t.Fatalf("Screen removed Go marker while redacting secret: %q", screened)
			}
		})
	}
}

// goJSONTranscript is the stdout of an unchanged `go test -json ./...` run
// under go1.27.1 on a three-test fixture, with times kept as emitted.
const goJSONTranscript = `{"Time":"2026-10-08T07:50:41.159497-04:00","Action":"start","Package":"example.com/fx"}
{"Time":"2026-10-08T07:50:41.426571-04:00","Action":"run","Package":"example.com/fx","Test":"TestAlpha"}
{"Time":"2026-10-08T07:50:41.42665-04:00","Action":"output","Package":"example.com/fx","Test":"TestAlpha","Output":"=== RUN   TestAlpha\n","OutputType":"frame"}
{"Time":"2026-10-08T07:50:41.426716-04:00","Action":"output","Package":"example.com/fx","Test":"TestAlpha","Output":"--- PASS: TestAlpha (0.00s)\n","OutputType":"frame"}
{"Time":"2026-10-08T07:50:41.426736-04:00","Action":"pass","Package":"example.com/fx","Test":"TestAlpha","Elapsed":0}
{"Time":"2026-10-08T07:50:41.426747-04:00","Action":"run","Package":"example.com/fx","Test":"TestBeta"}
{"Time":"2026-10-08T07:50:41.426752-04:00","Action":"output","Package":"example.com/fx","Test":"TestBeta","Output":"=== RUN   TestBeta\n","OutputType":"frame"}
{"Time":"2026-10-08T07:50:41.426759-04:00","Action":"run","Package":"example.com/fx","Test":"TestBeta/case_one"}
{"Time":"2026-10-08T07:50:41.426769-04:00","Action":"output","Package":"example.com/fx","Test":"TestBeta/case_one","Output":"=== RUN   TestBeta/case_one\n","OutputType":"frame"}
{"Time":"2026-10-08T07:50:41.426784-04:00","Action":"output","Package":"example.com/fx","Test":"TestBeta/case_one","Output":"--- PASS: TestBeta/case_one (0.00s)\n","OutputType":"frame"}
{"Time":"2026-10-08T07:50:41.426791-04:00","Action":"pass","Package":"example.com/fx","Test":"TestBeta/case_one","Elapsed":0}
{"Time":"2026-10-08T07:50:41.426827-04:00","Action":"output","Package":"example.com/fx","Test":"TestBeta","Output":"--- PASS: TestBeta (0.00s)\n","OutputType":"frame"}
{"Time":"2026-10-08T07:50:41.426833-04:00","Action":"pass","Package":"example.com/fx","Test":"TestBeta","Elapsed":0}
{"Time":"2026-10-08T07:50:41.426839-04:00","Action":"run","Package":"example.com/fx","Test":"TestSkip"}
{"Time":"2026-10-08T07:50:41.426847-04:00","Action":"output","Package":"example.com/fx","Test":"TestSkip","Output":"=== RUN   TestSkip\n","OutputType":"frame"}
{"Time":"2026-10-08T07:50:41.426852-04:00","Action":"output","Package":"example.com/fx","Test":"TestSkip","Output":"    fx_test.go:10: nope\n"}
{"Time":"2026-10-08T07:50:41.4269-04:00","Action":"output","Package":"example.com/fx","Test":"TestSkip","Output":"--- SKIP: TestSkip (0.00s)\n","OutputType":"frame"}
{"Time":"2026-10-08T07:50:41.426914-04:00","Action":"skip","Package":"example.com/fx","Test":"TestSkip","Elapsed":0}
{"Time":"2026-10-08T07:50:41.426921-04:00","Action":"output","Package":"example.com/fx","Output":"PASS\n","OutputType":"frame"}
{"Time":"2026-10-08T07:50:41.427213-04:00","Action":"output","Package":"example.com/fx","Output":"ok  \texample.com/fx\t0.267s\n"}
{"Time":"2026-10-08T07:50:41.431041-04:00","Action":"pass","Package":"example.com/fx","Elapsed":0.272}
`

func TestLTAV0015GoJSONPassMarker(t *testing.T) {
	const (
		head  = `{"Time":"2026-10-08T07:50:41.426716-04:00","Action":"output","Package":"example.com/fx",`
		frame = `,"OutputType":"frame"}`
	)
	line := func(test, output string) string {
		return head + `"Test":"` + test + `","Output":"` + output + `"` + frame + "\n"
	}
	for _, test := range []struct {
		name, text string
		wantHit    bool
		wantMarker bool // the event is recognised as a marker despite the redacted secret
	}{
		{name: "LTA-V0-015 real-transcript", text: goJSONTranscript},
		{name: "LTA-V0-015 top-level", text: line("TestAlpha", `--- PASS: TestAlpha (0.00s)\n`)},
		{name: "LTA-V0-015 subtest", text: line("TestBeta/case_one", `--- PASS: TestBeta/case_one (0.12s)\n`)},
		{name: "LTA-V0-015 indented-subtest", text: line("TestBeta/case_one", `    --- PASS: TestBeta/case_one (0.00s)\n`)},
		{name: "LTA-V0-015 no-output-type", text: `{"Time":"2026-10-08T07:50:41Z","Action":"output","Package":"p","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}` + "\n"},
		{name: "LTA-V0-015 no-time-no-package", text: `{"Action":"output","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}`},
		{name: "LTA-V0-015 crlf-line", text: line("TestA", `--- PASS: TestA (0.00s)\n`)[:len(line("TestA", `--- PASS: TestA (0.00s)\n`))-1] + "\r\n"},

		// Credentials inside the marker's free text stay screened.
		{name: "LTA-V0-015 test-name-bare-assignment", text: line("TestBeta/pass=hunter2x9", `--- PASS: TestBeta/pass=hunter2x9 (0.00s)\n`), wantHit: true, wantMarker: true},
		{name: "LTA-V0-015 test-name-colon-assignment", text: line("TestBeta/token:abc123", `--- PASS: TestBeta/token:abc123 (0.00s)\n`), wantHit: true, wantMarker: true},
		{name: "LTA-V0-015 test-name-vendor-token", text: line("TestA/ghp_abcdefghijklmnopqrst", `--- PASS: TestA/ghp_abcdefghijklmnopqrst (0.00s)\n`), wantHit: true, wantMarker: true},
		{name: "LTA-V0-015 package-assignment", text: `{"Action":"output","Package":"example.com/password=hunter2x9","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}`, wantHit: true, wantMarker: true},
		{name: "LTA-V0-015 credential-after-marker", text: line("TestA", `--- PASS: TestA (0.00s)\n`) + `{"Action":"output","Test":"TestA","Output":"pass: hunter2x9\n"}`, wantHit: true, wantMarker: true},

		// Anything short of the exact event shape keeps the PASS: field screened.
		{name: "LTA-V0-015 test-field-mismatch", text: line("TestOther", `--- PASS: hunter2x9 (0.00s)\n`), wantHit: true},
		{name: "LTA-V0-015 action-not-output", text: `{"Action":"pass","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}`, wantHit: true},
		{name: "LTA-V0-015 output-type-not-frame", text: `{"Action":"output","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n","OutputType":"error"}`, wantHit: true},
		{name: "LTA-V0-015 extra-field", text: `{"Action":"output","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n","Note":"x"}`, wantHit: true},
		{name: "LTA-V0-015 field-order", text: `{"Test":"TestA","Action":"output","Output":"--- PASS: TestA (0.00s)\n"}`, wantHit: true},
		{name: "LTA-V0-015 output-continues", text: `{"Action":"output","Test":"TestA","Output":"--- PASS: TestA (0.00s)\nmore\n"}`, wantHit: true},
		{name: "LTA-V0-015 output-prefix", text: `{"Action":"output","Test":"TestA","Output":"x --- PASS: TestA (0.00s)\n"}`, wantHit: true},
		{name: "LTA-V0-015 escaped-quote-in-name", text: `{"Action":"output","Test":"TestA\"x","Output":"--- PASS: TestA\"x (0.00s)\n"}`, wantHit: true},
		{name: "LTA-V0-015 escaped-unicode-in-name", text: `{"Action":"output","Test":"TestA x","Output":"--- PASS: TestA x (0.00s)\n"}`, wantHit: true},
		{name: "LTA-V0-015 leading-text", text: `log {"Action":"output","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}`, wantHit: true},
		{name: "LTA-V0-015 other-json-value", text: `{"note":"--- PASS: TestA (0.00s)\n"}`, wantHit: true},
		{name: "LTA-V0-015 unterminated", text: `{"Action":"output","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n`, wantHit: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := MatchString(test.text); got != test.wantHit {
				t.Fatalf("MatchString(%q) = %v, want %v", test.text, got, test.wantHit)
			}
			screened, hit := Screen(test.text)
			if hit != test.wantHit || !test.wantHit && screened != test.text {
				t.Fatalf("Screen(%q) = (%q, %v), want hit %v", test.text, screened, hit, test.wantHit)
			}
			// The structural prefix is masked exactly for a recognised event.
			if recognised := strings.Contains(maskGoPassMarkers(test.text), "--- GOOK  Test"); recognised != (!test.wantHit || test.wantMarker) {
				t.Fatalf("marker recognised = %v for %q", recognised, test.text)
			}
		})
	}
}
