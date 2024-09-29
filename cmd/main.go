package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type result struct {
	path string
	sum  [md5.Size]byte
	err  error
}

func sumFiles(done <-chan struct{}, root string) (<-chan result, <-chan error) {
	c := make(chan result)
	// use buffer for non-blocking
	errc := make(chan error, 1)
	go func() {
		var wg sync.WaitGroup
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				data, err := os.ReadFile(path)
				select {
				case c <- result{path: path, sum: md5.Sum(data), err: err}:
				case <-done:
					return
				}
			}()
			select {
			case <-done:
				return errors.New("walk cancelled")
			default:
				return nil
			}
		})
		go func() {
			wg.Wait()
			close(c)
		}()
		errc <- err
	}()
	return c, errc
}

func MD5All(root string) (map[string][md5.Size]byte, error) {
	done := make(chan struct{})
	defer close(done)
	c, errc := sumFiles(done, root)
	m := make(map[string][md5.Size]byte, 0)
	for r := range c {
		if r.err != nil {
			return nil, r.err
		}
		m[r.path] = r.sum
	}
	if err := <-errc; err != nil {
		return nil, err
	}
	return m, nil
}
func main() {
	m, err := MD5All(os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}
	var paths []string
	for path := range m {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Printf("%x %s\n", m[path], path)
	}
}
