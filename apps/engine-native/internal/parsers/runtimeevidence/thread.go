package runtimeevidence

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	td "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajstack"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/pythondump"
	"os"
	"path/filepath"
	"strconv"
)

func (b *builder) threadDump(raw []byte) error {
	var plugin td.Plugin
	switch b.options.ThreadFormat {
	case "java_jstack":
		plugin = javajstack.New()
	case "python_pyspy":
		plugin = pythondump.PySpyPlugin{}
	case "python_faulthandler":
		plugin = pythondump.FaulthandlerPlugin{}
	default:
		return fmt.Errorf("RUNTIME_THREAD_FORMAT_REQUIRED: java_jstack, python_pyspy or python_faulthandler")
	}
	dir, e := os.MkdirTemp("", "runtime-dump-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "dump.txt")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		return e
	}
	bundles := []models.ThreadDumpBundle{}
	if multi, ok := plugin.(td.MultiBundlePlugin); ok {
		bundles, e = multi.ParseAll(path)
		if e != nil {
			return e
		}
	}
	if len(bundles) == 0 {
		bundle, e := plugin.Parse(path)
		if e != nil {
			return e
		}
		bundles = append(bundles, bundle)
	}
	if len(bundles) > 128 {
		return fmt.Errorf("RUNTIME_DUMP_COUNT_BUDGET")
	}
	originalTime := b.options.Time
	for i, bundle := range bundles {
		if e := b.ctx.Err(); e != nil {
			return e
		}
		b.prefix = fmt.Sprintf("/dumps/%d", i)
		b.options.Time = originalTime
		if bundle.CapturedAt != nil && b.options.Clock == "unix" {
			value := strconv.FormatInt(bundle.CapturedAt.UnixNano(), 10)
			b.options.Time = &value
		}
		body, e := json.Marshal(bundle)
		if e != nil {
			return e
		}
		if e = b.threads(body); e != nil {
			return e
		}
	}
	b.options.Time = originalTime
	b.prefix = ""
	return nil
}
