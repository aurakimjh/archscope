// Command loadgen produces the controlled traffic that the T-571 spike scores
// every capture candidate against (docs/ko/SYSTEM_HTTP_CAPTURE.md §10.4.2).
//
// It runs an in-process HTTP listener and spawns N worker child processes.
// Each worker holds ONE long-lived keep-alive connection (so it owns one
// stable local port for the whole run) and drives its share of the target
// transaction rate. Because loadgen knows every worker's PID and local port,
// the resulting ground_truth.json is the answer key the judge uses for CAP-1
// (attribution accuracy) and CAP-2 (false attribution).
//
// One worker is designated a "bypass" process (goes_via_proxy=false). A real
// proxy-based capture would miss it; a kernel-scope candidate must still see
// it. That worker is the CAP-4 target.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/capmodel"
	"github.com/aurakimjh/archscope/spikes/t571-windows-coverage/internal/control"
)

func main() {
	role := flag.String("role", "parent", "parent | worker")
	label := flag.String("label", "", "worker label (worker role)")
	target := flag.String("target", "", "listener addr host:port (worker role)")
	bypass := flag.Bool("bypass", false, "this worker is the proxy-bypass control (worker role)")
	tps := flag.Int("tps", 500, "total target transactions/sec (parent role)")
	workers := flag.Int("workers", 5, "number of worker processes (parent role)")
	dur := flag.Duration("duration", 30*time.Second, "run duration")
	listen := flag.String("listen", "127.0.0.1:0", "listener bind addr (parent role); empty local port => auto")
	out := flag.String("out", "results/ground_truth.json", "ground-truth output path (parent role)")
	flag.Parse()

	if *role == "worker" {
		runWorker(*label, *target, *bypass, *tps, *dur)
		return
	}
	if err := runParent(*listen, *out, *tps, *workers, *dur); err != nil {
		fmt.Fprintln(os.Stderr, "loadgen:", err)
		os.Exit(1)
	}
}

// -------- parent --------

type portInfo struct {
	label  string
	pid    int
	bypass bool
	tx     int64
}

func runParent(listen, out string, tps, workers int, dur time.Duration) error {
	if workers < 1 {
		return fmt.Errorf("need at least 1 worker")
	}

	var mu sync.Mutex
	byPort := map[int]*portInfo{}
	var totalTx int64

	mux := http.NewServeMux()
	mux.HandleFunc("/tx", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&totalTx, 1)
		lp, _ := strconv.Atoi(r.Header.Get("X-Local-Port"))
		pid, _ := strconv.Atoi(r.Header.Get("X-Pid"))
		lbl := r.Header.Get("X-Label")
		bp := r.Header.Get("X-Bypass") == "1"
		if lp == 0 {
			// fall back to the source port the stack reports
			if _, p, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				lp, _ = strconv.Atoi(p)
			}
		}
		mu.Lock()
		pi := byPort[lp]
		if pi == nil {
			pi = &portInfo{label: lbl, pid: pid, bypass: bp}
			byPort[lp] = pi
		}
		pi.tx++
		mu.Unlock()
		io.WriteString(w, "ok")
	})

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	addr := ln.Addr().String()
	fmt.Printf("loadgen listener on %s; spawning %d workers for %s at ~%d tps\n", addr, workers, dur, tps)

	self, err := os.Executable()
	if err != nil {
		return err
	}
	perWorkerTPS := tps / workers
	if perWorkerTPS < 1 {
		perWorkerTPS = 1
	}

	start := time.Now()
	var wg sync.WaitGroup
	children := make([]*exec.Cmd, 0, workers)
	for i := 0; i < workers; i++ {
		lbl := fmt.Sprintf("worker-%d", i)
		isBypass := i == workers-1 && workers > 1 // last worker is the CAP-4 bypass control
		args := []string{
			"-role", "worker",
			"-label", lbl,
			"-target", addr,
			"-tps", strconv.Itoa(perWorkerTPS),
			"-duration", dur.String(),
		}
		if isBypass {
			args = append(args, "-bypass")
		}
		cmd := exec.Command(self, args...)
		cmd.Stderr = prefixWriter{lbl, os.Stderr}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("spawn %s: %w", lbl, err)
		}
		children = append(children, cmd)
		wg.Add(1)
		go func(c *exec.Cmd) { defer wg.Done(); c.Wait() }(cmd)
	}
	wg.Wait()
	end := time.Now()

	// Give any in-flight /tx handlers a moment to record.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	gt := capmodel.GroundTruth{
		StartedAt:    start,
		EndedAt:      end,
		TargetTPS:    tps,
		ListenerAddr: addr,
	}
	// Merge per-port records into per-PID control processes (each worker owns
	// one persistent connection => one port, but fall back gracefully).
	byPID := map[int]*capmodel.ControlProcess{}
	var txTotal int
	for port, pi := range byPort {
		txTotal += int(pi.tx)
		cp := byPID[pi.pid]
		if cp == nil {
			host, rport := splitAddr(addr)
			cp = &capmodel.ControlProcess{
				PID:          pi.pid,
				Label:        pi.label,
				Image:        self,
				RemoteHost:   host,
				RemotePort:   rport,
				GoesViaProxy: !pi.bypass,
			}
			byPID[pi.pid] = cp
		}
		if cp.LocalPort == 0 {
			cp.LocalPort = port
		}
		cp.Transactions += int(pi.tx)
	}
	for _, cp := range byPID {
		gt.Controls = append(gt.Controls, *cp)
	}
	gt.TotalConnections = len(byPort)
	gt.TotalTxAttempted = int(atomic.LoadInt64(&totalTx))
	gt.TotalTxOK = txTotal
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

func runWorker(label, target string, bypass bool, tps int, dur time.Duration) {
	pid := os.Getpid()
	// One persistent connection => one stable local port for the whole run.
	// We deliberately do NOT honor any system proxy (Transport with no Proxy),
	// which is what makes the bypass worker a genuine CAP-4 target.
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	var localPort int
	tr := &http.Transport{
		Proxy:               nil, // ignore system proxy — kernel probes must still see us
		MaxIdleConns:        1,
		MaxIdleConnsPerHost: 1,
		DisableKeepAlives:   false,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := dialer.DialContext(ctx, network, addr)
			if err == nil {
				if _, p, e := net.SplitHostPort(c.LocalAddr().String()); e == nil {
					localPort, _ = strconv.Atoi(p)
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

	var ok, fail int
	send := func() {
		req, _ := http.NewRequest("GET", "http://"+target+"/tx", nil)
		req.Header.Set("X-Label", label)
		req.Header.Set("X-Pid", strconv.Itoa(pid))
		req.Header.Set("X-Local-Port", strconv.Itoa(localPort))
		if bypass {
			req.Header.Set("X-Bypass", "1")
		}
		resp, err := client.Do(req)
		if err != nil {
			fail++
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		ok++
	}
	// prime one request so the connection (and localPort) exists before load
	send()
	for time.Now().Before(deadline) {
		<-ticker.C
		send()
	}
	fmt.Fprintf(os.Stderr, "worker %s pid=%d port=%d ok=%d fail=%d bypass=%v\n",
		label, pid, localPort, ok, fail, bypass)
	_ = bufio.NewWriter(os.Stderr)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
