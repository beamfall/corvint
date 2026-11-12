// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	json "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const flowsSchemaTestdata = "testdata/flows-schema"

// readFlowsBody writes body as the declaration of a fresh root and reads it
// through the one reader.
func readFlowsBody(t *testing.T, body string) (*FlowsDeclaration, error) {
	t.Helper()
	root := t.TempDir()
	writeRaw(t, root, FlowsDeclarationPath, []byte(body))
	return ReadFlowsDeclaration(root)
}

// FSV-V1-001 (V1-5410): today's file, with every version 0 member, reads as
// version 0, and no file is no declaration and no refusal.
func TestFSVV1001VersionZeroReads_V15410(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join(flowsSchemaTestdata, "0.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	declared, err := readFlowsBody(t, string(golden))
	if err != nil || declared == nil || declared.Version != 0 || declared.Dir != "flows" || len(declared.Kinds) != 2 || declared.PlaywrightConfig != "playwright.config.ts" {
		t.Fatalf("version 0 golden: %+v %v", declared, err)
	}
	declared, err = readFlowsBody(t, `{"profile":"corvint-flows/0","flows":"flows"}`)
	if err != nil || declared == nil || declared.Version != 0 {
		t.Fatalf("minimal version 0: %+v %v", declared, err)
	}
	if declared, err := ReadFlowsDeclaration(t.TempDir()); declared != nil || err != nil {
		t.Fatalf("no file: %+v %v", declared, err)
	}
	if got := FlowsSupportedVersions(); !slices.Contains(got, 0) {
		t.Fatalf("version 0 is not supported: %v", got)
	}
}

// FSV-V1-002, FSV-V1-004 (V1-5410): a well-formed version outside the
// supported list is FLOWS_SCHEMA_UNSUPPORTED naming the found and supported
// versions, checked before any other member, and never read as version 0.
func TestFSVV1004UnsupportedVersionRefused_V15410(t *testing.T) {
	supported := FlowsSupportedVersions()
	next := strconv.Itoa(FlowsWriteVersion() + 1)
	for _, tc := range []struct{ body, found string }{
		{`{"profile":"corvint-flows/` + next + `","flows":"flows"}`, next},
		{`{"profile":"corvint-flows/7","flows":"flows"}`, "7"},
		// A newer file's unknown member does not turn the refusal into
		// FLOWS_DECLARATION_INVALID: the version is read first.
		{`{"profile":"corvint-flows/7","flows":"flows","variants":{"a":1}}`, "7"},
		{`{"flows":"../out","profile":"corvint-flows/7"}`, "7"},
		{`{"profile":"corvint-flows/123456789012345678901234567890"}`, "123456789012345678901234567890"},
	} {
		declared, err := readFlowsBody(t, tc.body)
		refusal := AsFlowsRefusal(err)
		if declared != nil || refusal == nil || refusal.Reason != affected.UnknownFlowsSchemaUnsupported || !refusal.Unsupported() || refusal.Found != tc.found || !reflect.DeepEqual(refusal.Supported, supported) {
			t.Fatalf("%s: %+v %#v", tc.body, declared, err)
		}
		for _, want := range []string{FlowsDeclarationPath, "version " + tc.found, "reads version " + supportedList(supported)} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: %q lacks %q", tc.body, err.Error(), want)
			}
		}
		if want := FlowsDeclarationPath + " version " + tc.found + "; supported " + supportedList(supported); refusal.Detail() != want {
			t.Fatalf("%s: detail %q, want %q", tc.body, refusal.Detail(), want)
		}
	}
}

