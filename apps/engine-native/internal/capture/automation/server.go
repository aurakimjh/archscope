package automation

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/netpolicy"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Call struct {
	ID      string          `json:"id"`
	Command string          `json:"command"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type Reply struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}
type Descriptor struct {
	Version int    `json:"schemaVersion"`
	URL     string `json:"url"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
}
type Executor func(context.Context, Call) (any, error)
type cached struct {
	hash  [32]byte
	reply Reply
}
type Server struct {
	descriptor Descriptor
	path       string
	listener   net.Listener
	server     *http.Server
	execute    Executor
	mu         sync.Mutex
	cache      map[string]cached
	order      []string
}

func Start(path string, execute Executor) (*Server, error) {
	if err := netpolicy.RequireOnline("automation.listen"); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("automation descriptor must be absolute")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		listener.Close()
		return nil, err
	}
	s := &Server{descriptor: Descriptor{Version: 1, URL: "http://" + listener.Addr().String() + "/v1/control", Token: hex.EncodeToString(secret), PID: os.Getpid()}, path: path, listener: listener, execute: execute, cache: map[string]cached{}}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("create automation descriptor: %w", err)
	}
	cleanup := func() { file.Close(); os.Remove(path); listener.Close() }
	if err = ownerOnly(path); err != nil {
		cleanup()
		return nil, err
	}
	raw, _ := json.Marshal(s.descriptor)
	if _, err = file.Write(raw); err != nil {
		cleanup()
		return nil, err
	}
	if err = file.Sync(); err != nil {
		cleanup()
		return nil, err
	}
	if err = file.Close(); err != nil {
		os.Remove(path)
		listener.Close()
		return nil, err
	}
	s.server = &http.Server{Handler: http.HandlerFunc(s.handle), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 16384}
	go func() { _ = s.server.Serve(listener) }()
	return s, nil
}
func (s *Server) Close() error {
	err := s.server.Close()
	raw, readErr := readDescriptor(s.path)
	var current Descriptor
	if readErr == nil && json.Unmarshal(raw, &current) == nil && current.Token == s.descriptor.Token {
		err2 := os.Remove(s.path)
		if err == nil {
			err = err2
		}
	}
	return err
}
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(remote).IsLoopback() || r.Host != s.listener.Addr().String() || r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" {
		http.Error(w, "local controller only", 403)
		return
	}
	expected := "Bearer " + s.descriptor.Token
	got := r.Header.Get("Authorization")
	if len(got) != len(expected) || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if r.Method != "POST" || r.URL.Path != "/v1/control" || err != nil || media != "application/json" {
		http.Error(w, "POST JSON control request required", 400)
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var call Call
	if decoder.Decode(&call) != nil || decoder.Decode(new(any)) != io.EOF || call.ID == "" || len(call.ID) > 128 {
		http.Error(w, "invalid control request", 400)
		return
	}
	// Serialize mutating commands and cache bounded replies. Retries with the same
	// ID do not start/stop/save twice; different content under an ID is rejected.
	s.mu.Lock()
	defer s.mu.Unlock()
	// A request may disconnect while waiting behind another mutation. It has
	// not executed yet, so do not consume its retry ID or mutate desktop state.
	if err := r.Context().Err(); err != nil {
		http.Error(w, "control request canceled before execution", 408)
		return
	}

	raw, _ := json.Marshal(call)
	hash := sha256.Sum256(raw)
	write := func(reply Reply) { _ = json.NewEncoder(w).Encode(reply) }
	if previous, ok := s.cache[call.ID]; ok {
		if previous.hash != hash {
			write(Reply{ID: call.ID, Error: "request ID reused with different content"})
		} else {
			write(previous.reply)
		}
		return
	}
	reply := Reply{ID: call.ID}
	value, err := s.execute(r.Context(), call)
	if err != nil {
		reply.Error = err.Error()
	} else {
		reply.Result, err = json.Marshal(value)
		if err != nil {
			reply.Error = "command result cannot be encoded"
		} else if len(reply.Result) > 8<<20 {
			reply.Result = nil
			reply.Error = "command completed but result exceeds 8 MiB; fetch a smaller page"
		} else {
			reply.OK = true
		}
	}
	switch call.Command {
	case "attach", "detach", "start", "pause", "resume", "clear", "stop", "save", "open", "cache-delete":
		if len(reply.Result) > 256<<10 {
			reply.Result = nil
			reply.OK = false
			reply.Error = "command completed; retrieve its result with a read command"
		}
		s.cache[call.ID] = cached{hash: hash, reply: reply}
		s.order = append(s.order, call.ID)
		if len(s.order) > 128 {
			delete(s.cache, s.order[0])
			s.order = s.order[1:]
		}
	}
	write(reply)
}

func readDescriptor(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return nil, fmt.Errorf("invalid descriptor")
	}
	raw, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return nil, err
	}
	if len(raw) > 4096 {
		return nil, fmt.Errorf("invalid descriptor")
	}
	return raw, nil
}

// Invoke rejects non-loopback descriptors and never forwards local credentials
// through redirects or an environment proxy.
func Invoke(ctx context.Context, path string, call Call) (Reply, error) {
	if err := netpolicy.RequireOnline("automation.invoke"); err != nil {
		return Reply{}, err
	}
	raw, err := readDescriptor(path)
	if err != nil {
		return Reply{}, err
	}
	if len(raw) > 4096 {
		return Reply{}, fmt.Errorf("invalid descriptor")
	}
	var d Descriptor
	if json.Unmarshal(raw, &d) != nil || d.Version != 1 || len(d.Token) != 64 || !strings.HasPrefix(d.URL, "http://127.0.0.1:") {
		return Reply{}, fmt.Errorf("invalid descriptor")
	}
	target, parseErr := url.Parse(d.URL)
	if parseErr != nil || target.Scheme != "http" || target.Hostname() != "127.0.0.1" || target.User != nil || target.Path != "/v1/control" || target.RawQuery != "" || target.Fragment != "" {
		return Reply{}, fmt.Errorf("invalid control URL")
	}
	port, parseErr := strconv.Atoi(target.Port())
	if parseErr != nil || port < 1 || port > 65535 || target.Host != net.JoinHostPort("127.0.0.1", target.Port()) {
		return Reply{}, fmt.Errorf("invalid control port")
	}
	body, _ := json.Marshal(call)
	req, err := http.NewRequestWithContext(ctx, "POST", d.URL, bytes.NewReader(body))
	if err != nil {
		return Reply{}, err
	}
	req.Header.Set("Authorization", "Bearer "+d.Token)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return Reply{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Reply{}, fmt.Errorf("controller returned HTTP %d", resp.StatusCode)
	}
	var reply Reply
	err = json.NewDecoder(io.LimitReader(resp.Body, 9<<20)).Decode(&reply)
	return reply, err
}
