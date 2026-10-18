package contextindex

import (
	"context"
	"reflect"
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
		Sources: map[string]Source{"app/a.rb": {}, "app/b.rb": {}},
		Tracked: map[string]struct{}{"Gemfile": {}, "spec/a_spec.rb": {}, "app/a.rb": {}, "app/b.rb": {}},
	}
	got := verification(index, verificationEvidence("app/a.rb", "app/b.rb", "spec/a_spec.rb"))
	if !reflect.DeepEqual(got, []any{"bundle exec rspec", "git diff --check"}) {
		t.Fatalf("verification = %v", got)
	}
}
