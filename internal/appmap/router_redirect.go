// SPDX-License-Identifier: Apache-2.0
package appmap

import (
	"path"
	"strings"
)

// followRedirect resolves bounded literal Navigate chains without executing app code.
func (x *screenIndex) followRedirect(raw string, seen map[string]bool) ([]string, string) {
	ids, reason := x.directURLMatches(raw)
	if reason != "" || len(ids) != 1 {
		return ids, reason
	}
	route, ok := x.redirects[ids[0]]
	if !ok {
		return ids, ""
	}
	if seen[route.ID] || len(seen) >= 32 {
		return nil, "redirect-cycle-or-bound"
	}
	seen[route.ID] = true
	target, ok := x.redirectPath(route)
	if !ok {
		return nil, "non-literal-redirect-target"
	}
	// Rebuild the router URL with its hash prefix before the route path.
	// Otherwise urlPath treats a route fragment as an ordinary page fragment.
	prefix := x.basePath
	if x.hashPrefix != "" {
		prefix += "/" + x.hashPrefix
	}
	return x.followRedirect(prefix+target, seen)
}

func (x *screenIndex) redirectPath(route Screen) (string, bool) {
	if route.Redirect == nil || route.Redirect.To == nil {
		return "", false
	}
	to := *route.Redirect.To
	// External URLs and unresolved parameters are never invented destinations.
	if strings.HasPrefix(to, "//") || urlScheme.MatchString(to) || strings.Contains(strings.SplitN(to, "/", 2)[0], ":") || strings.ContainsAny(to, "\\\x00") {
		return "", false
	}
	// A hash router may name its route in the fragment itself. Preserve that
	// route before removing an ordinary query or page fragment.
	if x.hashPrefix != "" {
		if i := strings.Index(to, x.hashPrefix); i >= 0 && strings.Trim(to[:i], "/") == "" {
			to = to[i+len(x.hashPrefix):]
		}
	}
	to, _ = splitQuery(to)
	if i := strings.IndexByte(to, '#'); i >= 0 {
		to = to[:i]
	}
	if strings.HasPrefix(to, "/") {
		return path.Clean(to), true
	}
	base := route.Template
	if strings.ContainsAny(base, "{}*") {
		return "", false
	}
	if !route.Redirect.PathRelative {
		cur := route
		for to == ".." || strings.HasPrefix(to, "../") {
			to = strings.TrimPrefix(strings.TrimPrefix(to, ".."), "/")
			// Pathless and index routes do not contribute a resolution level.
			visited := map[string]bool{}
			for {
				if visited[cur.ID] {
					return "", false
				}
				visited[cur.ID] = true
				parent, exists := x.screens[cur.Parent]
				if !exists {
					base = "/"
					break
				}
				cur = parent
				if parent.Template != base {
					base = parent.Template
					break
				}
			}
		}
	}
	return path.Join(base, to), true
}

// attachRedirectFreshness joins every resolved alias chain's sources to its
// target. A changed alias must invalidate the resulting attributed screen.
func attachRedirectFreshness(hashPrefix string, screens []Screen) {
	x := newScreenIndex(hashPrefix, "", screens)
	byID := map[string]int{}
	for i, s := range screens {
		byID[s.ID] = i
	}
	for _, s := range screens {
		if s.Redirect == nil {
			continue
		}
		seen := map[string]bool{}
		ids, reason := x.followRedirect(s.Template, seen)
		if reason != "" || len(ids) != 1 {
			continue
		}
		target := &screens[byID[ids[0]]]
		// Iterate screen order, not map order, for deterministic artifact bytes.
		for _, r := range screens {
			if !seen[r.ID] {
				continue
			}
			anchors := append(append([]Anchor{r.Anchor}, r.NameFrom...), r.ParentFrom...)
			for _, a := range anchors {
				found := false
				for _, existing := range target.RedirectFrom {
					if existing == a {
						found = true
						break
					}
				}
				if !found {
					target.RedirectFrom = append(target.RedirectFrom, a)
				}
			}
		}
	}
}
