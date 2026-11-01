package appmap

import (
	"strings"
	"testing"
)

// AMAP-V0-813: a bundler `alias` key may map an external-looking package name onto repository code,
// so a relay import through an aliased name is a possible holder of the table (the name stays
// UNKNOWN), an alias the reader cannot list makes every package a possible holder, and a package no
// alias names stays external.
func TestAMAPV0813BundlerAliasIsNotExternal(t *testing.T) {
	const decl, index = "app/tables/routes/routes.constants.ts", "app/tables/routes/index.ts"
	table := "export const SectionTable = { REPORTS: 'ledger' };\n"
	star := "export * from './routes.constants';\n"
	reg := strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1) + "import R from '../tables/routes/relay';\nR.REPORTS = 'other';\n"
	relay := "import { SectionTable as T } from 'tables/routes.constants';\nexport default (T);\n"
	for name, c := range map[string]struct {
		config, text string
		resolved     bool
	}{
		"vite object alias":    {"vite.config.ts", "export default { resolve: { alias: { 'tables': '/app/tables/routes' } } };\n", false},
		"webpack exact alias":  {"webpack.config.js", "module.exports = { resolve: { alias: { 'tables$': './app/tables/routes', other: './x' } } };\n", false},
		"vite array alias":     {"vite.config.mts", "export default { resolve: { alias: [{ find: /^~/, replacement: '/app' }] } };\n", false},
		"computed alias":       {"config/webpack.dev.js", "const a = require('./aliases');\nmodule.exports = { resolve: { alias: a } };\n", false},
		"assigned alias":       {"webpack.config.js", "const c = {};\nc.resolve.alias = { 'tables': './app/tables/routes' };\nmodule.exports = c;\n", false},
		"unrelated alias only": {"vite.config.ts", "export default { resolve: { alias: { 'other-lib': '/app/other' } } };\n", true},
		"no alias":             {"vite.config.ts", "export default { server: { port: 3000 } };\n", true},
	} {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{diRegAt: reg, index: star, decl: table, "app/tables/routes/relay.ts": relay, c.config: c.text,
				"package.json": `{"private":true,"dependencies":{"tables":"1.0.0"}}`}
			_, _, m, err := injectRepo(t, `["app/setup"]`, diRouter, files)
			if err != nil {
				t.Fatal(err)
			}
			if got := diResolved(t, m); got != c.resolved {
				t.Fatalf("resolved = %v, want %v: %+v", got, c.resolved, m.Unknowns)
			}
			if !c.resolved {
				got := diUnknown(m)
				if len(got) != 1 || got[0].Reason != "not-read-whole" {
					t.Fatalf("diagnostic %+v", got)
				}
			}
		})
	}
}
