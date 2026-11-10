// Package comautomation implements a late-bound native automation surface over
// the existing desktop commands. It never replaces the vendor's COM identity.
package comautomation

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const ProgID = "ArchScope.HTTPAnalyzerStandAlone"
const ClassID = "{8AE58B7F-E26A-4B0F-98F1-48765C98E71A}"

type Object interface {
	Members() []string
	Invoke(context.Context, string, []any, bool) (any, error)
}
type Enumeration struct{ Items []Object }
type Controller struct {
	captureBodies                bool
	sessionID                    string
	cachedLog                    Object
	cachedKey                    string
	loadedCleared                bool
	mu                           sync.Mutex
	execute                      automation.Executor
	scope, backend, path, loaded string
	targets                      []models.ProcessKey
	counter                      atomic.Uint64
}

func New(execute automation.Executor) *Controller {
	return &Controller{execute: execute, scope: "all_sessions", backend: "windows_transparent"}
}
func (c *Controller) call(ctx context.Context, name string, params any, target any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	value, err := c.execute(ctx, automation.Call{ID: fmt.Sprintf("com-%d", c.counter.Add(1)), Command: name, Params: raw})
	if err != nil {
		return err
	}
	if target == nil {
		return nil
	}
	raw, err = json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}
func (c *Controller) Members() []string {
	return []string{"AttachAllSessions", "AttachCurrentSession", "AttachNewProcess", "AttachProcessByID", "AttachProcessByName", "DetachProcessByID", "DetachProcessByName", "Start", "Stop", "Pause", "Resume", "Clear", "IsLogging", "IsPaused", "Visible", "Log", "SaveLog", "LoadLog", "DeleteAllCache", "DeleteAllCookies", "DeleteCacheForRecordedURLs", "DeleteCookiesForRecordedURLs", "Backend", "SessionID", "ProxyAddress", "BuildCommit", "CaptureBodies"}
}
func (c *Controller) Invoke(ctx context.Context, name string, args []any, put bool) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	name = strings.ToLower(name)
	text := func(index int) (string, error) {
		if index >= len(args) {
			return "", fmt.Errorf("missing string argument")
		}
		value, ok := args[index].(string)
		if !ok {
			return "", fmt.Errorf("expected string argument")
		}
		return value, nil
	}
	if strings.HasPrefix(name, "attach") || strings.HasPrefix(name, "detach") {
		var current capture.Session
		if err := c.call(ctx, "status", map[string]any{}, &current); err != nil {
			return nil, err
		}
		if current.State == capture.StateRunning || current.State == capture.StateStarting || current.State == capture.StateStopping {
			return nil, fmt.Errorf("stop capture before changing attached processes")
		}
	}
	switch name {
	case "capturebodies":
		if put {
			if len(args) != 1 {
				return nil, fmt.Errorf("CaptureBodies requires one boolean")
			}
			value, ok := args[0].(bool)
			if !ok {
				return nil, fmt.Errorf("CaptureBodies requires a boolean")
			}
			c.captureBodies = value
		}
		return c.captureBodies, nil
	case "buildcommit":
		var value struct{ BuildCommit string }
		err := c.call(ctx, "capabilities", map[string]any{}, &value)
		return value.BuildCommit, err
	case "proxyaddress":
		var current capture.Session
		err := c.call(ctx, "status", map[string]any{}, &current)
		if c.sessionID != "" && string(current.ID) != c.sessionID {
			return nil, fmt.Errorf("capture session changed")
		}
		return current.ListenAddress, err
	case "sessionid":
		return c.sessionID, nil
	case "backend":
		if put {
			if len(args) != 1 {
				return nil, fmt.Errorf("Backend requires one value")
			}
			value, err := text(0)
			if err != nil {
				return nil, err
			}
			if value != "explicit_proxy" && value != "windows_transparent" {
				return nil, fmt.Errorf("unknown capture backend")
			}
			c.backend = value
		}
		return c.backend, nil
	case "visible":
		var request any = map[string]any{}
		if put {
			if len(args) != 1 {
				return nil, fmt.Errorf("Visible requires one boolean")
			}
			value, ok := args[0].(bool)
			if !ok {
				return nil, fmt.Errorf("Visible requires a boolean")
			}
			request = map[string]any{"value": value}
		}
		var result bool
		err := c.call(ctx, "visible", request, &result)
		return result, err
	case "attachallsessions", "attachcurrentsession":
		if len(args) != 0 {
			return nil, fmt.Errorf("unexpected arguments")
		}
		c.targets = nil
		c.path = ""
		c.scope = map[string]string{"attachallsessions": "all_sessions", "attachcurrentsession": "current_session"}[name]
		return nil, nil
	case "attachnewprocess":
		if len(args) != 1 {
			return nil, fmt.Errorf("AttachNewProcess requires one absolute executable/shortcut path")
		}
		path, err := text(0)
		if err != nil {
			return nil, err
		}
		c.path = path
		c.targets = nil
		c.scope = "launch"
		return nil, nil
	case "attachprocessbyid", "detachprocessbyid", "attachprocessbyname", "detachprocessbyname":
		if len(args) != 1 {
			return nil, fmt.Errorf("attach/detach requires one process identity")
		}
		var processes []models.ProcessInstance
		if err := c.call(ctx, "processes", map[string]any{}, &processes); err != nil {
			return nil, err
		}
		matches := []models.ProcessKey{}
		byName := strings.HasSuffix(name, "byname")
		detach := strings.HasPrefix(name, "detach")
		if byName {
			value, err := text(0)
			if err != nil {
				return nil, err
			}
			for _, p := range processes {
				if strings.EqualFold(p.Name, value) && p.Key.StartTime != "" {
					matches = append(matches, p.Key)
				}
			}
		} else {
			pid, err := integer(args[0])
			if err != nil || pid <= 0 || pid > math.MaxInt32 {
				return nil, fmt.Errorf("invalid process ID")
			}
			for _, p := range processes {
				if int64(p.Key.PID) == pid && p.Key.StartTime != "" {
					matches = append(matches, p.Key)
				}
			}
			if detach {
				for _, p := range c.targets {
					if int64(p.PID) == pid {
						matches = append(matches, p)
					}
				}
			}
		}
		if len(matches) == 0 && !detach {
			return false, nil
		}
		for _, key := range matches {
			if detach {
				next := c.targets[:0]
				for _, existing := range c.targets {
					if existing != key {
						next = append(next, existing)
					}
				}
				c.targets = next
			} else {
				found := false
				for _, existing := range c.targets {
					if existing == key {
						found = true
					}
				}
				if !found {
					c.targets = append(c.targets, key)
				}
			}
		}
		if len(c.targets) > 1024 {
			return nil, fmt.Errorf("too many process instances")
		}
		c.scope = "selected"
		c.path = ""
		return len(matches) > 0, nil
	case "start":
		if len(args) != 0 {
			return nil, fmt.Errorf("unexpected arguments")
		}
		cfg := capture.Config{CaptureBodies: c.captureBodies, Backend: c.backend, ProcessScope: c.scope, LaunchPath: c.path, AllowedProcesses: append([]models.ProcessKey(nil), c.targets...)}
		var started capture.Session
		err := c.call(ctx, "start", cfg, &started)
		if err == nil {
			c.sessionID = string(started.ID)
			c.loaded = ""
			c.loadedCleared = false
		}
		return nil, err
	case "clear":
		if len(args) != 0 {
			return nil, fmt.Errorf("unexpected arguments")
		}
		if c.loaded != "" {
			c.loadedCleared = true
			return nil, nil
		}
		if c.sessionID == "" {
			return nil, fmt.Errorf("this controller has no capture session")
		}
		return nil, c.call(ctx, "clear", map[string]any{"sessionId": c.sessionID}, nil)
	case "stop", "pause", "resume":
		if len(args) != 0 {
			return nil, fmt.Errorf("unexpected arguments")
		}
		if c.sessionID == "" {
			return nil, fmt.Errorf("this controller has no capture session")
		}
		return nil, c.call(ctx, name, map[string]any{"sessionId": c.sessionID}, nil)
	case "islogging", "ispaused":
		var current capture.Session
		err := c.call(ctx, "status", map[string]any{}, &current)
		if c.sessionID != "" && string(current.ID) != c.sessionID {
			return false, err
		}
		if name == "islogging" {
			return current.State == capture.StateRunning, err
		}
		return current.Paused, err
	case "log":
		if c.loadedCleared {
			empty := Collection{Items: []Object{}}
			if len(args) > 0 {
				return empty.Invoke(ctx, "Item", args, false)
			}
			return Typed{"ILog", empty}, nil
		}
		key := c.loaded
		if c.loaded == "" {
			var current capture.Session
			if err := c.call(ctx, "status", map[string]any{}, &current); err != nil {
				return nil, err
			}
			var stats capture.Stats
			if c.sessionID != "" {
				current.ID = capture.SessionID(c.sessionID)
			}
			if current.ID != "" {
				if err := c.call(ctx, "stats", map[string]any{"sessionId": current.ID}, &stats); err != nil {
					return nil, err
				}
			}
			key = fmt.Sprintf("%s:%s:%d:%d", current.ID, current.State, current.Generation, stats.SnapshotVersion)
		}
		if c.cachedLog == nil || key != c.cachedKey {
			var rows []models.CaptureTransaction
			if err := c.call(ctx, "legacy-log", map[string]any{"path": c.loaded, "sessionId": c.sessionID}, &rows); err != nil {
				return nil, err
			}
			items := make([]Object, len(rows))
			for i, row := range rows {
				items[i] = entryObject(row)
			}
			c.cachedLog = Typed{"ILog", Collection{items}}
			c.cachedKey = key
		}
		if len(args) > 0 {
			return c.cachedLog.Invoke(ctx, "Item", args, false)
		}
		return c.cachedLog, nil

	case "loadlog":
		if len(args) != 1 {
			return nil, fmt.Errorf("LoadLog requires a path")
		}
		path, err := text(0)
		if err != nil {
			return nil, err
		}
		if err = c.call(ctx, "open", map[string]any{"path": path}, nil); err != nil {
			return nil, err
		}
		c.loaded = path
		c.loadedCleared = false
		c.cachedLog = nil
		return nil, nil
	case "savelog":
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("SaveLog requires a path and optional named format")
		}
		path, err := text(0)
		if err != nil {
			return nil, err
		}
		format := "xml"
		if len(args) == 2 {
			format, err = namedLogType(args[1])
			if err != nil {
				return nil, err
			}
			if format == "full" {
				format = "xml"
			}
			if format == "compact" {
				format = "xml_compact"
			}
		}
		source := map[string]string{}
		if c.loaded != "" {
			source["path"] = c.loaded
		} else {
			var current capture.Session
			if err = c.call(ctx, "status", map[string]any{}, &current); err != nil {
				return nil, err
			}
			source["sessionId"] = string(current.ID)
			if c.sessionID != "" {
				source["sessionId"] = c.sessionID
			}
		}
		scope := "all"
		if c.loadedCleared {
			scope = "filtered"
		}
		return nil, c.call(ctx, "save", map[string]any{"source": source, "destination": path, "format": format, "scope": scope, "ids": []string{}}, nil)
	case "deleteallcache", "deleteallcookies", "deletecacheforrecordedurls", "deletecookiesforrecordedurls":
		if len(args) != 0 {
			return nil, fmt.Errorf("unexpected arguments")
		}
		kind := "cache"
		if strings.Contains(name, "cookies") {
			kind = "cookies"
		}
		scope := "all"
		source := map[string]string{}
		if strings.HasSuffix(name, "recordedurls") {
			scope = "recorded"
			if c.loaded != "" {
				source["path"] = c.loaded
			} else {
				var current capture.Session
				if err := c.call(ctx, "status", map[string]any{}, &current); err != nil {
					return nil, err
				}
				source["sessionId"] = string(current.ID)
				if c.sessionID != "" {
					source["sessionId"] = c.sessionID
				}
			}
		}
		var plan struct {
			ID string `json:"id"`
		}
		if err := c.call(ctx, "cache-plan", map[string]any{"kind": kind, "scope": scope, "source": source}, &plan); err != nil {
			return nil, err
		}
		var result struct{ Failed int }
		if err := c.call(ctx, "cache-delete", map[string]string{"planId": plan.ID}, &result); err != nil {
			return nil, err
		}
		if result.Failed > 0 {
			return nil, fmt.Errorf("cleanup failed for %d entries", result.Failed)
		}
		return nil, nil
	}
	return nil, fmt.Errorf("unsupported automation member %s", name)
}
func integer(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		if !math.IsNaN(v) && !math.IsInf(v, 0) && v == math.Trunc(v) && v >= math.MinInt32 && v <= math.MaxInt32 {
			return int64(v), nil
		}
	}
	return 0, fmt.Errorf("integer required")
}

