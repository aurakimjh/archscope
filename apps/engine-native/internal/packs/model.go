// Package packs validates immutable data-only extensions. It never loads code,
// executes templates/SQL/scripts, discovers signers from archives or fetches URLs.
package packs

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const Format = "1.0.0"
const Interpreter = "1.0.0"
const AppVersion = "0.3.5"
const MaxArchiveBytes = 8 << 20
const MaxExpandedBytes = 16 << 20
const MaxFileBytes = 4 << 20
const MaxFiles = 128
const MaxManifestBytes = 256 << 10
const MaxVersions = 64
const MaxActiveBytes = 32 << 20

var ID = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,95}$`)
var hashRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string       { return e.Code + ": " + e.Message }
func problem(code, msg string) error { return &Error{code, msg} }
func Hash(b []byte) string           { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func Strict(b []byte, v any) error {
	if e := wirejson.Decode(b, new(any)); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	d.UseNumber()
	return d.Decode(v)
}

type Range struct {
	Min    string `json:"min"`
	Before string `json:"before"`
}

func (r Range) accepts(v string) bool {
	return (resultcontract.Range{Min: r.Min, Before: r.Before}).Accepts(v)
}

type File struct {
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
	Role   string `json:"role"`
}
type Dependency struct {
	ID      string `json:"id"`
	Version Range  `json:"version"`
	SHA256  string `json:"sha256"`
}
type Fixture struct {
	Mapping  string `json:"mapping"`
	Input    string `json:"input"`
	Expected string `json:"expected"`
	Accept   bool   `json:"accept"`
}
type Manifest struct {
	Format       string          `json:"pack_format"`
	ID           string          `json:"id"`
	Version      string          `json:"version"`
	Signer       string          `json:"signer"`
	App          Range           `json:"app"`
	Adapter      Range           `json:"adapter"`
	Extension    Range           `json:"extension"`
	Dependencies []Dependency    `json:"dependencies"`
	License      string          `json:"license"`
	LicenseFile  string          `json:"license_file"`
	Changes      string          `json:"changes"`
	Files        map[string]File `json:"files"`
	Mappings     []string        `json:"mappings"`
	Fixtures     []Fixture       `json:"fixtures"`
}
type Signature struct {
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}
type Key struct {
	ID          string `json:"id"`
	Public      string `json:"public_key"`
	ManageTrust bool   `json:"manage_trust"`
	Revoked     bool   `json:"revoked"`
}

func (k Key) public() (ed25519.PublicKey, error) {
	b, e := base64.StdEncoding.Strict().DecodeString(k.Public)
	if e != nil || len(b) != ed25519.PublicKeySize || !ID.MatchString(k.ID) {
		return nil, problem("PACK_TRUST_INVALID", "invalid registered public key")
	}
	return ed25519.PublicKey(b), nil
}

type Ref struct {
	ID            string `json:"id"`
	Version       string `json:"version"`
	SHA256        string `json:"sha256"`
	ArchiveSHA256 string `json:"archive_sha256"`
	Signer        string `json:"signer"`
	Origin        string `json:"origin"`
	Scope         string `json:"scope"`
	Bytes         int    `json:"bytes"`
}
type Revision struct {
	Manifest  Manifest
	Raw       []byte
	Signature Signature
	Files     map[string][]byte
	Mappings  []Mapping
	Ref       Ref
}
type UnavailablePack struct {
	Pack       Ref    `json:"pack"`
	Code       string `json:"code"`
	Dependency string `json:"dependency,omitempty"`
}
type ScopeProblem struct {
	Scope string `json:"scope"`
	Code  string `json:"code"`
}
type Status struct {
	State
	Unavailable []UnavailablePack `json:"unavailable"`
	Problems    []ScopeProblem    `json:"problems"`
}
type Snapshot struct {
	Unavailable   []UnavailablePack `json:"unavailable,omitempty"`
	Revision      string            `json:"revision"`
	TrustSequence string            `json:"trust_sequence"`
	Refs          []Ref             `json:"packs"`
	Choices       map[string]string `json:"choices"`
	Mappings      []Mapping         `json:"-"`
}
type InstallOptions struct {
	AllowUnsigned bool   `json:"allow_unsigned"`
	Scope         string `json:"scope"`
}

func scopeOK(s string) bool {
	if s == "application" || s == "user" {
		return true
	}
	return strings.HasPrefix(s, "case:") && regexp.MustCompile(`^case:case-[0-9a-f]{32}$`).MatchString(s)
}
func safeName(p string) bool {
	if len(p) == 0 || len(p) > 256 || strings.Count(p, "/") > 7 || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\:\x00") {
		return false
	}
	for _, c := range p {
		if c > 127 || !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._-/", c)) {
			return false
		}
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." || strings.HasSuffix(s, ".") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
		if base == "CON" || base == "NUL" || base == "AUX" || base == "PRN" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			return false
		}
	}
	return true
}
func (m Manifest) validate() error {
	if m.Format != Format || !ID.MatchString(m.ID) || !m.App.accepts(AppVersion) || !m.Adapter.accepts(Interpreter) || !m.Extension.accepts(Interpreter) {
		return problem("PACK_INCOMPATIBLE", "unsupported pack/app/adapter/extension contract")
	}
	if _, ok := resultcontract.Version(m.Version); !ok {
		return problem("PACK_INVALID", "pack version must be semver")
	}
	if len(m.Files) < 3 || len(m.Files) > MaxFiles-2 || len(m.Mappings) == 0 || len(m.Mappings) > 16 || len(m.Fixtures) == 0 || len(m.Fixtures) > 64 || len(m.Dependencies) > 16 {
		return problem("PACK_BUDGET_EXCEEDED", "pack cardinality limit")
	}
	licenses := map[string]bool{"CC0-1.0": true, "MIT": true, "Apache-2.0": true, "OFL-1.1": true}
	if !licenses[m.License] || m.Files[m.LicenseFile].Role != "license" || m.Files[m.Changes].Role != "changes" {
		return problem("PACK_INVALID", "declared license, license text and changes required")
	}
	total := 0
	for name, file := range m.Files {
		if name == "manifest.json" || name == "signature.json" || !safeName(name) || file.Size < 0 || file.Size > MaxFileBytes || !hashRE.MatchString(file.SHA256) {
			return problem("PACK_INVALID", "invalid data file declaration")
		}
		total += file.Size
		role := file.Role
		ext := strings.ToLower(name[strings.LastIndex(name, ".")+1:])
		if !(role == "mapping" || role == "schema" || role == "fixture" || role == "license" || role == "changes") || !(ext == "json" || ext == "jsonl" || ext == "txt" || ext == "md") {
			return problem("PACK_EXECUTABLE_FORBIDDEN", "only declared bounded data files are permitted")
		}
	}
	if total > MaxExpandedBytes {
		return problem("PACK_BUDGET_EXCEEDED", "expanded pack size")
	}
	names := map[string]bool{}
	for _, p := range m.Mappings {
		if m.Files[p].Role != "mapping" || names[p] {
			return problem("PACK_INVALID", "missing or duplicate mapping")
		}
		names[p] = true
	}
	for _, d := range m.Dependencies {
		if !ID.MatchString(d.ID) || d.ID == m.ID || !d.Version.accepts(d.Version.Min) || !hashRE.MatchString(d.SHA256) {
			return problem("PACK_DEPENDENCY_INVALID", "invalid dependency contract")
		}
	}
	return nil
}
func nextRevision(v string) (string, error) {
	n, e := strconv.ParseUint(v, 10, 64)
	if e != nil || n == ^uint64(0) || strconv.FormatUint(n, 10) != v {
		return "", fmt.Errorf("invalid revision")
	}
	return strconv.FormatUint(n+1, 10), nil
}

func validStoredRef(ref Ref) bool {
	_, versionOK := resultcontract.Version(ref.Version)
	if !ID.MatchString(ref.ID) || !versionOK || ref.Bytes <= 0 || ref.Bytes > MaxExpandedBytes {
		return false
	}
	if ref.Origin == "signed" {
		return ref.Scope == "application" && ID.MatchString(ref.Signer)
	}
	return ref.Origin == "locally_authored" && ref.Scope != "application" && ref.Signer == ""
}
