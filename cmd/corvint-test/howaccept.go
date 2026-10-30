package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/skhproof"
)

const howAcceptUsage = "usage: corvint-test e2e how-accept --answer FILE [--write PATH] [--evidence FILE] [--oracle FILE]"

// howAcceptHelp is `corvint-test e2e how-accept --help` (CCE-V0-118); the options follow it.
const howAcceptHelp = `Turn a site-knowledge how draft into its candidate test and print the acceptance receipt (SKH-V0 stage 8).

Usage:
  corvint-test e2e how-accept --answer FILE [--write PATH] [--evidence FILE] [--oracle FILE]

Inputs:
  --answer is a site-knowledge-how/0 answer holding the stage 7 draft. --write names a new file
  for the candidate test; an existing path is refused. --evidence (site-knowledge-proof-evidence/0,
  from how-evidence) and --oracle (site-knowledge-accepted-oracle/0) are optional; without
  --evidence the receipt blocks as infrastructure. It starts no browser and runs no test.

Produces:
  The acceptance receipt on stdout, and the candidate test at --write.

Example:
  corvint-test e2e how-accept --answer /tmp/how.json --write e2e/specs/candidate.spec.ts
`

// howAccept is `corvint-test e2e how-accept` (SKH-V0-080, SKH-V0-083, SKH-V0-174): it turns the
// guarded how draft into the candidate by the one recorded transform, writes the candidate only to
// the caller-named --write path, and prints the acceptance receipt for the recorded evidence. It
// starts no browser and runs no test; with no --evidence the receipt blocks as infrastructure.
func howAccept(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("corvint-test e2e how-accept", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	answer := fs.String("answer", "", "site-knowledge-how/0 answer holding the stage 7 draft")
	write := fs.String("write", "", "new file to write the candidate test to; an existing path is refused")
	evidence := fs.String("evidence", "", "recorded site-knowledge-proof-evidence/0 for the candidate")
	oracle := fs.String("oracle", "", "site-knowledge-accepted-oracle/0 for the request")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return writeCommandHelp(stdout, "how-accept", howAcceptHelp, fs)
		}
		return fmt.Errorf("%v (%s)", err, howAcceptUsage)
	}
	if fs.NArg() != 0 || *answer == "" {
		return errors.New(howAcceptUsage)
	}
	data, err := os.ReadFile(*answer)
	if err != nil {
		return &skhproof.Refusal{Code: "skh-answer-invalid", Detail: err.Error()}
	}
	candidate, transform, err := skhproof.Candidate(data)
	if err != nil {
		return err
	}
	var e *skhproof.Evidence
	if *evidence != "" {
		raw, err := os.ReadFile(*evidence)
		if err != nil {
			return &skhproof.Refusal{Code: "skh-evidence-invalid", Detail: err.Error()}
		}
		if e, err = skhproof.DecodeEvidence(raw); err != nil {
			return err
		}
	}
	var o *skhproof.Oracle
	if *oracle != "" {
		raw, err := os.ReadFile(*oracle)
		if err != nil {
			return &skhproof.Refusal{Code: "skh-oracle-invalid", Detail: err.Error()}
		}
		if o, err = skhproof.DecodeOracle(raw); err != nil {
			return err
		}
	}
	if *write != "" {
		if candidate == nil {
			return &skhproof.Refusal{Code: skhproof.CodeGenerateIncomplete, Detail: "the draft is not COMPLETE; there is no candidate to write"}
		}
		if err := writeNew(*write, candidate); err != nil {
			return err
		}
	}
	_, err = stdout.Write(skhproof.Assess(transform, e, o).Encode())
	return err
}

// writeNew creates path exclusively, refusing an existing file or a symlink.
func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|openNoFollow, 0o644)
	if err != nil {
		return unsafeOut("candidate path %s: %v", path, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
