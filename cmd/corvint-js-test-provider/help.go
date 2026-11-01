package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-972). providerUsage stays
// the authored root help and each subcommand's flag set its authored detail.
const usageText = `corvint-js-test-provider unit [--dir DIR] [--config FILE] [--package-json FILE] [--lockfile FILE] [--runner-version VERSION] [--timeout DURATION] [--test-file FILE]... [--env-key KEY]... [--retain]
    [--watch RELPATH... --foreground --experimental --trusted-local]
corvint-js-test-provider bun [--dir DIR] [--bun FILE] [--package-json FILE] [--lockfile FILE] [--runner-version VERSION] [--max-concurrency N] [--test-name-pattern PATTERN] [--timeout DURATION] [--test-file FILE]... [--env-key KEY]... [--preload FILE]... [--service NAME]... [--retain]
corvint-js-test-provider e2e [--dir DIR] [--config FILE] [--package-json FILE] [--lockfile FILE] [--runner-version VERSION] [--app-build-dir DIR] [--external-server] [--app-identity ID] [--app-attestation-command FILE] [--app-attestation-config FILE] [--app-attestation-timeout DURATION]
    [--sensitive-input-redaction] [--sensitive-action-pattern PATTERN]... [--sensitive-field NAME]... [--retain-attempt-details] [--keep-reporters] [--keep-reporters-qualification FILE] [--server-ready-url URL] [--server-ready-timeout DURATION] [--preflight FILE]
    [--server-arg ARG]... [--test-arg ARG]... [--test-file FILE]... [--env-key KEY]... [--timeout DURATION] [--runner-output PATH] [--interrupt-grace DURATION] [--retain] [--watch RELPATH... --foreground --experimental --trusted-local]
corvint-js-test-provider qualify-keep-reporters --external-server [--dir DIR] [--config FILE] [--package-json FILE] [--lockfile FILE] [--runner-version VERSION] [--app-build-dir DIR] [--app-identity ID] [--app-attestation-command FILE] [--app-attestation-config FILE] [--app-attestation-timeout DURATION]
    [--sensitive-input-redaction] [--sensitive-action-pattern PATTERN]... [--sensitive-field NAME]... [--retain-attempt-details] [--server-ready-url URL] [--server-ready-timeout DURATION]
    [--server-arg ARG]... [--test-arg ARG]... [--test-file FILE]... [--env-key KEY]... [--timeout DURATION] [--runner-output PATH] [--interrupt-grace DURATION] [--retain]
corvint-js-test-provider negate --root DIR --spec FILE --test TITLE [--project NAME] [--step N | --all-steps] [--max-runs N] [--baseline-repeat N] [--dir DIR] [--config FILE] [--package-json FILE] [--lockfile FILE] [--runner-version VERSION]
    [--app-build-dir DIR] [--external-server] [--app-identity ID] [--app-attestation-command FILE] [--app-attestation-config FILE] [--app-attestation-timeout DURATION] [--server-ready-url URL] [--server-ready-timeout DURATION] [--timeout DURATION] [--test-file FILE]... [--env-key KEY]...
corvint-js-test-provider external-origins --policy FILE --network FILE...
`

var commandSummaries = map[string]string{
	"unit":                   "Run the bound Vitest adapter and print the receipt document",
	"bun":                    "Run bun test with the JUnit reporter and print the receipt document; refuses an unqualified bun version or lockfile",
	"e2e":                    "Run the bound Playwright adapter and print the receipt document; with --external-server the canonical qualified receipt",
	"qualify-keep-reporters": "Print the keep-reporters qualification record from an --external-server e2e run; refuses --keep-reporters, --keep-reporters-qualification, --preflight and watch",
	"negate":                 "Generate step-level negative controls for one Playwright test",
	"external-origins":       "Print the external-origin dependence report from a project policy and network evidence of 1..16 runs; runs no test",
}

// subcommandUsage writes the flag usage of a subcommand that has its own flag set; it runs no test.
var subcommandUsage = map[string]func(){
	"unit":                   func() { _, _, _ = parseUnitConfig([]string{"-h"}) },
	"bun":                    func() { _, _, _ = parseBunConfig([]string{"-h"}) },
	"e2e":                    func() { _ = runE2EMode("e2e", []string{"-h"}) },
	"qualify-keep-reporters": func() { _ = runE2EMode("e2e", []string{"-h"}) },
	"negate":                 func() { _, _ = parseNegate([]string{"-h"}) },
	"external-origins":       func() { _, _, _ = parseExternalOrigins([]string{"-h"}) },
}

// helpSpellings are the binary's own help words, which the tree help answers first except -help.
var helpSpellings = []clitree.Internal{
	{Word: "help", Reason: "alternative spelling of --help that the binary already accepted; its tree help answers it"},
	{Word: "-help", Reason: "single-dash alternative spelling of --help that the binary's own dispatch still answers"},
}

// companionHelp is corvint-js-test-provider's command tree and help (CAP-V0-973).
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{
		Tree:  clitree.CompanionTree("corvint-js-test-provider", usageText, commandSummaries, nil, nil, helpSpellings),
		Intro: printUsage,
		Detail: func(path []string, w io.Writer) {
			if show, ok := subcommandUsage[path[0]]; ok {
				io.WriteString(w, "\n")
				printSubcommandUsage(w, show)
			}
		},
	}
})
