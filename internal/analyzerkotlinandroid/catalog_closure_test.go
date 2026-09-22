package analyzerkotlinandroid

import (
	"os"
	"strings"
	"testing"
)

// catalogIdentity returns the pinned request plus its Android version-catalog
// input, so a vector exercises the closed catalog extractor under a real fact
// identity. The pinned projection binds each input's SHA-256 to a source
// constant, so mutated catalog bytes cannot be driven through Process: they are
// refused as EXACT_BINDING_UNAVAILABLE before any extractor runs.
func catalogIdentity(t *testing.T) (Request, Input) {
	t.Helper()
	request := pinned(t)
	for _, in := range request.Inputs {
		if in.Family == "android.version.catalog" {
			return request, in
		}
	}
	t.Fatal("pinned projection has no android.version.catalog input")
	return Request{}, Input{}
}

// TestCatalogTableSetIsClosed pins KA-V0's closed catalog-row boundary. The
// previous extractor tracked whatever name appeared between brackets and
// emitted facts only for `versions` and `plugins`, so an unknown table, a row
// outside any table, and a row that is not the closed form for its own table
// were all admitted and silently ignored — an open subtree behind a green
// suite. Every vector below is admitted-and-ignored under that extractor and
// must reject the whole request as UNSUPPORTED_SCHEMA under this one.
func TestCatalogTableSetIsClosed(t *testing.T) {
	request, in := catalogIdentity(t)
	for _, test := range []struct{ name, catalog string }{
		{"unknown-table", "[versions]\nkotlin = \"2.0.0\"\n\n[metadata]\nformat = \"1.1\"\n"},
		{"row-outside-any-table", "format = \"1.1\"\n\n[versions]\nkotlin = \"2.0.0\"\n"},
		{"unquoted-version-value", "[versions]\nkotlin = 2.0.0\n"},
		{"plugin-without-id", "[versions]\nkotlin = \"2.0.0\"\n\n[plugins]\nkover = { version = \"0.9.8\" }\n"},
		{"library-not-inline-table", "[versions]\nkotlin = \"2.0.0\"\n\n[libraries]\ncoil = \"io.coil-kt:coil\"\n"},
		{"bundle-not-array", "[versions]\nkotlin = \"2.0.0\"\n\n[bundles]\nmedia3 = \"exoplayer\"\n"},
		{"bundle-unterminated", "[versions]\nkotlin = \"2.0.0\"\n\n[bundles]\nmedia3 = [\n    \"exoplayer\",\n"},
		{"bundle-element-not-atom", "[versions]\nkotlin = \"2.0.0\"\n\n[bundles]\nmedia3 = [\n    injected = \"value\",\n]\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			facts, rejected := catalogFacts(request, in, []byte(test.catalog))
			if rejected != "UNSUPPORTED_SCHEMA" {
				t.Fatalf("reason=%q facts=%d, want UNSUPPORTED_SCHEMA for an unclosed catalog row", rejected, len(facts))
			}
		})
	}
}

// TestCatalogLibraryAndBundleRowsAreAdmittedAndFactFree makes the KA-V0 matrix
// decision explicit rather than incidental. `[libraries]` and `[bundles]` rows
// are part of the KA-V0-003 pinned projection, so they must be admitted; the
// closed fact matrix has no library or bundle tuple, so they must emit nothing.
// Nothing asserted either half before, which is how 92 of the 149 rows across
// the two pinned catalogs drove an unasserted path on every run.
func TestCatalogLibraryAndBundleRowsAreAdmittedAndFactFree(t *testing.T) {
	request, in := catalogIdentity(t)
	for _, test := range []struct {
		fixture            string
		versions, plugins  int
		libraries, bundles int
	}{
		{"android-libs.versions.toml", 31, 4, 51, 2},
		{"ui-libs.versions.toml", 20, 2, 37, 2},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			content, err := os.ReadFile("testdata/" + test.fixture)
			if err != nil {
				t.Fatal(err)
			}
			facts, rejected := catalogFacts(request, in, content)
			if rejected != "" {
				t.Fatalf("pinned catalog rejected: %s", rejected)
			}
			rows, table := catalogRowTables(string(content))
			for _, want := range []struct {
				name  string
				count int
			}{{"versions", test.versions}, {"plugins", test.plugins}, {"libraries", test.libraries}, {"bundles", test.bundles}} {
				if rows[want.name] != want.count {
					t.Fatalf("%s rows=%d, want %d", want.name, rows[want.name], want.count)
				}
			}
			kinds := map[string]int{}
			for _, fact := range facts {
				kinds[fact.Kind]++
				if source := table[fact.StartLine]; source != "versions" && source != "plugins" {
					t.Fatalf("fact %+v came from the %q table, which carries no tuple", fact, source)
				}
			}
			if len(kinds) != 2 || kinds["android.version.catalog"] != test.versions || kinds["android.version.catalog.plugin"] != test.plugins {
				t.Fatalf("kinds=%v, want exactly %d android.version.catalog and %d android.version.catalog.plugin", kinds, test.versions, test.plugins)
			}
			if len(facts) != test.versions+test.plugins {
				t.Fatalf("facts=%d, want %d: the %d library and %d bundle rows carry no tuple", len(facts), test.versions+test.plugins, test.libraries, test.bundles)
			}
		})
	}
}

