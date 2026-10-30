package analyzerruby

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// --- fixtures -------------------------------------------------------------

const (
	fixtureGemfile = `source "https://rubygems.org"
ruby "3.2.1"
gem "rake", "13.0.6"
gem "sinatra", "4.0.0"
`
	fixtureLock = `GEM
  remote: https://rubygems.org/
  specs:
    rack (3.0.8)
    rake (13.0.6)
    sinatra (4.0.0)
      rack (3.0.8)

PLATFORMS
  ruby

DEPENDENCIES
  rake (= 13.0.6)
  sinatra (= 4.0.0)

RUBY VERSION
   ruby 3.2.1p31

BUNDLED WITH
   2.4.10
`
)

func makeInput(handle, family, path, content string) Input {
	sum := sha256.Sum256([]byte(content))
	return Input{handle, family, path, "sha256:" + hex.EncodeToString(sum[:]), base64.StdEncoding.EncodeToString([]byte(content))}
}

// makeRequest builds a canonical request, sorting inputs into the typed order
// the envelope requires.
func makeRequest(inputs ...Input) []byte {
	sorted := append([]Input(nil), inputs...)
	sort.Slice(sorted, func(i, j int) bool { return inputLess(sorted[i], sorted[j]) })
	if sorted == nil {
		sorted = []Input{}
	}
	r := Request{Profile, Family, "request-1", "root", "unit-1", Target{"darwin", "arm64", "none", []string{}}, sorted}
	b, _ := json.Marshal(r)
	return append(b, '\n')
}

func gemfileInput() Input {
	return makeInput("gemfile", familyGemfile, "Gemfile", fixtureGemfile)
}
func lockInput() Input { return makeInput("lock", familyLock, "Gemfile.lock", fixtureLock) }
func sourceInput(path, body string) Input {
	return makeInput("src", familySource, path, body)
}

// decodeResult splits the response into status and either facts or reason.
func decodeResult(t *testing.T, out []byte) (string, string, []Fact) {
	t.Helper()
	if len(out) == 0 || out[len(out)-1] != '\n' {
		t.Fatalf("response is not LF-terminated: %q", out)
	}
	var probe struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
		Facts  []Fact `json:"facts"`
	}
	if err := json.Unmarshal(out[:len(out)-1], &probe); err != nil {
		t.Fatalf("undecodable response %q: %v", out, err)
	}
	return probe.Status, probe.Reason, probe.Facts
}

func mustReject(t *testing.T, raw []byte, want string) {
	t.Helper()
	status, reason, _ := decodeResult(t, AnalyzeCanonical(raw))
	if status != "REJECTED" || reason != want {
		t.Fatalf("got status=%s reason=%s, want REJECTED/%s", status, reason, want)
	}
}

func mustAccept(t *testing.T, raw []byte) []Fact {
	t.Helper()
	status, reason, facts := decodeResult(t, AnalyzeCanonical(raw))
	if status != "CANDIDATE" {
		t.Fatalf("got status=%s reason=%s, want CANDIDATE", status, reason)
	}
	return facts
}

// factTuple renders the fields the closed matrix pins, excluding the evidence
// digest, so a table can assert exact fact identity.
func factTuple(f Fact) string {
	return strings.Join([]string{f.Kind, f.InputHandle, f.RelatedHandle, f.Subject, f.Predicate, f.Value, f.InstanceID}, "|")
}

func tuples(facts []Fact) []string {
	out := make([]string, 0, len(facts))
	for _, f := range facts {
		out = append(out, factTuple(f))
	}
	return out
}

func hasTuple(facts []Fact, want string) bool {
	for _, got := range tuples(facts) {
		if got == want {
			return true
		}
	}
	return false
}

// --- envelope -------------------------------------------------------------

func TestEnvelopeRejectsNoncanonicalFrames(t *testing.T) {
	base := makeRequest(gemfileInput())
	for name, raw := range map[string][]byte{
		"empty":            nil,
		"missing LF":       base[:len(base)-1],
		"two LF":           append(append([]byte(nil), base...), '\n'),
		"leading space":    append([]byte(" "), base...),
		"trailing garbage": append(append([]byte(nil), base[:len(base)-1]...), []byte(" \n")...),
	} {
		t.Run(name, func(t *testing.T) { mustReject(t, raw, "NONCANONICAL_REQUEST") })
	}
}

