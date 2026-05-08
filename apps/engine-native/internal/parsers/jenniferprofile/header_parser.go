package jenniferprofile

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Jennifer headers are 2-column: each line has `KEY : VALUE` on the
// left half AND `KEY : VALUE` on the right half. Some keys carry an
// `(ID)` suffix (e.g. `DOMAIN (ID) : 상품 (11181)`) — we strip that
// here and lift the trailing parenthesised number into a sibling
// `_id` field.
//
// We can't split-on-N-spaces because some columns are alignment-padded
// after the colon (`SQL COUNT :  5    USER_ID :`), so we instead find
// the start position of every `KEY :` marker and slice between them.
//
// `keyMarkerRE` matches the leading `KEY` segment up to (but not
// including) the colon. KEY = uppercase letters/digits/underscore,
// possibly with internal spaces (`SQL COUNT`) and an optional
// `(ID)` token.
var keyMarkerRE = regexp.MustCompile(
	`(?m)([A-Z][A-Z0-9_]*(?:\s+[A-Z][A-Z0-9_]*)*(?:\s*\(\s*ID\s*\))?)\s*:`,
)

// parseHeader walks each header line, finds every `KEY :` marker
// in order, then slices the line into segments between markers.
// Each segment yields one (key, value) pair which we route onto
// the typed Header fields; unknown keys go into Header.Extra.
func parseHeader(headerText string, profile *models.JenniferTransactionProfile) {
	if profile.Header.Extra == nil {
		profile.Header.Extra = map[string]string{}
	}
	for _, line := range strings.Split(headerText, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// `APPLICATION : /xxx` is single-column — the slash prefix
		// would otherwise be hard to attribute to the right column.
		// Jennifer appends a hash suffix like `(-1059484346)` to the
		// APPLICATION URL; we strip it here so URL containment matching
		// against EXTERNAL_CALL urls works and the renderer table
		// shows the clean path the user actually deployed.
		if appValue, ok := matchSingleKey(line, "APPLICATION"); ok {
			profile.Header.Application = stripApplicationHash(appValue)
			continue
		}
		matches := keyMarkerRE.FindAllStringSubmatchIndex(line, -1)
		for i, m := range matches {
			// Each match captures: m[0..1]=full marker (incl colon),
			// m[2..3]=key portion before colon.
			keyEnd := m[3]
			afterColon := m[1]
			var valueEnd int
			if i+1 < len(matches) {
				valueEnd = matches[i+1][0]
			} else {
				valueEnd = len(line)
			}
			rawKey := strings.TrimSpace(line[m[2]:keyEnd])
			value := strings.TrimSpace(line[afterColon:valueEnd])
			normKey, idSuffix := normalizeHeaderKey(rawKey)
			assignHeaderField(profile, normKey, idSuffix, value)
		}
	}
}

// matchSingleKey checks for `KEY : value` where the rest of the line
// is the value (no second column). Used for APPLICATION which is
// always full-line.
func matchSingleKey(line, key string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	prefix := key + " :"
	if strings.HasPrefix(trimmed, prefix) {
		return strings.TrimSpace(trimmed[len(prefix):]), true
	}
	prefixNoSpace := key + ":"
	if strings.HasPrefix(trimmed, prefixNoSpace) {
		return strings.TrimSpace(trimmed[len(prefixNoSpace):]), true
	}
	return "", false
}

// normalizeHeaderKey collapses whitespace, uppercases, and strips
// the optional `(ID)` suffix. Returns (normalisedKey, hasIDSuffix).
//
// `DOMAIN (ID)` → ("DOMAIN", true)
// `SQL COUNT`   → ("SQL_COUNT", false)
// `SQL_TIME`    → ("SQL_TIME", false)
func normalizeHeaderKey(raw string) (string, bool) {
	hasID := false
	upper := strings.ToUpper(raw)
	if idx := strings.Index(upper, "(ID)"); idx >= 0 {
		hasID = true
		upper = strings.TrimSpace(upper[:idx])
	}
	// Collapse internal whitespace and turn spaces into underscores.
	parts := strings.Fields(upper)
	return strings.Join(parts, "_"), hasID
}

