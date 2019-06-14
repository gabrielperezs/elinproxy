package lsm

import (
	"bufio"
	"sync"
)

var itemDiskBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, defaultBufferReadSize)
	},
}

func get4K() []byte {
	in := itemDiskBufferPool.Get()
	if b, ok := in.([]byte); ok {
		return b
	}
	return make([]byte, defaultBufferReadSize)
}

func put4K(b []byte) {
	if cap(b) != defaultBufferReadSize {
		return
	}
	itemDiskBufferPool.Put(b[:cap(b)])
}

var internalBytesHeaderPool = sync.Pool{}

type internalBytes struct {
	size int64
	b    []byte
	buf  *bufio.Reader
}

func (i *internalBytes) Reset(l int64) {
	i.size = l
	if l <= int64(cap(i.b)) {
		i.b = i.b[:l]
		return
	}
	i.b = make([]byte, l)
	i.buf.Reset(i)
}

func (i *internalBytes) Read(p []byte) (n int, err error) {
	copy(p, i.b[:i.size])
	return int(i.size), nil
}

func newInternalBytes(l int64) *internalBytes {
	i := &internalBytes{
		size: l,
		b:    make([]byte, l),
	}
	i.buf = bufio.NewReader(i)
	return i
}

func getHeaderInternalBytes(l int64) *internalBytes {
	in := internalBytesHeaderPool.Get()
	if b, ok := in.(*internalBytes); ok {
		b.Reset(l)
		return b
	}
	return newInternalBytes(l)
}

func putHeaderInternalBytes(b *internalBytes) {
	internalBytesHeaderPool.Put(b)
}