// FSV-V1-001, FSV-V1-004 (V1-5410): a profile without the prefix or with a
// malformed N, and a malformed file, are FLOWS_DECLARATION_INVALID, never
// FLOWS_SCHEMA_UNSUPPORTED and never version 0.
func TestFSVV1004MalformedProfileInvalid_V15410(t *testing.T) {
	for _, body := range []string{
		`{"profile":"flows/1","flows":"flows"}`,
		`{"profile":"corvint-flows/01","flows":"flows"}`,
		`{"profile":"corvint-flows/00","flows":"flows"}`,
		`{"profile":"corvint-flows/-1","flows":"flows"}`,
		`{"profile":"corvint-flows/+1","flows":"flows"}`,
		`{"profile":"corvint-flows/1.0","flows":"flows"}`,
		`{"profile":"corvint-flows/","flows":"flows"}`,
		`{"profile":"corvint-flows/0 ","flows":"flows"}`,
		`{"profile":" corvint-flows/0","flows":"flows"}`,
		`{"profile":"Corvint-flows/0","flows":"flows"}`,
		`{"profile":"corvint-flows/٣","flows":"flows"}`,
		`{"profile":"corvint-e2e-flows/0","flows":"flows"}`,
		`{"profile":0,"flows":"flows"}`,
		`{"profile":null,"flows":"flows"}`,
		`{"flows":"flows"}`,
		`{"profile":"corvint-flows/0","profile":"corvint-flows/0","flows":"flows"}`,
		`{"profile":"corvint-flows/0","flows":"flows"`,
		`["corvint-flows/0"]`,
		``,
		// Version 0 with an unknown member or an invalid member.
		`{"profile":"corvint-flows/0","flows":"flows","extra":1}`,
		`{"profile":"corvint-flows/0","flows":"../out"}`,
	} {
		declared, err := readFlowsBody(t, body)
		refusal := AsFlowsRefusal(err)
		if declared != nil || refusal == nil || refusal.Reason != affected.UnknownFlowsDeclarationInvalid || refusal.Unsupported() || refusal.Found != "" || refusal.Detail() != FlowsDeclarationPath {
			t.Fatalf("%q: %+v %#v", body, declared, err)
		}
		if !strings.HasPrefix(err.Error(), FlowsDeclarationPath+": ") {
			t.Fatalf("%q: %q does not name the file", body, err.Error())
		}
	}
	// A file that is not a regular file is refused as invalid too.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(FlowsDeclarationPath)), 0o700); err != nil {
		t.Fatal(err)
	}
	if declared, err := ReadFlowsDeclaration(root); declared != nil || AsFlowsRefusal(err) == nil || AsFlowsRefusal(err).Reason != affected.UnknownFlowsDeclarationInvalid {
		t.Fatalf("directory: %+v %v", declared, err)
	}
}

// flowsMembers lists, sorted, every member path a decoded type accepts:
// `a.b` for a member of a nested object, `a[].b` for a member of an array
// element.
func flowsMembers(typ reflect.Type) []string {
	var out []string
	var walk func(prefix string, typ reflect.Type)
	walk = func(prefix string, typ reflect.Type) {
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		for i := range typ.NumField() {
			field := typ.Field(i)
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "-" || !field.IsExported() {
				continue
			}
			if name == "" {
				name = field.Name
			}
			member, elem := prefix+name, field.Type
			for elem.Kind() == reflect.Pointer {
				elem = elem.Elem()
			}
			switch {
			case elem.Kind() == reflect.Struct:
				walk(member+".", elem)
			case (elem.Kind() == reflect.Slice || elem.Kind() == reflect.Array) && indirect(elem.Elem()).Kind() == reflect.Struct:
				walk(member+"[].", indirect(elem.Elem()))
			case elem.Kind() == reflect.Map && indirect(elem.Elem()).Kind() == reflect.Struct:
				walk(member+"{}.", indirect(elem.Elem()))
			default:
				out = append(out, member)
			}
		}
	}
	walk("", typ)
	slices.Sort(out)
	return out
}

func indirect(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

// checkFlowsMembers compares the members a reader accepts for version n with
// the list recorded when n was released (FSV-V1-002).
func checkFlowsMembers(n int, typ reflect.Type, recorded []string) error {
	got := flowsMembers(typ)
	var added, removed []string
	for _, m := range got {
		if !slices.Contains(recorded, m) {
			added = append(added, m)
		}
	}
	for _, m := range recorded {
		if !slices.Contains(got, m) {
			removed = append(removed, m)
		}
	}
	if len(added) == 0 && len(removed) == 0 {
		return nil
	}
	return fmt.Errorf("%s version %d: the reader's members differ from %s/%d.members (added %v, removed %v); a member change must increment N: add version %d to flowsSchemas with its own member list and golden file (FSV-V1-002)",
		FlowsDeclarationPath, n, flowsSchemaTestdata, n, added, removed, FlowsWriteVersion()+1)
}

func readMemberList(t *testing.T, n int) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(flowsSchemaTestdata, strconv.Itoa(n)+".members"))
	if err != nil {
		t.Fatalf("version %d has no recorded member list (FSV-V1-002): %v", n, err)
	}
	return strings.Fields(string(raw))
}

