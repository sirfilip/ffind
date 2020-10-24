package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	filename := flag.String("name", "", "file name")
	flag.Parse()

	if *filename == "" {
		usage()
	}

	lookupPath := flag.Arg(0)

	if lookupPath == "" {
		lookupPath = "."
	}
	filepath.Walk(lookupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if filepath.Base(path) == *filename {
			fmt.Println(path)
		}
		return nil
	})
}

func usage() {
	fmt.Println("Usage: ffind -n filename [ startdir ]")
	os.Exit(1)
}
