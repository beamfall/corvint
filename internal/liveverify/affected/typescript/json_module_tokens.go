package typescript

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// jsonModuleTokens returns the sorted, unique path-shaped tokens of every JSON
// module the module at owner imports by a relative specifier (TJAA-V0-622).
// A JSON module is data the importing module reads at run time, and a string
// inside it (a `"artifact"` member naming a fixture) is a path the module may
// then open, so each such string is a candidate read of the importer, as a
// literal of its own text would be. Strings and member names of a document
// that parses are tokenized after JSON decoding; a module that is not valid
// JSON keeps every path-shaped token of its escape-decoded text instead, so
// an unparsable document widens rather than narrows. A module that cannot be
// read raises the unreadable-source frontier at the module; one that does not
// exist is left to the relative-import frontier the resolver raises. Only one
// level is read: a JSON module's own imports are not a notion JSON has.
func jsonModuleTokens(root *affected.Source, owner string, refs []string, frontier frontierSet) []string {
	tokens := map[string]bool{}
	for _, ref := range refs {
		clean := strings.SplitN(strings.SplitN(ref, "?", 2)[0], "#", 2)[0]
		if !strings.HasPrefix(clean, ".") || !strings.HasSuffix(strings.ToLower(clean), ".json") {
			continue
		}
		relative := path.Clean(path.Join(path.Dir(owner), clean))
		if relative == "." || relative == ".." || strings.HasPrefix(relative, "../") {
			continue
		}
		body, err := root.Read(relative)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			frontier.raise(FrontierUnreadableSource, relative)
			continue
		}
		for _, token := range jsonPathTokens(body) {
			tokens[token] = true
		}
	}
	if len(tokens) == 0 {
		return nil
	}
	values := make([]string, 0, len(tokens))
	for token := range tokens {
		values = append(values, token)
	}
	sort.Strings(values)
	return values
}

// jsonPathTokens is the path lexicon over a JSON document's strings and member
// names, or over its escape-decoded text when it does not parse.
func jsonPathTokens(body []byte) []string {
	tokens := map[string]bool{}
	add := func(text string) {
		for _, token := range pathLexicon.FindAllString(text, -1) {
			if pathShaped(token) {
				tokens[token] = true
			}
		}
	}
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		decoded, _ := decodeStringEscapes(string(body))
		add(decoded)
	} else {
		var walk func(value any)
		walk = func(value any) {
			switch v := value.(type) {
			case string:
				add(v)
			case []any:
				for _, item := range v {
					walk(item)
				}
			case map[string]any:
				for key, item := range v {
					add(key)
					walk(item)
				}
			}
		}
		walk(document)
	}
	values := make([]string, 0, len(tokens))
	for token := range tokens {
		values = append(values, token)
	}
	sort.Strings(values)
	return values
}

// mergeDataTokens adds a unit's JSON module tokens to its path tokens, keeping
// the AFP-V0-021 bound: a unit over maxPathTokens keeps none and is marked
// PathTokensBounded, as its own literals would mark it.
func mergeDataTokens(unit *affected.Unit, data []string) {
	if len(data) == 0 || unit.PathTokensBounded {
		return
	}
	merged := map[string]bool{}
	for _, token := range unit.PathTokens {
		merged[token] = true
	}
	for _, token := range data {
		merged[token] = true
	}
	if len(merged) > maxPathTokens {
		unit.PathTokens, unit.PathTokensBounded = nil, true
		return
	}
	unit.PathTokens = sortedKeys(merged)
}
