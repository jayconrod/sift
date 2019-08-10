package jq

import (
	gotoken "go/token"

	"go.jayconrod.com/sift"
)

// Compile parses a jq program and returns the sift filter it describes.
func Compile(name, src string) (filter sift.Filter, err error) {
	fset := gotoken.NewFileSet()
	f := fset.AddFile(name, -1, len(src))
	s := newScanner(f, []byte(src))
	p := newParser(s)
	defer func() {
		r := recover()
		if r == nil {
			return
		} else if e, ok := r.(error); ok {
			filter, err = nil, e
		} else {
			panic(r)
		}
	}()
	return p.parse(), nil
}
