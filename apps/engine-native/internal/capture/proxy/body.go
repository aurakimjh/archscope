package proxy

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Body-capture tier (opt-in). Bodies stay omitted unless the session explicitly
// enables capture, because a MITM proxy holding plaintext payloads is exactly
// what the SEC-10 crash-dump exclusion preflight exists to bound. The session
// records which gate was in force, so an analyst can always tell whether a
// missing body means "not sent" or "not recorded".
const (
	// BodyStorageOmitted is declared in server.go.

	// BodyStorageInline means the whole body is present in BodyPreview.
	BodyStorageInline = "inline"
	// BodyStorageTruncated means the body exceeded the per-message cap and
	// BodyPreview holds only its leading bytes.
	BodyStorageTruncated = "truncated"
	// BodyStorageEncoded means bytes were captured but the Content-Encoding is
	// one this tier does not decode, so no readable text is claimed.
	BodyStorageEncoded = "encoded"
	// BodyStorageBinary means the payload is not valid UTF-8 text, so it is
	// reported by size only rather than as a fabricated string.
	BodyStorageBinary = "binary"

	// DefaultMaxBodyBytes bounds a single captured message body.
	DefaultMaxBodyBytes = 256 << 10
	// MaxMaxBodyBytes is the ceiling a session may raise the per-body cap to.
	MaxMaxBodyBytes = 4 << 20
)

// bodyRecorder counts every byte that passes through a message body and, when
// capture is enabled, retains at most `limit` leading bytes of it.
type bodyRecorder struct {
	io.ReadCloser
	n         int64
	limit     int
	buffer    bytes.Buffer
	truncated bool
	capture   bool
}

func newBodyRecorder(body io.ReadCloser, capture bool, limit int) *bodyRecorder {
	return &bodyRecorder{ReadCloser: body, capture: capture, limit: normalizeBodyLimit(limit)}
}

func normalizeBodyLimit(limit int) int {
	if limit <= 0 {
		return DefaultMaxBodyBytes
	}
	if limit > MaxMaxBodyBytes {
		return MaxMaxBodyBytes
	}
	return limit
}

func (r *bodyRecorder) Read(data []byte) (int, error) {
	n, err := r.ReadCloser.Read(data)
	r.n += int64(n)
	if r.capture && n > 0 {
		remaining := r.limit - r.buffer.Len()
		if remaining <= 0 {
			r.truncated = true
		} else if n > remaining {
			r.buffer.Write(data[:remaining])
			r.truncated = true
		} else {
			r.buffer.Write(data[:n])
		}
	}
	return n, err
}

// Transferred is the on-the-wire byte count, independent of whether the body
// was retained.
func (r *bodyRecorder) Transferred() int64 { return r.n }

// apply writes the captured body onto the canonical message. Decoding, text
// classification, and truncation are all reflected in BodyStorage so the UI can
// state exactly why a body is or is not readable.
func (r *bodyRecorder) apply(message *models.HTTPMessage, contentEncoding string) {
	// The recorder owns the unredacted wire bytes. Clear them as soon as the
	// canonical message has been built. This shortens the window rather than
	// closing it — the message itself holds plaintext until pipeline redaction
	// runs, which is the exposure the SEC-10 preflight bounds.
	defer r.clearCaptured()
	message.BodySize = r.n
	message.TransferSize = r.n
	message.BodyEncoding = contentEncoding
	if !r.capture {
		message.BodyStorage = BodyStorageOmitted
		return
	}
	if r.n == 0 {
		message.BodyStorage = BodyStorageInline
		return
	}

	raw := r.buffer.Bytes()
	decoded, decodedTruncated, ok := decodeBody(raw, contentEncoding, r.truncated, r.limit)
	if !ok {
		message.BodyStorage = BodyStorageEncoded
		return
	}
	defer zeroBytes(decoded)
	message.BodyDecoded = int64(len(decoded))
	if !utf8.Valid(decoded) {
		message.BodyStorage = BodyStorageBinary
		return
	}
	message.BodyPreview = string(decoded)
	if r.truncated || decodedTruncated {
		message.BodyStorage = BodyStorageTruncated
		return
	}
	message.BodyStorage = BodyStorageInline
}

// decodeBody inflates the encodings this tier understands. A body that was
// truncated mid-stream cannot be decoded as a whole, so a partial compressed
// payload is reported as `encoded` rather than as a corrupt decode.
func decodeBody(raw []byte, contentEncoding string, truncated bool, limit int) ([]byte, bool, bool) {
	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	switch encoding {
	case "", "identity":
		return raw, truncated, true
	case "gzip", "x-gzip":
		if truncated {
			return nil, false, false
		}
		reader, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, false, false
		}
		defer reader.Close()
		return readBounded(reader, limit)
	case "deflate":
		if truncated {
			return nil, false, false
		}
		reader, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, false, false
		}
		defer reader.Close()
		return readBounded(reader, limit)
	default:
		// br, zstd, and anything else: bytes were seen but no readable text is
		// claimed for them.
		return nil, false, false
	}
}

func readBounded(reader io.Reader, limit int) ([]byte, bool, bool) {
	limit = normalizeBodyLimit(limit)
	decoded, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, false, false
	}
	if len(decoded) > limit {
		zeroBytes(decoded[limit:])
		return decoded[:limit], true, true
	}
	return decoded, false, true
}

func (r *bodyRecorder) clearCaptured() {
	zeroBytes(r.buffer.Bytes())
	r.buffer.Reset()
}

func zeroBytes(data []byte) {
	for index := range data {
		data[index] = 0
	}
}
