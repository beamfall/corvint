package store

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
)

// DeliveryQuery is one delivered progress head to check (TLC-V0-003);
// Branch is "" when the mark names none.
type DeliveryQuery struct{ Branch, Head string }

// DeliveryResult is the observation of one query. A nil tri-state member was
// not observed: its walk was over budget or was not needed.
type DeliveryResult struct {
	Ref, RefCommit                    string
	HeadExists, Integrated, HeadOnRef *bool
}

// DeliveryObservation is one batched TLC-V0-006 observation. Failed reports
// that a required Git read failed or root is no repository; Results is then
// empty. IntegrationRef is "" when no integration ref exists.
type DeliveryObservation struct {
	Failed                            bool
	IntegrationRef, IntegrationCommit string
	Results                           []DeliveryResult
	GitCalls                          int
}

// TLC-V0-006 budgets: walk output bytes and fallback ancestry checks.
const (
	deliveryWalkMaxBytes   = 16 << 20
	deliveryRefsMaxBytes   = 32 << 20
	deliveryAncestryChecks = 16
)

func boolp(b bool) *bool { return &b }

// ObserveDeliveries checks every query against the refs of root with a fixed
// number of Git processes (TLC-V0-006): one ref listing, one object check,
// one integration walk, one walk over the distinct branch refs and at most
// deliveryAncestryChecks bounded ancestry checks. It never writes.
func ObserveDeliveries(root, integrationBranch string, qs []DeliveryQuery) DeliveryObservation {
	var obs DeliveryObservation
	run := func(stdin string, max int64, args ...string) ([]byte, bool, error) {
		obs.GitCalls++
		return deliveryGit(root, stdin, max, args...)
	}
	refsOut, over, err := run("", deliveryRefsMaxBytes, "for-each-ref", "--format=%(objectname) %(refname)", "refs/heads/", "refs/remotes/")
	if err != nil || over {
		return DeliveryObservation{Failed: true, GitCalls: obs.GitCalls}
	}
	refs := map[string]string{}
	for _, line := range strings.Split(string(refsOut), "\n") {
		if oid, name, ok := strings.Cut(line, " "); ok {
			refs[name] = oid
		}
	}
	for _, name := range []string{"refs/remotes/origin/" + integrationBranch, "refs/heads/" + integrationBranch} {
		if oid, ok := refs[name]; ok && integrationBranch != "" {
			obs.IntegrationRef, obs.IntegrationCommit = name, oid
			break
		}
	}
	obs.Results = make([]DeliveryResult, len(qs))
	if len(qs) == 0 {
		return obs
	}
	// Object check: resolve each head to a commit, or record it missing.
	var check strings.Builder
	for _, q := range qs {
		if validHead(q.Head) {
			check.WriteString(q.Head + "\n")
		} else {
			check.WriteString(strings.Repeat("0", 40) + "\n")
		}
	}
	catOut, over, err := run(check.String(), deliveryWalkMaxBytes, "cat-file", "--batch-check=%(objectname) %(objecttype)")
	if err != nil || over {
		return DeliveryObservation{Failed: true, GitCalls: obs.GitCalls}
	}
	lines := strings.Split(strings.TrimSuffix(string(catOut), "\n"), "\n")
	if len(lines) != len(qs) {
		return DeliveryObservation{Failed: true, GitCalls: obs.GitCalls}
	}
	heads := make([]string, len(qs))
	for i, line := range lines {
		oid, kind, _ := strings.Cut(line, " ")
		exists := kind == "commit" && validHead(qs[i].Head)
		obs.Results[i].HeadExists = boolp(exists)
		if exists {
			heads[i] = oid
		}
		if ref := resolveBranch(refs, qs[i].Branch); ref != "" {
			obs.Results[i].Ref, obs.Results[i].RefCommit = ref, refs[ref]
		}
	}
	// Integration walk: a head the walk from heads ^tip emits is not integrated.
	if obs.IntegrationCommit != "" {
		var in strings.Builder
		for _, h := range heads {
			if h != "" {
				in.WriteString(h + "\n")
			}
		}
		if in.Len() > 0 {
			in.WriteString("^" + obs.IntegrationCommit + "\n")
			out, over, err := run(in.String(), deliveryWalkMaxBytes, "rev-list", "--stdin")
			if err != nil {
				return DeliveryObservation{Failed: true, GitCalls: obs.GitCalls}
			}
			emitted := lineSet(out, over)
			for i, h := range heads {
				switch {
				case h == "":
				case emitted[h]:
					obs.Results[i].Integrated = boolp(false)
				case !over:
					obs.Results[i].Integrated = boolp(true)
				}
			}
		}
	}
	// Branch walk over the distinct refs of non-integrated heads.
	pending := map[string]bool{}
	for i, h := range heads {
		r := &obs.Results[i]
		switch {
		case h == "" || r.Ref == "" || r.Integrated == nil || *r.Integrated:
		case r.RefCommit == h:
			r.HeadOnRef = boolp(true)
		default:
			pending[r.Ref] = true
		}
	}
	if len(pending) == 0 {
		return obs
	}
	var in strings.Builder
	for ref := range pending {
		in.WriteString(ref + "\n")
	}
	in.WriteString("^" + obs.IntegrationCommit + "\n")
	out, over, err := run(in.String(), deliveryWalkMaxBytes, "log", "--no-show-signature", "--no-color", "--source", "--format=%H %S", "--stdin")
	if err != nil {
		return DeliveryObservation{Failed: true, GitCalls: obs.GitCalls}
	}
	source := map[string]string{}
	for _, line := range completeLines(out, over) {
		if oid, ref, ok := strings.Cut(line, " "); ok {
			source[oid] = ref
		}
	}
	checks := 0
	for i, h := range heads {
		r := &obs.Results[i]
		if h == "" || !pending[r.Ref] || r.HeadOnRef != nil {
			continue
		}
		src, seen := source[h]
		switch {
		case seen && src == r.Ref:
			r.HeadOnRef = boolp(true)
		case !seen && !over:
			r.HeadOnRef = boolp(false)
		case checks < deliveryAncestryChecks:
			checks++
			obs.GitCalls++
			_, _, err := boundedGit(root, 0, "merge-base", "--is-ancestor", "--end-of-options", h, r.RefCommit)
			var exit *exec.ExitError
			switch {
			case err == nil:
				r.HeadOnRef = boolp(true)
			case errors.As(err, &exit) && exit.ExitCode() == 1:
				r.HeadOnRef = boolp(false)
			}
		}
	}
	return obs
}

