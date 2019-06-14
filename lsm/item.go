package lsm

import (
	"errors"
	"io"
	"net/http"
	"sync"
)

var (
	ErrWrongRange    = errors.New("Data is smaller that the range requested")
	ErrItemDiskWrite = errors.New("The disk items are inmutable")
)

type Item interface {
	Write(b []byte) (int, error)
	WriteTo(io.Writer) (int64, error)
	WriteToRange(io.Writer, int64, int64) (int64, error)
	ValidRange(reqStart, reqEnd int64) (from, to, length int64, err error)
	Bytes() []byte
	GetHIT() uint64
	Len() int
	GetStatusCode() int
	GetHeader() http.Header
	Done()
}

var (
	itemMemMinSize          = 1 * 1024        // 1KB
	itemMemMaxSize          = 5 * 1024 * 1024 // 5MB
	itemMemLimitForTempFile = 4 * 1024 * 1024 // 4MB
	itemMemPool             = make([]*sync.Pool, 0)
)

func init() {
	for i := 1; i <= itemMemMaxSize/itemMemMinSize; i++ {
		itemMemPool = append(itemMemPool, &sync.Pool{})
	}
}

func calcIndex(l int) int {
	return (l / itemMemMinSize) % len(itemMemPool)
}

func GetItemLen(l int) *ItemMem {
	if l > itemMemMaxSize {
		l = itemMemMaxSize
	}

	ni := itemMemPool[calcIndex(l)].Get()
	itm, ok := ni.(*ItemMem)
	if !ok {
		return &ItemMem{
			Header: make(map[string][]string, 0),
			Key:    0,
			Data:   make([]byte, 0, l),
		}
	}
	return itm
}

func PutItem(itm *ItemMem) {
	l := cap(itm.Data)
	for k := range itm.Header {
		delete(itm.Header, k)
	}
	itm.Key = 0
	itm.Data = itm.Data[:0]
	itm.HIT = 0
	itm.StatusCode = 0
	itm.Close()

	switch {
	case l <= itemMemMinSize:
		// Return to the pool, objects that are smaller than the min size
		itemMemPool[0].Put(itm)
	case l <= itemMemMaxSize:
		// Return to the pull objects calculating the index
		itemMemPool[calcIndex(l)].Put(itm)
	default:
		// Will NOT send back to the pool
	}

	return
}
