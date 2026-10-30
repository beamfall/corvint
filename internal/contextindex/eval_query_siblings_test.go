package contextindex

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// evalSiblingCandidate builds one same-path symbol candidate whose full
// support is its name support plus the given window words.
func evalSiblingCandidate(path, name, kind string, score int, queryTerms map[string]struct{}, window ...string) evalSymbolCandidate {
	prepared := evalPreparedSymbol{
		symbol:      Symbol{Path: path, Name: name, Kind: kind},
		name:        terms(name),
		nameParts:   evalOrderedTerms(name),
		compactName: compactText(name),
	}
	candidate := evalSymbolCandidate{score: score, id: path + ":" + name, symbol: prepared}
	candidate.support = evalNameSupport(candidate, queryTerms)
	for _, word := range window {
		candidate.support[word] = struct{}{}
	}
	return candidate
}

func evalSiblingIDs(candidates []evalSymbolCandidate) []string {
	ids := make([]string, len(candidates))
	for index, candidate := range candidates {
		ids[index] = candidate.id
	}
	return ids
}

// evalAdmissionIndex is one source with its declarations, for the identity
// admission's relation checks. A declaration is name@line or name@line-end.
func evalAdmissionIndex(path, text string, declarations ...string) *Index {
	index := new(Index)
	index.Sources = map[string]Source{path: {Path: path, Data: []byte(text)}}
	index.Markers = map[string][]Marker{}
	for _, declaration := range declarations {
		name, span, _ := strings.Cut(declaration, "@")
		first, last, _ := strings.Cut(span, "-")
		line, _ := strconv.Atoi(first)
		end, _ := strconv.Atoi(last)
		index.Symbols = append(index.Symbols, Symbol{Path: path, Name: name, Kind: "func", Line: line, EndLine: end})
	}
	return index
}

// evalAdmissionCandidate is evalSiblingCandidate carrying the declaration's
// line span from the index, so the relation checks find its extent.
func evalAdmissionCandidate(index *Index, name string, score int, queryTerms map[string]struct{}, window ...string) evalSymbolCandidate {
	for _, symbol := range index.Symbols {
		if symbol.Name == name {
			candidate := evalSiblingCandidate(symbol.Path, name, symbol.Kind, score, queryTerms, window...)
			candidate.symbol.symbol = symbol
			return candidate
		}
	}
	panic("no declaration " + name)
}

const evalActiveHelpSource = `package cobra

// AppendActiveHelp adds the string; see GetActiveHelpConfig.
func AppendActiveHelp(compArray []string, activeHelpStr string) []string {
	return append(compArray, activeHelpStr)
}

// GetActiveHelpConfig returns the value of the ActiveHelp environment variable.
func GetActiveHelpConfig(cmd string) string {
	return os.Getenv(activeHelpEnvVar(cmd))
}

func activeHelpEnvVar(name string) string {
	return name + "_ACTIVE_HELP"
}

func TestGetActiveHelpConfig() {}
`

func evalActiveHelpIndex() *Index {
	return evalAdmissionIndex("active_help.go", evalActiveHelpSource,
		"AppendActiveHelp@4", "GetActiveHelpConfig@9", "activeHelpEnvVar@13", "TestGetActiveHelpConfig@17")
}

