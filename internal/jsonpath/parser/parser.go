// Package parser implements a recursive descent parser for JSONPath expressions,
// producing an Abstract Syntax Tree (AST) of [Path], [Segment], and [Selector] nodes.
//
// The parser is a single-pass scanner: bytes are read and converted directly into
// AST nodes with no intermediate token types, minimising allocation overhead.
//
// Supported syntax: dot notation (.name), bracket notation (['name'], [0], [0:10:2],
// [*]), recursive descent (..), and multi-selectors ([a,b]) per RFC 9535.
package parser

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// MaxPathLength is the default maximum allowed byte length of a single JSONPath string.
// Applied automatically when [jsonpath.Limits].MaxPathLength is nil (the zero-value
// default). Oversized paths waste CPU and memory in the parser; this limit is
// enforced by default to prevent such abuse.
const MaxPathLength = 10000

// ParseError describes a syntax error encountered while parsing a JSONPath expression.
type ParseError struct {
	Path    string
	Pos     int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at position %d in %q: %s", e.Pos, e.Path, e.Message)
}

// ParseOption configures optional behaviour for [ParsePath].
type ParseOption func(*parseConfig)

type parseConfig struct {
	maxLength int // ≤ 0 means no restriction
}

// WithMaxLength restricts the byte length of the raw JSONPath string.
// Paths exceeding maxLength cause [ParsePath] to return a [ParseError].
// A value ≤ 0 disables the check.
func WithMaxLength(maxLength int) ParseOption {
	return func(c *parseConfig) { c.maxLength = maxLength }
}

// ParsePath parses a raw JSONPath string into an AST [Path].
// Pass [WithMaxLength] to enforce a byte-length limit on the input.
func ParsePath(raw string, opts ...ParseOption) (*Path, error) {
	var cfg parseConfig
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.maxLength > 0 && len(raw) > cfg.maxLength {
		return nil, &ParseError{Path: raw, Pos: 0, Message: "path exceeds maximum length"}
	}
	s := scanner{src: raw}
	segments, err := s.parse()
	if err != nil {
		return nil, err
	}
	return &Path{Segments: segments, Raw: raw}, nil
}

// scanner is a single-pass JSONPath parser. It reads bytes from src and produces
// AST nodes directly — no intermediate Token allocations.
type scanner struct {
	src string
	pos int
}

func (s *scanner) err(msg string) *ParseError {
	return &ParseError{Path: s.src, Pos: s.pos, Message: msg}
}

func (s *scanner) errAt(pos int, msg string) *ParseError {
	return &ParseError{Path: s.src, Pos: pos, Message: msg}
}

func (s *scanner) skipSpaces() {
	for s.pos < len(s.src) && s.src[s.pos] == ' ' {
		s.pos++
	}
}

// parse is the entry point: optional "$" root, then one or more segments.
func (s *scanner) parse() ([]Segment, error) {
	if s.pos < len(s.src) && s.src[s.pos] == '$' {
		s.pos++
	}

	var segments []Segment
	for s.pos < len(s.src) {
		seg, err := s.parseSegment()
		if err != nil {
			return nil, err
		}
		segments = append(segments, seg)
	}

	if len(segments) == 0 {
		return nil, s.err("empty path")
	}
	return segments, nil
}

// parseSegment handles ".name", "..name", "[...]", ".*", "..*" etc.
func (s *scanner) parseSegment() (Segment, error) {
	ch := s.src[s.pos]

	if ch == '.' {
		if s.pos+1 < len(s.src) && s.src[s.pos+1] == '.' {
			s.pos += 2
			return s.parseAfterDescendant()
		}
		s.pos++
		return s.parseAfterDot()
	}

	if ch == '[' {
		sel, err := s.parseBracket()
		if err != nil {
			return Segment{}, err
		}
		return Segment{Selectors: sel}, nil
	}

	return Segment{}, s.err("expected '.', '..', or '['")
}

// parseAfterDescendant handles ".." followed by a name, wildcard, or bracket.
func (s *scanner) parseAfterDescendant() (Segment, error) {
	if s.pos >= len(s.src) {
		return Segment{}, s.err("unexpected end after '..'")
	}

	ch := s.src[s.pos]

	if ch == '[' {
		sel, err := s.parseBracket()
		if err != nil {
			return Segment{}, err
		}
		return Segment{Selectors: sel, Descendant: true}, nil
	}

	if ch == '*' {
		s.pos++
		return Segment{Selectors: []Selector{WildcardSelector{}}, Descendant: true}, nil
	}

	if s.isNameStart() {
		name := s.scanName()
		return Segment{Selectors: []Selector{NameSelector{Name: name}}, Descendant: true}, nil
	}

	return Segment{}, s.err("expected name, '*', or '[' after '..'")
}

// parseAfterDot handles "." followed by a name or wildcard.
func (s *scanner) parseAfterDot() (Segment, error) {
	if s.pos >= len(s.src) {
		return Segment{}, s.err("unexpected end after '.'")
	}

	if s.src[s.pos] == '*' {
		s.pos++
		return Segment{Selectors: []Selector{WildcardSelector{}}}, nil
	}

	if s.isNameStart() {
		name := s.scanName()
		return Segment{Selectors: []Selector{NameSelector{Name: name}}}, nil
	}

	return Segment{}, s.err("expected name or '*' after '.'")
}

