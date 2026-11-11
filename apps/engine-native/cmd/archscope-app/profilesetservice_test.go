package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	pp "github.com/google/pprof/profile"
)

func TestT629WailsProfileSetExactHandleFlow(t *testing.T) {
	svc := evidenceBridge(t, nil)
	body, _ := catalog.Contracts.ReadFile("contracts/profile-set.json")
	path := filepath.Join(t.TempDir(), "model.json")
	os.WriteFile(path, body, 0600)
	job, e := svc.ImportEvidence(EvidenceImportRequest{Path: path, Importer: "profile-set", OptionsJSON: `{"profile_ids":["worker-a","worker-b"],"metric_id":"memory"}`})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	text, e := svc.GetProfileSetTree(ctx, job.ResultHandle)
	if e != nil || !strings.Contains(text, `"inclusive":"9007199254740993"`) {
		t.Fatal(text, e)
	}
	diff, e := svc.CompareProfileSets(ctx, job.ResultHandle, job.ResultHandle)
	if e != nil || !strings.Contains(diff, `"delta":"0"`) {
		t.Fatal(diff, e)
	}
	stages, e := svc.DrilldownProfileSet(ctx, job.ResultHandle, "[]")
	if e != nil || !strings.Contains(stages, `"inclusive":"9007199254740993"`) {
		t.Fatal(stages, e)
	}
	saved, e := svc.GetEvidenceResultJSON(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "result.json")
	os.WriteFile(file, []byte(saved), 0600)
	view, e := svc.LoadEvidenceResult(file)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	after, e := svc.GetEvidenceResultJSON(view.Handle)
	if e != nil || after != saved {
		t.Fatal("bridge load changed opaque source", e)
	}
	pprof := filepath.Join(t.TempDir(), "selected.pprof")
	if e = svc.ExportProfileSetPprof(ctx, view.Handle, "1", pprof); e != nil {
		t.Fatal(e)
	}
	data, _ := os.ReadFile(pprof)
	parsed, e := pp.ParseData(data)
	if e != nil || len(parsed.Sample) != 1 || parsed.Sample[0].Value[0] != 9007199254740993 {
		t.Fatal(parsed, e)
	}
	var set profilemodel.ProfileSet
	if e = json.Unmarshal(body, &set); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.DrilldownProfileSet(ctx, job.ResultHandle, `[{"path":[],"match":"prefix","ignored_required":true}]`); e == nil {
		t.Fatal("unknown filter key ignored")
	}
}

func TestR4WailsProfileQualityAndEntityRefusal(t *testing.T) {
	svc := evidenceBridge(t, nil)
	original, _ := catalog.Contracts.ReadFile("contracts/profile-set.json")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	importSet := func(body []byte) string {
		path := filepath.Join(t.TempDir(), "model.json")
		os.WriteFile(path, body, 0600)
		job, e := svc.ImportEvidence(EvidenceImportRequest{Path: path, Importer: "profile-set", OptionsJSON: `{"profile_ids":["worker-a","worker-b"],"metric_id":"duration"}`})
		if e != nil {
			t.Fatal(e)
		}
		job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
		if e != nil || job.State != "completed" {
			t.Fatal(job, e)
		}
		return job.ResultHandle
	}
	baseline := importSet(original)
	set, e := profilemodel.Decode(original)
	if e != nil {
		t.Fatal(e)
	}
	lost := "3"
	set.Quality.Lost = &lost
	modified, _ := json.Marshal(set)
	target := importSet(modified)
	text, e := svc.CompareProfileSets(ctx, baseline, target)
	if e != nil || !strings.Contains(text, `"quality_differences":["lost"]`) || !strings.Contains(text, `"lost":"3"`) {
		t.Fatal(text, e)
	}
	set.Profiles[0].Process = "pid-200"
	modified, _ = json.Marshal(set)
	target = importSet(modified)
	_, e = svc.CompareProfileSets(ctx, baseline, target)
	cause, marshalErr := json.Marshal(&e)
	if marshalErr != nil || !strings.Contains(string(cause), `"code":"entity_mapping_required"`) {
		t.Fatal(string(cause), e, marshalErr)
	}
}
