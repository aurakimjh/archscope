package automation

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestConcurrentMutationRetriesExecuteOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.json")
	var executed atomic.Int32
	server, err := Start(path, func(context.Context, Call) (any, error) { return executed.Add(1), nil })
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	call := Call{ID: "one-mutation", Command: "start", Params: json.RawMessage(`{}`)}
	var wait sync.WaitGroup
	for i := 0; i < 24; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			reply, err := Invoke(context.Background(), path, call)
			if err != nil || !reply.OK || string(reply.Result) != "1" {
				t.Errorf("retry result: %+v %v", reply, err)
			}
		}()
	}
	wait.Wait()
	if executed.Load() != 1 {
		t.Fatal("concurrent retries executed more than once")
	}
}
func TestCanceledQueuedMutationDoesNotExecuteOrConsumeRetryID(t *testing.T) {
	var executed atomic.Int32
	server, err := Start(filepath.Join(t.TempDir(), "control.json"), func(context.Context, Call) (any, error) { return executed.Add(1), nil })
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest("POST", server.descriptor.URL, strings.NewReader(`{"id":"retry-after-cancel","command":"start"}`)).WithContext(ctx)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("Authorization", "Bearer "+server.descriptor.Token)
	request.Header.Set("Content-Type", "application/json")
	server.handle(httptest.NewRecorder(), request)
	if executed.Load() != 0 {
		t.Fatal("canceled mutation executed")
	}
	reply, err := Invoke(context.Background(), server.path, Call{ID: "retry-after-cancel", Command: "start"})
	if err != nil || !reply.OK || executed.Load() != 1 {
		t.Fatal("canceled request consumed its retry ID")
	}
}
func TestControllerShutdownPreservesReplacementDescriptor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.json")
	server, err := Start(path, func(context.Context, Call) (any, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	replacement := []byte(`{"schemaVersion":1,"token":"a-replacement-owned-by-another-instance"}`)
	if err = os.WriteFile(path, replacement, 0600); err != nil {
		t.Fatal(err)
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(replacement) {
		t.Fatal("shutdown removed another instance's descriptor")
	}
	if _, err = Invoke(context.Background(), path, Call{}); err == nil {
		t.Fatal("invalid replacement descriptor accepted")
	}
}
func TestConcurrentJournalKeepsBoundedOrderedReplay(t *testing.T) {
	var journal Journal
	var wait sync.WaitGroup
	for writer := 0; writer < 8; writer++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := 0; i < 400; i++ {
				journal.Append("updated", "fixture", map[string]int{"index": i})
				_ = journal.Read(0, 10)
			}
		}()
	}
	wait.Wait()
	page := journal.Read(0, 100)
	if !page.ResyncRequired || page.Latest != 3200 {
		t.Fatal("journal did not signal bounded replay gap")
	}
	var last uint64
	for len(page.Items) > 0 {
		for _, event := range page.Items {
			if event.Sequence <= last {
				t.Fatal("replay order regressed")
			}
			last = event.Sequence
		}
		page = journal.Read(last, 100)
	}
	if last != 3200 || len(journal.items) > 512 || journal.size > 1<<20 {
		t.Fatal("journal bounds or replay tail failed")
	}
}

func TestDescriptorSizeIsBoundedBeforeReadOrCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	server, err := Start(path, func(context.Context, Call) (any, error) {
		t.Fatal("oversized descriptor invoked command")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a replacement/invalid local descriptor without allocating its data.
	if err = os.Truncate(path, 1<<20); err != nil {
		t.Fatal(err)
	}
	if _, err = Invoke(context.Background(), path, Call{}); err == nil {
		t.Fatal("oversized descriptor accepted")
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != 1<<20 {
		t.Fatal("cleanup removed an unrecognized descriptor")
	}
}
