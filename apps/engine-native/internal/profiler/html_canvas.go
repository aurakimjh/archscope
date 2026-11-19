package profiler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Decode data literals only; never execute JavaScript from imported HTML.
// Wire formats: async-profiler v2.9/v3.0 src/res/flame.html and flameGraph.cpp.
var (
	canvasPoolRE   = regexp.MustCompile(`\b(?:const|let|var)\s+cpool\s*=\s*\[`)
	canvasCallRE   = regexp.MustCompile(`(?m)^[\t ]*([fun])\(([0-9][^\r\n]*)\);?[\t ]*\r?$`)
	canvasUnpackRE = regexp.MustCompile(`\bunpack\s*\(\s*cpool\s*\)\s*;`)
)

type canvasLiteral struct {
	text   string
	quoted bool
}

// canvasLiterals consumes a comma-separated list of strings/decimal integers.
// The terminator is consumed, so ']' or ')' in a quoted name stays literal.
func canvasLiterals(s string, end byte) ([]canvasLiteral, int, error) {
	var out []canvasLiteral
	i := 0
	for {
		for i < len(s) && strings.ContainsRune(" \t\r\n", rune(s[i])) {
			i++
		}
		if i == len(s) {
			return nil, 0, fmt.Errorf("unterminated canvas data")
		}
		if s[i] == end {
			return out, i + 1, nil
		}
		v := canvasLiteral{}
		if s[i] == '\'' || s[i] == '"' {
			v.quoted = true
			quote := s[i]
			i++
			var b strings.Builder
			for {
				if i == len(s) {
					return nil, 0, fmt.Errorf("unterminated canvas string")
				}
				if s[i] == quote {
					i++
					break
				}
				if s[i] == '\\' && i+1 < len(s) && s[i+1] == '/' {
					b.WriteByte('/')
					i += 2
					continue
				}
				// JavaScript represents non-BMP escapes as UTF-16 pairs.
				if strings.HasPrefix(s[i:], `\u`) && i+6 <= len(s) {
					hi, err := strconv.ParseUint(s[i+2:i+6], 16, 16)
					if err == nil && hi >= 0xd800 && hi <= 0xdbff && i+12 <= len(s) && s[i+6:i+8] == `\u` {
						lo, err := strconv.ParseUint(s[i+8:i+12], 16, 16)
						if err == nil && lo >= 0xdc00 && lo <= 0xdfff {
							b.WriteRune(utf16.DecodeRune(rune(hi), rune(lo)))
							i += 12
							continue
						}
					}
				}
				r, _, tail, err := strconv.UnquoteChar(s[i:], quote)
				if err != nil {
					return nil, 0, fmt.Errorf("invalid canvas string: %w", err)
				}
				i = len(s) - len(tail)
				b.WriteRune(r)
			}
			v.text = b.String()
		} else {
			start := i
			if s[i] == '-' {
				i++
			}
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			if start == i || s[start:i] == "-" {
				return nil, 0, fmt.Errorf("canvas data must contain literals")
			}
			v.text = s[start:i]
		}
		out = append(out, v)
		for i < len(s) && strings.ContainsRune(" \t\r\n", rune(s[i])) {
			i++
		}
		if i < len(s) && s[i] == ',' {
			i++
			continue
		}
		if i < len(s) && s[i] == end {
			return out, i + 1, nil
		}
		return nil, 0, fmt.Errorf("invalid canvas data separator")
	}
}

type canvasFrame struct {
	name              string
	left, width, self int
}

