package monitor

import "github.com/prometheus/client_golang/prometheus"

var (
	Latency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_request_latency_seconds",
			Help:    "Latency of LLM requests by provider & endpoint",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"provider", "endpoint", "status"},
	)

	Tokens = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_tokens_total",
			Help: "Prompt / completion tokens",
		},
		[]string{"provider", "type"},
	)

	CostUSD = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_cost_usd_total",
			Help: "Accumulated cost (USD)",
		},
		[]string{"provider", "model"},
	)

	OptScore = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_optimizer_score",
			Help:    "Optimizer score by provider & template",
			Buckets: []float64{1, 3, 5, 7, 9, 10},
		},
		[]string{"provider", "template"},
	)

	CacheHit = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prompt_cache_hit_total", Help: "LLM prompt cache hit",
	})
	CacheMiss = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prompt_cache_miss_total", Help: "LLM prompt cache miss",
	})

	CompareLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_compare_latency_seconds",
			Help:    "Latency per provider in compare run",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"provider"},
	)
)

func init() {
	prometheus.MustRegister(Latency, Tokens, CostUSD, OptScore, CacheHit, CacheMiss, CompareLatency)
}
