// Package localfiles publishes user-selected new artifacts without replacement.
package localfiles

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
func failure(code string) error {
	messages := map[string]string{"OUTPUT_EXISTS": "Choose unused output and provenance names; existing files are never replaced.", "OUTPUT_MANAGED_PATH": "Choose output outside managed application state.", "OUTPUT_CHANGED": "Source bytes changed before publication; no output was published.", "OUTPUT_UNAVAILABLE": "Check free disk space and permissions in an existing destination directory; no output was published."}
	return &Error{code, messages[code]}
}

// Preflight is advisory. Publication repeats checks and links atomically without replacement.
func Preflight(target string, protected ...string) (string, error) {
	absolute, e := filepath.Abs(target)
	if e != nil {
		return "", failure("OUTPUT_UNAVAILABLE")
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(absolute))
	if e != nil {
		return "", failure("OUTPUT_UNAVAILABLE")
	}
	target = filepath.Join(parent, filepath.Base(absolute))
	for _, root := range protected {
		root, e = filepath.Abs(root)
		if e != nil {
			return "", failure("OUTPUT_UNAVAILABLE")
		}
		resolved, err := filepath.EvalSymlinks(root)
		if err == nil {
			root = resolved
		} else if !os.IsNotExist(err) {
			return "", failure("OUTPUT_UNAVAILABLE")
		}
		rel, e := filepath.Rel(root, target)
		if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", failure("OUTPUT_MANAGED_PATH")
		}
	}
	if _, e = os.Lstat(target); e == nil {
		return "", failure("OUTPUT_EXISTS")
	} else if !os.IsNotExist(e) {
		return "", failure("OUTPUT_UNAVAILABLE")
	}
	return target, nil
}
func Publish(ctx context.Context, target string, body []byte, protected ...string) error {
	h := sha256.Sum256(body)
	return PublishReader(ctx, target, bytes.NewReader(body), int64(len(body)), hex.EncodeToString(h[:]), protected...)
}

// PublishReader hashes/streams into a private same-directory file before atomic no-replace.
// No whole-document byte buffer or internal temporary path is returned to callers.
func PublishReader(ctx context.Context, target string, source io.Reader, size int64, digest string, protected ...string) error {
	target, e := Preflight(target, protected...)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(target), ".input-assist-")
	if e != nil {
		return failure("OUTPUT_UNAVAILABLE")
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()
	if e = f.Chmod(0600); e != nil {
		return failure("OUTPUT_UNAVAILABLE")
	}
	h := sha256.New()
	buffer := make([]byte, 32<<10)
	var total int64
	for {
		if e = ctx.Err(); e != nil {
			return e
		}
		n, err := source.Read(buffer)
		total += int64(n)
		if total > size {
			return failure("OUTPUT_CHANGED")
		}
		if n > 0 {
			h.Write(buffer[:n])
			written, e := f.Write(buffer[:n])
			if e != nil || written != n {
				return failure("OUTPUT_UNAVAILABLE")
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return failure("OUTPUT_UNAVAILABLE")
		}
	}
	if total != size || hex.EncodeToString(h.Sum(nil)) != digest {
		return failure("OUTPUT_CHANGED")
	}
	if e = f.Sync(); e != nil {
		return failure("OUTPUT_UNAVAILABLE")
	}
	if e = f.Close(); e != nil {
		return failure("OUTPUT_UNAVAILABLE")
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	if e = os.Link(tmp, target); e != nil {
		if os.IsExist(e) {
			return failure("OUTPUT_EXISTS")
		}
		return failure("OUTPUT_UNAVAILABLE")
	}
	return nil
}
