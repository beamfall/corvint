package convert

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// fixture is one committed source/golden pair under testdata.
type fixture struct {
	name, bind, spec string
}

var fixtures = []fixture{
	{"reachable-a79ab231", "a79ab231e86f5365d8a84732e90f4b2c0755ac4a", "cem/0.2"},
	{"cem01-valid", "0ea42c62cf2ed06e051ab583c49abbfedd3f5a86", "cem/0.1"},
	{"cem03-coverage-covered", "cef3e87ac46d04009f761104ea01d263ac3b32b2", "cem/0.3"},
}

const bindF0fda542 = "f0fda542fa64969df58dc9cfd8268f321d562880"

const f0fda542Sha256 = "bf653c5053abd0b2ce52b8c11b9b039d7845a0e95e5f7f8a1682585537971888"

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustConvert(t *testing.T, src []byte, in Inputs) ([]byte, Report) {
	t.Helper()
	out, r, err := Convert(src, in)
	if err != nil {
		t.Fatalf("convert refused: %v (%+v)", err, r)
	}
	return out, r
}

// TestCEMV1096ConvertAcceptsEachZeroDialect converts one 0.1, 0.2 and 0.3
// source to its committed golden bytes.
func TestCEMV1096ConvertAcceptsEachZeroDialect(t *testing.T) {
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			src := readTestdata(t, f.name+".source.json")
			out, r := mustConvert(t, src, Inputs{BindCommit: f.bind})
			if want := readTestdata(t, f.name+".converted.json"); !bytes.Equal(out, want) {
				t.Fatalf("output differs from golden %s.converted.json", f.name)
			}
			if r.Status != "CONVERTED" || r.Code != nil || r.SourceSpec == nil || *r.SourceSpec != f.spec {
				t.Fatalf("report = %+v", r)
			}
			if r.SourceBinding != BindingNotChecked {
				t.Fatalf("binding = %s, want NOT_CHECKED without a reader", r.SourceBinding)
			}
		})
	}
}

// TestCEMV1097ConvertMemberMapping checks the exact member mapping: change
// members carry over byte-for-value, producer-only members are the four empty
// arrays, role is absent and no member is synthesized.
func TestCEMV1097ConvertMemberMapping(t *testing.T) {
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			src := readTestdata(t, f.name+".source.json")
			out, _ := mustConvert(t, src, Inputs{BindCommit: f.bind})
			if _, err := wire.ParseStable(out); err != nil {
				t.Fatalf("ParseStable refused converter output: %v", err)
			}
			var in, got map[string]any
			if err := json.Unmarshal(src, &in); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatal(err)
			}
			if got["spec"] != wire.StableSpec || got["excludedPath"] != wire.ExcludedCEMPath {
				t.Fatalf("spec/excludedPath = %v/%v", got["spec"], got["excludedPath"])
			}
			for _, k := range []string{"baseRevision", "patchSha256", "hunks", "evidence"} {
				if !reflect.DeepEqual(in[k], got[k]) {
					t.Fatalf("member %s changed", k)
				}
			}
			for _, k := range []string{"criterionBindings", "runnerReceipts", "criterionLinks", "artifacts"} {
				if a, ok := got[k].([]any); !ok || len(a) != 0 {
					t.Fatalf("producer-only member %s = %v, want []", k, got[k])
				}
			}
			if _, ok := got["role"]; ok {
				t.Fatal("role synthesized")
			}
			want := map[string]bool{"spec": true, "baseRevision": true, "patchSha256": true, "excludedPath": true, "hunks": true, "evidence": true,
				"criterionBindings": true, "runnerReceipts": true, "criterionLinks": true, "artifacts": true, "provenance": true}
			for k := range got {
				if !want[k] {
					t.Fatalf("unexpected root member %s", k)
				}
			}
			if len(got) != len(want) {
				t.Fatalf("root members = %d, want %d", len(got), len(want))
			}
		})
	}
}

