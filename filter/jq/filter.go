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
