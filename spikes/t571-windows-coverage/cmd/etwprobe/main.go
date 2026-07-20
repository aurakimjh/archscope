// Command etwprobe measures the ETW TCP/IP candidate from §10.4.1: can the
// Microsoft-Windows-Kernel-Network provider attribute connections to a PID,
// what does its event payload actually contain, and how many events does the
// kernel report lost under load? This fills the Q-WIN-ETW-PAYLOAD ledger row.
//
// It uses only tools shipped with Windows (logman, tracerpt), so nothing has
// to be installed on the test box beyond Go. It MUST run elevated — kernel
// ETW sessions require an administrator token.
//
// Flow:
//  1. start an ETS session on the Kernel-Network provider (logman ... -ets)
//  2. hold for the capture window while loadgen drives traffic
//  3. stop the session, dump the .etl to XML + a summary (tracerpt)
//  4. parse events for ProcessID + source/dest ports
//  5. read "Events Lost" from the summary for CAP-3
package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/capmodel"
	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/control"
)

const sessionName = "archscope-t571-etw"
const provider = "Microsoft-Windows-Kernel-Network"

func main() {
	window := flag.Duration("window", 30*time.Second, "capture window")
	workdir := flag.String("workdir", "results/etw", "scratch dir for etl/xml")
	out := flag.String("out", "results/obs_etw.json", "observation output path")
	flag.Parse()

	obs := capmodel.Observation{
		Candidate: capmodel.CandidateETW,
		Scope:     capmodel.ScopeProcessAttribution,
		Host:      control.Hostname(),
		OSVersion: control.OSVersion(),
		Elevated:  control.IsElevated(),
		Tool:      fmt.Sprintf("logman/tracerpt provider=%q", provider),
		StartedAt: time.Now(),
	}

	if err := run(&obs, *window, *workdir); err != nil {
		obs.Error = err.Error()
	}
	obs.EndedAt = time.Now()
	if err := control.WriteJSON(*out, obs); err != nil {
		fmt.Fprintln(os.Stderr, "etwprobe: write:", err)
		os.Exit(1)
	}
	fmt.Printf("etwprobe: %d flows, delivered=%d dropped=%d err=%q -> %s\n",
		len(obs.Flows), obs.EventsDelivered, obs.KernelReportedDropped, obs.Error, *out)
}

func run(obs *capmodel.Observation, window time.Duration, workdir string) error {
	if !obs.Elevated {
		return fmt.Errorf("kernel ETW session requires an elevated (administrator) prompt")
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return err
	}
	etl := filepath.Join(workdir, "kernelnet.etl")
	xmlf := filepath.Join(workdir, "kernelnet.xml")
	sumf := filepath.Join(workdir, "kernelnet.summary.txt")
	_ = os.Remove(etl)
	_ = os.Remove(xmlf)

	// Best-effort clean up a session left behind by a crashed prior run.
	_ = exec.Command("logman", "stop", sessionName, "-ets").Run()

	// -nb (buffers) sized up so a 500 tps burst does not trivially drop; -bs KB.
	create := exec.Command("logman", "create", "trace", sessionName,
		"-p", provider, "0xffffffffffffffff", "0xff",
		"-ets", "-o", etl, "-nb", "64", "256", "-bs", "1024", "-mode", "globalsequence")
	if outb, err := create.CombinedOutput(); err != nil {
		return fmt.Errorf("logman create: %v: %s", err, strings.TrimSpace(string(outb)))
	}
	fmt.Printf("etwprobe: capturing for %s...\n", window)

	// Sample CPU during the capture window for CAP-5.
	cpuCh := make(chan float64, 1)
	go func() {
		if v, err := control.SampleProcessorTime(window); err == nil {
			cpuCh <- v
		} else {
			cpuCh <- -1
		}
	}()

	time.Sleep(window)

	if outb, err := exec.Command("logman", "stop", sessionName, "-ets").CombinedOutput(); err != nil {
		return fmt.Errorf("logman stop: %v: %s", err, strings.TrimSpace(string(outb)))
	}
	if v := <-cpuCh; v >= 0 {
		obs.CPUOverheadPct = v
	} else {
		obs.Notes = append(obs.Notes, "typeperf CPU sample failed; CAP-5 unmeasured by probe")
	}

	// Dump to XML + summary.
	if outb, err := exec.Command("tracerpt", etl, "-o", xmlf, "-of", "XML",
		"-summary", sumf, "-y").CombinedOutput(); err != nil {
		return fmt.Errorf("tracerpt: %v: %s", err, strings.TrimSpace(string(outb)))
	}

	delivered, dropped, err := parseSummary(sumf)
	if err != nil {
		obs.Notes = append(obs.Notes, "summary parse: "+err.Error())
	}
	obs.EventsDelivered = delivered
	obs.KernelReportedDropped = dropped

	flows, payloadNote, err := parseETWXML(xmlf)
	if err != nil {
		return fmt.Errorf("parse xml: %w", err)
	}
	obs.Flows = flows
	if payloadNote != "" {
		obs.Notes = append(obs.Notes, payloadNote)
	}
	return nil
}

