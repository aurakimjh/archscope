package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aurakimjh/archscope/apps/profiler-native/internal/profiler"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type ProfilerService struct{}

type AnalyzeRequest struct {
	Path               string  `json:"path"`
	Format             string  `json:"format"`
	IntervalMS         float64 `json:"intervalMs"`
	ElapsedSec         float64 `json:"elapsedSec"`
	TopN               int     `json:"topN"`
	ProfileKind        string  `json:"profileKind"`
	TimelineBaseMethod string  `json:"timelineBaseMethod"`
}

func (s *ProfilerService) Analyze(req AnalyzeRequest) (profiler.AnalysisResult, error) {
	options, err := s.optionsFromRequest(req)
	if err != nil {
		return profiler.AnalysisResult{}, err
	}
	return s.runAnalyze(req, options)
}

type DrilldownRequest struct {
	AnalyzeRequest
	Filters []profiler.DrilldownFilter `json:"filters"`
}

type DiffRequest struct {
	BaselinePath   string  `json:"baselinePath"`
	TargetPath     string  `json:"targetPath"`
	BaselineFormat string  `json:"baselineFormat"`
	TargetFormat   string  `json:"targetFormat"`
	IntervalMS     float64 `json:"intervalMs"`
	TopN           int     `json:"topN"`
	Normalize      bool    `json:"normalize"`
}

type ExportPprofRequest struct {
	AnalyzeRequest
	OutputPath string `json:"outputPath"`
}

type ExportPprofResponse struct {
	OutputPath string `json:"outputPath"`
	Bytes      int    `json:"bytes"`
}

func (s *ProfilerService) Diff(req DiffRequest) (profiler.AnalysisResult, error) {
	if strings.TrimSpace(req.BaselinePath) == "" || strings.TrimSpace(req.TargetPath) == "" {
		return profiler.AnalysisResult{}, fmt.Errorf("both baselinePath and targetPath are required")
	}
	baselineStacks, err := s.loadStacks(req.BaselinePath, req.BaselineFormat)
	if err != nil {
		return profiler.AnalysisResult{}, fmt.Errorf("baseline: %w", err)
	}
	targetStacks, err := s.loadStacks(req.TargetPath, req.TargetFormat)
	if err != nil {
		return profiler.AnalysisResult{}, fmt.Errorf("target: %w", err)
	}
	return profiler.AnalyzeProfilerDiff(baselineStacks, targetStacks, req.BaselinePath, req.TargetPath, profiler.DiffOptions{
		Normalize: req.Normalize,
	}), nil
}

func (s *ProfilerService) loadStacks(path, format string) (map[string]int, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jennifer", "jennifer_csv", "jennifer-csv":
		result, err := profiler.ParseJenniferFlamegraphCSV(path, nil)
		if err != nil {
			return nil, err
		}
		return stacksFromFlameNode(result.Root), nil
	case "flamegraph_svg", "svg":
		result, err := profiler.ParseSvgFlamegraphFile(path, nil)
		if err != nil {
			return nil, err
		}
		return result.Stacks, nil
	case "flamegraph_html", "html":
		result, err := profiler.ParseHtmlProfilerFile(path, nil)
		if err != nil {
			return nil, err
		}
		return result.Stacks, nil
	case "", "collapsed":
		stacks, _, err := profiler.ParseCollapsedFile(path)
		if err != nil {
			return nil, err
		}
		return stacks, nil
	default:
		return nil, fmt.Errorf("unsupported format: %q", format)
	}
}

func stacksFromFlameNode(root profiler.FlameNode) map[string]int {
	// Walk leaf paths with exclusive samples — the flamegraph leaves carry
	// the diff-meaningful counts.
	stacks := map[string]int{}
	var walk func(node profiler.FlameNode, path []string)
	walk = func(node profiler.FlameNode, path []string) {
		next := path
		if len(node.Path) > 0 || node.Name != "All" {
			next = append(append([]string{}, path...), node.Name)
		}
		if len(node.Children) == 0 {
			if node.Samples > 0 && len(next) > 0 {
				stacks[strings.Join(next, ";")] += node.Samples
			}
			return
		}
		// Inclusive node — record self-time as exclusive samples.
		var childTotal int
		for _, child := range node.Children {
			childTotal += child.Samples
		}
		exclusive := node.Samples - childTotal
		if exclusive > 0 && len(next) > 0 {
			stacks[strings.Join(next, ";")] += exclusive
		}
		for _, child := range node.Children {
			walk(child, next)
		}
	}
	walk(root, nil)
	return stacks
}

