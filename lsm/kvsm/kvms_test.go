package kvsm

import (
	"fmt"
	"testing"
	"time"

	"github.com/cespare/xxhash"
)

var (
	kv = New()
)

func TestKVSMWriteAndExpire(t *testing.T) {
	kv := New()
	for i := 0; i < 10; i++ {
		kv.Set(xxhash.Sum64String(fmt.Sprintf("key%d", i)), fmt.Sprintf("value%d", i), 1*time.Second)
		time.Sleep(2 * time.Millisecond)
		i++
		kv.Set(xxhash.Sum64String(fmt.Sprintf("key%d", i)), fmt.Sprintf("value%d", i), 1*time.Second)
		time.Sleep(3 * time.Microsecond)
	}
	for {
		if kv.Len() == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func BenchmarkKVSMWrite(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var i = uint64(0)
		for pb.Next() {
			kv.Set(i, i, 10*time.Second)
			i++
		}
	})
}

func BenchmarkKVSMRead(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < 10; i++ {
		kv.Set(uint64(i), i, 10*time.Second)
	}

	b.RunParallel(func(pb *testing.PB) {
		var i = uint64(0)
		for pb.Next() {
			kv.Get(uint64(i) % 10)
			i++
		}
	})
}

func BenchmarkKVSMSwap(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < 10; i++ {
		kv.Set(uint64(i), i, 10*time.Second)
	}

	b.RunParallel(func(pb *testing.PB) {
		var i = uint64(0)
		for pb.Next() {
			kv.Swap(uint64(i)%10, i, 5*time.Second)
			i++
		}
	})
}

// func BenchmarkKVSMDelete(b *testing.B) {
// 	b.ResetTimer()
// 	b.ReportAllocs()

// 	for i := 0; i < 10; i++ {
// 		kv.Set(uint64(i), i, 10*time.Second)
// 	}

// 	b.RunParallel(func(pb *testing.PB) {
// 		var i = uint64(0)
// 		for pb.Next() {
// 			kv.Remove(uint64(i) % 10)
// 			i++
// 		}
// 	})
// }
