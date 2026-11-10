package main

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/browsercache"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
)

type CaptureCacheRequest struct {
	Source CaptureFileSource `json:"source"`
	Kind   string            `json:"kind"`
	Scope  string            `json:"scope"`
}

func (s *CaptureService) GetCaptureCacheCapability() browsercache.Capability {
	return s.browserCache.Capability()
}
func (s *CaptureService) PlanCaptureCacheCleanup(request CaptureCacheRequest) (browsercache.Plan, error) {
	urls := []string{}
	if request.Scope == "recorded" {
		if (request.Source.SessionID == "") == (request.Source.Path == "") {
			return browsercache.Plan{}, fmt.Errorf("choose one recorded source")
		}
		seen := map[string]bool{}
		appendURL := func(raw string) error {
			if !seen[raw] {
				urls = append(urls, raw)
				seen[raw] = true
			}
			if len(urls) > 10000 {
				return fmt.Errorf("recorded cleanup exceeds 10000 URLs")
			}
			return nil
		}
		if request.Source.Path != "" {
			doc, _, err := sessionfile.Read(request.Source.Path)
			if err != nil {
				return browsercache.Plan{}, err
			}
			for _, tx := range doc.Transactions {
				if err := appendURL(tx.URL); err != nil {
					return browsercache.Plan{}, err
				}
			}
		} else {
			cursor := ""
			for {
				page, err := s.manager.Fetch(capture.SessionID(request.Source.SessionID), store.Filter{MetadataOnly: true}, cursor, store.MaxFetchLimit)
				if err != nil {
					return browsercache.Plan{}, err
				}
				for _, item := range page.Items {
					if err := appendURL(item.Transaction.URL); err != nil {
						return browsercache.Plan{}, err
					}
				}
				if page.NextCursor == "" {
					break
				}
				cursor = page.NextCursor
			}
		}
	}
	return s.browserCache.Plan(request.Kind, request.Scope, urls)
}
func (s *CaptureService) ExecuteCaptureCacheCleanup(planID string) (browsercache.Result, error) {
	return s.browserCache.Execute(planID)
}