// parseBracket parses "[" ... "]" with one or more selectors.
func (s *scanner) parseBracket() ([]Selector, error) {
	s.pos++ // consume '['
	var selectors []Selector

	for {
		s.skipSpaces()
		sel, err := s.parseBracketSelector()
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, sel)

		s.skipSpaces()
		if s.pos >= len(s.src) {
			return nil, s.err("unclosed '['")
		}

		ch := s.src[s.pos]
		if ch == ']' {
			s.pos++
			return selectors, nil
		}
		if ch == ',' {
			s.pos++
			continue
		}

		return nil, s.err("expected ',' or ']'")
	}
}

// parseBracketSelector parses a single selector inside brackets.
func (s *scanner) parseBracketSelector() (Selector, error) {
	if s.pos >= len(s.src) {
		return nil, s.err("unexpected end in bracket")
	}

	ch := s.src[s.pos]

	if ch == '*' {
		s.pos++
		return WildcardSelector{}, nil
	}

	if ch == '\'' || ch == '"' {
		name, err := s.scanString()
		if err != nil {
			return nil, err
		}
		return NameSelector{Name: name}, nil
	}

	if ch == '-' || (ch >= '0' && ch <= '9') {
		return s.parseNumberOrSlice()
	}

	if ch == ':' {
		return s.parseSliceFrom(nil)
	}

	return nil, s.err("expected number, string, or '*'")
}

// parseNumberOrSlice reads an integer, then checks if ':' follows (making it a slice start).
func (s *scanner) parseNumberOrSlice() (Selector, error) {
	n, err := s.scanInt()
	if err != nil {
		return nil, err
	}

	s.skipSpaces()
	if s.pos < len(s.src) && s.src[s.pos] == ':' {
		return s.parseSliceFrom(&n)
	}

	return IndexSelector{Index: n}, nil
}

// parseSliceFrom parses "start:end:step" from the first ':'. start may be nil.
func (s *scanner) parseSliceFrom(start *int) (Selector, error) {
	s.pos++ // consume ':'
	s.skipSpaces()

	var end *int
	if s.pos < len(s.src) && (s.src[s.pos] == '-' || (s.src[s.pos] >= '0' && s.src[s.pos] <= '9')) {
		n, err := s.scanInt()
		if err != nil {
			return nil, err
		}
		end = &n
	}

	s.skipSpaces()

	var step *int
	if s.pos < len(s.src) && s.src[s.pos] == ':' {
		s.pos++ // consume second ':'
		s.skipSpaces()
		if s.pos < len(s.src) && (s.src[s.pos] == '-' || (s.src[s.pos] >= '0' && s.src[s.pos] <= '9')) {
			n, err := s.scanInt()
			if err != nil {
				return nil, err
			}
			step = &n
		}
	}

	return SliceSelector{Start: start, End: end, Step: step}, nil
}

// isNameStart reports whether the current byte starts a valid name (letter or '_').
// Digits and '-' are valid continuations but not valid starts.
func (s *scanner) isNameStart() bool {
	if s.pos >= len(s.src) {
		return false
	}
	b := s.src[s.pos]
	if b < utf8.RuneSelf {
		return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
	}
	r, _ := utf8.DecodeRuneInString(s.src[s.pos:])
	return unicode.IsLetter(r)
}

// scanName scans an unquoted identifier (letters, digits, '_', '-') and returns the string.
func (s *scanner) scanName() string {
	start := s.pos
	for s.pos < len(s.src) {
		b := s.src[s.pos]
		if b < utf8.RuneSelf {
			if b == '_' || b == '-' ||
				(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
				(b >= '0' && b <= '9') {
				s.pos++
				continue
			}
			break
		}
		r, size := utf8.DecodeRuneInString(s.src[s.pos:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		s.pos += size
	}
	return s.src[start:s.pos]
}

// scanInt scans an integer directly (no strconv), returning the parsed value.
func (s *scanner) scanInt() (int, error) {
	start := s.pos
	neg := false
	if s.pos < len(s.src) && s.src[s.pos] == '-' {
		neg = true
		s.pos++
	}
	if s.pos >= len(s.src) || s.src[s.pos] < '0' || s.src[s.pos] > '9' {
		return 0, s.errAt(start, "expected integer")
	}
	n := 0
	for s.pos < len(s.src) && s.src[s.pos] >= '0' && s.src[s.pos] <= '9' {
		n = n*10 + int(s.src[s.pos]-'0')
		s.pos++
	}
	if neg {
		n = -n
	}
	return n, nil
}

// scanString scans a single- or double-quoted string, returning the unescaped content.
func (s *scanner) scanString() (string, error) {
	quote := s.src[s.pos]
	start := s.pos
	s.pos++ // consume opening quote

	contentStart := s.pos
	hasEscape := false
	for s.pos < len(s.src) {
		if s.src[s.pos] == '\\' {
			hasEscape = true
			s.pos += 2 // skip escape pair
			continue
		}
		if s.src[s.pos] == quote {
			content := s.src[contentStart:s.pos]
			s.pos++ // consume closing quote
			if !hasEscape {
				return content, nil // fast path: no escapes
			}
			return unescape(content, s.src, contentStart)
		}
		s.pos++
	}
	return "", s.errAt(start, "unclosed string")
}

// unescape processes backslash escapes within a quoted string.
func unescape(s, fullPath string, pos int) (string, error) {
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
