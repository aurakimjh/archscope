// Package threaddump ports archscope_engine.parsers.thread_dump.registry.
//
// Each plugin handles one source format (Java jstack, Go goroutine
// dump, Python py-spy / faulthandler, Node.js diagnostic report,
// .NET clrstack, jcmd JSON). The registry probes the first 4KB of
// every input file, asks plugins CanParse(head), and dispatches to
// the first match — or to an explicitly requested format via
// ParseOptions.FormatOverride.
//
// A bundle is one (file, snapshots) pair. The multi-dump pipeline
// rejects inputs whose detected source_format values disagree because
// mixing dumps from different runtimes makes the persistence findings
// (T-191) meaningless. The caller can opt out by passing an explicit
// FormatOverride — the override is honored uniformly and the
// mixed-format check is skipped.
package threaddump

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

// DetectHeadBytes mirrors Python `DETECT_HEAD_BYTES = 4096`. Plugins
// receive at most this many decoded bytes when the registry asks
// CanParse — enough to surface the runtime-specific header (Full
// thread dump …, goroutine 1 [running], etc.) without paying the
// cost of reading the whole file.
const DetectHeadBytes = 4096

// Plugin is the contract every concrete parser plugin must satisfy.
// FormatID is the stable identifier used by FormatOverride and surfaced
// on ThreadDumpBundle.SourceFormat. Language is the runtime label
// ("java", "go", "python", ...).
//
// CanParse is called with at most DetectHeadBytes of the decoded head
// — implementations should be cheap (regex / substring sniff, not full
// parse). Parse reads the whole file and returns one bundle.
type Plugin interface {
	FormatID() string
	Language() string
	CanParse(head string) bool
	Parse(path string) (models.ThreadDumpBundle, error)
}

// MultiBundlePlugin is the optional capability for plugins that emit
// more than one bundle per file (e.g. concatenated jstack dumps). The
// registry detects this via type assertion and prefers ParseAll when
// available.
type MultiBundlePlugin interface {
	Plugin
	ParseAll(path string) ([]models.ThreadDumpBundle, error)
}

// UnknownFormatError signals "no registered plugin claimed the input".
// Source is the path or format-id the caller passed; HeadPreview is
// the first 200 decoded bytes (truncated) so the error message points
// at the runtime-recognisable text.
type UnknownFormatError struct {
	Source      string
	HeadPreview string
}

func (e *UnknownFormatError) Error() string {
	preview := e.HeadPreview
	if len(preview) > 200 {
		preview = preview[:200]
	}
	return fmt.Sprintf(
		"No thread-dump parser plugin recognized %q. Header preview: %q",
		e.Source, preview,
	)
}

// MixedFormatError signals "this multi-dump bundle resolved to more
// than one source format". Formats maps source-file-path -> detected
// format-id so the renderer can show which file was the outlier.
type MixedFormatError struct {
	Formats map[string]string
}

func (e *MixedFormatError) Error() string {
	keys := make([]string, 0, len(e.Formats))
	for k := range e.Formats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, e.Formats[k]))
	}
	return fmt.Sprintf(
		"Multi-dump input mixes incompatible source formats: %s. "+
			"Pass --format to force a single parser if you intentionally "+
			"want to coerce one of them.",
		strings.Join(parts, ", "),
	)
}

// Registry holds plugins. The zero value is unusable; call NewRegistry.
// Plugins are dispatched in registration order — earlier registrations
// win on header conflicts, matching Python list-iteration semantics.
type Registry struct {
	plugins []Plugin
}

// NewRegistry constructs an empty registry. Plugins are typically
// registered at process start (CLI / web server bootstrap) — see
// `archscope-engine` for the canonical wiring.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register appends a plugin. Duplicate FormatID values are allowed
// (Python's protocol behaviour) — Get returns the first match.
func (r *Registry) Register(p Plugin) {
	r.plugins = append(r.plugins, p)
}

// Plugins returns a copy of the plugin list. Callers can iterate
// without risk of concurrent mutation by Register calls.
func (r *Registry) Plugins() []Plugin {
	out := make([]Plugin, len(r.plugins))
	copy(out, r.plugins)
	return out
}

// Get looks up a plugin by FormatID. Returns *UnknownFormatError when
// no registered plugin matches, with HeadPreview empty (Python parity:
// the format-id-not-found case has no head context).
func (r *Registry) Get(formatID string) (Plugin, error) {
	for _, p := range r.plugins {
		if p.FormatID() == formatID {
			return p, nil
		}
	}
	return nil, &UnknownFormatError{Source: formatID}
}

// DetectFormat returns the first plugin whose CanParse matches the
// head sample, or nil when none match.
func (r *Registry) DetectFormat(head string) Plugin {
	for _, p := range r.plugins {
		if p.CanParse(head) {
			return p
		}
	}
	return nil
}

