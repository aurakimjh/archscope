// Command wfpprobe measures the WFP candidate from §10.4.1: at what accuracy
// can the Windows Filtering Platform attribute observed connections to an
// application? This fills the Q-WIN-WFP-ATTR ledger row.
//
// It uses `netsh wfp show netevents`, a lightweight instant XML dump of the
// WFP net-event ring buffer (local/remote ports + appId, with appId.asString
// giving the NT image path directly). This replaces the old
// `netsh wfp capture start/stop` path, which on some builds spends many
// minutes serializing a full diagnostic .cab. Only built-in tools are used
// (netsh). Must run elevated.
//
// WFP attributes by appId (image path), not by process instance. That is a
// finding, not a bug: the judge joins WFP's (localPort, image) observations to
// loadgen's port->pid ground truth, so a flow counts as correctly attributed
// when WFP saw the port AND the image matches.
//
// Honesty caveat: whether WFP logs ALLOWED connections (not just drops) depends
// on filter audit configuration. If the dump contains only CLASSIFY_DROP
// events, our allowed control traffic will not appear — the probe records the
// event-type distribution so the judge/report can say so plainly rather than
// implying WFP "failed".
package main

import (
	"encoding/hex"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/capmodel"
	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/control"
)

func main() {
	window := flag.Duration("window", 30*time.Second, "capture window")
	workdir := flag.String("workdir", "results/wfp", "scratch dir for the netevents xml")
	out := flag.String("out", "results/obs_wfp.json", "observation output path")
	flag.Parse()

	obs := capmodel.Observation{
		Candidate: capmodel.CandidateWFP,
		Scope:     capmodel.ScopeProcessAttribution,
		Host:      control.Hostname(),
		OSVersion: control.OSVersion(),
		Elevated:  control.IsElevated(),
		Tool:      "netsh wfp set options netevents=on; netsh wfp show netevents",
		StartedAt: time.Now(),
	}
	if err := run(&obs, *window, *workdir); err != nil {
		obs.Error = err.Error()
	}
	obs.EndedAt = time.Now()
	if err := control.WriteJSON(*out, obs); err != nil {
		fmt.Fprintln(os.Stderr, "wfpprobe: write:", err)
		os.Exit(1)
	}
	fmt.Printf("wfpprobe: %d flows err=%q -> %s\n", len(obs.Flows), obs.Error, *out)
}

func run(obs *capmodel.Observation, window time.Duration, workdir string) error {
	if !obs.Elevated {
		return fmt.Errorf("netsh wfp requires an elevated (administrator) prompt")
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return err
	}
	xmlf := filepath.Join(workdir, "netevents.xml")
	_ = os.Remove(xmlf)

	// Enable net-event collection, and make sure we turn it back off afterwards.
	if outb, err := exec.Command("netsh", "wfp", "set", "options", "netevents=on").CombinedOutput(); err != nil {
		return fmt.Errorf("enable netevents: %v: %s", err, strings.TrimSpace(string(outb)))
	}
	defer exec.Command("netsh", "wfp", "set", "options", "netevents=off").Run()

	fmt.Printf("wfpprobe: collecting netevents for %s...\n", window)

	cpuCh := make(chan float64, 1)
	go func() {
		if v, err := control.SampleProcessorTime(window); err == nil {
			cpuCh <- v
		} else {
			cpuCh <- -1
		}
	}()

	time.Sleep(window)

	if v := <-cpuCh; v >= 0 {
		obs.CPUOverheadPct = v
	} else {
		obs.Notes = append(obs.Notes, "typeperf CPU sample failed; CAP-5 unmeasured by probe")
	}

	// Dump the collected net events straight to XML (instant, no cab/expand).
	if outb, err := exec.Command("netsh", "wfp", "show", "netevents", "file="+xmlf).CombinedOutput(); err != nil {
		return fmt.Errorf("show netevents: %v: %s", err, strings.TrimSpace(string(outb)))
	}

	flows, typeCounts, note, err := parseNetEvents(xmlf)
	if err != nil {
		return fmt.Errorf("parse netevents.xml: %w", err)
	}
	obs.Flows = flows
	obs.EventsDelivered = int64(len(flows))
	obs.KernelReportedDropped = -1 // WFP has no ETW-style kernel drop counter
	obs.Notes = append(obs.Notes,
		"WFP has no kernel drop counter comparable to ETW; CAP-3 loss is not-applicable for this candidate")
	if len(typeCounts) > 0 {
		obs.Notes = append(obs.Notes, "netEvent 유형 분포: "+formatCounts(typeCounts))
	}
	if note != "" {
		obs.Notes = append(obs.Notes, note)
	}
	return nil
}