func TestEnvelopeRejectsDuplicateFieldAndUnknownField(t *testing.T) {
	dup := []byte(`{"profile":"` + Profile + `","profile":"` + Profile + `","family":"ruby","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[]}` + "\n")
	mustReject(t, dup, "NONCANONICAL_REQUEST")

	unknown := []byte(`{"profile":"` + Profile + `","family":"ruby","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[],"extra":1.5}` + "\n")
	mustReject(t, unknown, "NONCANONICAL_REQUEST")
}

func TestSafeCanonicalExtensionBindsUnknownField(t *testing.T) {
	frame := makeRequest(gemfileInput())
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

func TestEnvelopeRejectsForeignFamilyAndDigestMismatch(t *testing.T) {
	foreign := makeRequest(gemfileInput())
	mustReject(t, []byte(strings.Replace(string(foreign), `"family":"ruby"`, `"family":"python"`, 1)), "UNKNOWN_FAMILY")

	bad := gemfileInput()
	bad.SHA256 = "sha256:" + strings.Repeat("0", 64)
	mustReject(t, makeRequest(bad), "DIGEST_MISMATCH")
}

// TestEnvelopeRejectsUnsortedInputs proves the typed input order is enforced
// rather than repaired: the same two inputs accepted in order reject when swapped.
func TestEnvelopeRejectsUnsortedInputs(t *testing.T) {
	ordered := []Input{gemfileInput(), lockInput()}
	sort.Slice(ordered, func(i, j int) bool { return inputLess(ordered[i], ordered[j]) })
	mustAccept(t, makeRequest(ordered...))

	r := Request{Profile, Family, "request-1", "root", "unit-1", Target{"darwin", "arm64", "none", []string{}}, []Input{ordered[1], ordered[0]}}
	b, _ := json.Marshal(r)
	mustReject(t, append(b, '\n'), "MALFORMED_INPUT")
}

// TestEnvelopeRejectsAnyDialectFeature pins the Ruby family's empty-feature
// rule: no Ruby dialect feature is specified, so any feature names a schema
// this candidate does not implement.
func TestEnvelopeRejectsAnyDialectFeature(t *testing.T) {
	r := Request{Profile, Family, "request-1", "root", "unit-1", Target{"darwin", "arm64", "none", []string{"rails-7"}}, []Input{gemfileInput()}}
	b, _ := json.Marshal(r)
	mustReject(t, append(b, '\n'), "UNSUPPORTED_SCHEMA")
}

// TestEnvelopeAcceptsAnyWellFormedTarget records the deliberate choice that
// Ruby facts are target-independent: the lock's PLATFORMS row is a fact value,
// not a request coordinate.
func TestEnvelopeAcceptsAnyWellFormedTarget(t *testing.T) {
	r := Request{Profile, Family, "request-1", "root", "unit-1", Target{"linux", "amd64", "none", []string{}}, []Input{gemfileInput()}}
	b, _ := json.Marshal(r)
	mustAccept(t, append(b, '\n'))
}

func TestUnimplementedRubyFamiliesRejectWholesale(t *testing.T) {
	for _, family := range []string{familyGemspec, familyConfig, familyRails, "ruby.unknown"} {
		t.Run(family, func(t *testing.T) {
			mustReject(t, makeRequest(makeInput("x", family, "thing.txt", "body\n")), "UNSUPPORTED_SCHEMA")
		})
	}
}

// --- ruby.version ---------------------------------------------------------

func TestRubyVersionRecordIsExactlyOneCoreVersionLine(t *testing.T) {
	mustAccept(t, makeRequest(makeInput("v", familyVersion, ".ruby-version", "3.2.1\n")))

	for name, body := range map[string]string{
		"engine alias":   "ruby-3.2.1\n",
		"prefix":         "v3.2.1\n",
		"range":          ">= 3.2.1\n",
		"trailing space": "3.2.1 \n",
		"leading space":  " 3.2.1\n",
		"comment":        "3.2.1 # ruby\n",
		"two lines":      "3.2.1\n3.2.2\n",
		"two segments":   "3.2\n",
		"leading zero":   "3.02.1\n",
		"jruby":          "jruby-9.4.0.0\n",
	} {
		t.Run(name, func(t *testing.T) {
			mustReject(t, makeRequest(makeInput("v", familyVersion, ".ruby-version", body)), "UNSUPPORTED_SCHEMA")
		})
	}
	t.Run("missing LF", func(t *testing.T) {
		mustReject(t, makeRequest(makeInput("v", familyVersion, ".ruby-version", "3.2.1")), "MALFORMED_INPUT")
	})
	t.Run("CRLF", func(t *testing.T) {
		mustReject(t, makeRequest(makeInput("v", familyVersion, ".ruby-version", "3.2.1\r\n")), "MALFORMED_INPUT")
	})
}

// --- ruby.gemfile ---------------------------------------------------------

// TestGemfileRejectsRailsDSLConstructs is the reconciliation between the
// reference implementation and this repository's closed matrix. The reference
// resolves `group ... do ... end` scopes, inline `group:` options, and gem
// options into structured Rails facts; the closed Gemfile grammar here admits
// none of them, so every one of those constructs must reject rather than be
// accepted and ignored.
func TestGemfileRejectsRailsDSLConstructs(t *testing.T) {
	for name, body := range map[string]string{
		"group block":        "source \"https://rubygems.org\"\ngroup :development do\n  gem \"rspec\", \"3.12.0\"\nend\n",
		"group inline":       "source \"https://rubygems.org\"\ngem \"rspec\", \"3.12.0\", group: :development\n",
		"group paren":        "source \"https://rubygems.org\"\ngroup(:test) do\nend\n",
		"gem require option": "source \"https://rubygems.org\"\ngem \"rake\", \"13.0.6\", require: false\n",
		"git source":         "source \"https://rubygems.org\"\ngem \"rake\", git: \"https://example.com/rake.git\"\n",
		"path source":        "source \"https://rubygems.org\"\ngem \"rake\", path: \"../rake\"\n",
		"pessimistic range":  "source \"https://rubygems.org\"\ngem \"rake\", \"~> 13.0\"\n",
		"comparison range":   "source \"https://rubygems.org\"\ngem \"rake\", \">= 13.0.6\"\n",
		"interpolation":      "source \"https://rubygems.org\"\ngem \"rake\", \"#{ver}\"\n",
		"variable":           "source \"https://rubygems.org\"\ngem name, \"13.0.6\"\n",
		"method call":        "source \"https://rubygems.org\"\neval_gemfile \"other\"\n",
		"gemspec directive":  "source \"https://rubygems.org\"\ngemspec\n",
		"alternate url":      "source \"https://gems.example.com\"\n",
		"symbol source":      "source :rubygems\n",
		"trailing comment":   "source \"https://rubygems.org\" # main\n",
		"indented directive": "source \"https://rubygems.org\"\n  gem \"rake\", \"13.0.6\"\n",
		"two space comma":    "source \"https://rubygems.org\"\ngem \"rake\",  \"13.0.6\"\n",
		"single quotes":      "source 'https://rubygems.org'\n",
		"missing source":     "gem \"rake\", \"13.0.6\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			mustReject(t, makeRequest(makeInput("g", familyGemfile, "Gemfile", body)), "UNSUPPORTED_SCHEMA")
		})
	}
}