type Properties map[string]any

func (p Properties) Members() []string {
	keys := make([]string, 0, len(p))
	for key := range p {
		keys = append(keys, key)
	}
	return keys
}
func (p Properties) Invoke(_ context.Context, name string, args []any, put bool) (any, error) {
	if put || len(args) != 0 {
		return nil, fmt.Errorf("read-only property")
	}
	for key, value := range p {
		if strings.EqualFold(name, key) {
			if failure, ok := value.(unavailable); ok {
				return nil, failure
			}
			return value, nil
		}
	}
	return nil, fmt.Errorf("property unavailable")
}

type Collection struct{ Items []Object }

func (Collection) Members() []string { return []string{"Count", "Item", "_NewEnum"} }
func (c Collection) Invoke(_ context.Context, name string, args []any, put bool) (any, error) {
	if put {
		return nil, fmt.Errorf("read-only collection")
	}
	switch strings.ToLower(name) {
	case "count":
		if len(args) == 0 {
			return len(c.Items), nil
		}
	case "_newenum":
		if len(args) == 0 {
			return Enumeration{c.Items}, nil
		}
	case "item":
		if len(args) == 1 {
			index, err := integer(args[0])
			if err == nil && index >= 0 && index < int64(len(c.Items)) {
				return c.Items[index], nil
			}
		}
	}
	return nil, fmt.Errorf("collection member/index unavailable")
}
func entryObject(tx models.CaptureTransaction) Object {
	process := ""
	pid := int32(0)
	if tx.Process != nil {
		process = tx.Process.Name
		pid = tx.Process.Key.PID
	}
	complete := tx.State == models.TxComplete || tx.State == models.TxFailed || tx.State == models.TxAborted
	size := any(tx.Response.BodySize)
	if tx.Response.BodyEncoding != "" {
		if tx.Response.BodyDecoded > 0 {
			size = tx.Response.BodyDecoded
		} else {
			size = unavailable("decoded content length was not captured")
		}
	}
	if !complete {
		size = unavailable("response content is still pending")
	}
	return Typed{"ILogEntry", Properties{"URL": tx.URL, "Method": tx.Method, "HTTPStatusCode": tx.StatusCode, "Result": tx.StatusText, "ProcessID": pid, "ProcessName": process, "SeqNO": tx.Sequence, "IsComplete": complete, "IsRedirect": tx.StatusCode >= 300 && tx.StatusCode < 400, "RedirectURL": tx.RedirectURL, "HasError": tx.Error != "", "ErrorMessage": tx.Error, "BytesSent": tx.Request.TransferSize, "BytesReceived": tx.Response.TransferSize, "TimeStart": automationDate(tx.StartedAt), "TimeEnd": automationDate(tx.EndedAt), "TimeDuration": int32(math.Round(tx.TotalMS)), "Request": Typed{"IRequest", messageObject(tx.Request)}, "Response": Typed{"IResponse", messageObject(tx.Response)}, "Content": Typed{"IResponseContent", Properties{"MimeType": tx.Response.ContentType, "ContentType": tx.Response.ContentType, "Size": size, "BodyStorage": tx.Response.BodyStorage}}}}
}
func messageObject(message models.HTTPMessage) Object {
	headers := make([]Object, len(message.Headers))
	for i, field := range message.Headers {
		headers[i] = Typed{"INameValue", Properties{"Name": field.Name, "Value": field.Value}}
	}
	return Properties{"Headers": Typed{"INameValues", Collection{headers}}, "ContentType": message.ContentType, "BodySize": message.BodySize, "BodyStorage": message.BodyStorage}
}

func automationDate(raw string) any {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return unavailable("timestamp was not captured")
	}
	return comDate(float64(parsed.Unix())/86400 + float64(parsed.Nanosecond())/(86400*1e9) + 25569)
}
