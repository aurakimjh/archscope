// Command judge turns the probe observations + loadgen ground truth into the
// §10.4.2 CAP-1..CAP-6 verdicts, the §10.4.3 disposition per candidate, the
// overall "counter fallback" safety decision, and the two appendix-A ledger
// rows (Q-WIN-ETW-PAYLOAD, Q-WIN-WFP-ATTR). It is pure data processing, so it
// builds and runs on any OS — only the probes are Windows-bound.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/capmodel"
	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/control"
)

func main() {
	dir := flag.String("dir", "results", "directory holding ground_truth.json and obs_*.json")
	baselineCPU := flag.Float64("baseline-cpu", -1, "baseline _Total %% Processor Time measured before capture (for CAP-5 overhead)")
	outJSON := flag.String("out", "results/report.json", "report JSON output")
	outMD := flag.String("md", "results/report.md", "human-readable report output")
	flag.Parse()

	var gt capmodel.GroundTruth
	if err := control.ReadJSON(filepath.Join(*dir, "ground_truth.json"), &gt); err != nil {
		fmt.Fprintln(os.Stderr, "judge: read ground truth:", err)
		os.Exit(1)
	}

	obsFiles := map[capmodel.Candidate]string{
		capmodel.CandidateETW:      "obs_etw.json",
		capmodel.CandidateWFP:      "obs_wfp.json",
		capmodel.CandidateTCPOwner: "obs_tcpowner.json",
		capmodel.CandidateNpcap:    "obs_npcap.json",
	}
	// Each candidate pass runs its own loadgen with fresh ephemeral ports, so
	// each obs must be scored against ITS pass's ground truth. When present, a
	// per-candidate ground_truth_<cand>.json is authoritative; otherwise fall
	// back to the shared ground_truth.json (last pass wins). Without this, an
	// obs from an earlier invocation is scored against a newer ground truth and
	// spuriously reads as "not observed".
	gtFiles := map[capmodel.Candidate]string{
		capmodel.CandidateETW:      "ground_truth_etw.json",
		capmodel.CandidateWFP:      "ground_truth_wfp.json",
		capmodel.CandidateTCPOwner: "ground_truth_tcpowner.json",
		capmodel.CandidateNpcap:    "ground_truth_npcap.json",
	}

	rep := capmodel.Report{
		GeneratedAt: time.Now(),
		Host:        control.Hostname(),
		GroundTruth: gt,
	}

	order := []capmodel.Candidate{
		capmodel.CandidateETW, capmodel.CandidateWFP,
		capmodel.CandidateTCPOwner, capmodel.CandidateNpcap,
	}
	for _, cand := range order {
		path := filepath.Join(*dir, obsFiles[cand])
		var obs capmodel.Observation
		if err := control.ReadJSON(path, &obs); err != nil {
			if cand == capmodel.CandidateNpcap {
				continue // Npcap is optional; skip silently when not exercised
			}
			// Missing observation for a candidate we expected = unmeasured.
			rep.Results = append(rep.Results, unmeasured(cand, "observation file missing: "+err.Error()))
			continue
		}
		if rep.OSVersion == "" {
			rep.OSVersion = obs.OSVersion
		}
		cgt := gt
		var pcg capmodel.GroundTruth
		if err := control.ReadJSON(filepath.Join(*dir, gtFiles[cand]), &pcg); err == nil && len(pcg.Controls) > 0 {
			cgt = pcg
		}
		rep.Results = append(rep.Results, score(cand, obs, cgt, *baselineCPU))
	}

	// §10.4.3 last row: if no candidate earns a ratio-bearing disposition,
	// remove absolute coverage ratios and keep only the five counters.
	ratioBearing := false
	for _, r := range rep.Results {
		if strings.Contains(r.Disposition, "ratio 노출") {
			ratioBearing = true
		}
	}
	rep.CounterFallback = !ratioBearing
	if ratioBearing {
		rep.OverallOutcome = "최소 한 후보가 CAP-1~CAP-4를 통과했으므로 해당 scope에서 coverage ratio를 노출할 수 있다."
	} else {
		rep.OverallOutcome = "어떤 후보도 CAP-1~CAP-4를 통과하지 못했다. 절대 coverage ratio를 제거하고 " +
			"captured/passthrough/unattributed/dropped/unsupported 5개 카운터만 유지한다 (§10.1.2)."
	}

	rep.LoopbackMode = strings.HasPrefix(gt.ListenerAddr, "127.")
	for _, r := range rep.Results {
		if strings.Contains(r.Disposition, "loopback 미관측") {
			rep.Findings = append(rep.Findings, fmt.Sprintf(
				"%s: loopback(127.x) 트래픽을 관측하지 못함 — 이 scope의 귀속 측정은 실 NIC 트래픽으로 재실행 필요",
				r.Candidate))
		}
	}

	rep.AppendixRows = appendixRows(rep.Results)

	if err := control.WriteJSON(*outJSON, rep); err != nil {
		fmt.Fprintln(os.Stderr, "judge: write json:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*outMD, []byte(renderMarkdown(rep)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "judge: write md:", err)
		os.Exit(1)
	}
	fmt.Printf("judge: wrote %s and %s\n", *outJSON, *outMD)
	fmt.Println(rep.OverallOutcome)
}

func unmeasured(cand capmodel.Candidate, reason string) capmodel.CapResult {
	return capmodel.CapResult{
		Candidate:   cand,
		Measured:    false,
		Criteria:    map[string]capmodel.Verdict{},
		Disposition: "미측정 — " + reason,
	}
}

func base(p string) string { return strings.ToLower(filepath.Base(strings.ReplaceAll(p, `\`, `/`))) }

func score(cand capmodel.Candidate, obs capmodel.Observation, gt capmodel.GroundTruth, baselineCPU float64) capmodel.CapResult {
	res := capmodel.CapResult{
		Candidate: cand,
		Scope:     obs.Scope,
		Criteria:  map[string]capmodel.Verdict{},
	}
	if obs.Error != "" {
		res.Measured = false
		res.Disposition = "미측정 — probe 오류: " + obs.Error
		return res
	}
	res.Measured = true

	// Index probe flows by local port.
	byPort := map[int][]capmodel.AttributedFlow{}
	for _, fl := range obs.Flows {
		byPort[fl.LocalPort] = append(byPort[fl.LocalPort], fl)
	}

	// Ground-truth control ports.
	var controls []capmodel.ControlProcess
	for _, c := range gt.Controls {
		if c.LocalPort > 0 {
			controls = append(controls, c)
		}
	}

	providesPID := false
	providesImage := false
	for _, fl := range obs.Flows {
		if fl.PID > 0 {
			providesPID = true
		}
		if fl.Image != "" {
			providesImage = true
		}
	}

	// CAP-1 attribution accuracy + CAP-2 false attribution.
	correct, falseAttr := 0, 0
	for _, c := range controls {
		flows := byPort[c.LocalPort]
		if len(flows) == 0 {
			continue // missed => not correct (counts against accuracy)
		}
		matched := false
		for _, fl := range flows {
			switch {
			case fl.PID > 0:
				if fl.PID == c.PID {
					matched = true
				} else {
					falseAttr++ // this port belongs to c.PID; a different PID here is a false attribution
				}
			case fl.Image != "":
				if base(fl.Image) == base(c.Image) {
					matched = true
				} else {
					falseAttr++
				}
			}
		}
		if matched {
			correct++
		}
	}
	acc := 0.0
	if len(controls) > 0 {
		acc = float64(correct) / float64(len(controls))
	}

	res.Criteria["CAP-1"] = capmodel.Verdict{
		ID: "CAP-1", Name: "귀속 정확도", Threshold: "≥ 95%",
		Value:   fmt.Sprintf("%.1f%% (%d/%d control ports)", acc*100, correct, len(controls)),
		Numeric: acc * 100, Applied: len(controls) > 0, Pass: len(controls) > 0 && acc >= 0.95,
		Detail: attrGranularity(providesPID, providesImage),
	}
	res.Criteria["CAP-2"] = capmodel.Verdict{
		ID: "CAP-2", Name: "오탐(false attribution)", Threshold: "= 0건",
		Value:   fmt.Sprintf("%d건", falseAttr),
		Numeric: float64(falseAttr), Applied: len(controls) > 0, Pass: falseAttr == 0,
		Detail: "통신하지 않은/다른 프로세스에 트래픽이 귀속된 사례",
	}

	// CAP-3 loss rate.
	capLoss := capmodel.Verdict{ID: "CAP-3", Name: "손실률", Threshold: "< 1% 이고 카운터 노출 가능"}
	if obs.KernelReportedDropped < 0 {
		capLoss.Applied = false
		capLoss.Value = "N/A"
		capLoss.Detail = "이 후보에는 커널 손실 카운터가 없다(폴링/미해당)"
	} else {
		total := obs.EventsDelivered + obs.KernelReportedDropped
		loss := 0.0
		if total > 0 {
			loss = float64(obs.KernelReportedDropped) / float64(total) * 100
		}
		capLoss.Applied = true
		capLoss.Numeric = loss
		capLoss.Value = fmt.Sprintf("%.3f%% (dropped=%d, delivered=%d)", loss, obs.KernelReportedDropped, obs.EventsDelivered)
		capLoss.Pass = loss < 1.0 // counter is exposable by construction (we read it)
	}
	res.Criteria["CAP-3"] = capLoss

	// CAP-4 bypass detection.
	capBypass := capmodel.Verdict{ID: "CAP-4", Name: "우회 탐지", Threshold: "탐지 성공"}
	var bypass *capmodel.ControlProcess
	for i := range gt.Controls {
		if !gt.Controls[i].GoesViaProxy && gt.Controls[i].LocalPort > 0 {
			bypass = &gt.Controls[i]
			break
		}
	}
	if bypass == nil {
		capBypass.Applied = false
		capBypass.Value = "N/A"
		capBypass.Detail = "ground truth에 bypass control이 없음 (loadgen -workers ≥ 2 필요)"
	} else {
		seen := false
		for _, fl := range byPort[bypass.LocalPort] {
			if (fl.PID > 0 && fl.PID == bypass.PID) || (fl.Image != "" && base(fl.Image) == base(bypass.Image)) {
				seen = true
			}
		}
		capBypass.Applied = true
		capBypass.Pass = seen
		if seen {
			capBypass.Value = fmt.Sprintf("탐지됨 (port %d, pid %d)", bypass.LocalPort, bypass.PID)
		} else {
			capBypass.Value = fmt.Sprintf("미탐지 (port %d)", bypass.LocalPort)
		}
	}
	res.Criteria["CAP-4"] = capBypass

	// CAP-5 CPU overhead.
	capCPU := capmodel.Verdict{ID: "CAP-5", Name: "CPU 오버헤드", Threshold: "< 10%p"}
	if obs.CPUOverheadPct <= 0 {
		capCPU.Applied = false
		capCPU.Value = "미측정"
		capCPU.Detail = "probe가 typeperf 샘플을 얻지 못함"
	} else if baselineCPU >= 0 {
		overhead := obs.CPUOverheadPct - baselineCPU
		if overhead < 0 {
			overhead = 0
		}
		capCPU.Applied = true
		capCPU.Numeric = overhead
		capCPU.Value = fmt.Sprintf("%.1f%%p (capture %.1f%% − baseline %.1f%%)", overhead, obs.CPUOverheadPct, baselineCPU)
		capCPU.Pass = overhead < 10.0
	} else {
		capCPU.Applied = true
		capCPU.Numeric = obs.CPUOverheadPct
		capCPU.Value = fmt.Sprintf("%.1f%% (절대값; baseline 미제공)", obs.CPUOverheadPct)
		capCPU.Pass = obs.CPUOverheadPct < 10.0
		capCPU.Detail = "-baseline-cpu 를 주면 오버헤드 delta로 판정한다"
	}
	res.Criteria["CAP-5"] = capCPU

	// CAP-6 privilege/install consistency.
	capPriv := capmodel.Verdict{
		ID: "CAP-6", Name: "권한·설치", Threshold: "9.3.5 요구와 일치",
		Applied: true, Pass: obs.Elevated,
	}
	if obs.Elevated {
		capPriv.Value = "관리자 권한으로 실행됨(요구와 일치)"
	} else {
		capPriv.Value = "비관리자 실행 — 이 후보의 권한 요구와 불일치"
	}
	capPriv.Detail = "설치 비용(예: Npcap 별도 설치)은 수동 확인 항목"
	res.Criteria["CAP-6"] = capPriv

	// Loopback awareness: the Kernel-Network ETW provider (and possibly other
	// candidates) do not observe 127.0.0.0/8 traffic. In loopback smoke mode a
	// candidate that saw NONE of the control ports has not "failed" attribution
	// — it simply cannot see loopback. Record CAP-1/2/4 as N/A rather than a
	// misleading 0%, so the ledger stays honest. CAP-3/5/6 (loss, CPU,
	// privilege) are independent of the control traffic and stay measured.
	loopback := strings.HasPrefix(gt.ListenerAddr, "127.")
	controlObserved := 0
	for _, c := range controls {
		if len(byPort[c.LocalPort]) > 0 {
			controlObserved++
		}
	}
	if loopback && len(controls) > 0 && controlObserved == 0 {
		for _, id := range []string{"CAP-1", "CAP-2", "CAP-4"} {
			v := res.Criteria[id]
			v.Applied = false
			v.Pass = false
			v.Value = "N/A — loopback 트래픽 미관측(실 NIC 재실행 필요)"
			res.Criteria[id] = v
		}
		res.Disposition = "loopback 미관측 — 귀속(CAP-1/2/4)은 실 NIC 재실행 필요; payload/손실/권한은 측정됨"
		return res
	}

	res.Disposition = disposition(res)
	return res
}

func attrGranularity(pid, image bool) string {
	switch {
	case pid:
		return "PID 단위 귀속"
	case image:
		return "image(appId) 단위 귀속 — 동일 실행 파일의 프로세스 인스턴스는 구분 불가"
	default:
		return "귀속 정보 없음"
	}
}

// disposition applies the §10.4.3 table for one candidate.
func disposition(r capmodel.CapResult) string {
	c2 := r.Criteria["CAP-2"]
	if c2.Applied && !c2.Pass {
		return "폐기 — CAP-2 실패(오탐 발생). 부분 사용도 하지 않는다 (§10.4.3)"
	}
	c1 := r.Criteria["CAP-1"]
	c3 := r.Criteria["CAP-3"]
	c4 := r.Criteria["CAP-4"]
	cap14Pass := c1.Applied && c1.Pass && c4.Applied && c4.Pass && (!c3.Applied || c3.Pass)
	if cap14Pass {
		return "coverage ratio 노출 가능, Confidence: high (§10.4.3)"
	}
	if c1.Applied && c1.Pass && c3.Applied && !c3.Pass {
		return "ratio 노출 + 손실률 병기, Confidence: medium (§10.4.3)"
	}
	if c1.Applied && c1.Pass {
		return "부분 통과 — CAP-1 통과이나 CAP-3/CAP-4 미충족; ratio 노출 보류"
	}
	return "coverage ratio 미지원 — CAP-1 미충족 (이 scope는 검증된 분모를 만들지 못함)"
}

func appendixRows(results []capmodel.CapResult) []capmodel.AppendixRow {
	find := func(c capmodel.Candidate) *capmodel.CapResult {
		for i := range results {
			if results[i].Candidate == c {
				return &results[i]
			}
		}
		return nil
	}

	rows := []capmodel.AppendixRow{}

	// Q-WIN-ETW-PAYLOAD — payload composition + loss are the core claim; the
	// attribution-accuracy number is a bonus that only a real-NIC run yields.
	etw := find(capmodel.CandidateETW)
	if etw == nil || !etw.Measured {
		rows = append(rows, capmodel.AppendixRow{ClaimID: "Q-WIN-ETW-PAYLOAD",
			Fact:   "미측정 — etwprobe가 실행되지 못함",
			Impact: "9.3.1 행렬의 `미검증` 칸을 채우지 못함", Status: "open"})
	} else {
		c1 := etw.Criteria["CAP-1"]
		c3 := etw.Criteria["CAP-3"]
		fact := fmt.Sprintf("payload: %s (ProcessID+sport+dport 존재); 손실률 %s", c1.Detail, valueOrNA(c3))
		status := "partial"
		switch {
		case c1.Applied && c1.Pass:
			fact += fmt.Sprintf("; 귀속 정확도 %s", c1.Value)
			status = "fixed"
		case !c1.Applied:
			fact += "; 귀속 정확도는 loopback 미관측으로 실 NIC 재실행 필요"
		default:
			fact += fmt.Sprintf("; 귀속 정확도 %s (기준 미달)", c1.Value)
		}
		rows = append(rows, capmodel.AppendixRow{ClaimID: "Q-WIN-ETW-PAYLOAD",
			Fact: fact, Impact: "disposition: " + etw.Disposition, Status: status})
	}

	// Q-WIN-WFP-ATTR — attribution accuracy is the whole claim here.
	wfp := find(capmodel.CandidateWFP)
	rows = append(rows, ledgerRow("Q-WIN-WFP-ATTR", wfp,
		func(r *capmodel.CapResult) string {
			c1 := r.Criteria["CAP-1"]
			return fmt.Sprintf("WFP netEvents 귀속 정확도 %s; %s", valueOrNA(c1), c1.Detail)
		}))

	return rows
}

func ledgerRow(id string, r *capmodel.CapResult, fact func(*capmodel.CapResult) string) capmodel.AppendixRow {
	if r == nil || !r.Measured {
		return capmodel.AppendixRow{
			ClaimID: id,
			Fact:    "미측정 — 해당 probe가 실행되지 못함",
			Impact:  "9.3.1 행렬의 `미검증` 칸을 채우지 못함; coverage ratio 승격 보류",
			Status:  "open",
		}
	}
	c1 := r.Criteria["CAP-1"]
	status := "partial"
	if c1.Applied && c1.Pass {
		status = "fixed"
	}
	return capmodel.AppendixRow{
		ClaimID: id,
		Fact:    fact(r),
		Impact:  "disposition: " + r.Disposition,
		Status:  status,
	}
}

func valueOrNA(v capmodel.Verdict) string {
	if !v.Applied {
		return "N/A"
	}
	return v.Value
}

func renderMarkdown(rep capmodel.Report) string {
	var b strings.Builder
	b.WriteString("# T-571 Windows proof-of-capability spike 결과\n\n")
	fmt.Fprintf(&b, "- 생성 시각: %s\n", rep.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- 호스트: %s\n", rep.Host)
	fmt.Fprintf(&b, "- OS: %s\n", rep.OSVersion)
	fmt.Fprintf(&b, "- 목표 부하: %d tps, 달성 %.0f tps, control 프로세스 %d개, 연결 %d개\n\n",
		rep.GroundTruth.TargetTPS, rep.GroundTruth.AchievedTPS,
		len(rep.GroundTruth.Controls), rep.GroundTruth.TotalConnections)

	if rep.LoopbackMode {
		b.WriteString("> ⚠ **loopback smoke 모드** — 대조군 트래픽이 127.x 였다. Kernel-Network ETW는 " +
			"loopback을 관측하지 않으므로 ETW의 귀속(CAP-1/2/4)은 실 NIC 타깃(`-Target host:port`)으로 " +
			"재실행해야 확정된다.\n\n")
	}
	if len(rep.Findings) > 0 {
		b.WriteString("### 교차 관찰\n\n")
		for _, f := range rep.Findings {
			fmt.Fprintf(&b, "- %s\n", f)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "## 종합 판정\n\n%s\n\n", rep.OverallOutcome)
	if rep.CounterFallback {
		b.WriteString("> **counter fallback = true** — 절대 coverage ratio를 제거하고 5개 카운터만 노출한다.\n\n")
	}

	caps := []string{"CAP-1", "CAP-2", "CAP-3", "CAP-4", "CAP-5", "CAP-6"}
	for _, r := range rep.Results {
		fmt.Fprintf(&b, "## 후보: %s (scope: %s)\n\n", r.Candidate, r.Scope)
		if !r.Measured {
			fmt.Fprintf(&b, "%s\n\n", r.Disposition)
			continue
		}
		b.WriteString("| ID | 기준 | 통과조건 | 측정값 | 판정 |\n|---|---|---|---|---|\n")
		for _, id := range caps {
			v := r.Criteria[id]
			verdict := "—"
			if v.Applied {
				if v.Pass {
					verdict = "✅ pass"
				} else {
					verdict = "❌ fail"
				}
			} else {
				verdict = "· N/A"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", v.ID, v.Name, v.Threshold, v.Value, verdict)
		}
		fmt.Fprintf(&b, "\n**Disposition:** %s\n\n", r.Disposition)
	}

	b.WriteString("## 부록 A 갱신 행 (open → 상태)\n\n")
	b.WriteString("| Claim ID | Fact | Impact | Status |\n|---|---|---|---|\n")
	rows := append([]capmodel.AppendixRow{}, rep.AppendixRows...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].ClaimID < rows[j].ClaimID })
	for _, r := range rows {
		fmt.Fprintf(&b, "| `%s` | %s | %s | **%s** |\n", r.ClaimID, r.Fact, r.Impact, r.Status)
	}
	b.WriteString("\n")

	b.WriteString("## 다음 단계\n\n")
	b.WriteString("1. 위 부록 A 행을 `docs/ko/SYSTEM_HTTP_CAPTURE.md` 부록 A에 반영한다.\n")
	b.WriteString("2. §9.3.1 fidelity 행렬의 `미검증` 칸을 측정값으로 확정한다.\n")
	b.WriteString("3. §10 게이트(표의 9번 행)를 판정 결과에 맞춰 갱신하고 T-571을 닫는다.\n")
	return b.String()
}
