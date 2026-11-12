package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/visuals"
	"os"
)

type testVisualWindow struct {
	token  string
	closed *atomic.Int32
}

func (w *testVisualWindow) ID() uint     { return 7 }
func (w *testVisualWindow) Name() string { return "visual-" + w.token }
func (w *testVisualWindow) Close()       { w.closed.Add(1) }

type visualContextKey struct{}

func TestT636SeparateRenderSessionSnapshotAndRegeneration(t *testing.T) {
	t.Setenv("ARCHSCOPE_STATE_ROOT", t.TempDir())
	svc := evidenceBridge(t, nil)
	t.Cleanup(svc.closeVisuals)
	body, _ := catalog.Contracts.ReadFile("contracts/profile-set.json")
	source := filepath.Join(t.TempDir(), "profile.json")
	os.WriteFile(source, body, 0600)
	job, e := svc.AnalyzeEvidence(EvidenceImportRequest{Path: source, Importer: "profile.set-json", OptionsJSON: `{"profile_ids":["worker-a","worker-b"],"metric_id":"duration"}`})
	if e != nil {
		t.Fatal(e)
	}
	job, e = svc.evidenceService.Wait(context.Background(), job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	svc.visualCallerOverride = func(ctx context.Context, token string) bool { return ctx.Value(visualContextKey{}) == token }
	var closed atomic.Int32
	errors := make(chan error, 10)
	svc.launchVisual = func(token string, width, height int) (visualRenderWindow, error) {
		w := &testVisualWindow{token, &closed}
		go func() {
			ctx := context.WithValue(context.Background(), visualContextKey{}, token)
			if _, e := svc.GetVisualRenderInput(context.Background(), token); e == nil {
				errors <- os.ErrPermission
				return
			}
			raw, e := svc.GetVisualRenderInput(ctx, token)
			if e != nil {
				errors <- e
				return
			}
			var input VisualRenderInput
			if e = json.Unmarshal([]byte(raw), &input); e != nil {
				errors <- e
				return
			}
			var picture bytes.Buffer
			if e = png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, width, height))); e != nil {
				errors <- e
				return
			}
			data := "data:image/png;base64," + base64.StdEncoding.EncodeToString(picture.Bytes())
			if e = svc.CompleteVisualRender(ctx, token, 1, data, ""); e == nil {
				errors <- os.ErrInvalid
				return
			}
			for i := range input.SVG {
				if e = svc.CompleteVisualRender(ctx, token, i, data, ""); e != nil {
					errors <- e
					return
				}
			}
		}()
		return w, nil
	}
	req := VisualRenderRequest{Handle: job.ResultHandle, Query: visuals.Query{Kind: "profile_top", Offset: 0, Limit: 25}, Presentation: visuals.DefaultPresentation()}
	req.Presentation.Width = 1600
	req.Presentation.Height = 1000
	req.Presentation.FontSize = 20
	req.Presentation.RowsPerPage = 5
	raw, _ := json.Marshal(req)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	output, e := svc.RenderVisual(ctx, string(raw))
	if e != nil {
		select {
		case err := <-errors:
			t.Fatal(err)
		default:
			t.Fatal(e)
		}
	}
	var first visuals.Descriptor
	if e = json.Unmarshal([]byte(output), &first); e != nil {
		t.Fatal(e)
	}
	if closed.Load() != 1 || len(svc.visualJobs) != 0 {
		t.Fatal("render window/session leaked", closed.Load())
	}
	preview, e := svc.GetVisualArtifact(first.ID, 0)
	if e != nil || !strings.Contains(preview, "data:image/png;base64,") {
		t.Fatal(preview, e)
	}
	req.Parent = first.ID
	req.Presentation = first.Spec.Presentation
	raw, _ = json.Marshal(req)
	output, e = svc.RenderVisual(ctx, string(raw))
	if e != nil {
		t.Fatal(e)
	}
	var next visuals.Descriptor
	json.Unmarshal([]byte(output), &next)
	if next.ID == first.ID || next.Spec.Parent != first.ID {
		t.Fatal("regeneration overwrote snapshot")
	}
	store, _ := svc.visualStore()
	before, _, e := store.Read(first.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.CopyVisualArtifact(ctx, first.ID, 0, "image"); e == nil {
		t.Fatal("windowless clipboard call succeeded")
	}
	req.Presentation.Annotation = "changed annotation"
	raw, _ = json.Marshal(req)
	if _, e = svc.RenderVisual(ctx, string(raw)); e == nil || !strings.Contains(e.Error(), "VISUAL_REGEN_SELECTION_CHANGED") {
		t.Fatal("implicit presentation revision", e)
	}
	req.Upgrade = true
	raw, _ = json.Marshal(req)
	if _, e = svc.RenderVisual(ctx, string(raw)); e != nil {
		t.Fatal(e)
	}
	after, _, e := store.Read(first.ID)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("historical bytes changed")
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if _, e = svc.RenderVisual(canceled, string(raw)); e == nil {
		t.Fatal("render ignored cancellation")
	}
	if len(svc.visualJobs) != 0 {
		t.Fatal("canceled render leaked")
	}
}
func TestT636NoDesktopAndInvalidBudgetRefusals(t *testing.T) {
	svc := &EngineService{}
	if _, e := svc.RenderVisual(context.Background(), `{"handle":"x","query":{"kind":"provider_table","offset":0,"limit":25},"presentation":{}}`); e == nil || !strings.Contains(e.Error(), "DESKTOP_RENDERER_REQUIRED") {
		t.Fatal(e)
	}
	if _, e := svc.GetVisualRenderInput(context.Background(), "expired"); e == nil {
		t.Fatal("stale session accepted")
	}
}
