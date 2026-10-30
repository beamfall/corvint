package appmap

import "strings"

// mountRouters places each mounted React router's states under the declared splat route that
// renders them (AMAP-V0-192), as react-router matches descendant `<Routes>`: every path of the
// mounted file, with or without a leading slash, is relative to the mount's path without its `/*`;
// top-level states take the mount route as their parent; and the mount route becomes abstract,
// since the descendant routes, not the splat, own the URLs below it. Each moved state carries the
// mount router's anchor as a parent binding, so a change to either file leaves it not FRESH. A
// mount path that the named router does not declare exactly once, other than as a redirect,
// refuses the manifest. A mount path names the route as its own router declares it; a router
// mounted into a mounted router is placed after its host, under the host route's placed path.
func mountRouters(routers []RouterFile, raws []rawState, spans [][2]int) error {
	declared := make([]string, len(raws))
	for k := range raws {
		declared[k] = raws[k].url
	}
	index := map[string]int{}
	for i, r := range routers {
		index[r.Path] = i
	}
	done := make([]bool, len(routers))
	var place func(i int) error
	place = func(i int) error {
		if done[i] {
			return nil
		}
		done[i] = true
		mt := routers[i].Mount
		if mt == nil {
			return nil
		}
		hostRouter := index[mt.Router]
		if err := place(hostRouter); err != nil {
			return err
		}
		at := -1
		for k := spans[hostRouter][0]; k < spans[hostRouter][1]; k++ {
			if declared[k] != "^"+mt.Path || raws[k].redirect {
				continue
			}
			if at >= 0 {
				return invalidManifest("router %s: mount path %s is declared more than once in %s", routers[i].Path, mt.Path, mt.Router)
			}
			at = k
		}
		if at < 0 {
			return invalidManifest("router %s: mount path %s is not a route of %s", routers[i].Path, mt.Path, mt.Router)
		}
		host := &raws[at]
		host.abstract = true
		prefix := strings.TrimSuffix(strings.TrimPrefix(host.url, "^"), "/*")
		for k := spans[i][0]; k < spans[i][1]; k++ {
			s := &raws[k]
			if s.parent == "" {
				s.parent = host.name
			}
			own := strings.TrimPrefix(s.url, "^")
			switch {
			case own != "/" && own != "":
				s.url = "^" + routeJoin(prefix, own)
			case prefix != "":
				s.url = "^" + prefix
			default:
				s.url = "^/"
			}
			s.parentFrom = append(s.parentFrom, host.anchor)
		}
		return nil
	}
	for i := range routers {
		if err := place(i); err != nil {
			return err
		}
	}
	return nil
}
