package analyzerruby

import "github.com/Beamfall/corvint/internal/rubysyntax"

// maskRuby returns the masked code view of `ruby.source` (rubysyntax.Mask): every comment,
// string, heredoc, percent literal, regex and `__END__` data section blanked to spaces, so only
// that view is searched for the closed token set. An unterminated literal fails closed.
func maskRuby(src []byte) ([]byte, string) { return rubysyntax.Mask(src) }

// identifierByte reports whether b can continue a Ruby identifier (rubysyntax.IdentifierByte).
func identifierByte(b byte) bool { return rubysyntax.IdentifierByte(b) }
