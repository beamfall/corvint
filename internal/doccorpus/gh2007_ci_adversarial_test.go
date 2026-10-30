package doccorpus

import (
	"strings"
	"testing"
)

// TestDCIV1021AdversarialPublishSteps walks the publish admission rule of DCI-V1-021 through
// malformed input, quoting, aliasing and indirection, ordering and unobserved state: each case
// ties only the directories a literal, executed publish command names, and every case Corvint
// cannot read is opaque, so it ties nothing.
func TestDCIV1021AdversarialPublishSteps(t *testing.T) {
	t.Parallel()
	const head = "on: push\njobs:\n  r:\n    runs-on: ubuntu-latest\n    steps:\n"
	const checkout = "      - uses: actions/checkout@v4\n"
	gh := func(steps string) string { return head + checkout + steps }
	for _, c := range []struct {
		name, rule, yaml string
		ties             string
		opaque           bool
	}{
		{"plain", githubJobRule, gh("      - run: npm publish\n"), ".", false},
		{"comment", githubJobRule, gh("      - run: npm publish # to the registry\n"), ".", false},
		{"quoted tool", githubJobRule, gh("      - run: |\n          'npm' publish\n"), ".", false},
		{"value option", githubJobRule, gh("      - run: npm publish --tag next --access public\n"), ".", false},
		{"cd then relative folder", githubJobRule, gh("      - run: cd packages/b && npm publish ../a\n"), "packages/a", false},
		{"dry run", githubJobRule, gh("      - run: npm publish --dry-run\n"), "", false},
		{"print", githubJobRule, gh("      - run: printf 'npm publish'\n"), "", false},
		{"tab indentation", githubJobRule, gh("    \t- run: npm publish\n"), "", true},
		{"tab separator", githubJobRule, gh("      - run:\tnpm publish\n"), ".", false},
		{"unterminated quote", githubJobRule, gh("      - run: npm publish './x\n"), "", true},
		{"alias run", githubJobRule, gh("      - run: *publish\n"), "", true},
		{"merge key", githubJobRule, gh("      - <<: *step\n        run: npm publish\n"), "", true},
		{"duplicate key", githubJobRule, gh("      - run: npm test\n        run: npm publish\n"), "", true},
		{"folded run", githubJobRule, gh("      - run: >\n          npm publish\n"), "", true},
		{"flow step", githubJobRule, gh("      - {run: npm publish}\n"), "", true},
		{"expanded tool", githubJobRule, gh("      - run: $NPM publish\n"), "", true},
		{"script run", githubJobRule, gh("      - run: npm run publish\n"), "", true},
		{"wrapper", githubJobRule, gh("      - run: env npm publish\n"), "", true},
		{"option before subcommand", githubJobRule, gh("      - run: pnpm -r publish\n"), "", true},
		{"selector", githubJobRule, gh("      - run: pnpm publish --filter @x/a\n"), "", true},
		{"dry run value", githubJobRule, gh("      - run: npm publish --dry-run=false\n"), "", true},
		{"workspace", githubJobRule, gh("      - run: yarn workspace @x/a publish\n"), "", true},
		{"berry folder", githubJobRule, gh("      - run: yarn npm publish ./packages/a\n"), "", true},
		{"berry", githubJobRule, gh("      - run: yarn npm publish\n"), ".", false},
		{"unknown option and folder", githubJobRule, gh("      - run: npm publish --weird ./packages/a\n"), "", true},
		{"tarball", githubJobRule, gh("      - run: npm publish ./a.tgz\n"), "", true},
		{"escape", githubJobRule, gh("      - run: npm publish ../../x\n"), "", true},
		{"conditional cd", githubJobRule, gh("      - run: false || cd packages/a && npm publish\n"), "", true},
		{"variable cd", githubJobRule, gh("      - run: cd $D && npm publish\n"), "", true},
		{"structured cd", githubJobRule, gh("      - run: |\n          if true; then cd packages/a; fi\n          npm publish\n"), "", true},
		{"heredoc", githubJobRule, gh("      - run: |\n          sh <<EOF\n          npm publish\n          EOF\n"), "", true},
		{"pwsh", githubJobRule, gh("      - run: npm publish\n        shell: pwsh\n"), "", true},
		{"windows runner", githubJobRule, "on: push\njobs:\n  r:\n    runs-on: windows-latest\n    steps:\n" + checkout + "      - run: npm publish\n", "", true},
		{"no checkout", githubJobRule, head + "      - run: npm publish\n", "", true},
		{"checkout after", githubJobRule, head + "      - run: npm publish\n" + checkout, "", true},
		{"checkout path", githubJobRule, head + checkout + "        with:\n          path: sub\n      - run: npm publish\n", "", true},
		{"checkout repository", githubJobRule, head + checkout + "        with:\n          repository: other/repo\n      - run: npm publish\n", "", true},
		{"anchored job", githubJobRule, "on: push\njobs:\n  r: &job\n    runs-on: ubuntu-latest\n    steps:\n" + checkout + "      - run: npm publish\n", ".", false},
		{"two documents", githubJobRule, gh("      - run: npm publish\n") + "---\nname: x\n", "", true},
		{"circle checkout", circleCIJobRule, "version: 2.1\njobs:\n  r:\n    steps:\n      - checkout\n      - run: npm publish\n", ".", false},
		{"circle windows", circleCIJobRule, "version: 2.1\njobs:\n  r:\n    machine:\n      image: windows-server-2022-gui:current\n    steps:\n      - checkout\n      - run: npm publish\n", "", true},
		{"circle home", circleCIJobRule, "version: 2.1\njobs:\n  r:\n    steps:\n      - checkout\n      - run:\n          command: npm publish\n          working_directory: ~/project\n", "", true},
	} {
		ties, opaque := publishTies(parseCI(strings.Split(c.yaml, "\n")), "r", c.rule)
		var dirs []string
		for _, tie := range ties {
			dirs = append(dirs, tie.dir)
		}
		if got := strings.Join(dirs, ","); got != c.ties || opaque != c.opaque {
			t.Errorf("%s: ties %q opaque %v, want %q %v", c.name, got, opaque, c.ties, c.opaque)
		}
	}
}
