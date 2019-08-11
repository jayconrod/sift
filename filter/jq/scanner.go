// Copyright 2019 Jay Conrod.
// Copyright 2009 The Go Authors. All rights reserved.

package jq

import (
	"bytes"
	"fmt"
	gotoken "go/token"
	"runtime"
	"unicode"
	"unicode/utf8"
)

type token int

const (
	illegal token = iota
	eof
	dot
	dotDot
	comma
	questionMark
	colon
	pipe
	star
	slash
	percent
	plus
	minus
	leftBracket
	rightBracket
	leftBrace
	rightBrace
	leftParen
	rightParen
	null
	true_
	false_
	identifier
	number
	str
)

func (t token) String() string {
	switch t {
	case illegal:
		return "ILLEGAL"
	case eof:
		return "EOF"
	case dot:
		return "."
	case dotDot:
		return ".."
	case comma:
		return ","
	case questionMark:
		return "?"
	case colon:
		return ":"
	case pipe:
		return "|"
	case star:
		return "*"
	case slash:
		return "/"
	case percent:
		return "%"
	case plus:
		return "+"
	case minus:
		return "-"
	case leftBracket:
		return "["
	case rightBracket:
		return "]"
	case leftBrace:
		return "{"
	case rightBrace:
		return "}"
	case leftParen:
		return "("
	case rightParen:
		return ")"
	case null:
		return "null"
	case true_:
		return "true"
	case false_:
		return "false"
	case identifier:
		return "identifier"
	case number:
		return "number"
	case str:
		return "string"
	default:
		return "unknown"
	}
}

type scanner struct {
	file     *gotoken.File
	src      []byte
	ch       rune
	offset   int // offset of ch
	rdOffset int // offset of character after ch
}

func newScanner(file *gotoken.File, src []byte) *scanner {
	s := &scanner{
		file: file,
		src:  src,
		ch:   ' ',
	}
	s.next()
	if s.ch == bom {
		s.next() // ignore BOM at beginning of file
	}
	return s
}

func (s *scanner) scan() (pos gotoken.Pos, tok token, lit string) {
Retry:
	s.skipWhitespace()

	// current token start
	pos = s.file.Pos(s.offset)

	// determine token value
	switch ch := s.ch; {
	case ch == '#':
		s.skipComment()
		goto Retry

	case isLetter(ch) || ch == '_':
		lit = s.scanIdentifier()
		switch lit {
		case "null":
			tok = null
		case "true":
			tok = true_
		case "false":
			tok = false_
		default:
			tok = identifier
		}

	case '0' <= ch && ch <= '9':
		lit = s.scanNumber()
		tok = number

	case ch == '\'' || ch == '"':
		lit = s.scanString()
		tok = str

	default:
		s.next() // always make progress
		switch ch {
		case '.':
			tok = dot
			if '0' <= s.ch && s.ch <= '9' {
				lit = "." + s.scanNumber()
				tok = number
			} else if s.ch == '.' {
				s.next()
				tok = dotDot
			}

		case ',':
			tok = comma

		case '?':
			tok = questionMark

		case ':':
			tok = colon

		case '|':
			tok = pipe

		case '*':
			tok = star

		case '/':
			tok = slash

		case '%':
			tok = percent

		case '-':
			tok = minus

		case '+':
			tok = plus

		case '[':
			tok = leftBracket

		case ']':
			tok = rightBracket

		case '{':
			tok = leftBrace

		case '}':
			tok = rightBrace

		case '(':
			tok = leftParen

		case ')':
			tok = rightParen

		case -1:
			tok = eof

		default:
			tok = illegal
			s.panicf(s.file.Offset(pos), "illegal character %#U", ch)
		}
	}

	return pos, tok, lit
}

func (s *scanner) scanOrError() (pos gotoken.Pos, tok token, lit string, err error) {
	defer recoverError(&err)
	pos, tok, lit = s.scan()
	return pos, tok, lit, nil
}

func (s *scanner) scanIdentifier() string {
	begin := s.offset
	for isLetter(s.ch) || isDigit(s.ch) || s.ch == '_' {
		s.next()
	}
	return string(s.src[begin:s.offset])
}

func (s *scanner) scanNumber() string {
	begin := s.offset
	haveInteger := false
	for '0' <= s.ch && s.ch <= '9' {
		haveInteger = true
		s.next()
	}
	haveBase := haveInteger
	if s.ch == '.' {
		s.next()
		haveFraction := false
		for '0' <= s.ch && s.ch <= '9' {
			haveFraction = true
			s.next()
		}
		if !haveInteger && !haveFraction {
			s.panicf(begin, "invalid number")
		}
		haveBase = true
	}
	if s.ch == 'e' || s.ch == 'E' {
		if !haveBase {
			s.panicf(begin, "invalid number")
		}
		s.next()
		if s.ch == '+' || s.ch == '-' {
			s.next()
		}
		haveExponent := false
		for '0' <= s.ch && s.ch <= '9' {
			haveExponent = true
			s.next()
		}
		if !haveExponent {
			s.panicf(begin, "invalid number")
		}
	}
	return string(s.src[begin:s.offset])
}

