package lsm

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	fileNo     int64
	maxWaiting = time.Hour * 1
)

type VFile struct {
	mu         sync.RWMutex
	ttl        time.Duration
	expireAt   time.Time
	name       string
	off        int
	w          *os.File
	r          *os.File
	size       int64
	autoExpire uint64
}

func VFileNew(ttl time.Duration, cfg *Config) *VFile {
	vf := &VFile{
		expireAt: time.Now().Add(ttl).Add(5 * time.Second),
		ttl:      ttl,
		name:     fmt.Sprintf("%s/mem-%s-%09d.bin", cfg.Dir, ttl.String(), atomic.AddInt64(&fileNo, 1)),
	}

	var err error
	vf.w, err = os.OpenFile(vf.name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Panic(vf.name, err)
	}

	vf.r, err = os.Open(vf.name)
	if err != nil {
		log.Panic(vf.name, err)
	}

	runtime.SetFinalizer(vf, func(vf *VFile) error {
		return vf.Close()
	})

	return vf
}

func (vf *VFile) Expired() bool {
	return time.Now().After(vf.expireAt)
}

func (vf *VFile) ReadAt(b []byte, off int64) (int, error) {
	return vf.r.ReadAt(b, off)
}

func (vf *VFile) Write(b []byte) (int, error) {
	vf.expireAt = time.Now().Add(vf.ttl).Add(5 * time.Second)
	n, err := vf.w.Write(b)
	if err != nil {
		return n, err
	}

	atomic.AddInt64(&vf.size, int64(n))
	return n, err
}

// Seek return the last "byte" position in the file
func (vf *VFile) Seek() int64 {
	return atomic.LoadInt64(&vf.size)
}

// CloseWriter will close the "writer" file
func (vf *VFile) CloseWriter() error {
	vf.mu.RLock()
	f := vf.w
	vf.w = nil
	vf.mu.RUnlock()

	if f == nil {
		return nil
	}

	if err := f.Close(); err != nil {
		return err
	}
	log.Printf("D: lsm/vfile CloseWriter %s", vf.name)
	return nil
}

// CloseReader will close the "reader" file
func (vf *VFile) CloseReader() error {
	vf.mu.RLock()
	f := vf.r
	vf.r = nil
	vf.mu.RUnlock()

	if f == nil {
		return nil
	}

	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

// Close force the expiration of this file, will not wait for the TTL
func (vf *VFile) Close() error {
	runtime.SetFinalizer(vf, nil)
	vf.CloseWriter()
	vf.CloseReader()
	log.Printf("D: lsm/vfile Close %s", vf.name)
	return os.Remove(vf.name)
}

// AutoExpire will close all the open files related and define
// a time to close everything and delete the file
func (vf *VFile) AutoExpire() {
	// If this function was called before just exists
	// we don't want to execute the close more than one time
	if atomic.AddUint64(&vf.autoExpire, 1) > 1 {
		return
	}
	// Close the writer
	vf.CloseWriter()
	// Sleep until the oldest record expire
	go func() {
		max := time.Duration((vf.ttl.Seconds() * 0.25)) * time.Second
		if max > maxWaiting {
			max = maxWaiting
		}
		time.Sleep(vf.ttl + max)
		vf.Close()
	}()
}
