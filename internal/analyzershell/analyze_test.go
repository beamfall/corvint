package analyzershell

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func fixture() Request {
	body := "#!/bin/sh\nROOT=${ROOT}\nexport HOME=/tmp\n. lib/common.sh\ntrap 'exit 0' EXIT\nif [ \"$0\" = \"$BASH_SOURCE\" ]; then\n  beamfallctl status\nfi\n"
	sum := sha256.Sum256([]byte(body))
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	value := "sha256:" + hex.EncodeToString(sum[:])
	inputs := make([]Input, 4)
	for i := range inputs {
		inputs[i] = Input{"input-" + string(rune('1'+i)), "shell.posix-bash", "scripts/run-" + string(rune('1'+i)) + ".sh", value, encoded}
	}
	return Request{Profile, Family, "req-1", "scope", "unit", Target{"darwin", "arm64", "none", []string{}}, inputs}
}
func wire(t *testing.T, r Request) []byte {
	t.Helper()
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	return append(b, '\n')
}
func TestExactCanonicalFacts(t *testing.T) {
	r := fixture()
	if !pathOK("lib/common.sh") {
		t.Fatal("path")
	}
	got := AnalyzeCanonical(wire(t, r))
	var s success
	if e := json.Unmarshal(got, &s); e != nil {
		t.Fatal(e)
	}
	if s.Status != "CANDIDATE" || len(s.Facts) != 32 {
		t.Fatalf("%s", got)
	}
	if got[len(got)-1] != '\n' || strings.Count(string(got), "\n") != 1 {
		t.Fatal("LF framing")
	}
	for i := 1; i < len(s.Facts); i++ {
		if factKey(s.Facts[i-1]) >= factKey(s.Facts[i]) {
			t.Fatal("unordered")
		}
	}
}
func TestLexerHandlesQuotedCommentsAndFixtureLines(t *testing.T) {
	r := fixture()
	if !pathOK("lib/common.sh") {
		t.Fatal("path")
	}
	w, _, _ := shellWords(". lib/common.sh")
	if len(w) != 2 || w[0] != "." {
		t.Fatalf("words=%q", w)
	}
	body, _ := base64.StdEncoding.DecodeString(r.Inputs[0].ContentBase64)
	for _, line := range strings.Split(strings.TrimSuffix(string(body), "\n"), "\n") {
		candidate := r
		candidate.Inputs = []Input{candidate.Inputs[0]}
		candidate.Inputs[0].ContentBase64 = base64.StdEncoding.EncodeToString([]byte(line + "\n"))
		s := sha256.Sum256([]byte(line + "\n"))
		candidate.Inputs[0].SHA256 = "sha256:" + hex.EncodeToString(s[:])
		_, why := lexShell([]byte(line + "\n"))
		if why != "" {
			t.Fatal(line, ": ", why)
		}
	}
}
func TestLexerRejectsDynamicAndIgnoresInertBodies(t *testing.T) {
	r := fixture()
	body := "# $(bad)\nx='$(inert)'\ncat <<'END'\n$(inert)\nEND\nsource $DYNAMIC\n"
	sum := sha256.Sum256([]byte(body))
	r.Inputs[0].ContentBase64 = base64.StdEncoding.EncodeToString([]byte(body))
	r.Inputs[0].SHA256 = "sha256:" + hex.EncodeToString(sum[:])
	if got := string(AnalyzeCanonical(wire(t, r))); !strings.Contains(got, "DYNAMIC_INPUT") {
		t.Fatal(got)
	}
}
func TestLexerPreservesQuotedEscapedAndHeredocProvenance(t *testing.T) {
	for _, body := range []string{"echo '\\$(inert)' \\$HOME\n"} {
		r := fixture()
		sum := sha256.Sum256([]byte(body))
		r.Inputs[0].ContentBase64 = base64.StdEncoding.EncodeToString([]byte(body))
		r.Inputs[0].SHA256 = "sha256:" + hex.EncodeToString(sum[:])
		if got := string(AnalyzeCanonical(wire(t, r))); !strings.Contains(got, "\"status\":\"CANDIDATE\"") {
			t.Fatal(got)
		}
	}
}
func TestEchoValidationPrecedesBoundResponse(t *testing.T) {
	r := fixture()
	r.Target.Features = nil
	got := string(AnalyzeCanonical(wire(t, r)))
	if !strings.Contains(got, "\"family\":\"unknown\"") {
		t.Fatal(got)
	}
	r = fixture()
	r.Inputs = append(r.Inputs, r.Inputs[0])
	got = string(AnalyzeCanonical(wire(t, r)))
	if !strings.Contains(got, "\"family\":\"unknown\"") {
		t.Fatal(got)
	}
}
func TestSafeCanonicalExtensionBindsUnknownField(t *testing.T) {
	frame := wire(t, fixture())
	var request Request
	if err := json.Unmarshal(frame, &request); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSuffix(string(frame), "}\n")
	for _, edited := range []string{
		base + `,"x":[9007199254740993,{"a":"b","c":null}]}`,
		base + `,"target inputs":0}`,
		strings.Replace(base, `"features":[]`, `"features":[],"x":true`, 1) + "}",
		strings.Replace(base, `"}]`, `","x":0,"y":"z"}]`, 1) + "}",
	} {
		if got, want := string(AnalyzeCanonical([]byte(edited+"\n"))), string(bound(request, "UNKNOWN_FIELD")); got != want {
			t.Fatalf("frame=%s\ngot=%s\nwant=%s", edited, got, want)
		}
	}
	for edited, reason := range map[string]string{
		base + `,"x":1.0}`: "NONCANONICAL_REQUEST", base + `,"y":0,"x":0}`: "NONCANONICAL_REQUEST", base + `,"x":{"b":0,"a":0}}`: "NONCANONICAL_REQUEST",
		strings.Replace(base, `{"profile":`, `{"x":0,"profile":`, 1) + "}": "NONCANONICAL_REQUEST",
		strings.Replace(base, `"unit`, `"!unit`, 1) + `,"x":0}`:            "INVALID_IDENTIFIER",
	} {
		if got := string(AnalyzeCanonical([]byte(edited + "\n"))); got != string(sentinel(reason)) {
			t.Fatalf("frame=%s\ngot=%s", edited, got)
		}
	}
}

