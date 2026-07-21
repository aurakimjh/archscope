package httpcapture

import (
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type Dialect string

const (
	DialectChrome   Dialect = "chrome"
	DialectFirefox  Dialect = "firefox"
	DialectSafari   Dialect = "safari"
	DialectCharles  Dialect = "charles"
	DialectFiddler  Dialect = "fiddler"
	DialectProxyman Dialect = "proxyman"
	DialectInsomnia Dialect = "insomnia"
	DialectGeneric  Dialect = "generic"
)

type DialectFeatures struct {
	ID              Dialect
	CreatorMatch    []string
	SSLInConnect    bool
	TimeIncludesSSL bool
	BodySizeOnH2    bool
	EncodesBase64   bool
	ProvidesProcess bool
}

var dialects = []DialectFeatures{
	{ID: DialectChrome, CreatorMatch: []string{"webinspector", "chrome", "chromium"}, SSLInConnect: true},
	{ID: DialectFirefox, CreatorMatch: []string{"firefox"}, SSLInConnect: true, TimeIncludesSSL: true, EncodesBase64: true},
	{ID: DialectSafari, CreatorMatch: []string{"webkit web inspector", "safari"}, SSLInConnect: true},
	{ID: DialectCharles, CreatorMatch: []string{"charles"}, SSLInConnect: true},
	{ID: DialectFiddler, CreatorMatch: []string{"fiddler"}, SSLInConnect: true},
	{ID: DialectProxyman, CreatorMatch: []string{"proxyman"}, SSLInConnect: true},
	{ID: DialectInsomnia, CreatorMatch: []string{"insomnia"}},
}

func DetectDialect(creator, browser string) Dialect {
	value := strings.ToLower(strings.TrimSpace(creator + " " + browser))
	for _, features := range dialects {
		for _, marker := range features.CreatorMatch {
			if strings.Contains(value, marker) {
				return features.ID
			}
		}
	}
	return DialectGeneric
}

func FeaturesFor(dialect Dialect) DialectFeatures {
	for _, features := range dialects {
		if features.ID == dialect {
			return features
		}
	}
	return DialectFeatures{ID: DialectGeneric}
}

type timingSignals struct {
	missing  bool
	negative bool
}

// NormalizeTimings is the first-class dialect stage. It separates TLS from
// connect where the generating tool embeds it, preserves unknown/N/A states,
// and removes Firefox's duplicate SSL contribution from entry.time.
func NormalizeTimings(raw harTimings, rawTotal *float64, dialect Dialect, features DialectFeatures, reused bool) (models.TimingPhases, float64, timingSignals) {
	signals := timingSignals{}
	phase := models.TimingPhases{
		Blocked: durationFromHAR(raw.Blocked, &signals),
		DNS:     durationFromHAR(raw.DNS, &signals),
		Connect: durationFromHAR(raw.Connect, &signals),
		TLS:     durationFromHAR(raw.SSL, &signals),
		Send:    durationFromHAR(raw.Send, &signals),
		Wait:    durationFromHAR(raw.Wait, &signals),
		Receive: durationFromHAR(raw.Receive, &signals),
	}
	if features.SSLInConnect && phase.Connect.State == models.TimingKnown && phase.TLS.State == models.TimingKnown {
		phase.Connect.MS -= phase.TLS.MS
		if phase.Connect.MS < 0 {
			phase.Connect = models.UnknownDuration()
			signals.negative = true
		}
	}
	if reused {
		phase.DNS = models.NotApplicableDuration()
		phase.Connect = models.NotApplicableDuration()
		phase.TLS = models.NotApplicableDuration()
	}

	total := phase.KnownSumMS()
	if rawTotal != nil {
		if *rawTotal < 0 {
			signals.negative = true
		} else {
			total = *rawTotal
			if features.TimeIncludesSSL && raw.SSL != nil && *raw.SSL >= 0 {
				total -= *raw.SSL
			}
			if total < 0 {
				total = phase.KnownSumMS()
				signals.negative = true
			}
		}
	}
	return phase, total, signals
}

func durationFromHAR(value *float64, signals *timingSignals) models.Duration {
	if value == nil {
		signals.missing = true
		return models.UnknownDuration()
	}
	if *value == -1 {
		return models.NotApplicableDuration()
	}
	if *value < 0 {
		signals.negative = true
		return models.UnknownDuration()
	}
	return models.KnownDuration(*value)
}
