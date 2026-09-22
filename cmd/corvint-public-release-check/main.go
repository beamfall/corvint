// Command corvint-public-release-check qualifies one already-retained companion
// bundle. It never builds or publishes artifacts and never changes repository
// visibility.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("corvint-public-release-check", flag.ContinueOnError)
	qualification := fs.String("qualification", "editor", "qualification profile: editor or core")
	nodeSHA := fs.String("node-sha256", "", "core: frozen Node executable SHA-256")
	npmSHA := fs.String("npm-sha256", "", "core: frozen npm CLI SHA-256")
	authority := fs.String("go-authority-bundle", "", "core: canonical trusted-local Go authority attachment")
	authoritySHA := fs.String("go-authority-sha256", "", "core: frozen Go authority attachment SHA-256")
	bundle := fs.String("bundle-dir", "", "retained companion bundle directory")
	sourceRoot := fs.String("source-root", "", "clean source root used to build this checker")
	scratch := fs.String("scratch", "", "fresh owned scratch path")
	output := fs.String("output", "", "new retained result JSON path")
	cache := fs.String("npm-cache", "", "populated npm cache seed")
	browserCache := fs.String("browser-cache", "", "populated pinned Playwright browser cache seed")
	node := fs.String("node", "", "canonical Node executable")
	python := fs.String("python", "", "canonical Python executable")
	pythonSHA := fs.String("python-sha256", "", "frozen Python executable SHA-256")
	commit := fs.String("expected-commit", "", "frozen Corvint source commit")
	tree := fs.String("expected-tree", "", "frozen Corvint source tree")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || *sourceRoot == "" || *bundle == "" || *scratch == "" || *output == "" || *cache == "" || *browserCache == "" || *node == "" || *python == "" || *pythonSHA == "" || *commit == "" || *tree == "" {
		fmt.Fprintln(os.Stderr, "corvint-public-release-check: all named arguments are required")
		return 2
	}
	if (*qualification != "core" && *qualification != "editor") || (*qualification == "core" && (*nodeSHA == "" || *npmSHA == "" || *authority == "" || *authoritySHA == "")) || (*qualification == "editor" && (*nodeSHA != "" || *npmSHA != "" || *authority != "" || *authoritySHA != "")) {
		fmt.Fprintln(os.Stderr, "corvint-public-release-check: core requires all four core runtime/authority arguments; editor rejects them")
		return 2
	}
	if err := verifySelf(*commit); err != nil {
		fmt.Fprintf(os.Stderr, "corvint-public-release-check: %v\n", err)
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Minute)
	defer timeoutCancel()
	opts := companionrelease.InstalledOptions{SourceRoot: *sourceRoot, BundleDirectory: *bundle, Scratch: *scratch, OutputPath: *output, NPMCache: *cache, BrowserCache: *browserCache, NodePath: *node, PythonPath: *python, ExpectedPythonSHA256: *pythonSHA, ExpectedCommit: *commit, ExpectedTree: *tree}
	var bundleSHA, sourceCommit, sourceTree string
	var err error
	if *qualification == "core" {
		var report companionrelease.CoreInstalledReport
		report, err = companionrelease.RunCoreInstalledQualification(ctx, companionrelease.CoreInstalledOptions{InstalledOptions: opts, ExpectedNodeSHA256: *nodeSHA, ExpectedNPMSHA256: *npmSHA, GoAuthorityPath: *authority, ExpectedGoAuthoritySHA256: *authoritySHA})
		bundleSHA, sourceCommit, sourceTree = report.BundleSHA256, report.SourceCommit, report.SourceTree
	} else {
		var report companionrelease.InstalledReport
		report, err = companionrelease.RunInstalledQualification(ctx, opts)
		bundleSHA, sourceCommit, sourceTree = report.BundleSHA256, report.SourceCommit, report.SourceTree
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "corvint-public-release-check: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "installed qualification PASS bundle=%s source=%s tree=%s result=%s\n", bundleSHA, sourceCommit, sourceTree, *output)
	return 0
}

func verifySelf(expectedCommit string) error {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.GoVersion != "go1.27.1" {
		return fmt.Errorf("checker was not built by Go 1.27.1")
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["vcs.revision"] != expectedCommit || settings["vcs.modified"] != "false" {
		return fmt.Errorf("checker binary is not the clean frozen source revision")
	}
	return nil
}
