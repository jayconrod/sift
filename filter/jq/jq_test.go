package jq_test

import (
	"strings"
	"testing"

	"go.jayconrod.com/sift"
	"go.jayconrod.com/sift/encoding/json"
	"go.jayconrod.com/sift/filter/jq"
)

func TestFilter(t *testing.T) {
	for _, tc := range []struct {
		desc, program, input, want, wantErr string
	}{
		{
			desc:    "id",
			program: "",
			input:   "1",
			want:    "1",
		}, {
			desc:    "lit_null",
			program: "null",
			input:   "1",
			want:    "null",
		}, {
			desc:    "lit_true",
			program: "true",
			input:   "null",
			want:    "true",
		}, {
			desc:    "lit_false",
			program: "false",
			input:   "null",
			want:    "false",
		}, {
			desc:    "lit_num",
			program: "12.3",
			input:   "null",
			want:    "12.3",
		}, {
			desc:    "lit_num_imprecise",
			program: "1234567890123456789",
			input:   "null",
			want:    "1234567890123456800",
		}, {
			desc:    "lit_num_range",
			program: "-1e10000",
			input:   "null",
			want:    "-1.7976931348623157e+308",
		}, {
			desc:    "lit_string",
			program: `"foo"`,
			input:   "null",
			want:    `"foo"`,
		}, {
			desc:    "dot",
			program: ".",
			input: `
null
12
"abc"
{"x":34}
`,
			want: `
null
12
"abc"
{"x":34}
`,
		}, {
			desc:    "whitespace",
			program: " \t.\n\r ",
			input:   "12",
			want:    "12",
		}, {
			desc: "comment",
			program: `
# com
. # com
# com
`,
			input: "12",
			want:  "12",
		}, {
			desc:    "field",
			program: `.x`,
			input:   `{"x":12}{"x":34}`,
			want: `
12
34
`,
		}, {
			desc:    "field_quote",
			program: `."☃"`,
			input:   `{"☃":12}`,
			want:    "12",
		}, {
			desc:    "field_not_object",
			program: `.x`,
			input:   `12`,
			want:    `null`,
		}, {
			desc:    "field_missing",
			program: `.x`,
			input:   `{}`,
			want:    `null`,
		}, {
			desc:    "fields",
			program: `.x.y.z`,
			input:   `{"x":{"y":{"z":12}}}`,
			want:    "12",
		}, {
			desc:    "field_opt_present",
			program: `.x?`,
			input:   `{"x":12}`,
			want:    "12",
		}, {
			desc:    "field_opt_missing",
			program: `.x?`,
			input:   `{}`,
			want:    "",
		}, {
			desc:    "array_construct_empty",
			program: `[]`,
			input:   `true`,
			want:    `[]`,
		}, {
			desc:    "array_construct",
			program: `[., .]`,
			input:   `1 2`,
			want: `
[1,1]
[2,2]
`,
		}, {
			desc:    "array_construct_group",
			program: `[("a","b"),(1,2)]`,
			input:   `true`,
			want:    `["a","b",1,2]`,
		}, {
			desc:    "object_construct_empty",
			program: `{}`,
			input:   `true`,
			want:    `{}`,
		}, {
			desc:    "object_construct",
			program: `{a:1}`,
			input:   `true`,
			want:    `{"a":1}`,
		}, {
			desc:    "object_construct_string",
			program: `{"a":1}`,
			input:   `true`,
			want:    `{"a":1}`,
		}, {
			desc:    "object_construct_expr",
			program: `{1:2}`,
			input:   `true`,
			wantErr: "expected attribute name or }",
		}, {
			desc:    "object_construct_group_not_string",
			program: `{(1):2}`,
			input:   `true`,
			wantErr: "cannot use value",
		}, {
			desc:    "object_construct_group",
			program: `{("a"):1}`,
			input:   `true`,
			want:    `{"a":1}`,
		}, {
			desc:    "object_construct_trailing_comma",
			program: `{"a":1,b:2}`,
			input:   `true`,
			want:    `{"a":1,"b":2}`,
		}, {
			desc:    "object_construct_pipe",
			program: `{a:1|2,b:3|4}`,
			input:   `true`,
			want:    `{"a":2,"b":4}`,
		}, {
			desc:    "object_construct_product",
			program: `{("a","b"):(1,2)}`,
			input:   `true`,
			want: `
{"a":1}
{"a":2}
{"b":1}
{"b":2}
`,
		}, {
			desc:    "array_index",
			program: `.[0]`,
			input:   `["a"]`,
			want:    `"a"`,
		}, {
			desc:    "array_index_bound",
			program: `.[1]`,
			input:   `[]`,
			want:    `null`,
		}, {
			desc:    "array_index_neg",
			program: `.[-1]`,
			input:   `["a", "b"]`,
			want:    `"b"`,
		}, {
			desc:    "array_index_neg_bound",
			program: `.[-5]`,
			input:   `[]`,
			want:    `null`,
		}, {
			desc:    "array_index_string",
			program: `.["a"]`,
			input:   `[]`,
			wantErr: "cannot index array with value",
		}, {
			desc:    "array_index_not_array",
			program: `.[0]`,
			input:   `"a"`,
			wantErr: "cannot index value",
		}, {
			desc:    "field_array_index",
			program: `.a[0]`,
			input:   `{"a":["b"]}`,
			want:    `"b"`,
		}, {
			desc:    "array_slice",
			program: `.[1:3]`,
			input:   `["a","b","c","d"]`,
			want:    `["b","c"]`,
		}, {
			desc:    "array_slice_bound",
			program: `.[0:1]`,
			input:   `[]`,
			want:    `[]`,
		}, {
			desc:    "array_slice_neg",
			program: `.[0:-1]`,
			input:   `["a", "b"]`,
			want:    `["a"]`,
		}, {
			desc:    "array_slice_opt_begin",
			program: `.[:1]`,
			input:   `["a", "b"]`,
			want:    `["a"]`,
		}, {
			desc:    "array_slice_opt_end",
			program: `.[1:]`,
			input:   `["a", "b"]`,
			want:    `["b"]`,
		}, {
			desc:    "array_slice_opt_both",
			program: `.[:]`,
			input:   `["a", "b"]`,
			wantErr: "expected expression",
		}, {
			desc:    "array_slice_float",
			program: `.[1.9:2.1]`,
			input:   `["a","b","c","d"]`,
			want:    `["b"]`,
		}, {
			desc:    "array_slice_string",
			program: `.[:"foo"]`,
			input:   `[]`,
			wantErr: "index must be number",
		}, {
			desc:    "string_slice",
			program: `.[1:-1]`,
			input:   `"abc"`,
			want:    `"b"`,
		}, {
			desc:    "array_iter",
			program: ".[]",
			input:   `[1,2]`,
			want: `
1
2
`,
		}, {
			desc:    "array_iter_opt",
			program: `.[]?`,
			input:   `1`,
			want:    "",
		}, {
			desc:    "field_array_iter",
			program: `.a[]`,
			input:   `{"a":[1,2,3]}`,
			want: `
1
2
3
`,
		}, {
			desc:    "comma",
			program: `.[], .[]`,
			input:   `["a", "b"]`,
			want: `
"a"
"b"
"a"
"b"
`,
		}, {
			desc:    "comma_paren",
			program: `.[(1, 0)]`,
			input:   `["a", "b"]`,
			want: `
"b"
"a"
`,
		}, {
			desc:    "pipe",
			program: `.x|.y`,
			input:   `{"x":{"y":12}}`,
			want:    `12`,
		}, {
			desc:    "comma_pipe_prec",
			program: `1, 2 | 3`,
			input:   `null`,
			want: `
3
3
`,
		}, {
			desc:    "mul_div_mod",
			program: `12 / 2 % 4`,
			input:   `true`,
			want:    `2`,
		}, {
			desc:    "mul_strings",
			program: `"foo" * "bar"`,
			input:   `true`,
			wantErr: `cannot use numeric operator`,
		}, {
			desc:    "walk",
			program: `..`,
			input:   `{"a":[[1],[2]],"b":3}`,
			want: `
{"a":[[1],[2]],"b":3}
[[1],[2]]
[1]
1
[2]
2
3
`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			gotErr := func(when string, err error) {
				if tc.wantErr != "" {
					text := err.Error()
					if !strings.Contains(text, tc.wantErr) {
						t.Errorf("got error %q; want error with %q", text, tc.wantErr)
					}
				} else {
					t.Errorf("%s: %v", when, err)
				}
			}
			f, err := jq.Compile(tc.desc, tc.program)
			if err != nil {
				gotErr("jq.Compile", err)
				return
			}
			r := strings.NewReader(tc.input)
			dec := json.NewDecoder(r)
			w := &strings.Builder{}
			enc := json.NewEncoder(w)
			if err := sift.Sift(dec, f, enc); err != nil {
				gotErr("sift.Sift", err)
				return
			}
			got := strings.TrimSpace(w.String())
			want := strings.TrimSpace(tc.want)
			if tc.wantErr != "" {
				t.Errorf("got:\n%s\n\nwant error", got)
			} else if got != want {
				t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
			}
		})
	}
}
