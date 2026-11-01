package ruby

import (
	"maps"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestViewTemplateSelectsRouteDispatchedSpecs_RUBYAFFECTEDV0761 (V1-1831): an
// app/views template is owned, and a change to it selects every
// route-dispatched spec with the advisory ROUTE_DISPATCH_BOOT witness while the
// plan stays BOUNDED; a template outside app/views stays unowned, and a view
// in an application with no route-dispatched test abstains by name.
func TestViewTemplateSelectsRouteDispatchedSpecs_RUBYAFFECTEDV0761(t *testing.T) {
	root := writeRouteDispatch(t)
	write(t, root, "app/views/alpha/show.html.erb", "<h1><%= @alpha.name %></h1>\n")
	write(t, root, "app/views/layouts/application.html.haml", "%html\n")
	write(t, root, "lib/templates/mail.html.erb", "<p></p>\n")
	language := New()
	if !language.Owns("app/views/alpha/show.html.erb") || !language.Owns("app/views/beta/index.html.slim") ||
		language.Owns("lib/templates/mail.html.erb") || language.Owns("engines/x/app/views/a.html.erb") {
		t.Fatal("view ownership is not exactly the root app/views directory")
	}
	for _, dirty := range []string{"app/views/alpha/show.html.erb", "app/views/layouts/application.html.haml"} {
		got := plan(t, root, dirty)
		want := map[string]string{
			"spec/controllers/alpha_controller_spec.rb": "ROUTE_DISPATCH_BOOT",
			"spec/controllers/beta_controller_spec.rb":  "ROUTE_DISPATCH_BOOT",
			"spec/requests/alpha_flow_spec.rb":          "ROUTE_DISPATCH_BOOT",
			"spec/requests/pages_spec.rb":               "ROUTE_DISPATCH_BOOT",
		}
		if kinds := selectedWitnesses(got); got.Scope != affected.ScopeBounded || !maps.Equal(kinds, want) {
			t.Fatalf("%s: scope=%s witnesses=%v unknown=%v", dirty, got.Scope, kinds, got.Unknown)
		}
	}
	outside := plan(t, root, "lib/templates/mail.html.erb")
	if len(outside.Unknown) != 1 || outside.Unknown[0].Reason != affected.UnknownUnownedDirtyPath {
		t.Fatalf("outside template unknown=%v", outside.Unknown)
	}

	library := t.TempDir()
	write(t, library, "lib/widget.rb", "class Widget\nend\n")
	write(t, library, "spec/widget_spec.rb", "require \"widget\"\nRSpec.describe Widget do\n  it { true }\nend\n")
	write(t, library, "app/views/widget/show.html.erb", "<p></p>\n")
	abstained := plan(t, library, "app/views/widget/show.html.erb")
	if len(abstained.Unknown) != 1 || abstained.Unknown[0].Reason != affected.UnknownNoSelectableTest || abstained.Unknown[0].Detail != "ruby:view:app/views/widget/show.html.erb" {
		t.Fatalf("library view unknown=%v", abstained.Unknown)
	}
}
