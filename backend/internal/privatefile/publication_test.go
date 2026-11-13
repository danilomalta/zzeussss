package privatefile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentPrivatePublication(t *testing.T) {
	dir, err := TempDir(t.TempDir(), "publication-")
	if err != nil {
		t.Fatal(err)
	}
	const n = 8
	paths := make([]string, n)
	for i := range paths {
		paths[i] = filepath.Join(dir, fmt.Sprint("source-", i))
		f, err := OpenNew(paths[i], os.O_RDWR)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(fmt.Sprint("synthetic-", i))); err != nil {
			t.Fatal(err)
		}
		if err = f.Sync(); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	dest := filepath.Join(dir, "archive")
	start := make(chan struct{})
	results := make(chan int, n)
	var wg sync.WaitGroup
	for i := range paths {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if PublishNew(paths[i], dest) == nil {
				results <- i
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	winners := []int{}
	for i := range results {
		winners = append(winners, i)
	}
	if len(winners) != 1 {
		t.Fatalf("publication winners=%d; expected exactly one", len(winners))
	}
	raw, err := os.ReadFile(dest)
	if err != nil || string(raw) != fmt.Sprint("synthetic-", winners[0]) {
		t.Fatal("published content mixed or replaced")
	}
	for i, p := range paths {
		if i != winners[0] {
			raw, err := os.ReadFile(p)
			if err != nil || string(raw) != fmt.Sprint("synthetic-", i) {
				t.Fatal("losing publisher lost its original")
			}
		}
	}
	info, err := os.Lstat(dest)
	if err != nil || !Regular(dest, info) {
		t.Fatal("published archive is not private")
	}
}

func TestConcurrentPrivateWriteNew(t *testing.T) {
	dir, err := TempDir(t.TempDir(), "write-")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "customer.key")
	start := make(chan struct{})
	results := make(chan int, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if WriteNew(path, []byte(fmt.Sprint("synthetic-", i))) == nil {
				results <- i
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	winners := []int{}
	for i := range results {
		winners = append(winners, i)
	}
	if len(winners) != 1 {
		t.Fatalf("private write winners=%d; expected exactly one", len(winners))
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != fmt.Sprint("synthetic-", winners[0]) {
		t.Fatal("private write mixed or overwritten")
	}
	info, err := os.Lstat(path)
	if err != nil || !Regular(path, info) {
		t.Fatal("private write permissions changed")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("private write left a temporary file")
	}
}