// ParseOptions captures the dispatch knobs ParseOne / ParseMany honor.
// FormatOverride bypasses head sniffing and forces a specific plugin;
// when set, ParseMany also skips the MixedFormatError check.
type ParseOptions struct {
	FormatOverride string
	DumpIndex      int
	DumpLabel      string
	// Labels lets ParseMany override the per-file label without forcing
	// every file through DumpLabel. Maps absolute path -> label.
	Labels map[string]string
}

// ParseOne sniffs the format (or honors FormatOverride), parses the
// file, and returns one bundle. DumpIndex / DumpLabel are stamped onto
// the bundle after the plugin returns so the plugin doesn't need to
// know about its position in a multi-file pipeline.
func (r *Registry) ParseOne(path string, opts ParseOptions) (models.ThreadDumpBundle, error) {
	plugin, err := r.selectPlugin(path, opts.FormatOverride)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}
	bundle, err := plugin.Parse(path)
	if err != nil {
		return bundle, err
	}
	bundle.DumpIndex = opts.DumpIndex
	label := opts.DumpLabel
	if label == "" {
		label = filepath.Base(path)
	}
	bundle.DumpLabel = &label
	return bundle, nil
}

// ParseMany parses multiple dumps and rejects mixed source formats by
// default. Honours FormatOverride uniformly — when the caller passes
// an explicit format every file is parsed with that plugin and the
// mixed-format check is skipped.
//
// DumpIndex starts at 0 and increments across all returned bundles
// (matching Python `next_dump_index`). When a plugin emits multiple
// bundles per file via the optional MultiBundlePlugin capability, the
// dump_label gets a "#N" suffix to disambiguate.
func (r *Registry) ParseMany(paths []string, opts ParseOptions) ([]models.ThreadDumpBundle, error) {
	bundles := make([]models.ThreadDumpBundle, 0, len(paths))
	nextDumpIndex := 0
	for _, path := range paths {
		plugin, err := r.selectPlugin(path, opts.FormatOverride)
		if err != nil {
			return nil, err
		}
		parsed, err := pluginBundles(plugin, path)
		if err != nil {
			return nil, err
		}
		baseLabel := path
		if opts.Labels != nil {
			if label, ok := opts.Labels[path]; ok {
				baseLabel = label
			} else {
				baseLabel = filepath.Base(path)
			}
		} else {
			baseLabel = filepath.Base(path)
		}
		multiFile := len(parsed) > 1
		for localIndex, bundle := range parsed {
			bundle.DumpIndex = nextDumpIndex
			if bundle.DumpLabel == nil || *bundle.DumpLabel == filepath.Base(path) {
				suffix := ""
				if multiFile {
					suffix = fmt.Sprintf("#%d", localIndex+1)
				}
				label := baseLabel + suffix
				bundle.DumpLabel = &label
			}
			nextDumpIndex++
			bundles = append(bundles, bundle)
		}
	}

	if opts.FormatOverride == "" {
		formats := map[string]string{}
		seen := map[string]struct{}{}
		for _, b := range bundles {
			formats[b.SourceFile] = b.SourceFormat
			seen[b.SourceFormat] = struct{}{}
		}
		if len(seen) > 1 {
			return nil, &MixedFormatError{Formats: formats}
		}
	}
	return bundles, nil
}

// selectPlugin picks the plugin to use for one file. FormatOverride
// shortcuts the head sniff; otherwise we read up to DetectHeadBytes,
// detect the encoding, decode, and ask each plugin in order.
func (r *Registry) selectPlugin(path, formatOverride string) (Plugin, error) {
	if formatOverride != "" {
		return r.Get(formatOverride)
	}
	head, err := readHead(path)
	if err != nil {
		return nil, err
	}
	plugin := r.DetectFormat(head)
	if plugin == nil {
		return nil, &UnknownFormatError{Source: path, HeadPreview: head}
	}
	return plugin, nil
}

// readHead reads up to DetectHeadBytes from path, detects the encoding
// using textio's universal fallback chain, and decodes to UTF-8 Go
// string. Mirrors Python `_read_head` byte-for-byte (errors=replace
// semantics — latin-1 fallback always succeeds in textio).
func readHead(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, DetectHeadBytes)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	raw := buf[:n]
	encoding, err := textio.DetectFromBytes(raw, nil)
	if err != nil {
		return "", err
	}
	return textio.DecodeBytes(raw, encoding)
}

// pluginBundles invokes plugin.ParseAll if available, else falls back
// to a single Parse call wrapped in a slice. Mirrors Python
// `_parse_path_bundles`.
func pluginBundles(plugin Plugin, path string) ([]models.ThreadDumpBundle, error) {
	if multi, ok := plugin.(MultiBundlePlugin); ok {
		bundles, err := multi.ParseAll(path)
		if err != nil {
			return nil, err
		}
		if len(bundles) > 0 {
			return bundles, nil
		}
	}
	bundle, err := plugin.Parse(path)
	if err != nil {
		return nil, err
	}
	return []models.ThreadDumpBundle{bundle}, nil
}

// DefaultRegistry is the package-level registry plugin packages
// register themselves into via init(). Mirrors Python's
// DEFAULT_REGISTRY singleton at the bottom of registry.py.
var DefaultRegistry = NewRegistry()
