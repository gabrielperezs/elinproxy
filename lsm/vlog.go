package lsm

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	defaultMaxFileSize     = 1000 * 1024 * 1024
	defaultTransformBuffer = 100000
)

var (
	endRecordMark = []byte(">>>>>!ELINPROXY\n")
	newLine       = []byte("\r\n")

	ttlMap = []float64{
		float64(10 * time.Minute),
		float64(30 * time.Minute),
		float64(1 * time.Hour),
		float64(6 * time.Hour),
		float64(12 * time.Hour),
		float64(24 * time.Hour),
	}
)

var (
	ErrInvalidTTL = errors.New("The ttl is ouf of the possible ranges")
)

type VLog struct {
	cfg        *Config
	in         chan *ItemMem
	ttlCh      sync.Map
	inflight   *singleflight.Group
	afterWrite func(itd *ItemDisk, ttl time.Duration)
}

func VlogNew(cfg *Config) *VLog {
	vl := &VLog{
		cfg:      cfg,
		ttlCh:    sync.Map{},
		inflight: &singleflight.Group{},
	}
	return vl
}

func (vl *VLog) SetAfterWrite(fn func(itd *ItemDisk, ttl time.Duration)) {
	vl.afterWrite = fn
}

func (vl *VLog) Get(itd *ItemDisk) error {
	return nil
}

func (vl *VLog) Set(itm *ItemMem, ttl time.Duration) error {
	ttlInt := int(ttl.Seconds())
	if v, ok := vl.ttlCh.Load(ttlInt); ok {
		v.(chan *ItemMem) <- itm
		return nil
	}

	v, _, _ := vl.inflight.Do(ttl.String(), func() (interface{}, error) {
		w := make(chan *ItemMem, defaultTransformBuffer)
		vl.ttlCh.Store(ttlInt, w)
		go vl.reader(ttl, w)
		return w, nil
	})

	v.(chan *ItemMem) <- itm
	return nil
}

func (vl *VLog) reader(ttl time.Duration, in chan *ItemMem) {
	// Start first file
	w := VFileNew(ttl, vl.cfg)

	for itm := range in {
		if w.Expired() {
			go w.AutoExpire()
			w = VFileNew(ttl, vl.cfg)
		}

		for try := 0; try < 3; try++ {
			itd, err := vl.transform(w, itm)
			if err != nil {
				go w.AutoExpire()
				w = VFileNew(ttl, vl.cfg)
				continue
			}
			if vl.afterWrite != nil {
				vl.afterWrite(itd, ttl)
			}
			break
		}

		if w.Seek() > defaultMaxFileSize {
			go w.AutoExpire()
			w = VFileNew(ttl, vl.cfg)
		}
	}
}

func (vl *VLog) transform(w *VFile, itm *ItemMem) (*ItemDisk, error) {

	itd := getItemDisk()
	itd.Key = itm.Key
	itd.StatusCode = itm.StatusCode
	itd.Off = w.Seek()
	itd.HeadSize = 0
	itd.BodySize = 0
	itd.VFile = w
	itd.HIT = atomic.LoadUint64(&itm.HIT)

	if err := itm.Header.Write(w); err != nil {
		log.Printf("ERROR lsm/vlog write header: %v", err)
		return nil, err
	}
	w.Write(newLine)
	itd.HeadSize = w.Seek() - itd.Off

	nb, err := itm.WriteTo(w)
	if err != nil {
		log.Printf("ERROR lsm/vlog write body: %v", err)
		return nil, err
	}

	itd.BodySize = w.Seek() - itd.Off - itd.HeadSize
	if itd.BodySize != int64(nb) {
		log.Printf("ERROR lsm/vlog body size: %d/%d", itd.BodySize, nb)
	}
	w.Write(endRecordMark)
	return itd, nil
}
