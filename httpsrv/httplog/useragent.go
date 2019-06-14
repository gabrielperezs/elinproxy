package httplog

import (
	"sync"

	"github.com/avct/uasurfer"
)

var (
	uaPool = sync.Pool{
		New: func() interface{} {
			return &uasurfer.UserAgent{}
		},
	}
)

func getUserAgentID(s string) (id int) {
	ua := uaPool.Get().(*uasurfer.UserAgent)
	uasurfer.ParseUserAgent(s, ua)
	id = int(ua.DeviceType)
	ua.Reset()
	uaPool.Put(ua)
	return
}
