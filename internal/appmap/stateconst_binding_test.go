package appmap

import (
	"fmt"
	"strings"
	"testing"
)

// AMAP-V0-196: the reported 200-member enum resolves with either the registering file alone
// or that file plus its routes directory in di_constants. Every state retains the barrel hop.
func TestAMAPV0196StarBarrelReportedScopes(t *testing.T) {
	const decl, index = "app/setup/routes/routes.constants.ts", "app/setup/routes/index.ts"
	var enum, router strings.Builder
	enum.WriteString("export enum RoutesConstants {\n")
	router.WriteString("angular.module('admin').config(function ($stateProvider, StateConstants) {\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&enum, "  STATE_%d = 'app.reported%d',\n", i, i)
		fmt.Fprintf(&router, "  $stateProvider.state(StateConstants.STATE_%d, { url: 'reported%d' });\n", i, i)
	}
	enum.WriteString("}\n")
	router.WriteString("});\n")
	files := map[string]string{
		diRegAt: "import { RoutesConstants } from './routes';\nangular.module('admin').constant('StateConstants', RoutesConstants);\n",
		index:   "export * from './routes.constants';\n",
		decl:    enum.String(),
	}
	for _, scope := range []string{`["app/setup/setup.module.ts"]`, `["app/setup/setup.module.ts", "app/setup/routes"]`} {
		t.Run(scope, func(t *testing.T) {
			_, _, m, err := injectRepo(t, scope, router.String(), files)
			if err != nil {
				t.Fatal(err)
			}
			if nameUnknownIn(m, constRouter) || len(diUnknown(m)) != 0 {
				t.Fatalf("reported scope did not resolve: %+v", m.Unknowns)
			}
			for i := 0; i < 200; i++ {
				s := screenByID(t, m, fmt.Sprintf("app.reported%d", i))
				if s.Status != StatusResolved || s.Template != fmt.Sprintf("/reported%d", i) {
					t.Fatalf("state %d: %+v", i, s)
				}
				if a := s.NameFrom; len(a) != 5 || a[0].Path != decl || a[0].Start != i+2 || a[1].Path != index || a[1].Start != 1 {
					t.Fatalf("state %d lost declaration/barrel evidence: %+v", i, a)
				}
			}
		})
	}
}

// AMAP-V0-196 (V1-2699): the reported registering module -- `.constant('X', Enum)` beside other
// registrations, with the enum behind an `export * from` directory barrel -- resolves; when a
// binding on the chain blocks the read, the not-read-whole unknown names the hop and that binding
// as `path:line:name:why` (AMAP-V0-203: why the binding may hold the table).
func TestAMAPV0196StarBarrelBindingDetail(t *testing.T) {
	const decl, index = "app/setup/routes/routes.constants.ts", "app/setup/routes/index.ts"
	enum := "export enum SectionTable { REPORTS = 'ledger', HOME = 'home' }\n"
	star := "export * from './routes.constants';\n"
	reg := "import { HomeCtrl } from './home.controller';\nimport { SectionTable } from './routes';\n" +
		"angular.module('core')\n  .controller('HomeCtrl', HomeCtrl)\n  .constant('SectionNames', SectionTable);\n"
	files := map[string]string{diRegAt: reg, index: star, decl: enum,
		"app/setup/home.controller.ts": "export class HomeCtrl {}\n"}
	_, _, m, err := injectRepo(t, `["app/setup"]`, diRouter, files)
	if err != nil {
		t.Fatalf("star barrel: %v", err)
	}
	if !diResolved(t, m) || len(diUnknown(m)) != 0 {
		t.Fatalf("star barrel enum not read: %+v", m.Unknowns)
	}

	cases := map[string]struct{ reg, detail string }{
		// A registration passes on a binding imported from another barrel, which may relay the table.
		"relayed import": {"import { Widgets } from './widgets';\nimport { SectionTable } from './routes';\n" +
			"angular.module('core')\n  .component('w', Widgets)\n  .constant('SectionNames', SectionTable);\n",
			"import-chain-bindings:" + diRegAt + ":1:Widgets:reaches-table:app/setup/routes/index.ts:./routes.constants"},
		// A dynamic import the reader cannot name may reach the table under any name.
		"unnamed module": {"import { SectionTable } from './routes';\nconst w = import(name);\n" +
			"angular.module('core').constant('SectionNames', SectionTable);\n",
			"import-chain-bindings:" + diRegAt + ":2:*:module-not-literal"},
	}
	for name, c := range cases {
		files := map[string]string{diRegAt: c.reg, index: star, decl: enum,
			"app/setup/widgets/index.ts": "export * from './widget';\n", "app/setup/widgets/widget.ts": "import { SectionTable } from '../routes';\nexport const Widgets = { s: SectionTable };\n"}
		_, _, m, err := injectRepo(t, `["app/setup"]`, diRouter, files)
		if err != nil || diResolved(t, m) {
			t.Fatalf("%s: resolved (%v)", name, err)
		}
		if u := diUnknown(m); len(u) != 1 || u[0].Reason != "not-read-whole" {
			t.Fatalf("%s: diagnostic %+v", name, u)
		}
		if got := diDetails(m); len(got) != 1 || got[0] != c.detail {
			t.Fatalf("%s: detail %q, want %q", name, got, c.detail)
		}
	}
}
