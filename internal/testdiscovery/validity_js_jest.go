package testdiscovery

// noParams reports a callback at token i that declares no parameter: `() =>`, `async () =>`,
// `(): T =>`, `function () {` or `function name() {`.
func (s *Source) noParams(i int) bool {
	if s.Ident(i, "async") {
		i++
	}
	if s.Ident(i, "function") {
		i++
		if s.Ident(i, "") {
			i++
		}
	}
	return s.Punct(i, "(") && s.Match[i] == i+1
}
