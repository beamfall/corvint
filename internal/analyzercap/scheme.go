package analyzercap

import "strings"

const (
	ExactScheme       Opaque = "exact/1"
	SemverExactScheme Opaque = "semver-2.0.0-exact/1"
	SemverRangeScheme Opaque = "semver-2.0.0-range/1"
)

type SemverRange struct{ Lower, Upper Semver }
type Semver struct{ Major, Minor, Patch uint64 }

func (v Semver) compare(other Semver) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch < other.Patch {
		return -1
	}
	if v.Patch > other.Patch {
		return 1
	}
	return 0
}
func (r SemverRange) Contains(v Semver) bool {
	return r.Lower.compare(v) <= 0 && v.compare(r.Upper) <= 0
}
func ValidateActualScheme(scheme, value Opaque) error {
	if scheme == "" {
		return fail(SchemeMissing)
	}
	if err := mustOpaque(scheme); err != nil {
		return fail(SchemeInvalid)
	}
	switch scheme {
	case ExactScheme:
		if err := mustOpaque(value); err != nil {
			return fail(SchemeInvalid)
		}
		return nil
	case SemverExactScheme:
		if _, ok := parseSemver(string(value)); !ok {
			return fail(SchemeInvalid)
		}
		return nil
	case SemverRangeScheme:
		return fail(SchemeUnsupported)
	case "legacy-equality/1":
		return fail(SchemeUnsupported)
	}
	if strings.HasPrefix(string(scheme), "semver-2.0.0-range/") || strings.HasPrefix(string(scheme), "semver-2.0.0-exact/") {
		suffix := strings.TrimPrefix(strings.TrimPrefix(string(scheme), "semver-2.0.0-range/"), "semver-2.0.0-exact/")
		if isPositiveDecimal(suffix) && suffix != "1" {
			return fail(SchemeFutureVersion)
		}
		return fail(SchemeInvalid)
	}
	return fail(SchemeUnknown)
}
func ValidateRequirementScheme(scheme, value Opaque) error {
	if scheme == SemverRangeScheme {
		_, err := ParseSemverRange(string(value))
		return err
	}
	return ValidateActualScheme(scheme, value)
}
func ParseSemverRange(raw string) (SemverRange, error) {
	if len(raw) > MaxOpaqueBytes || !strings.HasPrefix(raw, ">=") {
		return SemverRange{}, fail(SchemeInvalid)
	}
	parts := strings.Split(raw, " <=")
	if len(parts) != 2 || strings.Count(raw, " ") != 1 {
		return SemverRange{}, fail(SchemeInvalid)
	}
	lower, ok := parseSemver(strings.TrimPrefix(parts[0], ">="))
	if !ok {
		return SemverRange{}, fail(SchemeInvalid)
	}
	upper, ok := parseSemver(parts[1])
	if !ok || lower.compare(upper) > 0 {
		return SemverRange{}, fail(SchemeInvalid)
	}
	return SemverRange{Lower: lower, Upper: upper}, nil
}
func parseSemver(raw string) (Semver, bool) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return Semver{}, false
	}
	values := [3]uint64{}
	for index, part := range parts {
		if !isCanonicalNumber(part) {
			return Semver{}, false
		}
		for _, c := range []byte(part) {
			if values[index] > (^uint64(0)-uint64(c-'0'))/10 {
				return Semver{}, false
			}
			values[index] = values[index]*10 + uint64(c-'0')
		}
	}
	return Semver{Major: values[0], Minor: values[1], Patch: values[2]}, true
}
func isCanonicalNumber(raw string) bool {
	if raw == "0" {
		return true
	}
	if len(raw) == 0 || raw[0] < '1' || raw[0] > '9' {
		return false
	}
	for _, c := range []byte(raw[1:]) {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func isPositiveDecimal(raw string) bool { return isCanonicalNumber(raw) && raw != "0" }
