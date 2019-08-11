package jq

import (
	"fmt"

	"go.jayconrod.com/sift"
)

func id(v sift.Value) ([]sift.Value, error) {
	return []sift.Value{v}, nil
}

func attrLit(lit string, required bool) sift.Filter {
	return func(v sift.Value) ([]sift.Value, error) {
		if value, ok := sift.GetStringAttr(v, lit); !ok {
			if required {
				return []sift.Value{sift.Must(sift.ToValue(nil))}, nil
			} else {
				return nil, nil
			}
		} else {
			return []sift.Value{value}, nil
		}
	}
}

func index(base, idx sift.Value) ([]sift.Value, error) {
	switch base := base.(type) {
	case sift.Index:
		n := base.Length()
		f, ok := sift.AsFloat64(idx)
		if !ok {
			return nil, fmt.Errorf("cannot index array with value %#v", idx)
		}
		i := int(f)
		if f != float64(i) {
			return nil, nil
		}
		if i < 0 {
			i += n
		}
		v, ok := base.Index(i)
		if !ok {
			v = sift.Must(sift.ToValue(nil))
		}
		return []sift.Value{v}, nil

	case sift.Attr:
		v, ok := base.Attr(idx)
		if !ok {
			v = sift.Must(sift.ToValue(nil))
		}
		return []sift.Value{v}, nil

	default:
		if !sift.IsNull(base) {
			return nil, fmt.Errorf("cannot index value %v with value %v", base, idx)
		}
		v := sift.Must(sift.ToValue(nil))
		return []sift.Value{v}, nil
	}
}

func slice(base, begin, end sift.Value) ([]sift.Value, error) {
	if sift.IsNull(base) {
		return []sift.Value{sift.NullValue}, nil
	}
	n, ok := sift.Length(base)
	if !ok {
		return nil, fmt.Errorf("cannot slice value %v", base)
	}

	var beginI, endI int
	var err error
	if begin == nil {
		beginI = 0
	} else {
		beginI, err = clampIndex(begin, n)
		if err != nil {
			return nil, err
		}
	}
	if end == nil {
		endI = n
	} else {
		endI, err = clampIndex(end, n)
		if err != nil {
			return nil, err
		}
	}

	if baseIndex, ok := base.(sift.Index); ok {
		elems := make([]sift.Value, 0, endI-beginI)
		for i := beginI; i < endI; i++ {
			elem, ok := baseIndex.Index(i)
			if ok {
				elems = append(elems, elem)
			}
		}
		list := sift.Must(sift.ToValue(elems))
		return []sift.Value{list}, nil
	} else if baseString, ok := sift.AsString(base); ok {
		sub := sift.Must(sift.ToValue(baseString[beginI:endI]))
		return []sift.Value{sub}, nil
	} else {
		panic(fmt.Sprintf("unexpected value %#v", base))
	}
}

func clampIndex(idx sift.Value, n int) (int, error) {
	f, ok := sift.AsFloat64(idx)
	if !ok {
		return -1, fmt.Errorf("index must be number")
	}
	i := int(f)
	if i < 0 {
		i += n
	}
	if i < 0 {
		i = 0
	}
	if i > n {
		i = n
	}
	return i, nil
}

func iterate(v sift.Value) ([]sift.Value, error) {
	idx, ok := v.(sift.Index)
	if !ok {
		return nil, fmt.Errorf("cannot iterate over value %#v", v)
	}
	n := idx.Length()
	elems := make([]sift.Value, n)
	for i := 0; i < n; i++ {
		elem, ok := idx.Index(i)
		if !ok {
			elem = sift.Must(sift.ToValue(nil))
		}
		elems[i] = elem
	}
	return elems, nil
}

func iterateOpt(v sift.Value) ([]sift.Value, error) {
	if _, ok := v.(sift.Index); !ok {
		return nil, nil
	}
	return iterate(v)
}

func constructObject(attrs []sift.Value) ([]sift.Value, error) {
	if len(attrs)%2 != 0 {
		panic("constructObject with odd number of operands")
	}
	m := make(map[string]sift.Value)
	for ; len(attrs) > 0; attrs = attrs[2:] {
		key, ok := sift.AsString(attrs[0])
		if !ok {
			return nil, fmt.Errorf("cannot use value %v as object key", attrs[0])
		}
		m[key] = attrs[1]
	}
	out := sift.Must(sift.ToValue(m))
	return []sift.Value{out}, nil
}

func neg(v sift.Value) (sift.Value, error) {
	n, ok := sift.AsFloat64(v)
	if !ok {
		return nil, fmt.Errorf("cannot negate value %v", v)
	}
	out := sift.Must(sift.ToValue(-n))
	return out, nil
}

func binop(op func(xv, yv sift.Value) (sift.Value, error)) func(xf, yf sift.Filter) sift.Filter {
	return func(xf, yf sift.Filter) sift.Filter {
		return sift.Binary(xf, yf, func(x, y sift.Value) ([]sift.Value, error) {
			v, err := op(x, y)
			if err != nil {
				return nil, err
			}
			return []sift.Value{v}, nil
		})
	}
}

