package analyzerpython

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const restoredParentCommit = "90795d7750405933227962633b23c5441ef79ae9"
const restoredParentAnalyzerSHA256 = "796619485bd3565214f25897370e0c5479d79ca4179702c87cce70c596effd3f"

// representativeSource is intentionally the same grammar pressure used by the
// semantic acceptance test: indentation, triple/raw/f/bytes strings, aliases,
// relative imports, decorator call syntax, async, a comprehension, match, and
// a PEP 695 type alias. It is not the old five-line fast-path fixture.
const representativeSource = `# comments and indentation are syntax, not facts
"""triple quoted string"""
from .relay import client
from ... import support
import alpha as beta
@decorator(flag="x")
async def worker(items: list[str]) -> None:
    raw = r"\\n"
    blob = b"\\x00"
    rendered = f"{raw}:{blob!r}"
    values = [call(value) for value in items if value]
    match values:
        case []:
            return
    await client.run()
type Alias = list[str]
worker([])
`

func representativeFrame(tb testing.TB) []byte {
	tb.Helper()
	return canonicalFrame(tb, "request-performance",
		vectorInput{"input-001", "py.project", "pyproject.toml", "requires-python = \"==3.12.0\"\n"},
		vectorInput{"input-002", "py.requirements", "requirements.txt", "attrs==24.2.0\nrequests==2.32.3\n"},
		vectorInput{"input-003", "py.source", "tooling/representative.py", representativeSource},
	)
}

func BenchmarkRepresentativeCandidate(b *testing.B) {
	frame := representativeFrame(b)
	b.ReportAllocs()
	b.SetBytes(int64(len(frame)))
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = Process(frame)
	}
}

func BenchmarkLiteralPinnedCorpusCandidate(b *testing.B) {
	frames := literalPinnedPythonFrames(b)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = Process(frames[index%len(frames)])
	}
}

// restoredParentParseSource is a test-only restoration of the rejected parent
// production line parser. Its source identity is frozen below before use. It is
// intentionally isolated from production so its benchmark remains a causal
// comparison and cannot narrow this candidate's accepted grammar.
func restoredParentParseSource(request Request, item Input, content []byte) ([]Fact, failure) {
	if len(content) == 0 || content[len(content)-1] != '\n' || bytes.Contains(content, []byte{'\r'}) || !asciiPython(content) {
		return nil, failureMalformed
	}
	lines := strings.Split(string(content[:len(content)-1]), "\n")
	result := make([]Fact, 0, len(lines)*2)
	pending := ""
	for lineNumber, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "@") {
			if pending != "" || !dottedPythonName(strings.TrimPrefix(line, "@")) {
				return nil, failureDynamic
			}
			pending = strings.TrimPrefix(line, "@")
			continue
		}
		if module, ok := restoredParentImport(line); ok {
			if pending != "" {
				return nil, failureUnsupported
			}
			result = append(result, Fact{Kind: "python.import.static", InputHandle: item.Handle, RelatedHandle: "-", Subject: item.Path, Predicate: "imports", Value: module, InstanceID: item.Path})
			continue
		}
		if name, kind, ok := restoredParentDefinition(line); ok {
			result = append(result, Fact{Kind: "python.definition", InputHandle: item.Handle, RelatedHandle: "-", Subject: item.Path, Predicate: "defines", Value: kind + ":" + name, InstanceID: request.CompilationUnitID + ":" + name})
			if pending != "" {
				result = append(result, Fact{Kind: "python.decorator", InputHandle: item.Handle, RelatedHandle: "-", Subject: item.Path, Predicate: "decorates", Value: pending + "->" + kind + ":" + name, InstanceID: request.CompilationUnitID + ":" + name})
				pending = ""
			}
			continue
		}
		if call, ok := strings.CutSuffix(line, "()"); ok && dottedPythonName(call) && pending == "" {
			result = append(result, Fact{Kind: "python.call.static", InputHandle: item.Handle, RelatedHandle: "-", Subject: item.Path, Predicate: "calls", Value: call, InstanceID: item.Path + ":" + strconv.Itoa(lineNumber+1)})
			continue
		}
		return nil, failureUnsupported
	}
	if pending != "" {
		return nil, failureUnsupported
	}
	return result, ""
}

