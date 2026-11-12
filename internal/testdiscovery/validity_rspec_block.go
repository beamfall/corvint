package testdiscovery

// RSpecHelper is one `def` read directly in an example group or at file level: its group and
// its logical lines, the head and the closing `end` included.
type RSpecHelper struct {
	Group int
	Path  string
	Body  []RubyLine
}
