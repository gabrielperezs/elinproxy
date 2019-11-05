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
	labels = []string{"ratelimit", "hit", "code", "method", "schema"}
)

func newMetrics() *metrics {
	m := &metrics{
		responseTTFB: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "elinproxy",
			Subsystem: "handler",
			Name:      metricResponseTTFB,
			Help:      "Reponse TTFB",
		}, labels),
		responseTime: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "elinproxy",
			Subsystem: "handler",
			Name:      metricResponseTime,
			Help:      "Reponse time",
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
	m.responseTTFB.With(label).Observe(l.RespTTFMS)
	m.responseTime.With(label).Observe(l.RespTimeMS)
	m.responseSize.With(label).Add(float64(l.RespBytes))
}
