package appmap

import (
	"runtime/debug"
	"strings"
)

// builderIdentity is the Builder a map this binary compiles records and the one `--from` requires
// (AMAP-V0-215): BuilderVersion, plus the VCS revision the binary was built from when the build
// recorded one, so a compiler change that did not bump BuilderVersion still forces a full rebuild.
var builderIdentity = builderOf(debug.ReadBuildInfo())

// builderOf derives the builder identity from build information. A binary built from a modified
// tree gets a `-dirty` suffix, and a dirty builder never carries a map forward.
func builderOf(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return BuilderVersion
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return BuilderVersion
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	id := BuilderVersion + "+" + rev
	if dirty {
		id += "-dirty"
	}
	return id
}

// builderReuses reports whether a map prev's builder wrote may be carried forward by this binary.
func builderReuses(prevBuilder, current string) bool {
	return prevBuilder == current && !strings.HasSuffix(current, "-dirty")
}
