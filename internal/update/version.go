package update

import (
	"strconv"
	"strings"
)

type semVersion struct {
	major int
	minor int
	patch int
}

func compareVersions(current, latest string) (int, bool) {
	a, ok := parseSemVersion(current)
	if !ok {
		return 0, false
	}
	b, ok := parseSemVersion(latest)
	if !ok {
		return 0, false
	}
	switch {
	case a.major != b.major:
		if a.major < b.major {
			return -1, true
		}
		return 1, true
	case a.minor != b.minor:
		if a.minor < b.minor {
			return -1, true
		}
		return 1, true
	case a.patch != b.patch:
		if a.patch < b.patch {
			return -1, true
		}
		return 1, true
	default:
		return 0, true
	}
}

func parseSemVersion(raw string) (semVersion, bool) {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "v")
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return semVersion{}, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semVersion{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semVersion{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return semVersion{}, false
	}
	return semVersion{major: major, minor: minor, patch: patch}, true
}

func canonicalVersionTag(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if _, ok := parseSemVersion(value); ok && !strings.HasPrefix(value, "v") {
		return "v" + value
	}
	return value
}

func updateStatus(current, latest string) (string, bool) {
	current = strings.TrimSpace(current)
	latest = strings.TrimSpace(latest)
	if latest == "" {
		return "unknown", false
	}
	if current == latest {
		return "up_to_date", false
	}
	if cmp, ok := compareVersions(current, latest); ok {
		if cmp < 0 {
			return "update_available", true
		}
		return "up_to_date", false
	}
	if current == "" {
		return "update_available", true
	}
	return "update_available", true
}