// The blind-v6 forbidden-hit class (decision 0307): a sibling admitted beside
// the target because the two share a file and a context window. GPK-V0-093:
// beside a named subject a sibling is admitted only with a demonstrated
// relation (same declaration, reference edge, paired test, authority link).
func TestEvalAdmitSiblingsRequiresDemonstratedRelation(t *testing.T) {
	admit := func(index *Index, query string, candidates ...evalSymbolCandidate) []string {
		return evalSiblingIDs(evalAdmitSiblings(index, candidates, terms(query), query))
	}
	t.Run("GPK-V0-093 shared-window sibling is refused", func(t *testing.T) {
		query := "read active help configuration from the environment"
		index := evalActiveHelpIndex()
		got := admit(index, query,
			evalAdmissionCandidate(index, "AppendActiveHelp", 235, terms(query), "environment"),
			evalAdmissionCandidate(index, "GetActiveHelpConfig", 232, terms(query), "environment", "read"))
		if want := []string{"active_help.go:GetActiveHelpConfig"}; !slices.Equal(got, want) {
			t.Fatalf("kept %v, want %v: `config` answers \"configuration\", so AppendActiveHelp's {active, help} is a strict subset and nothing relates it", got, want)
		}
	})
	t.Run("GPK-V0-093 a window word is not a relation", func(t *testing.T) {
		query := "read active help configuration from the environment"
		index := evalActiveHelpIndex()
		got := admit(index, query,
			evalAdmissionCandidate(index, "GetActiveHelpConfig", 232, terms(query), "environment"),
			evalAdmissionCandidate(index, "AppendActiveHelp", 235, terms(query), "read"))
		if want := []string{"active_help.go:GetActiveHelpConfig"}; !slices.Equal(got, want) {
			t.Fatalf("kept %v, want %v: \"read\" in the sibling's window is shared vocabulary, not a relation (the 2026-10-01 rule admitted it)", got, want)
		}
	})
	t.Run("GPK-V0-093 a name in a comment is not a reference", func(t *testing.T) {
		// The sibling's doc comment names the subject, and the subject's code
		// names no sibling: neither direction is a reference edge.
		query := "read active help configuration from the environment"
		index := evalActiveHelpIndex()
		got := admit(index, query,
			evalAdmissionCandidate(index, "GetActiveHelpConfig", 232, terms(query)),
			evalAdmissionCandidate(index, "AppendActiveHelp", 200, terms(query)))
		if len(got) != 1 {
			t.Fatalf("kept %v, want only the subject", got)
		}
	})
	t.Run("GPK-V0-093 a declaration the subject calls is admitted", func(t *testing.T) {
		query := "which active help config value does a program read for a command"
		index := evalActiveHelpIndex()
		got := admit(index, query,
			evalAdmissionCandidate(index, "GetActiveHelpConfig", 232, terms(query)),
			evalAdmissionCandidate(index, "AppendActiveHelp", 220, terms(query), "command"),
			evalAdmissionCandidate(index, "activeHelpEnvVar", 210, terms(query), "program"))
		if want := []string{"active_help.go:GetActiveHelpConfig", "active_help.go:activeHelpEnvVar"}; !slices.Equal(got, want) {
			t.Fatalf("kept %v, want %v: the subject's code calls activeHelpEnvVar", got, want)
		}
	})
	t.Run("GPK-V0-093 a paired test is admitted", func(t *testing.T) {
		query := "read active help configuration"
		index := evalActiveHelpIndex()
		got := admit(index, query,
			evalAdmissionCandidate(index, "GetActiveHelpConfig", 232, terms(query)),
			evalAdmissionCandidate(index, "TestGetActiveHelpConfig", 200, terms(query)))
		if len(got) != 2 {
			t.Fatalf("kept %v, want both: TestGetActiveHelpConfig is the subject's paired test", got)
		}
	})
	t.Run("GPK-V0-093 an authority link is admitted", func(t *testing.T) {
		query := "read active help configuration from the environment"
		index := evalActiveHelpIndex()
		index.Markers["feature:active-help"] = []Marker{{Path: "active_help.go", Line: 4}, {Path: "active_help.go", Line: 9}}
		got := admit(index, query,
			evalAdmissionCandidate(index, "GetActiveHelpConfig", 232, terms(query)),
			evalAdmissionCandidate(index, "AppendActiveHelp", 200, terms(query)))
		if len(got) != 2 {
			t.Fatalf("kept %v, want both: one project marker id marks both declarations", got)
		}
	})
	t.Run("GPK-V0-093 an unloaded source demonstrates nothing", func(t *testing.T) {
		query := "which active help config value does a program read"
		index := evalActiveHelpIndex()
		index.Sources["active_help.go"] = Source{Path: "active_help.go"}
		got := admit(index, query,
			evalAdmissionCandidate(index, "GetActiveHelpConfig", 232, terms(query)),
			evalAdmissionCandidate(index, "activeHelpEnvVar", 210, terms(query)))
		if len(got) != 1 {
			t.Fatalf("kept %v, want only the subject: without source text no relation is demonstrated", got)
		}
	})
	t.Run("GPK-V0-093 identical support keeps both", func(t *testing.T) {
		query := "generate Bash shell completion output"
		index := evalAdmissionIndex("bash_completions.go", "package cobra\n", "GenBashCompletion@1", "GenBashCompletionFile@1")
		got := admit(index, query,
			evalAdmissionCandidate(index, "GenBashCompletion", 300, terms(query)),
			evalAdmissionCandidate(index, "GenBashCompletionFile", 290, terms(query)))
		if len(got) != 2 {
			t.Fatalf("kept %v, want both: nothing in the task tells them apart, so both are subjects", got)
		}
	})
	t.Run("GPK-V0-093 a sibling the task names stays", func(t *testing.T) {
		query := "why does AppendActiveHelp read the active help configuration"
		index := evalActiveHelpIndex()
		got := admit(index, query,
			evalAdmissionCandidate(index, "GetActiveHelpConfig", 232, terms(query), "read"),
			evalAdmissionCandidate(index, "AppendActiveHelp", 200, terms(query)))
		if len(got) != 2 {
			t.Fatalf("kept %v, want both: the task writes the sibling's name", got)
		}
	})
	t.Run("GPK-V0-093 different paths are never compared", func(t *testing.T) {
		query := "read active help configuration from the environment"
		got := admit(new(Index), query,
			evalSiblingCandidate("active_help.go", "GetActiveHelpConfig", "func", 232, terms(query)),
			evalSiblingCandidate("other.go", "ActiveHelp", "type", 100, terms(query)))
		if len(got) != 2 {
			t.Fatalf("kept %v, want both", got)
		}
	})
}

