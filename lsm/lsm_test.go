package lsm

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cespare/xxhash"
)

var (
	benckLSM = New(&Config{
		Dir:       os.TempDir(),
		MinLSMTTL: 0 * time.Second,
	})
)

func TestBasic1(t *testing.T) {
	ttl := 1 * time.Second
	l := New(&Config{
		Dir:       os.TempDir(),
		MinLSMTTL: 0 * time.Second,
	})

	key := fmt.Sprintf("key+%09d", 1)
	itm := &ItemMem{
		Key:        xxhash.Sum64String(key),
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/plain"},
			"X-Testing":    []string{"Testing header"},
		},
		Data: bytes.Repeat([]byte("b"), 1024),
	}
	l.Set(itm.Key, itm, ttl)
}

func TestMultipleReaders(t *testing.T) {
	ttl := 2 * time.Minute
	l := New(&Config{
		Dir:       os.TempDir(),
		MinLSMTTL: 0 * time.Second,
	})

	wg := &sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n := 0
			for {
				n++
				if n > 100 {
					return
				}

				key := fmt.Sprintf("key+%09d", i)
				itm := l.NewItem(0)
				itm.Key = xxhash.Sum64String(key)
				itm.StatusCode = 200
				itm.Header = http.Header{
					"Content-Type": []string{"text/plain"},
					"X-Testing":    []string{fmt.Sprintf("Testing header %d", i)},
					"X-Other":      []string{strings.Repeat(fmt.Sprintf("%d", i), 1024+i)},
				}
				itm.Data = append(itm.Data, bytes.Repeat([]byte("b"), (1024*1024*i)+33)...)
				l.Set(itm.Key, itm, ttl)
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	time.Sleep(1 * time.Second)

	for i := 200; i > 0; i-- {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			buf := bytes.NewBuffer(make([]byte, 0))

			key := fmt.Sprintf("key+%09d", i%10)
			item, _, err := l.Get(xxhash.Sum64String(key))
			if err != nil {
				t.Fatalf("Error key %s: %s - %v", key, err, item)
			}
			header := item.GetHeader()
			if len(header) != 3 {
				t.Fatalf("Invalid header: %s (%+v)", key, header)
			}

			s := strings.Repeat(fmt.Sprintf("%d", i%10), 1024+(i%10))
			if !strings.EqualFold(s, header.Get("X-Other")) {
				t.Fatalf("Invalid header: %s (%+v)", key, header)
			}

			comp := bytes.Repeat([]byte("b"), (1024*1024*(i%10))+33)
			if _, err := item.WriteTo(buf); err != nil {
				t.Fatalf("Error key %s: %s - %v", key, err, item)
			}
			if buf.Len() != len(comp) {
				t.Fatalf("Invalid key: %s original:%d now:%d", key, len(comp), buf.Len())
			}
			if !bytes.Equal(comp, buf.Bytes()) {
				t.Fatalf("Invalid key: %s original:%d now:%d", key, len(comp), buf.Len())
			}
		}(i)
	}
	wg.Wait()
}

// BenchmarkLsmWrite-4   	 1000000	      9447 ns/op	    1765 B/op	      15 allocs/op
func BenchmarkLsmWrite(b *testing.B) {

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ttl := time.Duration(i%2) * time.Second
			key := fmt.Sprintf("key+%09d", i)
			itm := &ItemMem{
				Key:        xxhash.Sum64String(key),
				StatusCode: 200,
				Header: http.Header{
					"Content-Type": []string{"text/plain"},
					"X-Testing":    []string{"Testing header"},
				},
				Data: bytes.Repeat([]byte("b"), 1024),
			}

			benckLSM.Set(itm.Key, itm, ttl)
			i++
		}
	})
}
