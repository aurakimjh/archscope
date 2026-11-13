// Package querypage binds bounded page offsets to immutable result and query
// digests. It does not authenticate data or bypass owner/hash checks.
package querypage

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

type cursor struct {
	Result, Query string
	Offset        int
}

var ErrStale = errors.New("cursor does not match result/query/page")

// Decode retains the established cursor wire bytes, including JSON field names.
func Decode(token, result, query string) (int, error) {
	if token == "" {
		return 0, nil
	}
	if len(token) > 2048 {
		return 0, ErrStale
	}
	var c cursor
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || json.Unmarshal(raw, &c) != nil || c.Result != result || c.Query != query || c.Offset < 0 {
		return 0, ErrStale
	}
	return c.Offset, nil
}
func Encode(result, query string, offset int) string {
	raw, _ := json.Marshal(cursor{result, query, offset})
	return base64.RawURLEncoding.EncodeToString(raw)
}

// Bounds avoids overflow when a hostile cursor contains the maximum integer.
func Bounds(offset, limit, total int) (int, error) {
	if offset < 0 || limit < 1 || total < 0 || offset > total {
		return 0, ErrStale
	}
	if limit > total-offset {
		return total, nil
	}
	return offset + limit, nil
}
