package systemtrace

import (
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"strconv"
)

type SourceInventory struct {
	SourceSHA256 string       `json:"source_sha256"`
	Format       string       `json:"format"`
	Tool         string       `json:"tool"`
	ToolVersion  string       `json:"tool_version"`
	Records      string       `json:"records"`
	Entities     []m.Entity   `json:"entities"`
	Projections  []Projection `json:"projections"`
	Warnings     []string     `json:"warnings"`
}

func Inventory(s m.Set) SourceInventory {
	return SourceInventory{s.SourceSHA256, s.Format, s.Capture.Tool, s.Capture.ToolVersion, strconv.Itoa(len(s.Records)), s.Capture.Entities, Projections(s), []string{"Producer/clock/lifetime and completeness are declared, not independently authenticated.", "Unknown/lost/ambiguous scheduler spans remain null. Off-CPU is not lock contention.", "Histogram output is aggregate-only; no request timeline or inferred quantiles.", "Projection requires an explicit metric; raw period is never automatically CPU time."}}
}
