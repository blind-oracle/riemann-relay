package main

import "github.com/prometheus/client_golang/prometheus"

var (
	promTgtSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tgt_sent",
			Help: "Number of events sent by target",
		},

		[]string{"output", "target"},
	)

	promTgtDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tgt_dropped",
			Help: "Number of events dropped by target",
		},

		[]string{"output", "target"},
	)

	promTgtConnFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tgt_conn_failed",
			Help: "Number of connection failures by target",
		},

		[]string{"output", "target"},
	)

	promTgtFlushFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tgt_flush_failed",
			Help: "Number of flush failures by target",
		},

		[]string{"output", "target"},
	)

	promTgtFlushDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tgt_flush_duration",
			Help:    "Duration of flushes by target",
			Buckets: prometheus.LinearBuckets(0.01, 0.01, 10),
		},

		[]string{"output", "target"},
	)

	promOutProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "out_sent",
			Help: "Number of events sent by output",
		},

		[]string{"output"},
	)

	promOutNoTarget = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "out_no_target",
			Help: "Number of events dropped because no targets alive by output",
		},

		[]string{"output"},
	)
)

func init() {
	prometheus.MustRegister(
		promTgtSent,
		promTgtDropped,
		promTgtConnFailed,
		promTgtFlushFailed,
		promTgtFlushDuration,

		promOutProcessed,
		promOutNoTarget,
	)
}
