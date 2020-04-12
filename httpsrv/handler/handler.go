package handler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/avct/uasurfer"

	"golang.org/x/sync/singleflight"

	"github.com/cespare/xxhash"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gabrielperezs/elinproxy/lsm"

	"github.com/gabrielperezs/elinproxy/httpsrv/cacherules"
	"github.com/gabrielperezs/elinproxy/httpsrv/httplog"
)

const (
	limitRange                   = 1024 * 1024
	defaultTLSHandshakeTimeout   = time.Second * 10
	defaultResponseHeaderTimeout = time.Second * 60
	defaultExpectContinueTimeout = time.Second * 1
	defaultIdleConnTimeout       = time.Second * 30
	defaultRateLimit             = 8
)

var (
	errNotFoundInCache = errors.New("The requests wasn't save in the cache")
	privateHeaders     = []string{
		"Set-Cache",
		"Proxy-Authenticate",
		"WWW-Authenticate",
	}
	uaDeviceCleaner = []uasurfer.DeviceType{
		uasurfer.DeviceUnknown,
		uasurfer.DeviceComputer,
		uasurfer.DeviceTablet,
		uasurfer.DevicePhone,
		uasurfer.DeviceConsole,
	}
)

type Config struct {
	BackendHost    string
	BackendPort    string
	BackendTLSHost string
	BackendTLSPort string
	BackendOnce    bool
	DomainSuffix   string

	RateLimit int

	ReqRemoveHeaders  []string
	RespRemoveHeaders []string

	CustomTags []string

	CacheRules *cacherules.Rules

	Cache *lsm.Config

	Debug bool
}

type Handler struct {
	mu           sync.Mutex
	rules        *cacherules.Rules
	cfg          *Config
	roundTripper *http.Transport
	bytesPool    bytesPool
	limiter      *limiter.Limiter
	cache        *lsm.LSM
	infligth     *singleflight.Group
}

func New(cfg *Config) *Handler {
	handler := &Handler{
		cfg: cfg,
		roundTripper: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       defaultIdleConnTimeout,
			TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
			ExpectContinueTimeout: defaultExpectContinueTimeout,
			ResponseHeaderTimeout: defaultResponseHeaderTimeout,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		rules:     &cacherules.Rules{},
		bytesPool: bytesPool{},
		infligth:  &singleflight.Group{},
	}

	// TODO: Handle this in the reload
	if handler.cfg.RateLimit == 0 {
		handler.cfg.RateLimit = defaultRateLimit
	}
	handler.limiter = tollbooth.NewLimiter(float64(handler.cfg.RateLimit), &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Hour,
	})
	handler.limiter.SetIPLookups([]string{"RemoteAddr"})

	*handler.rules = *cfg.CacheRules

	handler.cache = lsm.New(cfg.Cache)

	return handler
}

func (handler *Handler) Reload(cfg *Config) {
	handler.mu.Lock()
	*handler.cfg = *cfg
	handler.mu.Unlock()

	handler.cache.Reload(handler.cfg.Cache)
}

func (handler *Handler) customTags(w http.ResponseWriter) []string {
	customTags := make([]string, len(handler.cfg.CustomTags))
	for i, v := range handler.cfg.CustomTags {
		customTags[i] = w.Header().Get(v)
	}

	for _, v := range handler.cfg.RespRemoveHeaders {
		w.Header().Del(v)
	}

	return customTags
}

