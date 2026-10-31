package doccorpus

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// manifestDroppedEmpties lists every omitempty field reachable from Manifest whose explicitly
// empty value (a non-nil empty list, map or pointer target) the canonical encoder drops, as
// "Struct.Field". Scalars are excluded: an empty string or zero is no declaration.
func manifestDroppedEmpties(t *testing.T) []string {
	t.Helper()
	seen := map[reflect.Type]bool{}
	var out []string
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		for i := range typ.NumField() {
			f := typ.Field(i)
			tag := f.Tag.Get("json")
			if !f.IsExported() || tag == "-" {
				continue
			}
			walk(f.Type)
			if !strings.Contains(tag, ",omitempty") {
				continue
			}
			v := reflect.New(typ).Elem()
			switch field := v.Field(i); field.Kind() {
			case reflect.Pointer:
				field.Set(reflect.New(f.Type.Elem()))
			case reflect.Slice:
				field.Set(reflect.MakeSlice(f.Type, 0, 0))
			case reflect.Map:
				field.Set(reflect.MakeMap(f.Type))
			default:
				continue
			}
			data, err := Encode(v.Interface())
			if err != nil {
				t.Fatalf("%s.%s: %v", typ.Name(), f.Name, err)
			}
			if !strings.Contains(string(data), `"`+strings.Split(tag, ",")[0]+`"`) {
				out = append(out, typ.Name()+"."+f.Name)
			}
		}
	}
	walk(reflect.TypeFor[Manifest]())
	slices.Sort(out)
	return out
}

// TestDCIV1230ExplicitEmptyDeclarationsStayDistinct audits every optional manifest declaration
// (DCI-V1-230, V1-3106): a field whose explicitly empty value the encoder would drop is refused at
// validation, so a declared-empty manifest never round-trips as an undeclared one, while the same
// manifest without the field is admitted. A new omitempty field that drops its empty value fails
// the coverage check until it is added here with its refusal. Pointer fields such as
// production_scope encode their empty target and so are kept.
func TestDCIV1230ExplicitEmptyDeclarationsStayDistinct(t *testing.T) {
	t.Parallel()
	app, lib := inventoryRepos(t)
	workspace := func() Manifest { return inventoryManifest(t, app, lib) }
	single := func() Manifest {
		m, err := InventoryWorkspace(context.Background(), app, git(t, app, "rev-parse", "HEAD"), []string{".github", "docs", "go.mod"}, ManifestSchema, "2026-10-28T00:00:00Z", nil)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	inventory := func(m *Manifest) *RepositoryInventory {
		if m.RepositoryInventory == nil {
			m.RepositoryInventory = &RepositoryInventory{}
		}
		return m.RepositoryInventory
	}
	cases := map[string]struct {
		base  func() Manifest
		empty func(*Manifest)
	}{
		"Manifest.AdditionalRepositories": {single, func(m *Manifest) { m.AdditionalRepositories = []AdditionalRepository{} }},
		"AdditionalRepository.Revisions":  {workspace, func(m *Manifest) { m.AdditionalRepositories[0].Revisions = []string{} }},
		"Manifest.BehaviorRepositories":   {single, func(m *Manifest) { m.BehaviorRepositories = []BehaviorRepository{} }},
		"Provider.Shards":                 {single, func(m *Manifest) { m.Providers[0].Shards = []string{} }},
		"DerivationFilter.Paths": {single, func(m *Manifest) {
			m.Providers[0].Derive = &DerivationFilter{Paths: []string{}, Kinds: []string{"module"}}
		}},
		"DerivationFilter.Kinds": {single, func(m *Manifest) {
			m.Providers[0].Derive = &DerivationFilter{Paths: []string{"docs"}, Kinds: []string{}}
		}},
		"RepositoryInventory.Repositories": {single, func(m *Manifest) { inventory(m).Repositories = []RepositoryDeclaration{} }},
		"RepositoryDeclaration.Aliases":    {workspace, func(m *Manifest) { m.RepositoryInventory.Repositories[0].Aliases = []string{} }},
		"RepositoryInventory.FirstParty":   {single, func(m *Manifest) { inventory(m).FirstParty = []string{} }},
		"RepositoryInventory.EnvironmentWords": {single, func(m *Manifest) {
			inventory(m).EnvironmentWords = []string{}
		}},
		"RepositoryInventory.DeployJobs": {single, func(m *Manifest) { inventory(m).DeployJobs = []DeployJobDeclaration{} }},
		"ProductionScope.Exclude": {single, func(m *Manifest) {
			m.ProductionScope = &ProductionScope{Include: []string{"docs/**"}, Exclude: []string{}}
		}},
		"Manifest.OverBoundInputs": {single, func(m *Manifest) { m.OverBoundInputs = []OverBoundInput{} }},
	}
	dropped := manifestDroppedEmpties(t)
	var listed []string
	for name := range cases {
		listed = append(listed, name)
	}
	slices.Sort(listed)
	if !slices.Equal(dropped, listed) {
		t.Fatalf("omitempty fields whose empty value the encoder drops:\n got %v\nwant %v", dropped, listed)
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := c.base()
			if err := validateManifest(m); err != nil {
				t.Fatalf("base manifest refused: %v", err)
			}
			// The same declaration with the field absent is admitted where it has a container.
			if name == "DerivationFilter.Paths" || name == "DerivationFilter.Kinds" || name == "ProductionScope.Exclude" {
				absent := c.base()
				c.empty(&absent)
				switch name {
				case "DerivationFilter.Paths":
					absent.Providers[0].Derive.Paths = nil
				case "DerivationFilter.Kinds":
					absent.Providers[0].Derive.Kinds = nil
				default:
					absent.ProductionScope.Exclude = nil
				}
				if err := validateManifest(absent); err != nil {
					t.Fatalf("absent %s refused: %v", name, err)
				}
			}
			c.empty(&m)
			if err := validateManifest(m); err == nil {
				t.Fatalf("an explicitly empty %s was admitted, but encoding drops it", name)
			}
		})
	}
}
