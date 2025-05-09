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
)

func init() {
	prometheus.MustRegister(Latency, Tokens, CostUSD)
}
