package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestT635CaseBaselineCLIPreservesServiceRefusal(t *testing.T) {
	dir := t.TempDir()
	request := filepath.Join(dir, "request.json")
	os.WriteFile(request, []byte(`{"case_id":"invalid","revision":"1"}`), 0600)
	group, _, e := rootCmd.Find([]string{"case-query"})
	if e != nil {
		t.Fatal(e)
	}
	priorStore, _ := group.PersistentFlags().GetString("store")
	priorRequest, _ := group.PersistentFlags().GetString("request")
	t.Cleanup(func() {
		group.PersistentFlags().Set("store", priorStore)
		group.PersistentFlags().Set("request", priorRequest)
	})
	for _, name := range []string{"pin-baseline", "compare-baseline"} {
		if e = group.PersistentFlags().Set("store", filepath.Join(dir, name)); e != nil {
			t.Fatal(e)
		}
		group.PersistentFlags().Set("request", request)
		command, _, e := rootCmd.Find([]string{"case-query", name})
		if e != nil {
			t.Fatal(e)
		}
		command.SetContext(context.Background())
		if e = command.RunE(command, nil); e == nil {
			t.Fatal("service refusal was swallowed", name)
		}
	}
}