func restoredParentImport(line string) (string, bool) {
	if module, found := strings.CutPrefix(line, "import "); found && dottedPythonName(module) {
		return module, true
	}
	from, starts := strings.CutPrefix(line, "from ")
	if !starts {
		return "", false
	}
	module, symbol, found := strings.Cut(from, " import ")
	return module, found && dottedPythonName(module) && identifier(symbol)
}

func restoredParentDefinition(line string) (string, string, bool) {
	for _, definition := range []struct{ prefix, kind string }{{"def ", "function"}, {"class ", "class"}} {
		if value, found := strings.CutPrefix(line, definition.prefix); found {
			name, suffix, split := strings.Cut(value, ":")
			if split && suffix == " pass" {
				if definition.kind == "function" && strings.HasSuffix(name, "()") {
					return strings.TrimSuffix(name, "()"), definition.kind, restoredParentPythonIdentifier(strings.TrimSuffix(name, "()"))
				}
				if definition.kind == "class" && restoredParentPythonIdentifier(name) {
					return name, definition.kind, true
				}
			}
		}
	}
	return "", "", false
}

func dottedPythonName(value string) bool {
	for _, part := range strings.Split(value, ".") {
		if !restoredParentPythonIdentifier(part) {
			return false
		}
	}
	return !strings.Contains(value, "..")
}

func restoredParentPythonIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index := range value {
		c := value[index]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || index > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func asciiPython(value []byte) bool {
	for _, byte := range value {
		if byte != '\n' && (byte < 0x20 || byte > 0x7e) {
			return false
		}
	}
	return true
}

var parentImportExpression = regexp.MustCompile(`^import ([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)$`)
var parentFromExpression = regexp.MustCompile(`^from ([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*) import ([A-Za-z_][A-Za-z0-9_]*)$`)
var parentDecoratorExpression = regexp.MustCompile(`^@([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)$`)
var parentDefinitionExpression = regexp.MustCompile(`^(def|class) ([A-Za-z_][A-Za-z0-9_]*)(?:\(\))?: pass$`)
var parentCallExpression = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\(\)$`)

func restoredParentRegexp(request Request, item Input, content []byte) ([]Fact, failure) {
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	result := make([]Fact, 0, len(lines)*2)
	pending := ""
	for lineNumber, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if matches := parentDecoratorExpression.FindStringSubmatch(line); matches != nil {
			if pending != "" {
				return nil, failureDynamic
			}
			pending = matches[1]
			continue
		}
		if matches := parentImportExpression.FindStringSubmatch(line); matches != nil {
			if pending != "" {
				return nil, failureUnsupported
			}
			result = append(result, Fact{Kind: "python.import.static", InputHandle: item.Handle, RelatedHandle: "-", Subject: item.Path, Predicate: "imports", Value: matches[1], InstanceID: item.Path})
			continue
		}
		if matches := parentFromExpression.FindStringSubmatch(line); matches != nil {
			if pending != "" {
				return nil, failureUnsupported
			}
			result = append(result, Fact{Kind: "python.import.static", InputHandle: item.Handle, RelatedHandle: "-", Subject: item.Path, Predicate: "imports", Value: matches[1], InstanceID: item.Path})
			continue
		}
		if matches := parentDefinitionExpression.FindStringSubmatch(line); matches != nil {
			kind := map[string]string{"def": "function", "class": "class"}[matches[1]]
			name := matches[2]
			result = append(result, Fact{Kind: "python.definition", InputHandle: item.Handle, RelatedHandle: "-", Subject: item.Path, Predicate: "defines", Value: kind + ":" + name, InstanceID: request.CompilationUnitID + ":" + name})
			if pending != "" {
				result = append(result, Fact{Kind: "python.decorator", InputHandle: item.Handle, RelatedHandle: "-", Subject: item.Path, Predicate: "decorates", Value: pending + "->" + kind + ":" + name, InstanceID: request.CompilationUnitID + ":" + name})
				pending = ""
			}
			continue
		}
		if matches := parentCallExpression.FindStringSubmatch(line); matches != nil && pending == "" {
			result = append(result, Fact{Kind: "python.call.static", InputHandle: item.Handle, RelatedHandle: "-", Subject: item.Path, Predicate: "calls", Value: matches[1], InstanceID: item.Path + ":" + strconv.Itoa(lineNumber+1)})
			continue
		}
		return nil, failureUnsupported
	}
	if pending != "" {
		return nil, failureUnsupported
	}
	return result, ""
}

func BenchmarkRestoredParentScanner(b *testing.B) {
	const source = "import beamfall\nfrom relay.client import Client\n@decorators.entry\ndef main(): pass\nmain()\n"
	request, item := Request{CompilationUnitID: "unit-1"}, Input{Handle: "input-003", Path: "src/main.py"}
	content := []byte(source)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		if _, reason := restoredParentParseSource(request, item, content); reason != "" {
			b.Fatal(reason)
		}
	}
}

func BenchmarkRestoredParentRegexp(b *testing.B) {
	const source = "import beamfall\nfrom relay.client import Client\n@decorators.entry\ndef main(): pass\nmain()\n"
	request, item := Request{CompilationUnitID: "unit-1"}, Input{Handle: "input-003", Path: "src/main.py"}
	content := []byte(source)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		if _, reason := restoredParentRegexp(request, item, content); reason != "" {
			b.Fatal(reason)
		}
	}
}

func TestPerformanceRatchets(t *testing.T) {
	requirement := struct{ name string }{name: "PNC-010 restored parent corpus failure linear allocation growth"}
	if requirement.name == "" {
		t.Fatal("missing requirement claim")
	}
	if !performanceRatchetsEnabled() {
		t.Skip("race instrumentation changes allocation and latency measurements")
	}
	frames := literalPinnedPythonFrames(t)
	assertRestoredParentProductionIdentity(t)
	parentFailed := false
	for _, frame := range frames {
		var request Request
		if err := json.Unmarshal(bytes.TrimSuffix(frame, []byte{'\n'}), &request); err != nil {
			t.Fatal(err)
		}
		content, err := base64.StdEncoding.DecodeString(request.Inputs[0].ContentBase64)
		if err != nil {
			t.Fatal(err)
		}
		if _, reason := restoredParentParseSource(request, request.Inputs[0], content); reason != "" {
			parentFailed = true
			break
		}
	}
	if !parentFailed {
		t.Fatal("restored parent alternative accepted every pinned corpus input")
	}
	processCorpus := func() {
		for _, frame := range frames {
			if got := Process(frame); !bytes.Contains(got, []byte(`"status":"CANDIDATE"`)) {
				t.Fatalf("pinned corpus candidate failed: %s", got)
			}
		}
	}
	// Allocation is the stable causal ratchet. Wall time stays in benchmark
	// output only: it is intentionally not a portable test assertion.
	samples := make([]float64, 5)
	for index := range samples {
		samples[index] = testing.AllocsPerRun(1, processCorpus)
		t.Logf("private corpus allocation sample[%d]=%.0f", index, samples[index])
		if samples[index] > 600_000 {
			t.Fatalf("pinned corpus allocation ceiling sample[%d]=%.0f", index, samples[index])
		}
	}
	doubled := testing.AllocsPerRun(1, func() {
		processCorpus()
		processCorpus()
	})
	t.Logf("private doubled corpus allocation sample=%.0f", doubled)
	if doubled > samples[0]*2+1_024 {
		t.Fatalf("corpus allocation scaled superlinearly: once=%.0f twice=%.0f", samples[0], doubled)
	}
}

func assertRestoredParentProductionIdentity(t testing.TB) {
	t.Helper()
	command := exec.Command("git", "show", restoredParentCommit+":internal/analyzerpython/analyzer.go")
	source, err := command.Output()
	if err != nil {
		t.Fatalf("read restored parent production source: %v", err)
	}
	sum := sha256.Sum256(source)
	if got := hex.EncodeToString(sum[:]); got != restoredParentAnalyzerSHA256 {
		t.Fatalf("restored parent production identity=%s", got)
	}
	if !bytes.Contains(source, []byte("func parseSource(request request, item input, content []byte) ([]fact, failure)")) {
		t.Fatal("restored parent source no longer contains the production parser")
	}
}
