// Package localfiles publishes user-selected new artifacts without replacement.
package localfiles

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Publish(ctx context.Context, target string, body []byte, protected ...string) error {
	absolute, e := filepath.Abs(target)
	if e != nil {
		return e
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(absolute))
	if e != nil {
		return e
	}
	target = filepath.Join(parent, filepath.Base(absolute))
	for _, root := range protected {
		root, e = filepath.Abs(root)
		if e != nil {
			return e
		}
		resolved, err := filepath.EvalSymlinks(root)
		if err == nil {
			root = resolved
		} else if !os.IsNotExist(err) {
			return err
		}
		rel, e := filepath.Rel(root, target)
		if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("OUTPUT_MANAGED_PATH: choose a new file outside managed state")
		}
	}
	f, e := os.CreateTemp(parent, ".input-assist-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()
	if e = f.Chmod(0600); e != nil {
		return e
	}
	for len(body) > 0 {
		if e = ctx.Err(); e != nil {
			return e
		}
		n := min(len(body), 32<<10)
		written, err := f.Write(body[:n])
		if err != nil {
			return err
		}
		if written != n {
			return io.ErrShortWrite
		}
		body = body[n:]
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	return os.Link(tmp, target) // Atomic no-replace, same filesystem.
}
