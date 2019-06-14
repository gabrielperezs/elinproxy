package cacherules

import (
	"net/http"
	"strings"
	"time"
)

func (ir *InternalRules) IsRespCachable(resp *http.Response) (ttl time.Duration, ok bool) {
	ttl, ok, last := ir.getRespStatusCodeTTL(resp)
	if !ok || last {
		// If the response code is not cachable
		return
	}

	// Rules base in the response
	if ok = ir.isValidRespHeader(resp); !ok {
		return
	}

	// Rules base in the response
	if ttl, ok = ir.getRespContentTypeTTL(ttl, resp); ok {
		return
	}

	// By default, we cache the results
	ok = true
	return
}

// getRespStatusCodeTTL Will check it the response code is in the list of cacable codes
func (ir *InternalRules) getRespStatusCodeTTL(resp *http.Response) (time.Duration, bool, bool) {
	if len(ir.RespStatusCodeTTL) == 0 {
		return defaultTTL, true, false
	}

	if v, ok := ir.RespStatusCodeTTL[resp.StatusCode]; ok {
		if resp.StatusCode == http.StatusOK {
			// Response with the TTL, that TTL can be overwrite by the next rules
			return v, true, false
		}
		// This is the last rule that can be apply
		return v, true, true
	}
	// The response code is not in the list of the cachable content
	return defaultTTL, false, true
}

func (ir *InternalRules) isValidRespHeader(resp *http.Response) bool {
	if len(ir.RespHeadersBlackList) == 0 {
		return true
	}

	for k, v := range ir.RespHeadersBlackList {
		headValue := resp.Header.Get(k)
		if headValue == "" {
			continue
		}
		for _, s := range v {
			if strings.Contains(headValue, s) {
				return false
			}
		}
	}
	return true
}

func (ir *InternalRules) getRespContentTypeTTL(ttl time.Duration, resp *http.Response) (time.Duration, bool) {
	if resp.Header.Get("Content-Type") == "" {
		return ttl, false
	}

	if len(ir.RespContentTypeTTL) == 0 {
		return ttl, false
	}

	for k, v := range ir.RespContentTypeTTL {
		if strings.Contains(resp.Header.Get("Content-Type"), k) {
			return ttl + v, true
		}
	}
	return ttl, false
}
