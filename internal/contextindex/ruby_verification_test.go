package contextindex

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func rubyVerificationFiles(extra map[string]string) map[string]string {
	files := map[string]string{
		"app/api_clients/foo_client.rb":         "class FooClient\n  def fetch = :ok\nend\n",
		"spec/api_clients/foo_client_spec.rb":   "RSpec.describe FooClient do\n  it { expect(FooClient.new.fetch).to eq(:ok) }\nend\n",
		"spec/support/shared_examples/fetch.rb": "RSpec.shared_examples \"fetches\" do\nend\n",
	}
	for name, body := range extra {
		files[name] = body
	}
	return files
}

func impactVerificationOf(t *testing.T, files map[string]string, changed string) ([]any, []any) {
	t.Helper()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{changed}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	coverage := receipt["coverage"].(map[string]any)
	return anySlice(receipt["verification"]), anySlice(coverage["uncertainty"])
}

// TestNGIV0027RubyImpactNamesBundleExecRSpec: NGI-V0-027. A built index of a
// repository tracking a root Gemfile and a spec/ directory (or a root .rspec)
// names `bundle exec rspec` for a changed Ruby path and drops the
// unknown-language line; without a Gemfile it offers no command and keeps it.
func TestNGIV0027RubyImpactNamesBundleExecRSpec(t *testing.T) {
	t.Parallel()
	gemfile := "source \"https://rubygems.org\"\ngem \"rspec\"\n"
	for _, test := range []struct {
		name         string
		files        map[string]string
		wantCommands []any
		wantUnknown  bool
	}{
		{
			name:         "Gemfile and spec directory",
			files:        rubyVerificationFiles(map[string]string{"Gemfile": gemfile}),
			wantCommands: []any{"bundle exec rspec", "git diff --check"},
		},
		{
			name: "Gemfile and .rspec without a spec directory",
			files: map[string]string{
				"Gemfile":                       gemfile,
				".rspec":                        "--require spec_helper\n",
				"app/api_clients/foo_client.rb": "class FooClient\nend\n",
				"test/foo_client_spec.rb":       "RSpec.describe FooClient do\nend\n",
			},
			wantCommands: []any{"bundle exec rspec", "git diff --check"},
		},
		{
			name:         "spec directory without a Gemfile",
			files:        rubyVerificationFiles(nil),
			wantCommands: []any{"git diff --check"},
			wantUnknown:  true,
		},
		{
			name:         ".rspec without a Gemfile",
			files:        rubyVerificationFiles(map[string]string{".rspec": "--require spec_helper\n"}),
			wantCommands: []any{"git diff --check"},
			wantUnknown:  true,
		},
		{
			name: "Gemfile without spec directory or .rspec",
			files: map[string]string{
				"Gemfile":                       gemfile,
				"app/api_clients/foo_client.rb": "class FooClient\nend\n",
				"test/foo_client_test.rb":       "require \"minitest/autorun\"\n",
			},
			wantCommands: []any{"git diff --check"},
			wantUnknown:  true,
		},
		{
			name:         "nested Gemfile is not the repository root's",
			files:        rubyVerificationFiles(map[string]string{"vendor/engine/Gemfile": gemfile}),
			wantCommands: []any{"git diff --check"},
			wantUnknown:  true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			commands, uncertainty := impactVerificationOf(t, test.files, "app/api_clients/foo_client.rb")
			if !reflect.DeepEqual(commands, test.wantCommands) {
				t.Fatalf("verification = %v, want %v", commands, test.wantCommands)
			}
			if got := anyContains(uncertainty, unknownLanguageVerification); got != test.wantUnknown {
				t.Fatalf("uncertainty = %v, want unknown-language line %v", uncertainty, test.wantUnknown)
			}
		})
	}
}