// TestCatalogSectionSetIsExactlyFour pins the closed table set itself, so
// widening it is a deliberate edit rather than a silent one.
func TestCatalogSectionSetIsExactlyFour(t *testing.T) {
	for _, name := range []string{"versions", "libraries", "plugins", "bundles"} {
		if !catalogSection(name) {
			t.Fatalf("%q must be an admitted catalog table", name)
		}
	}
	for _, name := range []string{"", "metadata", "Versions", "libraries ", "dependencies", "plugin"} {
		if catalogSection(name) {
			t.Fatalf("%q must not be an admitted catalog table", name)
		}
	}
}

// TestCatalogPluginFactCarriesTheIDNotTheRawRow guards the fix for the
// version-catalog plugin fact carrying the raw TOML row instead of the
// plugin id: the emitted android.version.catalog.plugin value must be the
// bare id, matching how the versions row already extracts its inner value.
func TestCatalogPluginFactCarriesTheIDNotTheRawRow(t *testing.T) {
	request, in := catalogIdentity(t)
	facts, rejected := catalogFacts(request, in, []byte("[plugins]\nroborazzi = { id = \"io.github.takahirom.roborazzi\", version.ref = \"roborazzi\" }\n"))
	if rejected != "" {
		t.Fatalf("reason=%q", rejected)
	}
	fact, ok := findFact(facts, "android.version.catalog.plugin", "declares-plugin", "io.github.takahirom.roborazzi")
	if !ok {
		t.Fatalf("plugin id fact missing; facts=%+v", facts)
	}
	if strings.Contains(fact.Value, "{") {
		t.Fatalf("plugin fact still carries the raw TOML row: %q", fact.Value)
	}
}

// TestCatalogVersionSpanSkipsAliasNameCollision guards the fix to
// factFromLiteral (lexer.go): it searched the whole raw line for the literal,
// so an alias whose name embeds the version string -- `kotlin17 = "17"` --
// matched the "17" inside "kotlin17" first and reported that column instead
// of the quoted literal's own. The value-side text still equalled the literal
// either way, so a check that only compares the spanned text against
// fact.Value cannot see this: the span's start column must land inside the
// quotes, past the "=".
func TestCatalogVersionSpanSkipsAliasNameCollision(t *testing.T) {
	request, in := catalogIdentity(t)
	facts, rejected := catalogFacts(request, in, []byte("[versions]\nkotlin17 = \"17\"\n"))
	if rejected != "" {
		t.Fatalf("reason=%q", rejected)
	}
	fact, ok := findFact(facts, "android.version.catalog", "pins-version", "17")
	if !ok {
		t.Fatalf("version fact missing; facts=%+v", facts)
	}
	const line = "kotlin17 = \"17\""
	if want := strings.LastIndex(line, "17") + 1; fact.StartColumn != want {
		t.Fatalf("start column=%d want=%d (line=%q)", fact.StartColumn, want, line)
	}
}

// catalogRowTables walks the catalog the way the closed extractor does and
// returns the row count per table plus the owning table of each row's
// one-based line, so a fact can be traced back to the table that produced it.
func catalogRowTables(content string) (map[string]int, map[int]string) {
	rows, table := map[string]int{}, map[int]string{}
	section, inBundle := "", false
	for index, line := range strings.Split(content, "\n") {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if inBundle {
			inBundle = text != "]"
			continue
		}
		if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
			section = text[1 : len(text)-1]
			continue
		}
		pieces := strings.SplitN(text, "=", 2)
		if len(pieces) != 2 {
			continue
		}
		inBundle = strings.TrimSpace(pieces[1]) == "["
		rows[section]++
		table[index+1] = section
	}
	return rows, table
}