func add(x, y sift.Value) (sift.Value, error) {
	if xn, ok := sift.AsFloat64(x); ok {
		yn, ok := sift.AsFloat64(y)
		if !ok {
			return nil, fmt.Errorf("cannot use numeric operator on value %v", y)
		}
		return sift.Must(sift.ToValue(xn + yn)), nil
	} else if xs, ok := sift.AsString(x); ok {
		ys, ok := sift.AsString(y)
		if !ok {
			return nil, fmt.Errorf("cannot concatenate string with value %v", y)
		}
		return sift.Must(sift.ToValue(xs + ys)), nil
	} else if xl, ok := x.(sift.Index); ok {
		yl, ok := y.(sift.Index)
		if !ok {
			return nil, fmt.Errorf("cannot concatenate array with value %v", y)
		}
		xlen := xl.Length()
		ylen := yl.Length()
		outs := make([]sift.Value, 0, xlen+ylen)
		for xi := 0; xi < xlen; xi++ {
			elem, ok := xl.Index(xi)
			if ok {
				outs = append(outs, elem)
			}
		}
		for yi := 0; yi < ylen; yi++ {
			elem, ok := yl.Index(yi)
			if ok {
				outs = append(outs, elem)
			}
		}
		return sift.Must(sift.ToValue(outs)), nil
	} else if xa, ok := x.(sift.Attr); ok {
		ya, ok := y.(sift.Attr)
		if !ok {
			return nil, fmt.Errorf("cannot concatenate object with value %v", y)
		}
		out := make(map[string]sift.Value)
		for _, ykey := range ya.Keys() {
			ykeyStr, ok := sift.AsString(ykey)
			if !ok {
				return nil, fmt.Errorf("concatenated map has non-string key %v", ykey)
			}
			value, ok := ya.Attr(ykey)
			if ok {
				out[ykeyStr] = value
			}
		}
		for _, xkey := range xa.Keys() {
			xkeyStr, ok := sift.AsString(xkey)
			if !ok {
				return nil, fmt.Errorf("concatenated map has non-string key %v", xkey)
			}
			value, ok := xa.Attr(xkey)
			if ok {
				out[xkeyStr] = value
			}
		}
		return sift.Must(sift.ToValue(out)), nil
	} else {
		return nil, fmt.Errorf("cannot use numeric operator on values %v and %v", x, y)
	}
}

func sub(x, y sift.Value) (sift.Value, error) {
	if xn, ok := sift.AsFloat64(x); ok {
		yn, ok := sift.AsFloat64(y)
		if !ok {
			return nil, fmt.Errorf("cannot use numeric operator on value %v", y)
		}
		return sift.Must(sift.ToValue(xn - yn)), nil
	} else if xl, ok := x.(sift.Index); ok {
		yl, ok := y.(sift.Index)
		if !ok {
			return nil, fmt.Errorf("cannot substract value %v from list", y)
		}
		xlen := xl.Length()
		ylen := yl.Length()
		outs := make([]sift.Value, 0, xlen)
	Outer:
		for xi := 0; xi < xlen; xi++ {
			xelem, ok := xl.Index(xi)
			if !ok {
				continue
			}
			for yi := 0; yi < ylen; yi++ {
				yelem, ok := yl.Index(yi)
				if !ok {
					continue
				}
				if sift.Equal(xelem, yelem) {
					continue Outer
				}
			}
			outs = append(outs, xelem)
		}
		return sift.Must(sift.ToValue(outs)), nil
	} else {
		return nil, fmt.Errorf("cannot use numeric operator on values %v and %v", x, y)
	}
}

func numOp(op func(xn, yn float64) float64) func(x, y sift.Filter) sift.Filter {
	return func(x, y sift.Filter) sift.Filter {
		return sift.Binary(x, y, func(xv, yv sift.Value) ([]sift.Value, error) {
			xn, ok := sift.AsFloat64(xv)
			if !ok {
				return nil, fmt.Errorf("cannot use numeric operator on value %v", xv)
			}
			yn, ok := sift.AsFloat64(yv)
			if !ok {
				return nil, fmt.Errorf("cannot use numeric operator on value %v", yv)
			}
			v := sift.Must(sift.ToValue(op(xn, yn)))
			return []sift.Value{v}, nil
		})
	}
}

func walk(v sift.Value) ([]sift.Value, error) {
	var outs []sift.Value
	var visit func(v sift.Value)
	visit = func(v sift.Value) {
		outs = append(outs, v)
		if attr, ok := v.(sift.Attr); ok {
			for _, key := range attr.Keys() {
				value, ok := attr.Attr(key)
				if ok {
					visit(value)
				}
			}
		}
		if index, ok := v.(sift.Index); ok {
			n := index.Length()
			for i := 0; i < n; i++ {
				value, ok := index.Index(i)
				if ok {
					visit(value)
				}
			}
		}
	}
	visit(v)
	return outs, nil
}
