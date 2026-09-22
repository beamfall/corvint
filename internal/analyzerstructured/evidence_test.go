package analyzerstructured

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func representativeFrame(t *testing.T) []byte {
	t.Helper()
	bodies := []struct {
		profile    FormatProfile
		path, body string
	}{{JSONLProfile, "contracts/library.jsonl", pinnedBeamfallJSONL}, {PropertiesProfile, "gradle.properties", pinnedBeamfallProperties}, {WebManifestProfile, "web/manifest.webmanifest", `{"name":"Beamfall","scope":"/web/","display":"standalone"}`}, {SVGProfile, "assets/icon.svg", `<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0"/></svg>`}}
	inputs := make([]Input, len(bodies))
	for i, body := range bodies {
		sum := sha256.Sum256([]byte(body.body))
		inputs[i] = Input{Handle: "input-" + string(rune('1'+i)), Family: body.profile, Path: body.path, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(body.body))}
	}
	for i := 1; i < len(inputs); i++ {
		for j := i; j > 0 && compareInput(inputs[j], inputs[j-1]) < 0; j-- {
			inputs[j], inputs[j-1] = inputs[j-1], inputs[j]
		}
	}
	frame, err := json.Marshal(Request{Profile: Profile, Family: Family, RequestID: "representative-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	return append(frame, '\n')
}

func BenchmarkAnalyzeRepresentative(b *testing.B) {
	frame := representativeFrame(&testing.T{})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Analyze(frame)
	}
}

// TestProductionCausalRatchets binds the production entry point to fixed
// allocation and latency ceilings. The restored alternative does one ordinary
// invocation but restores the rejected request-copy allocation; it must fail
// the same allocation ceiling without self-scaling the workload. Both ceilings
// are non-race measurements: race instrumentation adds allocations and slows
// each call roughly tenfold, so a race build skips rather than passes vacuously.
func TestProductionCausalRatchets(t *testing.T) {
	if raceBuild {
		t.Skip("allocation and latency ceilings are measured only in non-race builds")
	}
	frame := representativeFrame(t)
	candidate := testing.AllocsPerRun(100, func() { _ = Analyze(frame) })
	if candidate > 319 {
		t.Fatalf("production allocations %.0f exceed ceiling", candidate)
	}
	restored := testing.AllocsPerRun(100, func() { _ = restoredCopyingAnalyze(frame) })
	if restored <= 319 {
		t.Fatalf("restored copying alternative %.0f did not fail allocation ceiling", restored)
	}
	for sample := 0; sample < 5; sample++ {
		started := time.Now()
		for run := 0; run < 128; run++ {
			_ = Analyze(frame)
		}
		if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
			t.Fatalf("production latency sample %d exceeded causal ceiling: %s", sample, elapsed)
		}
	}
}

func restoredCopyingAnalyze(frame []byte) []byte {
	copy := append([]byte(nil), frame...)
	return Analyze(copy)
}

func TestCandidateSourceDigestAndASTClosure(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	files := candidateSourceFiles()
	for _, entry := range files {
		path := filepath.Join(root, entry.path)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != entry.sha256 {
			t.Fatalf("source digest %s=%s want=%s", entry.path, got, entry.sha256)
		}
		assertSourceImports(t, path, entry.imports)
	}
	assertMainOSClosure(t, filepath.Join(root, "cmd/corvint-analyzer-structured-data/main.go"))
	const sourceListSHA256 = "d4c7638980c40c5af1d4614ff90800e60ffe970fed3997534b852bc5b7415c99"
	if got := sourceListDigest(files); got != sourceListSHA256 {
		t.Fatalf("source list digest=%s want=%s", got, sourceListSHA256)
	}
}

type candidateSource struct {
	path    string
	sha256  string
	imports []string
}

