//go:build windows

package procmap

import (
	"encoding/json"
	"math"
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"testing"
	"time"
)

const resolverPerformanceSchemaVersion = "archscope.resolver-cost.v1"

type resolverCostEvidence struct {
	SchemaVersion       string  `json:"schema_version"`
	OS                  string  `json:"os"`
	Architecture        string  `json:"architecture"`
	ConnectionScope     string  `json:"connection_scope"`
	OwnerTableReads     int     `json:"owner_table_reads_per_sample"`
	Samples             int     `json:"samples"`
	ConfirmedSamples    int     `json:"confirmed_samples"`
	MeanMilliseconds    float64 `json:"mean_ms"`
	P50Milliseconds     float64 `json:"p50_ms"`
	P95Milliseconds     float64 `json:"p95_ms"`
	MaximumMilliseconds float64 `json:"max_ms"`
	AcceptancePolicy    string  `json:"acceptance_policy"`
}

// TestWindowsResolverCostMeasurement measures the real GetExtendedTcpTable
// attribution path against one accepted loopback TCP connection. It is opt-in
// so the ordinary unit/race suite remains deterministic; R-RG1 CI enables it
// explicitly and archives the bounded, path-free JSON evidence.
//
// This test verifies attribution correctness and records cost. R-RG1 did not
// predeclare a product latency SLO, so the measurement does not retrofit one
// after observing the result.
func TestWindowsResolverCostMeasurement(t *testing.T) {
	if os.Getenv("ARCHSCOPE_RUN_RESOLVER_PERF") != "1" {
		t.Skip("set ARCHSCOPE_RUN_RESOLVER_PERF=1 for the R-RG1 native Windows measurement")
	}

	samples := resolverPerformanceSamples(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	acceptedResult := make(chan struct {
		connection net.Conn
		err        error
	}, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		acceptedResult <- struct {
			connection net.Conn
			err        error
		}{connection: connection, err: acceptErr}
	}()

	client, err := net.DialTimeout("tcp4", listener.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	accepted := <-acceptedResult
	if accepted.err != nil {
		t.Fatal(accepted.err)
	}
	defer accepted.connection.Close()

	clientAddress := accepted.connection.RemoteAddr()
	proxyAddress := accepted.connection.LocalAddr()
	resolver := Resolver{}
	if _, err := resolver.Resolve(clientAddress, proxyAddress); err != nil {
		t.Fatalf("warm-up attribution: %v", err)
	}

	durations := make([]time.Duration, 0, samples)
	confirmed := 0
	for sample := 0; sample < samples; sample++ {
		started := time.Now()
		process, resolveErr := resolver.Resolve(clientAddress, proxyAddress)
		durations = append(durations, time.Since(started))
		if resolveErr != nil {
			t.Fatalf("sample %d: %v", sample+1, resolveErr)
		}
		if process.Key.PID != int32(os.Getpid()) {
			t.Fatalf("sample %d attributed PID %d, want current process", sample+1, process.Key.PID)
		}
		if process.Attribution != "confirmed" {
			t.Fatalf("sample %d attribution=%q, want confirmed", sample+1, process.Attribution)
		}
		confirmed++
	}

	evidence := summarizeResolverCost(durations, confirmed)
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("R-RG1 resolver cost:\n%s", encoded)

	if output := os.Getenv("ARCHSCOPE_RESOLVER_PERF_OUT"); output != "" {
		encoded = append(encoded, '\n')
		if err := os.WriteFile(output, encoded, 0o600); err != nil {
			t.Fatalf("write resolver cost evidence: %v", err)
		}
	}
}

func resolverPerformanceSamples(t *testing.T) int {
	t.Helper()
	const defaultSamples = 50
	value := os.Getenv("ARCHSCOPE_RESOLVER_PERF_SAMPLES")
	if value == "" {
		return defaultSamples
	}
	samples, err := strconv.Atoi(value)
	if err != nil || samples < 10 || samples > 1000 {
		t.Fatalf("ARCHSCOPE_RESOLVER_PERF_SAMPLES=%q, want an integer in [10,1000]", value)
	}
	return samples
}

func summarizeResolverCost(durations []time.Duration, confirmed int) resolverCostEvidence {
	sorted := append([]time.Duration(nil), durations...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left] < sorted[right] })
	var total time.Duration
	for _, duration := range sorted {
		total += duration
	}
	return resolverCostEvidence{
		SchemaVersion:       resolverPerformanceSchemaVersion,
		OS:                  runtime.GOOS,
		Architecture:        runtime.GOARCH,
		ConnectionScope:     "accepted_loopback_tcp_connection",
		OwnerTableReads:     2,
		Samples:             len(sorted),
		ConfirmedSamples:    confirmed,
		MeanMilliseconds:    milliseconds(total / time.Duration(len(sorted))),
		P50Milliseconds:     milliseconds(percentileDuration(sorted, 0.50)),
		P95Milliseconds:     milliseconds(percentileDuration(sorted, 0.95)),
		MaximumMilliseconds: milliseconds(sorted[len(sorted)-1]),
		AcceptancePolicy:    "measurement_only_no_predeclared_product_slo",
	}
}

func percentileDuration(sorted []time.Duration, quantile float64) time.Duration {
	index := int(math.Ceil(quantile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	return sorted[index]
}

func milliseconds(duration time.Duration) float64 {
	return math.Round(float64(duration)/float64(time.Millisecond)*1000) / 1000
}