func stacksFromAsyncProfilerCalls(text string) (map[string]int, bool, error) {
	calls := canvasCallRE.FindAllStringSubmatch(text, 1_000_001)
	poolStart := canvasPoolRE.FindStringIndex(text)
	if len(calls) == 0 && poolStart == nil {
		return nil, false, nil
	}
	fail := func(msg string) (map[string]int, bool, error) {
		return nil, true, fmt.Errorf("async-profiler canvas: %s", msg)
	}
	var pool []string
	poolBytes := 0
	packed := canvasUnpackRE.MatchString(text)
	if poolStart != nil {
		literals, _, err := canvasLiterals(text[poolStart[1]:], ']')
		if err != nil {
			return fail(err.Error())
		}
		for _, v := range literals {
			if !v.quoted {
				return fail("constant pool entry must be a string")
			}
			name := v.text
			if len(pool) > 0 && packed {
				units := utf16.Encode([]rune(name))
				prev := utf16.Encode([]rune(pool[len(pool)-1]))
				if len(units) == 0 || units[0] < 32 || int(units[0]-32) > len(prev) {
					return fail("invalid constant pool prefix")
				}
				name = string(utf16.Decode(append(prev[:int(units[0]-32):int(units[0]-32)], units[1:]...)))
			}
			poolBytes += len(name)
			if poolBytes > 64<<20 {
				return fail("decoded constant pool exceeds safety limit")
			}
			pool = append(pool, name)
		}
	}
	if len(calls) == 0 {
		return fail("no frame records")
	}
	if len(calls) > 1_000_000 {
		return fail("frame count exceeds safety limit")
	}
	stacks := map[string]int{}
	var active []canvasFrame
	level0, left0, width0 := 0, 0, 0
	keyBytes := 0
	flush := func() error {
		f := active[len(active)-1]
		if f.self > 0 {
			path := make([]string, len(active))
			for i := range active {
				path[i] = active[i].name
			}
			key := strings.Join(path, ";")
			if _, exists := stacks[key]; !exists {
				keyBytes += len(key)
			}
			if keyBytes > 64<<20 {
				return fmt.Errorf("decoded stacks exceed safety limit")
			}
			stacks[key] += f.self
		}
		active = active[:len(active)-1]
		return nil
	}
	maxInt := int(^uint(0) >> 1)
	for _, call := range calls {
		args, consumed, err := canvasLiterals(call[2]+")", ')')
		if err != nil {
			return fail(err.Error())
		}
		if consumed != len(call[2])+1 {
			return fail("unexpected data after frame arguments")
		}
		num := func(i int) (int, bool) {
			if i >= len(args) || args[i].quoted {
				return 0, false
			}
			n, e := strconv.Atoi(args[i].text)
			return n, e == nil
		}
		var level, left, width int
		var name string
		if poolStart == nil {
			if call[1] != "f" || len(args) < 5 || len(args) > 8 || !args[4].quoted {
				return fail("invalid legacy frame")
			}
			var a, b, c bool
			level, a = num(0)
			left, b = num(1)
			width, c = num(2)
			if !a || !b || !c {
				return fail("invalid legacy coordinates")
			}
			name = args[4].text
		} else {
			key, ok := num(0)
			if !ok || key < 0 || key>>3 >= len(pool) {
				return fail("invalid constant pool index")
			}
			name = pool[key>>3]
			level, left, width = level0, left0, width0
			wi := 1
			switch call[1] {
			case "f":
				var a, b bool
				var delta int
				level, a = num(1)
				delta, b = num(2)
				if !a || !b || delta < 0 || left > maxInt-delta {
					return fail("invalid frame coordinates")
				}
				left += delta
				wi = 3
			case "u":
				level++
			case "n":
				if left > maxInt-width {
					return fail("frame coordinate overflow")
				}
				left += width
			}
			if len(args) > wi+4 {
				return fail("too many frame arguments")
			}
			for i := wi; i < len(args); i++ {
				n, ok := num(i)
				if !ok || n < 0 {
					return fail("invalid frame width or detail")
				}
				if i == wi && n != 0 {
					width = n
				}
			}
		}
		if level < 0 || level > 4096 || level > len(active) || left < 0 || width <= 0 || left > maxInt-width {
			return fail("invalid frame bounds")
		}
		for len(active) > level {
			previous := active[len(active)-1]
			if len(active)-1 == level && left < previous.left+previous.width {
				return fail("overlapping frames")
			}
			if err := flush(); err != nil {
				return fail(err.Error())
			}
		}
		if level > 0 {
			parent := &active[level-1]
			if left < parent.left || left+width > parent.left+parent.width || width > parent.self {
				return fail("frame exceeds parent")
			}
			parent.self -= width
		}
		active = append(active, canvasFrame{name: name, left: left, width: width, self: width})
		level0, left0, width0 = level, left, width
	}
	for len(active) > 0 {
		if err := flush(); err != nil {
			return fail(err.Error())
		}
	}
	return stacks, true, nil
}
