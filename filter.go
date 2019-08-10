package sift

import (
	"io"
)

// A Decoder reads values from a stream of data in an unspecified format.
// For example, a JSON decoder would read text and return values described
// by that text.
type Decoder interface {
	Decode() (Value, error)
}

// An Encoder writes values to a stream of data in an unspecified format.
// For example, an JSON encoder would transform values into JSON text.
type Encoder interface {
	Encode(Value) error
}

// A Filter reads and transforms a value. The value may have been produced
// by a Decoder or another Filter, so its representation may not be known.
// Zero or more values may be emitted.
type Filter func(v Value) ([]Value, error)

// Literal returns a Filter that always returns a singleton slice with
// the given value.
func Literal(v Value) Filter {
	s := []Value{v}
	return func(Value) ([]Value, error) { return s, nil }
}

// Map returns a Filter that produces a value for each value it consumes,
// transforming values with the given function.
func Map(f func(Value) Value) Filter {
	return func(vin Value) ([]Value, error) {
		vout := f(vin)
		return []Value{vout}, nil
	}
}

// FlatMap returns a Filter that produces a slice of values (but no error)
// for each value it consumes, transforming values with the given function.
func FlatMap(f func(Value) []Value) Filter {
	return func(vin Value) ([]Value, error) {
		return f(vin), nil
	}
}

// Compose returns a Filter that applies f to a value, then applies g to
// each value of the result. An error is returned if either f or g return
// an error for any value.
func Compose(f, g Filter) Filter {
	return func(v Value) ([]Value, error) {
		vfs, err := f(v)
		if err != nil {
			return nil, err
		}
		var vout []Value
		for _, vf := range vfs {
			vg, err := g(vf)
			if err != nil {
				return nil, err
			}
			vout = append(vout, vg...)
		}
		return vout, nil
	}
}

// Binary returns a filter that applies x and y to an input value, then applies
// op to the Cartesian product of the outputs of x and y.
func Binary(x, y Filter, op func(xv, yv Value) ([]Value, error)) Filter {
	return func(v Value) ([]Value, error) {
		xvs, err := x(v)
		if err != nil {
			return nil, err
		}
		yvs, err := y(v)
		if err != nil {
			return nil, err
		}
		var outs []Value
		for _, xv := range xvs {
			for _, yv := range yvs {
				opvs, err := op(xv, yv)
				if err != nil {
					return nil, err
				}
				outs = append(outs, opvs...)
			}
		}
		return outs, nil
	}
}

// Ternary returns a filter that applies x, y, and z to an input value, then
// applies op to the Cartesian product of the outputs of x, y, and z.
func Ternary(x, y, z Filter, op func(xv, yv, zv Value) ([]Value, error)) Filter {
	return func(v Value) ([]Value, error) {
		xvs, err := x(v)
		if err != nil {
			return nil, err
		}
		yvs, err := y(v)
		if err != nil {
			return nil, err
		}
		zvs, err := z(v)
		if err != nil {
			return nil, err
		}
		var outs []Value
		for _, xv := range xvs {
			for _, yv := range yvs {
				for _, zv := range zvs {
					opvs, err := op(xv, yv, zv)
					if err != nil {
						return nil, err
					}
					outs = append(outs, opvs...)
				}
			}
		}
		return outs, nil
	}
}

// Concat applies x and y to an input value and returns the outputs of x
// followed by the outputs of y.
func Concat(x, y Filter) Filter {
	return func(v Value) ([]Value, error) {
		xvs, err := x(v)
		if err != nil {
			return nil, err
		}
		yvs, err := y(v)
		if err != nil {
			return nil, err
		}
		outs := xvs[:len(xvs):len(xvs)]
		outs = append(outs, yvs...)
		return outs, nil
	}
}

// Sift reads values from dec, transforms them with f, and encodes the results
// with enc until an error occurs. When dec returns io.EOF, Sift stops and
// returns nil.
func Sift(dec Decoder, f Filter, enc Encoder) error {
	for {
		vin, err := dec.Decode()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		vouts, err := f(vin)
		if err != nil {
			return err
		}
		for _, vout := range vouts {
			if err := enc.Encode(vout); err != nil {
				return err
			}
		}
	}
}
