package typescript

import (
	"errors"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// PlaywrightUngreppedWrapperTemplate is the one wrapper config an ungrepped listing may name
// instead of the bound config (TJAA-V0-124): it imports the bound config from its own directory
// and removes only the top-level and per-project grep and grepInvert. CONFIG stands for "./"
// and the bound config's file name, with or without its extension. Comments, whitespace, either
// quote style, semicolons and trailing commas may differ; any other token refuses the wrapper.
const PlaywrightUngreppedWrapperTemplate = `import config from './CONFIG';
export default {
  ...config,
  grep: undefined,
  grepInvert: undefined,
  projects: (config.projects ?? []).map(({ grep, grepInvert, ...project }) => project),
};
`

// playwrightWrapperMaxBytes bounds the wrapper config read; the template is far shorter.
const playwrightWrapperMaxBytes = 4096

// CheckPlaywrightUngreppedWrapper refuses wrapper unless it is a canonical TypeScript/JavaScript
// source other than configPath in configPath's directory, so Playwright resolves testDir and every
// other config-relative path exactly as for the bound config, and its bytes in the worktree are
// PlaywrightUngreppedWrapperTemplate for configPath (TJAA-V0-124).
func CheckPlaywrightUngreppedWrapper(root, configPath, wrapper string) error {
	if !affected.ValidRelativePath(wrapper) || !hasSourceExtension(wrapper) {
		return errors.New("ungrepped wrapper config path is not canonical TypeScript/JavaScript source")
	}
	if wrapper == configPath || path.Dir(wrapper) != path.Dir(configPath) {
		return errors.New("ungrepped wrapper config " + wrapper + " is not another file in the directory of " + configPath)
	}
	body, err := affected.ReadSource(root, wrapper)
	if err != nil {
		return errors.New("ungrepped wrapper config " + wrapper + " is unreadable")
	}
	if len(body) > playwrightWrapperMaxBytes || !utf8.Valid(body) {
		return errors.New("ungrepped wrapper config " + wrapper + " is not the documented wrapper")
	}
	if !playwrightWrapperMatches(string(body), configPath) {
		return errors.New("ungrepped wrapper config " + wrapper + " is not the documented wrapper that imports " + configPath + " and removes only grep and grepInvert")
	}
	return nil
}

// playwrightWrapperMatches reports whether body is the wrapper template for configPath, compared
// token by token.
func playwrightWrapperMatches(body, configPath string) bool {
	clean, err := stripComments(body, false)
	if err != nil {
		return false
	}
	got, ok := playwrightWrapperTokens(clean)
	if !ok {
		return false
	}
	want, _ := playwrightWrapperTokens(PlaywrightUngreppedWrapperTemplate)
	if len(got) != len(want) {
		return false
	}
	base := path.Base(configPath)
	specifiers := map[string]bool{`"./` + base + `"`: true, `"./` + strings.TrimSuffix(base, path.Ext(base)) + `"`: true}
	for index := range want {
		if want[index] == `"./CONFIG"` {
			if !specifiers[got[index]] {
				return false
			}
		} else if got[index] != want[index] {
			return false
		}
	}
	return true
}

// playwrightWrapperTokens splits comment-free text into identifiers, string literals in double
// quotes, `...`, `??`, `=>` and single punctuation, dropping semicolons and a comma before a
// closing bracket. A backslash, template literal or any other character fails the split.
func playwrightWrapperTokens(text string) ([]string, bool) {
	var tokens []string
	for index := 0; index < len(text); {
		character := text[index]
		switch {
		case character == ' ' || character == '\t' || character == '\n' || character == '\r':
			index++
		case character == '_' || character == '$' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9':
			start := index
			for index < len(text) && (text[index] == '_' || text[index] == '$' || text[index] >= 'a' && text[index] <= 'z' || text[index] >= 'A' && text[index] <= 'Z' || text[index] >= '0' && text[index] <= '9') {
				index++
			}
			tokens = append(tokens, text[start:index])
		case character == '\'' || character == '"':
			end := strings.IndexByte(text[index+1:], character)
			if end < 0 {
				return nil, false
			}
			content := text[index+1 : index+1+end]
			if strings.ContainsAny(content, "\\\n\r'\"") {
				return nil, false
			}
			tokens = append(tokens, `"`+content+`"`)
			index += end + 2
		case strings.HasPrefix(text[index:], "..."):
			tokens = append(tokens, "...")
			index += 3
		case strings.HasPrefix(text[index:], "??"), strings.HasPrefix(text[index:], "=>"):
			tokens = append(tokens, text[index:index+2])
			index += 2
		case strings.IndexByte("{}()[],:.;=", character) >= 0:
			tokens = append(tokens, text[index:index+1])
			index++
		default:
			return nil, false
		}
	}
	kept := tokens[:0]
	for index, token := range tokens {
		if token == ";" || token == "," && index+1 < len(tokens) && strings.Contains("})]", tokens[index+1]) {
			continue
		}
		kept = append(kept, token)
	}
	return kept, true
}
