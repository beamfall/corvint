package dogfoodflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// passReceiptName is the private receipt a passing check leaves in the
// evidence directory, and passReceiptProfile its profile (DCW-V0-347).
const (
	passReceiptName    = "dogfood-check-pass.json"
	passReceiptProfile = "corvint-dogfood-check-pass/0"
	// maxPassCapture bounds the verdict a receipt keeps; a larger verdict
	// writes no receipt, so the next check reruns.
	maxPassCapture = 1 << 20
)

// passReceipt binds one passing check's verdict to the exact inputs it read.
type passReceipt struct {
	Profile string `json:"profile"`
	Base    string `json:"base"`
	Target  string `json:"target"`
	Inputs  string `json:"inputs"`
	Stdout  string `json:"stdout"`
	Stderr  string `json:"stderr"`
}

// passExcluded are the evidence-directory entries the inputs digest leaves out:
// the verifier executables, whose digests it binds instead, the receipt itself,
// the published review and the phase timings (DCW-V0-942), which no check step
// reads.
var passExcluded = map[string]bool{"corvint": true, "corvint-base": true, passReceiptName: true, "cem-review.md": true, checkTimings: true, checkBuildTimings: true}

// passLedgers are the private .corvint ledgers that read commands append to
// (AGENTS.md invariant 4); no check step reads them.
var passLedgers = map[string]bool{"self-observations.jsonl": true, "unplanned-reads.jsonl": true}

// passInputs digests everything the verifier runs of a check depend on beyond
// the steps that always run: BASE, HEAD, the anchor, every verifier identity,
// the running executable, the options, the CORVINT_ and DOGFOOD_ environment
// the verifiers inherit, and every regular file directly in .corvint/ and in
// the evidence directory. ok is false when any of them cannot be read, and
// then nothing is reused.
func (c *check) passInputs() (string, bool) {
	executable, err := os.Executable()
	if err != nil {
		return "", false
	}
	self, err := fileSHA256(executable)
	if err != nil {
		return "", false
	}
	lines := []string{
		passReceiptProfile, "base " + c.base, "target " + c.target,
		"anchor " + strconv.FormatBool(c.anchorObserved) + " " + c.anchorMergeBase,
		"base-verifier " + c.baseSHA, "tree-verifier " + c.treeSHA, "override-verifier " + c.overrideSHA, "executable " + self,
		"verbose " + strconv.FormatBool(c.options.Verbose), "output-change " + c.options.OutputChange, "exception " + c.options.Exception,
	}
	environment := []string{}
	for _, entry := range os.Environ() {
		if (strings.HasPrefix(entry, "CORVINT_") || strings.HasPrefix(entry, "DOGFOOD_")) && !strings.HasPrefix(entry, "DOGFOOD_CHECK_REUSE=") {
			environment = append(environment, "env "+strconv.Quote(entry))
		}
	}
	sort.Strings(environment)
	lines = append(lines, environment...)
	for _, directory := range []struct {
		label, path string
		skip        map[string]bool
	}{{".corvint", c.root + "/.corvint", passLedgers}, {"evidence", c.evidence, passExcluded}} {
		entries, err := os.ReadDir(directory.path)
		if err != nil {
			return "", false
		}
		for _, entry := range entries {
			if entry.IsDir() || directory.skip[entry.Name()] {
				continue
			}
			digest, err := fileSHA256(filepath.Join(directory.path, entry.Name()))
			if err != nil {
				return "", false
			}
			lines = append(lines, "file "+directory.label+" "+strconv.Quote(entry.Name())+" "+digest)
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
	return "sha256:" + hex.EncodeToString(sum[:]), true
}

// reusePass replays a former PASS when ReusePass is set and the receipt binds
// this BASE, HEAD and inputs digest (DCW-V0-347). Any other state removes the
// receipt and reports false, so the verifiers run. An aggregate check never
// reuses or writes a receipt: its own capture publishes the verdict.
func (c *check) reusePass() bool {
	receipt := c.evidence + "/" + passReceiptName
	if c.aggregate != nil || c.aggregateCapture != nil {
		_ = removeFile(receipt)
		return false
	}
	if c.options.ReusePass && isRegular(receipt) {
		var former passReceipt
		decoder := json.NewDecoder(bytes.NewReader(readFile(receipt)))
		decoder.DisallowUnknownFields()
		inputs, ok := c.passInputs()
		if ok && decoder.Decode(&former) == nil && former.Profile == passReceiptProfile && former.Base == c.base && former.Target == c.target && former.Inputs == inputs &&
			strings.Contains("\n"+former.Stdout, "\ndogfood-check: PASS\n") {
			c.say("dogfood-check: NOTE reused-pass receipt=%s inputs=%s; the verifiers did not rerun (DCW-V0-347)\n", receipt, inputs)
			_, _ = io.WriteString(c.stderr, former.Stderr)
			_, _ = io.WriteString(c.stdout, former.Stdout)
			return true
		}
		c.say("dogfood-check: NOTE reused-pass NOT_ADMITTED receipt=%s; it binds another BASE, HEAD or input, so the check reruns (DCW-V0-347)\n", receipt)
	}
	_ = removeFile(receipt)
	return false
}

// capturePass tees the verdict the verifier steps print, and returns the
// function that writes the receipt after they pass.
func (c *check) capturePass() func() {
	if c.aggregate != nil || c.aggregateCapture != nil {
		return func() {}
	}
	var stdout, stderr bytes.Buffer
	c.stdout, c.stderr = io.MultiWriter(c.stdout, &stdout), io.MultiWriter(c.stderr, &stderr)
	return func() {
		if stdout.Len()+stderr.Len() > maxPassCapture {
			return
		}
		inputs, ok := c.passInputs()
		if !ok {
			return
		}
		data, err := json.Marshal(passReceipt{Profile: passReceiptProfile, Base: c.base, Target: c.target, Inputs: inputs, Stdout: stdout.String(), Stderr: stderr.String()})
		if err != nil {
			return
		}
		staged := c.evidence + "/" + passReceiptName + ".tmp"
		if writePrivate(staged, append(data, '\n')) != nil || os.Rename(staged, c.evidence+"/"+passReceiptName) != nil {
			_ = removeFile(staged)
			c.say("dogfood-check: NOTE reused-pass receipt NOT_PRODUCED; the next check reruns (DCW-V0-347)\n")
		}
	}
}
