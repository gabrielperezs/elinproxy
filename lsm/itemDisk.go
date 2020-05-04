package lsm

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/textproto"
	"sync"
	"sync/atomic"
)

const (
	defaultBufferReadSize = 1024 * 32
)

var (
	// ErrReadingItemDisk is the error reponse if the range is not correct
	ErrReadingItemDisk = errors.New("The bytes read don't match with the body size definition")
	itemDiskPool       = sync.Pool{}
)

func getItemDisk() *ItemDisk {
	if v, ok := itemDiskPool.Get().(*ItemDisk); ok {
		return v
	}
	return &ItemDisk{}
}

func putItemDisk(v *ItemDisk) {
	*v = ItemDisk{}
	itemDiskPool.Put(v)
}

// ItemDisk is the struct that define the location of this item. Like the
// file where is stored and also the possition in the file for each element
type ItemDisk struct {
	Key        uint64
	StatusCode int
	VFile      *VFile
	Off        int64
	HeadSize   int64
	BodySize   int64
	HIT        uint64
	inUse      int64
}

// GetStatusCode will return the HTTP reponse code of this item
func (itd *ItemDisk) GetStatusCode() int {
	return itd.StatusCode
}

// Done is a way to mark the item as "unused" so the expiration
// process can remove from the tree and sent it back to the pool
func (itd *ItemDisk) Done() {
	atomic.AddInt64(&itd.inUse, -1)
}

// GetHeader will read the header stored in bytes in the disk
// parse with the standar lib and return the interface http.Header
func (itd *ItemDisk) GetHeader() http.Header {
	b := getHeaderInternalBytes(itd.HeadSize)
	defer putHeaderInternalBytes(b)

	n, err := itd.VFile.r.ReadAt(b.b, itd.Off)
	if itd.HeadSize != int64(n) || err != nil {
		// log.Printf("lsm/itemDisk/GetHeader error reading: %s (bytes expected %d, bytes readed %d, cap %d, len %d)",
		// 	err.Error(), itd.HeadSize, n, cap(b.b), len(b.b))
		return nil
	}

	tp := textproto.NewReader(b.buf)
	h, err := tp.ReadMIMEHeader()
	if err != nil {
		//log.Printf("lsm/itemDisk/GetHeader error parsing: %s", err.Error())
		return nil
	}
	return http.Header(h)
}

func (itd *ItemDisk) Write(b []byte) (int, error) {
	return 0, ErrItemDiskWrite
}

// WriteTo will write the content of the item that is located in the disk in the io.Writer provided
func (itd *ItemDisk) WriteTo(w io.Writer) (int64, error) {
	n, err := itd.writeTo(w, 0, 0)
	return int64(n), err
}

func (itd *ItemDisk) writeTo(w io.Writer, from, to int64) (int, error) {
	var last int64
	seek := itd.Off + itd.HeadSize
	left := itd.BodySize

	if from > 0 || to > 0 {
		seek += int64(from)
		left = int64(to - from)
	}

	b := get4K()
	defer put4K(b)
	for {
		if int(left) < cap(b) {
			b = b[:left]
		}

		n, err := itd.VFile.r.ReadAt(b, seek+last)
		if err != nil && err != io.EOF {
			return n, err
		}

		last += int64(n)
		left -= int64(n)
		w.Write(b[:n])

		//log.Printf("seek+last: %d - Seek: %d - Last: %d - Left: %d", seek+last, seek, last, left)

		if left == 0 {
			return int(last), nil
		}

		if last == itd.BodySize || ((from > 0 || to > 0) && last == int64(to)) {
			return int(last), nil
		}

		if last > itd.BodySize || ((from > 0 || to > 0) && last > int64(to)) {
			return int(last), ErrReadingItemDisk
		}
	}
}

func (itd *ItemDisk) ValidRange(reqStart, reqEnd int64) (from, to, length int64, err error) {
	if reqStart < 0 {
		err = ErrWrongRange
		return
	}

	itmLen := int64(itd.Len())
	if reqEnd >= itmLen {
		reqEnd = itmLen
	} else {
		reqEnd++
	}

	if reqStart == 0 && reqEnd == 0 {
		from = 0
		to = itmLen
		length = itmLen
		return
	}

	reqLength := reqEnd - reqStart
	if reqLength == 0 {
		err = ErrWrongRange
		return
	}

	from = reqStart
	to = reqEnd
	length = reqLength
	return
}

// WriteToRange going to return an slice of bytes of an specific range
func (itd *ItemDisk) WriteToRange(w io.Writer, from, to int64) (int64, error) {
	if from > to {
		return 0, ErrWrongRange
	}
	n, err := itd.writeTo(w, from, to)
	return int64(n), err
}

// Bytes will return the full content of the item in slice of bytes
// NOTE: Don't use for critical operations, it works without a sync.Pool,
// use WriteTo instead.
func (itd *ItemDisk) Bytes() []byte {
	buf := bytes.NewBuffer(make([]byte, 0, itd.Len()))
	_, err := itd.writeTo(buf, 0, 0)
	if err != nil {
		return nil
	}
	return buf.Bytes()
}

// GetHIT will return the total hits accumulated by this item
func (itd *ItemDisk) GetHIT() uint64 {
	return atomic.LoadUint64(&itd.HIT)
}

// Len will return the total size of the content of this key
func (itd *ItemDisk) Len() int {
	return int(itd.BodySize)
}
