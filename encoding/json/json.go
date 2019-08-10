package json

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"go.jayconrod.com/sift"
)

type value struct {
	i interface{}
}

var (
	_ sift.Null    = value{}
	_ sift.Bool    = value{}
	_ sift.Float64 = value{}
	_ sift.String  = value{}
)

func (v value) Truth() bool {
	if v.i == nil {
		return false
	}
	switch i := v.i.(type) {
	case bool:
		return i
	case float64:
		return i != 0
	case string:
		return i != ""
	default:
		return true
	}
}

func (v value) IsNull() bool { return v.i == nil }

func (v value) IsBool() bool {
	_, ok := v.i.(bool)
	return ok
}

func (v value) IsFloat64() bool {
	_, ok := v.i.(float64)
	return ok
}

func (v value) Float64() float64 {
	if f, ok := v.i.(float64); !ok {
		return 0
	} else {
		return f
	}
}

func (v value) IsString() bool {
	_, ok := v.i.(string)
	return ok
}

func (v value) String() string {
	if s, ok := v.i.(string); !ok {
		return ""
	} else {
		return s
	}
}

type attrValue map[string]interface{}

var _ sift.Attr = attrValue(nil)

func (v attrValue) Truth() bool {
	return true
}

func (v attrValue) Keys() []sift.Value {
	// TODO: should this return the keys in the order they appeared in source?
	// The JSON decoder doesn't give us that.
	keyStrings := make([]string, 0, len(v))
	for keyString := range v {
		keyStrings = append(keyStrings, keyString)
	}
	sort.Strings(keyStrings)
	keys := make([]sift.Value, len(v))
	for i, keyString := range keyStrings {
		key, err := sift.ToValue(keyString)
		if err != nil {
			panic(err)
		}
		keys[i] = key
	}
	return keys
}

func (v attrValue) Attr(key sift.Value) (sift.Value, bool) {
	s, ok := sift.AsString(key)
	if !ok {
		return nil, false
	}
	i, ok := v[s]
	if !ok {
		return nil, false
	}
	value, err := sift.ToValue(i)
	if err != nil {
		panic(err) // all JSON values should be representable
	}
	return value, true
}

type indexValue []interface{}

var _ sift.Index = indexValue(nil)

func (v indexValue) Truth() bool {
	return true
}

func (v indexValue) Length() int {
	return len(v)
}

func (v indexValue) Index(i int) (sift.Value, bool) {
	if i < 0 || len(v) <= i {
		return nil, false
	}
	elem, err := sift.ToValue(v[i])
	if err != nil {
		return nil, false
	}
	return elem, true
}

type decoder struct {
	dec *json.Decoder
}

// NewDecoder returns a JSON decoder that reads from r and returns
// sift elements until it reaches the end of the input.
func NewDecoder(r io.Reader) sift.Decoder {
	return &decoder{dec: json.NewDecoder(r)}
}

func (d *decoder) Decode() (sift.Value, error) {
	var raw interface{}
	if err := d.dec.Decode(&raw); err != nil {
		return nil, err
	}
	if obj, ok := raw.(map[string]interface{}); ok {
		return attrValue(obj), nil
	} else if arr, ok := raw.([]interface{}); ok {
		return indexValue(arr), nil
	} else {
		return value{raw}, nil
	}
}

type encoder struct {
	enc *json.Encoder
}

// NewEncoder returns a JSON encoder that encodes sift elements
// as JSON, which is written to w.
func NewEncoder(w io.Writer) sift.Encoder {
	return &encoder{enc: json.NewEncoder(w)}
}

func (e *encoder) Encode(v sift.Value) error {
	i, err := toJSONValue(v)
	if err != nil {
		return err
	}
	return e.enc.Encode(i)
}

func toJSONValue(v sift.Value) (interface{}, error) {
	if jsonValue, ok := v.(value); ok {
		return jsonValue.i, nil
	} else if sift.IsNull(v) {
		return nil, nil
	} else if b, ok := sift.AsBool(v); ok {
		return b, nil
	} else if f, ok := sift.AsFloat64(v); ok {
		return f, nil
	} else if s, ok := sift.AsString(v); ok {
		return s, nil
	} else if a, ok := v.(sift.Attr); ok {
		keys := a.Keys()
		m := make(map[string]interface{})
		for _, key := range keys {
			s, ok := sift.AsString(key)
			if !ok {
				return nil, fmt.Errorf("key %#v is not a string", key)
			}
			sv, ok := a.Attr(key)
			if !ok {
				return nil, fmt.Errorf("no value for key %q", key)
			}
			value, err := toJSONValue(sv)
			if err != nil {
				return nil, err
			}
			m[s] = value
		}
		return m, nil
	} else if i, ok := v.(sift.Index); ok {
		n := i.Length()
		list := make([]interface{}, n)
		for j := 0; j < n; j++ {
			v, ok := i.Index(j)
			if !ok {
				return nil, fmt.Errorf("value at index %d missing", j)
			}
			elem, err := toJSONValue(v)
			if err != nil {
				return nil, err
			}
			list[j] = elem
		}
		return list, nil
	} else {
		return nil, fmt.Errorf("cannot represent value %#v in JSON", v)
	}
}
