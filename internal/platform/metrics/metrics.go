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

	EmailsFetched       prometheus.Counter
	EmailsDeduplicated  prometheus.Counter
	FetchErrors         *prometheus.CounterVec
	FetchDuration       prometheus.Histogram
	NewslettersDetected prometheus.Counter
	EmailsRejected      prometheus.Counter

	ArticlesExtracted  prometheus.Counter
	ExtractionErrors   *prometheus.CounterVec
	ExtractionDuration prometheus.Histogram

	EmbeddingsGenerated prometheus.Counter
	EmbeddingErrors     *prometheus.CounterVec
	EmbeddingDuration   prometheus.Histogram
	LLMLatency          *prometheus.HistogramVec
	LLMTokens           *prometheus.CounterVec

	ClustersCreated    prometheus.Counter
	ClustersMerged     prometheus.Counter
	ArticlesClustered  prometheus.Counter
	ClusteringErrors   *prometheus.CounterVec
	ClusteringDuration prometheus.Histogram

	DigestsGenerated prometheus.Counter
	DigestErrors     *prometheus.CounterVec
	DigestDuration   prometheus.Histogram
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
		EmailsFetched: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nektar_emails_fetched_total",
			Help: "Total number of new emails fetched and stored",
		}),
		EmailsDeduplicated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nektar_emails_deduplicated_total",
			Help: "Total number of emails skipped as duplicates",
		}),
		FetchErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nektar_fetch_errors_total",
				Help: "Total number of fetch pipeline errors",
			},
			[]string{"reason"},
		),
		FetchDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "nektar_fetch_duration_seconds",
			Help:    "Duration of a full fetch job run",
			Buckets: prometheus.DefBuckets,
		}),
		NewslettersDetected: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nektar_newsletters_detected_total",
			Help: "Total number of emails classified as newsletters",
		}),
		EmailsRejected: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nektar_emails_rejected_total",
			Help: "Total number of emails rejected as non-newsletters",
		}),
		ArticlesExtracted: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nektar_articles_extracted_total",
			Help: "Total number of articles extracted from newsletters",
		}),
		ExtractionErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nektar_extraction_errors_total",
				Help: "Total number of extraction pipeline errors",
			},
			[]string{"reason"},
		),
		ExtractionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "nektar_extraction_duration_seconds",
			Help:    "Duration of an extraction handler run",
			Buckets: prometheus.DefBuckets,
		}),
		EmbeddingsGenerated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nektar_embeddings_generated_total",
			Help: "Total number of article embeddings generated",
		}),
		EmbeddingErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nektar_embedding_errors_total",
				Help: "Total number of embedding pipeline errors",
			},
			[]string{"reason"},
		),
		EmbeddingDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "nektar_embedding_duration_seconds",
			Help:    "Duration of an embedding handler run",
			Buckets: prometheus.DefBuckets,
		}),
		LLMLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "nektar_llm_latency_seconds",
				Help:    "Latency of LLM API calls",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"provider", "operation"},
		),
		LLMTokens: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nektar_llm_tokens_total",
				Help: "Total tokens consumed by LLM API calls",
			},
			[]string{"provider", "operation"},
		),
		ClustersCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nektar_clusters_created_total",
			Help: "Total number of clusters created",
		}),
		ClustersMerged: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nektar_clusters_merged_total",
			Help: "Total number of cluster merges",
		}),
		ArticlesClustered: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nektar_articles_clustered_total",
			Help: "Total number of articles assigned to clusters",
		}),
		ClusteringErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nektar_clustering_errors_total",
				Help: "Total number of clustering pipeline errors",
			},
			[]string{"reason"},
		),
		ClusteringDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "nektar_clustering_duration_seconds",
			Help:    "Duration of a clustering handler run",
			Buckets: prometheus.DefBuckets,
		}),
		DigestsGenerated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nektar_digests_generated_total",
			Help: "Total number of digests marked ready",
		}),
		DigestErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nektar_digest_errors_total",
				Help: "Total number of digest pipeline errors",
			},
			[]string{"reason"},
		),
		DigestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "nektar_digest_duration_seconds",
			Help:    "Duration of a digest handler run",
			Buckets: prometheus.DefBuckets,
		}),
	}

	prometheus.MustRegister(
		r.EventsPublished,
		r.EventsHandled,
		r.HandlerErrors,
		r.EmailsFetched,
		r.EmailsDeduplicated,
		r.FetchErrors,
		r.FetchDuration,
		r.NewslettersDetected,
		r.EmailsRejected,
		r.ArticlesExtracted,
		r.ExtractionErrors,
		r.ExtractionDuration,
		r.EmbeddingsGenerated,
		r.EmbeddingErrors,
		r.EmbeddingDuration,
		r.LLMLatency,
		r.LLMTokens,
		r.ClustersCreated,
		r.ClustersMerged,
		r.ArticlesClustered,
		r.ClusteringErrors,
		r.ClusteringDuration,
		r.DigestsGenerated,
		r.DigestErrors,
		r.DigestDuration,
	)

	return r
}

// Handler returns an HTTP handler exposing Prometheus metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}
