package ruby

import (
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestFrontierDeclarationStaleEntryIsReported_V12369 pins RUBY-AFFECTED-V0-960:
// a declared entry that matches no observed site is not silently ignored but
// raised as a plugin-wide ruby:frontier-declaration-stale site naming the
// entry's path and line, so a line shift invalidates it visibly.
func TestFrontierDeclarationStaleEntryIsReported_V12369(t *testing.T) {
	root := writeIssue1519(t)
	write(t, root, FrontierDeclarationPath, `{"schema":"ruby-frontiers/0","resolved":["ruby:conditional-load:app/services/exporter.rb:3","ruby:dynamic-test-definition:app/models/report.rb"]}`)
	if got := plan(t, root, "app/api_clients/foo_client.rb"); got.Scope != affected.ScopeBounded {
		t.Fatalf("matching declaration: scope=%s unknown=%v", got.Scope, got.Unknown)
	}
	for name, row := range map[string]struct{ body, stale string }{
		"line shift":        {`{"schema":"ruby-frontiers/0","resolved":["ruby:conditional-load:app/services/exporter.rb:4"]}`, "app/services/exporter.rb:4"},
		"reason mismatch":   {`{"schema":"ruby-frontiers/0","resolved":["ruby:dynamic-load:app/services/exporter.rb:3"]}`, "app/services/exporter.rb:3"},
		"file without site": {`{"schema":"ruby-frontiers/0","resolved":["ruby:dynamic-test-definition:app/services/exporter.rb"]}`, "app/services/exporter.rb"},
		"deleted file":      {`{"schema":"ruby-frontiers/0","resolved":["ruby:conditional-load:app/gone.rb:2"]}`, "app/gone.rb:2"},
	} {
		write(t, root, FrontierDeclarationPath, row.body)
		got := plan(t, root, "app/api_clients/foo_client.rb")
		if got.Scope != affected.ScopeUnknown || !hasUnknown(got, frontierUnknown(FrontierDeclarationStale+":"+row.stale)) {
			t.Fatalf("%s: scope=%s unknown=%v", name, got.Scope, got.Unknown)
		}
	}
}
