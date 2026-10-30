package changewitness

import (
	"reflect"
	"testing"
)

const demoPath = "internal/demo/demo.go"

// demoSource definition spans: Alpha 4-6, Beta 8-10, Gamma 13-13, Delta 14-14.
const demoSource = `package demo

// Alpha does a.
func Alpha() int {
	return 1
}

func Beta() int {
	return 2
}

type (
	Gamma struct{}
	Delta int
)
`

func demoInput(span string, hunks ...Hunk) Input {
	return Input{
		Disposition: "linked",
		IntentPath:  "docs/specs/demo.md",
		TargetSpan:  []byte(span),
		BasePresent: true,
		BaseSpan:    []byte(span),
		Sources: []Source{
			{Path: demoPath, BlobOID: "blob-demo", Bytes: []byte(demoSource)},
			{Path: "internal/other/other.go", BlobOID: "blob-other", Bytes: []byte("package other\n\nfunc Beta() {}\n")},
			{Path: "docs/specs/demo.md", BlobOID: "blob-spec", Bytes: []byte("func Alpha() {}\n")},
		},
		Hunks: hunks,
	}
}

func changedHunk(id, path string, start, count int) Hunk {
	return Hunk{ID: id, Path: path, Disposition: "supported", Changed: []LineRange{{start, count}}}
}

func alphaWitness(hunkID string) Witness {
	return Witness{Identifier: "Alpha", Path: demoPath, BlobOID: "blob-demo", StartLine: 4, EndLine: 6, HunkID: hunkID}
}

type vector struct {
	name  string
	input Input
	want  Result
}

func runVectors(t *testing.T, vectors []vector) {
	t.Helper()
	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			if got := Evaluate(v.input); !reflect.DeepEqual(got, v.want) {
				t.Fatalf("Evaluate = %+v, want %+v", got, v.want)
			}
		})
	}
}

func TestLinkedBaseStableObligationIsWitnessed(t *testing.T) {
	runVectors(t, []vector{
		{"witness", demoInput("`Alpha` returns one", changedHunk("h1", demoPath, 5, 1)),
			Result{Outcome: Witnessed, Witnesses: []Witness{alphaWitness("h1")},
				Diagnostics: []Diagnostic{{IdentifierUnresolved, 2}}}},
		{"unknown obligation is out of scope", func() Input {
			in := demoInput("Alpha", changedHunk("h1", demoPath, 5, 1))
			in.Disposition = "unknown"
			return in
		}(), Result{Outcome: Abstained, Reason: ReasonNotLinked}},
	})
}

func TestCallerAuthoredRequirementIsWithheld(t *testing.T) {
	changed := demoInput("Alpha returns two", changedHunk("h2", demoPath, 5, 1), changedHunk("h1", demoPath, 5, 1))
	changed.BaseSpan = []byte("Alpha returns one")
	absent := demoInput("Alpha", changedHunk("h1", demoPath, 5, 1))
	absent.BasePresent = false
	absent.BaseSpan = []byte("Alpha")
	runVectors(t, []vector{
		{"span bytes differ at base", changed,
			Result{Outcome: Withheld, Reason: SelfAuthoredObligation, HunkIDs: []string{"h1", "h2"}}},
		{"intent path or ID absent at base", absent,
			Result{Outcome: Withheld, Reason: SelfAuthoredObligation, HunkIDs: []string{"h1"}}},
	})
}

func TestIdentifierResolutionAbstainsOnZeroOrManyDefinitions(t *testing.T) {
	runVectors(t, []vector{
		{"ambiguous only", demoInput("Beta", changedHunk("h1", demoPath, 9, 1)),
			Result{Outcome: Abstained, Reason: ReasonNoNamedIdentifier, Diagnostics: []Diagnostic{{IdentifierAmbiguous, 1}}}},
		{"ambiguous and qualifying", demoInput("Alpha and Beta", changedHunk("h1", demoPath, 5, 1)),
			Result{Outcome: Witnessed, Witnesses: []Witness{alphaWitness("h1")},
				Diagnostics: []Diagnostic{{IdentifierAmbiguous, 1}, {IdentifierUnresolved, 1}}}},
		{"no token resolves", demoInput("nothing here", changedHunk("h1", demoPath, 5, 1)),
			Result{Outcome: Abstained, Reason: ReasonNoNamedIdentifier, Diagnostics: []Diagnostic{{IdentifierUnresolved, 2}}}},
		// A bare "_" is not a Go identifier: it must never itself become a
		// counted token (no diagnostic at all), regardless of the surrounding
		// word-splitting.
		{"bare underscore is not a token", demoInput("_", changedHunk("h1", demoPath, 5, 1)),
			Result{Outcome: Abstained, Reason: ReasonNoNamedIdentifier, Diagnostics: []Diagnostic{}}},
		// A leading-underscore word such as "_helper" does start an
		// identifier and, being undefined here, counts as unresolved.
		{"leading underscore starts an identifier", demoInput("_helper", changedHunk("h1", demoPath, 5, 1)),
			Result{Outcome: Abstained, Reason: ReasonNoNamedIdentifier, Diagnostics: []Diagnostic{{IdentifierUnresolved, 1}}}},
	})
}

