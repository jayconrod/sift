package jq

import (
	"fmt"
	gotoken "go/token"
	"math"
	"strconv"

	"go.jayconrod.com/sift"
)

type parser struct {
	file    *gotoken.File
	scanner *scanner

	pos gotoken.Pos
	tok token
	lit string

	initScanErr error
}

func newParser(s *scanner) *parser {
	p := &parser{
		file:    s.file,
		scanner: s,
	}
	p.pos, p.tok, p.lit, p.initScanErr = s.scanOrError()
	return p
}

func (p *parser) parse() sift.Filter {
	if p.initScanErr != nil {
		panic(p.initScanErr)
	}
	if p.tok == eof {
		return id
	}
	f := p.parseExpr()
	if p.tok != eof {
		p.panicf(p.pos, "junk at end of file")
	}
	return f
}

func (p *parser) parseExpr() sift.Filter {
	f := p.parsePrimary()
	return p.parsePostfixOrDot(f, false)
}

func (p *parser) parsePrimary() sift.Filter {
	if p.tok == null {
		p.scan()
		return sift.Literal(sift.Must(sift.ToValue(nil)))
	} else if p.tok == true_ {
		p.scan()
		return sift.Literal(sift.Must(sift.ToValue(true)))
	} else if p.tok == false_ {
		p.scan()
		return sift.Literal(sift.Must(sift.ToValue(false)))
	} else if p.tok == number {
		n, err := strconv.ParseFloat(p.lit, 64)
		if nerr, ok := err.(*strconv.NumError); ok && nerr.Err == strconv.ErrRange {
			// ParseFloat returns this error for numbers too large in either direction.
			// jq clamps them to the maximum non-infinite value.
			if p.lit[0] == '-' {
				n = -math.MaxFloat64
			} else {
				n = math.MaxFloat64
			}
		} else if err != nil {
			p.panicf(p.pos, "invalid number: %v", err)
		}
		p.scan()
		return sift.Literal(sift.Must(sift.ToValue(n)))
	} else if p.tok == str {
		s := p.lit
		p.scan()
		return sift.Literal(sift.Must(sift.ToValue(s)))
	} else if p.tok == leftBracket {
		return p.parseArrayConstruct()
	} else if p.tok == dot {
		dotOk := true
		return p.parsePostfixOrDot(id, dotOk)
	}
	p.panicf(p.pos, "expected expression; got %v", p.tok)
	return nil
}

func (p *parser) parsePostfixOrDot(f sift.Filter, dotOk bool) sift.Filter {
	for {
		switch p.tok {
		case dot:
			p.scan()
			switch p.tok {
			case identifier, str:
				_, _, lit := p.scan()
				if p.tok == questionMark {
					p.scan()
					f = sift.Compose(f, attrLit(lit, false))
				} else {
					f = sift.Compose(f, attrLit(lit, true))
				}

			default:
				if !dotOk {
					p.panicf(p.pos, "expected selector after %v; got %v", dot, p.tok)
				}
			}

		case leftBracket:
			f = p.parseIndex(f)

		default:
			return f
		}

		dotOk = false
	}
}

func (p *parser) parseIndex(base sift.Filter) sift.Filter {
	p.scan() // leftBracket
	var idx, begin, end sift.Filter
	if p.tok == rightBracket {
		p.scan()
		f := iterate
		if p.tok == questionMark {
			p.scan()
			f = iterateOpt
		}
		return sift.Compose(base, f)
	} else if p.tok == colon {
		p.scan()
		end = p.parseExpr()
	} else {
		idx = p.parseExpr()
		if p.tok == colon {
			begin, idx = idx, nil
			p.scan()
			if p.tok != rightBracket {
				end = p.parseExpr()
			}
		}
	}
	if p.tok != rightBracket {
		p.panicf(p.pos, "expected %v; got %v", rightBracket, p.tok)
	}
	p.scan()
	if idx != nil {
		return sift.Binary(base, idx, index)
	} else {
		if begin == nil {
			return sift.Binary(base, end, func(vbase, vend sift.Value) ([]sift.Value, error) {
				return slice(vbase, nil, vend)
			})
		} else if end == nil {
			return sift.Binary(base, begin, func(vbase, vbegin sift.Value) ([]sift.Value, error) {
				return slice(vbase, vbegin, nil)
			})
		} else {
			return sift.Ternary(base, begin, end, slice)
		}
	}
}

func (p *parser) parseArrayConstruct() sift.Filter {
	p.scan() // leftBracket
	var exprs []sift.Filter
	for p.tok != rightBracket {
		exprs = append(exprs, p.parseExpr())
		if p.tok == comma {
			p.scan()
		} else if p.tok != rightBracket {
			p.panicf(p.pos, "expected %v or %v; got %v", comma, rightBracket, p.tok)
		}
	}
	p.scan() // rightBracket

	return func(v sift.Value) ([]sift.Value, error) {
		var results []sift.Value
		for _, expr := range exprs {
			rs, err := expr(v)
			if err != nil {
				return nil, err
			}
			results = append(results, rs...)
		}
		arr, err := sift.ToValue(results)
		if err != nil {
			return nil, err
		}
		return []sift.Value{arr}, nil
	}
}

func (p *parser) scan() (gotoken.Pos, token, string) {
	pos, tok, lit := p.pos, p.tok, p.lit
	p.pos, p.tok, p.lit = p.scanner.scan()
	return pos, tok, lit
}

func (p *parser) panicf(pos gotoken.Pos, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	err := parseError{p.file.Position(pos), message}
	panic(err)
}

type parseError struct {
	position gotoken.Position
	message  string
}

func (e parseError) Error() string {
	return fmt.Sprintf("%s: %s", e.position, e.message)
}
