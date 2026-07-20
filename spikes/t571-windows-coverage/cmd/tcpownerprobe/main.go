// Command tcpownerprobe measures the TCP endpoint-ownership candidate from
// §10.4.1. It polls the OS TCP table (Get-NetTCPConnection, which wraps
// GetExtendedTcpTable) for owning PID + image. This is the most reliable PID
// attribution on Windows and serves as the spike's cross-check: if a kernel
// candidate (ETW/WFP) disagrees with the TCP table on a flow it observed, that
// is a false-attribution signal.
//
// Polling can miss sub-poll-interval connections; the loadgen workers hold
// persistent keep-alive connections precisely so this scope can see them. The
// tradeoff (no coverage of short-lived flows) is recorded as a note, because
// pretending otherwise would be the exact "silent lie" §10 exists to prevent.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
		Tool:      "Get-NetTCPConnection (poll)",
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
	LocalPort     int    `json:"LocalPort"`
	RemotePort    int    `json:"RemotePort"`
	RemoteAddress string `json:"RemoteAddress"`
	OwningProcess int    `json:"OwningProcess"`
	State         string `json:"State"`
}

func run(obs *capmodel.Observation, window, interval time.Duration) error {
	deadline := time.Now().Add(window)
	type key struct{ lport, pid int }
	agg := map[key]*capmodel.AttributedFlow{}
	imageCache := map[int]string{}
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
			img := imageCache[r.OwningProcess]
			if img == "" {
				img = resolveImage(r.OwningProcess)
				imageCache[r.OwningProcess] = img
			}
			k := key{r.LocalPort, r.OwningProcess}
			fl := agg[k]
			if fl == nil {
				fl = &capmodel.AttributedFlow{
					LocalPort:  r.LocalPort,
					RemotePort: r.RemotePort,
					RemoteHost: r.RemoteAddress,
					PID:        r.OwningProcess,
					Image:      img,
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
	obs.EventsDelivered = int64(polls)
	obs.KernelReportedDropped = -1 // polling has no kernel drop counter; gaps are structural, not "lost events"
	obs.Notes = append(obs.Notes,
		fmt.Sprintf("polled TCP table %d times at %s interval; sub-interval connections are not observable by this scope", polls, interval))
	return nil
}

func pollTCP() ([]netTCPRow, error) {
	// -EA SilentlyContinue so a transient enumeration hiccup does not abort.
	ps := `Get-NetTCPConnection -EA SilentlyContinue | ` +
		`Select-Object LocalPort,RemotePort,RemoteAddress,OwningProcess,@{n='State';e={$_.State.ToString()}} | ` +
		`ConvertTo-Json -Compress -Depth 3`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps).Output()
	if err != nil {
		return nil, fmt.Errorf("Get-NetTCPConnection: %w", err)
	}
	trimmed := trimJSON(out)
	if len(trimmed) == 0 {
		return nil, nil
	}
	// ConvertTo-Json emits an object (not array) when there is a single row.
	if trimmed[0] == '{' {
		var one netTCPRow
		if err := json.Unmarshal(trimmed, &one); err != nil {
			return nil, err
		}
		return []netTCPRow{one}, nil
	}
	var rows []netTCPRow
	if err := json.Unmarshal(trimmed, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func resolveImage(pid int) string {
	ps := fmt.Sprintf(`(Get-Process -Id %d -EA SilentlyContinue).Path`, pid)
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps).Output()
	if err != nil {
		return ""
	}
	return trimSpace(string(out))
}

func trimJSON(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t' || b[i] == 0xEF || b[i] == 0xBB || b[i] == 0xBF) {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\n' || b[j-1] == '\r' || b[j-1] == '\t') {
		j--
	}
	return b[i:j]
}

func trimSpace(s string) string {
	return string(trimJSON([]byte(s)))
}
