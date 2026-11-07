// Command corvint-readiness-record builds or verifies the stable readiness
// record for one verified candidate (SRR-V1-012). It runs no gate and never
// tags, signs, publishes or promotes.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/releasecandidate"
)

// usage is the source-owned synopsis for offline discovery.
const usage = `usage: corvint-readiness-record --candidate DIR --source-root DIR --evidence-file FILE --output FILE [--store-release ID --store-candidate-sha256 SHA256]
       corvint-readiness-record --verify FILE --candidate DIR --evidence-file FILE
Build mode takes both store flags together or neither. Verify mode accepts no build inputs.`

func main() {
	streams := &clierror.Streams{Binary: "corvint-readiness-record", Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-readiness-record", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}
	os.Exit(streams.Status(run(os.Args[1:], streams.Stdout(), streams.Stderr())))
}

// The two modes are variables so a test can run each through run.
var (
	writeRecord  = releasecandidate.WriteReadinessRecord
	verifyRecord = releasecandidate.VerifyReadinessFile
)

func run(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("corvint-readiness-record", flag.ContinueOnError)
	flags.SetOutput(stderr)
	verify := flags.String("verify", "", "readiness record to verify; selects verify mode, which writes nothing")
	candidate := flags.String("candidate", "", "verified release candidate directory")
	source := flags.String("source-root", "", "Corvint Git checkout containing the candidate commit (build mode)")
	evidenceFile := flags.String("evidence-file", "", "TSV of ROW, STATUS, PATH, DECISION and REASON, one supplied row per line")
	storeRelease := flags.String("store-release", "", "task-store release id (build mode, optional)")
	storeDigest := flags.String("store-candidate-sha256", "", "task-store candidate SHA-256 (build mode, optional)")
	output := flags.String("output", "", "new file for the record; an existing file is refused (build mode)")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	build := true
	flags.Visit(func(set *flag.Flag) { build = build && set.Name != "verify" })
	buildInputs := *source != "" || *storeRelease != "" || *storeDigest != "" || *output != ""
	complete := *candidate != "" && *evidenceFile != "" && (!build || (*source != "" && *output != ""))
	partialStore := (*storeRelease == "") != (*storeDigest == "")
	if flags.NArg() != 0 || !complete || partialStore || (!build && (buildInputs || *verify == "")) {
		fmt.Fprintln(stderr, "corvint-readiness-record: build mode needs -candidate, -source-root, -evidence-file and -output, and takes -store-release and -store-candidate-sha256 together or not at all; verify mode takes only a non-empty -verify, -candidate and -evidence-file; positional arguments are forbidden")
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	evidence, err := releasecandidate.ReadReadinessEvidence(*evidenceFile)
	if err != nil {
		fmt.Fprintln(stderr, "corvint-readiness-record:", err)
		return 1
	}
	if build {
		options := releasecandidate.ReadinessOptions{CandidateDirectory: *candidate, SourceRoot: *source, StoreReleaseID: *storeRelease, StoreCandidateSHA256: *storeDigest, Evidence: evidence}
		record, err := writeRecord(ctx, options, *output)
		return report(stdout, stderr, *output, record, err)
	}
	record, err := verifyRecord(ctx, *verify, *candidate, evidence)
	return report(stdout, stderr, *verify, record, err)
}

func report(stdout, stderr io.Writer, path string, record *releasecandidate.ReadinessRecord, err error) int {
	if err != nil {
		fmt.Fprintln(stderr, "corvint-readiness-record:", err)
		return 1
	}
	fmt.Fprintf(stdout, "record: %s\nversion: %s\ncommit: %s\n", path, record.Identity.Version, record.Identity.Commit)
	for _, row := range record.Rows {
		fmt.Fprintf(stdout, "%s: %s\n", row.ID, row.Status)
	}
	return 0
}