func TestShellDialectCoordinatesAndSeparators(t *testing.T) {
	for _, test := range []struct {
		name, body, reason string
		target             Target
	}{
		{"posix-separators", "one; two && three | four & five\n", "", Target{"darwin", "arm64", "none", []string{}}},
		{"bash-source", "source lib/common.sh\n", "", Target{"linux", "amd64", "none", []string{"bash-5.2"}}},
		{"posix-source", "source lib/common.sh\n", "UNSUPPORTED_SCHEMA", Target{"darwin", "arm64", "none", []string{}}},
		{"positional", "echo $1\n", "DYNAMIC_INPUT", Target{"darwin", "arm64", "none", []string{}}},
		{"substitution", "echo $(bad)\n", "DYNAMIC_INPUT", Target{"darwin", "arm64", "none", []string{}}},
		{"unsupported-coordinate", "echo okay\n", "UNSUPPORTED_SCHEMA", Target{"windows", "amd64", "none", []string{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := fixture()
			r.Target = test.target
			sum := sha256.Sum256([]byte(test.body))
			r.Inputs[0].ContentBase64 = base64.StdEncoding.EncodeToString([]byte(test.body))
			r.Inputs[0].SHA256 = "sha256:" + hex.EncodeToString(sum[:])
			got := string(AnalyzeCanonical(wire(t, r)))
			if test.reason == "" {
				if !strings.Contains(got, "\"status\":\"CANDIDATE\"") {
					t.Fatal(got)
				}
				return
			}
			if !strings.Contains(got, "\"reason\":\""+test.reason+"\"") || !strings.Contains(got, "\"input_echoes\"") {
				t.Fatal(got)
			}
		})
	}
}

func TestTokenStreamPreservesQuoteEscapeAndExpansionProvenance(t *testing.T) {
	for _, test := range []struct {
		name, body, reason string
		read, write        string
	}{
		{"single_quote_and_escape_are_inert", "A='$(inert)'\necho '\\$SECRET' \\$PATH \"${HOME}\"\n", "", "HOME", "A"},
		{"command_substitution", "echo $(unsafe)\n", "DYNAMIC_INPUT", "", ""},
		{"arithmetic_expansion", "echo $((1 + 1))\n", "DYNAMIC_INPUT", "", ""},
		{"braced_positional", "echo ${0}\n", "DYNAMIC_INPUT", "", ""},
		{"positional", "echo $1\n", "DYNAMIC_INPUT", "", ""},
		{"escaped_separator_is_argument", "echo \\; literal\n", "", "", ""},
		{"quoted_comment_is_not_comment", "echo '# literal' # comment\n", "", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := fixtureWithBody(t, test.body, Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
			got := AnalyzeCanonical(wire(t, r))
			if test.reason != "" {
				assertReason(t, got, test.reason)
				return
			}
			var value success
			if err := json.Unmarshal(got, &value); err != nil || value.Status != "CANDIDATE" {
				t.Fatalf("candidate=%q err=%v", got, err)
			}
			if test.read != "" && !hasShellFact(value.Facts, "shell.env.read", test.read) {
				t.Fatalf("missing read %s: %#v", test.read, value.Facts)
			}
			if test.write != "" && !hasShellFact(value.Facts, "shell.env.write", test.write) {
				t.Fatalf("missing write %s: %#v", test.write, value.Facts)
			}
			if hasShellFact(value.Facts, "shell.env.read", "SECRET") || hasShellFact(value.Facts, "shell.env.read", "PATH") {
				t.Fatalf("forged escaped or single-quoted read: %#v", value.Facts)
			}
		})
	}
}

func TestTokenStreamFailsClosedForGrammarAndSupportsStaticRedirections(t *testing.T) {
	bash := Target{OS: "linux", Architecture: "amd64", ABI: "none", Features: []string{"bash-5.2"}}
	posix := Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}
	for _, test := range []struct {
		name, body, reason string
		target             Target
	}{
		{"ordinary_redirects", "echo ok >/dev/null\ncat < input\nagain >>out\n", "", posix},
		{"quoted_heredoc_with_tail", "cat <<'END' ; echo after\n$(inert)\nEND\n", "", posix},
		{"escaped_heredoc", "cat <<\\END\n$(inert)\nEND\n", "", posix},
		{"tab_stripped_heredoc", "cat <<-END\n\t$(inert)\n\tEND\n", "", posix},
		{"unquoted_heredoc_is_unsupported", "cat <<END\nbody\nEND\n", "UNSUPPORTED_SCHEMA", posix},
		{"unterminated_heredoc", "cat <<'END'\nbody\nND\n", "MALFORMED_INPUT", posix},
		{"missing_heredoc_delimiter", "cat << ; echo later\n", "MALFORMED_INPUT", posix},
		{"adjacent_separator", "echo && && danger\n", "MALFORMED_INPUT", posix},
		{"bash_source", "source lib/common.sh\n", "", bash},
		{"posix_source", "source lib/common.sh\n", "UNSUPPORTED_SCHEMA", posix},
		{"bash_pipe_stderr_is_not_split", "one |& two\n", "UNSUPPORTED_SCHEMA", bash},
		{"posix_pipe_stderr_is_not_split", "one |& two\n", "UNSUPPORTED_SCHEMA", posix},
		{"array_assignment_never_becomes_write", "A=(one)\n", "UNSUPPORTED_SCHEMA", posix},
		{"if_elif_else", "if test \"$HOME\"; then\n yes\nelif false; then\n no\nelse\n fallback\nfi\n", "", posix},
		{"static_group", "{ echo grouped; }\n", "", posix},
		{"static_group_newline_terminator", "{\n echo grouped\n}\n", "", posix},
		{"static_group_ampersand_terminator", "{ echo grouped & }\n", "", posix},
		{"nested_static_group", "{ { nested; }; }\n", "", posix},
		{"empty_static_group", "{ }\n", "MALFORMED_INPUT", posix},
		{"static_group_without_terminator", "{ grouped }\n", "MALFORMED_INPUT", posix},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := AnalyzeCanonical(wire(t, fixtureWithBody(t, test.body, test.target)))
			if test.reason != "" {
				assertReason(t, got, test.reason)
				return
			}
			var value success
			if err := json.Unmarshal(got, &value); err != nil || value.Status != "CANDIDATE" {
				t.Fatalf("candidate=%q err=%v", got, err)
			}
		})
	}
}