// extractIDFromValue lifts the trailing parenthesised number out of a
// value like `상품 (11181)` → ("상품", "11181"). The label is whatever
// comes before the trailing `(NNN)`.
var trailingIDRE = regexp.MustCompile(`^(.*)\(\s*([^)]+?)\s*\)\s*$`)

func extractIDFromValue(value string) (string, string) {
	value = strings.TrimSpace(value)
	m := trailingIDRE.FindStringSubmatch(value)
	if m == nil {
		return value, ""
	}
	return strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
}

// assignHeaderField routes one parsed key:value onto the typed
// Header struct. Fields we don't recognise go into Extra.
func assignHeaderField(profile *models.JenniferTransactionProfile, key string, hasIDSuffix bool, value string) {
	h := &profile.Header
	switch key {
	case "TXID":
		h.TXID = value
	case "GUID":
		h.GUID = value
	case "DOMAIN":
		if hasIDSuffix {
			label, id := extractIDFromValue(value)
			h.Domain = label
			h.DomainID = id
		} else {
			h.Domain = value
		}
	case "INSTANCE":
		if hasIDSuffix {
			label, id := extractIDFromValue(value)
			h.Instance = label
			h.InstanceID = id
		} else {
			h.Instance = value
		}
	case "BUSINESS":
		if hasIDSuffix {
			label, id := extractIDFromValue(value)
			h.Business = label
			h.BusinessID = id
		} else {
			h.Business = value
		}
	case "START_TIME":
		h.StartTime = value
	case "COLLECTION_TIME":
		h.CollectionTime = value
	case "END_TIME":
		h.EndTime = value
	case "RESPONSE_TIME":
		h.ResponseTimeMs = parseIntPtr(value)
	case "SQL_TIME":
		h.SQLTimeMs = parseIntPtr(value)
	case "SQL_COUNT":
		h.SQLCount = parseIntPtr(value)
	case "EXTERNAL_CALL_TIME":
		h.ExternalCallMs = parseIntPtr(value)
	case "FETCH_TIME":
		h.FetchTimeMs = parseIntPtr(value)
	case "CPU_TIME":
		h.CPUTimeMs = parseIntPtr(value)
	case "CLIENT_IP":
		h.ClientIP = value
	case "CLIENT_ID":
		h.ClientID = value
	case "USER_ID":
		h.UserID = value
	case "USER_AGENT":
		h.UserAgent = value
	case "HTTP_STATUS_CODE":
		h.HTTPStatusCode = parseIntPtr(value)
	case "FRONT_APP_ID":
		h.FrontAppID = value
	case "FRONT_PAGE_ID":
		h.FrontPageID = value
	case "ERROR":
		h.Error = value
	case "APPLICATION":
		h.Application = stripApplicationHash(value)
	default:
		if value != "" {
			h.Extra[key] = value
		}
	}
}

// applicationHashRE strips the trailing `(-NNN)` or `(NNN)` hash that
// Jennifer appends to APPLICATION URLs. The number can be negative
// (sign-extended hashCode) and arbitrarily wide. We only strip when
// the parens sit at the very end after a path-like value so we don't
// accidentally trim a parenthesised path segment.
var applicationHashRE = regexp.MustCompile(`\s*\(-?\d+\)\s*$`)

// stripApplicationHash removes the trailing hash suffix Jennifer
// appends to APPLICATION URLs (e.g. `/api/users(-1059484346)` →
// `/api/users`). Idempotent and safe to call on already-clean values.
func stripApplicationHash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	return strings.TrimSpace(applicationHashRE.ReplaceAllString(value, ""))
}

// parseIntPtr returns nil for empty / non-numeric values so they
// JSON-marshal as null.
func parseIntPtr(value string) *int {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	v, err := strconv.Atoi(strings.ReplaceAll(value, ",", ""))
	if err != nil {
		return nil
	}
	return &v
}
