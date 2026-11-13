package wirejson

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// Hash retains the established identifier encoding, including map key sorting.
// Domain validators must reject non-serializable values before creating IDs.
func Hash(value any) string { body, _ := json.Marshal(value); return Digest(body) }

func Uint64(text string) (uint64, error) {
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != text {
		return 0, fmt.Errorf("canonical uint64 required")
	}
	return value, nil
}

func Text(value string, limit int) bool {
	return len(value) <= limit && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n")
}

func IsDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