func TestBashReservedControlsNeverBecomeStaticFacts(t *testing.T) {
	bash := Target{OS: "linux", Architecture: "amd64", ABI: "none", Features: []string{"bash-5.2"}}
	for _, control := range []string{"!", "case", "coproc", "do", "done", "elif", "else", "esac", "fi", "for", "function", "in", "select", "then", "time", "until", "while", "[[", "]]"} {
		t.Run("bare_"+control, func(t *testing.T) {
			body := control + " echo hi\n"
			request := fixtureWithBody(t, body, bash)
			facts, why := parse(request.Inputs[0], []byte(body), request)
			if why == "" || len(facts) != 0 {
				t.Fatalf("control=%s facts=%#v reason=%q", control, facts, why)
			}
		})
	}
	for _, control := range []string{"case", "coproc", "do", "done", "elif", "else", "esac", "fi", "for", "function", "if", "in", "select", "then", "time", "until", "while"} {
		for _, body := range []string{"\"" + control + "\"\n", "\\" + control + "\n"} {
			t.Run("literal_"+control+"_"+strconv.Itoa(len(body)), func(t *testing.T) {
				value := mustSuccess(t, AnalyzeCanonical(wire(t, fixtureWithBody(t, body, bash))))
				if !hasShellFact(value.Facts, "shell.command.static", control) {
					t.Fatalf("control=%s facts=%#v", control, value.Facts)
				}
			})
		}
	}
}

func TestUnsetRequiresPlainIdentifiersInBothDialects(t *testing.T) {
	for _, target := range []Target{
		{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
		{OS: "linux", Architecture: "amd64", ABI: "none", Features: []string{"bash-5.2"}},
	} {
		t.Run(strings.Join(target.Features, ",")+target.OS, func(t *testing.T) {
			valid := mustSuccess(t, AnalyzeCanonical(wire(t, fixtureWithBody(t, "unset HOME PATH\n", target))))
			for _, name := range []string{"HOME", "PATH"} {
				if !hasShellFact(valid.Facts, "shell.env.write", name) {
					t.Fatalf("target=%+v missing unset %s: %#v", target, name, valid.Facts)
				}
			}
			for _, body := range []string{"unset HOME=value\n", "unset \"HOME\"\n", "unset \\HOME\n"} {
				got := AnalyzeCanonical(wire(t, fixtureWithBody(t, body, target)))
				assertReason(t, got, "UNSUPPORTED_SCHEMA")
			}
		})
	}
}

func TestDuplicateFactsAreTerminal(t *testing.T) {
	request := fixtureWithBody(t, "tool\ntool\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
	assertCompleteBoundFailure(t, AnalyzeCanonical(wire(t, request)), request, "DUPLICATE_VALUE")
}

func TestControlWordsRequireUnquotedUnescapedProvenanceAndBranchesAreNonempty(t *testing.T) {
	posix := Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}
	for _, test := range []struct {
		name, body, reason string
	}{
		{"quoted_fi_is_a_command_not_a_closer", "if true; then\n  \"fi\"\nfi\n", ""},
		{"escaped_fi_is_a_command_not_a_closer", "if true; then\n  \\fi\nfi\n", ""},
		{"quoted_else_does_not_close_then_list", "if true; then\n  \"else\"\nfi\n", ""},
		{"escaped_elif_does_not_close_then_list", "if true; then\n  \\elif\nfi\n", ""},
		{"empty_then", "if true; then\nfi\n", "MALFORMED_INPUT"},
		{"empty_else", "if true; then\ntrue\nelse\nfi\n", "MALFORMED_INPUT"},
		{"empty_elif_then", "if true; then\ntrue\nelif false; then\nfi\n", "MALFORMED_INPUT"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := AnalyzeCanonical(wire(t, fixtureWithBody(t, test.body, posix)))
			if test.reason != "" {
				assertReason(t, got, test.reason)
				return
			}
			var value success
			if err := json.Unmarshal(got, &value); err != nil || value.Status != "CANDIDATE" {
				t.Fatalf("candidate=%q err=%v", got, err)
			}
		})
	}
}

func TestDoubleQuoteBackslashAndExpansionFormsFailClosed(t *testing.T) {
	posix := Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}
	for _, test := range []struct {
		name, body string
	}{
		{"double_quote_preserves_non_special_backslash", ". \"lib\\q.sh\"\n"},
		{"glob_star", "echo *\n"},
		{"glob_question", "echo ?\n"},
		{"glob_brackets", "echo [abc]\n"},
		{"tilde", "echo ~/lib.sh\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := AnalyzeCanonical(wire(t, fixtureWithBody(t, test.body, posix)))
			assertReason(t, got, "DYNAMIC_INPUT")
		})
	}
	tokens, why := lexShell([]byte(". \"lib\\q.sh\"\n"))
	if why != "" || len(tokens) < 2 || tokens[1].word.text != `lib\q.sh` {
		t.Fatalf("tokens=%#v why=%q", tokens, why)
	}
}

