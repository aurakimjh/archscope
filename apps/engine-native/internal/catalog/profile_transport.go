package catalog

import (
	"encoding/json"
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

// Operational transport bounds, not producer/diagnostic limits. Base64 chunks
// stay below the common 1 MiB serialized-record ceiling without raising policy.
const MaxProfileTransportBytes = 32 << 20
const profileChunkBytes = 256 << 10

type profileChunk struct {
	Offset int64  `json:"offset"`
	Data   []byte `json:"data"`
}

func decodeProfileChunk(raw json.RawMessage) (profileChunk, error) {
	var c profileChunk
	if e := wirejson.Decode(raw, &c); e != nil {
		return c, e
	}
	if c.Offset < 0 || len(c.Data) == 0 || len(c.Data) > profileChunkBytes {
		return c, fmt.Errorf("invalid ProfileSet transport chunk")
	}
	return c, nil
}

func appendProfileChunk(body []byte, c profileChunk) ([]byte, error) {
	if c.Offset != int64(len(body)) {
		return nil, ingestion.Problem("IMPORT_RESULT_INVALID", "ProfileSet chunks out of order")
	}
	if len(c.Data) > MaxProfileTransportBytes-len(body) {
		return nil, ingestion.BudgetProblem("profile_transport_bytes", "profile_transport_bytes", int64(len(body)+len(c.Data)), MaxProfileTransportBytes)
	}
	return append(body, c.Data...), nil
}
