package multithread

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"strconv"
	"strings"
)

// Qualified identity is opt-in via captured process/thread-lifetime metadata.
// Missing context isolates observations; a shared display name never joins them.
func identityKey(b models.ThreadDumpBundle, s models.ThreadSnapshot, index int) string {
	unqualified := fmt.Sprintf("unqualified:%s:%d:%d:%s", b.SourceFile, b.DumpIndex, index, s.SnapshotID)
	if b.Language == "" || b.SourceFormat == "" {
		return unqualified
	}
	if s.ThreadID == nil || *s.ThreadID == "" {
		return unqualified
	}
	raw, _ := json.Marshal(b.Metadata["correlation_context"])
	var context map[string]string
	if json.Unmarshal(raw, &context) != nil {
		return unqualified
	}
	if _, e := strconv.ParseUint(context["pid"], 10, 64); e != nil {
		return unqualified
	}
	if _, e := strconv.ParseUint(context["process_start_ns"], 10, 64); e != nil {
		return unqualified
	}
	parts := []string{b.Language, b.SourceFormat}
	for _, key := range []string{"host_id", "boot_id", "pid_namespace", "pid", "process_start_ns"} {
		v := context[key]
		if v == "" || v == "unknown" || strings.ContainsAny(v, "\x00\r\n") {
			return unqualified
		}
		parts = append(parts, v)
	}
	start, ok := s.Metadata["thread_start_ns"].(string)
	if !ok {
		return unqualified
	}
	if _, e := strconv.ParseUint(start, 10, 64); e != nil {
		return unqualified
	}
	parts = append(parts, *s.ThreadID, start)
	if view, ok := s.Metadata["stack_owner"].(string); ok {
		parts = append(parts, view)
	}
	body, _ := json.Marshal(parts)
	return "qualified:" + string(body)
}