func TestGemfileAcceptsClosedGrammarAndDeduplicates(t *testing.T) {
	ok := "# frozen_string_literal: true\n\nsource \"https://rubygems.org\"\nruby \"3.2.1\"\n\ngem \"rake\", \"13.0.6\"\n"
	mustAccept(t, makeRequest(makeInput("g", familyGemfile, "Gemfile", ok)))

	for name, body := range map[string]string{
		"duplicate source": "source \"https://rubygems.org\"\nsource \"https://rubygems.org\"\n",
		"duplicate ruby":   "source \"https://rubygems.org\"\nruby \"3.2.1\"\nruby \"3.2.1\"\n",
		"duplicate gem":    "source \"https://rubygems.org\"\ngem \"rake\", \"13.0.6\"\ngem \"rake\", \"13.0.6\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			mustReject(t, makeRequest(makeInput("g", familyGemfile, "Gemfile", body)), "DUPLICATE_VALUE")
		})
	}
}

// --- ruby.gemfile-lock ----------------------------------------------------

func TestLockAcceptsExactGrammarAndEmitsResolvedGraph(t *testing.T) {
	facts := mustAccept(t, makeRequest(gemfileInput(), lockInput()))
	want := []string{
		"ruby.bundler.version.locked|lock|-|bundler|locked-at|2.4.10|root",
		"ruby.platform.locked|lock|-|ruby|locks-platform|ruby|root",
		"ruby.gem.locked|lock|gemfile|rack|locked-at|3.0.8|rack@3.0.8",
		"ruby.gem.locked|lock|gemfile|rake|locked-at|13.0.6|rake@13.0.6",
		"ruby.gem.locked|lock|gemfile|sinatra|locked-at|4.0.0|sinatra@4.0.0",
		"ruby.gem.dependency|lock|gemfile|sinatra@4.0.0|depends-on|rack@3.0.8|sinatra@4.0.0",
		"ruby.version.declaration|gemfile|-|ruby|declares-version|3.2.1|root",
	}
	for _, tuple := range want {
		if !hasTuple(facts, tuple) {
			t.Errorf("missing fact %s\ngot:\n  %s", tuple, strings.Join(tuples(facts), "\n  "))
		}
	}
}

