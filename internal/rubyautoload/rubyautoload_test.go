package rubyautoload

import (
	"reflect"
	"strings"
	"testing"
)

// repository serves files as a Repository; a source's code lines are its raw
// lines, which suffices for the literal configuration below.
func repository(files map[string]string) Repository {
	paths := []string{}
	for name := range files {
		if strings.HasSuffix(name, ".rb") {
			paths = append(paths, name)
		}
	}
	declaration, present := files[DeclarationPath]
	return Repository{Paths: paths, DeclarationPresent: present, DeclarationReadable: present, DeclarationText: declaration,
		Source: func(name string) ([]string, []string, bool) {
			text, ok := files[name]
			lines := strings.Split(text, "\n")
			return lines, lines, ok
		},
		IsTest: func(name string) bool { return strings.HasPrefix(name, "spec/") }}
}

// TestRubyAutoloadConstantMapping: NGI-V0-016, NGI-V0-017, NGI-V0-039.
func TestRubyAutoloadConstantMapping(t *testing.T) {
	files := map[string]string{
		"config/initializers/inflections.rb": "ActiveSupport::Inflector.inflections(:en) do |inflect|\n  inflect.acronym \"API\"\nend\n",
		"config/initializers/zeitwerk.rb":    "Rails.autoloaders.main.inflector.inflect(\"html_parser\" => \"HTMLParser\")\n",
		"spec/support/inflections.rb":        "inflect.acronym \"SPEC\"\n",
	}
	for _, name := range []string{"app/models/thing.rb", "app/models/concerns/taggable.rb", "app/views/x.rb", "app/assets/y.rb",
		"lib/tasks/z.rb", "lib/gem/version.rb", "app/services/billing/invoice.rb", "app/models/api/user_api.rb",
		"app/models/html_parser.rb", "app/models/2fa.rb", "app/models/Thing.rb", "app/models/a-b.rb", "script/x.rb", "spec/models/thing_spec.rb"} {
		files[name] = ""
	}
	mapper := New(repository(files))
	report := mapper.Report()
	if !reflect.DeepEqual(report.Roots, []string{"app/models", "app/models/concerns", "app/services", "lib"}) ||
		!reflect.DeepEqual(report.Ignored, []string{"lib/assets", "lib/tasks"}) || report.RootsSource != RootsDefault || report.Declaration != "" {
		t.Fatalf("default report = %#v", report)
	}
	for name, want := range map[string]string{
		"app/models/thing.rb":             "Thing",
		"app/models/concerns/taggable.rb": "Taggable",
		"app/services/billing/invoice.rb": "Billing::Invoice",
		"lib/gem/version.rb":              "Gem::Version",
		"app/models/api/user_api.rb":      "API::UserAPI",
		"app/models/html_parser.rb":       "HTMLParser",
	} {
		if got, ok := mapper.Constant(name); !ok || got != want {
			t.Fatalf("%s -> %q %v, want %q", name, got, ok, want)
		}
		if back, ok := mapper.Path(want); !ok || back != name {
			t.Fatalf("%s -> %q %v, want %q", want, back, ok, name)
		}
	}
	for _, name := range []string{"app/views/x.rb", "lib/tasks/z.rb", "app/models/2fa.rb", "app/models/Thing.rb", "app/models/a-b.rb", "script/x.rb", "spec/models/thing_spec.rb"} {
		if got, ok := mapper.Constant(name); ok {
			t.Fatalf("%s must be unmapped, got %q", name, got)
		}
	}
	if !mapper.Known("Billing") || !mapper.Known("Billing::Invoice") || mapper.Known("Billing::Charge") || mapper.Known("Invoice") {
		t.Fatal("Known must hold mapped constants and their namespace prefixes only")
	}
	if got, ok := mapper.Camelize("api_client"); !ok || got != "APIClient" {
		t.Fatalf("Camelize(api_client) = %q %v", got, ok)
	}
	// A test path's configuration is not read.
	if _, ok := mapper.Camelize("spec_thing"); !ok {
		t.Fatal("plain camelize must apply")
	}
	if got, _ := mapper.Camelize("spec_thing"); got != "SpecThing" {
		t.Fatalf("a test path's acronym must not apply: %q", got)
	}
	want := []Inflection{
		{Word: "api", Constant: "API", Kind: "acronym", Path: "config/initializers/inflections.rb", Line: 2},
		{Word: "html_parser", Constant: "HTMLParser", Kind: "inflect", Path: "config/initializers/zeitwerk.rb", Line: 1},
	}
	if !reflect.DeepEqual(report.Inflections, want) || mapper.ConfigUnresolved() != 0 {
		t.Fatalf("inflections = %#v, unresolved %d", report.Inflections, mapper.ConfigUnresolved())
	}
}

// TestRubyAutoloadMapperDeclaration: NGI-V0-017, NGI-V0-039. A declared root
// set replaces the defaults; an invalid or unreadable declaration maps nothing.
func TestRubyAutoloadMapperDeclaration(t *testing.T) {
	files := map[string]string{
		DeclarationPath:      `{"schema": "ruby-autoload/0", "roots": ["src"], "inflections": {"oauth": "OAuth"}}`,
		"src/oauth/token.rb": "",
		"app/models/user.rb": "",
	}
	mapper := New(repository(files))
	if got, ok := mapper.Constant("src/oauth/token.rb"); !ok || got != "OAuth::Token" {
		t.Fatalf("declared root = %q %v", got, ok)
	}
	if _, ok := mapper.Constant("app/models/user.rb"); ok {
		t.Fatal("a declaration replaces the default roots")
	}
	if report := mapper.Report(); report.RootsSource != RootsDeclared || report.Declaration != DeclarationPath || len(report.Ignored) != 0 {
		t.Fatalf("declared report = %#v", report)
	}
	for name, body := range map[string]string{
		"unknown member": `{"schema": "ruby-autoload/0", "roots": ["src"], "collapse": []}`,
		"escaping root":  `{"schema": "ruby-autoload/0", "roots": ["../src"]}`,
		"git root":       `{"schema": "ruby-autoload/0", "roots": [".git/hooks"]}`,
		"no roots":       `{"schema": "ruby-autoload/0", "roots": []}`,
		"two documents":  `{"schema": "ruby-autoload/0", "roots": ["src"]} {}`,
		"bad schema":     `{"schema": "ruby-autoload/1", "roots": ["src"]}`,
	} {
		files[DeclarationPath] = body
		invalid := New(repository(files))
		if !invalid.DeclarationInvalid() || invalid.Report().RootsSource != RootsDeclarationInvalid {
			t.Fatalf("%s: declaration must be invalid", name)
		}
		if _, ok := invalid.Constant("src/oauth/token.rb"); ok {
			t.Fatalf("%s: an invalid declaration must map nothing", name)
		}
	}
	unreadable := repository(files)
	unreadable.DeclarationReadable = false
	if !New(unreadable).DeclarationInvalid() {
		t.Fatal("an unreadable declaration is invalid")
	}
}
