package httpcapture

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

const (
	DefaultMaxEntries            = 100_000
	DefaultMaxBytes              = int64(256 << 20)
	DefaultMaxStringBytes        = 16 << 20
	DefaultMaxBodyBytes          = 10 << 20
	DefaultMaxDepth              = 256
	DefaultMaxFields             = 2_000_000
	DefaultMaxDecompressionRatio = 1000
)

type inputInfo struct {
	compressedBytes int64
	gzip            bool
}

func normalizeOptions(opts Options) Options {
	opts.Format = firstNonEmpty(opts.Format, FormatAuto)
	opts.MaxEntries = boundedInt(opts.MaxEntries, DefaultMaxEntries)
	opts.MaxBytes = boundedInt64(opts.MaxBytes, DefaultMaxBytes)
	opts.MaxStringBytes = boundedInt(opts.MaxStringBytes, DefaultMaxStringBytes)
	opts.MaxBodyBytes = boundedInt(opts.MaxBodyBytes, DefaultMaxBodyBytes)
	opts.MaxDepth = boundedInt(opts.MaxDepth, DefaultMaxDepth)
	opts.MaxFields = boundedInt(opts.MaxFields, DefaultMaxFields)
	opts.MaxDecompressionRatio = boundedInt(opts.MaxDecompressionRatio, DefaultMaxDecompressionRatio)
	return opts
}

func readBoundedInput(path string, opts Options) ([]byte, inputInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, inputInfo{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, inputInfo{}, err
	}
	if info.Size() > opts.MaxBytes {
		return nil, inputInfo{compressedBytes: info.Size()}, fmt.Errorf("%w: input bytes %d exceed limit %d", ErrResourceLimit, info.Size(), opts.MaxBytes)
	}
	head := make([]byte, 2)
	n, err := io.ReadFull(file, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, inputInfo{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, inputInfo{}, err
	}
	isGzip := n == 2 && head[0] == 0x1f && head[1] == 0x8b
	var reader io.Reader = file
	if isGzip {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, inputInfo{}, fmt.Errorf("%w: invalid gzip stream", ErrStructuralHAR)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}
	limited := &io.LimitedReader{R: reader, N: opts.MaxBytes + 1}
	payload, err := io.ReadAll(limited)
	if err != nil {
		return nil, inputInfo{}, err
	}
	if int64(len(payload)) > opts.MaxBytes {
		return nil, inputInfo{compressedBytes: info.Size(), gzip: isGzip}, fmt.Errorf("%w: decompressed bytes exceed limit %d", ErrResourceLimit, opts.MaxBytes)
	}
	if isGzip && info.Size() > 0 && int64(len(payload)) > info.Size()*int64(opts.MaxDecompressionRatio) {
		return nil, inputInfo{compressedBytes: info.Size(), gzip: true}, fmt.Errorf("%w: gzip expansion ratio exceeds limit %d", ErrResourceLimit, opts.MaxDecompressionRatio)
	}
	payload = bytes.TrimPrefix(payload, []byte{0xef, 0xbb, 0xbf})
	return payload, inputInfo{compressedBytes: info.Size(), gzip: isGzip}, nil
}

func preflightJSON(payload []byte, opts Options) error {
	depth := 0
	fields := 0
	inString := false
	escaped := false
	stringBytes := 0
	for _, b := range payload {
		if inString {
			if escaped {
				escaped = false
				stringBytes++
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				inString = false
				continue
			}
			stringBytes++
			if stringBytes > opts.MaxStringBytes {
				return fmt.Errorf("%w: JSON string exceeds limit %d", ErrResourceLimit, opts.MaxStringBytes)
			}
			continue
		}
		switch b {
		case '"':
			inString = true
			stringBytes = 0
		case '{', '[':
			depth++
			if depth > opts.MaxDepth {
				return fmt.Errorf("%w: JSON depth exceeds limit %d", ErrResourceLimit, opts.MaxDepth)
			}
		case '}', ']':
			depth--
		case ':':
			fields++
			if fields > opts.MaxFields {
				return fmt.Errorf("%w: JSON field count exceeds limit %d", ErrResourceLimit, opts.MaxFields)
			}
		}
	}
	return nil
}

func boundedInt(value, maximum int) int {
	if value <= 0 || value > maximum {
		return maximum
	}
	return value
}

func boundedInt64(value, maximum int64) int64 {
	if value <= 0 || value > maximum {
		return maximum
	}
	return value
}