// goldenMembers lists, sorted, the member paths a golden JSON value uses.
func goldenMembers(prefix string, value any) []string {
	var out []string
	switch v := value.(type) {
	case map[string]any:
		for name, child := range v {
			switch child.(type) {
			case map[string]any:
				out = append(out, goldenMembers(prefix+name+".", child)...)
			case []any:
				out = append(out, goldenMembers(prefix+name+"[].", child)...)
			default:
				out = append(out, prefix+name)
			}
		}
	case []any:
		for _, element := range v {
			out = append(out, goldenMembers(prefix, element)...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// FSV-V1-002 (V1-5410): the supported list is closed and ascending, and for
// every supported version the reader's members equal the member list
// recorded at release, which the golden file uses in full and the reader
// accepts as that version.
func TestFSVV1002MemberListsAreFrozen_V15410(t *testing.T) {
	versions := FlowsSupportedVersions()
	if len(versions) == 0 || versions[0] != 0 || !slices.IsSorted(versions) || len(slices.Compact(slices.Clone(versions))) != len(versions) {
		t.Fatalf("supported versions %v must be a strictly ascending list starting at 0", versions)
	}
	if FlowsWriteVersion() != versions[len(versions)-1] {
		t.Fatalf("write version %d is not the highest supported %v", FlowsWriteVersion(), versions)
	}
	for _, schema := range flowsSchemas {
		recorded := readMemberList(t, schema.version)
		if err := checkFlowsMembers(schema.version, schema.file, recorded); err != nil {
			t.Fatal(err)
		}
		golden, err := os.ReadFile(filepath.Join(flowsSchemaTestdata, strconv.Itoa(schema.version)+".golden.json"))
		if err != nil {
			t.Fatal(err)
		}
		var value any
		if err := json.Unmarshal(golden, &value); err != nil {
			t.Fatal(err)
		}
		if got := goldenMembers("", value); !reflect.DeepEqual(got, recorded) {
			t.Fatalf("version %d golden uses %v, recorded %v", schema.version, got, recorded)
		}
		declared, err := decodeFlowsDeclaration(golden)
		if err != nil || declared.Version != schema.version {
			t.Fatalf("version %d golden: %+v %v", schema.version, declared, err)
		}
	}
}

// flowsV0WithAddedMember is the fixture of FSV-V1-002: version 0's members
// plus one, as a change that forgot to increment N would decode them.
type flowsV0WithAddedMember struct {
	Profile    string                 `json:"profile"`
	Flows      string                 `json:"flows"`
	Kinds      []flowsKindRule        `json:"kinds,omitempty"`
	Playwright *flowsPlaywrightMember `json:"playwright,omitempty"`
	Variants   []struct {
		Name string `json:"name"`
	} `json:"variants,omitempty"`
}

// FSV-V1-002 (V1-5410): the member-list test fails on a fixture that adds a
// member under version 0, names the member and says to increment N; a
// removed member fails the same way.
func TestFSVV1002AddedMemberWithoutNewVersionFails_V15410(t *testing.T) {
	recorded := readMemberList(t, 0)
	err := checkFlowsMembers(0, reflect.TypeFor[flowsV0WithAddedMember](), recorded)
	if err == nil || !strings.Contains(err.Error(), "variants[].name") || !strings.Contains(err.Error(), "must increment N") {
		t.Fatalf("added member: %v", err)
	}
	type removed struct {
		Profile string `json:"profile"`
		Flows   string `json:"flows"`
	}
	err = checkFlowsMembers(0, reflect.TypeFor[removed](), recorded)
	if err == nil || !strings.Contains(err.Error(), "removed [kinds[].kind kinds[].path playwright.config playwright.discovery]") || !strings.Contains(err.Error(), "must increment N") {
		t.Fatalf("removed members: %v", err)
	}
}
