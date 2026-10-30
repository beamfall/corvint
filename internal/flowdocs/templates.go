package flowdocs

import (
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// templateDeclarations tokenizes actual HTML tags and quoted attributes. It
// skips comments and raw script/style bodies instead of scanning their text as
// executable Angular bindings. Expressions are identified but never rendered.
func templateDeclarations(s contextindex.Source) ([]declaration, []string) {
	text := string(s.Data)
	d := declaration{name: "template", line: 1, end: 1}
	found := false
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "<!--") {
			end := strings.Index(text[i+4:], "-->")
			if end < 0 {
				break
			}
			i += end + 7
			continue
		}
		if text[i] != '<' {
			i++
			continue
		}
		start := i
		i++
		if i >= len(text) {
			break
		}
		if text[i] == '/' || text[i] == '!' || text[i] == '?' {
			for i < len(text) && text[i] != '>' {
				i++
			}
			continue
		}
		tagStart := i
		for i < len(text) && (text[i] >= 'a' && text[i] <= 'z' || text[i] >= 'A' && text[i] <= 'Z' || text[i] == '-') {
			i++
		}
		if i == tagStart {
			continue
		}
		tag := strings.ToLower(text[tagStart:i])
		attrs := map[string]string{}
		attrLines := map[string]int{}
		attrEnds := map[string]int{}
		for i < len(text) && text[i] != '>' {
			for i < len(text) && (unicode.IsSpace(rune(text[i])) || text[i] == '/') {
				i++
			}
			a := i
			for i < len(text) && !unicode.IsSpace(rune(text[i])) && text[i] != '=' && text[i] != '>' {
				i++
			}
			if i == a {
				if i < len(text) && text[i] != '>' {
					i++
				}
				continue
			}
			key := strings.ToLower(text[a:i])
			attributeLine := strings.Count(text[:a], "\n") + 1
			for i < len(text) && unicode.IsSpace(rune(text[i])) {
				i++
			}
			if i >= len(text) || text[i] != '=' {
				continue
			}
			i++
			for i < len(text) && unicode.IsSpace(rune(text[i])) {
				i++
			}
			if i >= len(text) {
				break
			}
			if text[i] != '\'' && text[i] != '"' {
				for i < len(text) && !unicode.IsSpace(rune(text[i])) && text[i] != '>' {
					i++
				}
				continue
			}
			q := text[i]
			i++
			v := i
			for i < len(text) && text[i] != q {
				i++
			}
			if i >= len(text) {
				break
			}
			attrs[key] = text[v:i]
			attrLines[key] = attributeLine
			attrEnds[key] = strings.Count(text[:i], "\n") + 1
			i++
		}
		if i < len(text) {
			i++
		}
		if tag == "script" || tag == "style" {
			end := strings.Index(strings.ToLower(text[i:]), "</"+tag)
			if end < 0 {
				break
			}
			i += end
			continue
		}
		line := strings.Count(text[:start], "\n") + 1
		for _, name := range []string{"ng-controller", "ng-if", "ng-show", "ng-hide", "ng-switch", "ng-repeat", "ng-click", "ng-submit", "ng-model", "ng-view"} {
			if _, ok := attrs[name]; !ok {
				continue
			}
			if !found {
				d.line = line
				found = true
			}
			d.end = line
			key := name
			if id := attrs["id"]; safeLiteral.MatchString(id) {
				key += ":" + id
			}
			d.observations = append(d.observations, observation{key: "behavior:" + key, kind: "behavior", section: "functional-overview", text: "Source template declares " + name + " binding; expression values, accepted variations and runtime behavior are unresolved.", line: attrLines[name], end: attrEnds[name]})
		}
	}
	if !found {
		return nil, []string{"template has no supported literal Angular bindings"}
	}
	return []declaration{d}, nil
}
