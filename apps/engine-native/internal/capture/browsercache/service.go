// Package browsercache manages explicit WinINet cache cleanup. It never reads
// cookie/header values or deletes browser profile files directly.
package browsercache

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Key, Kind string
	Size      uint64
	Modified  uint64
}
type Backend interface {
	Available() bool
	Entries() ([]Entry, error)
	Delete(Entry) error
}
type Capability struct {
	Available bool   `json:"available"`
	Provider  string `json:"provider"`
	Reason    string `json:"reason"`
}
type Item struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Kind string `json:"kind"`
	Size uint64 `json:"size"`
}
type Plan struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Scope     string    `json:"scope"`
	Count     int       `json:"count"`
	Items     []Item    `json:"items"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type Result struct {
	Deleted int `json:"deleted"`
	Changed int `json:"changed"`
	Failed  int `json:"failed"`
}
type planned struct {
	plan    Plan
	entries []Entry
	result  *Result
}
type Service struct {
	mu      sync.Mutex
	backend Backend
	plans   map[string]*planned
	order   []string
	now     func() time.Time
}

func New(backend Backend) *Service {
	if backend == nil {
		backend = systemBackend{}
	}
	return &Service{backend: backend, plans: map[string]*planned{}, now: time.Now}
}
func (s *Service) Capability() Capability {
	return Capability{s.backend.Available(), "WinINet / IE cache and persistent cookies", "Does not clear Chromium/Firefox profiles or other processes' in-memory session cookies"}
}
func (s *Service) Plan(kind, scope string, recordedURLs []string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.backend.Available() {
		return Plan{}, fmt.Errorf("WinINet cleanup requires Windows")
	}
	if kind != "cache" && kind != "cookies" {
		return Plan{}, fmt.Errorf("choose cache or cookies")
	}
	if scope != "recorded" && scope != "all" {
		return Plan{}, fmt.Errorf("choose recorded URLs or all WinINet entries")
	}
	if scope == "recorded" && (len(recordedURLs) == 0 || len(recordedURLs) > 10000) {
		return Plan{}, fmt.Errorf("recorded cleanup requires 1..10000 URLs")
	}
	entries, err := s.backend.Entries()
	if err != nil {
		return Plan{}, err
	}
	if len(entries) > 100000 {
		return Plan{}, fmt.Errorf("WinINet enumeration exceeds the supported bound")
	}
	var selected []Entry
	items := []Item{}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.Kind != kind || seen[entry.Key] {
			continue
		}
		display, valid := displayEntry(entry)
		if !valid {
			continue
		}
		if scope == "recorded" && !matches(entry, recordedURLs) {
			continue
		}
		selected = append(selected, entry)
		seen[entry.Key] = true
		if len(selected) > 10000 {
			return Plan{}, fmt.Errorf("cleanup exceeds 10000 entries; use recorded-URL scope")
		}
		if len(items) < 256 {
			sum := sha256.Sum256([]byte(entry.Key))
			items = append(items, Item{hex.EncodeToString(sum[:8]), display, entry.Kind, entry.Size})
		}
	}
	var random [24]byte
	if _, err := rand.Read(random[:]); err != nil {
		return Plan{}, err
	}
	plan := Plan{hex.EncodeToString(random[:]), kind, scope, len(selected), items, s.now().Add(2 * time.Minute)}
	s.plans[plan.ID] = &planned{plan: plan, entries: selected}
	s.order = append(s.order, plan.ID)
	for len(s.order) > 32 {
		delete(s.plans, s.order[0])
		s.order = s.order[1:]
	}
	return plan, nil
}
func (s *Service) Execute(id string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.plans[id]
	if p == nil {
		return Result{}, fmt.Errorf("cleanup plan is unavailable; preview again")
	}
	if p.result != nil {
		return *p.result, nil
	}
	if !s.now().Before(p.plan.ExpiresAt) {
		delete(s.plans, id)
		return Result{}, fmt.Errorf("cleanup preview expired; preview again")
	}
	current, err := s.backend.Entries()
	if err != nil {
		return Result{}, err
	}
	if len(current) > 100000 {
		return Result{}, fmt.Errorf("WinINet enumeration exceeds the supported bound")
	}
	found := map[string]Entry{}
	for _, entry := range current {
		found[entry.Key] = entry
	}
	result := Result{}
	for _, entry := range p.entries {
		actual, exists := found[entry.Key]
		if !exists || actual != entry {
			result.Changed++
			continue
		}
		if err := s.backend.Delete(entry); err != nil {
			result.Failed++
		} else {
			result.Deleted++
		}
	}
	p.result = &result
	p.entries = nil
	return result, nil
}
func normalized(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, false
	}
	u.User = nil
	u.Fragment = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	return u, true
}
func cookieDomainPath(key string) (string, string, bool) {
	if !strings.HasPrefix(strings.ToLower(key), "cookie:") {
		return "", "", false
	}
	_, rest, ok := strings.Cut(key[7:], "@")
	if !ok {
		return "", "", false
	}
	host, path, _ := strings.Cut(rest, "/")
	host = strings.TrimPrefix(strings.ToLower(host), ".")
	if host == "" || strings.ContainsAny(host, "@ :\\\x00") {
		return "", "", false
	}
	return host, "/" + path, true
}
func displayEntry(entry Entry) (string, bool) {
	if entry.Kind == "cookies" {
		host, path, ok := cookieDomainPath(entry.Key)
		return host + path, ok
	}
	u, ok := normalized(entry.Key)
	if !ok {
		return "", false
	}
	u.RawQuery = ""
	u.ForceQuery = false
	return u.String(), true
}
func matches(entry Entry, urls []string) bool {
	for _, raw := range urls {
		u, ok := normalized(raw)
		if !ok {
			continue
		}
		if entry.Kind == "cache" {
			target, ok := normalized(entry.Key)
			if ok && target.String() == u.String() {
				return true
			}
			continue
		}
		host, path, ok := cookieDomainPath(entry.Key)
		if !ok {
			continue
		}
		actual := strings.ToLower(u.Hostname())
		requestPath := u.EscapedPath()
		if requestPath == "" {
			requestPath = "/"
		}
		if (actual == host || strings.HasSuffix(actual, "."+host)) && (requestPath == path || strings.HasPrefix(requestPath, path) && (strings.HasSuffix(path, "/") || len(requestPath) > len(path) && requestPath[len(path)] == '/')) {
			return true
		}
	}
	return false
}
