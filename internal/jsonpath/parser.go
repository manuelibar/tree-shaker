package jsonpath

import (
	"strconv"
	"unicode"
	"unicode/utf8"
)

// parsePath parses a JSONPath string into a Path.
func parsePath(raw string) (*Path, error) {
	if len(raw) > MaxPathLength {
		return nil, &ParseError{Path: raw, Pos: 0, Message: "path exceeds maximum length"}
	}

	p := &parser{src: raw, pos: 0}
	segments, err := p.parse()
	if err != nil {
		return nil, err
	}
	return &Path{Segments: segments, Raw: raw}, nil
}

type parser struct {
	src string
	pos int
}

func (p *parser) parse() ([]Segment, error) {
	// Optional "$" root
	if p.pos < len(p.src) && p.src[p.pos] == '$' {
		p.pos++
	}

	var segments []Segment
	for p.pos < len(p.src) {
		seg, err := p.parseSegment()
		if err != nil {
			return nil, err
		}
		segments = append(segments, seg)
	}

	if len(segments) == 0 {
		return nil, &ParseError{Path: p.src, Pos: p.pos, Message: "empty path"}
	}

	return segments, nil
}

func (p *parser) parseSegment() (Segment, error) {
	if p.pos >= len(p.src) {
		return Segment{}, &ParseError{Path: p.src, Pos: p.pos, Message: "unexpected end of path"}
	}

	descendant := false

	if p.startsWith("..") {
		descendant = true
		p.pos += 2
		// After "..", we can have a name, wildcard, or bracket
		if p.pos >= len(p.src) {
			return Segment{}, &ParseError{Path: p.src, Pos: p.pos, Message: "unexpected end after '..'"}
		}
		if p.src[p.pos] == '[' {
			sel, err := p.parseBracket()
			if err != nil {
				return Segment{}, err
			}
			return Segment{Selectors: sel, Descendant: descendant}, nil
		}
		if p.src[p.pos] == '*' {
			p.pos++
			return Segment{Selectors: []Selector{WildcardSelector{}}, Descendant: descendant}, nil
		}
		name, err := p.parseName()
		if err != nil {
			return Segment{}, err
		}
		return Segment{Selectors: []Selector{NameSelector{Name: name}}, Descendant: descendant}, nil
	}

	if p.src[p.pos] == '.' {
		p.pos++
		if p.pos >= len(p.src) {
			return Segment{}, &ParseError{Path: p.src, Pos: p.pos, Message: "unexpected end after '.'"}
		}
		if p.src[p.pos] == '*' {
			p.pos++
			return Segment{Selectors: []Selector{WildcardSelector{}}, Descendant: false}, nil
		}
		name, err := p.parseName()
		if err != nil {
			return Segment{}, err
		}
		return Segment{Selectors: []Selector{NameSelector{Name: name}}, Descendant: false}, nil
	}

	if p.src[p.pos] == '[' {
		sel, err := p.parseBracket()
		if err != nil {
			return Segment{}, err
		}
		return Segment{Selectors: sel, Descendant: false}, nil
	}

	return Segment{}, &ParseError{
		Path:    p.src,
		Pos:     p.pos,
		Message: "expected '.', '..', or '['",
	}
}

func (p *parser) parseName() (string, error) {
	start := p.pos
	for p.pos < len(p.src) {
		r, size := utf8.DecodeRuneInString(p.src[p.pos:])
		if r == '.' || r == '[' {
			break
		}
		if !isNameChar(r) {
			return "", &ParseError{
				Path:    p.src,
				Pos:     p.pos,
				Message: "invalid character in name",
			}
		}
		p.pos += size
	}
	if p.pos == start {
		return "", &ParseError{Path: p.src, Pos: p.pos, Message: "expected name"}
	}
	return p.src[start:p.pos], nil
}

func (p *parser) parseBracket() ([]Selector, error) {
	if p.pos >= len(p.src) || p.src[p.pos] != '[' {
		return nil, &ParseError{Path: p.src, Pos: p.pos, Message: "expected '['"}
	}
	p.pos++ // consume '['

	var selectors []Selector

	for {
		p.skipSpaces()
		if p.pos >= len(p.src) {
			return nil, &ParseError{Path: p.src, Pos: p.pos, Message: "unclosed '['"}
		}

		sel, err := p.parseBracketSelector()
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, sel)

		p.skipSpaces()
		if p.pos >= len(p.src) {
			return nil, &ParseError{Path: p.src, Pos: p.pos, Message: "unclosed '['"}
		}
		if p.src[p.pos] == ']' {
			p.pos++
			return selectors, nil
		}
		if p.src[p.pos] == ',' {
			p.pos++
			continue
		}
		return nil, &ParseError{Path: p.src, Pos: p.pos, Message: "expected ',' or ']'"}
	}
}

