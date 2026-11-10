package processlaunch

import (
	"errors"
	"sync"
	"testing"
)

func TestResumedChildCannotBeAbortedThroughReusedHandles(t *testing.T) {
	resumes, aborts, releases := 0, 0, 0
	child := &Suspended{state: "prepared", resume: func() error { resumes++; return nil }, abort: func() { aborts++ }, release: func() { releases++ }}
	if child.Resume() != nil {
		t.Fatal("resume")
	}
	child.Abort()
	child.Abort()
	if child.Resume() == nil || resumes != 1 || aborts != 0 || releases != 1 {
		t.Fatalf("%d %d %d", resumes, aborts, releases)
	}
}
func TestFailedResumeRetainsOnlyOwnedAbortOnce(t *testing.T) {
	aborts, releases := 0, 0
	child := &Suspended{state: "prepared", resume: func() error { return errors.New("fixture") }, abort: func() { aborts++ }, release: func() { releases++ }}
	if child.Resume() == nil {
		t.Fatal("expected failure")
	}
	child.Abort()
	child.Abort()
	if aborts != 1 || releases != 1 {
		t.Fatal(aborts, releases)
	}
}
func TestConcurrentResumeAbortReleasesOnce(t *testing.T) {
	released := 0
	child := &Suspended{state: "prepared", resume: func() error { return nil }, abort: func() {}, release: func() { released++ }}
	var wait sync.WaitGroup
	for i := 0; i < 10; i++ {
		wait.Add(1)
		go func() { defer wait.Done(); _ = child.Resume(); child.Abort() }()
	}
	wait.Wait()
	if released != 1 {
		t.Fatal(released)
	}
}
