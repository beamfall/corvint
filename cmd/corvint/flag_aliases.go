package main

import (
	"slices"
	"strings"
)

// headAliasVerbs are the verbs whose range end is --target; each takes --head
// as that same option (ACX-V0-858, V1-3731).
var headAliasVerbs = map[string]bool{"prove": true, "surprise": true, "lrf": true, "obligations": true, "ocm": true, "dogfood-ocm": true}

// withHeadAlias rewrites --head and --head=VALUE to --target after a
// headAliasVerbs verb, which may follow the global --root option, so each
// verb's own parser sees one spelling. Some of those parsers keep the last
// value, so --head given with --target, or twice, is refused here rather than
// letting one value silently replace the other. Tokens after a "--"
// terminator are left alone; any other invocation is returned unchanged.
func withHeadAlias(arguments []string) ([]string, error) {
	verb := 0
	if len(arguments) >= 2 && arguments[0] == "--root" {
		verb = 2
	} else if len(arguments) >= 1 && strings.HasPrefix(arguments[0], "--root=") {
		verb = 1
	}
	if verb >= len(arguments) || !headAliasVerbs[arguments[verb]] {
		return arguments, nil
	}
	var rewritten []string
	targets := 0
	for index := verb + 1; index < len(arguments); index++ {
		token := arguments[index]
		if token == "--" {
			break
		}
		if token == "--target" || strings.HasPrefix(token, "--target=") {
			targets++
			continue
		}
		if token != "--head" && !strings.HasPrefix(token, "--head=") {
			continue
		}
		targets++
		if rewritten == nil {
			rewritten = slices.Clone(arguments)
		}
		rewritten[index] = "--target" + strings.TrimPrefix(token, "--head")
	}
	if rewritten == nil {
		return arguments, nil
	}
	if targets > 1 {
		return nil, argumentError("argument --target: --head is an alias of --target; give the range end once")
	}
	return rewritten, nil
}