// TestNGIV0027GoAndWebPlansUnchangedByRubySignals: NGI-V0-027. A tracked
// Gemfile and spec/ directory change nothing about a Go or web path's plan.
func TestNGIV0027GoAndWebPlansUnchangedByRubySignals(t *testing.T) {
	tracked := map[string]struct{}{"Gemfile": {}, ".rspec": {}, "spec/a_spec.rb": {}}
	for _, test := range []struct {
		name     string
		sources  map[string]Source
		path     string
		expected []string
	}{
		{
			name:     "Go package",
			sources:  map[string]Source{"go.mod": {Data: []byte("module example.test\n\ngo 1.24\n")}},
			path:     "internal/widget/widget.go",
			expected: []string{"go test ./internal/widget/...", "git diff --check"},
		},
		{
			name:     "web package with a test script",
			sources:  map[string]Source{"package.json": {Data: []byte(`{"scripts":{"test":"jest"}}`)}},
			path:     "lib/widget.js",
			expected: []string{"npm test", "git diff --check"},
		},
		{
			name: "Makefile gate",
			sources: map[string]Source{
				"go.mod":   {Data: []byte("module example.test\n\ngo 1.24\n")},
				"Makefile": {Data: []byte("gate: build\n\t@true\nbuild:\n\t@true\n")},
			},
			path:     "main.go",
			expected: []string{"go test ./...", "make gate"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			without := verification(&Index{Sources: test.sources}, verificationEvidence(test.path))
			with := verification(&Index{Sources: test.sources, Tracked: tracked}, verificationEvidence(test.path))
			if !reflect.DeepEqual(without, with) {
				t.Fatalf("Ruby signals changed the plan: without=%v with=%v", without, with)
			}
			got := make([]string, len(with))
			for i, value := range with {
				got[i] = value.(string)
			}
			if !reflect.DeepEqual(got, test.expected) {
				t.Fatalf("verification = %v, want %v", got, test.expected)
			}
		})
	}
}

// TestNGIV0027RubyCommandAppearsOnceAcrossPaths: NGI-V0-027. Several changed
// Ruby paths share one repository-wide command.
func TestNGIV0027RubyCommandAppearsOnceAcrossPaths(t *testing.T) {
	index := &Index{
		Sources: map[string]Source{"app/a.rb": {}, "app/b.rb": {}, "Gemfile": {Data: []byte("gem 'rspec'\n")}},
		Tracked: map[string]struct{}{"Gemfile": {}, "spec/a_spec.rb": {}, "app/a.rb": {}, "app/b.rb": {}},
	}
	got := verification(index, verificationEvidence("app/a.rb", "app/b.rb", "spec/a_spec.rb"))
	if !reflect.DeepEqual(got, []any{"bundle exec rspec", "git diff --check"}) {
		t.Fatalf("verification = %v", got)
	}
}

