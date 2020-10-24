package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

func main() {
	filename := flag.String("name", "", "file name")
	concurrency := flag.Int("c", 10, "concurrency")
	flag.Parse()

	if *filename == "" {
		usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	root := flag.Arg(0)

	if root == "" {
		root = "."
	}
	results := make(chan string)
	go perform(ctx, root, *filename, results, *concurrency, filter)
	for res := range results {
		fmt.Println(res)
	}
}

func usage() {
	fmt.Println("Usage: ffind -n filename -c concurrency [ startdir ]")
	os.Exit(1)
}

func stack() (chan<- string, <-chan string) {
	in := make(chan string)
	out := make(chan string)

	go func() {
		queue := make([]string, 0)
		outCh := func() chan string {
			if len(queue) == 0 {
				return nil
			}
			return out
		}
		currVal := func() string {
			if len(queue) == 0 {
				return ""
			}
			return queue[0]
		}
	loop:
		for {
			select {
			case val, ok := <-in:
				if !ok {
					break loop
				}
				queue = append(queue, val)
			case outCh() <- currVal():
				queue = queue[1:]
			}
		}
		close(out)
	}()

	return in, out
}

func filter(path string) bool {
	return strings.HasPrefix(filepath.Base(path), ".")
}

func perform(ctx context.Context, root, filename string, results chan<- string, concurrency int, filter func(string) bool) {
	g := new(errgroup.Group)
	in, out := stack()

	readDir := func(root, filename string, results chan<- string, dirs chan<- string, wg *sync.WaitGroup) error {
		if filter(root) {
			return nil
		}
		entries, err := filepath.Glob(root + "/*")
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if filter(entry) {
				continue
			}
			f, err := os.Open(entry)
			if err != nil {
				return err
			}
			stat, err := f.Stat()
			f.Close()
			if err != nil {
				return err
			}
			if stat.IsDir() {
				wg.Add(1)
				dirs <- entry
				continue
			}
			if filepath.Base(entry) == filename {
				results <- entry
			}
		}
		return nil
	}

	wg := new(sync.WaitGroup)
	wg.Add(1)
	in <- root

	go func() {
		wg.Wait()
		close(in)
	}()

	for i := 0; i < concurrency; i++ {
		g.Go(func() error {
			for dir := range out {
				err := readDir(dir, filename, results, in, wg)
				wg.Done()
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	g.Wait()
	close(results)
}
