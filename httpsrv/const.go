package httpsrv

import "time"

const (
	connStateAddActiveTimeout  = time.Duration(90 * time.Second)
	connStateAddIdleNewTimeout = time.Duration(5 * time.Second)
	readTimeout                = time.Duration(90 * time.Second)
	writeTimeout               = time.Duration(90 * time.Second)
	idleTimeout                = time.Duration(90 * time.Second)
	readHeaderTimeout          = time.Duration(10 * time.Second)
)
