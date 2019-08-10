package json_test

import (
	"strings"
	"testing"

	"go.jayconrod.com/sift"
	"go.jayconrod.com/sift/encoding/json"
)

func TestAttr(t *testing.T) {
	for _, tc := range []struct {
		desc, text, name string
		want             sift.Value
	}{
		{
			desc: "null",
			text: "null",
			name: "x",
		}, {
			desc: "object",
			text: `{"x":12}`,
			name: "x",
			want: sift.Must(sift.ToValue(float64(12.))),
		}, {
			desc: "object_without_field",
			text: `{"x":12}`,
			name: "y",
			want: nil,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			r := strings.NewReader(tc.text)
			dec := json.NewDecoder(r)
			v, err := dec.Decode()
			if err != nil {
				t.Fatal(err)
			}
			attr, ok := sift.GetStringAttr(v, tc.name)
			if tc.want == nil {
				if ok {
					t.Errorf("ok: got false; want true")
				}
				if attr != nil {
					t.Errorf("attr: got %v; want nil", attr)
				}
			} else {
				if !ok {
					t.Errorf("ok: got false; want true")
				}
				if !sift.Equal(attr, tc.want) {
					t.Errorf("field: got %v; want %v", attr, tc.want)
				}
			}
		})
	}
}

func TestIndex(t *testing.T) {
	t.Run("array", func(t *testing.T) {
		r := strings.NewReader(`["a", "b"]`)
		dec := json.NewDecoder(r)
		v, err := dec.Decode()
		if err != nil {
			t.Fatal(err)
		}
		if n, ok := sift.Length(v); !ok {
			t.Errorf("Length: value %#v does not have length", v)
		} else if n != 2 {
			t.Errorf("Length: got %d; want 2", 2)
		}
		elem, ok := sift.GetIntIndex(v, 1)
		if !ok {
			t.Error("GetIndex: could not get 1")
		}
		want := sift.Must(sift.ToValue("b"))
		if !sift.Equal(elem, want) {
			t.Errorf("GetIndex: got %#v; want %#v", elem, want)
		}
		_, ok = sift.GetIntIndex(v, 3)
		if ok {
			t.Error("GetIndex: got value beyond end of array")
		}
	})

	t.Run("not_array", func(t *testing.T) {
		r := strings.NewReader("null")
		dec := json.NewDecoder(r)
		v, err := dec.Decode()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := sift.Length(v); ok {
			t.Errorf("%#v should not have length", v)
		}
		if _, ok := sift.GetIntIndex(v, 0); ok {
			t.Errorf("GetIndex: got index")
		}
	})
}

func TestEncode(t *testing.T) {
	for _, tc := range []struct {
		desc  string
		value sift.Value
		want  string
	}{
		{
			desc:  "null",
			value: sift.Must(sift.ToValue(nil)),
			want:  "null",
		}, {
			desc:  "bool",
			value: sift.Must(sift.ToValue(true)),
			want:  "true",
		}, {
			desc:  "float64",
			value: sift.Must(sift.ToValue(float64(12.3))),
			want:  "12.3",
		}, {
			desc:  "string",
			value: sift.Must(sift.ToValue("foo")),
			want:  `"foo"`,
		}, {
			desc: "object",
			value: sift.Must(sift.ToValue(map[string]interface{}{
				"foo": 12,
				"bar": 34,
			})),
			want: `{"bar":34,"foo":12}`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			w := &strings.Builder{}
			enc := json.NewEncoder(w)
			if err := enc.Encode(tc.value); err != nil {
				t.Fatal(err)
			}
			got := strings.TrimSpace(w.String())
			want := strings.TrimSpace(tc.want)
			if got != want {
				t.Errorf("got %s; want %s", got, want)
			}
		})
	}
}
