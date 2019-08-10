package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"go.jayconrod.com/sift"
	"go.jayconrod.com/sift/encoding/json"
	"go.jayconrod.com/sift/filter/jq"
)

func main() {
	log.SetPrefix("sift: ")
	log.SetFlags(0)
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("sift", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("expected exactly 1 argument; got %d", fs.NArg())
	}

	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	filter, err := jq.Compile("command-line", fs.Arg(0))
	if err != nil {
		return err
	}

	return sift.Sift(dec, filter, enc)
}
