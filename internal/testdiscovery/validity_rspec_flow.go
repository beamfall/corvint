package testdiscovery

// RubyText is one logical line with its string and percent literals masked: masked has the same
// length as raw, with every character inside a literal replaced by \x01, so a pattern matched on
// masked never matches literal content and its positions slice raw. A `#{...}` interpolation in a
// double-quoted string stays code, so its names are read.
type RubyText struct {
	Line        int
	Raw, Masked string
}

// rspecFlow indexes the file for TSA-V0-120..130, built on first use: each line's masked text and
// words, the example owning each body line, and each group's subject names.
type rspecFlow struct {
	Text     []RubyText
	Words    [][]string
	Owner    map[int]int
	Subjects map[int]map[string]bool
}
