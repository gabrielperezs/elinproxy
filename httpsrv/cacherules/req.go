package cacherules

import (
	"net/http"
	"strings"
)

func (ir *InternalRules) IsReqCachable(req *http.Request) (ok bool, refresh bool) {
	ok = true

	if req.Method != methodGET && req.Method != methodHEAD {
		ok = false
		return
	}

	if ok = ir.isReqCachableExt(req); !ok {
		return
	}

	if ok = ir.isReqCachablePathContent(req); !ok {
		return
	}

	if ok = ir.isReqCachablePathPrefix(req); !ok {
		return
	}

	if ok = ir.isReqCachablePathSuffix(req); !ok {
		return
	}

	if ok = ir.isReqCachableCookieContains(req); !ok {
		// This flag will remove the current request from the cache
		refresh = true
		return
	}

	return
}

func (ir *InternalRules) isReqCachableExt(req *http.Request) bool {
	for _, v := range ir.NoReqExt {
		if strings.HasSuffix(req.URL.Path, v) {
			return false
		}
	}
	return true
}

func (ir *InternalRules) isReqCachablePathContent(req *http.Request) bool {
	for _, v := range ir.NoReqPathContains {
		if strings.Contains(req.URL.Path, v) {
			return false
		}
	}
	return true
}

func (ir *InternalRules) isReqCachablePathPrefix(req *http.Request) bool {
	for _, v := range ir.NoReqPathPrefix {
		if strings.HasPrefix(req.URL.Path, v) {
			return false
		}
	}
	return true
}

func (ir *InternalRules) isReqCachablePathSuffix(req *http.Request) bool {
	for _, v := range ir.NoReqPathSuffix {
		if strings.HasSuffix(req.URL.Path, v) {
			return false
		}
	}
	return true
}

func (ir *InternalRules) isReqCachableCookieContains(req *http.Request) bool {
	for _, v := range ir.NoReqCookieContains {
		for _, c := range req.Cookies() {
			if strings.Contains(c.Name, v) {
				return false
			}
		}
	}
	return true
}
