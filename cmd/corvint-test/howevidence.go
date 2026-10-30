package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/skhproof"
)

const howEvidenceUsage = "usage: corvint-test e2e how-evidence --answer FILE --prove FILE --validity FILE [--root DIR]"

// howEvidenceHelp is `corvint-test e2e how-evidence --help` (CCE-V0-118); the options follow it.
const howEvidenceHelp = `Record the live run's outputs for a how candidate as proof evidence (SKH-V0 stage 8).

Usage:
  corvint-test e2e how-evidence --answer FILE --prove FILE --validity FILE [--root DIR]

Inputs:
  --answer is the site-knowledge-how/0 answer, --prove the e2e-prove/0 report of
  corvint-test e2e prove --hosted-test for the candidate, and --validity the
  corvint-test-source-validity/0 report of corvint tests validity for it. The candidate must be
  committed in the repository at --root (default .). It runs no test and writes no file.

Produces:
  site-knowledge-proof-evidence/0 on stdout, for how-accept --evidence.

Example:
  corvint-test e2e how-evidence --root . --answer /tmp/how.json --prove /tmp/e2e-proof/e2e-prove.json --validity /tmp/validity.json
`

// maxCandidateBytes bounds the committed candidate bytes how-evidence reads back.
const maxCandidateBytes = 1 << 20

// howEvidence is `corvint-test e2e how-evidence` (SKH-V0-180, SKH-V0-184): it records the live
// run's own outputs for the answer's candidate as site-knowledge-proof-evidence/0 on stdout, for
// `how-accept --evidence`. It reads the candidate's committed bytes from the repository at --root,
// runs no test and writes no file.
func howEvidence(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("corvint-test e2e how-evidence", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "repository the prove and validity reports read")
	answer := fs.String("answer", "", "site-knowledge-how/0 answer holding the stage 7 draft")
	prove := fs.String("prove", "", "e2e-prove/0 report of `corvint-test e2e prove --hosted-test` for the candidate")
	validity := fs.String("validity", "", "corvint-test-source-validity/0 report of `corvint tests validity` for the candidate")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return writeCommandHelp(stdout, "how-evidence", howEvidenceHelp, fs)
		}
		return fmt.Errorf("%v (%s)", err, howEvidenceUsage)
	}
	if fs.NArg() != 0 || *answer == "" || *prove == "" || *validity == "" {
		return errors.New(howEvidenceUsage)
	}
	docs := make([][]byte, 3)
	for i, name := range []string{*answer, *prove, *validity} {
		data, err := os.ReadFile(name)
		if err != nil {
			code := "skh-evidence-invalid"
			if i == 0 {
				code = "skh-answer-invalid"
			}
			return &skhproof.Refusal{Code: code, Detail: err.Error()}
		}
		docs[i] = data
	}
	e, err := skhproof.Record(docs[0], docs[1], docs[2], committedFile(ctx, *root))
	if err != nil {
		return err
	}
	_, err = stdout.Write(e.Encode())
	return err
}

// committedFile reads path's blob at rev from the repository at root, bounded.
func committedFile(ctx context.Context, root string) skhproof.ReadFile {
	return func(rev, path string) ([]byte, error) {
		if rev == "" || strings.HasPrefix(rev, "-") || strings.ContainsAny(rev+path, ":\x00\n") {
			return nil, fmt.Errorf("invalid revision or path %q:%q", rev, path)
		}
		cmd := exec.CommandContext(ctx, "git", "-C", root, "cat-file", "blob", rev+":"+path)
		out := &capped{max: maxCandidateBytes}
		var stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = out, &stderr
		err := cmd.Run()
		if out.over {
			return nil, fmt.Errorf("%s:%s exceeds %d bytes", rev, path, maxCandidateBytes)
		}
		if err != nil {
			return nil, fmt.Errorf("git cat-file %s:%s: %v: %s", rev, path, err, strings.TrimSpace(stderr.String()))
		}
		return out.buf.Bytes(), nil
	}
}

// capped buffers at most max bytes and fails the write that would exceed it, so git is stopped
// before an oversized blob is held in memory.
type capped struct {
	buf  bytes.Buffer
	max  int
	over bool
}

func (c *capped) Write(p []byte) (int, error) {
	if c.buf.Len()+len(p) > c.max {
		c.over = true
		return 0, errors.New("candidate exceeds the read bound")
	}
	return c.buf.Write(p)
}