func (handler *Handler) ServeHTTP(orgW http.ResponseWriter, r *http.Request) {
	hlog := httplog.New(r, orgW, handler.customTags)
	defer hlog.Done()

	keyStr := hlog.GetKeyStr()
	key := xxhash.Sum64String(keyStr)
	isCachable, isRefreshable := handler.rules.IsReqCachable(r)

	if isCachable && handler.rules.Domain != nil {
		// Use the rules for domains/hosts
		if hostRule, ok := handler.rules.Domain[r.Host]; ok {
			isCachable, isRefreshable = hostRule.IsReqCachable(r)
		}
	}

	if !isCachable {
		if isRefreshable {
			for i := range uaDeviceCleaner {
				handler.cache.Delete(xxhash.Sum64String(hlog.GetKeyStrDevice(i)))
			}
		}

		if err := handler.reverseProxy(isCachable, key, r, hlog); err != nil {
			hlog.RateLimit = true
		}
		return
	}

	// Find in the cache an wirte the respond
	if handler.respondFromCache(key, hlog, r) {
		hlog.HIT = true
		return
	}

	// Go to the backend if the BackendOnce is false.
	if !handler.cfg.BackendOnce {
		if err := handler.reverseProxy(isCachable, key, r, hlog); err != nil {
			hlog.RateLimit = true
		}
		return
	}

	// BackendOnce will protect the backend if the server recive
	// many times the same request. Just one will go to the backend,
	// the others will wait.
	var firstCall uint32
	result := handler.infligth.DoChan(keyStr, func() (interface{}, error) {
		atomic.AddUint32(&firstCall, 1)
		if err := handler.reverseProxy(isCachable, key, r, hlog); err != nil {
			hlog.RateLimit = true
			return nil, err
		}
		return nil, nil
	})

	// Why NewTimer and not time.After:
	// The underlying Timer is not recovered by the garbage collector
	// until the timer fires. If efficiency is a concern, use NewTimer
	// instead and call Timer.Stop if the timer is no longer needed.
	timeoutInFligth := time.NewTimer(60 * time.Second)
	select {
	case <-result:
		// Prevent channel leak
		if !timeoutInFligth.Stop() {
			go func() {
				<-timeoutInFligth.C
				timeoutInFligth.Stop()
			}()
		}
		// We did the response in the infligth, nothing else to do
		if atomic.LoadUint32(&firstCall) > 0 {
			return
		}
		// We try to response from the cache
		if handler.respondFromCache(key, hlog, r) {
			hlog.HIT = true
			return
		}
		// the response from the infligth wasn't not store in the cache
		// could be that is not cachable response so we send the current
		// request to the backend
		if err := handler.reverseProxy(isCachable, key, r, hlog); err != nil {
			hlog.RateLimit = true
			return
		}
	case <-timeoutInFligth.C:
		// The following block to the requests that was waiting for the backend
		if atomic.LoadUint32(&firstCall) == 0 {
			log.Printf("httpsrv/handler/ServeHTTP doOnce StatusBadGateway timeout: %s://%s%s", r.URL.Scheme, r.Host, r.URL.Path)
			handler.badGateway(http.StatusBadGateway, "RFC 7231, 6.6.3", r, orgW)
			return
		}

		// The following block to the requests that was waiting for the backend
		// To don't block the url during long periods
		handler.infligth.Forget(keyStr)
		_, cancel := context.WithCancel(r.Context())
		cancel()

		select {
		case <-result:
			log.Printf("httpsrv/handler/ServeHTTP doOnce main request after timeout: %s://%s%s", r.URL.Scheme, r.Host, r.URL.Path)
		case <-time.After(60 * time.Second):
			log.Printf("httpsrv/handler/ServeHTTP doOnce main request timeout: %s://%s%s", r.URL.Scheme, r.Host, r.URL.Path)
		}
		return
	}
}

func (handler *Handler) badGateway(code int, msg string, r *http.Request, w http.ResponseWriter) {
	// Prevent CLOSE_WAIT leak, origin close connection
	cancel := handler.closeNotify(w, r)
	defer cancel()
	w.WriteHeader(code)
	w.Write([]byte(msg))
}

func (handler *Handler) reverseProxy(isCachable bool, key uint64, r *http.Request, w http.ResponseWriter) error {
	// Rate limit control to protect the backend
	err := tollbooth.LimitByRequest(handler.limiter, w, r)
	if err != nil {
		handler.badGateway(err.StatusCode, err.Message, r, w)
		return err
	}

	// Do the request in the backend
	outReq := new(http.Request)
	*outReq = *r // includes shallow copies of maps, but we handle this in Director

	revproxy := httputil.ReverseProxy{
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			req.Close = true

			if handler.cfg.Debug {
				if req == nil {
					log.Printf("httpsrv/handler/reverseProxy error without req: %s", err)
				} else if req.URL == nil {
					log.Printf("httpsrv/handler/reverseProxy error without req.URL: %+v - %s", req, err)
				} else {
					log.Printf("httpsrv/handler/reverseProxy error: %s://%s%s: %s", req.URL.Scheme, req.Host, req.URL.Path, err)
				}
			}

			if req != nil && req.Body != nil {
				n, err := io.Copy(ioutil.Discard, req.Body)
				err = req.Body.Close()
				if handler.cfg.Debug {
					log.Printf("httpsrv/handler/reverseProxy req.Body: - %v (%d) - OK", err, n)
				}
			}

			if w != nil {
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte("Backend error response"))
			} else {
				if handler.cfg.Debug {
					log.Printf("httpsrv/handler/reverseProxy error without writer: %+v - %s", req, err)
				}
			}
		},
		Director: func(req *http.Request) {
			handler.buildBackendURL(req)
			if !isCachable {
				return
			}
			handler.modifyRequest(key, req, r)
		},
		ModifyResponse: func(resp *http.Response) error {
			if handler.cfg.Cache == nil || !isCachable {
				return nil
			}
			ttl, ok := handler.rules.IsRespCachable(resp)
			if !ok {
				return nil
			}
			return handler.modifyResponse(key, resp, ttl)
		},
		Transport:  handler.roundTripper,
		BufferPool: handler.bytesPool,
	}
	revproxy.ServeHTTP(w, outReq)
	return nil
}