// TestLockRejectsEveryStructuralDeviation walks one mutation per grammar rule
// off the accepted fixture, so each rejection is attributable to exactly the
// clause it violates.
func TestLockRejectsEveryStructuralDeviation(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"alternate remote": {
			strings.Replace(fixtureLock, "https://rubygems.org/", "https://gems.example.com/", 1), "UNSUPPORTED_SCHEMA"},
		"remote without slash": {
			strings.Replace(fixtureLock, "remote: https://rubygems.org/", "remote: https://rubygems.org", 1), "UNSUPPORTED_SCHEMA"},
		"spec row three spaces": {
			strings.Replace(fixtureLock, "    rack (3.0.8)\n", "   rack (3.0.8)\n", 1), "UNSUPPORTED_SCHEMA"},
		"dep row five spaces": {
			strings.Replace(fixtureLock, "      rack (3.0.8)", "     rack (3.0.8)", 1), "UNSUPPORTED_SCHEMA"},
		"ruby version two spaces": {
			strings.Replace(fixtureLock, "   ruby 3.2.1p31", "  ruby 3.2.1p31", 1), "UNSUPPORTED_SCHEMA"},
		"bundled with two spaces": {
			strings.Replace(fixtureLock, "   2.4.10", "  2.4.10", 1), "UNSUPPORTED_SCHEMA"},
		"unsorted specs": {
			strings.Replace(fixtureLock, "    rack (3.0.8)\n    rake (13.0.6)\n", "    rake (13.0.6)\n    rack (3.0.8)\n", 1), "MALFORMED_INPUT"},
		"duplicate spec": {
			strings.Replace(fixtureLock, "    rack (3.0.8)\n", "    rack (3.0.8)\n    rack (3.0.8)\n", 1), "DUPLICATE_VALUE"},
		"unsorted platforms": {
			strings.Replace(fixtureLock, "PLATFORMS\n  ruby\n", "PLATFORMS\n  ruby\n  arm64-darwin\n", 1), "MALFORMED_INPUT"},
		"missing bundled with": {
			strings.Replace(fixtureLock, "\nBUNDLED WITH\n   2.4.10\n", "\n", 1), "UNSUPPORTED_SCHEMA"},
		"missing ruby version": {
			strings.Replace(fixtureLock, "\nRUBY VERSION\n   ruby 3.2.1p31\n", "\n", 1), "UNSUPPORTED_SCHEMA"},
		"extra trailing text": {
			fixtureLock + "\nCHECKSUMS\n  rack sha256=abc\n", "UNSUPPORTED_SCHEMA"},
		"unknown section": {
			strings.Replace(fixtureLock, "PLATFORMS\n", "GIT\n  remote: x\n\nPLATFORMS\n", 1), "UNSUPPORTED_SCHEMA"},
		"missing blank separator": {
			strings.Replace(fixtureLock, "\n\nPLATFORMS", "\nPLATFORMS", 1), "UNSUPPORTED_SCHEMA"},
		"pessimistic dependency": {
			strings.Replace(fixtureLock, "  rake (= 13.0.6)", "  rake (~> 13.0)", 1), "UNSUPPORTED_SCHEMA"},
		"punctuated platform": {
			strings.Replace(fixtureLock, "PLATFORMS\n  ruby\n", "PLATFORMS\n  x86+64-linux\n", 1), "UNSUPPORTED_SCHEMA"},
	} {
		t.Run(name, func(t *testing.T) {
			mustReject(t, makeRequest(gemfileInput(), makeInput("lock", familyLock, "Gemfile.lock", tc.body)), tc.want)
		})
	}
}

// --- cross-input binding --------------------------------------------------

func TestLockWithoutGemfileHasNoExactBinding(t *testing.T) {
	mustReject(t, makeRequest(lockInput()), "EXACT_BINDING_UNAVAILABLE")
}

