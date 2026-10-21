package testsource

import (
	"fmt"
	"path"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// largeActorSuite extends the issue #1115 fixture with fixtures seeding users through factory
// defaults and with filler source files, so the factory default lookup runs once per seeded user
// over a suite whose file count grows (V1-1777).
func largeActorSuite(fixtures, fillers int) map[string]string {
	files := actorAuthFixture()
	files["e2e/fixtures/factories.ts"] = `export const makeUser = async ({ email, role = pickRole() }) => db.insert('users', { email, role });
export function twinUser({ role = 'viewer' }) { return role; }
export function twinUser({ role = 'editor' }) { return role; }
export const adminUser = async function ({ role = 'admin', email }) { return { role, email }; };
`
	for i := 0; i < fixtures; i++ {
		files[fmt.Sprintf("e2e/fixtures/f%04d.ts", i)] = fmt.Sprintf(`import { createUser } from './users';
import { makeUser, twinUser, adminUser } from './factories';

export async function fixture%d() {
  const a = await createUser({ email: 'a%d@example.test' });
  const b = await makeUser({ email: 'b%d@example.test' });
  const c = await twinUser({ email: 'c%d@example.test' });
  const d = await adminUser({ email: 'd%d@example.test' });
  return { a, b, c, d };
}
`, i, i, i, i, i)
	}
	for i := 0; i < fillers; i++ {
		files[fmt.Sprintf("src/lib/m%05d.ts", i)] = fmt.Sprintf("export function helper%d(x = 1, y = 'two') { return x + y.length; }\nexport const other%d = (a, b) => a + b;\n", i, i)
	}
	return files
}

// BenchmarkActorAuthorizationLargeSuite measures one analysis over 200 fixtures and 2,000 filler
// files (V1-1777).
func BenchmarkActorAuthorizationLargeSuite(b *testing.B) {
	files := largeActorSuite(200, 2000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AnalyzeActorAuthorization(files, "config/routes.rb", "app/models/ability.rb")
	}
}

// referenceFactoryDefaultRole is the per-call scan the index replaced (TSA-V0-033 before V1-1777),
// kept as the oracle for TestFactoryDefaultRolesMatchPerCallScan.
func referenceFactoryDefaultRole(factory string, sources map[string]*source, names []string) (string, string, bool) {
	found, role, at, other := 0, "", "", false
	for _, name := range names {
		s := sources[name]
		if s == nil {
			continue
		}
		for i := range s.tokens {
			if !s.ident(i, factory) || s.memberAccess(i) {
				continue
			}
			open := -1
			switch {
			case s.ident(i-1, "function") && s.punct(i+1, "("):
				open = i + 1
			case s.punct(i+1, "=") && !s.tokens[i+1].Glued:
				j := i + 2
				if s.ident(j, "async") {
					j++
				}
				if s.ident(j, "function") {
					j++
				}
				if s.punct(j, "(") && s.match[j] > j && (s.arrow(s.match[j]+1) || s.ident(j-1, "function")) {
					open = j
				}
			}
			if open < 0 || s.match[open] < 0 {
				continue
			}
			for j := open + 1; j < s.match[open]; j++ {
				if !s.ident(j, "role") || !s.punct(j+1, "=") || s.punct(j+2, "=") || s.punct(j+2, ">") {
					continue
				}
				t := s.tokens[min(j+2, len(s.tokens)-1)]
				if (t.Kind == appmap.JSString || t.Kind == appmap.JSTemplate) && t.Literal &&
					(s.punct(j+3, ",") || s.punct(j+3, "}") || s.punct(j+3, ")")) {
					found++
					role, at = t.Text, locate(s.path, t.Line)
				} else {
					other = true
				}
			}
		}
	}
	return role, at, found == 1 && !other
}

// TestFactoryDefaultRolesMatchPerCallScan proves the once-per-analysis index answers every
// identifier exactly as the per-call scan did, and pins the seeded roles it yields (V1-1777).
func TestFactoryDefaultRolesMatchPerCallScan(t *testing.T) {
	files := largeActorSuite(3, 3)
	names := []string{}
	sources := map[string]*source{}
	for name, text := range files {
		if webExtensions[path.Ext(name)] {
			names = append(names, name)
			sources[name] = parseSource(name, text)
		}
	}
	roles := factoryDefaultRoles(sources, names)
	checked := 0
	for _, s := range sources {
		for i := range s.tokens {
			if !s.ident(i, "") {
				continue
			}
			name := s.tokens[i].Text
			wantRole, wantAt, wantOK := referenceFactoryDefaultRole(name, sources, names)
			r := roles[name]
			ok := r != nil && r.found == 1 && !r.other
			if ok != wantOK || ok && (r.role != wantRole || r.at != wantAt) {
				t.Fatalf("%s: index %+v, scan %q %q %v", name, r, wantRole, wantAt, wantOK)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no identifiers checked")
	}
	report := AnalyzeActorAuthorization(files, "config/routes.rb", "app/models/ability.rb")
	got := map[string]string{}
	for _, a := range report.Actors {
		if a.Fixture == "fixture1" {
			got[a.Key] = a.Role + "/" + a.RoleFrom + "/" + a.Reason
		}
	}
	want := map[string]string{"a": "assistant/factory-default/", "b": "//role-default-unresolved", "c": "//role-default-unresolved", "d": "admin/factory-default/"}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("fixture1 %s = %q, want %q (all %v)", k, got[k], w, got)
		}
	}
}