func (handler *Handler) modifyRequest(key uint64, req, origReq *http.Request) {
	for _, v := range handler.cfg.ReqRemoveHeaders {
		req.Header.Del(v)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Del("Range")
}

func (handler *Handler) modifyResponse(key uint64, resp *http.Response, ttl time.Duration) error {
	for _, v := range privateHeaders {
		resp.Header.Del(v)
	}

	for _, v := range handler.cfg.RespRemoveHeaders {
		resp.Header.Del(v)
	}

	// Close connection if is a redirect only to requests HTTP/1 or minor
	if resp.Request.ProtoMajor <= 1 {
		switch resp.StatusCode {
		case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
			resp.Header.Set("Connection", "close")
			resp.Close = true
		}
	}

	resp.Header.Set("Last-Modified", time.Now().Format(time.RFC1123Z))

	lengthStr := resp.Header.Get("Content-Length")
	length := 0
	if lengthStr != "" {
		length, _ = strconv.Atoi(lengthStr)
	}

	item := handler.cache.NewItem(length)
	item.Key = key
	item.StatusCode = resp.StatusCode
	for k, v := range resp.Header {
		item.Header[k] = append(item.Header[k], v...)
	}

	if err := DumpResponse(resp, true, item); err != nil {
		if handler.cfg.Debug {
			log.Printf("httpsrv/handler/modifyResponse DumpResponse: %s - %s", resp.Request.RequestURI, err)
		}
		return err
	}

	handler.cache.Set(key, item, ttl)
	return nil
}

func (handler *Handler) buildBackendURL(req *http.Request) {
	var uri string
	if req.TLS != nil && handler.cfg.BackendTLSHost != "" {
		uri = fmt.Sprintf("https://%s:%s%s", handler.cfg.BackendTLSHost, handler.cfg.BackendTLSPort, req.URL.Path)
	} else {
		uri = fmt.Sprintf("http://%s:%s%s", handler.cfg.BackendHost, handler.cfg.BackendPort, req.URL.Path)
	}

	if req.URL.RawQuery != "" {
		uri += "?" + req.URL.RawQuery
	}

	req.URL, _ = url.ParseRequestURI(uri)
	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
	}

	if strings.HasSuffix(host, handler.cfg.DomainSuffix) {
		req.Host = host[:len(host)-len(handler.cfg.DomainSuffix)] + ":" + handler.cfg.BackendPort
	}
}

func (handler *Handler) respondFromCache(key uint64, w http.ResponseWriter, r *http.Request) bool {
	item, ok, err := handler.cache.Get(key)
	if err != nil {
		return false
	}
	defer item.Done()

	// Control if is possible to retrive the header of the item
	headers := item.GetHeader()
	if headers == nil {
		return false
	}

	// Prevent CLOSE_WAIT leak, origin close connection
	cancel := handler.closeNotify(w, r)
	defer cancel()

	if !ok {
		w.Header().Set("X-Expired", fmt.Sprintf("%v", ok))
	}

	for k, v := range headers {
		w.Header().Set(k, strings.Join(v, ", "))
	}
	w.Header().Set("Accept-Ranges", "none")
	w.Header().Set("Server", "elinproxy")
	w.Header().Set("Age", strconv.FormatUint(item.GetHIT(), 10))

	var httpRange []httpRange
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		var err error
		httpRange, err = parseRange(rangeHeader, limitRange, int64(item.Len()))
		if err != nil {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			w.Write([]byte("ERROR 416 // RFC 7233, 4.4"))
			w.Write([]byte("\n"))
			w.Write([]byte(err.Error()))
			return true
		}
	}

	if item.GetStatusCode() != http.StatusOK || httpRange == nil {
		w.Header().Set("Content-Length", strconv.Itoa(item.Len()))
		w.WriteHeader(item.GetStatusCode())
		item.WriteTo(w)
		return true
	}

	rangeFrom, rangeTo, rangeLength, err := item.ValidRange(httpRange[0].start, httpRange[0].length)
	if err != nil {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		w.Write([]byte("ERROR 416 // RFC 7233, 4.4"))
		w.Write([]byte("\n"))
		w.Write([]byte(err.Error()))
		return true
	}

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeFrom, rangeTo-1, item.Len()))
	w.Header().Set("Content-Length", strconv.Itoa(int(rangeLength)))
	w.WriteHeader(http.StatusPartialContent)
	item.WriteToRange(w, rangeFrom, rangeTo)
	return true
}

// Prevent leak of CLOSE_WAIT
func (handler *Handler) closeNotify(w http.ResponseWriter, r *http.Request) context.CancelFunc {
	ctx, cancel := context.WithCancel(r.Context())
	if cn, ok := w.(http.CloseNotifier); ok {
		notifyChan := cn.CloseNotify()
		go func() {
			select {
			case <-notifyChan:
				cancel()
			case <-ctx.Done():
			}
		}()
	}

	if _, err := io.Copy(ioutil.Discard, r.Body); err != nil {
		r.Close = true
		if handler.cfg.Debug {
			log.Printf("handler/closeNotify WARNING reading body: %s", err)
		}
	}
	r.Body.Close()

	return cancel
}