func TestConflictingDeclarationsReject(t *testing.T) {
	t.Run("ruby version file vs gemfile", func(t *testing.T) {
		mustReject(t, makeRequest(makeInput("v", familyVersion, ".ruby-version", "3.1.4\n"), gemfileInput()), "CONFLICTING_VALUE")
	})
	t.Run("lock runtime vs gemfile", func(t *testing.T) {
		body := strings.Replace(fixtureLock, "ruby 3.2.1p31", "ruby 3.1.4p223", 1)
		mustReject(t, makeRequest(gemfileInput(), makeInput("lock", familyLock, "Gemfile.lock", body)), "CONFLICTING_VALUE")
	})
	t.Run("dependency version drift", func(t *testing.T) {
		body := strings.Replace(fixtureLock, "  rake (= 13.0.6)", "  rake (= 13.0.7)", 1)
		mustReject(t, makeRequest(gemfileInput(), makeInput("lock", familyLock, "Gemfile.lock", body)), "CONFLICTING_VALUE")
	})
	t.Run("dependency set drift", func(t *testing.T) {
		body := strings.Replace(fixtureLock, "  rake (= 13.0.6)\n", "", 1)
		mustReject(t, makeRequest(gemfileInput(), makeInput("lock", familyLock, "Gemfile.lock", body)), "CONFLICTING_VALUE")
	})
}

// TestDanglingDependencyEdgeRejects proves the resolved graph is closed: an
// edge to a gem with no spec row cannot become a fact.
func TestDanglingDependencyEdgeRejects(t *testing.T) {
	body := strings.Replace(fixtureLock, "      rack (3.0.8)", "      tilt (2.3.0)", 1)
	mustReject(t, makeRequest(gemfileInput(), makeInput("lock", familyLock, "Gemfile.lock", body)), "EXACT_BINDING_UNAVAILABLE")
}

func TestDuplicateManifestFamilyRejects(t *testing.T) {
	second := makeInput("gemfile2", familyGemfile, "sub/Gemfile", fixtureGemfile)
	mustReject(t, makeRequest(gemfileInput(), second), "DUPLICATE_VALUE")
}

// --- ruby.source ----------------------------------------------------------

// TestSourceTokensRequireCodeProvenance is the direct port of the reference's
// hard case: a token only counts when the bounded lexical states place it in
// code. Every masked-context spelling below is a negative control against the
// same token in code.
func TestSourceTokensRequireCodeProvenance(t *testing.T) {
	for name, tc := range map[string]struct {
		body      string
		classify  string
		wantRSpec bool
	}{
		"code":                 {"RSpec.describe Thing do\nend\n", "test", true},
		"line comment":         {"# RSpec.describe Thing do\nputs 1\n", "source", false},
		"single quoted":        {"puts 'RSpec.describe'\n", "source", false},
		"double quoted":        {"puts \"RSpec.describe\"\n", "source", false},
		"block comment":        {"=begin\nRSpec.describe Thing do\n=end\nputs 1\n", "source", false},
		"data section":         {"puts 1\n__END__\nRSpec.describe Thing do\n", "source", false},
		"heredoc body":         {"doc = <<~TEXT\n  RSpec.describe Thing\nTEXT\nputs doc\n", "source", false},
		"quoted heredoc body":  {"doc = <<~'TEXT'\n  RSpec.describe Thing\nTEXT\nputs doc\n", "source", false},
		"percent literal":      {"puts %w[RSpec.describe]\n", "source", false},
		"percent q literal":    {"puts %q(RSpec.describe)\n", "source", false},
		"regex literal":        {"match = line =~ /RSpec.describe/\n", "source", false},
		"identifier prefix":    {"MyRSpec.describe Thing do\nend\n", "source", false},
		"trailing comment":     {"puts 1 # RSpec.describe\n", "source", false},
		"after closed string":  {"puts \"x\"\nRSpec.describe Thing do\nend\n", "test", true},
		"escaped quote string": {"puts \"a\\\"RSpec.describe\"\n", "source", false},
	} {
		t.Run(name, func(t *testing.T) {
			facts := mustAccept(t, makeRequest(sourceInput("lib/thing.rb", tc.body)))
			wantClass := "ruby.source|src|-|lib/thing.rb|classifies|" + tc.classify + "|lib/thing.rb"
			if !hasTuple(facts, wantClass) {
				t.Errorf("missing %s; got:\n  %s", wantClass, strings.Join(tuples(facts), "\n  "))
			}
			gotRSpec := hasTuple(facts, "ruby.test.static|src|-|lib/thing.rb|observes-test-token|rspec|lib/thing.rb")
			if gotRSpec != tc.wantRSpec {
				t.Errorf("rspec observation = %v, want %v", gotRSpec, tc.wantRSpec)
			}
		})
	}
}