var (
	// tracerpt summary lines look like:
	//   Total Buffers Processed 29
	//   Total Events  Processed 100541
	//   Total Events  Lost      0
	// Match the EVENTS line specifically for delivered/lost, not buffers.
	reEventsTotal = regexp.MustCompile(`(?i)Total\s+Events\s+Processed\D+(\d+)`)
	reEventsLost  = regexp.MustCompile(`(?i)Total\s+Events\s+Lost\D+(\d+)`)
)

// parseSummary reads tracerpt's summary for the delivered / lost counters that
// back CAP-3. The exact wording varies by Windows build, so we match loosely
// and record whatever we find.
func parseSummary(path string) (delivered, lost int64, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	s := string(b)
	if m := reEventsTotal.FindStringSubmatch(s); m != nil {
		delivered, _ = strconv.ParseInt(m[1], 10, 64)
	}
	if m := reEventsLost.FindStringSubmatch(s); m != nil {
		lost, _ = strconv.ParseInt(m[1], 10, 64)
	}
	return delivered, lost, nil
}

// tracerpt XML shape (Crimson schema):
//
//	<Event><System><Execution ProcessID="1234" .../></System>
//	  <EventData><Data Name="sport">54321</Data>...</EventData></Event>
type etwEvent struct {
	System struct {
		Execution struct {
			ProcessID string `xml:"ProcessID,attr"`
		} `xml:"Execution"`
	} `xml:"System"`
	Data []struct {
		Name  string `xml:"Name,attr"`
		Value string `xml:",chardata"`
	} `xml:"EventData>Data"`
}

// parseETWXML streams the dump so a large capture does not have to fit in RAM,
// and aggregates per (pid, sport, dport) so repeated data events collapse into
// one flow with an event count.
func parseETWXML(path string) ([]capmodel.AttributedFlow, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	dec := xml.NewDecoder(f)
	type key struct{ pid, sport, dport int }
	agg := map[key]*capmodel.AttributedFlow{}
	sawPID := false
	sawPort := false

	for {
		tok, err := dec.Token()
		if err != nil {
			break // EOF or malformed tail; use what we have
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Event" {
			continue
		}
		var e etwEvent
		if err := dec.DecodeElement(&e, &se); err != nil {
			continue
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(e.System.Execution.ProcessID))
		if pid > 0 {
			sawPID = true
		}
		var sport, dport int
		for _, d := range e.Data {
			switch strings.ToLower(d.Name) {
			case "sport", "sourceport", "localport":
				sport = atoiClean(d.Value)
			case "dport", "destport", "destinationport", "remoteport":
				dport = atoiClean(d.Value)
			}
		}
		if sport > 0 || dport > 0 {
			sawPort = true
		}
		if sport == 0 && dport == 0 {
			continue // not a connection-bearing event
		}
		k := key{pid, sport, dport}
		fl := agg[k]
		if fl == nil {
			fl = &capmodel.AttributedFlow{LocalPort: sport, RemotePort: dport, PID: pid}
			agg[k] = fl
		}
		fl.Observed++
	}

	flows := make([]capmodel.AttributedFlow, 0, len(agg))
	for _, fl := range agg {
		flows = append(flows, *fl)
	}
	note := ""
	switch {
	case !sawPID && !sawPort:
		note = "ETW dump contained neither ProcessID nor port fields — provider/keyword mismatch on this build"
	case !sawPID:
		note = "ETW events carried ports but no usable ProcessID — attribution not available from this provider payload"
	case !sawPort:
		note = "ETW events carried ProcessID but no port fields — cannot join to ground-truth flows"
	}
	return flows, note, nil
}

func atoiClean(s string) int {
	s = strings.TrimSpace(s)
	// values may appear as "0x..." or plain decimal
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		if v, err := strconv.ParseInt(s[2:], 16, 32); err == nil {
			return int(v)
		}
	}
	v, _ := strconv.Atoi(s)
	return v
}
