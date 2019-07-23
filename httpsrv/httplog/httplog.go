package httplog

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	// register transports
	"nanomsg.org/go/mangos/v2/protocol/pub"
	_ "nanomsg.org/go/mangos/v2/transport/all"
)

var (
	listen      = "ipc:///tmp/elinproxy.sock"
	msgCh       = make(chan HTTPLog, 2)
	schemaHTTP  = "http"
	schemaHTTPS = "https"

	m = newMetrics()
)

func init() {
	os.Remove(strings.TrimPrefix(listen, "ipc://"))
	go server()
}

func server() {
	sock, err := pub.NewSocket()
	if err != nil {
		log.Printf("httpsrv/httplog ERROR new: %s", err)
		return
	}

	if err = sock.Listen(listen); err != nil {
		log.Printf("httpsrv/httplog ERROR Listen: %s", err)
		return
	}
	err = os.Chmod(strings.TrimPrefix(listen, "ipc://"), 0770)
	if err != nil {
		log.Printf("httpsrv/httplog Chmod Listen %s", err)
		return
	}
	log.Printf("httpsrv/httplog pubsub Listen %s", listen)

	for {
		err := sock.Send((<-msgCh).MarshalJSON())
		if err != nil {
			log.Printf("httpsrv/httplog ERROR Send: %s", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
	}
}

type HTTPLog struct {
	Time       time.Time
	ClientIP   string
	BackendIP  string
	Proto      string
	TLS        bool
	Method     string
	StatusCode int
	Schema     string
	Host       string
	User       string
	URL        string
	UserAgent  string
	Cookies    int
	RespTTFMS  float64
	RespTimeMS float64
	ReqBytes   int
	RespBytes  int
	HIT        bool
	RateLimit  bool
	Device     int
	CustomTags []string

	fnHeaders func(w http.ResponseWriter) []string
	w         http.ResponseWriter
}

func New(r *http.Request, w http.ResponseWriter, fnHeaders func(w http.ResponseWriter) []string) *HTTPLog {
	hl := &HTTPLog{
		Time: time.Now().UTC(),
		w:    w,
	}

	hl.ClientIP, _, _ = net.SplitHostPort(r.RemoteAddr)
	hl.Method = r.Method
	hl.ReqBytes = int(r.ContentLength)
	hl.Proto = r.Proto
	hl.Host = r.Host
	if localHost, _, err := net.SplitHostPort(r.Host); err == nil {
		hl.Host = localHost
	}
	if hl.Host == "" {
		hl.Host = r.Host
	}
	hl.UserAgent = r.UserAgent()
	hl.Device = getUserAgentID(hl.UserAgent)
	hl.Cookies = len(r.Cookies())
	if r.URL.User != nil {
		hl.User = r.URL.User.Username()
	}
	if r.TLS != nil {
		hl.TLS = true
	}

	if r.URL.Scheme == "" {
		if r.TLS == nil {
			hl.Schema = schemaHTTP
		} else {
			hl.Schema = schemaHTTPS
		}
	}

	hl.URL = hl.Schema + "://" + r.Host + r.URL.Path
	if r.URL.RawQuery != "" {
		hl.URL += "?" + r.URL.RawQuery
	}

	hl.fnHeaders = fnHeaders

	return hl
}

// GetKeyStr return a uniq string for each url
// IMPORTANT: This is the key of the cache engine, if this
// do not generate the correct string will fuck the cache
func (hl *HTTPLog) GetKeyStr() string {
	return strconv.Itoa(hl.Device) + hl.Method + hl.URL
}

func (hl *HTTPLog) end() {
	hl.RespTimeMS = float64(time.Now().Sub(hl.Time).Nanoseconds()) / 1000 / 1000
}

func (hl *HTTPLog) Done() {
	hl.end()
	m.save(hl)
	select {
	case msgCh <- *hl:
	default:
	}
}

func (hl HTTPLog) MarshalString() string {
	return fmt.Sprintf(
		"%s %s %s %v %v %v %s \"%s\" %d %.4f %.4f %d %d \"%s\" %s",
		hl.Time.Format(time.RFC3339),
		hl.ClientIP, hl.Proto, hl.TLS, hl.HIT, hl.RateLimit,
		hl.Method, hl.URL, hl.StatusCode,
		hl.RespTimeMS, hl.RespTTFMS,
		hl.ReqBytes, hl.RespBytes,
		hl.UserAgent,
		strings.Join(hl.CustomTags, " "),
	)
}

func (hl HTTPLog) MarshalJSON() []byte {
	b, _ := json.Marshal(hl)
	return b
}

func (hl *HTTPLog) Header() http.Header {
	return hl.w.Header()
}

func (hl *HTTPLog) Write(b []byte) (int, error) {
	n, err := hl.w.Write(b)
	if hl.RespBytes == 0 {
		hl.RespTTFMS = float64(time.Now().Sub(hl.Time).Nanoseconds()) / 1000 / 1000
	}
	hl.RespBytes += n
	return n, err
}

func (hl *HTTPLog) WriteHeader(statusCode int) {
	hl.StatusCode = statusCode
	hl.CustomTags = hl.fnHeaders(hl.w)
	hl.w.WriteHeader(statusCode)
}
