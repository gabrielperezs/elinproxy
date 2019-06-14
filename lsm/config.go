package lsm

import "time"

type Config struct {
	MinLSMTTLString string
	MinLSMTTL       time.Duration
	Dir             string
	ExtraTTLString  string
	ExtraTTL        time.Duration
}
