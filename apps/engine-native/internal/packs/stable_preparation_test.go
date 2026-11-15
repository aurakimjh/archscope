//go:build releasequalification

package packs

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// The runner changes only AppVersion with a Go source overlay. Ephemeral test
// signatures never replace bundled trust or qualify an official release pack.
func TestStablePackPreparation(t *testing.T) {
	ctx := context.Background()
	if expected := os.Getenv("ARCHSCOPE_PACK_EXPECTED_VERSION"); expected != "" && expected != AppVersion {
		t.Fatalf("overlay not applied: %s != %s", AppVersion, expected)
	}
	root := os.Getenv("ARCHSCOPE_PACK_PREPARATION_ROOT")
	if root == "" {
		root = t.TempDir()
	}
	phase := os.Getenv("ARCHSCOPE_PACK_PREPARATION_PHASE")
	keys, err := BootstrapKeys()
	if err != nil {
		t.Fatal(err)
	}
	original, err := Assets.ReadFile("assets/bootstrap.zip")
	if err != nil {
		t.Fatal(err)
	}
	keyMap := map[string]Key{}
	for _, k := range keys {
		keyMap[k.ID] = k
	}
	_, oldErr := Validate(ctx, original, keyMap, InstallOptions{})
	oldAllowed := (Range{"0.3.0", "0.4.0"}).acceptsAppVersion(AppVersion)
	if (oldErr == nil) != oldAllowed {
		t.Fatalf("original compatibility: %v", oldErr)
	}
	if !oldAllowed {
		var issue *Error
		if !errors.As(oldErr, &issue) || issue.Code != "PACK_INCOMPATIBLE" {
			t.Fatal("unexpected original rejection", oldErr)
		}
	}
	report := map[string]any{"appVersion": AppVersion, "originalAccepted": oldAllowed, "officialSignerQualified": false, "scope": "ephemeral fixture trust only; production bytes and trust unchanged"}
	var body []byte
	var key Key
	if phase == "stable" {
		body, err = os.ReadFile(filepath.Join(root, "candidate.zip"))
		if err != nil {
			t.Fatal(err)
		}
		b, e := os.ReadFile(filepath.Join(root, "candidate-key.json"))
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(b, &key); e != nil {
			t.Fatal(e)
		}
	} else {
		var manifest Manifest
		payload := map[string][]byte{}
		z, e := zip.NewReader(bytes.NewReader(original), int64(len(original)))
		if e != nil {
			t.Fatal(e)
		}
		for _, f := range z.File {
			r, e := f.Open()
			if e != nil {
				t.Fatal(e)
			}
			b, e := io.ReadAll(r)
			r.Close()
			if e != nil {
				t.Fatal(e)
			}
			if f.Name == "manifest.json" {
				if e = json.Unmarshal(b, &manifest); e != nil {
					t.Fatal(e)
				}
			} else if f.Name != "signature.json" {
				payload[f.Name] = b
			}
		}
		var privateKey []byte
		key, privateKey = keypair(t, "fixture.stable-preparation", false)
		manifest.Version = "1.0.1"
		manifest.App.Before = "0.4.1"
		manifest.Signer = key.ID
		body, err = Archive(manifest, payload, privateKey)
		if err != nil {
			t.Fatal(err)
		}
		if phase == "seed" {
			if e = os.WriteFile(filepath.Join(root, "candidate.zip"), body, 0600); e != nil {
				t.Fatal(e)
			}
			b, _ := json.Marshal(key)
			if e = os.WriteFile(filepath.Join(root, "candidate-key.json"), b, 0600); e != nil {
				t.Fatal(e)
			}
			// No private key is persisted. The two stores are owned synthetic state.
			for _, name := range []string{"legacy", "prepared"} {
				m, e := OpenDefault(ctx, filepath.Join(root, name), nil)
				if e != nil {
					t.Fatal(e)
				}
				if name == "prepared" {
					if e = m.RegisterKey(ctx, state(t, m).Revision, key); e != nil {
						t.Fatal(e)
					}
					r, e := m.Install(ctx, body, InstallOptions{})
					if e != nil {
						t.Fatal(e)
					}
					if _, e = m.Activate(ctx, state(t, m).Revision, "application", r.SHA256, nil); e != nil {
						t.Fatal(e)
					}
					for _, old := range state(t, m).Installed {
						if old.Version == "1.0.0" {
							if _, e = m.Rollback(ctx, state(t, m).Revision, "application", old.SHA256, nil); e != nil {
								t.Fatal(e)
							}
						}
					}
					if _, e = m.Activate(ctx, state(t, m).Revision, "application", r.SHA256, nil); e != nil {
						t.Fatal(e)
					}
					report["reversiblePreCutover"] = true
				}
				if e = m.Close(); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
	allowed := (Range{"0.3.0", "0.4.1"}).acceptsAppVersion(AppVersion)
	r, err := Validate(ctx, body, map[string]Key{key.ID: key}, InstallOptions{})
	if (err == nil) != allowed {
		t.Fatalf("candidate compatibility: %v", err)
	}
	if !allowed {
		var issue *Error
		if !errors.As(err, &issue) || issue.Code != "PACK_INCOMPATIBLE" {
			t.Fatal("unexpected future rejection", err)
		}
	}
	report["candidateAccepted"] = allowed
	if allowed {
		tampered := rewriteZip(t, body, func(name string, b []byte) (string, []byte) {
			if name == "manifest.json" {
				b = append([]byte(" "), b...)
			}
			return name, b
		})
		if _, e := Validate(ctx, tampered, map[string]Key{key.ID: key}, InstallOptions{}); e == nil {
			t.Fatal("tampered manifest trusted")
		}
		if _, e := Validate(ctx, body, keyMap, InstallOptions{}); e == nil {
			t.Fatal("ephemeral signer trusted by production keys")
		}
		m, dir := manager(t, key)
		ref, e := m.Install(ctx, body, InstallOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = m.Activate(ctx, state(t, m).Revision, "application", ref.SHA256, nil); e != nil {
			t.Fatal(e)
		}
		if e = m.Close(); e != nil {
			t.Fatal(e)
		}
		fresh, e := Open(dir, []Key{key}, nil)
		if e != nil {
			t.Fatal(e)
		}
		defer fresh.Close()
		snapshot, e := fresh.Snapshot(ctx, "application")
		if e != nil || len(snapshot.Refs) != 1 || snapshot.Refs[0].SHA256 != r.Ref.SHA256 {
			t.Fatal("fresh restart", snapshot, e)
		}
		report["installActivateRestart"] = true
		report["tamperedAndUntrustedRejected"] = true
	}
	if phase == "stable" {
		for _, name := range []string{"legacy", "prepared"} {
			m, e := Open(filepath.Join(root, name), keys, nil)
			if e != nil {
				t.Fatal(e)
			}
			s, readErr := m.Snapshot(ctx, "application")
			if name == "legacy" {
				var issue *Error
				if !errors.As(readErr, &issue) || issue.Code != "PACK_INCOMPATIBLE" {
					t.Fatal("legacy active pack should block", readErr)
				}
				report["unpreparedRcStoreRejected"] = true
			} else {
				if readErr != nil || len(s.Refs) != 1 || s.Refs[0].SHA256 != r.Ref.SHA256 {
					t.Fatal("prepared upgrade", s, readErr)
				}
				report["explicitlyPreparedRcStoreReopens"] = true
				before := state(t, m)
				for _, old := range before.Installed {
					if old.Version == "1.0.0" {
						if _, e = m.Rollback(ctx, before.Revision, "application", old.SHA256, nil); e == nil {
							t.Fatal("stable rollback accepted incompatible old pack")
						}
						if state(t, m).Revision != before.Revision {
							t.Fatal("failed rollback changed active state")
						}
						report["incompatibleRollbackRejectedWithoutStateChange"] = true
					}
				}
			}
			m.Close()
		}
	}
	if os.Getenv("ARCHSCOPE_PACK_PREPARATION_ROOT") != "" {
		b, _ := json.MarshalIndent(report, "", "  ")
		if err = os.WriteFile(filepath.Join(root, "result-"+AppVersion+".json"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
