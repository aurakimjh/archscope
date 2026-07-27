package certstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type backendCall struct {
	action string
	store  string
}

type fakeBackend struct {
	calls       []backendCall
	failInstall string
	failRemove  string
}

func (f *fakeBackend) Install(store string, _ []byte) error {
	f.calls = append(f.calls, backendCall{action: "install", store: store})
	if store == f.failInstall {
		return errors.New("install failed")
	}
	return nil
}

func (f *fakeBackend) Remove(store string, _ []byte) error {
	f.calls = append(f.calls, backendCall{action: "remove", store: store})
	if store == f.failRemove {
		return errors.New("remove failed")
	}
	return nil
}

func TestInstallFailureRollsBackSuccessfulStores(t *testing.T) {
	backend := &fakeBackend{failInstall: "second"}
	lifecycle := New(backend, []string{"first", "second"})
	status, err := lifecycle.Install([]byte("certificate"), time.Now().Add(time.Hour))
	if err == nil || status.State != "failed" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	want := []backendCall{
		{action: "install", store: "first"},
		{action: "install", store: "second"},
		{action: "remove", store: "first"},
	}
	if !reflect.DeepEqual(backend.calls, want) {
		t.Fatalf("calls=%+v, want %+v", backend.calls, want)
	}
}

func TestRemoveIsIdempotentAndReverseOrdered(t *testing.T) {
	backend := &fakeBackend{}
	lifecycle := New(backend, []string{"first", "second"})
	if _, err := lifecycle.Install([]byte("certificate"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.Remove(); err != nil {
		t.Fatal(err)
	}
	want := []backendCall{
		{action: "install", store: "first"},
		{action: "install", store: "second"},
		{action: "remove", store: "second"},
		{action: "remove", store: "first"},
	}
	if !reflect.DeepEqual(backend.calls, want) {
		t.Fatalf("calls=%+v, want %+v", backend.calls, want)
	}
}

func TestPersistedPublicCertificateAllowsCrashCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca-trust.json")
	firstBackend := &fakeBackend{}
	first := NewPersistent(firstBackend, []string{"root"}, path)
	if _, err := first.Install([]byte("public-certificate"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	secondBackend := &fakeBackend{}
	restarted := NewPersistent(secondBackend, []string{"root"}, path)
	if restarted.Status().State != "trusted" {
		t.Fatalf("status=%+v", restarted.Status())
	}
	if _, err := restarted.Remove(); err != nil {
		t.Fatal(err)
	}
	want := []backendCall{{action: "remove", store: "root"}}
	if !reflect.DeepEqual(secondBackend.calls, want) {
		t.Fatalf("calls=%+v, want %+v", secondBackend.calls, want)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("trust record still exists: %v", err)
	}
}
