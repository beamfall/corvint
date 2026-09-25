package appflows

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// OriginsSchema is the repository-owned origins.json in the flows directory (AFU-V1-029).
const OriginsSchema = "application-flow-origins/0"

// Origins lists the origins an observer may perform irreversible or external transitions against.
type Origins struct {
	Schema     string   `json:"schema"`
	Disposable []string `json:"disposable"`
}

// LoadOriginsAt reads origins.json committed in dir at revision, never the working tree. An absent
// file lists no disposable origin.
func LoadOriginsAt(ctx context.Context, root, dir, revision string) (Origins, error) {
	dir = filepath.ToSlash(dir)
	if !safePath(dir) || dir == "." {
		return Origins{}, errors.New("--flows must name a directory inside the repository root")
	}
	rev, err := ResolveRevision(ctx, root, revision)
	if err != nil {
		return Origins{}, err
	}
	entries, err := flowTree(ctx, root, rev, dir)
	if err != nil {
		return Origins{}, err
	}
	at := slices.IndexFunc(entries, func(e treeEntry) bool { return e.name == OriginsFile })
	if at < 0 {
		return Origins{Schema: OriginsSchema, Disposable: []string{}}, nil
	}
	raw, err := committedBlob(ctx, root, entries[at])
	if err != nil {
		return Origins{}, err
	}
	var o Origins
	if err = Decode(raw, &o); err != nil {
		return Origins{}, err
	}
	if o.Schema != OriginsSchema || len(o.Disposable) > maxFlowList || !uniqueTexts(o.Disposable) || slices.ContainsFunc(o.Disposable, notCanonicalOrigin) {
		return Origins{}, errors.New(OriginsFile + ": disposable must list unique canonical http or https origins")
	}
	return o, nil
}

// AdmitTransition is the observer gate: a read or write-reversible transition runs anywhere; any
// other class, including an unknown one, runs only against a listed disposable origin.
func AdmitTransition(o Origins, origin, effectClass string) error {
	if effectRank[effectClass] == 1 || effectRank[effectClass] == 2 {
		return nil
	}
	if !notCanonicalOrigin(origin) && slices.Contains(o.Disposable, origin) {
		return nil
	}
	return &gokernel.Error{Code: "observer-origin-not-disposable",
		Message: "a " + effectClass + " transition needs " + origin + " listed as disposable in " + OriginsFile}
}

// notCanonicalOrigin rejects anything but a lower-case scheme://host[:port] with no user, path,
// query or fragment.
func notCanonicalOrigin(s string) bool {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return true
	}
	return s != u.Scheme+"://"+u.Host || s != strings.ToLower(s)
}
