package lsm

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cespare/xxhash"
)

var (
	v = VlogNew(&Config{
		Dir:       "/tmp/",
		MinLSMTTL: 0 * time.Second,
	})
)

func TestVLogBasic(t *testing.T) {
	v := VlogNew(&Config{
		Dir:       "/tmp/",
		MinLSMTTL: 0 * time.Second,
	})

	for i := 0; i <= 100; i++ {
		ttl := time.Duration((i%2)+1) * time.Minute
		v.Set(&ItemMem{
			Key:        xxhash.Sum64String(fmt.Sprintf("key%020d", i)),
			StatusCode: 200,
			Header: http.Header{
				"Content-Type": []string{"text/plain"},
				"X-Testing":    []string{"Testing header"},
			},
			Data: bytes.Repeat([]byte("b"), 1024),
		}, ttl)
	}
	time.Sleep(2 * time.Second)
}

// BenchmarkVlogWriteDisk-4   	 1000000	     25198 ns/op	    1719 B/op	      17 allocs/op
func BenchmarkVlogWriteDisk(b *testing.B) {
	ttl := time.Duration(2) * time.Second

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			v.Set(&ItemMem{
				Key:        xxhash.Sum64String(fmt.Sprintf("key%020d", i)),
				StatusCode: 200,
				Header: http.Header{
					"Content-Type": []string{"text/plain"},
					"X-Testing":    []string{"Testing header"},
				},
				Data: bytes.Repeat([]byte("b"), 1024),
			}, ttl)
			i++
		}
	})
}
