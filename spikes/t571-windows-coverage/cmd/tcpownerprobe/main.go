// Command tcpownerprobe measures the TCP endpoint-ownership candidate from
// §10.4.1. It polls the OS TCP table through iphlpapi!GetExtendedTcpTable for
// owning PID. This is the most reliable PID attribution on Windows and serves
// as the spike's cross-check: if a kernel candidate (ETW/WFP) disagrees with
// the TCP table on a flow it observed, that is a false-attribution signal.
//
// Polling can miss sub-poll-interval connections; the loadgen workers hold
// persistent keep-alive connections precisely so this scope can see them. The
// tradeoff (no coverage of short-lived flows) is recorded as a note, because
// pretending otherwise would be the exact "silent lie" §10 exists to prevent.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/capmodel"
	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/control"
)

func main() {
	window := flag.Duration("window", 30*time.Second, "capture window")
	interval := flag.Duration("interval", 1*time.Second, "poll interval")
	out := flag.String("out", "results/obs_tcpowner.json", "observation output path")
	flag.Parse()

	obs := capmodel.Observation{
		Candidate: capmodel.CandidateTCPOwner,
		Scope:     capmodel.ScopeProcessAttribution,
		Host:      control.Hostname(),
		OSVersion: control.OSVersion(),
		Elevated:  control.IsElevated(),
		Tool:      "iphlpapi!GetExtendedTcpTable (direct poll, IPv4+IPv6)",
		StartedAt: time.Now(),
	}
	if err := run(&obs, *window, *interval); err != nil {
		obs.Error = err.Error()
	}
	obs.EndedAt = time.Now()
	if err := control.WriteJSON(*out, obs); err != nil {
		fmt.Fprintln(os.Stderr, "tcpownerprobe: write:", err)
		os.Exit(1)
	}
	fmt.Printf("tcpownerprobe: %d flows err=%q -> %s\n", len(obs.Flows), obs.Error, *out)
}

type netTCPRow struct {
	LocalPort     int
	RemotePort    int
	RemoteAddress string
	OwningProcess int
	State         uint32
}

func run(obs *capmodel.Observation, window, interval time.Duration) error {
	deadline := time.Now().Add(window)
	type key struct{ lport, pid int }
	agg := map[key]*capmodel.AttributedFlow{}
	polls := 0

	cpuCh := make(chan float64, 1)
	go func() {
		if v, err := control.SampleProcessorTime(window); err == nil {
			cpuCh <- v
		} else {
			cpuCh <- -1
		}
	}()

	for time.Now().Before(deadline) {
		rows, err := pollTCP()
		if err != nil {
			return err
		}
		polls++
		for _, r := range rows {
			if r.OwningProcess == 0 || r.LocalPort == 0 {
				continue
			}
			k := key{r.LocalPort, r.OwningProcess}
			fl := agg[k]
			if fl == nil {
				fl = &capmodel.AttributedFlow{
					LocalPort:  r.LocalPort,
					RemotePort: r.RemotePort,
					RemoteHost: r.RemoteAddress,
					PID:        r.OwningProcess,
				}
				agg[k] = fl
			}
			fl.Observed++
		}
		time.Sleep(interval)
	}

	if v := <-cpuCh; v >= 0 {
		obs.CPUOverheadPct = v
	}
	for _, fl := range agg {
		obs.Flows = append(obs.Flows, *fl)
	}
	sort.Slice(obs.Flows, func(i, j int) bool {
		if obs.Flows[i].LocalPort != obs.Flows[j].LocalPort {
			return obs.Flows[i].LocalPort < obs.Flows[j].LocalPort
		}
		if obs.Flows[i].PID != obs.Flows[j].PID {
			return obs.Flows[i].PID < obs.Flows[j].PID
		}
		if obs.Flows[i].RemoteHost != obs.Flows[j].RemoteHost {
			return obs.Flows[i].RemoteHost < obs.Flows[j].RemoteHost
		}
		return obs.Flows[i].RemotePort < obs.Flows[j].RemotePort
	})
	obs.EventsDelivered = int64(polls)
	obs.KernelReportedDropped = -1 // polling has no kernel drop counter; gaps are structural, not "lost events"
	obs.Notes = append(obs.Notes,
		fmt.Sprintf("polled IPv4+IPv6 TCP owner tables directly %d times at %s interval; sub-interval connections are not observable by this scope", polls, interval))
	return nil
}
