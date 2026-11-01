package store

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"slices"
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

// TLC-V0-006 budgets: walk output bytes, fallback ancestry checks and the
// explicit refs named one by one before the listing widens to all of refs/.
const (
	deliveryWalkMaxBytes   = 16 << 20
	deliveryRefsMaxBytes   = 32 << 20
	deliveryAncestryChecks = 16
	deliveryExactRefs      = 64
)

func boolp(b bool) *bool { return &b }

// ObserveDeliveries checks every query against the refs of root with a fixed
// number of Git processes (TLC-V0-006): one ref listing, one object check,
// one integration walk, one shallow check when that walk leaves a head
// unreached, one walk over the distinct branch-ref commits and at most
// deliveryAncestryChecks bounded ancestry checks. Every walk starts from the
// commits the listing captured, in sorted order. It never writes.
func ObserveDeliveries(root, integrationBranch string, qs []DeliveryQuery) DeliveryObservation {
	var obs DeliveryObservation
	run := func(stdin string, max int64, args ...string) ([]byte, bool, error) {
		obs.GitCalls++
		return deliveryGit(root, stdin, max, args...)
	}
	listing := append([]string{"for-each-ref", "--format=%(objectname) %(refname)", "refs/heads/", "refs/remotes/"}, exactRefPatterns(qs)...)
	refsOut, over, err := run("", deliveryRefsMaxBytes, listing...)
	if err != nil || over {
		return DeliveryObservation{Failed: true, GitCalls: obs.GitCalls}
	}
	refs := newRefIndex(refsOut)
	for _, name := range []string{"refs/remotes/origin/" + integrationBranch, "refs/heads/" + integrationBranch} {
		if oid, ok := refs.oids[name]; ok && integrationBranch != "" {
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
		if ref := refs.resolve(qs[i].Branch); ref != "" {
			obs.Results[i].Ref, obs.Results[i].RefCommit = ref, refs.oids[ref]
		}
	}
	// Integration walk: a head the walk from heads ^tip emits is not
	// integrated, unless the history is shallow and the head may lie past
	// the shallow boundary; then it is not observed (TLC-V0-004).
	if obs.IntegrationCommit != "" {
		var in strings.Builder
		for _, h := range sortedSet(heads) {
			in.WriteString(h + "\n")
		}
		if in.Len() > 0 {
			in.WriteString("^" + obs.IntegrationCommit + "\n")
			out, over, err := run(in.String(), deliveryWalkMaxBytes, "rev-list", "--stdin")
			if err != nil {
				return DeliveryObservation{Failed: true, GitCalls: obs.GitCalls}
			}
			emitted := lineSet(out, over)
			shallow := false
			if len(emitted) > 0 {
				out, _, err := run("", 64, "rev-parse", "--is-shallow-repository")
				if err != nil {
					return DeliveryObservation{Failed: true, GitCalls: obs.GitCalls}
				}
				shallow = strings.TrimSpace(string(out)) != "false"
			}
			for i, h := range heads {
				switch {
				case h == "":
				case emitted[h]:
					if !shallow {
						obs.Results[i].Integrated = boolp(false)
					}
				case !over:
					obs.Results[i].Integrated = boolp(true)
				}
			}
		}
	}
	// Branch walk over the distinct ref commits of non-integrated heads. An
	// integrated head is never pending, so it keeps headOnRef null.
	pending := make([]bool, len(qs))
	var starts []string
	for i, h := range heads {
		r := &obs.Results[i]
		switch {
		case h == "" || r.Ref == "" || r.Integrated == nil || *r.Integrated:
		case r.RefCommit == h:
			r.HeadOnRef = boolp(true)
		default:
			pending[i] = true
			starts = append(starts, r.RefCommit)
		}
	}
	if len(starts) == 0 {
		return obs
	}
	var in strings.Builder
	for _, oid := range sortedSet(starts) {
		in.WriteString(oid + "\n")
	}
	in.WriteString("^" + obs.IntegrationCommit + "\n")
	out, over, err := run(in.String(), deliveryWalkMaxBytes, "log", "--no-show-signature", "--no-color", "--source", "--format=%H %S", "--stdin")
	if err != nil {
		return DeliveryObservation{Failed: true, GitCalls: obs.GitCalls}
	}
	source := map[string]string{}
	for _, line := range completeLines(out, over) {
		if oid, start, ok := strings.Cut(line, " "); ok {
			source[oid] = start
		}
	}
	checks := 0
	for i, h := range heads {
		r := &obs.Results[i]
		if !pending[i] {
			continue
		}
		src, seen := source[h]
		switch {
		case seen && src == r.RefCommit:
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

// refIndex is one observation's ref listing: each ref's object name, and
// each remote-tracking short name mapped to its only ref, or "" when two
// remotes share it. It is built once, so resolution never rescans the refs.
type refIndex struct {
	oids, remote map[string]string
}

func newRefIndex(listing []byte) refIndex {
	x := refIndex{oids: map[string]string{}, remote: map[string]string{}}
	for _, line := range strings.Split(string(listing), "\n") {
		oid, name, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		x.oids[name] = oid
		if rest, ok := strings.CutPrefix(name, "refs/remotes/"); ok {
			if _, b, cut := strings.Cut(rest, "/"); cut {
				if _, dup := x.remote[b]; dup {
					x.remote[b] = ""
				} else {
					x.remote[b] = name
				}
			}
		}
	}
	return x
}

// resolve is the TLC-V0-003 branch resolution, or "" when no ref matches.
func (x refIndex) resolve(branch string) string {
	if branch == "" {
		return ""
	}
	if strings.HasPrefix(branch, "refs/") {
		if _, ok := x.oids[branch]; ok {
			return branch
		}
		return ""
	}
	for _, name := range []string{"refs/heads/" + branch, "refs/remotes/origin/" + branch} {
		if _, ok := x.oids[name]; ok {
			return name
		}
	}
	return x.remote[branch]
}

// exactRefPatterns are the distinct, sorted explicit refs/ branches of qs
// that the default listing of refs/heads/ and refs/remotes/ misses, so an
// exact ref in any namespace resolves (TLC-V0-003). A name with a pattern
// metacharacter or whitespace is no valid ref and is left unresolved. Past
// deliveryExactRefs names the listing takes all of refs/ instead, which keeps
// the argument list bounded.
func exactRefPatterns(qs []DeliveryQuery) []string {
	var names []string
	for _, q := range qs {
		b := q.Branch
		if strings.HasPrefix(b, "refs/") && !strings.HasPrefix(b, "refs/heads/") && !strings.HasPrefix(b, "refs/remotes/") &&
			!strings.ContainsFunc(b, func(c rune) bool { return c <= ' ' || c == 0x7f || strings.ContainsRune("*?[\\", c) }) {
			names = append(names, b)
		}
	}
	if names = sortedSet(names); len(names) > deliveryExactRefs {
		return []string{"refs/"}
	}
	return names
}

// sortedSet is the distinct non-empty members of xs in sorted order.
func sortedSet(xs []string) []string {
	out := slices.Sorted(slices.Values(xs))
	out = slices.Compact(out)
	if len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	return out
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