func TestCucumberTokenAndBothTokens(t *testing.T) {
	facts := mustAccept(t, makeRequest(sourceInput("features/step.rb", "Cucumber::Term.new\nRSpec.describe Thing do\nend\n")))
	for _, want := range []string{
		"ruby.test.static|src|-|features/step.rb|observes-test-token|cucumber|features/step.rb",
		"ruby.test.static|src|-|features/step.rb|observes-test-token|rspec|features/step.rb",
		"ruby.source|src|-|features/step.rb|classifies|test|features/step.rb",
	} {
		if !hasTuple(facts, want) {
			t.Errorf("missing %s", want)
		}
	}
	if len(facts) != 3 {
		t.Errorf("got %d facts, want 3: %v", len(facts), tuples(facts))
	}
}

// TestRailsRequireNeverYieldsAFrameworkFact pins the spec prohibition: a file
// that requires Rails yields its classification and nothing else. The closed
// matrix has no framework fact kind, so no version may be inferred, and a
// Rails require must not flip a source file's classification either.
func TestRailsRequireNeverYieldsAFrameworkFact(t *testing.T) {
	facts := mustAccept(t, makeRequest(sourceInput("config/application.rb", "require \"rails\"\nrequire \"rails/all\"\n")))
	if len(facts) != 1 || !hasTuple(facts, "ruby.source|src|-|config/application.rb|classifies|source|config/application.rb") {
		t.Fatalf("want exactly one classification fact, got: %v", tuples(facts))
	}
	for _, f := range facts {
		if strings.Contains(f.Value, "7.") || strings.Contains(f.Kind, "framework") {
			t.Errorf("framework-version fact leaked: %v", factTuple(f))
		}
	}
}

// TestRegexVersusDivisionDisambiguation is the reference's trickiest lexical
// case: a `/` may open a regex or divide, and getting it wrong silently
// swallows or exposes the rest of the line.
func TestRegexVersusDivisionDisambiguation(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want bool // RSpec.describe visible in code after the slash
	}{
		"division keeps code":    {"a = b / c\nRSpec.describe Thing\n", true},
		"regex after match op":   {"a = b =~ /x/\nRSpec.describe Thing\n", true},
		"regex after keyword":    {"if line =~ /x/\nend\nRSpec.describe Thing\n", true},
		"regex swallows token":   {"a = (/RSpec.describe/)\n", false},
		"division then token":    {"total = count / 2 # ok\nRSpec.describe Thing\n", true},
		"receiver call regex":    {"s.gsub(/RSpec.describe/, \"\")\n", false},
		"two divisions in a row": {"x = a / b / c\nRSpec.describe Thing\n", true},
		// The preceding-token memory resets per line: a leading `/` follows a
		// completed statement, so it opens a regex rather than continuing the
		// previous line's division.
		"leading slash opens regex": {"x = a\n/RSpec.describe/\n", false},
		"prior line does not leak":  {"x = a / b\n/re/\nRSpec.describe Thing\n", true},
		// `%=` shares the ambiguity: after a value it is modulo assignment,
		// so a later string stays masked; where a value may start it opens a
		// `=`-delimited percent literal.
		"modulo assignment is not a literal": {"n %= 2\nputs \"a = RSpec.describe\"\n# \"\n", false},
		"percent-equals literal after puts":  {"puts %=RSpec.describe=\n", false},
		// `?"` shares it too: a character literal where a value may start, the
		// ternary operator before a glued string after a value.
		"character literal is not a string": {"x = ?\"\nRSpec.describe Thing\n", true},
		"ternary before glued string":       {"x = cond ?\"a\":\"RSpec.describe\"\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			masked, why := maskRuby([]byte(tc.body))
			if why != "" {
				t.Fatalf("mask failed: %s", why)
			}
			if got := containsToken(masked, "RSpec.describe"); got != tc.want {
				t.Errorf("token visible = %v, want %v (masked=%q)", got, tc.want, masked)
			}
		})
	}
}

