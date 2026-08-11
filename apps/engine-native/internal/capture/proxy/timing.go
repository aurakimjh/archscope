package proxy

import (
	"context"
	"crypto/tls"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// upstreamTrace records the wall-clock boundaries of one proxy→upstream
// exchange so the UI can draw a phase waterfall instead of a single total.
//
// Perspective (SYSTEM_HTTP_CAPTURE "Timings"): a MITM proxy cannot observe the
// original client's DNS/TCP/TLS handshakes, so everything measured here belongs
// to `TimingSet.ProxyUpstream` and must never be presented as the client's own
// network time. The client-visible leg the proxy *can* measure — the time from
// accepting the request to finishing the response write — is reported
// separately as `TimingSet.ProxyInternal`.
//
// A phase whose hook never fired stays `unknown` rather than collapsing to
// `0 ms`; a reused connection reports DNS/Connect/TLS as `not_applicable`
// (invariant INV-5), which is a different claim from "took no time".
type upstreamTrace struct {
	mu sync.Mutex

	getConn      time.Time
	dnsStart     time.Time
	dnsDone      time.Time
	connectStart time.Time
	connectDone  time.Time
	tlsStart     time.Time
	tlsDone      time.Time
	gotConn      time.Time
	wroteRequest time.Time
	firstByte    time.Time

	reused bool
}

// newUpstreamTrace returns the trace recorder and a context carrying the
// httptrace hooks. Callers pass the returned context to the outbound request.
func newUpstreamTrace(ctx context.Context) (*upstreamTrace, context.Context) {
	trace := &upstreamTrace{}
	return trace, httptrace.WithClientTrace(ctx, trace.hooks())
}

func (t *upstreamTrace) hooks() *httptrace.ClientTrace {
	stamp := func(target *time.Time) {
		t.mu.Lock()
		defer t.mu.Unlock()
		if target.IsZero() {
			*target = time.Now()
		}
	}
	return &httptrace.ClientTrace{
		GetConn:              func(string) { stamp(&t.getConn) },
		DNSStart:             func(httptrace.DNSStartInfo) { stamp(&t.dnsStart) },
		DNSDone:              func(httptrace.DNSDoneInfo) { stamp(&t.dnsDone) },
		ConnectStart:         func(string, string) { stamp(&t.connectStart) },
		ConnectDone:          func(string, string, error) { stamp(&t.connectDone) },
		TLSHandshakeStart:    func() { stamp(&t.tlsStart) },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { stamp(&t.tlsDone) },
		WroteRequest:         func(httptrace.WroteRequestInfo) { stamp(&t.wroteRequest) },
		GotFirstResponseByte: func() { stamp(&t.firstByte) },
		GotConn: func(info httptrace.GotConnInfo) {
			stamp(&t.gotConn)
			t.mu.Lock()
			defer t.mu.Unlock()
			t.reused = info.Reused
		},
	}
}

// Reused reports whether the transport served this exchange on a pooled
// connection. It drives `CaptureTransaction.UsedExistingConnection`.
func (t *upstreamTrace) Reused() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.reused
}

// Phases folds the recorded instants into the canonical phase set. `ended` is
// when the last response byte was read, so `receive` covers the full body and
// not just the headers RoundTrip returned on.
func (t *upstreamTrace) Phases(ended time.Time) models.TimingPhases {
	t.mu.Lock()
	defer t.mu.Unlock()

	phases := models.TimingPhases{
		Blocked: models.UnknownDuration(),
		DNS:     models.UnknownDuration(),
		Connect: models.UnknownDuration(),
		TLS:     models.UnknownDuration(),
		Send:    models.UnknownDuration(),
		Wait:    models.UnknownDuration(),
		Receive: models.UnknownDuration(),
	}

	// A reused connection did not resolve, dial, or handshake for this
	// exchange. Reporting 0 ms would let a waterfall imply the work happened
	// instantly; `not_applicable` states that it did not happen at all.
	if t.reused {
		phases.DNS = models.NotApplicableDuration()
		phases.Connect = models.NotApplicableDuration()
		phases.TLS = models.NotApplicableDuration()
	} else {
		phases.DNS = span(t.dnsStart, t.dnsDone)
		phases.Connect = span(t.connectStart, t.connectDone)
		phases.TLS = span(t.tlsStart, t.tlsDone)
	}

	if !t.getConn.IsZero() {
		// Time spent before the transport started any observable work for this
		// exchange. On a reused connection this is GetConn→GotConn, the actual
		// pool-acquisition interval. GotConnInfo.IdleTime must not be used here:
		// it is how long the connection sat idle before reuse, not how long this
		// request waited for it.
		phases.Blocked = span(t.getConn, firstNonZero(t.dnsStart, t.connectStart, t.tlsStart, t.gotConn))
	}

	// `send` is measured from the moment a connection was available, so it
	// excludes handshake time already attributed above.
	phases.Send = span(firstNonZero(t.gotConn, t.tlsDone, t.connectDone), t.wroteRequest)
	// `wait` is TTFB for this leg only — request fully written to first
	// response byte. It is not the client-observed TTFB.
	phases.Wait = span(t.wroteRequest, t.firstByte)
	phases.Receive = span(t.firstByte, ended)
	return phases
}

// observed reports whether any transport hook fired. A stubbed dialer or a
// failure before the round trip leaves every phase unknown, and an all-unknown
// TimingSet is omitted rather than attached as an empty waterfall.
func (t *upstreamTrace) observed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.getConn.IsZero() || !t.gotConn.IsZero() || !t.firstByte.IsZero()
}

// proxyInternalPhases describes the leg the proxy observes directly: it holds
// the client connection for `ended - started`, and the upstream wait inside
// that window is what the client experiences as this proxy's latency. Only the
// phases the proxy can actually attribute are reported; the client's own DNS,
// TCP, and TLS to the proxy are `not_applicable` at this observation point.
func proxyInternalPhases(started, ended time.Time, upstream *upstreamTrace) models.TimingPhases {
	phases := models.TimingPhases{
		Blocked: models.UnknownDuration(),
		DNS:     models.NotApplicableDuration(),
		Connect: models.NotApplicableDuration(),
		TLS:     models.NotApplicableDuration(),
		Send:    models.UnknownDuration(),
		Wait:    models.UnknownDuration(),
		Receive: models.UnknownDuration(),
	}
	if started.IsZero() || ended.IsZero() || !ended.After(started) {
		return phases
	}
	if upstream == nil {
		return phases
	}
	upstream.mu.Lock()
	getConn, firstByte := upstream.getConn, upstream.firstByte
	upstream.mu.Unlock()

	phases.Blocked = span(started, getConn)
	phases.Wait = span(getConn, firstByte)
	phases.Receive = span(firstByte, ended)
	return phases
}

func span(from, to time.Time) models.Duration {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return models.UnknownDuration()
	}
	return models.KnownDuration(millis(to.Sub(from)))
}

func millis(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

func firstNonZero(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}
