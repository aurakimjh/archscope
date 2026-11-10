package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/procmap"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

var desktopBuildCommit = "unknown"

type DesktopLaunchState struct {
	ShowHTTP bool   `json:"showHttp"`
	OpenPath string `json:"openPath,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (s *CaptureService) GetDesktopLaunchState() DesktopLaunchState {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	return s.launchState
}
func (s *CaptureService) ApplyDesktopLaunch() DesktopLaunchState {
	s.launchOnce.Do(func() {
		opts := s.launchOptions
		s.controlMu.Lock()
		openPath := opts.OpenPath
		if openPath == "" {
			openPath = s.launchState.OpenPath
		}
		s.launchState = DesktopLaunchState{ShowHTTP: opts.Capture || opts.OpenPath != "" || opts.Descriptor != "", OpenPath: openPath}
		s.controlMu.Unlock()
		if !opts.Capture {
			return
		}
		cfg := capture.Config{ProcessScope: opts.Scope}
		if opts.Process != "" {
			cfg.ProcessScope = "launch"
			cfg.LaunchPath = opts.Process
		} else if len(opts.PIDs) > 0 {
			cfg.ProcessScope = "selected"
			for _, pid := range opts.PIDs {
				process := procmap.Lookup(pid)
				if process.Key.StartTime == "" {
					s.setLaunchError(fmt.Errorf("process identity is unavailable"))
					return
				}
				cfg.AllowedProcesses = append(cfg.AllowedProcesses, process.Key)
			}
		}
		started, err := s.StartCapture(cfg)
		if err != nil {
			s.setLaunchError(err)
			return
		}
		if opts.Duration > 0 {
			ctx, cancel := context.WithCancel(context.Background())
			s.controlMu.Lock()
			s.launchCancel = cancel
			s.controlMu.Unlock()
			go func() {
				timer := time.NewTimer(opts.Duration)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
				current := s.GetCurrentCaptureSession()
				if current.ID != started.ID || current.State != capture.StateRunning {
					return
				}
				if _, err := s.StopCapture(string(started.ID)); err != nil {
					s.setLaunchError(err)
					return
				}
				if opts.Destination != "" {
					_, err := s.ExportCaptureFile(CaptureExportRequest{Source: CaptureFileSource{SessionID: string(started.ID)}, Destination: opts.Destination, Format: opts.Format, Scope: "all"})
					if err != nil {
						s.setLaunchError(err)
					}
				}
			}()
		}
	})
	return s.GetDesktopLaunchState()
}
func (s *CaptureService) setLaunchError(err error) {
	s.controlMu.Lock()
	s.launchState.Error = err.Error()
	s.controlMu.Unlock()
	emitEvent("capture:automation-error", map[string]string{"message": err.Error()})
}
func (s *CaptureService) shutdownAutomation() {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.closing = true
	s.controlMu.Lock()
	if s.launchCancel != nil {
		s.launchCancel()
	}
	s.controlMu.Unlock()
	current := s.GetCurrentCaptureSession()
	if current.State == capture.StateRunning || current.State == capture.StateStarting {
		_, _ = s.stopCaptureLocked(string(current.ID))
	} else {
		_, _ = s.trust.Remove()
		s.manager.ReleaseAuthority()
	}
}
func decodeControl(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid command parameters: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing command data")
	}
	return nil
}
func (s *CaptureService) executeAutomation(ctx context.Context, call automation.Call) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch call.Command {
	case "capabilities":
		return map[string]any{"buildCommit": desktopBuildCommit, "contract": s.GetLiveCaptureContract(), "modes": s.ListCaptureModes(), "backends": s.ListCaptureBackends()}, nil
	case "status":
		return s.GetCurrentCaptureSession(), nil
	case "processes":
		return s.ListCaptureProcesses()
	case "events":
		var request struct {
			After uint64 `json:"after"`
			Limit int    `json:"limit"`
		}
		if err := decodeControl(call.Params, &request); err != nil {
			return nil, err
		}
		return s.journal.Read(request.After, request.Limit), nil
	case "attach":
		var request struct {
			Processes []models.ProcessKey `json:"processes"`
		}
		if err := decodeControl(call.Params, &request); err != nil {
			return nil, err
		}
		if len(request.Processes) == 0 || len(request.Processes) > 1024 {
			return nil, fmt.Errorf("choose 1–1024 process instances")
		}
		for _, key := range request.Processes {
			if key.PID <= 0 || key.StartTime == "" || procmap.Lookup(key.PID).Key != key {
				return nil, fmt.Errorf("process identity changed or is unavailable")
			}
		}
		current := s.GetCurrentCaptureSession()
		if current.State == capture.StateRunning {
			return nil, fmt.Errorf("stop capture before changing attached processes")
		}
		s.controlMu.Lock()
		s.controlTargets = append([]models.ProcessKey(nil), request.Processes...)
		s.controlMu.Unlock()
		return request.Processes, nil
	case "detach":
		current := s.GetCurrentCaptureSession()
		if current.State == capture.StateRunning {
			return nil, fmt.Errorf("stop capture before detaching")
		}
		s.controlMu.Lock()
		s.controlTargets = nil
		s.controlMu.Unlock()
		return true, nil
	case "start":
		var config capture.Config
		if err := decodeControl(call.Params, &config); err != nil {
			return nil, err
		}
		if config.ProcessScope == "" {
			s.controlMu.Lock()
			config.AllowedProcesses = append([]models.ProcessKey(nil), s.controlTargets...)
			s.controlMu.Unlock()
			if len(config.AllowedProcesses) == 0 {
				return nil, fmt.Errorf("attach process instances or specify an explicit scope")
			}
			config.ProcessScope = "selected"
		}
		emitEvent("capture:show", nil)
		return s.StartCapture(config)
	case "stats", "pause", "resume", "clear", "stop", "detail", "summary":
		var request struct {
			SessionID string             `json:"sessionId"`
			ID        string             `json:"id,omitempty"`
			Process   *models.ProcessKey `json:"process,omitempty"`
		}
		if err := decodeControl(call.Params, &request); err != nil {
			return nil, err
		}
		if request.SessionID == "" {
			request.SessionID = string(s.GetCurrentCaptureSession().ID)
		}
		switch call.Command {
		case "stats":
			return s.GetCaptureStats(request.SessionID)
		case "pause", "resume":
			return s.SetCapturePaused(request.SessionID, call.Command == "pause")
		case "clear":
			return s.ClearCapture(request.SessionID)
		case "stop":
			return s.StopCapture(request.SessionID)
		case "detail":
			return s.GetCaptureTransaction(request.SessionID, request.ID)
		default:
			return s.GetCaptureSummary(request.SessionID, request.Process)
		}
	case "list":
		var request CaptureFetchRequest
		if err := decodeControl(call.Params, &request); err != nil {
			return nil, err
		}
		if request.SessionID == "" {
			request.SessionID = string(s.GetCurrentCaptureSession().ID)
		}
		request.Filter.MetadataOnly = true
		if request.Limit <= 0 || request.Limit > 100 {
			request.Limit = 25
		}
		return s.FetchCaptureTransactions(request)
	case "save":
		var request CaptureExportRequest
		if err := decodeControl(call.Params, &request); err != nil {
			return nil, err
		}
		if !filepath.IsAbs(request.Destination) {
			return nil, fmt.Errorf("destination must be absolute")
		}
		return s.ExportCaptureFile(request)
	case "open":
		var request struct {
			Path string `json:"path"`
		}
		if err := decodeControl(call.Params, &request); err != nil {
			return nil, err
		}
		if !filepath.IsAbs(request.Path) {
			return nil, fmt.Errorf("session path must be absolute")
		}
		opened, err := s.OpenCaptureFile(request.Path)
		if err != nil {
			return nil, err
		}
		s.controlMu.Lock()
		s.launchState.ShowHTTP = true
		s.launchState.OpenPath = request.Path
		s.controlMu.Unlock()
		emitEvent("capture:open-file", map[string]string{"path": request.Path})
		return map[string]any{"transactions": opened.Result.Summary["total_transactions"], "selectedId": opened.SelectedID}, nil
	default:
		return nil, fmt.Errorf("unknown automation command")
	}
}
