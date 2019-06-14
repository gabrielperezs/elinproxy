package httpsrv

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/mholt/certmagic"
	"github.com/valyala/fasthttp/reuseport"

	"github.com/gabrielperezs/elinproxy/certs"
	"github.com/gabrielperezs/elinproxy/httpsrv/handler"
)

type Config struct {
	Listen    []string
	ListenTLS []string
	Reuse     bool
	Certs     *certs.Config
	Handler   *handler.Config
	Debug     bool
}

type SRV struct {
	sync.Mutex
	cfg         *Config
	done        chan struct{}
	openSockets []net.Listener
	tlsCfg      *tls.Config
	http        *http.Server

	acme    *certmagic.Config
	certs   *certs.Certs
	handler *handler.Handler
}

func New(cfg *Config) *SRV {

	s := &SRV{
		done: make(chan struct{}, 1),
		cfg: &Config{
			Listen:    cfg.Listen,
			ListenTLS: cfg.ListenTLS,
			Reuse:     cfg.Reuse,
		},
	}

	s.handler = handler.New(cfg.Handler)
	s.certs = certs.New(cfg.Certs)

	s.tlsCfg = &tls.Config{
		GetCertificate:           s.certs.GetCertificate,
		InsecureSkipVerify:       false,
		MinVersion:               s.certs.TLSConfig().MinVersion,
		NextProtos:               s.certs.TLSConfig().NextProtos,
		PreferServerCipherSuites: s.certs.TLSConfig().PreferServerCipherSuites,
		CurvePreferences:         s.certs.TLSConfig().CurvePreferences,
		CipherSuites:             s.certs.TLSConfig().CipherSuites,
		ClientSessionCache:       s.certs.TLSConfig().ClientSessionCache,
	}

	s.http = &http.Server{
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ConnState: func(c net.Conn, cs http.ConnState) {
			switch cs {
			case http.StateIdle, http.StateNew:
				c.SetReadDeadline(time.Now().Add(connStateAddIdleNewTimeout))
			case http.StateActive:
				c.SetReadDeadline(time.Now().Add(connStateAddActiveTimeout))
			}
		},
		Handler: s.certs.HTTPChallengeHandler(s.handler),
	}

	return s
}

func (s *SRV) Reload(cfg *Config) {
	s.cfg.Reuse = cfg.Reuse
	s.cfg.Debug = cfg.Debug
	s.handler.Reload(cfg.Handler)
	s.certs.Reload(s.cfg.Certs)
}

func (s *SRV) runListen(p string, reuse bool) {
	var ln net.Listener
	var err error

	if reuse {
		ln, err = reuseport.Listen("tcp4", p)
	} else {
		ln, err = net.Listen("tcp4", p)
	}

	if err != nil {
		log.Printf("Error listen: %s - %s", p, err)
		return
	}
	s.Lock()
	s.openSockets = append(s.openSockets, ln)
	s.Unlock()
	s.http.Serve(ln)
}

func (s *SRV) runListenTLS(p string, reuse bool) {
	var ln net.Listener
	var err error

	if reuse {
		ln, err = reuseport.Listen("tcp4", p)
	} else {
		ln, err = net.Listen("tcp4", p)
	}

	if err != nil {
		log.Printf("Error listenTLS: %s - %s", p, err)
		return
	}
	tlsLn := tls.NewListener(ln, s.tlsCfg)
	s.Lock()
	s.openSockets = append(s.openSockets, tlsLn)
	s.Unlock()
	s.http.Serve(tlsLn)

}

func (s *SRV) Listen() {
	if s.cfg.Reuse {
		for i := 0; i <= runtime.NumCPU(); i++ {
			for _, p := range s.cfg.Listen {
				log.Printf("httpsrv listen: %s", p)
				go s.runListen(p, true)
			}

			for _, p := range s.cfg.ListenTLS {
				log.Printf("httpsrv listenTLS: %s", p)
				go s.runListenTLS(p, true)
			}
		}
	} else {
		for _, p := range s.cfg.Listen {
			log.Printf("httpsrv listen: %s", p)
			go s.runListen(p, false)
		}

		for _, p := range s.cfg.ListenTLS {
			log.Printf("httpsrv listenTLS: %s", p)
			go s.runListenTLS(p, false)
		}
	}

	<-s.done

	for _, l := range s.openSockets {
		l.Close()
	}
}
