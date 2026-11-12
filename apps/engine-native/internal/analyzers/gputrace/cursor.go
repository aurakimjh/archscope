package gputrace

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
)

type cursor struct {
	Identity string `json:"identity"`
	Offset   int    `json:"offset"`
}

func decodeCursor(raw, id string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	if len(raw) > 1024 {
		return 0, fmt.Errorf("GPU_TRACE_CURSOR_BUDGET")
	}
	b, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil {
		return 0, e
	}
	var c cursor
	if g.Strict(b, &c, 1024) != nil || c.Identity != id || c.Offset < 0 {
		return 0, fmt.Errorf("GPU_TRACE_CURSOR_STALE")
	}
	return c.Offset, nil
}
func encodeCursor(id string, offset int) string {
	b, _ := json.Marshal(cursor{id, offset})
	return base64.RawURLEncoding.EncodeToString(b)
}