// TestCEMV1098ConvertProvenanceValues recomputes every provenance value from
// the source bytes and the bind commit.
func TestCEMV1098ConvertProvenanceValues(t *testing.T) {
	for _, f := range fixtures {
		src := readTestdata(t, f.name+".source.json")
		out, r := mustConvert(t, src, Inputs{BindCommit: f.bind})
		m, err := wire.ParseStable(out)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(src)
		want := wire.StableProvenance{SourceSpec: f.spec, SourceBlobOid: BlobOid(src), SourceSha256: hex.EncodeToString(sum[:]), BindCommit: f.bind, Converter: Converter}
		if m.Provenance == nil || *m.Provenance != want || r.Provenance == nil || *r.Provenance != want {
			t.Fatalf("%s provenance = %+v / %+v, want %+v", f.name, m.Provenance, r.Provenance, want)
		}
		outSum := sha256.Sum256(out)
		if r.OutputSha256 == nil || *r.OutputSha256 != hex.EncodeToString(outSum[:]) {
			t.Fatalf("%s outputSha256 = %v", f.name, r.OutputSha256)
		}
	}
	if BlobOid(nil) != "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391" {
		t.Fatal("BlobOid(empty) is not Git's empty blob")
	}
	if got := BindCommitFromName("x/a79ab231e86f5365d8a84732e90f4b2c0755ac4a.cem.json"); got != "a79ab231e86f5365d8a84732e90f4b2c0755ac4a" {
		t.Fatalf("BindCommitFromName = %q", got)
	}
	for _, bad := range []string{"change.cem.json", "A79AB231E86F5365D8A84732E90F4B2C0755AC4A.cem.json", "a79ab231.cem.json"} {
		if BindCommitFromName(bad) != "" {
			t.Fatalf("BindCommitFromName(%q) pinned a commit", bad)
		}
	}
}

type fakeReader struct {
	oid     string
	present bool
	err     error
}

func (f fakeReader) SidecarBlob(string) (string, bool, error) { return f.oid, f.present, f.err }

// TestCEMV1099ConvertSourceBinding covers every binding state: the output bytes
// never depend on the state, and only MISMATCH refuses.
func TestCEMV1099ConvertSourceBinding(t *testing.T) {
	f := fixtures[0]
	src := readTestdata(t, f.name+".source.json")
	golden := readTestdata(t, f.name+".converted.json")
	for _, c := range []struct {
		reader fakeReader
		want   string
	}{
		{fakeReader{oid: BlobOid(src), present: true}, BindingExact},
		{fakeReader{}, BindingAbsent},
		{fakeReader{err: errors.New("object unavailable")}, BindingUnavailable},
	} {
		out, r := mustConvert(t, src, Inputs{BindCommit: f.bind, Git: c.reader})
		if r.SourceBinding != c.want || !bytes.Equal(out, golden) {
			t.Fatalf("binding %s: got %s, golden equal %v", c.want, r.SourceBinding, bytes.Equal(out, golden))
		}
	}
	out, r, err := Convert(src, Inputs{BindCommit: f.bind, Git: fakeReader{oid: BlobOid([]byte("other")), present: true}})
	if out != nil || r.SourceBinding != BindingMismatch || cemcode.CodeOf(err) != CodeSourceBindingMismatch || r.Provenance != nil {
		t.Fatalf("mismatch: out=%d report=%+v err=%v", len(out), r, err)
	}
}

