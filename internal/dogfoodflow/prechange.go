package dogfoodflow

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
)

// prechangeProfile names the binding `dogfood prechange` writes beside the
// receipts it records (DCW-V0-880).
const prechangeProfile = "corvint-dogfood-prechange/0"

// prechangeBindingFile is the binding's name under <git-dir>/corvint/.
const prechangeBindingFile = "prechange-binding.json"

// PrechangeOptions are the inputs of `corvint dogfood prechange`.
type PrechangeOptions struct {
	Root, Base, Task string
	Paths            []string
	// Steps runs each read; its Path is hashed into the binding.
	Steps Runner
}

// prechangeSlot is one receipt the recorder writes: its file name without
// .json, the command that satisfies it, and its outcome.
type prechangeSlot struct {
	Name    string   `json:"name"`
	Role    string   `json:"role"`
	Command string   `json:"command"`
	Argv    []string `json:"argv"`
	Status  string   `json:"status"`
	Reason  string   `json:"reason"`
	SHA256  string   `json:"sha256"`
}

// prechangeBinding is prechange-binding.json.
type prechangeBinding struct {
	Profile          string          `json:"profile"`
	Base             string          `json:"base"`
	BaseTree         string          `json:"baseTree"`
	GitDir           string          `json:"gitDir"`
	Task             string          `json:"task"`
	ExecutableSHA256 string          `json:"executableSha256"`
	Slots            []prechangeSlot `json:"slots"`
}

// slot returns the binding's entry for name, if any.
func (b prechangeBinding) slot(name string) (prechangeSlot, bool) {
	for _, slot := range b.Slots {
		if slot.Name == name {
			return slot, true
		}
	}
	return prechangeSlot{}, false
}

// prechangeReceipts are the files the recorder owns, in recording order.
var prechangeReceipts = []string{"prechange-context", "prechange-query", "prechange-impact"}

type prechange struct {
	flow
	options PrechangeOptions
}

// Prechange records the agent pre-change receipts at BASE and binds them to
// BASE, its tree, this Git directory and the task (DCW-V0-880). It refuses
// once HEAD or the tracked tree differs from BASE, so no receipt is written
// after the change began, and never overwrites a receipt it did not write.
func Prechange(ctx context.Context, options PrechangeOptions, stdout, stderr io.Writer) (int, error) {
	run := &prechange{flow: flow{ctx: ctx, prefix: "dogfood-prechange", root: options.Root, stderr: stderr}, options: options}
	return boundary(ctx, run.run, func() {})
}

func (p *prechange) run() int {
	p.requireRoot()
	p.gitDir = p.gitValue("rev-parse", "--absolute-git-dir")
	p.base = p.gitValue("rev-parse", p.options.Base+"^{commit}")
	baseTree := p.gitValue("rev-parse", p.base+"^{tree}")
	if p.gitValue("rev-parse", "HEAD^{tree}") != baseTree {
		p.refuse("prechange-after-change: HEAD's tree is not BASE's tree; a pre-change receipt is recorded only before the first commit of the change")
	}
	if dirty, _ := p.git(true, "status", "--porcelain", "--untracked-files=no"); strings.TrimSpace(dirty) != "" {
		p.refuse("prechange-after-change: a tracked file differs from HEAD; record the receipts before the first edit")
	}
	evidence := p.gitDir + "/corvint"
	if err := os.MkdirAll(evidence, 0o777); err != nil {
		p.refuse("prechange-evidence-unwritable")
	}
	previous, _ := readBinding(evidence)
	for _, name := range prechangeReceipts {
		path := evidence + "/" + name + ".json"
		if !exists(path) {
			continue
		}
		slot, ok := previous.slot(name)
		digest, err := fileSHA256(path)
		if !ok || err != nil || slot.SHA256 != digest {
			p.refuse("prechange-receipt-exists: " + path + " was not written by dogfood prechange; keep it (it stays an unbound agent receipt) or move it aside and rerun")
		}
	}
	task := p.options.Task
	if task == "" {
		task = "Dogfood change from " + p.base[:12] + " to HEAD"
	}
	executable, _ := fileSHA256(p.options.Steps.Path)
	binding := prechangeBinding{Profile: prechangeProfile, Base: p.base, BaseTree: baseTree, GitDir: p.gitDir, Task: task, ExecutableSHA256: executable}
	_ = p.options.Steps.Run(p.ctx, p.root, []string{"index", "--if-stale"}, io.Discard, io.Discard)
	p.checkpoint()
	commands := map[string][]string{
		"prechange-context": {"context", "--task", task, "--limit", "20"},
		"prechange-query":   {"query", "--task", task, "--limit", "1"},
	}
	if len(p.options.Paths) > 0 {
		commands["prechange-impact"] = append(append([]string{"impact"}, p.options.Paths...), "--limit", "10")
	}
	recorded := 0
	for _, name := range prechangeReceipts {
		slot := prechangeSlot{Name: name, Role: "slot", Command: "corvint " + strings.TrimPrefix(name, "prechange-")}
		if name == "prechange-context" {
			slot.Role = "orientation"
		}
		path := evidence + "/" + name + ".json"
		_ = removeFile(path)
		args, ok := commands[name]
		if !ok {
			slot.Status, slot.Reason = "NOT_PRODUCED", "prechange-impact-paths-absent"
			binding.Slots = append(binding.Slots, slot)
			p.say("dogfood-prechange: NOT_PRODUCED %s %s\n", name, slot.Reason)
			continue
		}
		slot.Argv = append([]string{p.options.Steps.Path, "--root", p.root}, args...)
		status, digest, stderr := p.record(path, args)
		if status != 0 {
			_ = removeFile(path)
			slot.Status, slot.Reason = "NOT_PRODUCED", failureReason(stderr, status)
			binding.Slots = append(binding.Slots, slot)
			p.say("dogfood-prechange: NOT_PRODUCED %s %s\n", name, slot.Reason)
			continue
		}
		slot.Status, slot.Reason, slot.SHA256 = "PRODUCED", "none", digest
		binding.Slots = append(binding.Slots, slot)
		if slot.Role == "slot" {
			recorded++
		}
		p.say("dogfood-prechange: RECORD %s sha256=%s command=%s\n", name, digest, slot.Command)
	}
	data, _ := json.MarshalIndent(binding, "", "  ")
	if writePrivate(evidence+"/"+prechangeBindingFile, append(data, '\n')) != nil {
		p.refuse("prechange-evidence-unwritable")
	}
	if recorded != 2 {
		return 1
	}
	return 0
}

