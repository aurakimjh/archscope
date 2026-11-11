package analysis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func TestReviewR08RetentionByteAndCountBudgets(t *testing.T) {
	c, _ := ingestion.NewCatalog(nil, nil, nil)
	r, _ := resultcontract.New()
	s, e := New(t.TempDir(), c, r)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.storedBytes = maxStoredBytes
	staged := filepath.Join(t.TempDir(), "result.json")
	os.WriteFile(staged, []byte("{}"), 0600)
	if _, e = s.publish(context.Background(), staged, "result-00000000000000000000000000000000", []byte("{}")); e == nil {
		t.Fatal("retention byte budget ignored")
	}
	if _, e = os.Stat(staged); e != nil {
		t.Fatal("failed publish consumed staged file")
	}
	s.storedBytes = 0
	for i := 0; i < maxStoredResults; i++ {
		s.stored[fmt.Sprint(i)] = storedRevision{}
	}
	if _, e = s.publish(context.Background(), staged, "result-00000000000000000000000000000000", []byte("{}")); e == nil {
		t.Fatal("retention count budget ignored")
	}

}

func TestReviewR08LoadQueueCancellationAndShutdown(t *testing.T) {
	c, _ := ingestion.NewCatalog(nil, nil, nil)
	r, _ := resultcontract.New()
	s, e := New(t.TempDir(), c, r)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for i := 0; i < cap(s.slots); i++ {
		s.slots <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e = s.beginLoad(ctx); !errors.Is(e, context.Canceled) {
		t.Fatal("parent cancellation misclassified", e)
	}
	s.policy.QueueTimeoutMS = 10
	if _, _, e = s.beginLoad(context.Background()); e == nil || !strings.Contains(e.Error(), "IMPORT_QUEUE_TIMEOUT") {
		t.Fatal(e)
	}
	for len(s.slots) > 0 {
		<-s.slots
	}
	_, finish, e := s.beginLoad(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	closed := make(chan struct{})
	go func() { s.Close(); close(closed) }()
	select {
	case <-s.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel load")
	}
	finish()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not wait for load")
	}
}