// netsh wfp show netevents XML shape:
//
//	<netEvents><item>
//	  <header>
//	    <localPort>7173</localPort><remotePort>7172</remotePort>
//	    <appId><data>hex</data><asString>\device\...\loadgen.exe</asString></appId>
//	  </header>
//	  <type>FWPM_NET_EVENT_TYPE_CLASSIFY_ALLOW</type>
//	</item>...</netEvents>
type wfpItem struct {
	Header struct {
		LocalPort  int `xml:"localPort"`
		RemotePort int `xml:"remotePort"`
		AppID      struct {
			Data     string `xml:"data"`
			AsString string `xml:"asString"`
		} `xml:"appId"`
	} `xml:"header"`
	Type string `xml:"type"`
}

func parseNetEvents(path string) (flows []capmodel.AttributedFlow, typeCounts map[string]int, note string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, "", err
	}
	defer f.Close()

	dec := xml.NewDecoder(f)
	type key struct {
		lport, rport int
		image        string
	}
	agg := map[key]*capmodel.AttributedFlow{}
	typeCounts = map[string]int{}
	sawApp := false

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		// netsh uses <item>; older/diagnostic dumps use <netEvent>.
		if !ok || (se.Name.Local != "item" && se.Name.Local != "netEvent") {
			continue
		}
		var it wfpItem
		if err := dec.DecodeElement(&it, &se); err != nil {
			continue
		}
		if it.Type != "" {
			typeCounts[strings.TrimSpace(it.Type)]++
		}
		if it.Header.LocalPort == 0 && it.Header.RemotePort == 0 {
			continue
		}
		image := strings.TrimSpace(it.Header.AppID.AsString)
		if image == "" {
			image = decodeAppID(it.Header.AppID.Data)
		}
		if image != "" {
			sawApp = true
		}
		k := key{it.Header.LocalPort, it.Header.RemotePort, image}
		fl := agg[k]
		if fl == nil {
			fl = &capmodel.AttributedFlow{
				LocalPort:  it.Header.LocalPort,
				RemotePort: it.Header.RemotePort,
				Image:      image,
				PID:        0, // WFP attributes by image, not PID
			}
			agg[k] = fl
		}
		fl.Observed++
	}

	for _, fl := range agg {
		flows = append(flows, *fl)
	}
	switch {
	case len(flows) == 0:
		note = "netevents 덤프에 연결 이벤트가 없음 — 허용(ALLOW) 연결이 감사(audit)되지 않는 기본 구성일 수 있음; WFP 귀속은 이 구성에서 미확정"
	case !sawApp:
		note = "netEvents에 appId 이미지 경로가 비어 있음 — 귀속 정보 없음"
	}
	return flows, typeCounts, note, nil
}

func formatCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

// decodeAppID turns a hex-encoded UTF-16LE NT device path into a plain string.
func decodeAppID(hexStr string) string {
	hexStr = strings.TrimSpace(hexStr)
	if hexStr == "" {
		return ""
	}
	raw, err := hex.DecodeString(strings.ReplaceAll(hexStr, " ", ""))
	if err != nil || len(raw) < 2 {
		return hexStr
	}
	u16 := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		u16 = append(u16, uint16(raw[i])|uint16(raw[i+1])<<8)
	}
	return strings.TrimRight(string(utf16.Decode(u16)), "\x00")
}
