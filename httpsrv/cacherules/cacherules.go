package cacherules

import (
	"time"
)

const (
	methodGET  = "GET"
	methodHEAD = "HEAD"
	defaultTTL = 1 * time.Hour
)

type InternalRules struct {
	// Request related
	NoReqExt            []string
	NoReqPathPrefix     []string
	NoReqPathSuffix     []string
	NoReqPathContains   []string
	NoReqCookieContains []string
	NoReqHeaders        map[string]string

	// Headers that condition the cache, blacklist
	RespHeadersBlackList map[string][]string

	RespContentTypeTTL       map[string]time.Duration
	RespContentTypeTTLString map[string]string
	RespStatusCodeTTL        map[int]time.Duration
	RespStatusCodeTTLString  map[string]string
}

type Rules struct {
	InternalRules
	Domain map[string]InternalRules
}

type Between []int

func (b Between) From() int {
	return b[0]
}
func (b Between) To() int {
	return b[1]
}