// record runs one read into path and returns its status, the receipt's
// SHA-256 and the command's stderr.
func (p *prechange) record(path string, args []string) (int, string, []byte) {
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 1, "", nil
	}
	var stderr strings.Builder
	status := p.options.Steps.Run(p.ctx, p.root, args, output, &stderr)
	if err := output.Close(); err != nil && status == 0 {
		status = 1
	}
	p.checkpoint()
	if status != 0 {
		return status, "", []byte(stderr.String())
	}
	digest, err := fileSHA256(path)
	if err != nil {
		return 1, "", nil
	}
	return 0, digest, nil
}

// readBinding reads prechange-binding.json under evidence: the open error when
// it is absent, errBindingMalformed when it does not decode as this profile.
func readBinding(evidence string) (prechangeBinding, error) {
	var binding prechangeBinding
	data, err := readPrefix(evidence+"/"+prechangeBindingFile, maxPlanBytes+1)
	if err != nil {
		return binding, err
	}
	if len(data) > maxPlanBytes || json.Unmarshal(data, &binding) != nil || binding.Profile != prechangeProfile {
		return prechangeBinding{}, errBindingMalformed
	}
	return binding, nil
}

// errBindingMalformed marks a binding that is present but unusable.
var errBindingMalformed = errors.New("prechange-binding-malformed")

// noteBoundReceipt prints how a base-tree receipt relates to the recorder's
// binding (DCW-V0-881): nothing without a binding, OBSERVED when the binding
// names this BASE, Git directory and receipt digest, a typed note otherwise.
func (c *change) noteBoundReceipt(name, path string) {
	binding, err := readBinding(c.evidence)
	switch {
	case os.IsNotExist(err):
		return
	case err != nil:
		c.say("dogfood-change: NOTE %s NOT_OBSERVED agent-receipt-binding-malformed\n", name)
		return
	}
	slot, ok := binding.slot(name)
	digest, digestErr := fileSHA256(path)
	switch {
	case binding.Base != c.base:
		c.say("dogfood-change: NOTE %s STALE agent-receipt-binding-other-base base=%s\n", name, binding.Base)
	case binding.GitDir != c.gitDir:
		c.say("dogfood-change: NOTE %s NOT_OBSERVED agent-receipt-wrong-worktree\n", name)
	case !ok || digestErr != nil || slot.SHA256 != digest:
		c.say("dogfood-change: NOTE %s NOT_OBSERVED agent-receipt-binding-mismatch\n", name)
	default:
		c.say("dogfood-change: NOTE %s OBSERVED bound sha256=%s command=%s\n", name, digest, slot.Command)
	}
}
