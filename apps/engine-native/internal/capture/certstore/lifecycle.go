// Package certstore manages the temporary capture CA trust lifecycle. It
// records every successfully mutated store and rolls those mutations back if
// a later install fails.
package certstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const CurrentUserRoot = "current-user-root"

type Backend interface {
	Install(store string, certificateDER []byte) error
	Remove(store string, certificateDER []byte) error
}

type Status struct {
	State       string    `json:"state"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Stores      []string  `json:"stores"`
	ExpiresAt   time.Time `json:"expiresAt,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type Lifecycle struct {
	mu         sync.Mutex
	backend    Backend
	stores     []string
	der        []byte
	status     Status
	recordPath string
}

func New(backend Backend, stores []string) *Lifecycle {
	if len(stores) == 0 {
		stores = []string{CurrentUserRoot}
	}
	return &Lifecycle{backend: backend, stores: append([]string(nil), stores...), status: Status{State: "absent", Stores: []string{}}}
}

func NewPersistent(backend Backend, stores []string, recordPath string) *Lifecycle {
	lifecycle := New(backend, stores)
	lifecycle.recordPath = recordPath
	lifecycle.loadRecord()
	return lifecycle
}

func NewSystem(recordPath string) *Lifecycle {
	return NewPersistent(systemBackend{}, []string{CurrentUserRoot}, recordPath)
}

func (l *Lifecycle) Install(certificateDER []byte, expiresAt time.Time) (Status, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(certificateDER) == 0 {
		return l.status, errors.New("capture CA certificate is empty")
	}
	if l.backend == nil {
		return l.status, errors.New("capture CA trust backend is unavailable")
	}
	fingerprint := sha256.Sum256(certificateDER)
	fingerprintText := hex.EncodeToString(fingerprint[:])
	if len(l.der) > 0 {
		current := sha256.Sum256(l.der)
		if fingerprintText == hex.EncodeToString(current[:]) && l.status.State == "trusted" {
			return l.status, nil
		}
		return l.status, errors.New("remove the previously installed capture CA before installing another")
	}
	status := Status{
		State: "installing", Fingerprint: fingerprintText,
		Stores: []string{}, ExpiresAt: expiresAt,
	}
	for _, name := range l.stores {
		if err := l.backend.Install(name, certificateDER); err != nil {
			rollbackErr := l.rollback(certificateDER, status.Stores)
			status.State = "failed"
			status.Error = errors.Join(fmt.Errorf("install %s: %w", name, err), rollbackErr).Error()
			l.status = status
			return status, errors.New(status.Error)
		}
		status.Stores = append(status.Stores, name)
	}
	l.der = append([]byte(nil), certificateDER...)
	status.State = "trusted"
	l.status = status
	if err := l.saveRecord(); err != nil {
		rollbackErr := l.rollback(certificateDER, status.Stores)
		for index := range l.der {
			l.der[index] = 0
		}
		l.der = nil
		status.State = "failed"
		status.Error = errors.Join(fmt.Errorf("persist trust record: %w", err), rollbackErr).Error()
		l.status = status
		return status, errors.New(status.Error)
	}
	return status, nil
}

func (l *Lifecycle) Remove() (Status, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.der) == 0 {
		l.status = Status{State: "absent", Stores: []string{}}
		return l.status, nil
	}
	var removeErr error
	for index := len(l.status.Stores) - 1; index >= 0; index-- {
		name := l.status.Stores[index]
		if err := l.backend.Remove(name, l.der); err != nil {
			removeErr = errors.Join(removeErr, fmt.Errorf("remove %s: %w", name, err))
		}
	}
	if removeErr != nil {
		l.status.State = "partial"
		l.status.Error = removeErr.Error()
		return l.status, removeErr
	}
	for index := range l.der {
		l.der[index] = 0
	}
	l.der = nil
	l.status = Status{State: "absent", Stores: []string{}}
	if l.recordPath != "" {
		if err := os.Remove(l.recordPath); err != nil && !os.IsNotExist(err) {
			l.status.State = "partial"
			l.status.Error = fmt.Sprintf("remove trust record: %v", err)
			return l.status, err
		}
	}
	return l.status, nil
}

func (l *Lifecycle) Status() Status {
	l.mu.Lock()
	defer l.mu.Unlock()
	status := l.status
	status.Stores = append([]string(nil), status.Stores...)
	if status.State == "trusted" && !status.ExpiresAt.IsZero() && time.Now().After(status.ExpiresAt) {
		status.State = "expired"
	}
	return status
}

func (l *Lifecycle) rollback(certificateDER []byte, stores []string) error {
	var rollbackErr error
	for index := len(stores) - 1; index >= 0; index-- {
		if err := l.backend.Remove(stores[index], certificateDER); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("rollback %s: %w", stores[index], err))
		}
	}
	return rollbackErr
}

type trustRecord struct {
	CertificateDER []byte `json:"certificateDer"`
	Status         Status `json:"status"`
}

func (l *Lifecycle) saveRecord() error {
	if l.recordPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.recordPath), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(trustRecord{CertificateDER: l.der, Status: l.status})
	if err != nil {
		return err
	}
	temporary := l.recordPath + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, l.recordPath)
}

func (l *Lifecycle) loadRecord() {
	if l.recordPath == "" {
		return
	}
	data, err := os.ReadFile(l.recordPath)
	if err != nil {
		return
	}
	var record trustRecord
	if json.Unmarshal(data, &record) != nil || len(record.CertificateDER) == 0 {
		l.status = Status{State: "partial", Stores: []string{}, Error: "invalid persisted capture CA trust record"}
		return
	}
	l.der = append([]byte(nil), record.CertificateDER...)
	l.status = record.Status
	l.status.Stores = append([]string(nil), record.Status.Stores...)
}
