// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// pageCommand is one command of the `docs pages` group (AFL-V1-008): its new spelling after
// `docs pages`, and the old spelling that stays its alias for one release. Both run the same
// handler, which still dispatches on the old words.
type pageCommand struct {
	verb string
	old  []string
}

// pageCommands are the six commands that write or check a generated documentation page, in help
// order; the names are recorded in the flows-status-report design (AFL-V1-008).
var pageCommands = []pageCommand{
	{"generate", []string{"docs", "flows", "generate"}},
	{"check", []string{"docs", "flows", "check"}},
	{"finalize", []string{"docs", "flows", "finalize"}},
	{"flow", []string{"flows", "docs"}},
	{"flow-maintain", []string{"flows", "docs", "maintain"}},
	{"maintain", []string{"docs", "maintain"}},
}

// pageVerbs are the `docs pages` verbs, in help order.
func pageVerbs() []string {
	verbs := make([]string, 0, len(pageCommands))
	for _, command := range pageCommands {
		verbs = append(verbs, command.verb)
	}
	return verbs
}

// aliasNotice is the one stderr line an old spelling prints before it runs (AFL-V1-008).
func aliasNotice(old, renamed string) string {
	return fmt.Sprintf("%s: %s is renamed %s; %s is an alias until the next release (AFL-V1-008)\n", binaryName, old, renamed, old)
}

// resolveDocsPages maps `[--root PATH] docs pages VERB ...` onto the handler's own words, and
// prints the alias notice for an old spelling (`docs flows ...`, `flows docs [maintain] ...`,
// `docs maintain ...`) before it runs unchanged. A `docs pages` help request is returned as given
// for the help parser; `docs pages` with no verb, or an unknown one (help requested or not), and
// a positional word after `docs pages flow` are refused. Every other invocation is returned as
// given.
func resolveDocsPages(arguments []string, stderr io.Writer) ([]string, error) {
	position := commandPositionAfterRoots(arguments)
	if position < 0 {
		return arguments, nil
	}
	rest := arguments[position:]
	if len(rest) >= 2 && rest[0] == "docs" && rest[1] == "pages" {
		// An unknown verb is refused before help, so `docs pages bogus --help` names the verbs
		// instead of reaching a parser that has none.
		if len(rest) > 2 && !argparseOptionLike(rest[2]) && !slices.Contains(pageVerbs(), rest[2]) {
			return nil, argumentError("docs pages requires one of " + strings.Join(pageVerbs(), ", ") + "; got " + rest[2])
		}
		// `docs pages flow` takes options only. Its handler is the old `flows docs`, which reads a
		// leading maintain as flow-maintain, whose --apply writes, so a positional is refused.
		if len(rest) > 3 && rest[2] == "flow" && !argparseOptionLike(rest[3]) {
			return nil, argumentError("docs pages flow takes no positional argument: " + rest[3] + "; flow-maintain is docs pages flow-maintain")
		}
		if helpTokenBeforeEnd(rest[2:]) {
			return arguments, nil // the help parser names the docs pages topic
		}
		for _, command := range pageCommands {
			if len(rest) > 2 && rest[2] == command.verb {
				resolved := append(slices.Clone(arguments[:position]), command.old...)
				return append(resolved, rest[3:]...), nil
			}
		}
		return nil, argumentError("docs pages requires one of " + strings.Join(pageVerbs(), ", "))
	}
	// The longest old spelling first, so flows docs maintain is not read as flows docs; every
	// `docs flows` form, a bad mode included, is the old family's.
	for _, old := range [][]string{{"flows", "docs", "maintain"}, {"flows", "docs"}, {"docs", "maintain"}, {"docs", "flows"}} {
		if len(rest) < len(old) || !slices.Equal(rest[:len(old)], old) {
			continue
		}
		if helpTokenBeforeEnd(rest) {
			return arguments, nil // help prints the topic, which names the new spelling
		}
		spelled, renamed := strings.Join(old, " "), "docs pages generate|check|finalize"
		for _, command := range pageCommands {
			if slices.Equal(command.old, old) || len(rest) > 2 && slices.Equal(command.old, rest[:3]) && old[1] == "flows" {
				spelled, renamed = strings.Join(command.old, " "), "docs pages "+command.verb
				break
			}
		}
		_, _ = io.WriteString(stderr, aliasNotice(spelled, renamed))
		return arguments, nil
	}
	return arguments, nil
}

// docsPagesHelp is the `docs pages` topic: the one group of the commands that write or check a
// generated documentation page (AFL-V1-008), with the old spellings marked as aliases.
const docsPagesHelp = `Generated documentation pages: generate, check, finalize and maintain the pages Corvint writes.

Usage:
  corvint-docs [--root PATH] docs pages generate --revision FULL_SHA --scope PATH --output-dir FRESH_DIR ...
  corvint-docs [--root PATH] docs pages check --revision FULL_SHA --previous FILE ...
  corvint-docs [--root PATH] docs pages finalize --previous FILE --provider-revision FULL_SHA --provider-path PATH
  corvint-docs [--root PATH] docs pages flow --flows DIR --page FILE --claims FILE ...
  corvint-docs [--root PATH] docs pages flow-maintain --flows DIR --page FILE --claims FILE --preview ...
  corvint-docs [--root PATH] docs pages maintain --check --page PATH ...

generate, check and finalize write and check the bounded source-derived flow pages of a
corpus; flow renders one flow page and its claims sidecar from the committed intents (--check
compares); flow-maintain previews or applies maintenance of that committed page pair; maintain
refreshes one human page's generated blocks from source. Every one is experimental. Each
command's full usage: corvint-docs help docs pages VERB.

Old spellings, aliases until the next release (AFL-V1-008): each runs the same command and
prints one stderr notice naming its new spelling.
  docs flows generate|check|finalize  is  docs pages generate|check|finalize
  flows docs                          is  docs pages flow
  flows docs maintain                 is  docs pages flow-maintain
  docs maintain                       is  docs pages maintain
Example: corvint-docs docs pages maintain --check --page docs/guide.md
`

// pagesHelpTopic is the help topic of `docs pages VERB`: the flow-page generator's for generate,
// check and finalize, the page maintainer's for maintain, the group's otherwise.
func pagesHelpTopic(verb string) string {
	switch verb {
	case "generate", "check", "finalize":
		return "docs-flows"
	case "maintain":
		return "docs-maintain"
	}
	return "docs-pages"
}
