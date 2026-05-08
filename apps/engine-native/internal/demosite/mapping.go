// Package demosite ports archscope_engine.demo_site_mapping +
// archscope_engine.demo_site_runner — the manifest-driven demo
// orchestrator that the parity gate and the docs site lean on. This
// file is the mapping half (analyzer_type → CLI command tuple); the
// runner half lives in runner.go.
//
// JSON-only: the Python loader accepts both YAML and JSON, but the
// canonical mapping file checked in to the repo is JSON. Keeping the
// Go port stdlib-only avoids pulling in a YAML dependency for a single
// fixture form. If a future user ships a YAML mapping, they can
// convert it with `yq -o=json` before invoking the Go runner.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] demosite/mapping.go — analyzer_type ↔ CLI command 매핑 표
// 로더.
//
// analyzer_type_mapping.json 의 역할
//   매니페스트가 사용하는 추상 라벨(analyzer_type)을 실제 CLI 명령으로
//   변환하는 사전. 예시:
//     "access_log"        → ["access-log", "analyze"]
//     "thread_dump_multi" → ["thread-dump", "analyze-multi"]
//     "jennifer_profile"  → ["jennifer-profile", "analyze"]
//
// 자동 발견 (auto-discovery)
//   매니페스트 디렉토리에서 parent 를 거슬러 올라가며
//   analyzer_type_mapping.json 을 찾음. 매번 path 명시할 필요 없도록
//   편의 제공. --mapping 으로 명시하면 자동 발견 skip.
//
// reference_doc 패턴
//   AnalyzerTypeMapping.Command 가 nil 인 entry 는 "이건 분석 대상이
//   아니라 참고 문서" 라는 표시. 매니페스트 안에서 사용자에게 추가 컨텍스트
//   를 제공하는 README 같은 파일을 가리킬 때 사용.
//
// JSON-only 제약
//   stdlib 만 유지하기 위해 YAML 미지원. demo fixture 의 canonical
//   포맷은 JSON. 사용자가 YAML 을 가지고 있다면 `yq -o=json` 으로 변환.
package demosite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AnalyzerTypeMapping mirrors the Python dataclass. `Command` is nil
// when the entry is a documentation pointer (see "reference_doc"
// pattern in tests) — the runner treats those as reference files.
type AnalyzerTypeMapping struct {
	AnalyzerType    string
	Command         []string
	InputOption     string
	FormatOverrides map[string]AnalyzerTypeMapping
	Note            string
}

// rawMapping is the on-disk JSON shape; we decode through this so
// `command: null` round-trips as a nil slice (vs. an empty one).
type rawMapping struct {
	Command         *[]string             `json:"command"`
	InputOption     *string               `json:"input_option"`
	FormatOverrides map[string]rawMapping `json:"format_overrides"`
	Note            *string               `json:"note"`
}

type rawFile struct {
	Mappings map[string]rawMapping `json:"mappings"`
}

// LoadAnalyzerTypeMappings finds the mapping file near `anchor` (a
// manifest path or directory) and decodes it into a name-keyed map.
// Mirrors `load_analyzer_type_mappings(anchor)`.
func LoadAnalyzerTypeMappings(anchor string) (map[string]AnalyzerTypeMapping, error) {
	mappingPath, err := FindAnalyzerTypeMapping(anchor)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(mappingPath)
	if err != nil {
		return nil, fmt.Errorf("read mapping %s: %w", mappingPath, err)
	}
	// Reject malformed payloads (`{"wrong_key": {}}`) before we look
	// at the inner shape — the Python loader does the same.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("decode mapping %s: %w", mappingPath, err)
	}
	if _, ok := probe["mappings"]; !ok {
		return nil, fmt.Errorf("Invalid analyzer type mapping file: %s", mappingPath)
	}
	var file rawFile
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("decode mapping %s: %w", mappingPath, err)
	}
	if file.Mappings == nil {
		return nil, fmt.Errorf("Invalid analyzer type mapping file: %s", mappingPath)
	}
	out := make(map[string]AnalyzerTypeMapping, len(file.Mappings))
	for analyzerType, raw := range file.Mappings {
		out[analyzerType] = mappingFromRaw(analyzerType, raw)
	}
	return out, nil
}

// FindAnalyzerTypeMapping walks up from `anchor` looking for an
// `analyzer_type_mapping.json`. Mirrors the Python search order: the
// file's own dir, then each parent up to the filesystem root.
func FindAnalyzerTypeMapping(anchor string) (string, error) {
	abs, err := filepath.Abs(anchor)
	if err != nil {
		return "", fmt.Errorf("abs %s: %w", anchor, err)
	}
	info, err := os.Stat(abs)
	var start string
	if err == nil && info.IsDir() {
		start = abs
	} else {
		// `abs` may not exist (Python tests assert FileNotFoundError
		// on missing dirs); we still walk parents.
		start = filepath.Dir(abs)
	}
	candidates := []string{filepath.Join(start, "analyzer_type_mapping.json")}
	cur := start
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		candidates = append(candidates, filepath.Join(parent, "analyzer_type_mapping.json"))
		cur = parent
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("demo-site analyzer_type_mapping.json was not found near the manifest root")
}

// CommandForMapping returns the command tuple for `m`, honouring a
// `format` override when present. Returns nil for reference-only
// entries (Command is nil). Mirrors `command_for_mapping`.
func CommandForMapping(m AnalyzerTypeMapping, format string) []string {
	if format != "" {
		if override, ok := m.FormatOverrides[format]; ok {
			return override.Command
		}
	}
	return m.Command
}

// InputOptionForMapping is the input-option twin of CommandForMapping.
func InputOptionForMapping(m AnalyzerTypeMapping, format string) string {
	if format != "" {
		if override, ok := m.FormatOverrides[format]; ok {
			return override.InputOption
		}
	}
	return m.InputOption
}

func mappingFromRaw(analyzerType string, raw rawMapping) AnalyzerTypeMapping {
	out := AnalyzerTypeMapping{AnalyzerType: analyzerType}
	if raw.Command != nil {
		out.Command = append([]string(nil), (*raw.Command)...)
	}
	if raw.InputOption != nil {
		out.InputOption = *raw.InputOption
	}
	if raw.Note != nil {
		out.Note = *raw.Note
	}
	if len(raw.FormatOverrides) > 0 {
		out.FormatOverrides = make(map[string]AnalyzerTypeMapping, len(raw.FormatOverrides))
		for name, child := range raw.FormatOverrides {
			out.FormatOverrides[name] = mappingFromRaw(analyzerType, child)
		}
	} else {
		out.FormatOverrides = map[string]AnalyzerTypeMapping{}
	}
	return out
}
