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

// A preview cap or interrupted read is not evidence of compression overhead.
func TestCompressionUsesOnlyCompleteUntruncatedBodyPairs(t *testing.T) {
	rows := []models.CaptureTransaction{
		{State: models.TxComplete, Response: models.HTTPMessage{BodySize: 46, BodyDecoded: 26, BodyStorage: "inline"}},
		{State: models.TxComplete, StatusCode: 503, Response: models.HTTPMessage{BodySize: 27, BodyDecoded: 27, BodyStorage: "inline"}},
		{State: models.TxFailed, Response: models.HTTPMessage{BodySize: 12, BodyDecoded: 5, BodyStorage: "inline"}},
		{State: models.TxAborted, Response: models.HTTPMessage{BodySize: 100, BodyDecoded: 10, BodyStorage: "inline"}},
		{State: models.TxReceiving, Response: models.HTTPMessage{BodySize: 100, BodyDecoded: 20, BodyStorage: "inline"}},
		{State: models.TxComplete, Response: models.HTTPMessage{BodySize: 1048634, BodyDecoded: 1048576, BodyStorage: "truncated"}},
		{State: models.TxComplete, Response: models.HTTPMessage{BodySize: 21, BodyDecoded: -1, BodyStorage: "encoded"}},
		{State: models.TxComplete, Response: models.HTTPMessage{BodySize: -1, BodyDecoded: 3}},
	}
	got := Build(rows)
	if got.CompressionKnown != 2 || got.CompressionSaved != -20 {
		t.Fatalf("partial body sizes invented compression: known=%d saved=%d", got.CompressionKnown, got.CompressionSaved)
	}
	if got.Received.Bytes != 1048940 || got.Decoded.Bytes != 1048667 || got.Received.Unknown != 1 || got.Decoded.Unknown != 1 {
		t.Fatalf("compression eligibility must not discard independently observed byte totals: %+v", got)
	}
}
