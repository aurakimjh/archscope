// Package profilelegacy is the explicit compatibility composition for old
// profiler endpoints. New producer adapters belong to ProfileSet importers.
package profilelegacy

import (
	"fmt"
	"strings"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
)

type adapter struct {
	analyze func(string, profiler.Options) (profiler.AnalysisResult, error)
	stacks  func(string) (map[string]int, error)
}

var aliases = map[string]string{"": "collapsed", "collapsed": "collapsed", "jennifer": "jennifer", "jennifer_csv": "jennifer", "jennifer-csv": "jennifer", "flamegraph_svg": "svg", "svg": "svg", "flamegraph_html": "html", "html": "html", "v8-cpuprofile": "v8", "cpuprofile": "v8", "chrome-trace-json": "trace", "chrome-trace": "trace"}
var adapters = map[string]adapter{
	"collapsed": {profiler.AnalyzeCollapsedFile, func(path string) (map[string]int, error) { s, _, e := profiler.ParseCollapsedFile(path); return s, e }},
	"svg": {profiler.AnalyzeFlamegraphSVGFile, func(path string) (map[string]int, error) {
		r, e := profiler.ParseSvgFlamegraphFile(path, nil)
		return r.Stacks, e
	}},
	"html": {profiler.AnalyzeFlamegraphHTMLFile, func(path string) (map[string]int, error) {
		r, e := profiler.ParseHtmlProfilerFile(path, nil)
		return r.Stacks, e
	}},
	"jennifer": {profiler.AnalyzeJenniferFile, func(path string) (map[string]int, error) {
		r, e := profiler.ParseJenniferFlamegraphCSV(path, nil)
		if e != nil {
			return nil, e
		}
		return fromTree(r.Root)
	}},
	"v8":    {nil, func(path string) (map[string]int, error) { return parsedStacks(path, "v8-cpuprofile") }},
	"trace": {nil, func(path string) (map[string]int, error) { return parsedStacks(path, "chrome-trace-json") }},
}

func resolve(format string) (adapter, error) {
	canonical, ok := aliases[strings.ToLower(strings.TrimSpace(format))]
	if !ok {
		return adapter{}, fmt.Errorf("unsupported legacy profiler format: %q", format)
	}
	return adapters[canonical], nil
}
func Analyze(path, format string, options profiler.Options) (profiler.AnalysisResult, error) {
	a, e := resolve(format)
	if e != nil {
		return profiler.AnalysisResult{}, e
	}
	if a.analyze == nil {
		return profiler.AnalysisResult{}, fmt.Errorf("format uses the profile_evidence endpoint")
	}
	return a.analyze(path, options)
}
func LoadStacks(path, format string) (map[string]int, error) {
	a, e := resolve(format)
	if e != nil {
		return nil, e
	}
	stacks, e := a.stacks(path)
	if e != nil {
		return nil, e
	}
	if e = profilemodel.CheckLegacyRange(stacks); e != nil {
		return nil, e
	}
	return stacks, nil
}
func fromTree(root profiler.FlameNode) (map[string]int, error) {
	out := map[string]int{}
	var walk func(profiler.FlameNode, []string) error
	walk = func(n profiler.FlameNode, path []string) error {
		children := 0
		for _, child := range n.Children {
			if children > int(^uint(0)>>1)-child.Samples {
				return fmt.Errorf("legacy child total overflow")
			}
			children += child.Samples
		}
		if n.ID != "root" && n.Name != "All" {
			path = append(path, n.Name)
		}
		self := n.Samples - children
		if self > 0 && len(path) > 0 {
			out[strings.Join(path, ";")] += self
		}
		for _, child := range n.Children {
			if e := walk(child, append([]string(nil), path...)); e != nil {
				return e
			}
		}
		return nil
	}
	return out, walk(root, nil)
}
func parsedStacks(path, format string) (map[string]int, error) {
	parsed, _, e := parser.ParseFile(path, format, parser.Options{})
	if e != nil {
		return nil, e
	}
	out := map[string]int{}
	limit := int64(int(^uint(0) >> 1))
	for _, sample := range parsed.Samples {
		if sample.Value <= 0 {
			continue
		}
		names := []string{}
		for _, frame := range sample.Stack {
			if strings.TrimSpace(frame.Name) != "" {
				names = append(names, frame.Name)
			}
		}
		if len(names) == 0 {
			continue
		}
		key := strings.Join(names, ";")
		if sample.Value > limit-int64(out[key]) {
			return nil, fmt.Errorf("legacy value int range exceeded")
		}
		out[key] += int(sample.Value)
	}
	return out, profilemodel.CheckLegacyRange(out)
}
