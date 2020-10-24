package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sync/errgroup"
)

func main() {
	filename := flag.String("name", "", "file name")
	flag.Parse()

	if *filename == "" {
		usage()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal)
	go func() {
		<-c
		cancel()
		fmt.Println("Canceling...")
		os.Exit(1)
	}()

	lookupPath := flag.Arg(0)

	if lookupPath == "" {
		lookupPath = "."
	}

	result := make(chan string)
	go lookup(ctx, *filename, lookupPath, result)
	for found := range result {
		fmt.Println(found)
	}
}

func usage() {
	fmt.Println("Usage: ffind -n filename [ startdir ]")
	os.Exit(1)
}

func lookup(ctx context.Context, filename, root string, res chan<- string) {
	g, ctx := errgroup.WithContext(ctx)
	files := make(chan string)

	g.Go(func() error {
		defer close(files)
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}

			select {
			case files <- path:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	})

	const numWorkers = 20
	for i := 0; i < numWorkers; i++ {
		g.Go(func() error {
			for file := range files {
				if filepath.Base(file) != filename {
					continue
				}
				select {
				case res <- file:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}
	close(res)
}