func TestTokenStreamFactAndOutputAdmissionIsProspective(t *testing.T) {
	var body strings.Builder
	for i := 0; i < MaxFacts+1024; i++ {
		body.WriteString("tool")
		body.WriteString(string(rune('a' + i%26)))
		body.WriteString(strings.Repeat("x", i/26))
		body.WriteByte('\n')
	}
	got := AnalyzeCanonical(wire(t, fixtureWithBody(t, body.String(), Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})))
	if len(got) > MaxOutputBytes || (!strings.Contains(string(got), "LIMIT_EXCEEDED") && !strings.Contains(string(got), "OUTPUT_LIMIT")) {
		t.Fatalf("admission=%q", got)
	}
}

func fixtureWithBody(t *testing.T, body string, target Target) Request {
	t.Helper()
	r := fixture()
	sum := sha256.Sum256([]byte(body))
	r.Target = target
	r.Inputs = []Input{{Handle: "single", Family: "shell.posix-bash", Path: "script/test.sh", SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(body))}}
	return r
}

func assertReason(t *testing.T, got []byte, reason string) {
	t.Helper()
	var value failure
	if err := json.Unmarshal(got, &value); err != nil || value.Reason != reason {
		t.Fatalf("reason=%q want=%q err=%v", value.Reason, reason, err)
	}
}

