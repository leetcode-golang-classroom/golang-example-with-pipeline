# golang-example-with-pipeline

This is a example with pipeline pattern with golang

## use Pipeline pattern to improve a md5 parse function

origin file
```golang
import (
	"crypto/md5"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func MD5All(root string) (map[string][md5.Size]byte, error) {
	m := make(map[string][md5.Size]byte, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		m[path] = md5.Sum(data)
		return nil
	})
	if err != nil {
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
```
## version 1: use goroutine to ReadFile concurrently

1. pass result and error by channel
2. default struct pass throught channel
```golang
type result struct {
	path string
	sum  [md5.Size]byte
	err  error
}
```
3. implement sumFiles by use goroutine to ReadFile and return result by channel
```golang
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
```
4. modify MD5All to receive result by channel
```golang
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
```