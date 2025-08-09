package server

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusCollector holds the metrics and the handler path.
type PrometheusCollector struct {
	reqCount    *prometheus.CounterVec
	reqDurHist  *prometheus.HistogramVec
	inFlight    prometheus.Gauge
	registry    *prometheus.Registry
	MetricsPath string
}

// NewPrometheusCollector creates and registers standard HTTP metrics.
func NewPrometheusCollector(metricsPath string) *PrometheusCollector {
	reg := prometheus.NewRegistry()

	reqCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	reqDurHist := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Histogram of request durations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	inFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_in_flight_requests",
		Help: "Current number of in-flight requests",
	})

	reg.MustRegister(reqCount, reqDurHist, inFlight)

	return &PrometheusCollector{
		reqCount:    reqCount,
		reqDurHist:  reqDurHist,
		inFlight:    inFlight,
		registry:    reg,
		MetricsPath: metricsPath,
	}
}

// PrometheusMiddleware returns a gin middleware that collects metrics.
func (pc *PrometheusCollector) PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		pc.inFlight.Inc()
		c.Next()
		pc.inFlight.Dec()

		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		pc.reqCount.WithLabelValues(method, path, status).Inc()
		pc.reqDurHist.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	}
}

// RegisterMetricsEndpoint registers /metrics (or custom path) on Gin engine.
func (pc *PrometheusCollector) RegisterMetricsEndpoint(engine *gin.Engine) {
	if pc.MetricsPath == "" {
		pc.MetricsPath = "/metrics"
	}
	// Use promhttp handler
	engine.GET(pc.MetricsPath, gin.WrapH(promhttp.HandlerFor(pc.registry, promhttp.HandlerOpts{})))
}