func assertSentinel(t *testing.T, got []byte, reason string) {
	t.Helper()
	want := []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"` + reason + `"}` + "\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("sentinel reason=%s\ngot:  %q\nwant: %q", reason, got, want)
	}
}

func hasShellFact(facts []Fact, kind, value string) bool {
	for _, fact := range facts {
		if fact.Kind == kind && fact.Value == value {
			return true
		}
	}
	return false
}
func TestPreEnvelopeDuplicateNeverEchoes(t *testing.T) {
	r := fixture()
	r.Inputs = append(r.Inputs, r.Inputs[0])
	got := string(AnalyzeCanonical(wire(t, r)))
	if !strings.Contains(got, "\"family\":\"unknown\"") || strings.Contains(got, "input_echoes") {
		t.Fatal(got)
	}
}
func TestFreshCanonicalRuns(t *testing.T) {
	raw := wire(t, fixture())
	want := string(AnalyzeCanonical(raw))
	for run := 0; run < 1000; run++ {
		if got := string(AnalyzeCanonical(append([]byte(nil), raw...))); got != want {
			t.Fatalf("run=%d got=%s", run, got)
		}
	}
}
func TestPinnedShellBlobExpectations(t *testing.T) {
	for _, blob := range []struct{ name, commit, path, sha256, reason string }{
		{"beamfall", "da38c59eb30b2121cbac37b912485b30b2e54841", "script/context-packet.sh", "84108938b18b12a8a6a67990d565f66cfc7170683fef099fb0cf7bc2fe886150", "DYNAMIC_INPUT"},
		{"relay", "9723152fdd4ead36b32553a171d98749e4fc23e9", "script/relay-core-e2e.sh", "66b02253abc5faa372441776a06820caecf8a8831c075e672ee120958c4868b1", "DYNAMIC_INPUT"},
		{"sdk", "5536ecde6d12214d6786a6832e537bd5d4feca02", "script/gate.sh", "4a21753f97f725d2f2b656b8befa1bcf292c630c629f17982c7b1609212aaef4", "DYNAMIC_INPUT"},
	} {
		t.Run(blob.name, func(t *testing.T) {
			if len(blob.commit) != 40 || len(blob.sha256) != 64 || !pathOK(blob.path) || blob.reason != "DYNAMIC_INPUT" {
				t.Fatalf("invalid literal pin: %+v", blob)
			}
		})
	}
}
func TestRejectsDynamicAndBindsWholeEnvelope(t *testing.T) {
	r := fixture()
	body := "eval \"$x\"\n"
	sum := sha256.Sum256([]byte(body))
	r.Inputs[0].ContentBase64 = base64.StdEncoding.EncodeToString([]byte(body))
	r.Inputs[0].SHA256 = "sha256:" + hex.EncodeToString(sum[:])
	got := string(AnalyzeCanonical(wire(t, r)))
	for _, v := range []string{"\"reason\":\"DYNAMIC_INPUT\"", "\"scope_id\":\"scope\"", "\"compilation_unit_id\":\"unit\"", "\"input_echoes\""} {
		if !strings.Contains(got, v) {
			t.Fatal(got)
		}
	}
}
func TestRejectsDigestAndNoncanonical(t *testing.T) {
	r := fixture()
	r.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	if !strings.Contains(string(AnalyzeCanonical(wire(t, r))), "DIGEST_MISMATCH") {
		t.Fatal("digest")
	}
	if !strings.Contains(string(AnalyzeCanonical(append([]byte(" "), wire(t, fixture())...))), "NONCANONICAL_REQUEST") {
		t.Fatal("canonical")
	}
}
func BenchmarkAnalyzeCandidate(b *testing.B) {
	raw, err := json.Marshal(fixture())
	if err != nil {
		b.Fatal(err)
	}
	raw = append(raw, '\n')
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = AnalyzeCanonical(raw)
	}
}

func TestHardAllocationOutputAndSourceRatchets(t *testing.T) {
	if raceBuild {
		t.Skip("race instrumentation is outside the absolute allocation and latency profile")
	}
	raw := wire(t, fixture())
	if got := len(AnalyzeCanonical(raw)); got > MaxOutputBytes {
		t.Fatalf("output bytes=%d", got)
	}
	production := benchmarkCandidate(raw, AnalyzeCanonical)
	if got := production.AllocedBytesPerOp(); got > MaxBytesPerOp {
		t.Fatalf("production bytes/op=%d max=%d", got, MaxBytesPerOp)
	}
	if got := production.AllocsPerOp(); got > MaxAllocsPerOp {
		t.Fatalf("production allocs/op=%d max=%d", got, MaxAllocsPerOp)
	}
	restoredCopy := benchmarkCandidate(raw, func(input []byte) []byte {
		return append([]byte(nil), AnalyzeCanonical(input)...)
	})
	if got := restoredCopy.AllocedBytesPerOp(); got <= MaxBytesPerOp {
		t.Fatalf("restored response-copy alternative bytes/op=%d must fail max=%d", got, MaxBytesPerOp)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	total := 0
	for _, name := range productionSourceFiles(t, root) {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		total += len(content)
	}
	if total > MaxSourceBytes {
		t.Fatalf("production source bytes=%d max=%d", total, MaxSourceBytes)
	}
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-shell")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", binary, "./cmd/corvint-analyzer-shell")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("binary build: %v\n%s", err, output)
	}
	info, err := os.Stat(binary)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > MaxBinaryBytes {
		t.Fatalf("binary bytes=%d max=%d", info.Size(), MaxBinaryBytes)
	}
}

func benchmarkCandidate(raw []byte, analyze func([]byte) []byte) testing.BenchmarkResult {
	return testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = analyze(raw)
		}
	})
}

func productionSourceFiles(t *testing.T, root string) []string {
	t.Helper()
	files := make([]string, 0, 4)
	for _, dir := range []string{filepath.Join(root, "internal", "analyzershell"), filepath.Join(root, "cmd", "corvint-analyzer-shell")} {
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no production source files")
	}
	return files
}

func TestPublicBoundsHaveJustUnderAtAndOverWitnesses(t *testing.T) {
	for _, test := range []struct {
		name  string
		bound int
	}{
		{"request", MaxRequestBytes},
		{"input", MaxInputBytes},
		{"output", MaxOutputBytes},
		{"facts", MaxFacts},
		{"source", MaxSourceBytes},
		{"binary", MaxBinaryBytes},
		{"bytes_per_op", MaxBytesPerOp},
		{"allocs_per_op", MaxAllocsPerOp},
	} {
		t.Run(test.name+"_comparison", func(t *testing.T) {
			for _, edge := range []struct {
				name  string
				value int
				want  bool
			}{
				{"just_under", test.bound - 1, false},
				{"at", test.bound, false},
				{"over", test.bound + 1, true},
			} {
				if got := overBound(edge.value, test.bound); got != edge.want {
					t.Fatalf("%s value=%d bound=%d over=%t want=%t", edge.name, edge.value, test.bound, got, edge.want)
				}
			}
		})
	}
	t.Run("request_bytes", func(t *testing.T) {
		for _, size := range []int{MaxRequestBytes - 1, MaxRequestBytes, MaxRequestBytes + 1} {
			raw := append(bytes.Repeat([]byte{' '}, size-1), '\n')
			got := AnalyzeCanonical(raw)
			assertSentinel(t, got, "NONCANONICAL_REQUEST")
		}
	})
	t.Run("input_bytes", func(t *testing.T) {
		for _, size := range []int{MaxInputBytes - 1, MaxInputBytes, MaxInputBytes + 1} {
			body := "#" + strings.Repeat("x", size-2) + "\n"
			got := AnalyzeCanonical(wire(t, fixtureWithBody(t, body, Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})))
			if size <= MaxInputBytes {
				var value success
				if err := json.Unmarshal(got, &value); err != nil || value.Status != "CANDIDATE" {
					t.Fatalf("bytes=%d candidate=%q err=%v", size, got, err)
				}
			} else {
				assertReason(t, got, "LIMIT_EXCEEDED")
			}
		}
	})
	t.Run("facts", func(t *testing.T) {
		r := Request{Profile: Profile, Family: Family, RequestID: "a", ScopeID: "a", CompilationUnitID: "a", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}}
		collector := newFactCollector(r)
		in := Input{Handle: "a", Path: "a.sh", SHA256: "sha256:" + strings.Repeat("0", 64)}
		for i := 0; i < MaxFacts; i++ {
			if why := collector.add(in, "shell.command.static", "invokes", "tool"+strconv.Itoa(i)); why != "" {
				t.Fatalf("fact=%d reason=%s", i, why)
			}
		}
		if why := collector.add(in, "shell.command.static", "invokes", "overflow"); why != "LIMIT_EXCEEDED" {
			t.Fatalf("fact overflow=%q", why)
		}
	})
	t.Run("output", func(t *testing.T) {
		r := fixtureWithBody(t, "true\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
		collector := newFactCollector(r)
		in := r.Inputs[0]
		if why := collector.add(in, "shell.command.static", "invokes", strings.Repeat("x", MaxFactFieldBytes)); why != "" {
			t.Fatalf("fact field at bound reason=%q", why)
		}
		if why := collector.add(in, "shell.command.static", "invokes", strings.Repeat("y", MaxFactFieldBytes+1)); why != "LIMIT_EXCEEDED" {
			t.Fatalf("fact field overflow=%q", why)
		}
	})
}

func TestFramedLimitsHaveUnderAtAndOverWitnesses(t *testing.T) {
	t.Run("input_count", func(t *testing.T) {
		for _, count := range []int{MaxInputs - 1, MaxInputs, MaxInputs + 1} {
			request := shellInputCountRequest(t, count)
			got := AnalyzeCanonical(wire(t, request))
			if count <= MaxInputs {
				if value := mustSuccess(t, got); len(value.InputEchoes) != count {
					t.Fatalf("count=%d echoes=%d", count, len(value.InputEchoes))
				}
				continue
			}
			assertSentinel(t, got, "LIMIT_EXCEEDED")
		}
	})
	t.Run("feature_count", func(t *testing.T) {
		for _, count := range []int{MaxFeatures - 1, MaxFeatures, MaxFeatures + 1} {
			request := fixtureWithBody(t, "tool\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: featureNames(count)})
			got := AnalyzeCanonical(wire(t, request))
			if count <= MaxFeatures {
				assertCompleteBoundFailure(t, got, request, "UNSUPPORTED_SCHEMA")
				continue
			}
			assertSentinel(t, got, "LIMIT_EXCEEDED")
		}
	})
	t.Run("aggregate_base64", func(t *testing.T) {
		for _, encoded := range []int{MaxAggregateBase64Bytes - 4, MaxAggregateBase64Bytes, MaxAggregateBase64Bytes + 4} {
			body := commentBody(base64DecodedBytes(encoded))
			request := fixtureWithBody(t, body, Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
			if got := len(request.Inputs[0].ContentBase64); got != encoded {
				t.Fatalf("encoded=%d got=%d", encoded, got)
			}
			got := AnalyzeCanonical(wire(t, request))
			if encoded <= MaxAggregateBase64Bytes {
				mustSuccess(t, got)
				continue
			}
			assertSentinel(t, got, "LIMIT_EXCEEDED")
		}
	})
	t.Run("identifier", func(t *testing.T) {
		for _, length := range []int{MaxIdentifierBytes - 1, MaxIdentifierBytes, MaxIdentifierBytes + 1} {
			request := fixtureWithBody(t, "tool\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
			request.RequestID = "r" + strings.Repeat("x", length-1)
			got := AnalyzeCanonical(wire(t, request))
			if length <= MaxIdentifierBytes {
				mustSuccess(t, got)
				continue
			}
			assertSentinel(t, got, "INVALID_IDENTIFIER")
		}
	})
	t.Run("path_and_fact_subject", func(t *testing.T) {
		for _, length := range []int{MaxPathBytes - 1, MaxPathBytes, MaxPathBytes + 1} {
			request := fixtureWithBody(t, "tool\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
			request.Inputs[0].Path = shellPath(length, 0)
			got := AnalyzeCanonical(wire(t, request))
			if length > MaxPathBytes {
				assertSentinel(t, got, "MALFORMED_INPUT")
				continue
			}
			value := mustSuccess(t, got)
			if len(value.Facts) != 1 || value.Facts[0].Subject != request.Inputs[0].Path || value.Facts[0].InstanceID != request.Inputs[0].Path {
				t.Fatalf("path=%d facts=%#v", length, value.Facts)
			}
		}
	})
	t.Run("path_segment", func(t *testing.T) {
		for _, length := range []int{MaxPathSegmentBytes - 1, MaxPathSegmentBytes, MaxPathSegmentBytes + 1} {
			request := fixtureWithBody(t, "tool\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
			request.Inputs[0].Path = strings.Repeat("a", length-3) + ".sh"
			got := AnalyzeCanonical(wire(t, request))
			if length <= MaxPathSegmentBytes {
				mustSuccess(t, got)
				continue
			}
			assertSentinel(t, got, "MALFORMED_INPUT")
		}
	})
	t.Run("fact_field_import", func(t *testing.T) {
		for _, length := range []int{MaxFactFieldBytes - 1, MaxFactFieldBytes, MaxFactFieldBytes + 1} {
			importPath := shellPath(length, 1)
			request := fixtureWithBody(t, ". "+importPath+"\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
			got := AnalyzeCanonical(wire(t, request))
			if length > MaxFactFieldBytes {
				assertCompleteBoundFailure(t, got, request, "DYNAMIC_INPUT")
				continue
			}
			value := mustSuccess(t, got)
			if len(value.Facts) != 1 || value.Facts[0].Value != importPath || len(value.Facts[0].Value) != length {
				t.Fatalf("field=%d facts=%#v", length, value.Facts)
			}
		}
	})
	t.Run("json_depth", func(t *testing.T) {
		request := fixtureWithBody(t, "tool\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
		for _, arrays := range []int{MaxJSONDepth - 2, MaxJSONDepth - 1, MaxJSONDepth} {
			got := AnalyzeCanonical(nestedUnknownJSON(t, request, arrays))
			if arrays < MaxJSONDepth {
				assertCompleteBoundFailure(t, got, request, "UNKNOWN_FIELD")
				continue
			}
			assertSentinel(t, got, "NONCANONICAL_REQUEST")
		}
	})
	t.Run("json_tokens", func(t *testing.T) {
		request := fixtureWithBody(t, "tool\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
		for _, values := range []int{MaxJSONTokens - 20, MaxJSONTokens - 19, MaxJSONTokens - 18} {
			got := AnalyzeCanonical(unknownTokenJSON(t, request, values))
			if values <= MaxJSONTokens-19 {
				assertCompleteBoundFailure(t, got, request, "UNKNOWN_FIELD")
				continue
			}
			assertSentinel(t, got, "NONCANONICAL_REQUEST")
		}
	})
	t.Run("derived_facts", func(t *testing.T) {
		for _, count := range []int{MaxFacts - 1, MaxFacts, MaxFacts + 1} {
			request := fixtureWithBody(t, staticCommands(count), Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
			got := AnalyzeCanonical(wire(t, request))
			if count <= MaxFacts {
				if value := mustSuccess(t, got); len(value.Facts) != count {
					t.Fatalf("facts=%d got=%d", count, len(value.Facts))
				}
				continue
			}
			assertCompleteBoundFailure(t, got, request, "LIMIT_EXCEEDED")
		}
	})
	t.Run("output_bytes", func(t *testing.T) {
		for _, size := range []int{MaxOutputBytes - 1, MaxOutputBytes, MaxOutputBytes + 1} {
			request := outputSizedRequest(t, size)
			got := AnalyzeCanonical(wire(t, request))
			if size <= MaxOutputBytes {
				if len(got) != size {
					t.Fatalf("output=%d got=%d", size, len(got))
				}
				mustSuccess(t, got)
				continue
			}
			assertCompleteBoundFailure(t, got, request, "OUTPUT_LIMIT")
		}
	})
}

func shellInputCountRequest(t *testing.T, count int) Request {
	t.Helper()
	request := fixtureWithBody(t, "#\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
	request.Inputs = make([]Input, count)
	for index := range request.Inputs {
		input := fixtureWithBody(t, "#\n", request.Target).Inputs[0]
		input.Handle = "input-" + fmtIndex(index, 3)
		input.Path = "script/" + fmtIndex(index, 3) + ".sh"
		request.Inputs[index] = input
	}
	return request
}

func featureNames(count int) []string {
	features := make([]string, count)
	for index := range features {
		features[index] = "feature-" + fmtIndex(index, 3)
	}
	return features
}

func base64DecodedBytes(encoded int) int { return (encoded/4-1)*3 + 1 }

func commentBody(length int) string { return "#" + strings.Repeat("x", length-2) + "\n" }

func fmtIndex(index, width int) string {
	value := strconv.Itoa(index)
	return strings.Repeat("0", width-len(value)) + value
}

func shellPath(length, index int) string {
	if length < 4 {
		panic("short shell path")
	}
	remaining := length
	parts := make([]string, 0, length/MaxPathSegmentBytes+1)
	for remaining > MaxPathSegmentBytes {
		partLength := MaxPathSegmentBytes
		if remaining-partLength-1 < 4 {
			partLength = remaining - 5
		}
		part := strings.Repeat("a", partLength)
		if len(parts) == 0 && partLength >= 3 {
			part = fmtIndex(index, 3) + part[3:]
		}
		parts = append(parts, part)
		remaining -= partLength + 1
	}
	part := strings.Repeat("a", remaining-3) + ".sh"
	if len(parts) == 0 && len(part) >= 3 {
		part = fmtIndex(index, 3) + part[3:]
	}
	parts = append(parts, part)
	return strings.Join(parts, "/")
}

func nestedUnknownJSON(t *testing.T, request Request, arrays int) []byte {
	t.Helper()
	base := wire(t, request)
	value := strings.Repeat("[", arrays) + "null" + strings.Repeat("]", arrays)
	return append(append(base[:len(base)-2], []byte(`,"probe":`+value)...), '}', '\n')
}

func unknownTokenJSON(t *testing.T, request Request, values int) []byte {
	t.Helper()
	base := wire(t, request)
	value := strings.Repeat("null,", values-1) + "null"
	return append(append(base[:len(base)-2], []byte(`,"probe":[`+value+"]")...), '}', '\n')
}

func staticCommands(count int) string {
	var body strings.Builder
	for index := 0; index < count; index++ {
		body.WriteString("t")
		body.WriteString(fmtIndex(index, 4))
		body.WriteByte('\n')
	}
	return body.String()
}

func outputSizedRequest(t *testing.T, target int) Request {
	t.Helper()
	if target > MaxOutputBytes {
		request := outputSizedRequest(t, MaxOutputBytes)
		body, err := base64.StdEncoding.DecodeString(request.Inputs[0].ContentBase64)
		if err != nil || len(body) < 2 {
			t.Fatalf("output edge body=%q err=%v", body, err)
		}
		setInputBody(&request.Inputs[0], string(body[:len(body)-1])+"x\n")
		return request
	}
	request := shellInputCountRequest(t, MaxInputs)
	for index := range request.Inputs {
		request.Inputs[index].Path = shellPath(7, index)
		setInputBody(&request.Inputs[index], "t\n")
	}
	if why := validPreEnvelope(&request); why != "" {
		t.Fatalf("output request admission=%s first=%#v last=%#v", why, request.Inputs[0], request.Inputs[len(request.Inputs)-1])
	}
	baseline := predictedOutputSize(request)
	if response := AnalyzeCanonical(wire(t, request)); len(response) != baseline {
		t.Fatalf("output baseline=%d predicted=%d response=%s", len(response), baseline, response)
	}
	if baseline >= target {
		t.Fatalf("output baseline=%d target=%d", baseline, target)
	}
	remaining := target - baseline
	commandGrowth := remaining % 2
	pathGrowth := (remaining - commandGrowth) / 2
	for index := range request.Inputs {
		capacity := MaxPathBytes - len(request.Inputs[index].Path)
		growth := pathGrowth
		if growth > capacity {
			growth = capacity
		}
		request.Inputs[index].Path = shellPath(len(request.Inputs[index].Path)+growth, index)
		pathGrowth -= growth
	}
	if pathGrowth != 0 {
		t.Fatalf("output path growth=%d", pathGrowth)
	}
	for index := range request.Inputs {
		growth := commandGrowth
		if growth > MaxIdentifierBytes-1 {
			growth = MaxIdentifierBytes - 1
		}
		setInputBody(&request.Inputs[index], "t"+strings.Repeat("x", growth)+"\n")
		commandGrowth -= growth
	}
	if commandGrowth != 0 {
		t.Fatalf("output command growth=%d", commandGrowth)
	}
	if response := AnalyzeCanonical(wire(t, request)); len(response) != target {
		t.Fatalf("output target=%d got=%d predicted=%d response=%q", target, len(response), predictedOutputSize(request), response)
	}
	return request
}

func predictedOutputSize(request Request) int {
	size := emptySuccessSize(request) + 1
	for index, input := range request.Inputs {
		body, _ := base64.StdEncoding.DecodeString(input.ContentBase64)
		command := strings.TrimSuffix(string(body), "\n")
		fact := Fact{Kind: "shell.command.static", InputHandle: input.Handle, RelatedHandle: "-", Subject: input.Path, Predicate: "invokes", Value: command, InstanceID: input.Path}
		fact.EvidenceSHA256 = evidence(request, input, fact)
		if index != 0 {
			size++
		}
		size += factJSONSize(fact)
	}
	return size
}

func setInputBody(input *Input, body string) {
	sum := sha256.Sum256([]byte(body))
	input.ContentBase64 = base64.StdEncoding.EncodeToString([]byte(body))
	input.SHA256 = "sha256:" + hex.EncodeToString(sum[:])
}

func TestEnvelopeAndEvidenceBindingsAreMetamorphic(t *testing.T) {
	base := fixtureWithBody(t, "tool\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
	baseline := mustSuccess(t, AnalyzeCanonical(wire(t, base)))
	if len(baseline.Facts) != 1 {
		t.Fatalf("baseline facts=%#v", baseline.Facts)
	}
	for _, test := range []struct {
		name   string
		mutate func(*Request)
	}{
		{"request_id", func(r *Request) { r.RequestID = "request-2" }},
		{"scope_id", func(r *Request) { r.ScopeID = "scope-2" }},
		{"compilation_unit_id", func(r *Request) { r.CompilationUnitID = "unit-2" }},
		{"target", func(r *Request) {
			r.Target = Target{OS: "linux", Architecture: "amd64", ABI: "none", Features: []string{"bash-5.2"}}
		}},
		{"input_handle", func(r *Request) { r.Inputs[0].Handle = "input-2" }},
		{"input_path", func(r *Request) { r.Inputs[0].Path = "script/other.sh" }},
		{"input_content_and_digest", func(r *Request) {
			body := []byte("other-tool\n")
			sum := sha256.Sum256(body)
			r.Inputs[0].ContentBase64 = base64.StdEncoding.EncodeToString(body)
			r.Inputs[0].SHA256 = "sha256:" + hex.EncodeToString(sum[:])
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := base
			candidate.Inputs = append([]Input(nil), base.Inputs...)
			test.mutate(&candidate)
			got := AnalyzeCanonical(wire(t, candidate))
			value := mustSuccess(t, got)
			if bytes.Equal(got, AnalyzeCanonical(wire(t, base))) || len(value.Facts) != 1 || value.Facts[0].EvidenceSHA256 == baseline.Facts[0].EvidenceSHA256 {
				t.Fatalf("binding %s did not change complete response and evidence: %q", test.name, got)
			}
		})
	}
}

func TestClosedRejectionsCoverEveryReachableReason(t *testing.T) {
	valid := fixtureWithBody(t, "tool\n", Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}})
	for _, test := range []struct {
		name, reason string
		mutate       func(*Request)
		sentinel     bool
	}{
		{"profile", "MALFORMED_INPUT", func(r *Request) { r.Profile = "other" }, true},
		{"family", "UNKNOWN_FAMILY", func(r *Request) { r.Family = "other" }, true},
		{"request_id", "INVALID_IDENTIFIER", func(r *Request) { r.RequestID = "!" }, true},
		{"scope_id", "INVALID_IDENTIFIER", func(r *Request) { r.ScopeID = "!" }, true},
		{"compilation_unit_id", "INVALID_IDENTIFIER", func(r *Request) { r.CompilationUnitID = "!" }, true},
		{"target", "UNSUPPORTED_SCHEMA", func(r *Request) { r.Target.OS = "windows" }, false},
		{"input_handle", "MALFORMED_INPUT", func(r *Request) { r.Inputs[0].Handle = "!" }, true},
		{"input_family", "UNSUPPORTED_SCHEMA", func(r *Request) { r.Inputs[0].Family = "other" }, false},
		{"input_path", "MALFORMED_INPUT", func(r *Request) { r.Inputs[0].Path = "*.sh" }, true},
		{"input_digest_shape", "MALFORMED_INPUT", func(r *Request) { r.Inputs[0].SHA256 = "sha256:bad" }, true},
		{"input_digest", "DIGEST_MISMATCH", func(r *Request) { r.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64) }, false},
		{"input_base64", "MALFORMED_INPUT", func(r *Request) { r.Inputs[0].ContentBase64 = "@@@@" }, false},
		{"source_controls", "MALFORMED_INPUT", func(r *Request) {
			body := []byte("tool\x00\n")
			sum := sha256.Sum256(body)
			r.Inputs[0].ContentBase64 = base64.StdEncoding.EncodeToString(body)
			r.Inputs[0].SHA256 = "sha256:" + hex.EncodeToString(sum[:])
		}, false},
		{"dynamic", "DYNAMIC_INPUT", func(r *Request) {
			body := []byte("eval x\n")
			sum := sha256.Sum256(body)
			r.Inputs[0].ContentBase64 = base64.StdEncoding.EncodeToString(body)
			r.Inputs[0].SHA256 = "sha256:" + hex.EncodeToString(sum[:])
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Inputs = append([]Input(nil), valid.Inputs...)
			test.mutate(&candidate)
			got := AnalyzeCanonical(wire(t, candidate))
			if test.sentinel {
				assertSentinel(t, got, test.reason)
				return
			}
			assertCompleteBoundFailure(t, got, candidate, test.reason)
		})
	}
	assertSentinel(t, AnalyzeCanonical([]byte(" {\n")), "NONCANONICAL_REQUEST")
	output := outputSizedRequest(t, MaxOutputBytes+1)
	assertCompleteBoundFailure(t, AnalyzeCanonical(wire(t, output)), output, "OUTPUT_LIMIT")
}

func mustSuccess(t *testing.T, got []byte) success {
	t.Helper()
	var value success
	if err := json.Unmarshal(got, &value); err != nil || value.Status != "CANDIDATE" {
		t.Fatalf("success=%q err=%v", got, err)
	}
	return value
}

func assertCompleteBoundFailure(t *testing.T, got []byte, want Request, reason string) {
	t.Helper()
	var value failure
	if err := json.Unmarshal(got, &value); err != nil || value.Reason != reason || value.Profile != Profile || value.Family != Family || value.RequestID != want.RequestID || value.ScopeID != want.ScopeID || value.CompilationUnitID != want.CompilationUnitID || value.Target == nil || !reflect.DeepEqual(*value.Target, want.Target) || len(value.InputEchoes) != len(want.Inputs) {
		t.Fatalf("reason=%q response=%q request=%+v err=%v", reason, got, want, err)
	}
	for i, echo := range value.InputEchoes {
		if echo.Handle != want.Inputs[i].Handle || echo.SHA256 != want.Inputs[i].SHA256 {
			t.Fatalf("reason=%q echo=%+v want=%+v", reason, echo, want.Inputs[i])
		}
	}
}
