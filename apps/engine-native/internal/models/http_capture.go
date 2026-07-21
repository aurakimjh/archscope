package models

// CaptureSchemaVersion versions the offline HAR transaction contract. It is
// independent from the AnalysisResult envelope version.
const CaptureSchemaVersion = 1

type TimingState string

const (
	TimingKnown         TimingState = "known"
	TimingNotApplicable TimingState = "not_applicable"
	TimingUnknown       TimingState = "unknown"
)

type Duration struct {
	MS    float64     `json:"ms"`
	State TimingState `json:"state"`
}

type TimingPhases struct {
	Blocked Duration `json:"blocked"`
	DNS     Duration `json:"dns"`
	Connect Duration `json:"connect"`
	TLS     Duration `json:"tls"`
	Send    Duration `json:"send"`
	Wait    Duration `json:"wait"`
	Receive Duration `json:"receive"`
}

type TimingSet struct {
	ClientProxy   *TimingPhases `json:"clientProxy,omitempty"`
	ProxyInternal *TimingPhases `json:"proxyInternal,omitempty"`
	ProxyUpstream *TimingPhases `json:"proxyUpstream,omitempty"`
	ImportedHAR   *TimingPhases `json:"importedHar,omitempty"`
}

type ProcessKey struct {
	PID       int32  `json:"pid"`
	StartTime string `json:"startTime"`
}

type ProcessInstance struct {
	Key         ProcessKey `json:"key"`
	Name        string     `json:"name"`
	ExecPath    string     `json:"execPath,omitempty"`
	CommandLine string     `json:"commandLine,omitempty"`
	User        string     `json:"user,omitempty"`
	ParentPID   int32      `json:"parentPid,omitempty"`
	Attribution string     `json:"attribution"`
}

type HeaderField struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Redacted bool   `json:"redacted,omitempty"`
}

type HTTPMessage struct {
	Headers      []HeaderField `json:"headers"`
	Cookies      []HeaderField `json:"cookies"`
	HeaderSize   int64         `json:"headerSize"`
	BodySize     int64         `json:"bodySize"`
	BodyDecoded  int64         `json:"bodyDecoded"`
	TransferSize int64         `json:"transferSize"`
	BodyEncoding string        `json:"bodyEncoding,omitempty"`
	ContentType  string        `json:"contentType,omitempty"`
	BodyStorage  string        `json:"bodyStorage"`
	BodyRef      string        `json:"bodyRef,omitempty"`
	BodyPreview  string        `json:"bodyPreview,omitempty"`
	Redacted     bool          `json:"redacted,omitempty"`
}

type TxState string

const (
	TxRequestSent TxState = "request_sent"
	TxReceiving   TxState = "receiving"
	TxComplete    TxState = "complete"
	TxFailed      TxState = "failed"
	TxAborted     TxState = "aborted"
)

// CaptureTransaction is the common normalized transaction consumed by both
// offline HAR analysis and the later live-capture pipeline.
type CaptureTransaction struct {
	ID           string `json:"id"`
	ConnectionID string `json:"connectionId"`
	Sequence     int    `json:"sequence"`
	StreamID     int    `json:"streamId,omitempty"`

	Method      string `json:"method"`
	URL         string `json:"url"`
	Scheme      string `json:"scheme"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	Query       string `json:"query,omitempty"`
	HTTPVersion string `json:"httpVersion"`

	StatusCode int    `json:"statusCode"`
	StatusText string `json:"statusText,omitempty"`

	Request  HTTPMessage `json:"request"`
	Response HTTPMessage `json:"response"`
	Timings  TimingSet   `json:"timings"`

	UsedExistingConnection bool    `json:"usedExistingConnection"`
	StartedAt              string  `json:"startedAt,omitempty"`
	EndedAt                string  `json:"endedAt,omitempty"`
	State                  TxState `json:"state"`
	TotalMS                float64 `json:"totalMs"`

	CaptureMode      string           `json:"captureMode"`
	ObservationPoint string           `json:"observationPoint"`
	Coverage         string           `json:"coverage"`
	Fidelity         string           `json:"fidelity"`
	Process          *ProcessInstance `json:"process,omitempty"`
	Error            string           `json:"error,omitempty"`
}

func UnknownDuration() Duration {
	return Duration{State: TimingUnknown}
}

func NotApplicableDuration() Duration {
	return Duration{State: TimingNotApplicable}
}

func KnownDuration(ms float64) Duration {
	return Duration{MS: ms, State: TimingKnown}
}

func (p TimingPhases) KnownSumMS() float64 {
	values := [...]Duration{p.Blocked, p.DNS, p.Connect, p.TLS, p.Send, p.Wait, p.Receive}
	var total float64
	for _, value := range values {
		if value.State == TimingKnown {
			total += value.MS
		}
	}
	return total
}