// resolveBranch is the TLC-V0-003 branch resolution, or "" when no ref
// matches.
func resolveBranch(refs map[string]string, branch string) string {
	if branch == "" {
		return ""
	}
	if strings.HasPrefix(branch, "refs/") {
		if _, ok := refs[branch]; ok {
			return branch
		}
		return ""
	}
	for _, name := range []string{"refs/heads/" + branch, "refs/remotes/origin/" + branch} {
		if _, ok := refs[name]; ok {
			return name
		}
	}
	match := ""
	for name := range refs {
		rest, ok := strings.CutPrefix(name, "refs/remotes/")
		if _, b, cut := strings.Cut(rest, "/"); ok && cut && b == branch {
			if match != "" {
				return ""
			}
			match = name
		}
	}
	return match
}

// validHead admits a hexadecimal object name only, so no batch input can be
// read as a revision expression.
func validHead(h string) bool {
	if len(h) < 4 || len(h) > 64 {
		return false
	}
	for _, c := range h {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// completeLines splits out into lines, dropping a final line that a
// truncated read cut short.
func completeLines(out []byte, truncated bool) []string {
	s := string(out)
	if truncated {
		s = s[:max(strings.LastIndexByte(s, '\n'), 0)]
	}
	return strings.Split(s, "\n")
}

// lineSet is the set of the first field of each complete output line.
func lineSet(out []byte, truncated bool) map[string]bool {
	set := map[string]bool{}
	for _, line := range completeLines(out, truncated) {
		if f, _, _ := strings.Cut(line, " "); f != "" {
			set[f] = true
		}
	}
	return set
}

// deliveryGit runs one read-only Git process with stdin, keeping at most
// max output bytes; over reports that more was produced.
func deliveryGit(root, stdin string, max int64, args ...string) (out []byte, over bool, err error) {
	c := exec.Command("git", append(append([]string{}, preflightGitConfig...), append([]string{"-c", "credential.helper="}, args...)...)...)
	c.Dir, c.Env = root, gitEnvironment()
	c.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	pipe, err := c.StdoutPipe()
	if err != nil {
		return nil, false, err
	}
	if err = c.Start(); err != nil {
		return nil, false, err
	}
	out, readErr := io.ReadAll(io.LimitReader(pipe, max+1))
	if over = int64(len(out)) > max; over {
		out = out[:max]
		_ = c.Process.Kill()
	}
	waitErr := c.Wait()
	switch {
	case over:
		return out, true, nil
	case readErr != nil:
		return nil, false, readErr
	}
	return out, false, waitErr
}
