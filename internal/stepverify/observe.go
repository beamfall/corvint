package stepverify

import (
	"context"
	"sort"
)

func inputs(d Declaration, h Host) (Declaration, Host, error) {
	d, err := DecodeDeclaration(Encode(d))
	if err != nil {
		return d, h, err
	}
	h, err = DecodeHost(Encode(h), d)
	return d, h, err
}
func pinMatches(ctx context.Context, d Declaration, h Host) bool {
	data, err := ReadInput(ctx, h.GitBinary, &d)
	return err == nil && hash(data) == h.GitBinaryDigest
}
func capturePass(ctx context.Context, d Declaration, h Host, pinned bool) State {
	s := State{Profile: "corvint-step-state/0", DeclarationDigest: DeclarationDigest(d), HostDigest: digest(h), SessionID: d.SessionID, CapabilityID: d.CapabilityID, Checkouts: []CheckoutState{}, Complete: true}
	b := &inventoryBudget{}
	seen := map[string]bool{}
	for _, checkout := range append([]Checkout{d.Author}, d.ReadOnly...) {
		c := CheckoutState{ID: checkout.ID, Root: checkout.Root, Entries: []Entry{}, AdminEntries: []Entry{}, Unknowns: []string{}}
		rows, _, err := inventory(ctx, checkout.Root, b, true)
		c.Entries = rows
		c.ContentDigest = digest(rows)
		c.ContentComplete = err == nil
		if err != nil {
			c.Unknowns = append(c.Unknowns, "CONTENT_UNSUPPORTED")
		}
		if len(rows) > 0 && rows[0].Path == "." {
			if seen[rows[0].Identity] {
				c.Unknowns = append(c.Unknowns, "CHECKOUT_ALIAS_UNSUPPORTED")
			}
			seen[rows[0].Identity] = true
		}
		if checkout.ID == d.Author.ID && scopeAncestors(checkout.Root, append(append([]string{}, d.WritePaths...), d.GuardPaths...)) != nil {
			c.Unknowns = append(c.Unknowns, "SCOPE_ANCESTOR_UNSUPPORTED")
		}
		if pinned {
			c.AdminEntries, c.GitDir, c.CommonDir, c.Commit, c.Tree, err = admin(ctx, checkout, h, b)
		} else {
			err = ErrUnsupported
			c.Unknowns = append(c.Unknowns, "GIT_PIN_UNAVAILABLE")
		}
		c.AdminDigest = digest(c.AdminEntries)
		c.IndexDigest = indexDigest(c.AdminEntries)
		if err != nil {
			c.Unknowns = append(c.Unknowns, "ADMIN_UNSUPPORTED")
		}
		c.Complete = c.ContentComplete && len(c.Unknowns) == 0
		s.Complete = s.Complete && c.Complete
		s.Checkouts = append(s.Checkouts, c)
	}
	return s
}

// Snapshot takes repeated observations of raw filesystem/admin state. Its
// race witnesses detect available drift; they do not make the capture atomic.
func Snapshot(ctx context.Context, d Declaration, h Host) (State, error) {
	d, h, err := inputs(d, h)
	if err != nil {
		return State{}, err
	}
	pinned := pinMatches(ctx, d, h)
	first := capturePass(ctx, d, h, pinned)
	second := capturePass(ctx, d, h, pinned)
	for i := range second.Checkouts {
		before := first.Checkouts[i]
		after := &second.Checkouts[i]
		if !before.ContentComplete || before.ContentDigest != after.ContentDigest {
			after.ContentComplete = false
			after.Complete = false
			after.Unknowns = append(after.Unknowns, "CONTENT_DRIFT")
		}
		if !before.Complete || before.AdminDigest != after.AdminDigest || before.Commit != after.Commit || before.Tree != after.Tree || before.GitDir != after.GitDir || before.CommonDir != after.CommonDir {
			after.Complete = false
			after.Unknowns = append(after.Unknowns, "ADMIN_OR_BINDING_DRIFT")
		}
		sort.Strings(after.Unknowns)
		after.Unknowns = unique(after.Unknowns)
		second.Complete = second.Complete && after.Complete
	}
	if !pinMatches(ctx, d, h) {
		second.Complete = false
		for i := range second.Checkouts {
			second.Checkouts[i].Complete = false
			second.Checkouts[i].Unknowns = append(second.Checkouts[i].Unknowns, "GIT_BINARY_DRIFT")
		}
	}
	if second.Complete {
		second.Digest = stateDigest(second)
	}
	if len(Encode(second)) > MaxRecordBytes {
		return State{}, ErrUnsupported
	}
	if !second.Complete {
		return second, ErrUnsupported
	}
	return second, nil
}
func unique(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}
