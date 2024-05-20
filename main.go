package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/zregvart/tkn-fmt/format"
)

var (
	list  = flag.Bool("l", false, "list files whose formatting differs from tkn-fmt's")
	write = flag.Bool("w", false, "write result to (source) file instead of stdout")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: tkn fmt [flags] [path ...]\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 && *write {
		fmt.Fprintf(os.Stderr, "error: cannot use -w with standard input")
		os.Exit(1)
	}

	if len(args) == 0 {
		format.Format(os.Stdin, os.Stdout)
	} else {
		for _, path := range args {
			in, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: cannot read file: %q %s", path, err)
				os.Exit(1)
			}

			var out bytes.Buffer
			if err := format.Format(bytes.NewBuffer(in), &out); err != nil {
				fmt.Fprintf(os.Stderr, "error: %s", err)
				os.Exit(1)
			}

			if out := out.Bytes(); !bytes.Equal(in, out) {
				if *list {
					fmt.Println(path)
				}

				if !*write {
					continue
				}

				s, err := os.Stat(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: unable to stat file: %q, %s", path, err)
					os.Exit(1)
				}

				os.WriteFile(path, out, s.Mode().Perm())
			}
		}
	}
}
