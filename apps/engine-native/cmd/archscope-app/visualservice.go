package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/distribution"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/visualclipboard"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/visuals"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type visualRenderWindow interface {
	ID() uint
	Name() string
	Close()
}
type visualJob struct {
	window   visualRenderWindow
	svg      [][]byte
	spec     visuals.Spec
	done     chan error
	png      [][]byte
	finished bool
	cancel   context.CancelFunc
}
type VisualRenderRequest struct {
	Handle       string               `json:"handle"`
	Target       string               `json:"target"`
	Query        visuals.Query        `json:"query"`
	Presentation visuals.Presentation `json:"presentation"`
	Parent       string               `json:"parent,omitempty"`
	Upgrade      bool                 `json:"upgrade"`
}
type VisualRenderInput struct {
	Version    string   `json:"version"`
	SVG        []string `json:"svg"`
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	FontURL    string   `json:"font_url"`
	FontSHA256 string   `json:"font_sha256"`
}

func (s *EngineService) visualStore() (*visuals.Store, error) {
	s.visualMu.Lock()
	defer s.visualMu.Unlock()
	if s.visualClosed {
		return nil, fmt.Errorf("VISUAL_SERVICE_CLOSED")
	}
	if s.visualArtifacts != nil {
		return s.visualArtifacts, nil
	}
	root, e := cases.DefaultRoot()
	if e != nil {
		return nil, e
	}
	store, e := visuals.OpenStore(filepath.Join(root, "visual-artifacts"))
	if e != nil {
		return nil, e
	}
	s.visualArtifacts = store
	return store, nil
}
func (s *EngineService) VisualCapabilities() string {
	s.visualMu.Lock()
	render := s.launchVisual != nil && !s.visualClosed
	s.visualMu.Unlock()
	return mustOfflineJSON(map[string]any{"version": visuals.Version, "renderer": render, "native_clipboard": render && visualclipboard.Supported(), "font_sha256": visuals.FontSHA256, "templates": []string{"provider_table", "profile_top", "profile_flamegraph", "profile_diff"}, "notes": []string{"A separate desktop render window is required; no CLI image rendering.", "Office paste and SVG insertion depend on host Office/WebView versions; PNG file is the fallback."}})
}
func defaultVisualPresentation(p visuals.Presentation) visuals.Presentation {
	d := visuals.DefaultPresentation()
	if p.Width == 0 {
		p.Width = d.Width
	}
	if p.Height == 0 {
		p.Height = d.Height
	}
	if p.FontSize == 0 {
		p.FontSize = d.FontSize
	}
	if p.RowsPerPage == 0 {
		p.RowsPerPage = d.RowsPerPage
	}
	if p.Title == "" {
		p.Title = d.Title
	}
	return p
}
func (s *EngineService) RenderVisual(parent context.Context, requestJSON string) (string, error) {
	var req VisualRenderRequest
	if e := visuals.Strict([]byte(requestJSON), &req, 128<<10); e != nil {
		return "", e
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	s.visualMu.Lock()
	if s.launchVisual == nil || s.visualClosed {
		s.visualMu.Unlock()
		return "", fmt.Errorf("VISUAL_DESKTOP_RENDERER_REQUIRED")
	}
	if len(s.visualJobs) >= 2 {
		s.visualMu.Unlock()
		return "", fmt.Errorf("VISUAL_RENDER_BUSY")
	}
	var tokenBytes [16]byte
	if _, e := rand.Read(tokenBytes[:]); e != nil {
		s.visualMu.Unlock()
		return "", e
	}
	token := hex.EncodeToString(tokenBytes[:])
	job := &visualJob{done: make(chan error, 1), cancel: cancel}
	if s.visualJobs == nil {
		s.visualJobs = map[string]*visualJob{}
	}
	s.visualJobs[token] = job
	launch := s.launchVisual
	s.visualMu.Unlock()
	defer func() {
		s.visualMu.Lock()
		delete(s.visualJobs, token)
		window := job.window
		s.visualMu.Unlock()
		if window != nil {
			window.Close()
		}
	}()
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	view, e := service.VisualView(ctx, req.Handle, req.Target, req.Query)
	if e != nil {
		return "", e
	}
	store, e := s.visualStore()
	if e != nil {
		return "", e
	}
	p := defaultVisualPresentation(req.Presentation)
	spec, e := visuals.NewSpec(view, p, req.Parent)
	if e != nil {
		return "", e
	}
	if req.Parent != "" {
		_, old, e := store.Read(req.Parent)
		if e != nil {
			return "", e
		}
		if visuals.Hash(old.Receipt.Spec.Sources) != visuals.Hash(spec.Sources) {
			return "", fmt.Errorf("VISUAL_REGEN_SOURCE_MISMATCH: reopen the pinned result revisions")
		}
		if !req.Upgrade {
			if !old.Receipt.Spec.Reproducible() {
				return "", fmt.Errorf("VISUAL_RENDER_DEPENDENCY_UNAVAILABLE: open the saved image or explicitly upgrade")
			}
			if visuals.Hash(view) != old.Receipt.Spec.ViewSHA256 || visuals.Hash(p) != visuals.Hash(old.Receipt.Spec.Presentation) {
				return "", fmt.Errorf("VISUAL_REGEN_SELECTION_CHANGED: explicitly create an upgraded revision")
			}
			spec = old.Receipt.Spec
			spec.Parent = req.Parent
			if e = spec.Validate(view, false); e != nil {
				return "", e
			}
		}
	}
	font, e := distribution.Assets.ReadFile("assets/fonts/NotoSansKR.ttf")
	if e != nil {
		return "", e
	}
	license, e := distribution.Assets.ReadFile("assets/fonts/OFL.txt")
	if e != nil {
		return "", e
	}
	svgs := [][]byte{}
	for page := 0; page < visuals.Pages(view, p); page++ {
		if e = ctx.Err(); e != nil {
			return "", e
		}
		svg, e := visuals.SVG(view, spec, page, font)
		if e != nil {
			return "", e
		}
		svgs = append(svgs, svg)
	}
	s.visualMu.Lock()
	job.svg = svgs
	job.spec = spec
	s.visualMu.Unlock()
	if e = ctx.Err(); e != nil {
		return "", e
	}
	window, e := launch(token, p.Width, p.Height)
	if e != nil {
		return "", e
	}
	s.visualMu.Lock()
	job.window = window
	s.visualMu.Unlock()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("VISUAL_RENDER_CANCELED_OR_TIMEOUT: %w", ctx.Err())
	case e = <-job.done:
		if e != nil {
			return "", e
		}
	}
	if e = ctx.Err(); e != nil {
		return "", e
	}
	s.visualMu.Lock()
	pages := make([]visuals.Page, len(svgs))
	for i, svg := range svgs {
		pages[i] = visuals.Page{PNG: job.png[i], SVG: svg}
	}
	s.visualMu.Unlock()
	body, e := visuals.BuildArtifact(ctx, view, spec, pages, runtime.GOOS+"/"+runtime.GOARCH+"; separate WebView; pinned font; layout independent of screen DPR", license)
	if e != nil {
		return "", e
	}
	if e = ctx.Err(); e != nil {
		return "", e
	}
	descriptor, e := store.SaveContext(ctx, body)
	return caseJSON(descriptor, e)
}
func (s *EngineService) visualCaller(ctx context.Context, token string, job *visualJob) bool {
	if s.visualCallerOverride != nil {
		return s.visualCallerOverride(ctx, token)
	}
	window, _ := ctx.Value(application.WindowKey).(application.Window)
	if window == nil || window.Name() != "visual-"+token {
		return false
	}
	return job.window == nil || window.ID() == job.window.ID()
}
func (s *EngineService) GetVisualRenderInput(ctx context.Context, token string) (string, error) {
	s.visualMu.Lock()
	defer s.visualMu.Unlock()
	job := s.visualJobs[token]
	if job == nil || job.finished || !s.visualCaller(ctx, token, job) {
		return "", fmt.Errorf("VISUAL_RENDER_SESSION_STALE")
	}
	svgs := make([]string, len(job.svg))
	for i, svg := range job.svg {
		svgs[i] = string(svg)
	}
	return caseJSON(VisualRenderInput{visuals.RendererVersion, svgs, job.spec.Presentation.Width, job.spec.Presentation.Height, "/offline-assets/fonts/NotoSansKR.ttf", visuals.FontSHA256}, nil)
}
func (s *EngineService) CompleteVisualRender(ctx context.Context, token string, page int, pngData, errorText string) error {
	if len(pngData) > 24<<20 || len(errorText) > 1024 {
		return fmt.Errorf("VISUAL_RENDER_RESPONSE_BUDGET")
	}
	s.visualMu.Lock()
	defer s.visualMu.Unlock()
	job := s.visualJobs[token]
	if job == nil || job.finished || !s.visualCaller(ctx, token, job) {
		return fmt.Errorf("VISUAL_RENDER_SESSION_STALE")
	}
	if errorText != "" {
		job.finished = true
		job.done <- fmt.Errorf("VISUAL_RENDER_FAILED: font readiness/rasterization failed; save/retry from the desktop")
		return nil
	}
	if page != len(job.png) || page >= len(job.svg) {
		return fmt.Errorf("VISUAL_RENDER_PAGE_MISMATCH")
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(pngData, prefix) {
		return fmt.Errorf("VISUAL_PNG_INVALID")
	}
	body, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(pngData, prefix))
	if e != nil {
		return e
	}
	if e = visuals.ValidatePNG(body, job.spec.Presentation.Width, job.spec.Presentation.Height); e != nil {
		return e
	}
	job.png = append(job.png, body)
	if len(job.png) == len(job.svg) {
		job.finished = true
		job.done <- nil
	}
	return nil
}
func (s *EngineService) ListVisualArtifacts() (string, error) {
	store, e := s.visualStore()
	if e != nil {
		return "", e
	}
	return caseJSON(store.List())
}
func (s *EngineService) GetVisualArtifact(id string, page int) (string, error) {
	store, e := s.visualStore()
	if e != nil {
		return "", e
	}
	_, a, e := store.Read(id)
	if e != nil {
		return "", e
	}
	if page < 0 || page >= a.Receipt.Pages {
		return "", fmt.Errorf("VISUAL_PAGE_INVALID")
	}
	return caseJSON(map[string]any{"id": id, "table_protection": visuals.TableProtection(a.View), "spec": a.Receipt.Spec, "pages": a.Receipt.Pages, "reproducible": a.Receipt.Spec.Reproducible(), "png": "data:image/png;base64," + base64.StdEncoding.EncodeToString(a.Files[fmt.Sprintf("image-%02d.png", page+1)])}, nil)
}
func (s *EngineService) ExportVisualArtifact(ctx context.Context, id string, page int, format, path string) error {
	store, e := s.visualStore()
	if e != nil {
		return e
	}
	body, a, e := store.Read(id)
	if e != nil {
		return e
	}
	if page < 0 || page >= a.Receipt.Pages {
		return fmt.Errorf("VISUAL_PAGE_INVALID")
	}
	switch format {
	case "zip":
	case "png", "svg":
		body = a.Files[fmt.Sprintf("image-%02d.%s", page+1, format)]
	case "tsv":
		body = visuals.TSV(a.View)
	case "html":
		body = []byte(visuals.StandaloneHTML(a.View, a.Receipt.Spec))
	default:
		return fmt.Errorf("VISUAL_EXPORT_UNSUPPORTED")
	}
	if strings.ToLower(filepath.Ext(path)) != "."+format {
		return fmt.Errorf("VISUAL_EXTENSION_MISMATCH")
	}
	roots, e := assistanceRoots()
	if e != nil {
		return e
	}
	return localfiles.Publish(ctx, path, body, roots...)
}
func (s *EngineService) ImportVisualArtifact(ctx context.Context, path string) (string, error) {
	if e := ctx.Err(); e != nil {
		return "", e
	}
	body, _, e := visuals.ReadLocalArtifact(path)
	if e != nil {
		return "", e
	}
	store, e := s.visualStore()
	if e != nil {
		return "", e
	}
	if e = ctx.Err(); e != nil {
		return "", e
	}
	return caseJSON(store.SaveContext(ctx, body))
}
func (s *EngineService) DeleteVisualArtifact(id string) error {
	store, e := s.visualStore()
	if e != nil {
		return e
	}
	return store.Remove(id)
}
func (s *EngineService) CopyVisualArtifact(ctx context.Context, id string, page int, kind string) error {
	if !visualclipboard.Supported() {
		return fmt.Errorf("VISUAL_CLIPBOARD_UNSUPPORTED: save PNG or HTML/TSV instead")
	}
	window, _ := ctx.Value(application.WindowKey).(application.Window)
	if window == nil {
		return fmt.Errorf("VISUAL_DESKTOP_RENDERER_REQUIRED")
	}
	store, e := s.visualStore()
	if e != nil {
		return e
	}
	_, a, e := store.Read(id)
	if e != nil {
		return e
	}
	if page < 0 || page >= a.Receipt.Pages {
		return fmt.Errorf("VISUAL_PAGE_INVALID")
	}
	payload := visualclipboard.Payload{}
	switch kind {
	case "image":
		payload.PNG = a.Files[fmt.Sprintf("image-%02d.png", page+1)]
	case "table":
		payload.HTML = visuals.ClipboardTableHTML(a.View, a.Receipt.Spec)
		payload.Text = string(visuals.TSV(a.View))
	default:
		return fmt.Errorf("VISUAL_CLIPBOARD_KIND_INVALID")
	}
	return application.InvokeSyncWithError(func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		return visualclipboard.Write(ctx, uintptr(window.NativeWindow()), payload)
	})
}
func (s *EngineService) closeVisuals() {
	s.visualMu.Lock()
	s.visualClosed = true
	for _, job := range s.visualJobs {
		job.cancel()
	}
	store := s.visualArtifacts
	s.visualMu.Unlock()
	if store != nil {
		store.Close()
	}
}

