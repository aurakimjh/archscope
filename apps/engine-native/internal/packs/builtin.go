package packs

import (
	"context"
	"embed"
	"encoding/json"
)

//go:embed assets/*
var Assets embed.FS

func BootstrapKeys() ([]Key, error) {
	b, e := Assets.ReadFile("assets/trusted-keys.json")
	if e != nil {
		return nil, e
	}
	var keys []Key
	e = Strict(b, &keys)
	return keys, e
}
func OpenDefault(ctx context.Context, root string, check func(Snapshot) error) (*Manager, error) {
	keys, e := BootstrapKeys()
	if e != nil {
		return nil, e
	}
	m, e := Open(root, keys, check)
	if e != nil {
		return nil, e
	}
	closeFail := func(e error) (*Manager, error) { m.Close(); return nil, e }
	b, e := Assets.ReadFile("assets/bootstrap.zip")
	if e != nil {
		return closeFail(e)
	}
	r, e := m.Install(ctx, b, InstallOptions{})
	if e != nil {
		return closeFail(e)
	}
	s, e := m.State()
	if e != nil {
		return closeFail(e)
	}
	// A nonempty owner state is never overwritten by a default activation.
	if len(s.Active) == 0 && s.Revision == "2" {
		if _, e = m.Activate(ctx, s.Revision, "application", r.SHA256, nil); e != nil {
			return closeFail(e)
		}
	}
	return m, nil
}
func MarshalSnapshot(s Snapshot) string { b, _ := json.Marshal(s); return string(b) }
