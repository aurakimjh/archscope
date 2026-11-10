package summary

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"testing"
)

func TestSummaryKeepsUnknownsPerspectivesAndConnections(t *testing.T) {
	rows := []models.CaptureTransaction{{ID: "a", ConnectionID: "same", StartedAt: "2026-11-10T00:00:00Z", TotalMS: 40, StatusCode: 200, State: models.TxComplete, Request: models.HTTPMessage{BodySize: 10}, Response: models.HTTPMessage{BodySize: 20, BodyDecoded: 30}, Timings: models.TimingSet{ProxyUpstream: &models.TimingPhases{DNS: models.KnownDuration(2), Wait: models.KnownDuration(30)}, ClientProxy: &models.TimingPhases{Wait: models.KnownDuration(5)}}}, {ID: "b", ConnectionID: "same", StartedAt: "2026-11-10T00:00:00.050Z", TotalMS: 20, StatusCode: 503, State: models.TxComplete, Request: models.HTTPMessage{BodySize: -1}, Response: models.HTTPMessage{BodySize: -1, BodyDecoded: -1}, Timings: models.TimingSet{ProxyUpstream: &models.TimingPhases{DNS: models.NotApplicableDuration(), Wait: models.KnownDuration(10)}}}}
	got := Build(rows)
	if got.Total != 2 || got.Completed != 2 || got.Sent.Bytes != 10 || got.Sent.Unknown != 1 || got.Connections != 1 || got.DNS != 1 || got.ElapsedMS != 70 || got.CompressionSaved != 10 || got.CompressionKnown != 1 || got.StatusCounts["5xx"] != 1 {
		t.Fatalf("%+v", got)
	}
	for _, p := range got.Phases {
		if p.Name == "wait" && p.Perspective == "proxyUpstream" && p.MS != 40 {
			t.Fatal(p)
		}
		if p.Name == "wait" && p.Perspective == "clientProxy" && (p.MS != 5 || p.Unknown != 1) {
			t.Fatal(p)
		}
	}
	rows[1].State = models.TxReceiving
	got = Build(rows)
	if got.Pending != 1 || got.Completed != 1 {
		t.Fatal(got)
	}
}
