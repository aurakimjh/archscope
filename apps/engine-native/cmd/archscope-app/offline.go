package main

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/distribution"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/netpolicy"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type offlineOptions struct{ Root, Report, Webview string }

// Parse only owned flags; leave the existing automation contract untouched.
func parseOffline(args []string) (offlineOptions, []string, error) {
	o := offlineOptions{}
	rest := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, val, has := strings.Cut(a, "=")
		if name != "--offline-root" && name != "--offline-report" {
			rest = append(rest, a)
			continue
		}
		if !has {
			if i+1 == len(args) {
				return o, nil, fmt.Errorf("%s requires a path", name)
			}
			i++
			val = args[i]
		}
		if val == "" {
			return o, nil, fmt.Errorf("%s requires a path", name)
		}
		if name == "--offline-root" {
			o.Root = val
		} else {
			o.Report = val
		}
	}
	if o.Root != "" {
		var e error
		o.Root, e = filepath.Abs(o.Root)
		if e != nil {
			return o, nil, e
		}
		if runtime.GOOS == "windows" {
			for _, name := range []string{"WEBVIEW2_BROWSER_EXECUTABLE_FOLDER", "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "WEBVIEW2_USER_DATA_FOLDER", "WEBVIEW2_RELEASE_CHANNEL_PREFERENCE"} {
				if os.Getenv(name) != "" {
					return o, nil, fmt.Errorf("OFFLINE_ENVIRONMENT_OVERRIDE: clear process override %s before using a pinned bundle", name)
				}
			}
		}
		m, e := distribution.Verify(o.Root, true)
		if e != nil {
			return o, nil, e
		}
		if m.WebviewPath != "" {
			o.Webview = filepath.Join(o.Root, m.WebviewPath)
		}
		netpolicy.Enable()
	}
	return o, rest, nil
}
func desktopAssets(base http.Handler) http.Handler {
	resources, _ := fs.Sub(distribution.Assets, "assets")
	local := http.StripPrefix("/offline-assets/", http.FileServer(http.FS(resources)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if netpolicy.Enabled() {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; object-src 'none'; frame-src 'none'; base-uri 'self'; form-action 'none'")
		}
		if strings.HasPrefix(r.URL.Path, "/offline-assets/") {
			local.ServeHTTP(w, r)
			return
		}
		base.ServeHTTP(w, r)
	})
}
func (s *EngineService) GetOfflineStatus() string {
	b := netpolicy.Snapshot()
	return mustOfflineJSON(b)
}
func (s *EngineService) RecordOfflineRendererViolation() error {
	return netpolicy.RequireOnline("renderer.csp")
}
func writeOfflineReport(path string) {
	if e := netpolicy.WriteReport(path); e != nil {
		fmt.Fprintln(os.Stderr, e)
	}
}

func mustOfflineJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
