// Command loadgen produces the controlled traffic that the T-571 spike scores
// every capture candidate against (docs/ko/SYSTEM_HTTP_CAPTURE.md §10.4.2).
//
// In parent mode it either runs an in-process HTTP listener or connects its
// workers to a remote server-mode listener. Each worker holds ONE long-lived
// keep-alive connection (so it owns one stable local port for the whole run)
// and writes its PID, local port, and counters to a private result file. The
// parent combines those files into ground_truth.json, which is the answer key
// the judge uses for CAP-1 (attribution accuracy) and CAP-2 (false
// attribution). Keeping ground truth on the measurement host lets the HTTP
// listener live on a genuinely separate Windows host.
//
// One worker is designated a "bypass" process (goes_via_proxy=false). A real
// proxy-based capture would miss it; a kernel-scope candidate must still see
// it. That worker is the CAP-4 target.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/capmodel"
	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/control"
)

func main() {
	role := flag.String("role", "parent", "parent | worker | server")
	label := flag.String("label", "", "worker label (worker role)")
	target := flag.String("target", "", "remote listener host:port (parent or worker role)")
	bypass := flag.Bool("bypass", false, "this worker is the proxy-bypass control (worker role)")
	tps := flag.Int("tps", 500, "total target transactions/sec (parent role)")
	workers := flag.Int("workers", 5, "number of worker processes (parent role)")
	dur := flag.Duration("duration", 30*time.Second, "run duration")
	listen := flag.String("listen", "127.0.0.1:0", "listener bind addr (local parent or server role); port 0 => auto")
	out := flag.String("out", "results/ground_truth.json", "ground-truth output path (parent role)")
	result := flag.String("result", "", "private worker-result JSON path (worker role)")
	flag.Parse()

	switch *role {
	case "worker":
		wr := runWorker(*label, *target, *bypass, *tps, *dur)
		if *result != "" {
			if err := control.WriteJSON(*result, wr); err != nil {
				fmt.Fprintln(os.Stderr, "loadgen worker:", err)
				os.Exit(1)
			}
		}
		if wr.LocalPort == 0 || wr.OK == 0 {
			os.Exit(1)
		}
		return
	case "server":
		if err := runServer(*listen); err != nil {
			fmt.Fprintln(os.Stderr, "loadgen server:", err)
			os.Exit(1)
		}
		return
	case "parent":
		if err := runParent(*listen, *target, *out, *tps, *workers, *dur); err != nil {
			fmt.Fprintln(os.Stderr, "loadgen:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "loadgen: unknown -role %q (want parent, worker, or server)\n", *role)
		os.Exit(2)
	}
}

// -------- parent --------

type workerResult struct {
	PID       int    `json:"pid"`
	Label     string `json:"label"`
	Bypass    bool   `json:"bypass"`
	LocalPort int    `json:"local_port"`
	Attempted int    `json:"attempted"`
	OK        int    `json:"ok"`
	Failed    int    `json:"failed"`
}

func runParent(listen, remoteTarget, out string, tps, workers int, dur time.Duration) error {
	if workers < 1 {
		return fmt.Errorf("need at least 1 worker")
	}

	target := strings.TrimSpace(remoteTarget)
	var localServer *http.Server
	if target == "" {
		ln, err := net.Listen("tcp", listen)
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		localServer = &http.Server{Handler: transactionHandler()}
		go localServer.Serve(ln)
		defer localServer.Close()
		target = ln.Addr().String()
		fmt.Printf("loadgen local listener on %s\n", target)
	} else {
		if err := validateTarget(target); err != nil {
			return err
		}
		fmt.Printf("loadgen remote listener target %s\n", target)
	}
	fmt.Printf("spawning %d workers for %s at ~%d tps\n", workers, dur, tps)

	self, err := os.Executable()
	if err != nil {
		return err
	}
	resultDir, err := os.MkdirTemp("", "archscope-t571-loadgen-")
	if err != nil {
		return fmt.Errorf("create worker result directory: %w", err)
	}
	defer os.RemoveAll(resultDir)

	perWorkerTPS := tps / workers
	if perWorkerTPS < 1 {
		perWorkerTPS = 1
	}

	start := time.Now()
	var wg sync.WaitGroup
	resultPaths := make([]string, 0, workers)
	var waitMu sync.Mutex
	var waitErrors []string
	for i := 0; i < workers; i++ {
		lbl := fmt.Sprintf("worker-%d", i)
		isBypass := i == workers-1 && workers > 1 // last worker is the CAP-4 bypass control
		resultPath := filepath.Join(resultDir, fmt.Sprintf("worker-%d.json", i))
		args := []string{
			"-role", "worker",
			"-label", lbl,
			"-target", target,
			"-tps", strconv.Itoa(perWorkerTPS),
			"-duration", dur.String(),
			"-result", resultPath,
		}
		if isBypass {
			args = append(args, "-bypass")
		}
		cmd := exec.Command(self, args...)
		cmd.Stderr = prefixWriter{lbl, os.Stderr}
		if err := cmd.Start(); err != nil {
			// Do not abandon already-started workers or remove their result
			// directory while they are still running.
			wg.Wait()
			return fmt.Errorf("spawn %s: %w", lbl, err)
		}
		resultPaths = append(resultPaths, resultPath)
		wg.Add(1)
		go func(label string, c *exec.Cmd) {
			defer wg.Done()
			if err := c.Wait(); err != nil {
				waitMu.Lock()
				waitErrors = append(waitErrors, fmt.Sprintf("%s: %v", label, err))
				waitMu.Unlock()
			}
		}(lbl, cmd)
	}
	wg.Wait()
	end := time.Now()

	gt := capmodel.GroundTruth{
		StartedAt:    start,
		EndedAt:      end,
		TargetTPS:    tps,
		ListenerAddr: target,
	}

	host, remotePort := splitAddr(target)
	ports := map[int]struct{}{}
	var resultErrors []string
	for i, resultPath := range resultPaths {
		var wr workerResult
		if err := control.ReadJSON(resultPath, &wr); err != nil {
			resultErrors = append(resultErrors, fmt.Sprintf("worker-%d result: %v", i, err))
			continue
		}
		if wr.LocalPort == 0 || wr.OK == 0 {
			resultErrors = append(resultErrors,
				fmt.Sprintf("%s produced no usable remote connection (port=%d ok=%d failed=%d)",
					wr.Label, wr.LocalPort, wr.OK, wr.Failed))
			continue
		}
		ports[wr.LocalPort] = struct{}{}
		gt.Controls = append(gt.Controls, capmodel.ControlProcess{
			PID:          wr.PID,
			Label:        wr.Label,
			Image:        self,
			LocalPort:    wr.LocalPort,
			RemoteHost:   host,
			RemotePort:   remotePort,
			GoesViaProxy: !wr.Bypass,
			Transactions: wr.OK,
		})
		gt.TotalTxAttempted += wr.Attempted
		gt.TotalTxOK += wr.OK
	}
	if len(waitErrors) > 0 || len(resultErrors) > 0 || len(gt.Controls) != workers {
		allErrors := append(waitErrors, resultErrors...)
		return fmt.Errorf("remote load validation failed: %s", strings.Join(allErrors, "; "))
	}

	gt.TotalConnections = len(ports)
	secs := end.Sub(start).Seconds()
	if secs > 0 {
		gt.AchievedTPS = float64(gt.TotalTxOK) / secs
	}

	if err := control.WriteJSON(out, gt); err != nil {
		return err
	}
	fmt.Printf("wrote %s: %d control processes, %d connections, achieved %.0f tps\n",
		out, len(gt.Controls), gt.TotalConnections, gt.AchievedTPS)
	return nil
}

func validateTarget(target string) error {
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return fmt.Errorf("invalid -target %q (want host:port): %w", target, err)
	}
	port, err := strconv.Atoi(portText)
	if strings.TrimSpace(host) == "" || err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid -target %q (want host:port with port 1-65535)", target)
	}
	return nil
}

func transactionHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/tx", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "ok")
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "ok")
	})
	return mux
}

func runServer(listen string) error {
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	addr := ln.Addr().String()
	fmt.Printf("loadgen server listening on %s; press Ctrl+C to stop\n", addr)
	return (&http.Server{Handler: transactionHandler()}).Serve(ln)
}

func splitAddr(addr string) (string, int) {
	host, p, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 0
	}
	port, _ := strconv.Atoi(p)
	return host, port
}

type prefixWriter struct {
	prefix string
	w      io.Writer
}

func (p prefixWriter) Write(b []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		fmt.Fprintf(p.w, "[%s] %s\n", p.prefix, line)
	}
	return len(b), nil
}

// -------- worker --------

func runWorker(label, target string, bypass bool, tps int, dur time.Duration) workerResult {
	result := workerResult{
		PID:    os.Getpid(),
		Label:  label,
		Bypass: bypass,
	}
	// One persistent connection => one stable local port for the whole run.
	// We deliberately do NOT honor any system proxy (Transport with no Proxy),
	// which is what makes the bypass worker a genuine CAP-4 target.
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	tr := &http.Transport{
		Proxy:               nil, // ignore system proxy — kernel probes must still see us
		MaxIdleConns:        1,
		MaxIdleConnsPerHost: 1,
		DisableKeepAlives:   false,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := dialer.DialContext(ctx, network, addr)
			if err == nil {
				if _, p, e := net.SplitHostPort(c.LocalAddr().String()); e == nil {
					result.LocalPort, _ = strconv.Atoi(p)
				}
			}
			return c, err
		},
	}
	client := &http.Client{Transport: tr, Timeout: 3 * time.Second}

	interval := time.Second / time.Duration(max(tps, 1))
	deadline := time.Now().Add(dur)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	send := func() {
		result.Attempted++
		req, _ := http.NewRequest("GET", "http://"+target+"/tx", nil)
		req.Header.Set("X-Label", label)
		req.Header.Set("X-Pid", strconv.Itoa(result.PID))
		req.Header.Set("X-Local-Port", strconv.Itoa(result.LocalPort))
		if bypass {
			req.Header.Set("X-Bypass", "1")
		}
		resp, err := client.Do(req)
		if err != nil {
			result.Failed++
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			result.Failed++
			return
		}
		result.OK++
	}
	// prime one request so the connection (and localPort) exists before load
	send()
	for time.Now().Before(deadline) {
		<-ticker.C
		send()
	}
	fmt.Fprintf(os.Stderr, "worker %s pid=%d port=%d ok=%d fail=%d bypass=%v\n",
		label, result.PID, result.LocalPort, result.OK, result.Failed, bypass)
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