func (s *EngineService) ExportVisualReport(ctx context.Context, handlesJSON, idsJSON, path string) error {
	var handles, ids []string
	if e := visuals.Strict([]byte(handlesJSON), &handles, 16<<10); e != nil {
		return e
	}
	if e := visuals.Strict([]byte(idsJSON), &ids, 16<<10); e != nil {
		return e
	}
	if len(handles) > 8 || len(ids) < 1 || len(ids) > 4 {
		return fmt.Errorf("VISUAL_REPORT_SELECTION_INVALID")
	}
	store, e := s.visualStore()
	if e != nil {
		return e
	}
	artifacts := map[string][]byte{}
	for _, id := range ids {
		if artifacts[id] != nil {
			return fmt.Errorf("VISUAL_REPORT_DUPLICATE")
		}
		body, _, e := store.Read(id)
		if e != nil {
			return e
		}
		artifacts[id] = body
	}
	var base []byte
	if len(handles) > 0 {
		service, e := s.evidence()
		if e != nil {
			return e
		}
		base, e = service.ReportPack(handles)
		if e != nil {
			return e
		}
	}
	body, e := visuals.ReportPack(ctx, base, artifacts)
	if e != nil {
		return e
	}
	if strings.ToLower(filepath.Ext(path)) != ".zip" {
		return fmt.Errorf("VISUAL_EXTENSION_MISMATCH")
	}
	roots, e := assistanceRoots()
	if e != nil {
		return e
	}
	return localfiles.Publish(ctx, path, body, roots...)
}