func (p *parser) parseBracketSelector() (Selector, error) {
	p.skipSpaces()
	if p.pos >= len(p.src) {
		return nil, &ParseError{Path: p.src, Pos: p.pos, Message: "unexpected end in bracket"}
	}

	ch := p.src[p.pos]

	// Wildcard
	if ch == '*' {
		p.pos++
		return WildcardSelector{}, nil
	}

	// String selector
	if ch == '\'' || ch == '"' {
		return p.parseStringSelector()
	}

	// Number: could be index, slice, or negative index
	// Also handle bare ":" for slice with no start
	return p.parseNumberOrSlice()
}

func (p *parser) parseStringSelector() (Selector, error) {
	quote := p.src[p.pos]
	p.pos++ // consume opening quote
	start := p.pos

	for p.pos < len(p.src) {
		if p.src[p.pos] == '\\' {
			p.pos += 2 // skip escape
			continue
		}
		if p.src[p.pos] == quote {
			name := p.src[start:p.pos]
			p.pos++ // consume closing quote
			// Unescape basic sequences
			unescaped, err := unescapeString(name, p.src, start)
			if err != nil {
				return nil, err
			}
			return NameSelector{Name: unescaped}, nil
		}
		p.pos++
	}
	return nil, &ParseError{Path: p.src, Pos: start - 1, Message: "unclosed string"}
}

func (p *parser) parseNumberOrSlice() (Selector, error) {
	// Could be: int, int:int, int:int:int, :int, ::int, etc.
	hasNum := false
	var startVal *int

	if p.pos < len(p.src) && (p.src[p.pos] == '-' || isDigit(p.src[p.pos])) {
		n, err := p.parseInt()
		if err != nil {
			return nil, err
		}
		startVal = &n
		hasNum = true
	}

	// Check if this is a slice (next char is ':')
	if p.pos < len(p.src) && p.src[p.pos] == ':' {
		return p.parseSliceRest(startVal)
	}

	if !hasNum {
		return nil, &ParseError{Path: p.src, Pos: p.pos, Message: "expected number, string, or '*'"}
	}

	return IndexSelector{Index: *startVal}, nil
}

func (p *parser) parseSliceRest(start *int) (Selector, error) {
	// We're at ':'
	p.pos++ // consume first ':'

	var end *int
	p.skipSpaces()
	if p.pos < len(p.src) && (p.src[p.pos] == '-' || isDigit(p.src[p.pos])) {
		n, err := p.parseInt()
		if err != nil {
			return nil, err
		}
		end = &n
	}

	var step *int
	p.skipSpaces()
	if p.pos < len(p.src) && p.src[p.pos] == ':' {
		p.pos++ // consume second ':'
		p.skipSpaces()
		if p.pos < len(p.src) && (p.src[p.pos] == '-' || isDigit(p.src[p.pos])) {
			n, err := p.parseInt()
			if err != nil {
				return nil, err
			}
			step = &n
		}
	}

	return SliceSelector{Start: start, End: end, Step: step}, nil
}

func (p *parser) parseInt() (int, error) {
	start := p.pos
	if p.pos < len(p.src) && p.src[p.pos] == '-' {
		p.pos++
	}
	if p.pos >= len(p.src) || !isDigit(p.src[p.pos]) {
		return 0, &ParseError{Path: p.src, Pos: start, Message: "expected integer"}
	}
	for p.pos < len(p.src) && isDigit(p.src[p.pos]) {
		p.pos++
	}
	n, err := strconv.Atoi(p.src[start:p.pos])
	if err != nil {
		return 0, &ParseError{Path: p.src, Pos: start, Message: "invalid integer"}
	}
	return n, nil
}

func (p *parser) skipSpaces() {
	for p.pos < len(p.src) && p.src[p.pos] == ' ' {
		p.pos++
	}
}

func (p *parser) startsWith(prefix string) bool {
	return len(p.src)-p.pos >= len(prefix) && p.src[p.pos:p.pos+len(prefix)] == prefix
}

func isNameChar(r rune) bool {
	return r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func unescapeString(s, fullPath string, pos int) (string, error) {
	// Fast path: no escapes
	hasEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			hasEscape = true
			break
		}
	}
	if !hasEscape {
		return s, nil
	}

	buf := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			if i >= len(s) {
				return "", &ParseError{Path: fullPath, Pos: pos + i, Message: "trailing backslash"}
			}
			switch s[i] {
			case '\\', '\'', '"':
				buf = append(buf, s[i])
			case 'n':
				buf = append(buf, '\n')
			case 't':
				buf = append(buf, '\t')
			case 'r':
				buf = append(buf, '\r')
			default:
				return "", &ParseError{Path: fullPath, Pos: pos + i, Message: "invalid escape sequence"}
			}
		} else {
			buf = append(buf, s[i])
		}
	}
	return string(buf), nil
}
