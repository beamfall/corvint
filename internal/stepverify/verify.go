package stepverify

import (
	"context"
	"sort"
	"strings"
)

func matches(paths []string, path string) bool {
	for _, p := range paths {
		if path == p || strings.HasSuffix(p, "/") && (path == strings.TrimSuffix(p, "/") || strings.HasPrefix(path, p)) {
			return true
		}
	}
	return false
}
func changed(a, b []Entry) []string {
	left := map[string]Entry{}
	right := map[string]Entry{}
	paths := map[string]bool{}
	for _, e := range a {
		left[e.Path] = e
		paths[e.Path] = true
	}
	for _, e := range b {
		right[e.Path] = e
		paths[e.Path] = true
	}
	out := []string{}
	for p := range paths {
		if x, ok := left[p]; !ok {
			out = append(out, p)
		} else if y, ok := right[p]; !ok || x != y {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}
func anchored(paths []string) []string {
	set := map[string]bool{".": true}
	for _, p := range paths {
		parts := strings.Split(strings.TrimSuffix(p, "/"), "/")
		if !strings.HasSuffix(p, "/") {
			parts = parts[:len(parts)-1]
		}
		for i := range parts {
			set[strings.Join(parts[:i+1], "/")] = true
		}
	}
	out := []string{}
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Verify accepts a host-protected complete before state and recaptures the
// actual post-state. It never accepts an author's claimed post-state digest.
func Verify(ctx context.Context, d Declaration, h Host, before State) (Receipt, int, error) {
	d, h, err := inputs(d, h)
	if err != nil {
		return Receipt{}, 2, err
	}
	before, err = DecodeState(Encode(before), d, h)
	if err != nil {
		return Receipt{}, 2, err
	}
	after, captureErr := Snapshot(ctx, d, h)
	if after.Checkouts == nil {
		after.Checkouts = []CheckoutState{}
	}
	r := Receipt{Profile: "corvint-step-receipt/0", DeclarationDigest: DeclarationDigest(d), HostDigest: digest(h), BeforeDigest: before.Digest, Before: before.Checkouts, After: after.Checkouts, WriteScopeVerdict: "PASS", Findings: []Finding{}, Unknowns: []string{"HOST_ASSERTIONS_UNAUTHENTICATED", "EXTERNAL_AND_TRANSIENT_WRITES_NOT_OBSERVED", "CONCURRENCY_NOT_ATTESTED", "ADMIN_OBJECTS_LOGS_OTHER_WORKTREES_AND_PRIVATE_LEDGERS_HOST_PROTECTED_NOT_INVENTORIED", "ACL_XATTR_AND_TIMESTAMP_CHANGES_NOT_OBSERVED"}}
	r.EnvironmentVerdict, r.Findings = Environment(d, h)
	if after.Complete {
		value := after.Digest
		r.AfterDigest = &value
	}
	for i, a := range after.Checkouts {
		if i >= len(before.Checkouts) {
			return Receipt{}, 2, ErrInput
		}
		b := before.Checkouts[i]
		if a.ID != b.ID {
			return Receipt{}, 2, ErrInput
		}
		add := func(code, path string) { r.Findings = append(r.Findings, Finding{code, a.ID, path}) }
		if a.ContentComplete {
			for _, path := range changed(b.Entries, a.Entries) {
				if a.ID != d.Author.ID {
					add("READ_ONLY_CHANGED", path)
				} else if matches(d.GuardPaths, path) {
					add("GUARDED_WRITE", path)
				} else if !matches(d.WritePaths, path) {
					add("OUT_OF_SCOPE_WRITE", path)
				}
			}
			if a.ID == d.Author.ID {
				old := map[string]Entry{}
				now := map[string]Entry{}
				for _, e := range b.Entries {
					old[e.Path] = e
				}
				for _, e := range a.Entries {
					now[e.Path] = e
				}
				for _, path := range anchored(append(append([]string{}, d.WritePaths...), d.GuardPaths...)) {
					if e, ok := old[path]; ok && e.Kind == "DIRECTORY" {
						if n, ok := now[path]; !ok || n.Kind != "DIRECTORY" || e.Identity != n.Identity || e.Mode != n.Mode {
							add("SCOPE_ANCESTOR_CHANGED", path)
						}
					}
				}
			}
		}
		if a.Complete {
			if a.Commit != b.Commit || a.Tree != b.Tree {
				add("COMMIT_CHANGED", ".git/HEAD")
			}
			if a.IndexDigest != b.IndexDigest {
				add("INDEX_CHANGED", ".git/index")
			}
			for _, path := range changed(b.AdminEntries, a.AdminEntries) {
				add("ADMIN_CHANGED", path)
				if a.ID != d.Author.ID {
					add("READ_ONLY_CHANGED", path)
				}
			}
			if a.GitDir != b.GitDir || a.CommonDir != b.CommonDir {
				add("ADMIN_CHANGED", ".git")
			}
		} else {
			for _, unknown := range a.Unknowns {
				r.Unknowns = append(r.Unknowns, unknown)
			}
		}
	}
	for _, f := range r.Findings {
		if !strings.HasPrefix(f.Code, "ENVIRONMENT_") {
			r.WriteScopeVerdict = "FAIL"
		}
	}
	code := 0
	if r.WriteScopeVerdict == "FAIL" || r.EnvironmentVerdict == "FAIL" {
		code = 1
	}
	if captureErr != nil {
		r.WriteScopeVerdict = "UNSUPPORTED"
		r.Unknowns = append(r.Unknowns, "POST_STATE_INCOMPLETE")
		code = 2
	}
	sort.Slice(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if a.Checkout != b.Checkout {
			return a.Checkout < b.Checkout
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Code < b.Code
	})
	sort.Strings(r.Unknowns)
	r.Unknowns = unique(r.Unknowns)
	if err := sealReceipt(&r); err != nil {
		return Receipt{}, 2, err
	}
	return r, code, nil
}

func sealReceipt(r *Receipt) error {
	r.Digest = ""
	if len(Encode(r))+64 > MaxRecordBytes {
		return ErrUnsupported
	}
	r.Digest = digest(r)
	_, err := BoundedEncode(r)
	return err
}
