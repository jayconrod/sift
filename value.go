package sift

import (
	"fmt"
	"sort"
)

// A Value is an element that may be processed and filtered by sift.
// The actual representation of a value is abstract. This interface
// and a number of related interfaces are used to describe values.
// This package provides implementations for basic types, but encoding
// packages may provide their own implementations.
type Value interface {
	// Truth returns true if the value should be considered true in
	// conditional expressions.
	Truth() bool
}

// Null is implemented by null values.
type Null interface {
	Value

	// IsNull returns whether the value is null.
	IsNull() bool
}

// IsNull returns whether a value is null.
func IsNull(v Value) bool {
	if n, ok := v.(Null); ok && n.IsNull() {
		return true
	}
	return false
}

// Bool is implemented by boolean values.
type Bool interface {
	Value

	// IsBool returns whether the value is boolean. Use Truth to find the
	// value of the boolean.
	IsBool() bool
}

// AsBool returns a bool and true if v implements Bool. Otherwise,
// false and false are returned.
func AsBool(v Value) (bool, bool) {
	if b, ok := v.(Bool); ok && b.IsBool() {
		return b.Truth(), true
	}
	return false, false
}

// Float64 is implemented by double-precision floating point values.
type Float64 interface {
	Value

	// IsFloat64 returns whether the value is a float64.
	IsFloat64() bool

	// Float64 returns the number this value represents.
	Float64() float64
}

// AsFloat64 returns a number and true if v implements Float64. Otherwise,
// 0 and false are returned.
func AsFloat64(v Value) (float64, bool) {
	if f, ok := v.(Float64); ok && f.IsFloat64() {
		return f.Float64(), true
	}
	return 0, false
}

// String is implemented by strings.
type String interface {
	Value

	// IsString returns whether the value is a string.
	IsString() bool

	// String returns the string this value represents.
	String() string
}

// AsString returns a string and true if v implements String. Otherwise,
// "" and false are returned.
func AsString(v Value) (string, bool) {
	if s, ok := v.(String); ok && s.IsString() {
		return s.String(), true
	}
	return "", false
}

// Attr is implemented by values that have named attributes.
type Attr interface {
	Value

	// Keys returns a list of keys of attributes that this value has.
	// Attr must return a value for each of these; however, Attr may return
	// values for keys not included in this list.
	Keys() []Value

	// Attr returns the value of an attribute named by key and true.
	// If the value has no such attribute, nil and false are returned.
	Attr(key Value) (Value, bool)
}

// GetAttr returns the value of v's attribute named by name and true.
// If v has no such attribute (or does not implement Attr), nil and false
// are returned.
func GetAttr(v, key Value) (Value, bool) {
	if a, ok := v.(Attr); ok {
		return a.Attr(key)
	}
	return nil, false
}

// GetStringAttr is like GetAttr, but accepts string keys instead of Values.
func GetStringAttr(v Value, name string) (Value, bool) {
	key := stringType(name)
	return GetAttr(v, key)
}

// Index is implemented by values that have numbered child elements like arrays.
type Index interface {
	Value

	// Length returns one plus the greatest integer for which Index returns true.
	// If Index returns false for all values, Length returns 0.
	// Normally Length is the number of elements in a list, but it may be
	// greater if there are "holes".
	Length() int

	// Index returns the value at index i and true if the instance has a value
	// at that index. Otherwise, nil and false are returned.
	// i must be non-negative.
	Index(i int) (Value, bool)
}

// Length returns the v's Length and true if v satisfies Index or is a string.
// Otherwise, 0 and false are returned.
func Length(v Value) (int, bool) {
	if i, ok := v.(Index); ok {
		return i.Length(), true
	} else if s, ok := AsString(v); ok {
		return len(s), true
	} else {
		return 0, false
	}
}

// GetIndex returns the value at v's index i and true. If v does not implement
// Index or if i does not implement Float64 or i is not a non-negative integer,
// or if v.Index returns false, nil and false are returned.
func GetIndex(v, i Value) (Value, bool) {
	ni, ok := AsFloat64(i)
	if !ok {
		return nil, false
	}
	ii := int(ni)
	if float64(ii) != ni {
		// inexact conversion
		return nil, false
	}
	if ii < 0 {
		return nil, false
	}
	return GetIntIndex(v, ii)
}

// GetIntIndex is like GetIndex, but accepts an integer index
// instead of a value.
func GetIntIndex(v Value, i int) (Value, bool) {
	ix, ok := v.(Index)
	if !ok {
		return nil, false
	}
	return ix.Index(i)
}