func TestMaterialIntersectionExcludesEmptyAndWhitespaceHunks(t *testing.T) {
	whitespace := changedHunk("h1", demoPath, 5, 1)
	whitespace.WhitespaceOnly = true
	runVectors(t, []vector{
		{"hunk touches another definition", demoInput("Alpha", changedHunk("h1", demoPath, 9, 1)),
			Result{Outcome: Abstained, Reason: ReasonNoIntersection, Diagnostics: []Diagnostic{}}},
		{"same lines in another path", demoInput("Alpha", changedHunk("h1", "internal/other/other.go", 5, 1)),
			Result{Outcome: Abstained, Reason: ReasonNoIntersection, Diagnostics: []Diagnostic{}}},
		{"zero-line range", demoInput("Alpha", changedHunk("h1", demoPath, 5, 0)),
			Result{Outcome: Abstained, Reason: ReasonNoIntersection, Diagnostics: []Diagnostic{}}},
		{"whitespace-only hunk", demoInput("Alpha", whitespace),
			Result{Outcome: Abstained, Reason: ReasonNoIntersection, Diagnostics: []Diagnostic{}}},
		{"grouped sibling edit", demoInput("Delta", changedHunk("h1", demoPath, 13, 1)),
			Result{Outcome: Abstained, Reason: ReasonNoIntersection, Diagnostics: []Diagnostic{}}},
		{"grouped member edit", demoInput("Delta", changedHunk("h1", demoPath, 14, 1)),
			Result{Outcome: Witnessed, Witnesses: []Witness{{Identifier: "Delta", Path: demoPath, BlobOID: "blob-demo", StartLine: 14, EndLine: 14, HunkID: "h1"}},
				Diagnostics: []Diagnostic{}}},
	})
}

func TestIntentPathAndUnsupportedHunksNeverWitness(t *testing.T) {
	intent := demoInput("Alpha", changedHunk("h1", demoPath, 5, 1))
	intent.IntentPath = demoPath
	unsupported := changedHunk("h1", demoPath, 5, 1)
	unsupported.Disposition = "unknown"
	runVectors(t, []vector{
		{"hunk inside the intent path", intent,
			Result{Outcome: Abstained, Reason: ReasonIneligibleHunk, Diagnostics: []Diagnostic{}}},
		{"hunk not supported", demoInput("Alpha", unsupported),
			Result{Outcome: Abstained, Reason: ReasonIneligibleHunk, Diagnostics: []Diagnostic{}}},
	})
}

func TestResolverBoundaryAbstainsOutsideParsedGo(t *testing.T) {
	unparsed := demoInput("Alpha", changedHunk("h1", demoPath, 5, 1))
	unparsed.Sources = append(unparsed.Sources, Source{Path: "internal/broken/broken.go", BlobOID: "blob-broken", Bytes: []byte("package broken\nfunc (")})
	runVectors(t, []vector{
		{"any Go source refused by the grammar", unparsed,
			Result{Outcome: Abstained, Reason: ReasonSourceUnparsed}},
		{"hunk in another language", demoInput("Alpha", changedHunk("h1", "src/alpha.py", 1, 3)),
			Result{Outcome: Abstained, Reason: ReasonNoIntersection, Diagnostics: []Diagnostic{{LanguageUnsupported, 1}}}},
	})
}

// TestNonGoSuffixIsNotParsedAsGo pins isGoPath to an exact ".go" suffix: a
// path that merely contains ".go", such as "x.go.txt", must be skipped by the
// resolver rather than fed to go/parser (CWR-V0-014).
func TestNonGoSuffixIsNotParsedAsGo(t *testing.T) {
	in := demoInput("Alpha", changedHunk("h1", demoPath, 5, 1))
	in.Sources = append(in.Sources, Source{Path: "internal/other/x.go.txt", BlobOID: "blob-notgo", Bytes: []byte("not valid go source")})
	got := Evaluate(in)
	if got.Outcome != Witnessed {
		t.Fatalf("Evaluate = %+v, want Witnessed (a .go.txt source must not be parsed as Go)", got)
	}
}

func TestEvaluationIsInputOrderIndependent(t *testing.T) {
	forward := demoInput("Delta Alpha", changedHunk("h2", demoPath, 14, 1), changedHunk("h1", demoPath, 4, 3))
	reversed := demoInput("Delta Alpha", changedHunk("h1", demoPath, 4, 3), changedHunk("h2", demoPath, 14, 1))
	reversed.Sources[0], reversed.Sources[2] = reversed.Sources[2], reversed.Sources[0]
	first, second := Evaluate(forward), Evaluate(reversed)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("order changed the result: %+v vs %+v", first, second)
	}
	if len(first.Witnesses) != 2 || first.Witnesses[0].Identifier != "Alpha" {
		t.Fatalf("witnesses = %+v, want Alpha then Delta", first.Witnesses)
	}
}

func TestDefinitionSpansIgnoreLineDirectives(t *testing.T) {
	for _, tc := range []struct {
		name string
		line int
		want string
	}{
		{"physical definition", 4, Witnessed},
		{"remapped unrelated definition", 8, Abstained},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := demoInput("Alpha", changedHunk("h1", demoPath, tc.line, 1))
			in.Sources = []Source{{Path: demoPath, BlobOID: "blob-lines", Bytes: []byte("package demo\n//line virtual.go:7\nfunc Alpha() int {\nreturn 1\n}\n\nfunc Other() int {\nreturn 2\n}\n")}}
			got := Evaluate(in)
			if got.Outcome != tc.want {
				t.Fatalf("Evaluate = %+v, want %s", got, tc.want)
			}
			if got.Outcome == Witnessed && (got.Witnesses[0].StartLine != 3 || got.Witnesses[0].EndLine != 5) {
				t.Fatalf("definition span = %+v, want physical lines 3-5", got.Witnesses[0])
			}
		})
	}
}
