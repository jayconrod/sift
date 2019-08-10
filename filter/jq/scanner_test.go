package jq

import (
	gotoken "go/token"
	"testing"
)

func TestIdentifier(t *testing.T) {
	for _, tc := range []struct {
		text string
		ok   bool
	}{
		{"", false},
		{"x", true},
		{"xyz", true},
		{"x00", true},
		{"0", false},
		{"0x", false},
		{"_", true},
		{"_0", true},
		{"_a", true},
		{"a-", false},
		{"a-a", false},
		{"a.", false},
		{"𝔣𝔞𝔫𝔠𝔶123", true},
		{"123𝔣𝔞𝔫𝔠𝔶", false},
		{"☃", false},
	} {
		fset := gotoken.NewFileSet()
		file := fset.AddFile("test", -1, len(tc.text))
		s := newScanner(file, []byte(tc.text))
		_, tok, lit, err := s.scanOrError()
		if !tc.ok {
			if err == nil && tok == identifier && lit == tc.text {
				t.Errorf("%q: got %v; want something other than %v", tc.text, tok, identifier)
			}
		} else if err != nil {
			t.Errorf("%q: %v", tc.text, err)
		} else if tok != identifier {
			t.Errorf("%q: got token %v; want %v", tc.text, tok, identifier)
		} else if lit != tc.text {
			t.Errorf("%q: got literal %q", tc.text, lit)
		}
	}
}

func TestNumber(t *testing.T) {
	for _, tc := range []string{
		"0",
		"0",
		"12345678901234567890",
		"-12",
		"+12",
		".12",
		"-.12",
		"12.",
		"-12.",
		"12.34",
		"-12.34",
		"1e23",
		"1E23",
		"-1e23",
		"-1.2e3",
		"-.12e3",
		"1e+1",
		"1e-1",
	} {
		fset := gotoken.NewFileSet()
		file := fset.AddFile("test", -1, len(tc))
		s := newScanner(file, []byte(tc))
		_, tok, lit, err := s.scanOrError()
		if err != nil {
			t.Errorf("%q: %v", tc, err)
		} else if tok != number {
			t.Errorf("%q: got token %v; want %v", tc, tok, number)
		} else if lit != tc {
			t.Errorf("%q: got %q; want %q", tc, lit, tc)
		}
	}
}

func TestNumberBad(t *testing.T) {
	for _, tc := range []string{
		"",
		".",
		"--0",
		"-.",
		"-e",
		".e",
		"1e",
		"1e+",
		"1e--0",
	} {
		fset := gotoken.NewFileSet()
		file := fset.AddFile("test", -1, len(tc))
		s := newScanner(file, []byte(tc))
		_, tok, lit, err := s.scanOrError()
		if err == nil && tok == number && lit == tc {
			t.Errorf("%q: got number; want something else", tc)
		}
	}
}

func TestString(t *testing.T) {
	for _, tc := range []struct {
		text, want string
	}{
		{`""`, ""},
		{`"abc"`, "abc"},
		{`'abc'`, "abc"},
		{`"'"`, "'"},
		{`'"'`, `"`},
		{`"\"\'\\x"`, `"'\x`},
		{`"\n\r\v\t\b\f"`, "\n\r\v\t\b\f"},
		{`"\0 \62 \141 \377 \600 \29"`, "\x00 2 a ÿ 00 \x029"},
		{`"\u12345"`, "ሴ5"},
		{`"\x5A\x5a5a"`, "ZZ5a"},
	} {
		fset := gotoken.NewFileSet()
		file := fset.AddFile("test", -1, len(tc.text))
		s := newScanner(file, []byte(tc.text))
		_, tok, lit, err := s.scanOrError()
		if err != nil {
			t.Errorf("%q: %v", tc.text, err)
		} else if tok != str {
			t.Errorf("%q: got token %v; want %v", tc.text, tok, str)
		} else if lit != tc.want {
			t.Errorf("%q: got %q; want %q", tc.text, lit, tc.want)
		}
	}
}

func TestStringBadEscape(t *testing.T) {
	for _, tc := range []string{
		`"\y"`,
		`"\N"`,
		`"\U1234"`,
		`"\u12"`,
		`"\x5"`,
		`"\xG`,
	} {
		fset := gotoken.NewFileSet()
		file := fset.AddFile("test", -1, len(tc))
		s := newScanner(file, []byte(tc))
		_, _, _, err := s.scanOrError()
		if err == nil {
			t.Error("got nil; want error")
		}
	}
}
