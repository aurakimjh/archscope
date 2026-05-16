package apicontract

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

type Operation struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OperationID string   `json:"operation_id,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Source      string   `json:"source,omitempty"`
}

type Channel struct {
	Name         string   `json:"name"`
	Direction    string   `json:"direction,omitempty"`
	OperationID  string   `json:"operation_id,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	MessageNames []string `json:"message_names,omitempty"`
	Source       string   `json:"source,omitempty"`
}

type OpenAPIContract struct {
	Title      string      `json:"title,omitempty"`
	Version    string      `json:"version,omitempty"`
	Operations []Operation `json:"operations"`
}

type AsyncAPIContract struct {
	Title    string    `json:"title,omitempty"`
	Version  string    `json:"version,omitempty"`
	Channels []Channel `json:"channels"`
}

type Options struct{}

var httpMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true, "patch": true, "options": true, "head": true, "trace": true,
}

func ParseOpenAPIFile(path string, _ Options) (OpenAPIContract, *diagnostics.ParserDiagnostics, error) {
	diags := diagnostics.New("openapi")
	diags.SetSourceFile(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return OpenAPIContract{}, diags, err
	}
	diags.TotalLines = lineCount(data)
	if json.Valid(data) {
		contract, err := parseOpenAPIJSON(data)
		if err != nil {
			diags.AddError(0, "INVALID_OPENAPI_JSON", err.Error(), preview(data))
			return OpenAPIContract{}, diags, err
		}
		diags.ParsedRecords = len(contract.Operations)
		return contract, diags, nil
	}
	contract := parseOpenAPIYAMLish(string(data), diags)
	diags.ParsedRecords = len(contract.Operations)
	return contract, diags, nil
}

func ParseAsyncAPIFile(path string, _ Options) (AsyncAPIContract, *diagnostics.ParserDiagnostics, error) {
	diags := diagnostics.New("asyncapi")
	diags.SetSourceFile(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return AsyncAPIContract{}, diags, err
	}
	diags.TotalLines = lineCount(data)
	if json.Valid(data) {
		contract, err := parseAsyncAPIJSON(data)
		if err != nil {
			diags.AddError(0, "INVALID_ASYNCAPI_JSON", err.Error(), preview(data))
			return AsyncAPIContract{}, diags, err
		}
		diags.ParsedRecords = len(contract.Channels)
		return contract, diags, nil
	}
	contract := parseAsyncAPIYAMLish(string(data), diags)
	diags.ParsedRecords = len(contract.Channels)
	return contract, diags, nil
}

func parseOpenAPIJSON(data []byte) (OpenAPIContract, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return OpenAPIContract{}, err
	}
	info, _ := root["info"].(map[string]any)
	paths, _ := root["paths"].(map[string]any)
	var ops []Operation
	for path, rawPathItem := range paths {
		pathItem, _ := rawPathItem.(map[string]any)
		for method, rawOperation := range pathItem {
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			opObj, _ := rawOperation.(map[string]any)
			ops = append(ops, Operation{
				Method:      strings.ToUpper(method),
				Path:        path,
				OperationID: str(opObj["operationId"]),
				Summary:     str(opObj["summary"]),
				Tags:        stringArray(opObj["tags"]),
				Source:      "openapi",
			})
		}
	}
	sortOperations(ops)
	return OpenAPIContract{Title: str(info["title"]), Version: str(info["version"]), Operations: ops}, nil
}

func parseAsyncAPIJSON(data []byte) (AsyncAPIContract, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return AsyncAPIContract{}, err
	}
	info, _ := root["info"].(map[string]any)
	channelsObj, _ := root["channels"].(map[string]any)
	var channels []Channel
	for name, rawChannel := range channelsObj {
		channelObj, _ := rawChannel.(map[string]any)
		added := false
		for _, direction := range []string{"publish", "subscribe"} {
			if op, ok := channelObj[direction].(map[string]any); ok {
				channels = append(channels, Channel{Name: name, Direction: direction, OperationID: str(op["operationId"]), Summary: str(op["summary"]), MessageNames: messageNames(op["message"]), Source: "asyncapi"})
				added = true
			}
		}
		if !added {
			channels = append(channels, Channel{Name: name, Source: "asyncapi"})
		}
	}
	sortChannels(channels)
	return AsyncAPIContract{Title: str(info["title"]), Version: str(info["version"]), Channels: channels}, nil
}

