package proxy

import (
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestReusedConnectionBlockedMeasuresPoolAcquisition(t *testing.T) {
	base := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	trace := &upstreamTrace{
		getConn:      base,
		gotConn:      base.Add(2 * time.Millisecond),
		wroteRequest: base.Add(3 * time.Millisecond),
		firstByte:    base.Add(8 * time.Millisecond),
		reused:       true,
	}

	phases := trace.Phases(base.Add(10 * time.Millisecond))
	if phases.Blocked.State != models.TimingKnown || phases.Blocked.MS != 2 {
		t.Fatalf("blocked=%+v, want the 2ms GetConn-to-GotConn pool acquisition", phases.Blocked)
	}
	for name, phase := range map[string]models.Duration{
		"dns": phases.DNS, "connect": phases.Connect, "tls": phases.TLS,
	} {
		if phase.State != models.TimingNotApplicable {
			t.Fatalf("%s=%+v, want not_applicable on a reused connection", name, phase)
		}
	}
}
