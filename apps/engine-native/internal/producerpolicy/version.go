// Package producerpolicy owns explicit, structural-only producer qualification.
// It never infers compatibility or changes export-schema requirements.
package producerpolicy

import (
	"fmt"
	"regexp"
)

const Unverified = "unverified_version"
const Limitation = "Producer version is unverified: required fields and value shapes were checked, not producer/version compatibility. Actual or unknown version is retained; do not substitute a qualified version."

var versionText = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.+:-]{0,127}$`)

func Select(actual, qualified string, allow bool) (version, qualification string, err error) {
	if actual == qualified {
		return actual, "", nil
	}
	if !allow {
		return "", "", fmt.Errorf("PRODUCER_VERSION_UNQUALIFIED: declare actual producer_version (or unknown) and explicitly set allow_unverified_version=true")
	}
	if actual == "" {
		actual = "unknown"
	}
	if !versionText.MatchString(actual) {
		return "", "", fmt.Errorf("PRODUCER_VERSION_INVALID")
	}
	return actual, Unverified, nil
}

func Valid(version, qualification string) bool {
	return qualification == "" || qualification == Unverified && versionText.MatchString(version)
}