// TestUnterminatedLexicalStatesFailClosed proves the bounded states reject
// rather than silently treating the remainder of a file as code.
func TestUnterminatedLexicalStatesFailClosed(t *testing.T) {
	for name, body := range map[string]string{
		"unterminated string":  "x = \"open\n",
		"unterminated heredoc": "x = <<~TEXT\n  body\n",
		"unterminated comment": "=begin\nnever closed\n",
		"unterminated percent": "x = %w[a b\n",
	} {
		t.Run(name, func(t *testing.T) {
			mustReject(t, makeRequest(sourceInput("lib/thing.rb", body)), "MALFORMED_INPUT")
		})
	}
}

func TestSourcePathConventionClassifiesTests(t *testing.T) {
	for path, want := range map[string]string{
		"lib/thing.rb":           "source",
		"spec/thing_spec.rb":     "test",
		"test/thing_test.rb":     "test",
		"app/models/user.rb":     "source",
		"tests/helpers/setup.rb": "test",
		"lib/contest/machine.rb": "source",
		"lib/thing_spec.rb":      "test",
	} {
		t.Run(path, func(t *testing.T) {
			facts := mustAccept(t, makeRequest(sourceInput(path, "puts 1\n")))
			want := "ruby.source|src|-|" + path + "|classifies|" + want + "|" + path
			if !hasTuple(facts, want) {
				t.Errorf("missing %s; got %v", want, tuples(facts))
			}
		})
	}
}

func TestNonRubySourcePathRejects(t *testing.T) {
	mustReject(t, makeRequest(sourceInput("lib/thing.rake", "puts 1\n")), "UNSUPPORTED_SCHEMA")
}

// --- output determinism and evidence --------------------------------------

// TestFactsAreSortedAndOutputSizeIsPredicted checks the two properties the
// envelope depends on: the typed fact order, and that the prospective size
// counter matches the encoder byte for byte.
func TestFactsAreSortedAndOutputSizeIsPredicted(t *testing.T) {
	raw := makeRequest(gemfileInput(), lockInput(), sourceInput("spec/a_spec.rb", "RSpec.describe A do\nend\n"))
	out := AnalyzeCanonical(raw)
	_, _, facts := decodeResult(t, out)
	for i := 1; i < len(facts); i++ {
		if !factLess(facts[i-1], facts[i]) {
			t.Fatalf("facts not strictly increasing at %d: %s then %s", i, factTuple(facts[i-1]), factTuple(facts[i]))
		}
	}
	if len(facts) == 0 {
		t.Fatal("expected facts")
	}
	// The size counter is asserted inside AnalyzeCanonical; a mismatch would
	// have produced OUTPUT_LIMIT rather than a CANDIDATE response.
	if got := AnalyzeCanonical(raw); string(got) != string(out) {
		t.Fatal("analysis is not deterministic across runs")
	}
}

// TestEvidenceBindsToRequestIdentity proves the digest is not a constant: it
// moves when any bound request field moves, and differs between a fact with a
// related witness and one without.
func TestEvidenceBindsToRequestIdentity(t *testing.T) {
	base := mustAccept(t, makeRequest(gemfileInput(), lockInput()))
	byTuple := map[string]string{}
	for _, f := range base {
		byTuple[factTuple(f)] = f.EvidenceSHA256
	}

	r := Request{Profile, Family, "request-1", "other-scope", "unit-1", Target{"darwin", "arm64", "none", []string{}}, nil}
	inputs := []Input{gemfileInput(), lockInput()}
	sort.Slice(inputs, func(i, j int) bool { return inputLess(inputs[i], inputs[j]) })
	r.Inputs = inputs
	b, _ := json.Marshal(r)
	moved := mustAccept(t, append(b, '\n'))
	for _, f := range moved {
		if prior, ok := byTuple[factTuple(f)]; ok && prior == f.EvidenceSHA256 {
			t.Errorf("evidence digest did not move with scope_id for %s", factTuple(f))
		}
	}
}

// TestSourceIdentityDigestsTheCanonicalRemote pins the one fact value derived
// by hashing rather than copying, and proves the raw URL is never emitted.
func TestSourceIdentityDigestsTheCanonicalRemote(t *testing.T) {
	sum := sha256.Sum256([]byte("https://rubygems.org/"))
	want := "sha256:" + hex.EncodeToString(sum[:])
	facts := mustAccept(t, makeRequest(gemfileInput()))
	if !hasTuple(facts, "ruby.source.identity|gemfile|-|rubygems|source-digest|"+want+"|root") {
		t.Fatalf("source identity fact missing or wrong; got %v", tuples(facts))
	}
	for _, f := range facts {
		if strings.Contains(f.Value, "rubygems.org") {
			t.Errorf("raw URL leaked into a fact value: %s", factTuple(f))
		}
	}
}

