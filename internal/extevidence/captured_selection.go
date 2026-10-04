package extevidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
)

// CapturedRecord owns a bounded copy: later file or caller mutations cannot
// change the bytes used for decoding, selection, binding or assertion facts.
type CapturedRecord struct {
	data    []byte
	digest  string
	failure bool
}

func CaptureRecord(data []byte) (CapturedRecord, error) {
	if len(data) > MaxRecordBytes {
		return CapturedRecord{}, errors.New("captured-record-bound")
	}
	sum := sha256.Sum256(data)
	return CapturedRecord{data: append([]byte(nil), data...), digest: hex.EncodeToString(sum[:])}, nil
}
func FailedCapture() CapturedRecord     { return CapturedRecord{failure: true} }
func (c CapturedRecord) Digest() string { return c.digest }

type DeclaredAssertion struct{ Subject, Test, Repository string }
type CapturedSelection struct {
	Selection  map[string]any
	Assertions []DeclaredAssertion
}

// SelectionCaptured has no filename or transport dispatch. All facts come
// from the same immutable captures and the existing strict selector/binder.
func SelectionCaptured(ctx context.Context, dir, revision string, captures []CapturedRecord, checkouts []Checkout, input SelectionInput) CapturedSelection {
	root := headRoot(dir, revision)
	root.changed, root.selecting = input.Changed, true
	providers := make([]provider, 0, len(captures))
	for _, c := range captures {
		entry := provider{source: "capture:sha256:" + c.digest, state: StateUnavailable, reason: "input-capture-unavailable"}
		if !c.failure && c.digest != "" {
			entry = decodeRecord(ctx, root, entry, c.data)
		}
		providers = append(providers, entry)
	}
	repository := repositoryTree(ctx, root, providers)
	bound := bindV1(ctx, root, providers, checkouts)
	s := newSelector(input)
	s.inspect(ctx, bound)
	assertions := []DeclaredAssertion{}
	seen := map[DeclaredAssertion]bool{}
	for _, entry := range providers {
		if entry.state != StateLoaded {
			s.failed++
			s.block("provider-"+entry.state, entry.source, "", entry.reason)
			continue
		}
		v := entry.viewOf(ctx, root, repository)
		s.add(entry, v)
		if !v.pathToPath {
			continue
		}
		for _, link := range v.links {
			if link.relation.Type != "asserts" || !link.from.isPath() || !link.to.isPath() || link.to.isScope() || link.from.isScope() || link.to.repository != v.primary {
				continue
			}
			if _, changed := s.changed[link.to.path]; !changed || s.pathCode(entry, v, link) != "" {
				continue
			}
			a := DeclaredAssertion{Subject: link.to.path, Test: link.from.path, Repository: link.from.repository}
			if !seen[a] {
				seen[a] = true
				assertions = append(assertions, a)
			}
		}
	}
	sort.Slice(assertions, func(i, j int) bool {
		a, b := assertions[i], assertions[j]
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		if a.Repository != b.Repository {
			return a.Repository < b.Repository
		}
		return a.Test < b.Test
	})
	return CapturedSelection{Selection: s.result(providers), Assertions: assertions}
}
