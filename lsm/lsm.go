package lsm

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gabrielperezs/elinproxy/lsm/kvsm"
)

const (
	retryEvictionSize = 1024
)

var (
	ErrItemNotFound = errors.New("Item not found")
)

type EvictFunc func(key interface{}, value interface{})

type LSM struct {
	mu              sync.Mutex
	cfg             *Config
	vlog            *VLog
	mem             *kvsm.KVSM
	retryEvictionCh chan interface{}
}

func New(cfg *Config) *LSM {

	c := &LSM{
		cfg:             &Config{},
		retryEvictionCh: make(chan interface{}, retryEvictionSize),
	}
	*c.cfg = *cfg

	go c.retryEviction()

	RemoveContents(c.cfg.Dir)

	c.vlog = VlogNew(c.cfg)
	c.vlog.SetAfterWrite(c.afterWrite)

	c.mem = kvsm.New()
	c.mem.SetOnEvicted(c.onEvictInternal)

	return c
}

func (c *LSM) Reload(cfg *Config) {
	c.mu.Lock()
	*c.cfg = *cfg
	c.mu.Unlock()
}

func (c *LSM) afterWrite(itd *ItemDisk, ttl time.Duration) {
	c.mem.Swap(itd.Key, itd, ttl)
}

func (c *LSM) retryEviction() {
	for x := range c.retryEvictionCh {
		if n := c.onEvictInternal(x); n > 0 {
			// Stop the eviction if one of the elements is still in use
			time.Sleep(time.Duration(n) * time.Second)
		}
	}
}

func (c *LSM) onEvictInternal(x interface{}) (n int64) {
	switch it := x.(type) {
	case *ItemMem:
		n = atomic.LoadInt64(&it.inUse)
		if n == 0 {
			PutItem(it)
			return
		}
	case *ItemDisk:
		n = atomic.LoadInt64(&it.inUse)
		if n == 0 {
			it.VFile = nil
			return
		}
	default:
		log.Printf("lsm/onEvictInternal ERROR: Unknown object %+v", x)
		return
	}
	select {
	case c.retryEvictionCh <- x:
	default:
		log.Printf("lsm/onEvictInternal ERROR: retry eviction channel full %+v", x)
	}
	return
}

func (c *LSM) NewItem(l int) *ItemMem {
	return GetItemLen(l)
}

func (c *LSM) Set(key uint64, itm *ItemMem, ttl time.Duration) {
	c.mem.Set(itm.Key, itm, ttl)
	if ttl > c.cfg.MinLSMTTL {
		c.vlog.Set(itm, ttl)
	}
}

func (c *LSM) Get(key uint64) (Item, bool, error) {
	if x, expired, ok := c.mem.Get(key); ok {
		switch item := x.(type) {
		case *ItemMem:
			atomic.AddInt64(&item.inUse, 1)
			atomic.AddUint64(&item.HIT, 1)
			return item, expired, nil
		case *ItemDisk:
			atomic.AddInt64(&item.inUse, 1)
			atomic.AddUint64(&item.HIT, 1)
			return item, expired, nil
		}
	}
	return nil, false, ErrItemNotFound
}

func RemoveContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}