// Equal returns whether two values are equivalent.
func Equal(l, r Value) bool {
	if IsNull(l) {
		return IsNull(r)
	} else if _, ok := l.(Bool); ok {
		_, ok := r.(Bool)
		return ok && l.Truth() == r.Truth()
	} else if lf, ok := l.(Float64); ok {
		rf, ok := r.(Float64)
		return ok && lf.Float64() == rf.Float64()
	} else if ls, ok := l.(String); ok {
		rs, ok := r.(String)
		return ok && ls == rs
	} else if la, ok := l.(Attr); ok {
		ra, ok := r.(Attr)
		if !ok {
			return false
		}
		lkeys, rkeys := la.Keys(), ra.Keys()
		if len(lkeys) != len(rkeys) {
			return false
		}
		for i, lkey := range lkeys {
			rkey := rkeys[i]
			if !Equal(lkey, rkey) {
				return false
			}
			lvalue, ok := la.Attr(lkey)
			if !ok {
				return false
			}
			rvalue, ok := ra.Attr(rkey)
			if !ok {
				return false
			}
			if !Equal(lvalue, rvalue) {
				return false
			}
		}
		return true
	} else if li, ok := l.(Index); ok {
		ri, ok := r.(Index)
		if !ok {
			return false
		}
		ln, rn := li.Length(), ri.Length()
		if ln != rn {
			return false
		}
		for i := 0; i < ln; i++ {
			le, lok := li.Index(i)
			re, rok := ri.Index(i)
			if lok != rok || lok && !Equal(le, re) {
				return false
			}
		}
		return true
	} else {
		return false
	}
}

// ToValue converts an arbitrary value to an implementation of Value.
//
// Null is returned for nil values.
//
// A Bool is return for bool values.
//
// A Float64 is returned for float64 values.
//
// An Attr is returned for map[string]interface{} values. The keys are sorted.
// The values are converted to Values recursively.
//
// An Index is returned for []inteface{} and []sift.Value values.
//
// An error is returned for all other values.
func ToValue(v interface{}) (Value, error) {
	switch v := v.(type) {
	case Value:
		return v, nil
	case nil:
		return NullValue, nil
	case bool:
		return boolType(v), nil
	case int8:
		return float64Type(v), nil
	case int16:
		return float64Type(v), nil
	case int32:
		return float64Type(v), nil
	case uint8:
		return float64Type(v), nil
	case uint16:
		return float64Type(v), nil
	case uint32:
		return float64Type(v), nil
	case float64:
		return float64Type(v), nil
	case int:
		f := float64Type(v)
		if int(f) != v {
			return nil, fmt.Errorf("cannot represent as value: %#v", v)
		}
		return f, nil
	case int64:
		f := float64Type(v)
		if int64(f) != v {
			return nil, fmt.Errorf("cannot represent as value: %#v", v)
		}
		return f, nil
	case uint:
		f := float64Type(v)
		if uint(f) != v {
			return nil, fmt.Errorf("cannot represent as value: %#v", v)
		}
		return f, nil
	case uint64:
		f := float64Type(v)
		if uint64(f) != v {
			return nil, fmt.Errorf("cannot represent as value: %#v", v)
		}
		return f, nil
	case uintptr:
		f := float64Type(v)
		if uintptr(f) != v {
			return nil, fmt.Errorf("cannot represent as value: %#v", v)
		}
		return f, nil
	case string:
		return stringType(v), nil
	case map[string]interface{}:
		m := v
		vm := make(attrType)
		for k, v := range m {
			if value, err := ToValue(v); err != nil {
				return nil, err
			} else {
				vm[k] = value
			}
		}
		return vm, nil
	case []interface{}:
		l := v
		ix := make(indexType, len(l))
		for j, e := range l {
			v, err := ToValue(e)
			if err != nil {
				return nil, err
			}
			ix[j] = v
		}
		return ix, nil
	case []Value:
		return indexType(v), nil
	default:
		return nil, fmt.Errorf("cannot represent as value: %#v", v)
	}
}

func Must(v Value, err error) Value {
	if err != nil {
		panic(err)
	}
	return v
}

type nullType struct{}

// NullValue is a value that satisfies Null and returns true for IsNull.
// It is not the only such value.
var NullValue Value = nullType{}

func (nullType) Truth() bool  { return false }
func (nullType) IsNull() bool { return true }

type boolType bool

func (b boolType) Truth() bool  { return bool(b) }
func (b boolType) IsBool() bool { return true }

type float64Type float64

var _ Float64 = float64Type(0)

func (f float64Type) Truth() bool      { return f != 0 }
func (f float64Type) IsFloat64() bool  { return true }
func (f float64Type) Float64() float64 { return float64(f) }

type stringType string

func (s stringType) Truth() bool    { return s != "" }
func (s stringType) IsString() bool { return true }
func (s stringType) String() string { return string(s) }

type attrType map[string]Value

func (a attrType) Truth() bool { return true }

func (a attrType) Keys() []Value {
	keys := make([]Value, 0, len(a))
	for key := range a {
		keys = append(keys, stringType(key))
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].(stringType) < keys[j].(stringType)
	})
	return keys
}

func (a attrType) Attr(key Value) (Value, bool) {
	name, ok := AsString(key)
	if !ok {
		return nil, false
	}
	value, ok := a[name]
	return value, ok
}

type indexType []Value

func (ix indexType) Truth() bool { return true }

func (ix indexType) Length() int { return len(ix) }

func (ix indexType) Index(i int) (Value, bool) {
	if i < 0 || i >= len(ix) || ix[i] == nil {
		return nil, false
	}
	return ix[i], true
}
