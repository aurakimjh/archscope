// Package summary projects one immutable store snapshot without mixing timing
// observation points or inventing values for unobserved bytes/phases.
package summary

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"math"
	"time"
)

type CountedBytes struct {
	Bytes   int64 `json:"bytes"`
	Unknown int   `json:"unknown"`
}
type Phase struct {
	Perspective   string  `json:"perspective"`
	Name          string  `json:"name"`
	MS            float64 `json:"ms"`
	Known         int     `json:"known"`
	Unknown       int     `json:"unknown"`
	NotApplicable int     `json:"notApplicable"`
}
type Snapshot struct {
	Total              int            `json:"total"`
	Completed          int            `json:"completed"`
	Pending            int            `json:"pending"`
	Sent               CountedBytes   `json:"sent"`
	Received           CountedBytes   `json:"received"`
	Decoded            CountedBytes   `json:"decoded"`
	CompressionSaved   int64          `json:"compressionSaved"`
	CompressionKnown   int            `json:"compressionKnown"`
	Connections        int            `json:"connections"`
	ConnectionsUnknown int            `json:"connectionsUnknown"`
	DNS                int            `json:"dns"`
	DNSUnknown         int            `json:"dnsUnknown"`
	ElapsedMS          float64        `json:"elapsedMs"`
	TimeKnown          bool           `json:"timeKnown"`
	StatusCounts       map[string]int `json:"statusCounts"`
	Phases             []Phase        `json:"phases"`
}

func Build(rows []models.CaptureTransaction) Snapshot {
	out := Snapshot{Total: len(rows), StatusCounts: map[string]int{}, Phases: []Phase{}}
	connections := map[string]bool{}
	dns := map[string]bool{}
	phases := map[string]*Phase{}
	var first, last time.Time
	addBytes := func(sum *CountedBytes, value int64) {
		if value < 0 {
			sum.Unknown++
		} else {
			sum.Bytes += value
		}
	}
	for _, tx := range rows {
		if tx.State == models.TxComplete || tx.State == models.TxFailed || tx.State == models.TxAborted {
			out.Completed++
		} else {
			out.Pending++
		}
		addBytes(&out.Sent, tx.Request.BodySize)
		addBytes(&out.Received, tx.Response.BodySize)
		addBytes(&out.Decoded, tx.Response.BodyDecoded)
		if tx.Response.BodyDecoded >= 0 && tx.Response.BodySize >= 0 {
			out.CompressionKnown++
			out.CompressionSaved += tx.Response.BodyDecoded - tx.Response.BodySize
		}
		if tx.ConnectionID == "" {
			out.ConnectionsUnknown++
		} else {
			connections[tx.ConnectionID] = true
		}
		group := "other"
		if tx.StatusCode >= 200 && tx.StatusCode < 600 {
			group = string(rune('0'+tx.StatusCode/100)) + "xx"
		}
		out.StatusCounts[group]++
		start, e := time.Parse(time.RFC3339Nano, tx.StartedAt)
		if e == nil {
			end, e := time.Parse(time.RFC3339Nano, tx.EndedAt)
			if e != nil && tx.TotalMS >= 0 && !math.IsNaN(tx.TotalMS) && !math.IsInf(tx.TotalMS, 0) {
				end = start.Add(time.Duration(tx.TotalMS * float64(time.Millisecond)))
			}
			if first.IsZero() || start.Before(first) {
				first = start
			}
			if end.After(last) {
				last = end
			}
		}
		upstream := tx.Timings.ProxyUpstream
		if upstream == nil {
			upstream = tx.Timings.ImportedHAR
		}
		if upstream == nil || upstream.DNS.State == models.TimingUnknown || upstream.DNS.State == "" {
			out.DNSUnknown++
		} else if upstream.DNS.State == models.TimingKnown && upstream.DNS.MS > 0 {
			key := tx.ConnectionID
			if key == "" {
				key = tx.ID
			}
			dns[key] = true
		}
		for _, entry := range []struct {
			name string
			p    *models.TimingPhases
		}{{"clientProxy", tx.Timings.ClientProxy}, {"proxyInternal", tx.Timings.ProxyInternal}, {"proxyUpstream", tx.Timings.ProxyUpstream}, {"importedHar", tx.Timings.ImportedHAR}} {
			for _, field := range []string{"blocked", "dns", "connect", "tls", "send", "wait", "receive"} {
				key := entry.name + "/" + field
				phase := phases[key]
				if phase == nil {
					phase = &Phase{Perspective: entry.name, Name: field}
					phases[key] = phase
				}
				v := models.UnknownDuration()
				if entry.p != nil {
					switch field {
					case "blocked":
						v = entry.p.Blocked
					case "dns":
						v = entry.p.DNS
					case "connect":
						v = entry.p.Connect
					case "tls":
						v = entry.p.TLS
					case "send":
						v = entry.p.Send
					case "wait":
						v = entry.p.Wait
					case "receive":
						v = entry.p.Receive
					}
				}
				switch v.State {
				case models.TimingNotApplicable:
					phase.NotApplicable++
				case models.TimingKnown:
					if v.MS >= 0 && !math.IsNaN(v.MS) && !math.IsInf(v.MS, 0) {
						phase.MS += v.MS
						phase.Known++
					} else {
						phase.Unknown++
					}
				default:
					phase.Unknown++
				}
			}
		}
	}
	out.Connections = len(connections)
	out.DNS = len(dns)
	if !first.IsZero() && !last.Before(first) {
		out.TimeKnown = true
		out.ElapsedMS = float64(last.Sub(first).Microseconds()) / 1000
	}
	for _, p := range []string{"clientProxy", "proxyInternal", "proxyUpstream", "importedHar"} {
		for _, name := range []string{"blocked", "dns", "connect", "tls", "send", "wait", "receive"} {
			if phase := phases[p+"/"+name]; phase != nil {
				out.Phases = append(out.Phases, *phase)
			}
		}
	}
	return out
}
