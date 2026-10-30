package changee2e

import (
	"context"
	"path"
	"regexp"
	"sort"
	"strings"
)

// maxAdopterConfigBytes bounds the adopter Playwright config Prove reads (CCE-V0-049).
const maxAdopterConfigBytes = 256 << 10

// adopterConfigNames are Playwright's default config file names, in its lookup order.
var adopterConfigNames = []string{"playwright.config.ts", "playwright.config.js", "playwright.config.mts", "playwright.config.mjs", "playwright.config.cts", "playwright.config.cjs"}

// runnerConfig records how the prove config a hosted test runs under differs from the adopter's
// own Playwright config at head (CCE-V0-049). The adopter config is read as text and never run.
type runnerConfig struct {
	Applied             string            `json:"applied"` // always the prove config
	AdopterConfig       string            `json:"adopter_config,omitempty"`
	AdopterConfigSHA256 string            `json:"adopter_config_sha256,omitempty"`
	Unread              string            `json:"unread,omitempty"` // why some or all adopter keys were not read
	Deviations          []configDeviation `json:"deviations"`
}

type configDeviation struct {
	Key     string `json:"key"`
	Adopter string `json:"adopter"` // a safe literal, `<set>` for any other value, or `<unset>`
	Prove   string `json:"prove"`
}

// proveConfigKeys are the prove config's settings as recorded, and playwrightDefaults the value
// Playwright uses when the adopter config leaves a key unset (`<varies>` when it has no fixed one).
var proveConfigKeys = map[string]string{
	"testDir": "'.'", "workers": "1", "retries": "0", "use.browserName": "'chromium'",
	"use.baseURL": "<" + envBaseURL + ">", "projects": "<set>",
}

var playwrightDefaults = map[string]string{
	"testDir": "'.'", "workers": "<varies>", "retries": "0", "use.browserName": "'chromium'",
	"use.baseURL": "<unset>", "projects": "<unset>",
}

// safeLiteral is a value recorded as written: a number, a boolean, or a short quoted string
// without characters that could carry credentials.
var safeLiteral = regexp.MustCompile(`^(?:-?[0-9]+(?:\.[0-9]+)?|true|false|'[A-Za-z0-9._/:-]{0,64}'|"[A-Za-z0-9._/:-]{0,64}")$`)

