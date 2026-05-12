package jenniferprofile

import (
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	customRuleSourceProfileApplication = "profile_application"
	customRuleSourceMethod             = "method"
	customRuleSourceExternalCallURL    = "external_call_url"
	customRuleSampleLimit              = 20
)

func customRuleStats(files []jenniferFileBucket, rules []models.JenniferCustomAnalysisRule) []map[string]any {
	normalized := normalizeCustomRules(rules)
	if len(normalized) == 0 {
		return []map[string]any{}
	}

	stats := make([]customRuleAccumulator, len(normalized))
	for i, rule := range normalized {
		stats[i] = customRuleAccumulator{
			rule:    rule,
			txidSet: map[string]struct{}{},
			seenSet: map[string]struct{}{},
		}
	}

	for _, file := range files {
		for _, profile := range file.profiles {
			for i := range stats {
				applyCustomRule(&stats[i], profile)
			}
		}
	}

	rows := make([]map[string]any, 0, len(stats))
	for _, stat := range stats {
		avg := 0.0
		if stat.count > 0 {
			avg = float64(stat.totalMs) / float64(stat.count)
		}
		rows = append(rows, map[string]any{
			"id":              stat.rule.ID,
			"label":           stat.rule.Label,
			"group":           stat.rule.Group,
			"source":          stat.rule.Source,
			"patterns":        stat.rule.Patterns,
			"count":           stat.count,
			"total_ms":        stat.totalMs,
			"avg_ms":          avg,
			"max_ms":          stat.maxMs,
			"matched_txids":   stat.txids,
			"matched_samples": stat.samples,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		gi := strings.TrimSpace(rows[i]["group"].(string))
		gj := strings.TrimSpace(rows[j]["group"].(string))
		if gi != gj {
			return gi < gj
		}
		ti := rows[i]["total_ms"].(int)
		tj := rows[j]["total_ms"].(int)
		if ti != tj {
			return ti > tj
		}
		return rows[i]["label"].(string) < rows[j]["label"].(string)
	})
	return rows
}

type customRuleAccumulator struct {
	rule    models.JenniferCustomAnalysisRule
	count   int
	totalMs int
	maxMs   int
	txidSet map[string]struct{}
	seenSet map[string]struct{}
	txids   []string
	samples []string
}

func normalizeCustomRules(rules []models.JenniferCustomAnalysisRule) []models.JenniferCustomAnalysisRule {
	out := make([]models.JenniferCustomAnalysisRule, 0, len(rules))
	for _, rule := range rules {
		rule.ID = strings.TrimSpace(rule.ID)
		rule.Label = strings.TrimSpace(rule.Label)
		rule.Group = strings.ToLower(strings.TrimSpace(rule.Group))
		rule.Source = normalizeCustomRuleSource(rule.Source)
		patterns := make([]string, 0, len(rule.Patterns))
		seen := map[string]struct{}{}
		for _, pattern := range rule.Patterns {
			pattern = strings.ToLower(strings.TrimSpace(pattern))
			if pattern == "" {
				continue
			}
			if _, ok := seen[pattern]; ok {
				continue
			}
			seen[pattern] = struct{}{}
			patterns = append(patterns, pattern)
		}
		if rule.Label == "" || rule.Source == "" || len(patterns) == 0 {
			continue
		}
		rule.Patterns = patterns
		out = append(out, rule)
	}
	return out
}

func normalizeCustomRuleSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case customRuleSourceProfileApplication, "profile", "application", "url":
		return customRuleSourceProfileApplication
	case customRuleSourceMethod, "event", "method_name":
		return customRuleSourceMethod
	case customRuleSourceExternalCallURL, "external", "external_call", "external-call":
		return customRuleSourceExternalCallURL
	default:
		return ""
	}
}

func applyCustomRule(stat *customRuleAccumulator, profile models.JenniferTransactionProfile) {
	switch stat.rule.Source {
	case customRuleSourceProfileApplication:
		haystack := strings.ToLower(profile.Header.Application)
		if customRuleMatchAny(haystack, stat.rule.Patterns) {
			elapsed := 0
			if profile.Header.ResponseTimeMs != nil {
				elapsed = *profile.Header.ResponseTimeMs
			}
			stat.add(profile.Header.TXID, profile.Header.Application, elapsed)
		}
	case customRuleSourceMethod:
		for _, ev := range profile.Body.Events {
			if ev.ElapsedMs == nil {
				continue
			}
			if ev.EventType == models.JenniferEventStart ||
				ev.EventType == models.JenniferEventEnd ||
				ev.EventType == models.JenniferEventTotal {
				continue
			}
			haystack := strings.ToLower(ev.RawMessage + "\n" + strings.Join(ev.DetailLines, "\n"))
			if customRuleMatchAny(haystack, stat.rule.Patterns) {
				stat.add(profile.Header.TXID, ev.RawMessage, *ev.ElapsedMs)
			}
		}
	case customRuleSourceExternalCallURL:
		for _, ev := range profile.Body.Events {
			if ev.EventType != models.JenniferEventExternalCall || ev.ElapsedMs == nil {
				continue
			}
			haystack := strings.ToLower(ev.ExternalURL + "\n" + ev.RawMessage)
			if customRuleMatchAny(haystack, stat.rule.Patterns) {
				sample := ev.ExternalURL
				if sample == "" {
					sample = ev.RawMessage
				}
				stat.add(profile.Header.TXID, sample, *ev.ElapsedMs)
			}
		}
	}
}

func (s *customRuleAccumulator) add(txid string, sample string, elapsedMs int) {
	s.count++
	s.totalMs += elapsedMs
	if elapsedMs > s.maxMs {
		s.maxMs = elapsedMs
	}
	if txid != "" {
		if _, ok := s.txidSet[txid]; !ok {
			s.txidSet[txid] = struct{}{}
			if len(s.txids) < customRuleSampleLimit {
				s.txids = append(s.txids, txid)
			}
		}
	}
	sample = strings.TrimSpace(sample)
	if sample == "" {
		return
	}
	if _, ok := s.seenSet[sample]; ok {
		return
	}
	s.seenSet[sample] = struct{}{}
	if len(s.samples) < customRuleSampleLimit {
		s.samples = append(s.samples, sample)
	}
}

func customRuleMatchAny(haystack string, patterns []string) bool {
	if haystack == "" {
		return false
	}
	for _, pattern := range patterns {
		if pattern != "" && strings.Contains(haystack, pattern) {
			return true
		}
	}
	return false
}