func (s *scanner) scanString() string {
	begin := s.offset
	q := s.ch
	if q != '\'' && q != '"' {
		s.panicf(s.offset, "not a string: %#U", s.ch)
	}
	s.next()

	buf := &bytes.Buffer{}
	for {
		ch := s.ch
		if ch == '\n' || ch < 0 {
			s.panicf(begin, "string literal not terminated")
		}
		if ch == q {
			s.next()
			break
		}
		if ch == '\\' {
			r := s.scanEscape()
			buf.WriteRune(r)
			continue
		}
		buf.WriteRune(ch)
		s.next()
	}
	return buf.String()
}

func (s *scanner) scanEscape() rune {
	s.next() // consume backslash
	var n int
	var base, max uint32
	var exact bool
	var r rune
	switch ch := s.ch; ch {
	case '\'', '"', '\\':
		r = ch
	case 'n':
		r = '\n'
	case 'r':
		r = '\r'
	case 'v':
		r = '\v'
	case 't':
		r = '\t'
	case 'b':
		r = '\b'
	case 'f':
		r = '\f'
	case '0', '1', '2', '3', '4', '5', '6', '7':
		n, base, max = 3, 8, 0xFF
	case 'u':
		n, base, max = 4, 16, 0xFFFF
		exact = true
	case 'x':
		n, base, max = 2, 16, 0xFF
		exact = true
	default:
		s.panicf(s.offset, "invalid escape: %c", s.ch)
	}
	if n != 3 {
		// consume next character, except for octal escape
		s.next()
	}
	if n > 0 {
		var code uint32
		for i := 0; i < n; i++ {
			h, ok := hexDigit(s.ch)
			if !ok || h >= base || code*base+h > max {
				if exact {
					s.panicf(s.offset, "invalid escape")
				} else {
					break
				}
			}
			s.next()
			code = code*base + h
		}
		r = rune(code)
	}
	return r
}

func (s *scanner) skipWhitespace() {
	for s.ch == ' ' || s.ch == '\t' || s.ch == '\n' || s.ch == '\r' {
		s.next()
	}
}

func (s *scanner) skipComment() {
	if s.ch != '#' {
		s.panicf(s.offset, "not a comment: %#U", s.ch)
	}
	for s.ch != '\n' && s.ch != -1 {
		s.next()
	}
}

const bom = 0xFEFF // byte order mark, only permitted as first character

// next reads the next unicode character into s.ch.
// s.ch < 0 means EOF.
func (s *scanner) next() {
	if s.rdOffset < len(s.src) {
		s.offset = s.rdOffset
		if s.ch == '\n' {
			s.file.AddLine(s.offset)
		}
		r, w := rune(s.src[s.rdOffset]), 1
		switch {
		case r == 0:
			s.panicf(s.offset, "illegal character NUL")
		case r >= utf8.RuneSelf:
			// not ASCII
			r, w = utf8.DecodeRune(s.src[s.rdOffset:])
			if r == utf8.RuneError && w == 1 {
				s.panicf(s.offset, "illegal UTF-8 encoding")
			} else if r == bom && s.offset > 0 {
				s.panicf(s.offset, "illegal byte order mark")
			}
		}
		s.rdOffset += w
		s.ch = r
	} else {
		s.offset = len(s.src)
		if s.ch == '\n' {
			s.file.AddLine(s.offset)
		}
		s.ch = -1 // eof
	}
}

// peek returns the byte following the most recently read character without
// advancing the scanner. If the scanner is at EOF, peek returns 0.
func (s *scanner) peek() byte {
	if s.rdOffset < len(s.src) {
		return s.src[s.rdOffset]
	}
	return 0
}

func (s *scanner) panicf(offset int, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	err := scanError{s.file.Position(s.file.Pos(offset)), message}
	panic(err)
}

func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || unicode.IsLetter(ch)
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9' || unicode.IsDigit(ch)
}

func hexDigit(ch rune) (uint32, bool) {
	var base, offset uint32
	switch {
	case '0' <= ch && ch <= '9':
		base, offset = '0', 0
	case 'A' <= ch && ch <= 'F':
		base, offset = 'A', 10
	case 'a' <= ch && ch <= 'f':
		base, offset = 'a', 10
	default:
		return 0, false
	}
	return uint32(ch) - base + offset, true
}

type scanError struct {
	position gotoken.Position
	message  string
}

func (e scanError) Error() string {
	return fmt.Sprintf("%s: %s", e.position, e.message)
}

func recoverError(err *error) {
	r := recover()
	if e, ok := r.(error); ok {
		if _, ok := e.(runtime.Error); ok {
			panic(r)
		}
		*err = e
	} else if r != nil {
		panic(r)
	}
}