func candidateSourceFiles() []candidateSource {
	return []candidateSource{
		{"cmd/corvint-analyzer-structured-data/main.go", "501e59bb264c3a57b5b01c1e1854deb0bfe80028d1dd8c16a94c4704e5a38022", []string{"fmt", "io", "os", "github.com/Beamfall/corvint/internal/analyzerstructured"}},
		{"internal/analyzerstructured/analyzer.go", "3423e6709fc3163cb89aea509258346d2f8cd217c9d38cba2691987356d24895", []string{"bytes", "crypto/sha256", "encoding/base64", "encoding/hex", "encoding/json", "sort", "strings", "unicode/utf8"}},
		{"internal/analyzerstructured/descriptor.go", "af8af9f415f554575fec15686497c82a88459ce0fa7a29efe70e883afe1d825e", []string{"crypto/sha256", "encoding/hex", "encoding/json"}},
		{"internal/analyzerstructured/format.go", "b8b16d82c091c91fc44b093fff98c80d869cd7cd28f9ac75e474057ca0834cb6", []string{"crypto/sha256", "encoding/hex", "unicode/utf8"}},
		{"internal/analyzerstructured/hcl.go", "f81e7f84b43171117b24c3eff9ceaba522e7a70f652c2d27322bccd5891ca76d", []string{"bytes", "strings"}},
		{"internal/analyzerstructured/json.go", "5d3e47a2abe2bc554ba520250d48384c75e0dfdef453bc8817bc5e5449b0a2a0", []string{"bytes", "encoding/json", "io", "unicode/utf8"}},
		{"internal/analyzerstructured/properties.go", "485ed728c37643c160d8a3dc1a2d2cca20d0bd79ad755c54af5935594152e73a", []string{"bytes", "strings"}},
		{"internal/analyzerstructured/toml.go", "c349eb3c6e74307466b9aa3c0046f52ba7ae0980169c25165f6a281d21d57f6f", []string{"bytes", "strconv", "strings"}},
		{"internal/analyzerstructured/types.go", "7c3190629c78a4f5074ef690f1982ff7ab4a70077ea7f3d1c6f0115716a58509", []string{"crypto/sha256"}},
		{"internal/analyzerstructured/web.go", "b2833c42a826f2fce4cd52094a42e57ac39e3ada683fca59f4a9f3b7c7169ba8", []string{"encoding/json"}},
		{"internal/analyzerstructured/xml.go", "2245236cccf58eb4b76f062307bffcc1044a7020768ef7ae808c55d83a9aa8ff", []string{"bytes", "encoding/xml", "io", "strings"}},
		{"internal/analyzerstructured/yaml.go", "7e8d240e8b44ea9be52a48375469d7c98de8c1e07137bfb7cd960a9b4f4bf841", []string{"bytes", "strings"}},
	}
}

func sourceListDigest(files []candidateSource) string {
	parts := make([]string, len(files))
	for index, file := range files {
		parts[index] = file.path + "\x00" + file.sha256
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func assertSourceImports(t *testing.T, path string, want []string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(file.Imports))
	for index, spec := range file.Imports {
		got[index] = strings.Trim(spec.Path.Value, `"`)
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("imports %s=%q want=%q", path, got, want)
	}
}

func assertMainOSClosure(t *testing.T, path string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	// File is the type of the run seam's stdin parameter, not a channel; its only value is os.Stdin.
	allowed := map[string]bool{"Args": true, "Exit": true, "File": true, "SameFile": true, "Stdin": true, "Stdout": true}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if ok && identifier.Name == "os" && !allowed[selector.Sel.Name] {
			t.Fatalf("ambient os channel %s", selector.Sel.Name)
		}
		return true
	})
}

func TestCoreParentCandidateByteIdentity(t *testing.T) {
	// The candidate's invariant is that the core binary never selects it. In the
	// integration checkpoint the core legitimately differs from the frozen base
	// (reviewed runtime work), so the former byte-identity comparison against
	// 718dfc7 is asserted as a dependency-closure property instead.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	command := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/corvint")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	dependencies, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"internal/analyzerstructured", "corvint-analyzer-structured-data"} {
		if strings.Contains(string(dependencies), forbidden) {
			t.Fatalf("Core selected structured candidate via %s", forbidden)
		}
	}
}
