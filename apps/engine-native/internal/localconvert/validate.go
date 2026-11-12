package localconvert

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"unicode/utf8"
)

const maxScalarBytes = 1 << 20
const maxJSONDepth = 256

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.reader.Read(b)
}

// A bounded lexical pass prevents Decoder.Token from allocating giant scalar
// strings. The second pass validates syntax/shape without retaining event trees.
func validateJSON(ctx context.Context, path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	r := bufio.NewReader(contextReader{ctx, f})
	inString, escaped := false, false
	scalar := 0
	for {
		c, n, err := r.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if c == utf8.RuneError && n == 1 {
			return fail("CONVERTER_OUTPUT_INVALID")
		}
		if inString {
			scalar += n
			if scalar > maxScalarBytes {
				return fail("CONVERTER_JSON_LIMIT")
			}
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
		} else if c == '"' {
			inString = true
			scalar = 0
		} else {
			switch c {
			case ' ', '\t', '\r', '\n', '{', '}', '[', ']', ':', ',':
				scalar = 0
			default:
				scalar += n
				if scalar > maxScalarBytes {
					return fail("CONVERTER_JSON_LIMIT")
				}
			}
		}
	}
	if _, e = f.Seek(0, io.SeekStart); e != nil {
		return e
	}
	d := json.NewDecoder(contextReader{ctx, f})
	d.UseNumber()
	shape, e := readShape(ctx, d, 0)
	if e != nil {
		return e
	}
	if _, e = d.Token(); e != io.EOF {
		return fail("CONVERTER_OUTPUT_INVALID")
	}
	if shape.kind != '{' || shape.recordingSeen && !shape.recordingEvents || !shape.recordingSeen && !shape.eventsArray {
		return fail("CONVERTER_OUTPUT_INVALID")
	}
	return nil
}

type jsonShape struct {
	kind                                        byte
	eventsArray, recordingSeen, recordingEvents bool
}

func readShape(ctx context.Context, d *json.Decoder, depth int) (jsonShape, error) {
	if e := ctx.Err(); e != nil {
		return jsonShape{}, e
	}
	if depth > maxJSONDepth {
		return jsonShape{}, fail("CONVERTER_JSON_LIMIT")
	}
	token, e := d.Token()
	if e != nil {
		if ctx.Err() != nil {
			return jsonShape{}, ctx.Err()
		}
		return jsonShape{}, fail("CONVERTER_OUTPUT_INVALID")
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return jsonShape{kind: 'v'}, nil
	}
	s := jsonShape{kind: byte(delim)}
	switch delim {
	case '{':
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return s, fail("CONVERTER_OUTPUT_INVALID")
			}
			name, ok := key.(string)
			if !ok {
				return s, fail("CONVERTER_OUTPUT_INVALID")
			}
			isEvents, isRecording := name == "events", name == "recording"
			value, e := readShape(ctx, d, depth+1)
			if e != nil {
				return s, e
			}
			if isEvents {
				s.eventsArray = value.kind == '['
			}
			if isRecording {
				s.recordingSeen = true
				s.recordingEvents = value.kind == '{' && value.eventsArray
			}
		}
	case '[':
		for d.More() {
			if _, e = readShape(ctx, d, depth+1); e != nil {
				return s, e
			}
		}
	default:
		return s, fail("CONVERTER_OUTPUT_INVALID")
	}
	closing, e := d.Token()
	if e != nil || closing != json.Delim(map[json.Delim]json.Delim{'{': '}', '[': ']'}[delim]) {
		return s, fail("CONVERTER_OUTPUT_INVALID")
	}
	return s, nil
}
