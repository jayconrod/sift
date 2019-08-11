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
	return p.parseBinary(binaryLevels)
}

type binaryLevel []struct {
	tok     token
	combine func(x, y sift.Filter) sift.Filter
}

var binaryLevels = []binaryLevel{
	{
		{
			tok:     pipe,
			combine: sift.Compose,
		},
	}, {
		{
			tok:     comma,
			combine: sift.Concat,
		},
	}, {
		{
			tok:     plus,
			combine: binop(add),
		}, {
			tok:     minus,
			combine: binop(sub),
		},
	}, {
		{
			tok:     star,
			combine: numOp(func(x, y float64) float64 { return x * y }),
		}, {
			tok:     slash,
			combine: numOp(func(x, y float64) float64 { return x / y }),
		}, {
			tok:     percent,
			combine: numOp(math.Mod),
		},
	},
}

var binaryLevelsWithoutComma = append(binaryLevels[:1:1], binaryLevels[2:]...)

func (p *parser) parseBinary(levels []binaryLevel) sift.Filter {
	if len(levels) == 0 {
		return p.parsePrimaryWithPostfix()
	}
	x := p.parseBinary(levels[1:])
Terms:
	for {
		for _, op := range levels[0] {
			if p.tok == op.tok {
				p.scan()
				y := p.parseBinary(levels[1:])
				x = op.combine(x, y)
				continue Terms
			}
		}
		break
	}
	return x
}

func (p *parser) parsePrimaryWithPostfix() sift.Filter {
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
	} else if p.tok == dotDot {
		p.scan()
		return walk
	} else if p.tok == minus {
		p.scan()
		f := p.parsePrimary()
		return sift.Compose(f, sift.MapError(neg))
	} else if p.tok == leftBracket {
		return p.parseArrayConstruct()
	} else if p.tok == leftBrace {
		return p.parseObjectConstruct()
	} else if p.tok == dot {
		dotOk := true
		return p.parsePostfixOrDot(id, dotOk)
	} else if p.tok == leftParen {
		return p.parseGroup()
	}
	p.panicf(p.pos, "expected expression; got %v", p.tok)
	return nil
}

func (p *parser) parseGroup() sift.Filter {
	p.scan()
	f := p.parseExpr()
	if p.tok != rightParen {
		p.panicf(p.pos, "expected %v; got %v", rightParen, p.tok)
	}
	p.scan()
	return f
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

func (p *parser) parseObjectConstruct() sift.Filter {
	p.scan() // leftBrace

	var attrs []sift.Filter
	for p.tok != rightBrace {
		var key sift.Filter
		if p.tok == identifier || p.tok == str {
			_, _, id := p.scan()
			key = sift.Literal(sift.Must(sift.ToValue(id)))
		} else if p.tok == leftParen {
			key = p.parseGroup()
		} else {
			p.panicf(p.pos, "expected attribute name or %v; got %v", rightBrace, p.tok)
		}

		if p.tok != colon {
			p.panicf(p.pos, "expected %v; got %v", colon, p.tok)
		}
		p.scan()

		value := p.parseBinary(binaryLevelsWithoutComma)
		attrs = append(attrs, key, value)

		if p.tok == comma {
			p.scan() // trailing comma is okay
		} else if p.tok != rightBrace {
			p.panicf(p.pos, "expected %v or %v; got %v", comma, rightBrace, p.tok)
		}
	}
	p.scan() // rightBrace

	if len(attrs) == 0 {
		return func(sift.Value) ([]sift.Value, error) {
			empty := sift.Must(sift.ToValue(map[string]sift.Value{}))
			return []sift.Value{empty}, nil
		}
	}
	return sift.Nary(attrs, constructObject)
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