const evalViewsSource = `class View:
    """Call :meth:` + "`as_view`" + ` to create a view function."""

    def dispatch_request(self):
        raise NotImplementedError()

    def as_view(cls, name):
        def view(**kwargs):
            return self.dispatch_request(**kwargs)
        return view


class MethodView(View):
    """app.add_url_rule("/counter", view_func=CounterAPI.as_view("counter"))"""

    def dispatch_request(self, **kwargs):
        meth = getattr(self, request.method.lower(), None)
        return meth(**kwargs)


def async_to_sync(func):
    return func


def ensure_sync(func):
    return async_to_sync(func)
`

// Python siblings from the flask reconstruction: the decision 0307 hit
// views.py:as_view and the development answer app.py:ensure_sync.
func TestEvalAdmitSiblingsPythonRelations(t *testing.T) {
	views := func() *Index {
		return evalAdmissionIndex("views.py", evalViewsSource,
			"View@1-10", "as_view@7-10", "MethodView@13-18", "dispatch_request@16-18", "async_to_sync@21-22", "ensure_sync@25-26")
	}
	admit := func(index *Index, query string, candidates ...evalSymbolCandidate) []string {
		return evalSiblingIDs(evalAdmitSiblings(index, candidates, terms(query), query))
	}
	t.Run("GPK-V0-093 a docstring and a caller of another concept are not relations", func(t *testing.T) {
		query := "dispatch the request to the matching HTTP method handler of a class based view"
		index := views()
		got := admit(index, query,
			evalAdmissionCandidate(index, "MethodView", 300, terms(query)),
			evalAdmissionCandidate(index, "as_view", 250, terms(query)),
			evalAdmissionCandidate(index, "View", 240, terms(query)))
		if want := []string{"views.py:MethodView", "views.py:View"}; !slices.Equal(got, want) {
			t.Fatalf("kept %v, want %v: MethodView's code names View as its base; as_view appears only in its docstring", got, want)
		}
	})
	t.Run("GPK-V0-093 a method inside the subject is the same declaration", func(t *testing.T) {
		query := "class based method view"
		index := views()
		got := admit(index, query,
			evalAdmissionCandidate(index, "MethodView", 300, terms(query)),
			evalAdmissionCandidate(index, "dispatch_request", 100, terms(query)))
		if len(got) != 2 {
			t.Fatalf("kept %v, want both: dispatch_request lies inside MethodView", got)
		}
	})
	t.Run("GPK-V0-093 a wrapper over the same concept is admitted", func(t *testing.T) {
		query := "convert an async view function to synchronous wsgi execution"
		index := views()
		got := admit(index, query,
			evalAdmissionCandidate(index, "async_to_sync", 300, terms(query)),
			evalAdmissionCandidate(index, "ensure_sync", 280, terms(query)))
		if len(got) != 2 {
			t.Fatalf("kept %v, want both: ensure_sync calls async_to_sync and its {synchronous} lies within the subject's", got)
		}
	})
}

func TestEvalBlankCommentsAndStrings(t *testing.T) {
	t.Run("GPK-V0-093 comments and literals are blanked, lines kept", func(t *testing.T) {
		got := evalBlankCommentsAndStrings("a.go", "x := \"as_view\" // as_view\n/* as_view */ y(as_view)")
		if strings.Count(got, "as_view") != 1 || strings.Count(got, "\n") != 1 || !evalUsesIdentifier(got, "as_view") {
			t.Fatalf("blanked %q", got)
		}
		python := evalBlankCommentsAndStrings("a.py", "'''as_view\n'''  # as_view\nself.as_view()")
		if strings.Count(python, "as_view") != 1 || strings.Count(python, "\n") != 2 {
			t.Fatalf("blanked %q", python)
		}
		if evalUsesIdentifier("ensure_sync_more()", "ensure_sync") {
			t.Fatal("a longer identifier counted as a use")
		}
	})
}

func TestEvalNameSupportAnswersPrefixes(t *testing.T) {
	t.Run("GPK-V0-040 four-byte prefixes in either direction", func(t *testing.T) {
		queryTerms := terms("convert an async view to synchronous execution with the config")
		candidate := evalSiblingCandidate("app.py", "ensure_sync", "method", 1, queryTerms)
		support := evalNameSupport(candidate, queryTerms)
		if _, ok := support["synchronous"]; !ok {
			t.Fatalf("`sync` does not answer \"synchronous\": %v", support)
		}
		short := evalSiblingCandidate("app.py", "to_env", "func", 1, queryTerms)
		if support := evalNameSupport(short, terms("read the environment")); len(support) != 0 {
			t.Fatalf("a three-byte part answered a prefix: %v", support)
		}
	})
}
