// Package symbolinfo separates raw identity from bounded display resolution.
package symbolinfo

import (
	"github.com/ianlancetaylor/demangle"
	"regexp"
	"strings"
)

const Version = "1.0.0"
const ResolverVersion = "v0.0.0-20261012173340-cbb8aaea4290"

type Info struct {
	Raw        string `json:"raw"`
	Display    string `json:"display"`
	Runtime    string `json:"runtime"`
	Language   string `json:"language"`
	Basis      string `json:"basis"`
	Confidence string `json:"confidence"`
	Resolver   string `json:"resolver"`
	Version    string `json:"resolver_version"`
}

var rustLegacy = regexp.MustCompile(`^_?ZN.*17h[0-9a-f]{16}E$`)

func Describe(raw, file, declared string) Info {
	v := Info{Raw: raw, Display: raw, Runtime: "unknown", Language: "unknown", Basis: "no_language_evidence", Confidence: "unknown", Resolver: "none", Version: Version}
	if len(raw) > 1024 {
		return v
	}
	symbol := raw
	if strings.HasPrefix(raw, "_R") {
		symbol = raw
	} else if strings.HasPrefix(raw, "__R") {
		symbol = raw[1:]
	} else if strings.HasPrefix(raw, "__ZN") {
		symbol = raw[1:]
	}
	rust := strings.HasPrefix(symbol, "_R") || rustLegacy.MatchString(symbol)
	if rust {
		display, e := demangle.ToString(symbol, demangle.MaxLength(10))
		if e == nil && display != "" && len(display) <= 1024 {
			v.Display = display
			v.Runtime = "Rust"
			v.Language = "Rust"
			v.Basis = "validated_rust_mangling"
			v.Confidence = "encoded"
			v.Resolver = "ianlancetaylor/demangle"
			v.Version = ResolverVersion
			return v
		}
		v.Basis = "unresolved_rust_mangling"
		v.Confidence = "candidate"
	}
	if strings.EqualFold(declared, "rust") || strings.HasSuffix(strings.ToLower(file), ".rs") {
		v.Runtime = "Rust"
		v.Language = "Rust"
		v.Basis = "declared_language_or_rust_source"
		v.Confidence = "declared"
	} else if strings.Contains(raw, "::") && v.Confidence == "unknown" {
		v.Basis = "namespace_syntax_is_not_language"
	}
	return v
}
