package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/99designs/basicauth-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var metricsBuckets = []float64{300, 1200, 5000}

type prometheusMiddleware struct {
	reqs    *prometheus.CounterVec
	latency *prometheus.HistogramVec
}

// MetricsMiddleware is the request logger that provides metrics to prometheus
func metricsMiddleware(name string) func(http.Handler) http.Handler {
	m := prometheusMiddleware{
		reqs: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "chi_requests_total",
				Help:        "How many HTTP requests processed, partitioned by status code, method and HTTP path.",
				ConstLabels: prometheus.Labels{"service": name},
			},
			[]string{"code", "method", "path"},
		),
		latency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "chi_request_duration_milliseconds",
				Help:        "How long it took to process the request, partitioned by status code, method and HTTP path.",
				ConstLabels: prometheus.Labels{"service": name},
				Buckets:     metricsBuckets,
			},
			[]string{"code", "method", "path"},
		),
	}

	prometheus.MustRegister(m.reqs)
	prometheus.MustRegister(m.latency)

	return m.handler
}

func (m prometheusMiddleware) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		m.reqs.WithLabelValues(statusLabel(ww.Status()), r.Method, r.URL.Path).Inc()
		m.latency.WithLabelValues(statusLabel(ww.Status()), r.Method, r.URL.Path).
			Observe(float64(time.Since(start).Nanoseconds()) / 1000000)
	})
}

func statusLabel(status int) string {
	if status == 0 {
		status = http.StatusOK
	}

	if label := http.StatusText(status); label != "" {
		return label
	}

	return strconv.Itoa(status)
}

func metricsMount(r chi.Router, username, password string) {
	r.Route("/metrics", func(r chi.Router) {
		r.Use(basicauth.New("Metrics", map[string][]string{
			username: {password},
		}))
		r.Handle("/", promhttp.Handler())
	})
}
