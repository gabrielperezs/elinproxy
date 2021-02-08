package httplog

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	metricResponseTTFB = "response_ttfb"
	metricResponseTime = "response_time"
	metricResponseCode = "response_code"
	metricResponseSize = "response_size"
)

type metrics struct {
	responseTTFB *prometheus.HistogramVec
	responseTime *prometheus.HistogramVec
	responseCode *prometheus.CounterVec
	responseSize *prometheus.CounterVec
}

var (
	labels     = []string{"ratelimit", "hit", "code", "method", "schema"}
	defBuckets = []float64{0.005, 0.05, 0.1, 0.5, 1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000, 300000, 600000, 1800000, 3600000}
)

func newMetrics() *metrics {
	m := &metrics{
		responseTTFB: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "elinproxy",
			Subsystem: "handler",
			Name:      metricResponseTTFB,
			Help:      "Reponse TTFB",
			Buckets:   defBuckets,
		}, labels),
		responseTime: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "elinproxy",
			Subsystem: "handler",
			Name:      metricResponseTime,
			Help:      "Reponse time",
			Buckets:   defBuckets,
		}, labels),
		responseCode: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "elinproxy",
			Subsystem: "handler",
			Name:      metricResponseCode,
			Help:      "Reponse code",
		}, labels),
		responseSize: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "elinproxy",
			Subsystem: "handler",
			Name:      metricResponseSize,
			Help:      "Reponse size",
		}, labels),
	}

	return m
}
func (m *metrics) save(l *HTTPLog) {
	label := prometheus.Labels{
		"ratelimit": strconv.FormatBool(l.RateLimit),
		"hit":       strconv.FormatBool(l.HIT),
		"code":      strconv.Itoa(l.StatusCode),
		"method":    l.Method,
		"schema":    l.Schema,
	}
	m.responseCode.With(label).Inc()
	m.responseSize.With(label).Add(float64(l.RespBytes))
	m.responseTTFB.With(label).Observe(l.RespTTFMS)
	m.responseTime.With(label).Observe(l.RespTimeMS)
}
