package testdiscovery

// RSpecSupport is the support set a reader resolves helpers against: the readers of the bound
// support files by path, the files that did not read with their unread readers, and the reason
// the set is unbound.
type RSpecSupport struct {
	Readers  map[string]*RSpecReader
	Unread   []string
	UnreadBy map[string]*RSpecReader
	Unbound  string
}
