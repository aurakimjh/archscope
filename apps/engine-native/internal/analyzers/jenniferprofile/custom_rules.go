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

type customBreakdownAccumulator struct {
	customRuleAccumulator
	sourceBuckets map[string]int
}

func normalizeCustomRules(rules []models.JenniferCustomAnalysisRule) []models.JenniferCustomAnalysisRule {
	out := make([]models.JenniferCustomAnalysisRule, 0, len(rules))
	indexByKey := map[customRuleKey]int{}
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
		key := customRuleKey{
			label:  strings.ToLower(rule.Label),
			group:  rule.Group,
			source: rule.Source,
		}
		if idx, ok := indexByKey[key]; ok {
			out[idx].Patterns = mergeCustomRulePatterns(out[idx].Patterns, rule.Patterns)
			if out[idx].ID == "" {
				out[idx].ID = rule.ID
			}
			continue
		}
		indexByKey[key] = len(out)
		out = append(out, rule)
	}
	return out
}

type customRuleKey struct {
	label  string
	group  string
	source string
}

func mergeCustomRulePatterns(existing []string, incoming []string) []string {
	seen := map[string]struct{}{}
	for _, pattern := range existing {
		seen[pattern] = struct{}{}
	}
	for _, pattern := range incoming {
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		existing = append(existing, pattern)
	}
	return existing
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

func applyCustomBreakdownRules(
	bd *models.JenniferResponseTimeBreakdown,
	group *guidGroupBucket,
	edges []models.JenniferExternalCallEdge,
	rules []models.JenniferCustomAnalysisRule,
) {
	accs := customRuleBreakdownAccumulators(group, edges, rules)
	if len(accs) == 0 {
		recomputeBreakdownRatios(bd)
		return
	}
	for i := range accs {
		for bucket, value := range accs[i].sourceBuckets {
			subtractBreakdownBucket(bd, bucket, value)
		}
	}
	slices := make([]models.JenniferResponseTimeCustomSlice, 0, len(accs))
	for _, acc := range accs {
		if acc.totalMs <= 0 {
			continue
		}
		slices = append(slices, models.JenniferResponseTimeCustomSlice{
			ID:             acc.rule.ID,
			Label:          acc.rule.Label,
			Group:          acc.rule.Group,
			Source:         acc.rule.Source,
			ValueMs:        acc.totalMs,
			Count:          acc.count,
			MatchedTXIDs:   acc.txids,
			MatchedSamples: acc.samples,
		})
	}
	bd.CustomSlices = slices
	recomputeBreakdownRatios(bd)
}

func customRuleBreakdownAccumulators(
	group *guidGroupBucket,
	edges []models.JenniferExternalCallEdge,
	rules []models.JenniferCustomAnalysisRule,
) []customBreakdownAccumulator {
	normalized := normalizeCustomRules(rules)
	if len(normalized) == 0 {
		return nil
	}
	stats := make([]customBreakdownAccumulator, len(normalized))
	for i, rule := range normalized {
		stats[i] = customBreakdownAccumulator{
			customRuleAccumulator: customRuleAccumulator{
				rule:    rule,
				txidSet: map[string]struct{}{},
				seenSet: map[string]struct{}{},
			},
			sourceBuckets: map[string]int{},
		}
	}
	for _, profile := range group.profiles {
		for i := range stats {
			applyCustomBreakdownProfileRule(&stats[i], profile)
		}
	}
	for _, edge := range edges {
		for i := range stats {
			applyCustomBreakdownExternalRule(&stats[i], edge)
		}
	}
	out := stats[:0]
	for _, stat := range stats {
		if stat.totalMs > 0 {
			out = append(out, stat)
		}
	}
	return out
}

func applyCustomBreakdownProfileRule(stat *customBreakdownAccumulator, profile models.JenniferTransactionProfile) {
	switch stat.rule.Source {
	case customRuleSourceProfileApplication:
		haystack := strings.ToLower(profile.Header.Application)
		if customRuleMatchAny(haystack, stat.rule.Patterns) {
			elapsed := 0
			if profile.Header.ResponseTimeMs != nil {
				elapsed = *profile.Header.ResponseTimeMs
			}
			stat.addWithBucket(profile.Header.TXID, profile.Header.Application, elapsed, "method_time_ms")
		}
	case customRuleSourceMethod:
		matches := customRuleMethodMatches(profile.Body.Events, stat.rule.Patterns)
		matchedMethodIndexes := customRuleMatchedMethodIndexes(matches)
		for _, match := range matches {
			elapsed := customRuleMethodElapsed(profile.Body.Events, match.index, match.bucket, matchedMethodIndexes)
			stat.addWithBucket(profile.Header.TXID, match.sample, elapsed, match.bucket)
		}
	}
}

func applyCustomBreakdownExternalRule(stat *customBreakdownAccumulator, edge models.JenniferExternalCallEdge) {
	if stat.rule.Source != customRuleSourceExternalCallURL {
		return
	}
	haystack := strings.ToLower(edge.ExternalCallURL)
	if !customRuleMatchAny(haystack, stat.rule.Patterns) {
		return
	}
	elapsed := edge.ExternalCallElapsedMs
	bucket := "unprofiled_external_call_ms"
	if edge.MatchStatus == models.JenniferMatchOK {
		bucket = "network_call_ms"
		if edge.AdjustedNetworkGapMs != nil {
			elapsed = *edge.AdjustedNetworkGapMs
		} else {
			elapsed = 0
		}
	}
	stat.addWithBucket(edge.CallerTXID, edge.ExternalCallURL, elapsed, bucket)
}

func (s *customBreakdownAccumulator) addWithBucket(txid string, sample string, elapsedMs int, bucket string) {
	if elapsedMs <= 0 {
		return
	}
	s.add(txid, sample, elapsedMs)
	if bucket != "" {
		s.sourceBuckets[bucket] += elapsedMs
	}
}

func breakdownBucketForEvent(t models.JenniferEventType) string {
	switch t {
	case models.JenniferEventSQLExecute, models.JenniferEventSQLUpdate, models.JenniferEventSQLQuery:
		return "sql_execute_ms"
	case models.JenniferEventCheckQuery:
		return "check_query_ms"
	case models.JenniferEventTwoPCStart, models.JenniferEventTwoPCEnd,
		models.JenniferEventTwoPCPrepare, models.JenniferEventTwoPCCommit,
		models.JenniferEventTwoPCRollback, models.JenniferEventTwoPCUnknown:
		return "two_pc_ms"
	case models.JenniferEventFetch:
		return "fetch_ms"
	case models.JenniferEventConnAcquire:
		return "connection_acquire_ms"
	case models.JenniferEventNetworkPrep:
		return "network_prep_ms"
	case models.JenniferEventServletDispatch:
		return "servlet_dispatch_ms"
	case models.JenniferEventExternalCall:
		return "unprofiled_external_call_ms"
	default:
		return "method_time_ms"
	}
}

type customRuleMethodMatch struct {
	index  int
	bucket string
	sample string
}

func customRuleMethodMatches(events []models.JenniferProfileEvent, patterns []string) []customRuleMethodMatch {
	out := []customRuleMethodMatch{}
	for i, ev := range events {
		if ev.ElapsedMs == nil {
			continue
		}
		if ev.EventType == models.JenniferEventStart ||
			ev.EventType == models.JenniferEventEnd ||
			ev.EventType == models.JenniferEventTotal {
			continue
		}
		haystack := strings.ToLower(ev.RawMessage + "\n" + strings.Join(ev.DetailLines, "\n"))
		if customRuleMatchAny(haystack, patterns) {
			out = append(out, customRuleMethodMatch{
				index:  i,
				bucket: breakdownBucketForEvent(ev.EventType),
				sample: ev.RawMessage,
			})
		}
	}
	return out
}

func customRuleMatchedMethodIndexes(matches []customRuleMethodMatch) []int {
	out := []int{}
	for _, match := range matches {
		if match.bucket == "method_time_ms" {
			out = append(out, match.index)
		}
	}
	return out
}

func customRuleMethodElapsed(events []models.JenniferProfileEvent, idx int, bucket string, matchedMethodIndexes []int) int {
	if idx < 0 || idx >= len(events) {
		return 0
	}
	ev := events[idx]
	if ev.ElapsedMs == nil {
		return 0
	}
	elapsed := *ev.ElapsedMs
	if bucket != "method_time_ms" {
		return elapsed
	}
	parent, ok := eventOffsetInterval(ev)
	if !ok {
		return elapsed
	}
	for _, otherIdx := range matchedMethodIndexes {
		if otherIdx == idx || otherIdx < 0 || otherIdx >= len(events) {
			continue
		}
		otherInterval, ok := eventOffsetInterval(events[otherIdx])
		if !ok {
			continue
		}
		if sameInterval(parent, otherInterval) && otherIdx < idx {
			return 0
		}
	}
	childIntervals := []interval{}
	for childIdx := range events {
		if childIdx == idx {
			continue
		}
		child := events[childIdx]
		if !customRuleDeductsChildEvent(child.EventType) {
			continue
		}
		childInterval, ok := eventOffsetInterval(child)
		if !ok {
			continue
		}
		if intervalContains(parent, childInterval) && !sameInterval(parent, childInterval) {
			childIntervals = append(childIntervals, childInterval)
		}
	}
	for _, childIdx := range matchedMethodIndexes {
		if childIdx == idx || childIdx < 0 || childIdx >= len(events) {
			continue
		}
		childInterval, ok := eventOffsetInterval(events[childIdx])
		if !ok {
			continue
		}
		if intervalContains(parent, childInterval) && !sameInterval(parent, childInterval) {
			childIntervals = append(childIntervals, childInterval)
		}
	}
	deduct := unionDuration(childIntervals)
	if deduct >= elapsed {
		return 0
	}
	return elapsed - deduct
}

func customRuleDeductsChildEvent(t models.JenniferEventType) bool {
	switch t {
	case models.JenniferEventSQLExecute, models.JenniferEventSQLUpdate, models.JenniferEventSQLQuery,
		models.JenniferEventCheckQuery,
		models.JenniferEventTwoPCStart, models.JenniferEventTwoPCEnd,
		models.JenniferEventTwoPCPrepare, models.JenniferEventTwoPCCommit,
		models.JenniferEventTwoPCRollback, models.JenniferEventTwoPCUnknown,
		models.JenniferEventFetch,
		models.JenniferEventConnAcquire,
		models.JenniferEventNetworkPrep,
		models.JenniferEventServletDispatch,
		models.JenniferEventExternalCall:
		return true
	default:
		return false
	}
}

func subtractBreakdownBucket(bd *models.JenniferResponseTimeBreakdown, bucket string, value int) {
	if value <= 0 {
		return
	}
	switch bucket {
	case "sql_execute_ms":
		subtractInt(&bd.SQLExecuteMs, value)
	case "check_query_ms":
		subtractInt(&bd.CheckQueryMs, value)
	case "two_pc_ms":
		subtractInt(&bd.TwoPCMs, value)
	case "fetch_ms":
		subtractInt(&bd.FetchMs, value)
	case "network_call_ms":
		subtractInt(&bd.NetworkCallMs, value)
	case "unprofiled_external_call_ms":
		subtractInt(&bd.UnprofiledExternalCallMs, value)
	case "network_prep_ms":
		subtractInt(&bd.NetworkPrepMs, value)
	case "servlet_dispatch_ms":
		subtractInt(&bd.ServletDispatchMs, value)
	case "connection_acquire_ms":
		subtractInt(&bd.ConnectionAcquireMs, value)
	case "method_time_ms":
		subtractInt(&bd.MethodTimeMs, value)
	}
}

func subtractInt(target *int, value int) {
	if value >= *target {
		*target = 0
		return
	}
	*target -= value
}

func recomputeBreakdownRatios(bd *models.JenniferResponseTimeBreakdown) {
	if bd.RootResponseTimeMs <= 0 {
		return
	}
	bd.MethodTimeRatio = float64(bd.MethodTimeMs) / float64(bd.RootResponseTimeMs)
	covered := bd.SQLExecuteMs + bd.CheckQueryMs + bd.TwoPCMs + bd.FetchMs +
		bd.NetworkCallMs + bd.UnprofiledExternalCallMs + bd.NetworkPrepMs +
		bd.ConnectionAcquireMs + bd.ServletDispatchMs
	for _, slice := range bd.CustomSlices {
		covered += slice.ValueMs
	}
	bd.Coverage = float64(covered) / float64(bd.RootResponseTimeMs)
	if covered > bd.RootResponseTimeMs {
		bd.NegativeMethodTime = true
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
		matches := customRuleMethodMatches(profile.Body.Events, stat.rule.Patterns)
		matchedMethodIndexes := customRuleMatchedMethodIndexes(matches)
		for _, match := range matches {
			stat.add(profile.Header.TXID, match.sample, customRuleMethodElapsed(profile.Body.Events, match.index, match.bucket, matchedMethodIndexes))
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
