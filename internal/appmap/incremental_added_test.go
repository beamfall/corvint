package appmap

import (
	"context"
	"strings"
	"testing"
)

// AMAP-V0-368 (V1-2966, #1952): a path added outside every manifest input, router glob and the
// test root, with a suffix no web import probes and a base name no kept test import names, is
// re-derived like a source edit instead of rebuilding in full, and the map is the full build's.
// An added test source, web module, JSON file or named path, and any added path while a bare
// test import is unresolved, still rebuild in full.
func TestAMAPV0368AddedPathRederives(t *testing.T) {
	ctx := context.Background()
	root, commits := incrementalHistory(t, 0, 0)
	gitTest(t, root, "rm", "-q", "e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts")
	base := commitAll(t, root, "no bare imports")
	_ = commits
	prev := build(t, root, base)
	for _, c := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"ruby spec and service", map[string]string{"spec/models/item_spec.rb": "describe Item do\nend\n", "app/services/thing.rb": "class Thing\nend\n"}, ""},
		{"asset", map[string]string{"assets/new.png": "png"}, ""},
		{"test root source", map[string]string{"e2e/specs/more.spec.ts": "import { test } from '@playwright/test';\n"}, RebuildChangedInput},
		{"web module", map[string]string{"app/lib/extra.js": "export const x = 1;\n"}, RebuildChangedInput},
		{"json", map[string]string{"spec/data.json": "{}\n"}, RebuildChangedInput},
		{"named by a test import", map[string]string{"app/lib/format": "x\n"}, RebuildChangedInput},
		{"config shaped", map[string]string{"server/conf.rb": "{\"compilerOptions\": {}}\n"}, RebuildConfigShaped},
	} {
		for rel, content := range c.files {
			writeFile(t, root, rel, content)
		}
		head := commitAll(t, root, c.name)
		m, inc, err := BuildFrom(ctx, root, "appmap.json", head, Roots{}, prev)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if inc.Reason != c.want || inc.Reused || (c.want == "") != (strings.Join(inc.Rederived, ",") == "routers,test-joins,flows") {
			t.Errorf("%s: %+v, want reason %q", c.name, inc, c.want)
		}
		if m.Digest != build(t, root, head).Digest {
			t.Errorf("%s: digest differs from a full build", c.name)
		}
		gitTest(t, root, "reset", "-q", "--hard", base)
	}
	// A specifier with a web suffix probes its target without it (`theme.css.js` -> `theme.css`).
	writeFile(t, root, "app/lib/theme.css.ts", "export const t = 1;\n")
	writeFile(t, root, "e2e/specs/theme.spec.ts", "import { test } from '@playwright/test';\nimport { t } from '../../app/lib/theme.css.js';\n")
	themed := commitAll(t, root, "theme")
	themePrev := build(t, root, themed)
	writeFile(t, root, "app/lib/theme.css", "a {}\n")
	head := commitAll(t, root, "theme css")
	if m, inc, err := BuildFrom(ctx, root, "appmap.json", head, Roots{}, themePrev); err != nil || inc.Reason != RebuildChangedInput || m.Digest != build(t, root, head).Digest {
		t.Fatalf("suffixed specifier: %+v %v", inc, err)
	}
	gitTest(t, root, "reset", "-q", "--hard", base)
	// A bare test import the map left unresolved may resolve through a config to any path.
	writeFile(t, root, "spec/models/item_spec.rb", "describe Item do\nend\n")
	head = commitAll(t, root, "with bare import")
	if _, inc, err := BuildFrom(ctx, root, "appmap.json", head, Roots{}, build(t, root, commits[0])); err != nil || inc.Reason != RebuildChangedInput {
		t.Fatalf("bare unresolved import: %+v %v", inc, err)
	}
}
