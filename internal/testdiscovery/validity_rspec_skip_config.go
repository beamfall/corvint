package testdiscovery

// RSpecExclusion is one metadata filter a committed support file excludes from every run.
type RSpecExclusion struct {
	Key, Value string // value "" matches any value of key
	Cause      string // path:line of the filter_run_excluding call
}