// adopterRunnerConfig finds the adopter's Playwright config at head, from the hosted test's
// directory up to the repository root, and lists every key whose effective value differs from the
// prove config (CCE-V0-049). It never refuses: an absent, oversized or unreadable config is named
// in Unread and the prove keys are compared against Playwright's defaults.
func adopterRunnerConfig(ctx context.Context, root, head, testFile string) *runnerConfig {
	rc := &runnerConfig{Applied: proveConfigName, Deviations: []configDeviation{}}
	var text []byte
	for dir := path.Dir(testFile); ; dir = path.Dir(dir) {
		for _, name := range adopterConfigNames {
			p := path.Join(dir, name)
			b, _, ok, err := blobAt(ctx, root, head, p)
			if err != nil {
				rc.Unread = "adopter-config-unreadable"
				return rc
			}
			if ok {
				rc.AdopterConfig, rc.AdopterConfigSHA256, text = p, "sha256:"+sha256Hex(b), b
				break
			}
		}
		if rc.AdopterConfig != "" || dir == "." || dir == "/" {
			break
		}
	}
	adopter := map[string]string{}
	switch {
	case rc.AdopterConfig == "":
		rc.Unread = "adopter-config-absent"
	case len(text) > maxAdopterConfigBytes:
		rc.Unread = "adopter-config-over-limit"
	default:
		var partial bool
		adopter, partial = configKeys(string(text))
		if adopter == nil {
			rc.Unread, adopter = "adopter-config-object-not-found", map[string]string{}
		} else if partial {
			rc.Unread = "adopter-config-spread"
		}
	}
	keys := map[string]bool{}
	for k := range adopter {
		keys[k] = true
	}
	for k := range proveConfigKeys {
		keys[k] = true
	}
	for _, k := range sortedKeys(keys) {
		a, set := adopter[k]
		p, proved := proveConfigKeys[k]
		if !proved {
			p = "<not applied>"
		}
		if !set {
			if playwrightDefaults[k] == p {
				continue
			}
			a = "<unset>"
		} else if a == p && a != "<set>" {
			continue
		}
		rc.Deviations = append(rc.Deviations, configDeviation{Key: k, Adopter: a, Prove: p})
	}
	return rc
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// configStart finds the config object: the first `{` after `defineConfig(`, `module.exports =` or
// `export default`.
var configStart = regexp.MustCompile(`defineConfig\s*\(\s*\{|module\.exports\s*=\s*\{|export\s+default\s+\{`)

// configKeys reads the config object's top-level keys and the keys of its `use` object, each
// mapped to its value as a safe literal or `<set>`. A spread member makes the result partial. It
// returns nil when no config object is found or the object does not close.
func configKeys(src string) (map[string]string, bool) {
	clean := stripComments(src)
	loc := configStart.FindStringIndex(clean)
	if loc == nil {
		return nil, false
	}
	members, ok := objectMembers(clean[loc[1]-1:])
	if !ok {
		return nil, false
	}
	out, partial := map[string]string{}, false
	for _, m := range members {
		switch {
		case m.spread:
			partial = true
		case m.key == "use" && strings.HasPrefix(m.value, "{"):
			out["use"] = "<set>"
			inner, ok := objectMembers(m.value)
			if !ok {
				partial = true
				continue
			}
			for _, u := range inner {
				if u.spread {
					partial = true
					continue
				}
				out["use."+u.key] = literal(u.value)
			}
		default:
			out[m.key] = literal(m.value)
		}
	}
	delete(out, "use")
	return out, partial
}

func literal(v string) string {
	if safeLiteral.MatchString(v) {
		return strings.ReplaceAll(v, `"`, "'")
	}
	return "<set>"
}

type member struct {
	key, value string
	spread     bool
}

// objectMembers splits the object literal starting at obj[0] == '{' into its depth-one members.
// Strings and nested brackets are skipped; a shorthand member has the value `<set>`.
func objectMembers(obj string) ([]member, bool) {
	var out []member
	depth, start := 0, 1
	flush := func(end int) {
		entry := strings.TrimSpace(obj[start:end])
		start = end + 1
		if entry == "" {
			return
		}
		if strings.HasPrefix(entry, "...") {
			out = append(out, member{spread: true})
			return
		}
		key, value, found := strings.Cut(entry, ":")
		key = strings.Trim(strings.TrimSpace(key), `'"`)
		if !found {
			value = "<set>"
		}
		if key == "" || strings.ContainsAny(key, " \t\n([{") {
			// a method, computed key or other member form this reader does not place
			out = append(out, member{spread: true})
			return
		}
		out = append(out, member{key: key, value: strings.TrimSpace(value)})
	}
	for i := 0; i < len(obj); i++ {
		switch c := obj[i]; c {
		case '\'', '"', '`':
			j := i + 1
			for j < len(obj) && obj[j] != c {
				if obj[j] == '\\' {
					j++
				}
				j++
			}
			i = j
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
			if depth == 0 {
				flush(i)
				return out, true
			}
		case ',':
			if depth == 1 {
				flush(i)
			}
		}
	}
	return nil, false
}

// stripComments blanks `//` and `/* */` comments outside string literals.
func stripComments(src string) string {
	b := []byte(src)
	for i := 0; i < len(b); i++ {
		switch c := b[i]; {
		case c == '\'' || c == '"' || c == '`':
			for i++; i < len(b) && b[i] != c; i++ {
				if b[i] == '\\' {
					i++
				}
			}
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			for ; i < len(b) && b[i] != '\n'; i++ {
				b[i] = ' '
			}
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			for ; i < len(b) && !(b[i] == '*' && i+1 < len(b) && b[i+1] == '/'); i++ {
				if b[i] != '\n' {
					b[i] = ' '
				}
			}
			if i+1 < len(b) {
				b[i], b[i+1] = ' ', ' '
				i++
			}
		}
	}
	return string(b)
}
