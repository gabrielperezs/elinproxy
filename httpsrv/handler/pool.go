package handler

import "sync"

const (
	defaultMinPoolByteSize = 128
	defaultMaxPoolByteSize = 1024 * 1024
)

var (
	basicBytesPool = sync.Pool{}
)

type bytesPool struct {
}

func (v bytesPool) Get() []byte {
	x := basicBytesPool.Get()
	if b, ok := x.([]byte); ok {
		return b
	}
	return make([]byte, 0, defaultMinPoolByteSize)
}

func (v bytesPool) Put(b []byte) {
	if cap(b) >= defaultMaxPoolByteSize {
		return
	}
	b = b[:0]
	basicBytesPool.Put(b)
}