func (s *ProfilerService) Drilldown(req DrilldownRequest) ([]profiler.DrilldownStage, error) {
	options, err := s.optionsFromRequest(req.AnalyzeRequest)
	if err != nil {
		return nil, err
	}
	result, err := s.runAnalyze(req.AnalyzeRequest, options)
	if err != nil {
		return nil, err
	}
	root := result.Charts.Flamegraph
	stages := profiler.BuildDrilldownStages(root, req.Filters, options.IntervalMS, options.ElapsedSec, options.TopN)
	return stages, nil
}

func (s *ProfilerService) optionsFromRequest(req AnalyzeRequest) (profiler.Options, error) {
	if strings.TrimSpace(req.Path) == "" {
		return profiler.Options{}, fmt.Errorf("path is required")
	}
	options := profiler.Options{
		IntervalMS:         req.IntervalMS,
		TopN:               req.TopN,
		ProfileKind:        req.ProfileKind,
		TimelineBaseMethod: req.TimelineBaseMethod,
	}
	if req.ElapsedSec >= 0 {
		elapsed := req.ElapsedSec
		options.ElapsedSec = &elapsed
	}
	return options, nil
}

func (s *ProfilerService) runAnalyze(req AnalyzeRequest, options profiler.Options) (profiler.AnalysisResult, error) {
	switch strings.ToLower(strings.TrimSpace(req.Format)) {
	case "jennifer", "jennifer_csv", "jennifer-csv":
		return profiler.AnalyzeJenniferFile(req.Path, options)
	case "flamegraph_svg", "svg":
		return profiler.AnalyzeFlamegraphSVGFile(req.Path, options)
	case "flamegraph_html", "html":
		return profiler.AnalyzeFlamegraphHTMLFile(req.Path, options)
	case "", "collapsed":
		return profiler.AnalyzeCollapsedFile(req.Path, options)
	default:
		return profiler.AnalysisResult{}, fmt.Errorf("unsupported format: %q (expected 'collapsed' / 'jennifer' / 'flamegraph_svg' / 'flamegraph_html')", req.Format)
	}
}

func (s *ProfilerService) ExportPprof(req ExportPprofRequest) (ExportPprofResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return ExportPprofResponse{}, fmt.Errorf("path is required")
	}
	out := strings.TrimSpace(req.OutputPath)
	if out == "" {
		picked, err := s.PickPprofOutput()
		if err != nil {
			return ExportPprofResponse{}, err
		}
		if picked == "" {
			return ExportPprofResponse{}, fmt.Errorf("export cancelled")
		}
		out = picked
	}
	stacks, err := s.loadStacks(req.Path, req.Format)
	if err != nil {
		return ExportPprofResponse{}, err
	}
	intervalMs := req.IntervalMS
	if intervalMs <= 0 {
		intervalMs = 100
	}
	if err := profiler.ExportToPprof(stacks, out, "samples", "count", intervalMs); err != nil {
		return ExportPprofResponse{}, err
	}
	info, err := os.Stat(out)
	if err != nil {
		return ExportPprofResponse{}, err
	}
	return ExportPprofResponse{OutputPath: out, Bytes: int(info.Size())}, nil
}

func (s *ProfilerService) PickPprofOutput() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("application not initialized")
	}
	dialog := app.Dialog.SaveFile().
		SetMessage("Save pprof profile").
		SetFilename("profile.pb.gz").
		AddFilter("pprof profile (.pb.gz)", "*.pb.gz").
		AddFilter("All files", "*.*")
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}

func (s *ProfilerService) PickProfileFile() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("application not initialized")
	}
	dialog := app.Dialog.OpenFile().
		SetTitle("Select profiler input").
		AddFilter("Collapsed stacks", "*.collapsed;*.txt").
		AddFilter("Jennifer flamegraph CSV", "*.csv").
		AddFilter("FlameGraph SVG", "*.svg").
		AddFilter("FlameGraph HTML", "*.html;*.htm").
		AddFilter("All files", "*.*")
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}