// TestCEMV1100ConvertCorpusIdempotent converts every committed corpus map
// twice to identical bytes and proves a converted map is refused on reconvert.
// After the V1-1277 migration every committed map is cem/1.0: a migrated map
// (one with converter provenance) must equal the conversion of its
// provenance-named source blob byte for byte (CEM-V1-200), and a natively
// produced stable map must refuse.
func TestCEMV1100ConvertCorpusIdempotent(t *testing.T) {
	dir := filepath.Join("..", "..", "..", ".corvint", "changes")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n, migrated := 0, 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".cem.json") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		bind := BindCommitFromName(e.Name())
		if wire.DeclaresStable(src) {
			if _, _, err := Convert(src, Inputs{BindCommit: bind}); cemcode.CodeOf(err) != CodeSourceIsStable {
				t.Fatalf("%s: stable map not refused: %v", e.Name(), err)
			}
			doc, err := wire.ParseStable(src)
			if doc == nil {
				t.Fatalf("%s: %v", e.Name(), err)
			}
			if p := doc.Provenance; p != nil {
				source := sourceBlob(t, dir, p.SourceBlobOid)
				if p.BindCommit != bind || BlobOid(source) != p.SourceBlobOid || sha256Hex(source) != p.SourceSha256 {
					t.Fatalf("%s: provenance does not name its source blob", e.Name())
				}
				if again, _ := mustConvert(t, source, Inputs{BindCommit: bind}); !bytes.Equal(again, src) {
					t.Fatalf("%s: committed bytes differ from the conversion of %s", e.Name(), p.SourceBlobOid)
				}
				migrated++
			} else if bound := sourceBlob(t, dir, bind+":.corvint/change.cem.json"); !bytes.Equal(bound, src) {
				// A natively produced stable map is the sidecar its bind
				// commit tracks; a migrated map without provenance is not.
				t.Fatalf("%s: stable map without provenance differs from its bind commit's sidecar", e.Name())
			}
			continue
		}
		first, _ := mustConvert(t, src, Inputs{BindCommit: bind})
		second, _ := mustConvert(t, src, Inputs{BindCommit: bind})
		if !bytes.Equal(first, second) {
			t.Fatalf("%s: second conversion differs", e.Name())
		}
		if _, _, err := Convert(first, Inputs{BindCommit: bind}); cemcode.CodeOf(err) != CodeSourceIsStable {
			t.Fatalf("%s: reconvert = %v, want %s", e.Name(), err, CodeSourceIsStable)
		}
		n++
	}
	if n+migrated == 0 {
		t.Fatal("no corpus maps converted")
	}
	t.Logf("converted %d corpus maps twice; reconverted %d migrated maps from their source blobs", n, migrated)
}

// sourceBlob reads a migrated map's source blob from the repository's objects.
func sourceBlob(t *testing.T, dir, oid string) []byte {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "cat-file", "blob", oid).Output()
	if err != nil {
		t.Skipf("NOT_RUN: source blob %s unavailable: %v", oid, err)
	}
	return out
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// TestCEMV1100ConvertF0fda542 pins the 877-hunk map's converted bytes.
func TestCEMV1100ConvertF0fda542(t *testing.T) {
	src := readTestdata(t, "f0fda542.source.json")
	out, r := mustConvert(t, src, Inputs{BindCommit: bindF0fda542})
	again, _ := mustConvert(t, src, Inputs{BindCommit: bindF0fda542})
	if !bytes.Equal(out, again) || r.HunkCount != 877 || *r.OutputSha256 != f0fda542Sha256 {
		t.Fatalf("f0fda542: equal=%v hunks=%d sha=%s", bytes.Equal(out, again), r.HunkCount, *r.OutputSha256)
	}
}

// TestCEMV1101ConvertRefusalCodes covers each bounded refusal code.
func TestCEMV1101ConvertRefusalCodes(t *testing.T) {
	f := fixtures[0]
	src := readTestdata(t, f.name+".source.json")
	golden := readTestdata(t, f.name+".converted.json")
	var zero map[string]any
	if err := json.Unmarshal(src, &zero); err != nil {
		t.Fatal(err)
	}
	zero["hunks"], zero["evidence"] = []any{}, []any{}
	empty, err := json.Marshal(zero)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		src  []byte
		bind string
		want string
	}{
		{"stable", golden, f.bind, CodeSourceIsStable},
		{"json", []byte("{"), f.bind, cemcode.InvalidJSON},
		{"bind-missing", src, "", CodeBindCommitUnavailable},
		{"bind-short", src, "a79ab231", CodeBindCommitUnavailable},
		{"bind-upper", src, strings.ToUpper(f.bind), CodeBindCommitUnavailable},
	} {
		out, r, err := Convert(c.src, Inputs{BindCommit: c.bind})
		if out != nil || r.Status != "REFUSED" || r.Code == nil || *r.Code != c.want || cemcode.CodeOf(err) != c.want || r.Provenance != nil || r.OutputSha256 != nil {
			t.Fatalf("%s: out=%d report=%+v err=%v, want %s", c.name, len(out), r, err, c.want)
		}
	}
	// The 0.x reader admits an empty hunk list; a stable map needs one hunk.
	if out, r, err := Convert(empty, Inputs{BindCommit: f.bind}); out != nil || cemcode.CodeOf(err) != CodeEmptyHunks || r.Code == nil || *r.Code != CodeEmptyHunks {
		t.Fatalf("zero hunks: out=%d report=%+v err=%v", len(out), r, err)
	}
}