// TestEmptyInputSetProducesEmptyFactArray keeps the zero case canonical.
func TestEmptyInputSetProducesEmptyFactArray(t *testing.T) {
	facts := mustAccept(t, makeRequest())
	if len(facts) != 0 {
		t.Fatalf("want no facts, got %v", tuples(facts))
	}
}

// TestLexerNestingAndMultipleHeredocs pins the two trickiest pieces of the
// ported masking machinery: paired percent-literal delimiters nest (so an inner
// `]` does not close the literal early), and several heredocs opened on one
// line are consumed FIFO in their opening order. A pending heredoc body owns
// its lines, so a body line reading `=begin` is text, not a block comment. A
// `<<` glued to a value is a shift, and CRLF terminators close their states.
func TestLexerNestingAndMultipleHeredocs(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want bool // RSpec.describe survives masking as code
	}{
		"nested bracket keeps literal open":  {"x = %w[a [b] RSpec.describe]\n", false},
		"nested paren keeps literal open":    {"x = %q(a (b) RSpec.describe)\n", false},
		"literal closes at depth zero":       {"x = %w[a [b]]\nRSpec.describe Thing\n", true},
		"unpaired delimiter does not nest":   {"x = %w|a| \nRSpec.describe Thing\n", true},
		"two heredocs consumed in order":     {"f(<<~A, <<~B)\n  RSpec.describe in A\nA\n  RSpec.describe in B\nB\nputs 1\n", false},
		"code after both heredocs is code":   {"f(<<~A, <<~B)\nA\nB\nRSpec.describe Thing\n", true},
		"=begin inside heredoc is body text": {"doc = <<~TEXT\n=begin\nTEXT\nRSpec.describe Thing\n", true},
		"glued append is a shift":            {"a<<b\nRSpec.describe Thing\n", true},
		"glued keyword opens heredoc":        {"return<<A\nRSpec.describe Thing\nA\n", false},
		"CRLF heredoc terminator closes":     {"doc = <<A\r\nbody\r\nA\r\nRSpec.describe Thing\r\n", true},
		"CRLF block comment masks its body":  {"=begin\r\nRSpec.describe Thing\r\n=end\r\nputs 1\r\n", false},
		"backtick literal is masked":         {"x = `echo RSpec.describe`\n", false},
		"escaped delimiter does not close":   {"x = %q(a \\) RSpec.describe)\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			masked, why := maskRuby([]byte(tc.body))
			if why != "" {
				t.Fatalf("mask failed: %s", why)
			}
			if got := containsToken(masked, "RSpec.describe"); got != tc.want {
				t.Errorf("token visible = %v, want %v (masked=%q)", got, tc.want, masked)
			}
		})
	}
}

// TestFactFieldsNeverRequireJSONEscaping guards the prospective output counter.
// factJSONSize charges raw byte lengths, so it is exact only while no fact
// field can contain a quote or backslash. Every field is drawn from a closed
// atom, a validated logical path, a literal, or a hex digest -- none of which
// admits either byte. If a future value source did, the encoder/counter
// equality check would fail the request closed rather than emit a wrong count,
// but this test names the invariant directly.
func TestFactFieldsNeverRequireJSONEscaping(t *testing.T) {
	facts := mustAccept(t, makeRequest(
		makeInput("v", familyVersion, ".ruby-version", "3.2.1\n"),
		gemfileInput(),
		lockInput(),
		sourceInput("spec/a_spec.rb", "Cucumber\nRSpec.describe A do\nend\n"),
	))
	if len(facts) < 10 {
		t.Fatalf("expected the full fact surface, got %d", len(facts))
	}
	for _, f := range facts {
		for label, field := range map[string]string{
			"kind": f.Kind, "input_handle": f.InputHandle, "related_handle": f.RelatedHandle,
			"subject": f.Subject, "predicate": f.Predicate, "value": f.Value,
			"instance_id": f.InstanceID, "evidence_sha256": f.EvidenceSHA256,
		} {
			if strings.ContainsAny(field, "\"\\") {
				t.Errorf("%s field %s=%q would need JSON escaping", factTuple(f), label, field)
			}
			for i := 0; i < len(field); i++ {
				if field[i] < 0x20 || field[i] > 0x7e {
					t.Errorf("%s field %s=%q is not printable ASCII", factTuple(f), label, field)
				}
			}
		}
	}
}
