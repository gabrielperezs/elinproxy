package lsm

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync/atomic"
)

// ItemMem is the struct that store all the information related
// with the item. If the body is too big, then will be write in
// a temporary file that is also defined in the struct
type ItemMem struct {
	Key        uint64
	StatusCode int
	Header     http.Header
	Data       []byte
	HIT        uint64
	inUse      int64
	written    int64
	w          *os.File
}

// GetStatusCode will return the HTTP reponse code of this item
func (itm *ItemMem) GetStatusCode() int {
	return itm.StatusCode
}

// Done is a way to mark the item as "unused" so the expiration
// process can remove from the tree and sent it back to the pool
func (itm *ItemMem) Done() {
	atomic.AddInt64(&itm.inUse, -1)
}

// GetHeader will read the header stored in bytes in the disk
// parse with the standar lib and return the interface http.Header
func (itm *ItemMem) GetHeader() http.Header {
	return itm.Header
}

func (itm *ItemMem) Write(b []byte) (int, error) {
	if itm.w != nil {
		n, err := itm.w.Write(b)
		if err == nil {
			itm.written += int64(len(b))
		}
		return n, err
	}

	if len(b)+len(itm.Data) > itemMemLimitForTempFile {
		var err error
		itm.w, err = ioutil.TempFile(os.TempDir(), "elinproxy-")
		if err == nil {
			if _, err = itm.w.Write(itm.Data); err != nil {
				return 0, err
			}
			itm.Data = itm.Data[:0]
			return itm.w.Write(b)
		}
		log.Printf("lsm/itemMem tmp: %s", err)
	}

	itm.Data = append(itm.Data, b...)
	return len(b), nil
}

// WriteTo will write the bytes of the item in the provided io.Writer interface
func (itm *ItemMem) WriteTo(w io.Writer) (n int64, err error) {
	// If the item is not working with a temporary file
	// we just write the bytes (Data) in the io.Writer provided
	// in the arguments
	if itm.w == nil {
		var v int
		v, err = w.Write(itm.Data)
		n = int64(v)
		return
	}

	// If the item is storing in a temporary file, we are
	// going to read from the temporary file and write the
	// content in the io.Writer provided in the arguments
	var r *os.File
	r, err = os.Open(itm.w.Name())
	if err != nil {
		return
	}
	n, err = io.Copy(w, r)
	r.Close()
	return
}

func (itm *ItemMem) ValidRange(reqStart, reqEnd int64) (from, to, length int64, err error) {
	if reqStart < 0 {
		err = ErrWrongRange
		return
	}

	itmLen := int64(itm.Len())
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
func (itm *ItemMem) WriteToRange(w io.Writer, from, to int64) (int64, error) {
	if from < 0 || to < 0 || from >= to || from > int64(itm.Len()) {
		return 0, ErrWrongRange
	}

	if itm.w == nil {
		n, err := w.Write(itm.Data[from:to])
		return int64(n), err
	}

	r, err := os.Open(itm.w.Name())
	if err != nil {
		return 0, err
	}
	defer r.Close()

	seek := from
	left := to - from
	written := 0
	b := get4K()
	defer put4K(b)
	for {
		n, err := r.ReadAt(b, int64(seek))
		if err != nil && err != io.EOF {
			return 0, err
		}

		seek += int64(n)
		left -= int64(n)
		if err == io.EOF || left <= 0 {
			if left < 0 {
				n += int(left)
			}
			n, err := w.Write(b[:n])
			written += n
			return int64(written), err
		}

		n, err = w.Write(b[:n])
		written += n
		if err != nil {
			return int64(written), err
		}
	}
}

// Bytes will return the full content of the item in slice of bytes
func (itm *ItemMem) Bytes() []byte {
	if itm.w == nil {
		return itm.Data
	}
	r, err := os.Open(itm.w.Name())
	if err != nil {
		return nil
	}
	defer r.Close()

	b, _ := ioutil.ReadAll(r)
	return b
}

// GetHIT will return the total hits accumulated by this item
func (itm *ItemMem) GetHIT() uint64 {
	return atomic.LoadUint64(&itm.HIT)
}

// Len will return the total size of the content of this key
func (itm *ItemMem) Len() int {
	if itm.w == nil {
		return len(itm.Data)
	}

	s, err := itm.w.Stat()
	if err != nil {
		return 0
	}
	return int(s.Size())
}

// Close will close and remove the underline file if is defined
func (itm *ItemMem) Close() {
	if itm.w == nil {
		return
	}
	itm.w.Close()
	os.Remove(itm.w.Name())
	itm.w = nil
}