func parseOpenAPIYAMLish(text string, diags *diagnostics.ParserDiagnostics) OpenAPIContract {
	var ops []Operation
	var title, version, currentPath, currentMethod string
	current := Operation{}
	flush := func() {
		if current.Path != "" && current.Method != "" {
			current.Source = "openapi-yaml"
			ops = append(ops, current)
		}
		current = Operation{}
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	inPaths := false
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "title:") {
			title = cleanYAMLValue(strings.TrimPrefix(trimmed, "title:"))
		}
		if strings.HasPrefix(trimmed, "version:") {
			version = cleanYAMLValue(strings.TrimPrefix(trimmed, "version:"))
		}
		if trimmed == "paths:" {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		key := strings.TrimSuffix(trimmed, ":")
		if indent == 2 && strings.HasPrefix(key, "/") {
			flush()
			currentPath = key
			currentMethod = ""
			continue
		}
		if indent == 4 && httpMethods[strings.ToLower(key)] {
			flush()
			currentMethod = strings.ToUpper(key)
			current = Operation{Path: currentPath, Method: currentMethod}
			continue
		}
		if current.Path == "" || current.Method == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "operationId:"):
			current.OperationID = cleanYAMLValue(strings.TrimPrefix(trimmed, "operationId:"))
		case strings.HasPrefix(trimmed, "summary:"):
			current.Summary = cleanYAMLValue(strings.TrimPrefix(trimmed, "summary:"))
		case strings.HasPrefix(trimmed, "tags:"):
			current.Tags = splitYAMLList(strings.TrimPrefix(trimmed, "tags:"))
		case strings.HasPrefix(trimmed, "- ") && len(current.Tags) == 0:
			current.Tags = append(current.Tags, cleanYAMLValue(strings.TrimPrefix(trimmed, "- ")))
		default:
			if strings.HasSuffix(trimmed, ":") && indent <= 2 && !strings.HasPrefix(key, "/") {
				inPaths = false
			}
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		diags.AddWarning(lineNo, "OPENAPI_YAML_SCAN_ERROR", err.Error(), "", false)
	}
	sortOperations(ops)
	return OpenAPIContract{Title: title, Version: version, Operations: ops}
}

func parseAsyncAPIYAMLish(text string, diags *diagnostics.ParserDiagnostics) AsyncAPIContract {
	var channels []Channel
	var title, version, currentName string
	current := Channel{}
	flush := func() {
		if current.Name != "" {
			current.Source = "asyncapi-yaml"
			channels = append(channels, current)
		}
		current = Channel{}
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	inChannels := false
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "title:") {
			title = cleanYAMLValue(strings.TrimPrefix(trimmed, "title:"))
		}
		if strings.HasPrefix(trimmed, "version:") {
			version = cleanYAMLValue(strings.TrimPrefix(trimmed, "version:"))
		}
		if trimmed == "channels:" {
			inChannels = true
			continue
		}
		if !inChannels {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		key := strings.TrimSuffix(trimmed, ":")
		if indent == 2 {
			flush()
			currentName = key
			current = Channel{Name: currentName}
			continue
		}
		if current.Name == "" {
			continue
		}
		if indent == 4 && (key == "publish" || key == "subscribe") {
			current.Direction = key
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "operationId:"):
			current.OperationID = cleanYAMLValue(strings.TrimPrefix(trimmed, "operationId:"))
		case strings.HasPrefix(trimmed, "summary:"):
			current.Summary = cleanYAMLValue(strings.TrimPrefix(trimmed, "summary:"))
		case strings.HasPrefix(trimmed, "name:"):
			value := cleanYAMLValue(strings.TrimPrefix(trimmed, "name:"))
			if value != "" {
				current.MessageNames = append(current.MessageNames, value)
			}
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		diags.AddWarning(lineNo, "ASYNCAPI_YAML_SCAN_ERROR", err.Error(), "", false)
	}
	sortChannels(channels)
	return AsyncAPIContract{Title: title, Version: version, Channels: channels}
}

func messageNames(value any) []string {
	switch v := value.(type) {
	case map[string]any:
		if ref := str(v["$ref"]); ref != "" {
			return []string{refName(ref)}
		}
		if name := str(v["name"]); name != "" {
			return []string{name}
		}
		if oneOf := array(v["oneOf"]); len(oneOf) > 0 {
			var names []string
			for _, item := range oneOf {
				names = append(names, messageNames(item)...)
			}
			return names
		}
	case []any:
		var names []string
		for _, item := range v {
			names = append(names, messageNames(item)...)
		}
		return names
	}
	return nil
}

func refName(ref string) string {
	ref = strings.TrimSpace(ref)
	if idx := strings.LastIndex(ref, "/"); idx >= 0 && idx+1 < len(ref) {
		return ref[idx+1:]
	}
	return ref
}

func sortOperations(ops []Operation) {
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})
}

func sortChannels(channels []Channel) {
	sort.SliceStable(channels, func(i, j int) bool {
		if channels[i].Name != channels[j].Name {
			return channels[i].Name < channels[j].Name
		}
		return channels[i].Direction < channels[j].Direction
	})
}

func cleanYAMLValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return value
}

func splitYAMLList(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if cleaned := cleanYAMLValue(part); cleaned != "" {
			out = append(out, cleaned)
		}
	}
	return out
}

func array(v any) []any {
	if out, ok := v.([]any); ok {
		return out
	}
	return nil
}

func stringArray(v any) []string {
	values := array(v)
	out := make([]string, 0, len(values))
	for _, value := range values {
		if s := str(value); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func lineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	return strings.Count(string(data), "\n") + 1
}

func preview(data []byte) string {
	if len(data) > diagnostics.RawPreviewLimit {
		data = data[:diagnostics.RawPreviewLimit]
	}
	return string(data)
}

func ValidateOpenAPI(contract OpenAPIContract) error {
	if len(contract.Operations) == 0 {
		return fmt.Errorf("openapi contract has no operations")
	}
	return nil
}

func ValidateAsyncAPI(contract AsyncAPIContract) error {
	if len(contract.Channels) == 0 {
		return fmt.Errorf("asyncapi contract has no channels")
	}
	return nil
}
