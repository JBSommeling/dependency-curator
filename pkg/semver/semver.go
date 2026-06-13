package semver

import (
	"fmt"
	"strconv"
	"strings"
)

type UpdateType string

const (
	Patch UpdateType = "patch"
	Minor UpdateType = "minor"
	Major UpdateType = "major"
	None  UpdateType = "none"
)

func (u UpdateType) String() string {
	return string(u)
}

type Version struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Raw        string
}

func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		s += "-" + v.PreRelease
	}
	return s
}

func Parse(s string) (Version, error) {
	raw := s
	// Strip common range prefixes
	s = strings.TrimSpace(s)
	for _, prefix := range []string{">=", "<=", "=>", "=<", "^", "~", ">", "<", "=", "v"} {
		s = strings.TrimPrefix(s, prefix)
	}
	s = strings.TrimSpace(s)

	if s == "" {
		return Version{}, fmt.Errorf("empty version string")
	}

	// Split off pre-release
	preRelease := ""
	if idx := strings.Index(s, "-"); idx != -1 {
		preRelease = s[idx+1:]
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return Version{}, fmt.Errorf("invalid version format: %s", raw)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor := 0
	if len(parts) >= 2 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return Version{}, fmt.Errorf("invalid minor version: %s", parts[1])
		}
	}

	patch := 0
	if len(parts) >= 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return Version{}, fmt.Errorf("invalid patch version: %s", parts[2])
		}
	}

	return Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
		Raw:        raw,
	}, nil
}

// Compare returns -1 if a < b, 0 if a == b, 1 if a > b.
// Pre-release versions have lower precedence than the associated normal version.
func Compare(a, b Version) int {
	if a.Major != b.Major {
		return cmpInt(a.Major, b.Major)
	}
	if a.Minor != b.Minor {
		return cmpInt(a.Minor, b.Minor)
	}
	if a.Patch != b.Patch {
		return cmpInt(a.Patch, b.Patch)
	}
	// Both have no pre-release: equal
	if a.PreRelease == "" && b.PreRelease == "" {
		return 0
	}
	// Pre-release has lower precedence
	if a.PreRelease != "" && b.PreRelease == "" {
		return -1
	}
	if a.PreRelease == "" && b.PreRelease != "" {
		return 1
	}
	// Both have pre-release: lexicographic
	return strings.Compare(a.PreRelease, b.PreRelease)
}

func ClassifyUpdate(current, latest Version) UpdateType {
	if Compare(current, latest) >= 0 {
		return None
	}
	if latest.Major > current.Major {
		return Major
	}
	if latest.Minor > current.Minor {
		return Minor
	}
	return Patch
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	return 1
}
