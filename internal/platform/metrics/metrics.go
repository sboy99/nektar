package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry wraps Prometheus metrics for the application.
type Registry struct {
	EventsPublished *prometheus.CounterVec
	EventsHandled   *prometheus.CounterVec
	HandlerErrors   *prometheus.CounterVec
}

// NewRegistry creates and registers application metrics.
func NewRegistry() *Registry {
	r := &Registry{
		EventsPublished: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nektar_events_published_total",
				Help: "Total number of events published",
			},
			[]string{"topic"},
		),
		EventsHandled: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nektar_events_handled_total",
				Help: "Total number of events handled",
			},
			[]string{"topic"},
		),
		HandlerErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nektar_handler_errors_total",
				Help: "Total number of handler errors",
			},
			[]string{"topic"},
		),
	}

	prometheus.MustRegister(
		r.EventsPublished,
		r.EventsHandled,
		r.HandlerErrors,
	)

	return r
}

// Handler returns an HTTP handler exposing Prometheus metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}