// TestNGIV0027RubySignalKeepsOtherReceiptsUnknownLine: NGI-V0-027. An RSpec
// project's Gemfile and spec/ do not answer for a receipt without Ruby
// evidence: a JS path with no test script keeps the full receipt's
// `git diff --check` plan and the unknown-language line, as it did before.
func TestNGIV0027RubySignalKeepsOtherReceiptsUnknownLine(t *testing.T) {
	t.Parallel()
	files := rubyVerificationFiles(map[string]string{
		"Gemfile":       "source \"https://rubygems.org\"\ngem \"rspec\"\n",
		"lib/widget.js": "export const widget = 1;\n",
	})
	commands, uncertainty := impactVerificationOf(t, files, "lib/widget.js")
	if !reflect.DeepEqual(commands, []any{"git diff --check"}) {
		t.Fatalf("verification = %v", commands)
	}
	if !anyContains(uncertainty, unknownLanguageVerification) {
		t.Fatalf("a non-Ruby receipt must keep the unknown-language line: %v", uncertainty)
	}

	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	withRuby, err := Impact(index, []string{"lib/widget.js"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	index.Tracked = nil
	delete(index.Sources, "Gemfile")
	withoutRuby, err := Impact(index, []string{"lib/widget.js"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(withRuby["verification"], withoutRuby["verification"]) ||
		!reflect.DeepEqual(withRuby["coverage"].(map[string]any)["uncertainty"], withoutRuby["coverage"].(map[string]any)["uncertainty"]) {
		t.Fatalf("Ruby signals changed a JS receipt: with=%v without=%v", withRuby["coverage"], withoutRuby["coverage"])
	}
}

// TestNGIV0052RSpecNeedsDeclaredGem: NGI-V0-052. RSpec is named only when the
// root Gemfile or Gemfile.lock declares an rspec gem; a Minitest repository
// that also tracks spec/ keeps the unknown-language line.
func TestNGIV0052RSpecNeedsDeclaredGem(t *testing.T) {
	t.Parallel()
	rspecCommands := []any{"bundle exec rspec", "git diff --check"}
	unknownCommands := []any{"git diff --check"}
	for _, test := range []struct {
		name         string
		extra        map[string]string
		wantCommands []any
	}{
		{"Minitest Gemfile with a spec directory", map[string]string{
			"Gemfile": "source \"https://rubygems.org\"\ngem \"minitest\"\n"}, unknownCommands},
		{"Minitest Gemfile with .rspec", map[string]string{
			"Gemfile": "gem 'minitest'\n", ".rspec": "--require spec_helper\n"}, unknownCommands},
		{"commented rspec gem", map[string]string{
			"Gemfile": "# gem \"rspec\"\ngem \"minitest\"\n"}, unknownCommands},
		{"gem name only prefixed by rspec", map[string]string{
			"Gemfile": "gem \"rspec_junit_formatter\"\n"}, unknownCommands},
		{"rspec-rails in the Gemfile", map[string]string{
			"Gemfile": "group :test do\n  gem 'rspec-rails', '~> 6.0'\nend\n"}, rspecCommands},
		{"rspec-core only in Gemfile.lock", map[string]string{
			"Gemfile":      "source \"https://rubygems.org\"\ngemspec\n",
			"Gemfile.lock": "GEM\n  remote: https://rubygems.org/\n  specs:\n    rspec-core (3.13.0)\n      rspec-support (~> 3.13.0)\n\nPLATFORMS\n  ruby\n\nDEPENDENCIES\n  foo!\n"}, rspecCommands},
		{"rspec only as a transitive lockfile requirement", map[string]string{
			"Gemfile":      "gemspec\n",
			"Gemfile.lock": "GEM\n  specs:\n    foo (1.0)\n      rspec (>= 3)\n"}, unknownCommands},
		{"oversized Gemfile is not read", map[string]string{
			"Gemfile": "gem \"rspec\"\n" + strings.Repeat("# padding\n", rubyManifestBound.limit/10+1)}, unknownCommands},
		// TSA-V0-461: the bound removes only the oversized Gemfile's evidence; the readable
		// lockfile still declares rspec-core, so the command stands on it alone.
		{"oversized Gemfile with a readable Gemfile.lock", map[string]string{
			"Gemfile":      "gem \"minitest\"\n" + strings.Repeat("# padding\n", rubyManifestBound.limit/10+1),
			"Gemfile.lock": "GEM\n  specs:\n    rspec-core (3.13.0)\n\nDEPENDENCIES\n  rspec-core\n"}, rspecCommands},
		{"oversized Gemfile.lock with a Minitest Gemfile", map[string]string{
			"Gemfile":      "gem \"minitest\"\n",
			"Gemfile.lock": "GEM\n  specs:\n    rspec-core (3.13.0)\n" + strings.Repeat("# padding\n", rubyManifestBound.limit/10+1)}, unknownCommands},
	} {
		t.Run(test.name, func(t *testing.T) {
			commands, uncertainty := impactVerificationOf(t, rubyVerificationFiles(test.extra), "app/api_clients/foo_client.rb")
			if !reflect.DeepEqual(commands, test.wantCommands) {
				t.Fatalf("verification = %v, want %v", commands, test.wantCommands)
			}
			wantUnknown := len(test.wantCommands) == 1
			if got := anyContains(uncertainty, unknownLanguageVerification); got != wantUnknown {
				t.Fatalf("uncertainty = %v, want unknown-language line %v", uncertainty, wantUnknown)
			}
		})
	}
}

// TestNGIV0053MinitestCommands: NGI-V0-053. A Minitest repository gets
// `bin/rails test` or `bundle exec rake test` only when that runner is proved,
// and otherwise keeps the unknown-language line.
func TestNGIV0053MinitestCommands(t *testing.T) {
	t.Parallel()
	model := "class Widget\nend\n"
	minitest := "require \"minitest/autorun\"\nclass WidgetTest < Minitest::Test\nend\n"
	for _, test := range []struct {
		name         string
		files        map[string]string
		wantCommands []any
	}{
		{"Rails application", map[string]string{
			"Gemfile": "gem \"rails\", \"~> 7.1\"\n", "bin/rails": "#!/usr/bin/env ruby\n",
			"app/models/widget.rb": model, "test/models/widget_test.rb": minitest,
		}, []any{"bin/rails test", "git diff --check"}},
		{"Rails gem without bin/rails", map[string]string{
			"Gemfile":              "gem \"rails\"\n",
			"app/models/widget.rb": model, "test/models/widget_test.rb": minitest,
		}, []any{"git diff --check"}},
		{"Rake test task", map[string]string{
			"Gemfile":              "gem \"minitest\"\ngem \"rake\"\n",
			"Rakefile":             "require \"rake/testtask\"\nRake::TestTask.new(:test) do |t|\n  t.libs << \"test\"\nend\n",
			"app/models/widget.rb": model, "test/widget_test.rb": minitest,
		}, []any{"bundle exec rake test", "git diff --check"}},
		{"Minitest without a Rakefile test task", map[string]string{
			"Gemfile":              "gem \"minitest\"\n",
			"Rakefile":             "task :default do\nend\n",
			"app/models/widget.rb": model, "test/widget_test.rb": minitest,
		}, []any{"git diff --check"}},
		{"RSpec wins over a Rails test directory", map[string]string{
			"Gemfile": "gem \"rails\"\ngem \"rspec-rails\"\n", "bin/rails": "#!/usr/bin/env ruby\n",
			"app/models/widget.rb": model, "test/models/widget_test.rb": minitest, "spec/models/widget_spec.rb": "RSpec.describe Widget do\nend\n",
		}, []any{"bundle exec rspec", "git diff --check"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			commands, uncertainty := impactVerificationOf(t, test.files, "app/models/widget.rb")
			if !reflect.DeepEqual(commands, test.wantCommands) {
				t.Fatalf("verification = %v, want %v", commands, test.wantCommands)
			}
			wantUnknown := len(test.wantCommands) == 1
			if got := anyContains(uncertainty, unknownLanguageVerification); got != wantUnknown {
				t.Fatalf("uncertainty = %v, want unknown-language line %v", uncertainty, wantUnknown)
			}
		})
	}
}

// rangeVerificationOf commits change over files and returns the committed
// range receipt's verification plan.
func rangeVerificationOf(t *testing.T, files map[string]string, changed, replacement string) []any {
	t.Helper()
	root := impactRepositoryWithFiles(t, files)
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, changed, replacement)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "changed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ranged, err := RangeImpact(context.Background(), index, base, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	return anySlice(ranged["verification"])
}

// TestNGIV0051RangeImpactNamesRubyCommand: NGI-V0-051. A committed range with a
// Ruby path names the repository's Ruby command before the gate; a Minitest
// repository's range names no RSpec command.
func TestNGIV0051RangeImpactNamesRubyCommand(t *testing.T) {
	t.Parallel()
	const changed = "app/api_clients/foo_client.rb"
	edited := "class FooClient\n  def fetch = :changed\nend\n"
	got := rangeVerificationOf(t, rubyIssue1130Files(t), "app/api_clients/foo_client.rb",
		strings.Replace(rubyIssue1130Files(t)[changed], "ok: true", "ok: false", 1))
	if !reflect.DeepEqual(got, []any{"bundle exec rspec", "git diff --check"}) {
		t.Fatalf("issue #1130 range verification = %v", got)
	}
	got = rangeVerificationOf(t, rubyVerificationFiles(map[string]string{"Gemfile": "gem \"rspec\"\n"}), changed, edited)
	if !reflect.DeepEqual(got, []any{"bundle exec rspec", "git diff --check"}) {
		t.Fatalf("RSpec range verification = %v", got)
	}
	got = rangeVerificationOf(t, rubyVerificationFiles(map[string]string{"Gemfile": "gem \"minitest\"\n"}), changed, edited)
	if !reflect.DeepEqual(got, []any{"git diff --check"}) {
		t.Fatalf("Minitest range verification = %v", got)
	}
}

// TestNGIV0051GoRangePlanUnchangedByRubySignals: NGI-V0-051. A range with no
// Ruby path keeps its Go-only plan exactly in an RSpec repository.
func TestNGIV0051GoRangePlanUnchangedByRubySignals(t *testing.T) {
	t.Parallel()
	goFiles := map[string]string{
		"go.mod":                    "module example.test/widget\n\ngo 1.24\n",
		"internal/widget/widget.go": "package widget\n\nfunc Widget() int { return 1 }\n",
	}
	withRuby := rubyVerificationFiles(map[string]string{"Gemfile": "gem \"rspec\"\n", ".rspec": "--require spec_helper\n"})
	for name, body := range goFiles {
		withRuby[name] = body
	}
	edited := "package widget\n\nfunc Widget() int { return 2 }\n"
	plain := rangeVerificationOf(t, goFiles, "internal/widget/widget.go", edited)
	ruby := rangeVerificationOf(t, withRuby, "internal/widget/widget.go", edited)
	plainBytes, _ := CanonicalJSON(plain)
	rubyBytes, _ := CanonicalJSON(ruby)
	if string(plainBytes) != string(rubyBytes) || !reflect.DeepEqual(plain, []any{"go test ./internal/widget/...", "git diff --check"}) {
		t.Fatalf("Go range plan changed by Ruby signals: plain=%s ruby=%s", plainBytes, rubyBytes)
	}
}
