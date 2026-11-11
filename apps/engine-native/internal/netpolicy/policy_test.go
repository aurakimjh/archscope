package netpolicy

import "testing"

func TestOfflineCountsAttemptsBeforeAnyNetworkWork(t *testing.T) {
	Enable()
	t.Cleanup(func() { state.Lock(); state.enabled = false; state.attempts = nil; state.Unlock() })
	if e := RequireOnline("positive-control"); e == nil {
		t.Fatal("online operation accepted")
	}
	r := Snapshot()
	if r.Attempts["positive-control"] != 1 {
		t.Fatal(r)
	}
	r.Attempts["positive-control"] = 55
	if Snapshot().Attempts["positive-control"] != 1 {
		t.Fatal("report aliases policy")
	}
}
