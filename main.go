package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	// "github.com/pkg/profile"
	"golang.org/x/sync/errgroup"
)

func main() {
	// defer profile.Start(profile.MemProfile, profile.ProfilePath(".")).Stop()
	STDOUT := bufio.NewWriter(os.Stdout)
	defer STDOUT.Flush()

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
		fmt.Fprintln(STDOUT, res)
	}
}

func usage() {
	fmt.Println("Usage: ffind -n filename -c concurrency [ startdir ]")
	os.Exit(1)
}

func queue() (chan<- string, <-chan string) {
	in := make(chan string)
	out := make(chan string)

	go func() {
		buff := make([]string, 0)
		outCh := func() chan string {
			if len(buff) == 0 {
				return nil
			}
			return out
		}
		currVal := func() string {
			if len(buff) == 0 {
				return ""
			}
			return buff[0]
		}
	loop:
		for {
			select {
			case val, ok := <-in:
				if !ok {
					break loop
				}
				buff = append(buff, val)
			case outCh() <- currVal():
				buff = buff[1:]
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
	in, out := queue()

	readDir := func(root, filename string, results chan<- string, dirs chan<- string, wg *sync.WaitGroup) error {
		if filter(root) {
			return nil
		}
		entries, err := filepath.Glob(root + "/*")
		if err != nil {
			return fmt.Errorf("in glob: %w", err)
		}
		for _, entry := range entries {
			if filter(entry) {
				continue
			}
			if filepath.Base(entry) == filename {
				results <- entry
				continue
			}
			wg.Add(1)
			dirs <- entry
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
