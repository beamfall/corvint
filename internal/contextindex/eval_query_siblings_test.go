package contextindex

import (
	"strconv"
	"strings"
)

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
